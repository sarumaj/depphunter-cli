package r

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// A package (pkgs/shopr: DESCRIPTION with Remotes and a Bioconductor dependency,
// NAMESPACE, R/ files calling each other, R6, reference and S4 classes, a test and
// vignettes with a child document) beside a second package of the repository
// (pkgs/localhelper), an renv analysis project (analysis: renv.lock with
// Requirements, renv 1.1 Imports, GitHub, local and Bioconductor sources; source(),
// here(), box::use, pacman, a Quarto include, the renv library that is not claimed)
// and a packrat project (legacy).
//
// Verifies: REQ-R-001, REQ-R-002, REQ-R-003, REQ-R-004, REQ-R-005, REQ-R-006
// Verifies: REQ-R-007, REQ-R-008, REQ-R-009, REQ-R-010
func TestPackageAndAnalysisProject(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	tidyr := lang.Target{Ecosystem: ecoCRAN, Package: "tidyr", Version: "5c6f7e2d9a1b0c3d4e5f60718293a4b5c6d7e8f9", Pinned: true, Origin: "https://github.com/tidyverse/tidyr"}
	readr := lang.Target{Ecosystem: ecoCRAN, Package: "readr", Version: "2.1.5", Pinned: true}
	glue := lang.Target{Ecosystem: ecoCRAN, Package: "glue", Version: "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567", Pinned: true, Origin: "https://github.com/tidyverse/glue"}
	r6 := lang.Target{Ecosystem: ecoCRAN, Package: "R6", Version: "2.5.1", Pinned: true}
	dplyrDeclared := lang.Target{Ecosystem: ecoCRAN, Package: "dplyr", Version: ">= 1.1.0"}
	imports := map[string]map[string]lang.Target{
		"analysis/.Rprofile": {
			"source(\"renv/activate.R\")": {Local: "analysis/renv/activate.R"},
		},
		"analysis/R/clean.R":         {"tidyr::": tidyr},
		"analysis/R/load.R":          {"readr::": readr},
		"analysis/_setup.qmd":        {"library(glue)": {Ecosystem: ecoCRAN, Package: "glue", Version: "1.7.0", Pinned: true}},
		"analysis/modules/helpers.R": {},
		"analysis/renv/activate.R":   {},
		"analysis/report.qmd": {
			"child: _setup.qmd":    {Local: "analysis/_setup.qmd"},
			"source(\"R/load.R\")": {Local: "analysis/R/load.R"},
			"load_data()":          {Local: "analysis/R/load.R"},
		},
		"analysis/run.R": {
			"library(dplyr)":                           {Ecosystem: ecoCRAN, Package: "dplyr", Version: "1.1.4", Pinned: true},
			"require(readr)":                           readr,
			"requireNamespace(\"jsonlite\")":           {Ecosystem: ecoCRAN, Package: "jsonlite", Unresolved: true},
			"pacman::":                                 {Ecosystem: ecoCRAN, Package: "pacman", Version: "0.5.1", Pinned: true},
			"p_load(tidyr)":                            tidyr,
			"p_load(DESeq2)":                           {Ecosystem: ecoBioc, Package: "DESeq2", Version: "1.44.0", Pinned: true},
			"p_load(mylocal)":                          {Ecosystem: ecoCRAN, Package: "mylocal", Version: "0.1.0", Origin: "path:~/src/mylocal"},
			"library(stats)":                           {Ecosystem: ecoStd, Package: "stats"},
			"library(MASS)":                            {Ecosystem: ecoCRAN, Package: "MASS", Version: "7.3-60.2", Pinned: true},
			"library(survival)":                        {Ecosystem: ecoStd, Package: "survival"},
			"loadNamespace(\"S4Vectors\")":             {Ecosystem: ecoBioc, Package: "S4Vectors", Version: "0.42.1", Pinned: true},
			"source(\"R/load.R\")":                     {Local: "analysis/R/load.R"},
			"source(here(\"R/clean.R\"))":              {Local: "analysis/R/clean.R"},
			"here::":                                   {Ecosystem: ecoCRAN, Package: "here", Version: "1.0.1", Pinned: true},
			"sys.source(\"R/missing.R\")":              {},
			"source(\"https://example.org/remote.R\")": {},
			"box::":                       {Ecosystem: ecoCRAN, Package: "box", Version: "1.2.0", Pinned: true},
			"box::use(stringr)":           {Ecosystem: ecoCRAN, Package: "stringr", Version: "1.5.1", Pinned: true},
			"box::use(./modules/helpers)": {Local: "analysis/modules/helpers.R"},
			"box::use(modules/helpers)":   {Local: "analysis/modules/helpers.R"},
			"load_data()":                 {Local: "analysis/R/load.R"},
			"clean()":                     {Local: "analysis/R/clean.R"},
			"summarise_all()":             {},
		},
		"legacy/script.r": {
			"library(jsonlite)": {Ecosystem: ecoCRAN, Package: "jsonlite", Version: "1.8.8", Pinned: true},
			"library(httr)":     {Ecosystem: ecoCRAN, Package: "httr", Version: "9f8e7d6c5b4a39281706f5e4d3c2b1a098765432", Pinned: true, Origin: "https://github.com/r-lib/httr"},
		},
		"pkgs/localhelper/DESCRIPTION": {"Imports: shopr": {Local: "pkgs/shopr/DESCRIPTION"}},
		"pkgs/localhelper/R/helper.R":  {},
		"pkgs/shopr/DESCRIPTION": {
			"Depends: methods":              {Ecosystem: ecoStd, Package: "methods"},
			"Imports: dplyr (>= 1.1.0)":     dplyrDeclared,
			"Imports: rlang":                {Ecosystem: ecoCRAN, Package: "rlang", Floating: true},
			"Imports: R6 (== 2.5.1)":        r6,
			"Imports: MASS":                 {Ecosystem: ecoCRAN, Package: "MASS", Floating: true},
			"Imports: localhelper":          {Local: "pkgs/localhelper/DESCRIPTION"},
			"Imports: BiocGenerics":         {Ecosystem: ecoBioc, Package: "BiocGenerics", Floating: true},
			"Imports: glue":                 glue,
			"Suggests: testthat (>= 3.0.0)": {Ecosystem: ecoCRAN, Package: "testthat", Version: ">= 3.0.0"},
			"Suggests: knitr":               {Ecosystem: ecoCRAN, Package: "knitr", Floating: true},
			"LinkingTo: Rcpp":               {Ecosystem: ecoCRAN, Package: "Rcpp", Floating: true},
		},
		"pkgs/shopr/NAMESPACE": {
			"import(rlang)":       {Ecosystem: ecoCRAN, Package: "rlang", Floating: true},
			"importFrom(dplyr)":   dplyrDeclared,
			"importFrom(methods)": {Ecosystem: ecoStd, Package: "methods"},
			"importFrom(utils)":   {Ecosystem: ecoStd, Package: "utils"},
		},
		"pkgs/shopr/R/cart.R": {
			"@include utils.R": {Local: "pkgs/shopr/R/utils.R"},
			"@importFrom glue": glue,
			"@import R6":       r6,
			"R6::":             r6,
			"dplyr::":          dplyrDeclared,
			"shopr:::":         {},
			"price_of()":       {Local: "pkgs/shopr/R/utils.R"},
			"round_price()":    {Local: "pkgs/shopr/R/utils.R"},
		},
		"pkgs/shopr/R/utils.R": {
			"methods::":      {Ecosystem: ecoStd, Package: "methods"},
			"stats::":        {Ecosystem: ecoStd, Package: "stats"},
			"localhelper::":  {Local: "pkgs/localhelper/R"},
			"MASS::":         {Ecosystem: ecoCRAN, Package: "MASS", Floating: true},
			"survival::":     {Ecosystem: ecoStd, Package: "survival"},
			"helper_price()": {},
		},
		"pkgs/shopr/tests/testthat/test-cart.R": {
			"library(testthat)": {Ecosystem: ecoCRAN, Package: "testthat", Version: ">= 3.0.0"},
			"library(shopr)":    {Local: "pkgs/shopr/R"},
			"shopr:::":          {Local: "pkgs/shopr/R"},
			"test_that()":       {},
			"new_cart()":        {Local: "pkgs/shopr/R/cart.R"},
			"expect_equal()":    {},
			"total()":           {Local: "pkgs/shopr/R/cart.R"},
		},
		"pkgs/shopr/vignettes/details.Rmd": {
			"BiocGenerics::": {Ecosystem: ecoBioc, Package: "BiocGenerics", Floating: true},
		},
		"pkgs/shopr/vignettes/intro.Rmd": {
			"child: details.Rmd": {Local: "pkgs/shopr/vignettes/details.Rmd"},
			"library(shopr)":     {Local: "pkgs/shopr/R"},
			"library(knitr)":     {Ecosystem: ecoCRAN, Package: "knitr", Floating: true},
			"library(ggplot2)":   {Ecosystem: ecoCRAN, Package: "ggplot2", Unresolved: true},
			"ggplot()":           {},
			"total()":            {Local: "pkgs/shopr/R/cart.R"},
		},
	}
	if len(res) != len(imports) {
		var got []string
		for f := range res {
			got = append(got, f)
		}
		t.Errorf("analyzed %d files, want %d: %v", len(res), len(imports), got)
	}
	for file, want := range imports {
		langtest.CheckImports(t, res[file], want)
	}
	symbols := map[string]map[string]string{
		"analysis/R/clean.R":          {"clean": "function"},
		"analysis/R/load.R":           {"load_data": "function"},
		"analysis/modules/helpers.R":  {"greet": "function"},
		"analysis/report.qmd":         {"d": "var"},
		"analysis/run.R":              {"pkg": "var", "data": "var", "report": "function"},
		"pkgs/localhelper/R/helper.R": {"helper": "function"},
		"pkgs/shopr/R/cart.R": {
			"Cart": "class", "Cart.initialize": "method", "Cart.add": "method", "Cart.secret": "method",
			"total": "function", "%+%": "function", "discount": "function", "tax_rate": "var", "new_cart": "function",
		},
		"pkgs/shopr/R/utils.R": {
			"round_price": "function", "price_of": "function", "Receipt": "class", "describe": "generic",
			"Receipt.describe": "method", "Receipt.show": "method", "Account": "class", "Account.deposit": "method",
			"Account.withdraw": "method", "make_receipt": "function", "stats_summary": "function",
		},
		"pkgs/shopr/vignettes/intro.Rmd": {"plot_total": "function"},
		"pkgs/shopr/DESCRIPTION":         {},
	}
	for file, want := range symbols {
		langtest.CheckSymbols(t, res[file], want)
	}
	// Lines point into the document, not into its chunk.
	for _, s := range res["pkgs/shopr/vignettes/intro.Rmd"].Symbols {
		if s.Name == "plot_total" && s.Line != 25 {
			t.Errorf("plot_total at line %d, want 25", s.Line)
		}
	}
}

