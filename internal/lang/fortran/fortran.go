// Package fortran analyzes Fortran sources in free form (.f90, .f95, .f03,
// .f08, .f18 and their upper-case, preprocessed spellings) and fixed form (.f,
// .for, .ftn, .f77, .fpp and upper-case .F, .FOR), fypp templates (.fypp), and
// the Fortran Package Manager's fpm.toml.
//
// `use m` names a module, which any file of the project may define: the resolver
// indexes every source's module and submodule statements (case-insensitive), so
// CMake, Make and fpm projects resolve alike. Intrinsic modules
// (iso_fortran_env, iso_c_binding, ieee_*, omp_lib, openacc) are the hidden
// compiler library; MPI, HDF5 and NetCDF modules are the C libraries the cpp
// plugin maps their headers to. Other modules belong to an fpm dependency: the
// one fpm fetched into build/dependencies/ that defines the module, else a
// declared dependency the module's name spells or a curated table names
// (known.go), else an unresolved module. `include 'x'`, `#include "x"` and fypp's
// `#:include "x"` name files next to the including file or in fpm's include
// directories (resolve.go).
//
// Fortran is read by a small statement reader of its own (source.go), not the
// vendored tree-sitter grammar (REQ-FORTRAN-010).
package fortran

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemFpm      = "fpm"
	ecosystemStd      = "fortran-std"
	ecosystemExternal = "fortran-external"
)

const classManifest = "fpm"

// freeExtensions and fixedExtensions are the source extensions, compared as written: .F90
// and .F are the preprocessed spellings.
var (
	freeExtensions = map[string]bool{
		".f90": true, ".f95": true, ".f03": true, ".f08": true, ".f18": true,
		".F90": true, ".F95": true, ".F03": true, ".F08": true, ".F18": true, ".fypp": true,
	}
	fixedExtensions = map[string]bool{
		".f": true, ".for": true, ".ftn": true, ".f77": true, ".fpp": true,
		".F": true, ".FOR": true, ".FTN": true, ".F77": true, ".FPP": true,
	}
)

func source(p string) bool {
	extension := path.Ext(p)
	return freeExtensions[extension] || fixedExtensions[extension]
}

// Implements: REQ-FORTRAN-001
type Plugin struct{}

func (Plugin) Name() string { return "fortran" }
func (Plugin) Version() int { return 1 }

// Claims takes Fortran sources - a .f or .for file only when scan labeled it
// Fortran, since Forth uses .f too - and fpm.toml, except what fpm wrote into a
// build/ directory beside an fpm.toml.
//
// Implements: REQ-FORTRAN-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	if !source(f.Path) && path.Base(f.Path) != "fpm.toml" {
		return false
	}
	if f.Language != "" && f.Language != "Fortran" && f.Language != "TOML" {
		return false
	}
	return !generated(f)
}

// generated reports whether f lies in a build/ directory that has an fpm.toml
// beside it: fpm builds there and fetches dependencies into
// build/dependencies/. Other projects may keep sources in build/.
func generated(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	for i, s := range segments[:len(segments)-1] {
		if s == "build" && f.AbsolutePath != "" && strings.HasSuffix(filepath.ToSlash(f.AbsolutePath), f.Path) {
			base := f.AbsolutePath[:len(f.AbsolutePath)-len(f.Path)]
			if hasManifest(filepath.Join(base, filepath.FromSlash(strings.Join(segments[:i], "/")))) {
				return true
			}
		}
	}
	return false
}

// hasManifest reports whether an absolute directory has an fpm.toml.
var hasManifest = lang.MarkerMemo("fpm.toml")

// Class tells fpm.toml apart from other TOML files.
//
// Implements: REQ-FORTRAN-001
func (Plugin) Class(f *scan.File) string {
	if path.Base(f.Path) == "fpm.toml" {
		return classManifest
	}
	return ""
}

// Implements: REQ-FORTRAN-009
func (Plugin) Ecosystems() []lang.Ecosystem {
	return append([]lang.Ecosystem{
		{ID: ecosystemFpm, Name: "fpm packages"},
		{ID: ecosystemExternal, Name: "Fortran external modules"},
		{ID: ecosystemStd, Name: "Fortran intrinsic modules", Std: true},
	}, cpp.PackageEcosystems()...)
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Implements: REQ-FORTRAN-002, REQ-FORTRAN-003, REQ-FORTRAN-005
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	if path.Base(f.Path) == "fpm.toml" {
		return extractManifest(source), nil
	}
	return extractSource(source, fixedExtensions[path.Ext(f.Path)]), nil
}
