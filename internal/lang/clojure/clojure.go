// Package clojure analyzes Clojure, ClojureScript and babashka sources (.clj, .cljs,
// .cljc, .bb, and scripts whose #! line runs bb) with the manifests that declare
// their dependencies: deps.edn, bb.edn, shadow-cljs.edn, Leiningen's project.clj and
// Boot's build.boot.
//
// A namespace an ns form or a top-level require/use names resolves to the project
// file defining it (the source paths' a/b_c.clj(c|s) first, then any file whose ns
// form declares it), to Clojure's and ClojureScript's own namespaces (the hidden
// clojure-std island), or to the Maven artifact (group:artifact) a manifest declares
// that provides it - Clojars is a Maven repository, so Clojure's libraries share the
// Java plugins' maven island. :import names Java classes: the JDK's go to the jdk
// island, a record or type of the project to its file, others to a declared
// artifact as the Java plugin matches a Java import to one (java.Artifacts). A
// ClojureScript string require is an npm package.
//
// Clojure is read by a reader of its own (internal/lang/edn), not the vendored
// tree-sitter grammar (REQ-CLOJURE-011).
package clojure

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoMaven = "maven"
	ecoStd   = "clojure-std"
	ecoJDK   = "jdk"
	ecoNPM   = "npm"
	ecoNode  = "node"
)

var sourceExts = map[string]bool{".clj": true, ".cljs": true, ".cljc": true, ".bb": true}

// ignoredDirs are caches: shadow-cljs's .shadow-cljs, tools.deps' .cpcache,
// clojure-lsp's .lsp, and node_modules beside a shadow-cljs build. .clj-kondo is
// read: projects keep their linter hooks there, as code (metabase's :paths list it).
var ignoredDirs = map[string]bool{".cpcache": true, ".shadow-cljs": true, ".lsp": true, "node_modules": true}

// Implements: REQ-CLOJURE-001
type Plugin struct{}

func (Plugin) Name() string { return "clojure" }
func (Plugin) Version() int { return 2 }

// Claims takes Clojure sources, the manifests, other .edn files (claimed as data:
// they declare nothing) and babashka scripts without an extension.
//
// Implements: REQ-CLOJURE-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || skipped(f.Path) {
		return false
	}
	base := path.Base(f.Path)
	ext := path.Ext(base)
	return sourceExts[ext] || ext == ".edn" || manifestNames[base] ||
		(f.Interpreter == "bb" && scan.Language(f.Path) == "")
}

func skipped(p string) bool {
	for _, seg := range strings.Split(path.Dir(p), "/") {
		if ignoredDirs[seg] {
			return true
		}
	}
	return false
}

// Ecosystems: Maven artifacts (shared with the Java plugins), Clojure's own
// namespaces, the JDK (shared with Java) for :import, and npm for ClojureScript.
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoMaven, Name: "Maven"},
		{ID: ecoStd, Name: "Clojure standard library", Std: true},
		{ID: ecoJDK, Name: "Java standard library", Std: true},
		{ID: ecoNPM, Name: "npm"},
		{ID: ecoNode, Name: "Node.js built-ins", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Class tells a manifest (by its name) from sources and from .edn data.
//
// Implements: REQ-CLOJURE-001
func (Plugin) Class(f *scan.File) string {
	if base := path.Base(f.Path); manifestNames[base] {
		return base
	}
	return ""
}

// Extract reads a manifest's dependencies, or a source's namespace, requires,
// imports and definitions. Other .edn files declare nothing.
//
// Implements: REQ-CLOJURE-002, REQ-CLOJURE-003, REQ-CLOJURE-004
func (p Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	if kind := p.Class(f); kind != "" {
		m := readManifest(kind, src)
		ex := &lang.Extraction{Imports: manifestImports(m), Symbols: []lang.Symbol{}}
		if len(m.requires) > 0 { // bb.edn tasks' :requires
			s := &source{seen: map[string]bool{}, names: map[string]bool{}}
			for _, r := range m.requires {
				s.libspec(r, "", r.Line)
			}
			ex.Imports = append(ex.Imports, s.imports...)
		}
		return ex, nil
	}
	if path.Ext(f.Path) == ".edn" {
		return &lang.Extraction{Symbols: []lang.Symbol{}}, nil
	}
	s := readSource(src)
	symbols := s.symbols
	if symbols == nil {
		symbols = []lang.Symbol{}
	}
	var set lang.SymbolSet
	for _, sym := range symbols {
		set.Add(sym.Name, sym.Kind, sym.Line)
	}
	out := set.List()
	if out == nil {
		out = []lang.Symbol{}
	}
	return &lang.Extraction{Imports: s.imports, Symbols: out}, nil
}
