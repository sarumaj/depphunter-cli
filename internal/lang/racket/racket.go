// Package racket analyzes Racket modules (.rkt, .rktl load files, .scrbl
// Scribble documents, and .scm/.ss files that start with a #lang line),
// .rktd data files (claimed, nothing to link) and packages' info.rkt.
//
// A module's #lang line names its language, a module path; require forms
// (with only-in, prefix-in, for-syntax, submod, lib, file, planet and the
// rest), include, include-section and a module form's language name the
// others. A string is a path relative to the file; a collection-based path
// such as racket/list resolves to a collection of the repository (a package
// directory whose info.rkt defines its collection, each directory of a
// multi-collection package, each directory of a collects/ tree) when one has
// the file, else to the base collections (the hidden racket-std island), a
// catalog package of a curated table or of the package's info.rkt deps, or an
// unresolved package named by the collection.
//
// Racket is read by a small reader of its own (read.go), not the vendored
// tree-sitter grammar (REQ-RACKET-010).
package racket

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoRaco = "raco"
	ecoStd  = "racket-std"
)

const classInfo = "info"

// Implements: REQ-RACKET-001
type Plugin struct{}

func (Plugin) Name() string { return "racket" }
func (Plugin) Version() int { return 1 }

// Claims takes Racket's files: .rkt, .rktl, .rktd and .scrbl, and a Scheme
// extension (.scm, .ss) or a script without one only when the scan labelled it
// Racket (a #lang line, a racket #! line). Nothing in a compiled/ directory
// (raco make's bytecode and dependency files) is read.
//
// Implements: REQ-RACKET-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".rkt", ".rktl", ".rktd", ".scrbl":
	default:
		if f.Lang != "Racket" {
			return false
		}
	}
	return !strings.HasPrefix(f.Path, "compiled/") && !strings.Contains(f.Path, "/compiled/")
}

// Class tells info.rkt, a package's metadata, apart from modules.
//
// Implements: REQ-RACKET-001
func (Plugin) Class(f *scan.File) string {
	if path.Base(f.Path) == "info.rkt" {
		return classInfo
	}
	return ""
}

// Implements: REQ-RACKET-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoRaco, Name: "Racket packages"},
		{ID: ecoStd, Name: "Racket base collections", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-RACKET-002, REQ-RACKET-003, REQ-RACKET-006
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	if path.Base(f.Path) == "info.rkt" {
		return extractInfo(src), nil
	}
	return extractSource(src, strings.ToLower(path.Ext(f.Path))), nil
}
