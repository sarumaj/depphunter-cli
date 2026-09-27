// Package purescript analyzes PureScript projects: modules (.purs), spago's
// spago.yaml and spago.lock, legacy spago.dhall and packages.dhall, and bower.json.
//
// `import A.B` names the module a source file of the importing package declares
// (spago.yaml packages read src/ and test/, spago.dhall its `sources` globs; the
// package's own files first, then its workspace's), else the registry package
// providing it: the package spago installed in .spago/ when it is on disk, else
// a curated table (Data.Map is ordered-collections, Effect.Aff aff) or a listed
// package whose name spells the module (Node.FS is node-fs, Data.Maybe maybe).
// The compiler's Prim modules are built in. `foreign import` needs the module's
// JavaScript companion, the .js file of the same name beside it.
//
// PureScript is read by a small lexer of its own (source.go), not the vendored
// tree-sitter grammar (REQ-PURESCRIPT-010), and Dhall by a small evaluator
// (dhall.go).
package purescript

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// ecoPureScript is the island of registry packages, named as the PureScript
// registry and spago name them (prelude, halogen); packages installed from git
// are named by their repository.
const ecoPureScript = "purescript"

// ecoStd is the island of the compiler's built-in Prim modules.
const ecoStd = "purescript-std"

// Classes of the files the plugin claims besides modules.
const (
	classYAML  = "spago.yaml"
	classLock  = "spago.lock"
	classDhall = "dhall"     // a spago configuration written in Dhall
	classSet   = "dhall-set" // packages.dhall: a package set
	classBower = "bower"
)

// Implements: REQ-PURESCRIPT-001
type Plugin struct{}

func (Plugin) Name() string { return "purescript" }
func (Plugin) Version() int { return 1 }

// Claims takes PureScript modules and spago's and bower's manifests, except
// what spago and bower install (.spago/, bower_components/).
//
// Implements: REQ-PURESCRIPT-001
func (p Plugin) Claims(f *scan.File) bool {
	if f.Binary || installedPath(f.Path) {
		return false
	}
	return path.Ext(f.Path) == ".purs" || p.Class(f) != ""
}

// installedPath reports whether a path is inside spago's or bower's package
// directories.
func installedPath(p string) bool {
	for _, d := range []string{".spago", "bower_components"} {
		if strings.HasPrefix(p, d+"/") || strings.Contains(p, "/"+d+"/") {
			return true
		}
	}
	return false
}

// Class tells spago's and bower's manifests apart from other YAML, JSON and
// Dhall files. A Dhall file is a spago configuration when it is spago.dhall or
// its name says spago (spago-test.dhall) or it is a test/ configuration
// (test.dhall); packages.dhall is the package set.
//
// Implements: REQ-PURESCRIPT-001
func (Plugin) Class(f *scan.File) string {
	base := path.Base(f.Path)
	switch {
	case base == "spago.yaml":
		return classYAML
	case base == "spago.lock":
		return classLock
	case base == "packages.dhall":
		return classSet
	case base == "bower.json":
		return classBower
	case path.Ext(base) == ".dhall" && (strings.Contains(base, "spago") || base == "test.dhall"):
		return classDhall
	}
	return ""
}

// Ecosystems are the registry's packages and the compiler's built-in modules.
//
// Implements: REQ-PURESCRIPT-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoPureScript, Name: "PureScript packages"},
		{ID: ecoStd, Name: "PureScript built-ins", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-PURESCRIPT-002, REQ-PURESCRIPT-003, REQ-PURESCRIPT-005
func (p Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch p.Class(f) {
	case classYAML:
		return extractSpagoYAML(src), nil
	case classLock:
		return extractLock(src), nil
	case classSet:
		return extractDhall(src, true), nil
	case classDhall:
		return extractDhall(src, false), nil
	case classBower:
		return extractBower(src), nil
	}
	return extractSource(src), nil
}
