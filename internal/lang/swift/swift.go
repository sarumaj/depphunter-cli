// Package swift analyzes Swift with tree-sitter. `import` statements name modules,
// which resolve to the directories of the project's own targets (Package.swift,
// else a directory named after the module), to the Swift toolchain's libraries and
// Apple's SDK frameworks (modules.go), or to the Swift package providing them, read
// from Package.swift, Package.resolved and Xcode projects (manifest.go). A
// Package.swift's `.package(...)` lines are imports of the packages they declare. A
// Swift module has no imports between its own files, so type names a file uses
// resolve to the file of the same module that declares them.
package swift

import (
	"bytes"
	"cmp"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/swift"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cocoapods"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoSwiftPM = "swiftpm"
	ecoStd     = "swift-std"
	ecoApple   = "apple-sdk"
)

// @import is an import declaration (its text parsed in Go); @kind/@def.type a type
// declaration (class, struct, enum, actor or extension); @def.<kind> another
// definition; @ref a type name the code uses. Where a definition sits is read from
// its ancestors.
const query = `
(import_declaration) @import

(class_declaration declaration_kind: _ @kind name: _ @def.type)
(protocol_declaration name: (type_identifier) @def.protocol)
(protocol_function_declaration name: (simple_identifier) @def.method)
(function_declaration name: (simple_identifier) @def.func)
(init_declaration "init" @def.init)
(typealias_declaration name: (type_identifier) @def.alias)
(property_declaration name: (pattern) @def.property)

(user_type) @ref
(call_expression (simple_identifier) @ref)
`

var grammar = treesitter.MustGrammar("swift", swift.Language(), query)

// Implements: REQ-SWIFT-001
type Plugin struct{}

func (Plugin) Name() string { return "swift" }
func (Plugin) Version() int { return 1 }

// Claims takes every Swift source, Package.swift and its versioned variants
// (Package@swift-5.9.swift) included, except what SwiftPM checked out under .build.
//
// Implements: REQ-SWIFT-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && strings.ToLower(path.Ext(f.Path)) == ".swift" && !buildOutput(f.Path)
}

// buildOutput reports whether a path is under SwiftPM's build directory, where it
// keeps its checkouts of dependencies: code of other packages, not the project's.
func buildOutput(p string) bool {
	return strings.HasPrefix(p, ".build/") || strings.Contains(p, "/.build/")
}

func (Plugin) Ecosystems() []lang.Ecosystem {
	return append([]lang.Ecosystem{
		{ID: ecoSwiftPM, Name: "Swift packages"},
		{ID: ecoStd, Name: "Swift standard library", Std: true},
		{ID: ecoApple, Name: "Apple SDKs", Std: true},
	}, cocoapods.Ecosystems()...)
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Import kinds, carried in RawImport.Name.
const (
	kindImport = "import" // an import declaration; Module is the module
	kindURL    = "url"    // a manifest's .package(url:); Module is the URL
	kindPath   = "path"   // a manifest's .package(path:); Module is "__DIR__/<path>"
	kindID     = "id"     // a manifest's .package(id:), a registry package
	kindType   = "type"   // a type name the code uses; Name is "type:<imported modules>"
)

// Implements: REQ-SWIFT-002, REQ-SWIFT-003, REQ-SWIFT-006, REQ-SWIFT-011
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	type ref struct {
		name string
		line int
	}
	var refs []ref
	declared := map[string]bool{}
	var modules []string
	err := grammar.Matches(prepare(src), func(m treesitter.Match) {
		for _, c := range m {
			switch c.Name {
			case "import":
				if imp, ok := parseImport(c.Text); ok {
					imp.Line = c.Line
					ex.Imports = append(ex.Imports, imp)
					modules = append(modules, imp.Module)
				}
			case "ref":
				if name := typeName(c.Text); name != "" {
					refs = append(refs, ref{name, c.Line})
				}
			case "def.type":
				kind, _ := m.Get("kind")
				owner, inBody := owner(c.Scopes(), true)
				if inBody {
					continue
				}
				name := typePath(c.Text)
				if kind != "extension" {
					declared[name] = true
				}
				symbols.Add(qualify(owner, name), typeKinds[kind], c.Line)
			case "def.protocol", "def.alias":
				owner, inBody := owner(c.Scopes(), true)
				if inBody {
					continue
				}
				declared[c.Text] = true
				symbols.Add(qualify(owner, c.Text), map[string]string{"def.protocol": "interface", "def.alias": "type"}[c.Name], c.Line)
			case "def.func", "def.method":
				switch owner, inBody := owner(c.Scopes(), true); {
				case inBody:
				case owner != "":
					symbols.Add(owner+"."+c.Text, "method", c.Line)
				default:
					symbols.Add(c.Text, "func", c.Line)
				}
			case "def.init":
				if owner, inBody := owner(c.Scopes(), true); !inBody && owner != "" {
					symbols.Add(owner+".init", "method", c.Line)
				}
			case "def.property":
				if !isIdent(c.Text) {
					continue // a tuple pattern: let (a, b) = ...
				}
				switch owner, inBody := owner(c.Scopes(), true); {
				case inBody:
				case owner != "":
					symbols.Add(owner+"."+c.Text, "property", c.Line)
				default:
					symbols.Add(c.Text, "var", c.Line)
				}
			}
		}
	})
	if manifest(src) {
		ex.Imports = append(ex.Imports, packageImports(string(src))...)
	}
	slices.SortFunc(refs, func(a, b ref) int { return cmp.Or(cmp.Compare(a.line, b.line), cmp.Compare(a.name, b.name)) })
	seen := map[string]bool{}
	scope := kindType + ":" + strings.Join(modules, ",")
	for _, r := range refs {
		if seen[r.name] || declared[r.name] {
			continue
		}
		seen[r.name] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: r.name, Module: r.name, Name: scope, Line: r.line})
	}
	slices.SortStableFunc(ex.Imports, func(a, b lang.RawImport) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Spec, b.Spec))
	})
	ex.Symbols = symbols.List()
	return ex, err
}

