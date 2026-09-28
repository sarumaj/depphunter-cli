// Package haskell analyzes Haskell - modules, literate modules, .hs-boot and hsc2hs
// files - and its package descriptions and projects: .cabal files, hpack's
// package.yaml, cabal.project and stack.yaml. An import resolves to the file of the
// repository that declares the module (the importer's package, a package of its
// project, a local package it depends on), else to the package that provides it: GHC's
// own libraries (base, ghc-prim, template-haskell, ghc) make the hidden haskell-std
// island, everything else is a Hackage package, pinned by the build plan, the freeze
// file, stack.yaml.lock or extra-deps, or as build-depends declares it (resolve.go).
// The dependencies of package descriptions and the packages and extra-deps of
// projects are imports of what they name.
//
// Haskell is read by a lexer of its own (lex.go), not the vendored tree-sitter
// grammar, which was slow and lost many files to CPP and extensions (REQ-HASKELL-011).
package haskell

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoHackage = "hackage"
	ecoStd     = "haskell-std"
)

// Implements: REQ-HASKELL-001
type Plugin struct{}

func (Plugin) Name() string { return "haskell" }
func (Plugin) Version() int { return 2 }

// Claims takes Haskell sources (.hs, .lhs, .hs-boot, .hsc), package descriptions
// (*.cabal, package.yaml) and project files (cabal.project, stack.yaml), except what
// cabal and stack build into dist-newstyle and .stack-work.
//
// Implements: REQ-HASKELL-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary || ignored(f.Path) {
		return false
	}
	if sourceExt(f.Path) != "" {
		return true
	}
	return manifestClass(f.Path) != ""
}

func manifestClass(p string) string {
	base := path.Base(p)
	switch {
	case strings.HasSuffix(base, ".cabal"):
		return "cabal"
	case base == "cabal.project" || base == "package.yaml" || base == "stack.yaml":
		return base
	}
	return ""
}

// Class tells package.yaml and stack.yaml apart from other YAML files, and
// cabal.project from other files of its extension.
//
// Implements: REQ-HASKELL-001
func (Plugin) Class(f *scan.File) string {
	switch c := manifestClass(f.Path); c {
	case "cabal", "":
		return ""
	default:
		return c
	}
}

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoHackage, Name: "Hackage"},
		{ID: ecoStd, Name: "GHC libraries", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Extract dispatches on the file: a package description, a project file, or a
// module.
//
// Implements: REQ-HASKELL-002, REQ-HASKELL-003, REQ-HASKELL-005, REQ-HASKELL-011
func (p Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch manifestClass(f.Path) {
	case "cabal":
		return extractCabal(src), nil
	case "cabal.project":
		return extractCabalProject(src), nil
	case "package.yaml":
		return extractHpack(src), nil
	case "stack.yaml":
		return extractStack(src), nil
	}
	return extractSource(src, sourceExt(f.Path) == ".lhs"), nil
}
