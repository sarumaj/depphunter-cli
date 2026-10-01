// Package elm analyzes Elm projects: modules (.elm) and elm.json.
//
// `import A.B` names the module A/B.elm under a source directory of the project
// the importing file belongs to (elm.json `source-directories` of an application,
// src/ of a package, and tests/ for elm-test), else the package exposing it: the
// elm.json of the package as installed in ELM_HOME when it is on disk, else a
// curated table of well-known modules (Html is elm/html, Json.Decode.Pipeline
// NoRedInk/elm-json-decode-pipeline) or a declared package whose name spells the
// module (List.Extra is elm-community/list-extra). elm/core's modules (Basics,
// List, Maybe, ...) are the package elm/core, which elm.json pins like any other.
// elm.json's dependencies are imports of the packages it names.
//
// Elm is read by a small lexer of its own (source.go), not the vendored
// tree-sitter grammar (REQ-ELM-010).
package elm

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// ecosystemElm is the island of Elm packages, named author/name as elm.json and
// package.elm-lang.org name them.
const ecosystemElm = "elm"

const classManifest = "manifest"

// Implements: REQ-ELM-001
type Plugin struct{}

func (Plugin) Name() string { return "elm" }
func (Plugin) Version() int { return 1 }

// Claims takes Elm modules and elm.json, except what the compiler keeps in
// elm-stuff/.
//
// Implements: REQ-ELM-001
func (p Plugin) Claims(f *scan.File) bool {
	if f.Binary || inElmStuff(f.Path) {
		return false
	}
	return path.Ext(f.Path) == ".elm" || p.Class(f) != ""
}

// inElmStuff reports whether a path is inside the compiler's elm-stuff/ directory.
func inElmStuff(p string) bool {
	return strings.HasPrefix(p, "elm-stuff/") || strings.Contains(p, "/elm-stuff/")
}

// Class tells elm.json apart from other JSON files.
//
// Implements: REQ-ELM-001
func (Plugin) Class(f *scan.File) string {
	if path.Base(f.Path) == "elm.json" {
		return classManifest
	}
	return ""
}

// Ecosystems is the one island of Elm packages.
//
// Implements: REQ-ELM-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{{ID: ecosystemElm, Name: "Elm packages"}}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all, elmHome(root)), nil
}

// Implements: REQ-ELM-002, REQ-ELM-003, REQ-ELM-005
func (p Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	if p.Class(f) == classManifest {
		return extractManifest(source), nil
	}
	return extractSource(source), nil
}