// typeKinds names the symbol kinds of a class_declaration by its keyword.
var typeKinds = map[string]string{
	"class": "class", "actor": "class", "struct": "type", "enum": "type", "extension": "extension",
}

// manifest reports whether src is a package manifest: it imports PackageDescription.
func manifest(src []byte) bool {
	return strings.Contains(string(src), "import PackageDescription")
}

// packageImports turns a manifest's `.package(...)` lines into imports of the
// packages they declare.
//
// Implements: REQ-SWIFT-006
func packageImports(src string) []lang.RawImport {
	var out []lang.RawImport
	clean := stripComments(src)
	for _, c := range calls(clean, "package") {
		d := dependencies(".package(" + strings.Join(c.args, ", ") + ")")
		if len(d) != 1 {
			continue
		}
		imp := lang.RawImport{Spec: ".package(" + strings.Join(strings.Fields(strings.Join(c.args, ", ")), " ") + ")", Line: c.line}
		switch {
		case d[0].url != "":
			imp.Module, imp.Name = d[0].url, kindURL
		case d[0].path != "":
			imp.Module, imp.Name = "__DIR__/"+d[0].path, kindPath
		default:
			imp.Module, imp.Name = d[0].id, kindID
		}
		out = append(out, imp)
	}
	return out
}

// directive matches the conditional-compilation lines #if, #elseif, #else, #endif.
func directive(line []byte) bool {
	t := strings.TrimLeft(string(line), " \t")
	for _, d := range []string{"#if", "#elseif", "#else", "#endif"} {
		if rest, ok := strings.CutPrefix(t, d); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\r' || rest[0] == '(' || rest[0] == '!' || rest[0] == '/') {
			return true
		}
	}
	return false
}

// prepare rewrites what the grammar misreads, keeping every line and column:
//
//   - #if/#elseif/#else/#endif lines become spaces. The grammar does not know them
//     where declarations are expected, and its error recovery then drops what they
//     guard (`#else` + `import AppKit` became one error node). With the lines gone
//     every branch is read: each is code the project builds somewhere.
//   - A freestanding macro starting a line (`#expect(x)`, `#Preview { }`) becomes a
//     call (`_expect(x)`): after another statement the grammar fails on it, and
//     a tree with errors costs the parser a retry ladder that takes seconds for a
//     test file of a few hundred lines.
//   - Swift 6.2's `unsafe` expression marker (`unsafe Array($0)`) becomes spaces:
//     the grammar predates it.
//
// Implements: REQ-SWIFT-002
func prepare(src []byte) []byte {
	src = unsafeExpr.ReplaceAllFunc(src, func(m []byte) []byte {
		i := bytes.LastIndex(m, []byte("unsafe"))
		out := append([]byte(nil), m...)
		copy(out[i:], "      ")
		return out
	})
	var out []byte
	start := 0
	for i := 0; i <= len(src); i++ {
		if i < len(src) && src[i] != '\n' {
			continue
		}
		line := src[start:i]
		j := 0
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		if j+1 < len(line) && line[j] == '#' && (line[j+1] == '_' || line[j+1] >= 'a' && line[j+1] <= 'z' || line[j+1] >= 'A' && line[j+1] <= 'Z') {
			if out == nil {
				out = append([]byte(nil), src...)
			}
			if directive(line) {
				for k := start; k < i; k++ {
					if out[k] != '\r' {
						out[k] = ' '
					}
				}
			} else {
				out[start+j] = '_'
			}
		}
		start = i + 1
	}
	if out == nil {
		return src
	}
	return out
}

