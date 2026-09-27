// Package ocaml analyzes OCaml - implementations (.ml), interfaces (.mli),
// ocamllex (.mll) and Menhir or ocamlyacc (.mly) sources - with dune's build files
// (dune, dune-project, dune-workspace) and opam's package descriptions (*.opam,
// opam, *.opam.locked).
//
// OCaml files do not import files: they name modules, and dune decides which
// modules a name can mean. A module path resolves to the file of the module in the
// same dune library or executable (its directory, or the tree include_subdirs
// adds), to a module of a library of the repository the component uses (a wrapped
// library's Lib.Module, an unwrapped library's modules), to OCaml's standard
// library (the hidden ocaml-std island), or to the opam package of a library the
// component uses; dune's libraries and ppx rewriters, and the dependencies of
// dune-project and opam files, are imports of what they name (resolve.go).
//
// OCaml is read by a lexer of its own (lex.go), not the vendored tree-sitter
// grammar, which was slow and lost files to errors (REQ-OCAML-011).
package ocaml

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoOpam = "opam"
	ecoStd  = "ocaml-std"
)

// Kinds of claimed files; Class names them.
const (
	classSource    = "source"
	classDune      = "dune"
	classProject   = "dune-project"
	classWorkspace = "dune-workspace"
	classOpam      = "opam"
	classLock      = "opam-lock"
)

// Implements: REQ-OCAML-001
type Plugin struct{}

func (Plugin) Name() string { return "ocaml" }
func (Plugin) Version() int { return 1 }

var sourceExts = map[string]bool{".ml": true, ".mli": true, ".mll": true, ".mly": true}

// fileClass tells the claimed files apart by name.
func fileClass(p string) string {
	base := path.Base(p)
	switch {
	case sourceExts[path.Ext(base)]:
		return classSource
	case base == "dune":
		return classDune
	case base == "dune-project":
		return classProject
	case base == "dune-workspace" || strings.HasPrefix(base, "dune-workspace."):
		return classWorkspace
	case base == "opam" || strings.HasSuffix(base, ".opam") && base != ".opam":
		return classOpam
	case base == "opam.locked" || strings.HasSuffix(base, ".opam.locked"):
		return classLock
	}
	return ""
}

// ignored is build output and local switches: dune's _build, opam's _opam, esy's
// _esy.
func ignored(p string) bool {
	for _, seg := range strings.Split(path.Dir(p), "/") {
		switch seg {
		case "_build", "_opam", "_esy":
			return true
		}
	}
	return false
}

// Claims takes OCaml sources and dune's and opam's files, except build output and
// local opam switches.
//
// Implements: REQ-OCAML-001
func (Plugin) Claims(f *scan.File) bool {
	return !f.Binary && !ignored(f.Path) && fileClass(f.Path) != ""
}

// Class keeps dune, dune-project, opam files and opam.locked apart: they share an
// (empty) extension.
//
// Implements: REQ-OCAML-001
func (Plugin) Class(f *scan.File) string { return fileClass(f.Path) }

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoOpam, Name: "opam"},
		{ID: ecoStd, Name: "OCaml standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Extract dispatches on the kind of file.
//
// Implements: REQ-OCAML-002, REQ-OCAML-003, REQ-OCAML-006, REQ-OCAML-007, REQ-OCAML-011
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch fileClass(f.Path) {
	case classDune:
		return extractDune(src), nil
	case classProject:
		return extractDuneProject(src), nil
	case classWorkspace:
		return extractWorkspace(src), nil
	case classOpam, classLock:
		return extractOpam(src), nil
	}
	return readSource(src, path.Ext(f.Path)), nil
}