// renv.lock's Requirements (and renv 1.1's Imports) and packrat.lock's Requires answer
// --resolve-depth, each dependency as the same lock pinned it; base packages are R's.
//
// Verifies: REQ-R-008
func TestLockDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	for _, tt := range []struct {
		pkg  lang.Target
		want []lang.Target
	}{
		{lang.Target{Ecosystem: ecoCRAN, Package: "dplyr", Version: "1.1.4"}, []lang.Target{
			{Ecosystem: ecoCRAN, Package: "R6", Version: "2.5.1", Pinned: true},
			{Ecosystem: ecoCRAN, Package: "cli", Version: "3.6.3", Pinned: true},
			{Ecosystem: ecoCRAN, Package: "generics"},
			{Ecosystem: ecoCRAN, Package: "glue", Version: "1.7.0", Pinned: true},
			{Ecosystem: ecoCRAN, Package: "lifecycle"},
			{Ecosystem: ecoCRAN, Package: "magrittr"},
			{Ecosystem: ecoCRAN, Package: "rlang", Version: "1.1.4", Pinned: true},
			{Ecosystem: ecoCRAN, Package: "tibble"},
		}},
		{lang.Target{Ecosystem: ecoCRAN, Package: "readr", Version: "2.1.5"}, []lang.Target{
			{Ecosystem: ecoCRAN, Package: "MASS", Version: "7.3-60.2", Pinned: true},
			{Ecosystem: ecoCRAN, Package: "cli", Version: "3.6.3", Pinned: true},
			{Ecosystem: ecoCRAN, Package: "tibble"},
		}},
		{lang.Target{Ecosystem: ecoBioc, Package: "DESeq2", Version: "1.44.0"}, []lang.Target{
			{Ecosystem: ecoBioc, Package: "S4Vectors", Version: "0.42.1", Pinned: true},
			{Ecosystem: ecoCRAN, Package: "ggplot2"},
		}},
		{lang.Target{Ecosystem: ecoCRAN, Package: "httr", Version: "9f8e7d6c5b4a39281706f5e4d3c2b1a098765432"}, []lang.Target{
			{Ecosystem: ecoCRAN, Package: "curl"},
			{Ecosystem: ecoCRAN, Package: "jsonlite", Version: "1.8.8", Pinned: true},
			{Ecosystem: ecoCRAN, Package: "mime"},
		}},
		// Another version than the lock's is not answered for.
		{lang.Target{Ecosystem: ecoCRAN, Package: "dplyr", Version: "1.0.0"}, nil},
	} {
		if got := r.Dependencies(tt.pkg); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s %s: got %+v, want %+v", tt.pkg.Package, tt.pkg.Version, got, tt.want)
		}
	}
}

