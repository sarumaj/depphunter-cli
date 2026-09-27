// Package cpp analyzes C and C++ with one plugin, since their files include each
// other freely: a C++ source includes a C header, both share one include path. .c
// files are parsed with the C grammar; every other extension, .h included, with
// the C++ grammar, which reads nearly all C headers too and is the only one that
// reads the classes, namespaces and templates of C++ headers.
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

	cgrammar "github.com/odvcencio/gotreesitter/grammars/c"
	cppgrammar "github.com/odvcencio/gotreesitter/grammars/cpp"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
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

// The patterns C and C++ share. @def.<kind> names a definition, @decl.func a
// function declared but not defined (a prototype, a member function), and @guard a
// valueless #define, which is a header guard when an #ifndef of the same name
// encloses it. Where a function sits (a class, a function body) is read from the
// capture's ancestors in Extract.
const cQuery = `
(function_definition declarator: (function_declarator declarator: (_) @def.func))
(function_definition declarator: (pointer_declarator declarator: (function_declarator declarator: (_) @def.func)))
(function_definition declarator: (pointer_declarator declarator: (pointer_declarator declarator: (function_declarator declarator: (_) @def.func))))
(declaration declarator: (function_declarator declarator: (_) @decl.func))
(declaration declarator: (pointer_declarator declarator: (function_declarator declarator: (_) @decl.func)))
(field_declaration declarator: (function_declarator declarator: (_) @decl.func))
(field_declaration declarator: (pointer_declarator declarator: (function_declarator declarator: (_) @decl.func)))
(struct_specifier name: (_) @def.struct body: (_))
(union_specifier name: (_) @def.union body: (_))
(enum_specifier name: (_) @def.enum body: (_))
(type_definition declarator: (_) @def.type)
(preproc_def name: (identifier) @def.macro value: (_))
(preproc_def name: (identifier) @guard !value)
(preproc_function_def name: (identifier) @def.macro)
`

// What only C++ has.
const cppQuery = cQuery + `
(function_definition declarator: (reference_declarator (function_declarator declarator: (_) @def.func)))
(declaration declarator: (reference_declarator (function_declarator declarator: (_) @decl.func)))
(field_declaration declarator: (reference_declarator (function_declarator declarator: (_) @decl.func)))
(class_specifier name: (_) @def.class body: (_))
(alias_declaration name: (_) @def.type)
(namespace_definition name: (_) @def.namespace)
`

var (
	cGrammar   = treesitter.MustGrammar("c", cgrammar.Language(), cQuery)
	cppGrammar = treesitter.MustGrammar("cpp", cppgrammar.Language(), cppQuery)
)

// Implements: REQ-CPP-001
type Plugin struct{}

func (Plugin) Name() string { return "cpp" }
func (Plugin) Version() int { return 1 }
func (Plugin) Claims(f *scan.File) bool {
	return exts[strings.ToLower(path.Ext(f.Path))] && !f.Binary
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
	ex := &lang.Extraction{Imports: includes}
	g := cppGrammar
	if strings.EqualFold(path.Ext(f.Path), ".c") {
		g = cGrammar
	}
	var defs []def
	namespaces := map[string]bool{}
	err := g.Matches(src, func(m treesitter.Match) {
		for _, c := range m {
			if c.Line < len(dead) && dead[c.Line] {
				continue
			}
			scopes := c.Scopes()
			if slices.ContainsFunc(scopes, func(s treesitter.Scope) bool { return s.Type == "compound_statement" }) {
				continue // a local declaration, or `std::string s(x);` read as one
			}
			kind, _ := strings.CutPrefix(c.Name, "def.")
			switch c.Name {
			case "guard":
				if slices.ContainsFunc(scopes, func(s treesitter.Scope) bool {
					return s.Type == "preproc_ifdef" && s.Name == c.Text
				}) {
					continue
				}
				defs = append(defs, def{c.Text, "macro", c.Line, false})
			case "def.macro":
				defs = append(defs, def{c.Text, kind, c.Line, false})
			case "def.namespace":
				name := strings.ReplaceAll(strings.Join(strings.Fields(c.Text), ""), "::", ".")
				if !namespaces[name] { // namespaces reopen; one symbol for each
					namespaces[name] = true
					defs = append(defs, def{name, kind, c.Line, false})
				}
			case "def.func", "decl.func":
				name := qualified(c.Text)
				if macroCall(name) {
					continue // TEST(Suite, Case) { ... } parses as a function
				}
				if owner := owners(scopes, false); owner != "" {
					name = owner + "." + name
				}
				fn := "func"
				if strings.Contains(name, ".") {
					fn = "method"
				}
				defs = append(defs, def{name, fn, c.Line, c.Name == "decl.func"})
			case "def.type":
				if name := declaratorName(c.Text); name != "" {
					if owner := owners(scopes, false); owner != "" {
						name = owner + "." + name
					}
					defs = append(defs, def{name, kind, c.Line, false})
				}
			default: // class, struct, union, enum
				name := qualified(c.Text)
				if owner := owners(scopes, true); owner != "" {
					name = owner + "." + name
				}
				defs = append(defs, def{name, kind, c.Line, false})
			}
		}
	})
	ex.Symbols = symbols(defs)
	return ex, err
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

// owners joins the classes around a capture, outermost first. A class's own name
// sits directly in its specifier, which is not its owner (skipSelf).
func owners(scopes []treesitter.Scope, skipSelf bool) string {
	var names []string
	for i, s := range scopes {
		switch s.Type {
		case "class_specifier", "struct_specifier", "union_specifier":
			if (skipSelf && i == 0) || s.Name == "" {
				continue
			}
			names = append(names, qualified(s.Name))
		}
	}
	slices.Reverse(names)
	return strings.Join(names, ".")
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

var (
	identifier = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	upper      = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)
)

// declaratorName finds the name a typedef declares in its declarator: "node_t",
// "*node_p", "(*callback)(int)".
func declaratorName(declarator string) string {
	for _, id := range identifier.FindAllString(declarator, -1) {
		switch id {
		case "const", "volatile", "restrict", "__restrict", "__cdecl", "__stdcall", "__fastcall", "WINAPI", "CALLBACK":
			continue
		}
		return id
	}
	return ""
}

// macroCall reports whether a defined "function" is a macro invocation in disguise,
// as the test and benchmark macros are: an all-capitals name.
func macroCall(name string) bool { return upper.MatchString(name) }
