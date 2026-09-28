// Package commonlisp analyzes Common Lisp sources (.lisp, .lsp, and .cl files
// that are not OpenCL kernels), ASDF system definitions (.asd), and the
// manifests of Qlot (qlfile, qlfile.lock) and ocicl (ocicl.csv).
//
// A defsystem's :components are edges from the .asd to its files (modules
// and :pathname followed) and its :depends-on names systems: one the
// repository defines (secondary foo/bar systems and package-inferred
// systems' files included), one the implementation provides (ASDF, UIOP,
// SBCL's contribs: the hidden cl-std island), else the Quicklisp project
// releasing it. In sources, a defpackage's :use, :import-from and
// :local-nicknames, in-package and qualified symbols (pkg:sym) name
// packages: the repository's own resolve to the file defining them, the
// standard's and implementations' to cl-std, others to the declared system
// they belong to (a curated table, the package's name). ql:quickload,
// asdf:load-system, require and load name systems and files. Pins come from
// qlfile.lock, the qlfile (a dist date, a git commit) and ocicl.csv (image
// digests); without them a Quicklisp project floats.
//
// Lisp is read by a small reader of its own (read.go), not the vendored
// tree-sitter grammar (REQ-COMMONLISP-010).
package commonlisp

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoQuicklisp = "quicklisp"
	ecoStd       = "cl-std"
)

// Classes of the manifests, which share their extensions with other files.
const (
	classQlfile = "qlfile"
	classLock   = "qlfile.lock"
	classOcicl  = "ocicl.csv"
)

// lispSource reports whether f is Lisp source: .lisp, .lsp, and .cl when the
// scan did not find an OpenCL kernel in it.
func lispSource(f *scan.File) bool {
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".lisp", ".lsp":
		return true
	case ".cl":
		return f.Lang == "Common Lisp"
	}
	return false
}

// Implements: REQ-COMMONLISP-001
type Plugin struct{}

func (Plugin) Name() string { return "commonlisp" }
func (Plugin) Version() int { return 1 }

// Claims takes Lisp sources, .asd files, qlfile, qlfile.lock and ocicl.csv,
// except what Qlot installed into .qlot/ and ocicl into systems/ beside an
// ocicl.csv.
//
// Implements: REQ-COMMONLISP-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	switch base := path.Base(f.Path); {
	case lispSource(f), strings.EqualFold(path.Ext(base), ".asd"):
	case base == classQlfile, base == classLock, base == classOcicl:
	default:
		return false
	}
	return !ignored(f)
}

// Class tells the manifests apart from other files of their extensions.
//
// Implements: REQ-COMMONLISP-001
func (Plugin) Class(f *scan.File) string {
	switch base := path.Base(f.Path); base {
	case classQlfile, classLock, classOcicl:
		return base
	}
	return ""
}

// Implements: REQ-COMMONLISP-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoQuicklisp, Name: "Quicklisp projects"},
		{ID: ecoStd, Name: "Common Lisp built-ins", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-COMMONLISP-002, REQ-COMMONLISP-003, REQ-COMMONLISP-006
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch path.Base(f.Path) {
	case classQlfile:
		return extractQlfile(src), nil
	case classLock:
		return extractLock(src), nil
	case classOcicl:
		return extractOcicl(src), nil
	}
	in := read(src)
	return &lang.Extraction{Imports: in.imports, Symbols: in.symbols}, nil
}
