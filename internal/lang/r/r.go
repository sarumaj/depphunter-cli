// Package r analyzes R - scripts, packages, R Markdown and Quarto documents - and
// its package repositories, CRAN and Bioconductor. library(), require(),
// requireNamespace(), loadNamespace(), pacman::p_load(), box::use(), pkg::fun and
// roxygen @import tags resolve to the importing package's own R/ directory, to the
// R/ directory of another package of the repository, to the hidden r-std island (R's
// base packages, and the recommended ones a project neither declares nor locks),
// and to CRAN or Bioconductor packages by what renv.lock or packrat.lock pinned and
// DESCRIPTION declares (resolve.go). source(), sys.source(), box::use(./module),
// targets::tar_source() and knitr child documents resolve to project files. Inside
// a package, where files have no imports of each other, a call resolves to the file
// that defines the function. DESCRIPTION's dependency fields and NAMESPACE's imports
// are imports of what they name.
//
// R is read by a lexer of its own (lex.go), not the vendored tree-sitter grammar:
// everything needed is token-level, and the lexer is several times faster
// (REQ-R-011).
package r

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoCRAN = "cran"
	ecoBioc = "bioconductor"
	ecoStd  = "r-std"
)

// Implements: REQ-R-001
type Plugin struct{}

func (Plugin) Name() string { return "r" }
func (Plugin) Version() int { return 1 }

// Claims takes R sources (.R, .r, .Rprofile), R Markdown and Quarto documents (.Rmd,
// .qmd) and the DESCRIPTION and NAMESPACE files of packages, except the libraries
// renv and packrat install into the project.
//
// Implements: REQ-R-001
func (Plugin) Claims(f *scan.File) bool {
	if f.Binary {
		return false
	}
	segments := strings.Split(f.Path, "/")
	for i := 0; i+1 < len(segments)-1; i++ {
		switch segments[i] + "/" + segments[i+1] {
		case "renv/library", "renv/staging", "renv/local", "renv/sandbox", "renv/python",
			"packrat/lib", "packrat/lib-R", "packrat/lib-ext", "packrat/src":
			return false
		}
	}
	base := path.Base(f.Path)
	switch strings.ToLower(path.Ext(base)) {
	case ".r", ".rmd", ".qmd", ".rprofile":
		return true
	}
	return base == "DESCRIPTION" || base == "NAMESPACE"
}

// Class tells DESCRIPTION and NAMESPACE apart, which have no extension.
//
// Implements: REQ-R-001
func (Plugin) Class(f *scan.File) string {
	switch base := path.Base(f.Path); base {
	case "DESCRIPTION", "NAMESPACE":
		return base
	}
	return ""
}

func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoCRAN, Name: "CRAN"},
		{ID: ecoBioc, Name: "Bioconductor"},
		{ID: ecoStd, Name: "R base packages", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Extract dispatches on the file: DESCRIPTION, NAMESPACE, a document with R chunks,
// or R source.
//
// Implements: REQ-R-002, REQ-R-003, REQ-R-004, REQ-R-005, REQ-R-011
func (p Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	switch p.Class(f) {
	case "DESCRIPTION":
		return extractDescription(src), nil
	case "NAMESPACE":
		return extractNamespace(src), nil
	}
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".rmd", ".qmd":
		return extractDocument(src), nil
	}
	tokens, comments := lex(src, 1)
	return extractCode(newCode(tokens), comments), nil
}