// What only looks like code - in a comment, a string, a raw string - is not read,
// and a definition's name may be backquoted or a string.
//
// Verifies: REQ-R-002, REQ-R-003
func TestLexerHidesNonCode(t *testing.T) {
	src := "# library(incomment)\n" +
		"x <- \"library(instring)\"\n" +
		"y <- r\"-(library(inraw) )\" )-\"\n" +
		"`my fun` <- function() NULL\n" +
		"\"quoted\" = function() NULL\n" +
		"z <- 1e-3L; w <- 0x1F\n" +
		"f <- function(x) {\n  inner <- function() 1\n}\n" +
		"x %>%\n  g <- function() 2\n" +
		"library(real)\n"
	ex, err := Plugin{}.Extract(&scan.File{Path: "a.R"}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var specs []string
	for _, im := range ex.Imports {
		specs = append(specs, im.Spec)
	}
	if want := []string{"library(real)"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
	var names []string
	for _, s := range ex.Symbols {
		names = append(names, s.Name+" "+s.Kind)
	}
	if want := []string{"x var", "y var", "my fun function", "quoted function", "z var", "w var", "f function"}; !reflect.DeepEqual(names, want) {
		t.Errorf("symbols %q, want %q", names, want)
	}
}

// Installed libraries (renv/library, packrat/lib) are not the project's code;
// DESCRIPTION and NAMESPACE are told apart by name.
//
// Verifies: REQ-R-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"R/cart.R": true, "script.r": true, "doc.Rmd": true, "report.qmd": true, ".Rprofile": true,
		"DESCRIPTION": true, "pkgs/a/NAMESPACE": true, "README.md": false, "notes.txt": false,
		"renv/library/R-4.4/x86_64/dplyr/R/zzz.R": false, "packrat/lib/x86_64/4.2/httr/R/a.R": false,
		"renv/activate.R": true,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: claimed %v, want %v", p, got, want)
		}
	}
	if (Plugin{}).Class(&scan.File{Path: "x/DESCRIPTION"}) == (Plugin{}).Class(&scan.File{Path: "x/NAMESPACE"}) {
		t.Error("DESCRIPTION and NAMESPACE share a cache class")
	}
}
