// Package swift analyzes Swift with a scanner (lex.go, decls.go). `import`
// statements name modules, which resolve to the directories of the project's own
// targets (Package.swift,
// else a directory named after the module), to the Swift toolchain's libraries and
// Apple's SDK frameworks (modules.go), or to the Swift package providing them, read
// from Package.swift, Package.resolved and Xcode projects (manifest.go). A
// Package.swift's `.package(...)` lines are imports of the packages they declare. A
// Swift module has no imports between its own files, so type names a file uses
// resolve to the file of the same module that declares them.
package swift

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cocoapods"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoSwiftPM = "swiftpm"
	ecoStd     = "swift-std"
	ecoApple   = "apple-sdk"
)

// Implements: REQ-SWIFT-001
type Plugin struct{}

func (Plugin) Name() string { return "swift" }
func (Plugin) Version() int { return 2 }

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

// Implements: REQ-SWIFT-002, REQ-SWIFT-003, REQ-SWIFT-006, REQ-SWIFT-011, REQ-SWIFT-014
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	p := parse(src)
	ex := &lang.Extraction{Imports: p.imports}
	var modules []string
	for _, imp := range p.imports {
		modules = append(modules, imp.Module)
	}
	if manifest(src) {
		ex.Imports = append(ex.Imports, packageImports(string(src))...)
	}
	// The type uses: each name once, at its first line, unless the file declares
	// it outside a body.
	refs := p.refs
	slices.SortFunc(refs, func(a, b ref) int { return cmp.Or(cmp.Compare(a.line, b.line), cmp.Compare(a.name, b.name)) })
	seen := map[string]bool{}
	scope := kindType + ":" + strings.Join(modules, ",")
	for _, r := range refs {
		if seen[r.name] || p.declared[r.name] {
			continue
		}
		seen[r.name] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: r.name, Module: r.name, Name: scope, Line: r.line})
	}
	slices.SortStableFunc(ex.Imports, func(a, b lang.RawImport) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Spec, b.Spec))
	})
	ex.Symbols = p.symbols.List()
	return ex, nil
}

// typeKinds names the symbol kinds of a type declaration by its keyword.
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