// unsafeExpr is `unsafe` where an expression starts, followed by one.
var unsafeExpr = regexp.MustCompile(`(?m)(?:^|[{(\[=,:]|\breturn|\btry|\bawait|\bin)[ \t]*unsafe[ \t]+[A-Za-z_(&\[$]`)

// importKinds are the declaration kinds an import may single out
// (`import struct Collections.Deque`).
var importKinds = set("typealias", "struct", "class", "enum", "protocol", "let", "var", "func", "actor", "macro")

// parseImport reads an import declaration: attributes (@testable, @_exported,
// @_implementationOnly, @preconcurrency, @_spi(...)) and access modifiers are
// shown but skipped, a declaration kind singles out one declaration, and the module
// is the first component of the path (`import os.log` imports os).
//
// Implements: REQ-SWIFT-002
func parseImport(text string) (lang.RawImport, bool) {
	spec := strings.Join(strings.Fields(strings.TrimSuffix(strings.TrimSpace(text), ";")), " ")
	rest := spec
	for {
		rest = strings.TrimSpace(rest)
		switch {
		case strings.HasPrefix(rest, "@"):
			i := 1
			for i < len(rest) && identChar(rest[i]) {
				i++
			}
			if i < len(rest) && rest[i] == '(' {
				if end := closing([]byte(rest), i); end > 0 {
					i = end + 1
				}
			}
			rest = rest[i:]
			continue
		}
		word, after, _ := strings.Cut(rest, " ")
		if word == "import" {
			rest = after
			break
		}
		if accessModifiers[word] {
			rest = after
			continue
		}
		return lang.RawImport{}, false
	}
	fields := strings.Fields(rest)
	if len(fields) == 2 && importKinds[fields[0]] {
		fields = fields[1:]
	}
	if len(fields) != 1 {
		return lang.RawImport{}, false
	}
	module, _, _ := strings.Cut(fields[0], ".")
	if !isIdent(module) {
		return lang.RawImport{}, false
	}
	return lang.RawImport{Spec: spec, Module: module, Name: kindImport}, true
}

var accessModifiers = set("public", "internal", "private", "fileprivate", "package", "open")

// typeName is the leading type of a type reference, generic arguments and nested
// names dropped (Foo.Bar<Int> is Foo), when it names a type: capitalized.
func typeName(text string) string {
	t := strings.TrimSpace(text)
	if i := strings.IndexAny(t, ".<?! \t\n"); i >= 0 {
		t = t[:i]
	}
	if t == "" || t[0] < 'A' || t[0] > 'Z' || !isIdent(t) || t == "Self" {
		return ""
	}
	return t
}

func isIdent(s string) bool {
	if s == "" || s[0] >= '0' && s[0] <= '9' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !identChar(s[i]) {
			return false
		}
	}
	return true
}

// bodies are the nodes whose declarations are local: nobody else can name them.
var bodies = set("function_body", "computed_property", "lambda_literal", "statements",
	"computed_getter", "computed_setter", "computed_modify", "willset_didset_block",
	"subscript_declaration", "deinit_declaration")

// owner reads the type a definition belongs to from its ancestors: the types and
// extensions around it joined with ".", and whether it sits in a body, where it is
// nobody's to name. A definition's own node is its first ancestor and is skipped
// when self is set.
func owner(scopes []treesitter.Scope, self bool) (string, bool) {
	var outer []string
	for i, s := range scopes {
		if i == 0 && self {
			continue
		}
		switch {
		case bodies[s.Type]:
			return "", true
		case s.Type == "class_declaration" || s.Type == "protocol_declaration":
			outer = append(outer, typePath(s.Name))
		}
	}
	slices.Reverse(outer)
	return strings.Join(outer, "."), false
}

// typePath is a declared or extended type's name without generic arguments or
// spaces: `Dictionary<K, V>` is Dictionary, `Foo . Bar` Foo.Bar.
func typePath(s string) string {
	if i := strings.Index(s, "<"); i >= 0 {
		s = s[:i]
	}
	return strings.Join(strings.Fields(s), "")
}

func qualify(owner, name string) string {
	if owner == "" {
		return typePath(name)
	}
	return owner + "." + typePath(name)
}
