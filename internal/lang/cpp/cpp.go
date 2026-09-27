// Package cpp analyzes C and C++ with one plugin, since their files include each
// other freely: a C++ source includes a C header, both share one include path.
// Definitions are read by a hand-written scanner (lex.go, decls.go) rather than a
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
// (packages.go), and otherwise to a third-party library named after the include's
// first directory (resolve.go).
package cpp

import (
	"cmp"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoCStd     = "c-std"
	ecoCppStd   = "cpp-std"
	ecoSystem   = "c-system"
	ecoExternal = "c-external"
)

// exts are the extensions the plugin claims.
//
// Implements: REQ-CPP-001
var exts = map[string]bool{
	".c": true, ".h": true, ".cc": true, ".cpp": true, ".cxx": true, ".c++": true,
	".hpp": true, ".hh": true, ".hxx": true, ".h++": true, ".ipp": true, ".inl": true,
}

// Implements: REQ-CPP-001
type Plugin struct{}

func (Plugin) Name() string { return "cpp" }
func (Plugin) Version() int { return 2 }

// Claims takes the C and C++ files, except a ".h" the scan found to be an
// Objective-C header (REQ-LANG-015), which the objc plugin reads.
//
// Implements: REQ-CPP-001, REQ-OBJC-001
func (Plugin) Claims(f *scan.File) bool {
	return exts[strings.ToLower(path.Ext(f.Path))] && !f.Binary && f.Lang != "Objective-C"
}
func (Plugin) Ecosystems() []lang.Ecosystem {
	return append(PackageEcosystems(), []lang.Ecosystem{
		{ID: ecoCStd, Name: "C standard library", Std: true},
		{ID: ecoCppStd, Name: "C++ standard library", Std: true},
		{ID: ecoSystem, Name: "System headers", Std: true},
	}...)
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

type def struct {
	name, kind string
	line       int
	decl       bool // declared, not defined
}

// Implements: REQ-CPP-001, REQ-CPP-003, REQ-CPP-007
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	includes, dead := preprocess(src)
	cplus := !strings.EqualFold(path.Ext(f.Path), ".c")
	return &lang.Extraction{Imports: includes, Symbols: symbols(scanDefinitions(src, dead, cplus))}, nil
}

// symbols orders the definitions by line and drops the declarations of functions
// the file also defines: a prototype ahead of its definition is one function.
func symbols(defs []def) []lang.Symbol {
	slices.SortStableFunc(defs, func(a, b def) int {
		return cmp.Or(cmp.Compare(a.line, b.line), strings.Compare(a.name, b.name))
	})
	defined := map[string]bool{}
	for _, d := range defs {
		if !d.decl {
			defined[d.name] = true
		}
	}
	var set lang.SymbolSet
	for _, d := range defs {
		if !d.decl || !defined[d.name] {
			set.Add(d.name, d.kind, d.line)
		}
	}
	return set.List()
}

// qualified turns a C++ name into a symbol name: "ns::Box<T>::get" -> "ns.Box.get".
// The operator of an operator function stays as written ("Foo.operator==").
func qualified(name string) string {
	name, op, isOp := strings.Cut(name, "operator")
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
		out += "operator" + strings.Join(strings.Fields(op), " ")
	}
	return out
}

var upper = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)

// macroCall reports whether a defined "function" is a macro invocation in disguise,
// as the test and benchmark macros are: an all-capitals name.
func macroCall(name string) bool { return upper.MatchString(name) }
