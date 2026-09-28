// Package cpp analyzes C and C++ with one plugin, since their files include each
// other freely: a C++ source includes a C header, both share one include path.
// The C and C++ dialects of GPU programming are read the same way: CUDA (.cu,
// .cuh) and Metal (.metal) as C++, OpenCL C kernels (.cl the scan labels OpenCL,
// .clh) as C; their toolkits' headers are islands of their own (gpu.go).
// Definitions are read by a hand-written scanner (lex.go, declarations.go) rather than a
// grammar: a tolerant recursive descent over the declaration contexts that steps
// over function bodies. .c files are read as C, where `class`, `new` and
// `namespace` are names; every other extension, .h included, as C++.
//
// Includes are read by a line scanner (preproc.go) rather than the parser, so a
// directive anywhere counts and `#if 0` blocks are skipped. `#include` resolves to
// project files (the includer's directory, the include paths of a
// compile_commands.json, then the conventional include/ and src/ directories and a
// unique file whose path ends in the include), to the C and C++ standard libraries
// and the system headers, to a package a vcpkg or Conan manifest declares
// (packages.go) or content the CMake build fetches (fetched.go), and otherwise to a third-party library named after the include's
// first directory (resolve.go).
package cpp

import (
	"cmp"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemCStd     = "c-std"
	ecosystemCppStd   = "cpp-std"
	ecosystemSystem   = "c-system"
	ecosystemExternal = "c-external"
)

// extensions are the extensions the plugin claims.
//
// Implements: REQ-CPP-001
var extensions = map[string]bool{
	".c": true, ".h": true, ".cc": true, ".cpp": true, ".cxx": true, ".c++": true,
	".hpp": true, ".hh": true, ".hxx": true, ".h++": true, ".ipp": true, ".inl": true,
	".cu": true, ".cuh": true, ".metal": true, ".clh": true,
}

// cExtensions are the extensions read as C rather than C++: C, and OpenCL C, whose kernels
// may use C++ keywords as names.
var cExtensions = map[string]bool{".c": true, ".cl": true, ".clh": true}

// Implements: REQ-CPP-001
type Plugin struct {
	// Fetches reads the content the project's build fetches when it is configured
	// (the cmake plugin), which an include of its headers is attributed to. nil
	// leaves such includes to c-external.
	Fetches FetchReader
}

func (Plugin) Name() string { return "cpp" }
func (Plugin) Version() int { return 3 }

// Claims takes the C and C++ files, except a ".h" the scan found to be an
// Objective-C header (REQ-LANG-015), which the objc plugin reads, and the CUDA,
// Metal and OpenCL sources: a ".cl" file only when the scan found an OpenCL kernel
// in it, Common Lisp's otherwise.
//
// Implements: REQ-CPP-001, REQ-OBJC-001, REQ-CPP-015
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	extension := strings.ToLower(path.Ext(f.Path))
	if extension == ".cl" {
		return f.Language == "OpenCL"
	}
	return extensions[extension] && f.Language != "Objective-C"
}

// Ecosystems: the package managers', the standard and system headers', and the GPU
// toolkits' islands. Apple's SDKs are the swift plugin's island, which Metal's
// headers belong to.
//
// Implements: REQ-CPP-016
func (p Plugin) Ecosystems() []lang.Ecosystem {
	out := PackageEcosystems()
	if p.Fetches != nil {
		out = append(out, p.Fetches.FetchIsland())
	}
	return append(out, []lang.Ecosystem{
		{ID: ecosystemCStd, Name: "C standard library", Std: true},
		{ID: ecosystemCppStd, Name: "C++ standard library", Std: true},
		{ID: ecosystemSystem, Name: "System headers", Std: true},
		{ID: ecosystemCUDA, Name: "CUDA Toolkit", Std: true},
		{ID: ecosystemOpenCL, Name: "OpenCL headers", Std: true},
		{ID: swift.AppleEcosystem, Name: "Apple SDKs", Std: true},
	}...)
}

// Implements: REQ-CPP-017
func (p Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	r := newResolver(root, all)
	if p.Fetches != nil {
		Packages{r.packages}.Fetch(p.Fetches.Fetched(all))
	}
	return r, nil
}

type definition struct {
	name, kind  string
	line        int
	declaration bool // declared, not defined
}

// Implements: REQ-CPP-001, REQ-CPP-003, REQ-CPP-007
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	includes, dead := preprocess(source)
	cplus := !cExtensions[strings.ToLower(path.Ext(f.Path))]
	return &lang.Extraction{Imports: includes, Symbols: symbols(scanDefinitions(source, dead, cplus))}, nil
}

// symbols orders the definitions by line and drops the declarations of functions
// the file also defines: a prototype ahead of its definition is one function.
func symbols(definitions []definition) []lang.Symbol {
	slices.SortStableFunc(definitions, func(a, b definition) int {
		return cmp.Or(cmp.Compare(a.line, b.line), strings.Compare(a.name, b.name))
	})
	defined := map[string]bool{}
	for _, d := range definitions {
		if !d.declaration {
			defined[d.name] = true
		}
	}
	var set lang.SymbolSet
	for _, d := range definitions {
		if !d.declaration || !defined[d.name] {
			set.Add(d.name, d.kind, d.line)
		}
	}
	return set.List()
}

// qualified turns a C++ name into a symbol name: "ns::Box<T>::get" -> "ns.Box.get".
// The operator of an operator function stays as written ("Foo.operator==").
func qualified(name string) string {
	name, operator, isOp := strings.Cut(name, "operator")
	var b strings.Builder
	depth := 0
	for _, r := range name {
		switch {
		case r == '<':
			depth++
		case r == '>':
			depth--
		case depth == 0 && r != ' ' && r != '\t' && r != '\n':
			b.WriteRune(r)
		}
	}
	out := strings.ReplaceAll(b.String(), "::", ".")
	if isOp {
		out += "operator" + strings.Join(strings.Fields(operator), " ")
	}
	return out
}

var upper = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)

// macroCall reports whether a defined "function" is a macro invocation in disguise,
// as the test and benchmark macros are: an all-capitals name.
func macroCall(name string) bool { return upper.MatchString(name) }
