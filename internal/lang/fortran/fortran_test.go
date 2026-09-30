package fortran

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture testdata/repo is an fpm package "shop" whose fpm.toml declares a
// metapackage (stdlib), OpenMP and MPI metapackages, git dependencies at a tag
// (json-fortran), a rev (toml-f) and a branch on a server of its own (fancy), a
// path dependency inside the repository (libs/widgets) and one outside it, a
// registry package (plotter), a dev-dependency, external modules and a linked
// library. build/ is what fpm wrote: cache.toml and two fetched dependencies.
// src/ holds free-form modules and submodules, legacy/ fixed-form code, a free-
// form .f file and a Forth file. testdata/cmake is a CMake project without fpm.

var (
	stdlibMeta = lang.Target{Ecosystem: ecosystemFpm, Package: "stdlib", Version: "*", Floating: true}
	jsonF      = lang.Target{Ecosystem: ecosystemFpm, Package: "json-fortran", Version: "89abcdef0123456789abcdef0123456789abcdef", Requested: "8.3.0"}
	tomlF      = lang.Target{Ecosystem: ecosystemFpm, Package: "toml-f", Version: "d7b892b1d074b7cfc5d75c3e0eb36ebc1f7958c1", Pinned: true}
	fancy      = lang.Target{Ecosystem: ecosystemFpm, Package: "fancy", Version: "0123456789abcdef0123456789abcdef01234567", Requested: "main", Floating: true, Origin: "https://git.acme.corp/fortran/fancy.git"}
	plotter    = lang.Target{Ecosystem: ecosystemFpm, Package: "plotter", Version: "1.2.0", Pinned: true}
	testDrive  = lang.Target{Ecosystem: ecosystemFpm, Package: "test-drive", Floating: true}
	mpi        = lang.Target{Ecosystem: "c-external", Package: "mpi", Unresolved: true}
	netcdf     = lang.Target{Ecosystem: "c-external", Package: "netcdf", Unresolved: true}
)

func std(name string) lang.Target { return lang.Target{Ecosystem: ecosystemStd, Package: name} }

// Verifies: REQ-FORTRAN-002, REQ-FORTRAN-004, REQ-FORTRAN-006, REQ-FORTRAN-007, REQ-FORTRAN-008, REQ-FORTRAN-011
func TestImports(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["src/shop.f90"], map[string]lang.Target{
		"use, intrinsic :: iso_fortran_env": std("iso_fortran_env"),
		"use iso_c_binding":                 std("iso_c_binding"),
		"use shop_cart":                     {Local: "src/cart.F90"}, // MODULE Shop_Cart: names are case-insensitive
		"use stdlib_kinds":                  stdlibMeta,              // spelled by the declared metapackage
		"use json_module":                   jsonF,                   // defined by what fpm fetched into build/dependencies
		"use tomlf":                         tomlF,                   // spelled by the declared name
		"use fancy_core":                    fancy,                   // defined by what fpm fetched
		"use mpi_f08":                       mpi,                     // the C library the cpp plugin names
		"use netcdf":                        netcdf,
		"use vendor_mod":                    {Ecosystem: ecosystemExternal, Package: "vendor_mod"}, // [build] external-modules
		"use plotter_axes":                  plotter,
		"use widgets":                       {Local: "libs/widgets/src/widgets.f90"},
		"use nothere_mod":                   {Ecosystem: ecosystemExternal, Package: "nothere_mod", Unresolved: true},
		"use shop_missing":                  {}, // the package's own module, missing
		"use, non_intrinsic :: iso_varying_string": {Ecosystem: ecosystemExternal, Package: "iso_varying_string", Unresolved: true},
		"use omp_lib":           std("openmp"), // !$ OpenMP conditional compilation
		"include 'shop.inc'":    {Local: "src/shop.inc"},
		"include 'common.inc'":  {Local: "include/common.inc"}, // [library] include-dir
		"include 'mpif.h'":      mpi,
		"include 'nowhere.inc'": {},
	})
	langtest.CheckImports(t, results["src/cart.F90"], map[string]lang.Target{
		`#include "config.h"`:   {Local: "src/config.h"},
		"#include <petscsys.h>": {Ecosystem: "c-external", Package: "petscsys", Unresolved: true},
		"use mpi":               mpi, // both branches of #ifdef are read
	})
	langtest.CheckImports(t, results["src/cart_impl.f90"], map[string]lang.Target{
		"submodule (shop_cart)": {Local: "src/cart.F90"},
	})
	langtest.CheckImports(t, results["src/cart_more.f90"], map[string]lang.Target{
		"submodule (shop_cart:cart_impl)": {Local: "src/cart_impl.f90"}, // the parent submodule
		"use testdrive":                   testDrive,
	})
	langtest.CheckImports(t, results["test/check.f90"], map[string]lang.Target{
		"use testdrive": testDrive, // a dev-dependency
		"use shop_cart": {Local: "src/cart.F90"},
	})
	langtest.CheckImports(t, results["app/main.f90"], map[string]lang.Target{
		"use shop": {Local: "src/shop.f90"},
	})
	langtest.CheckImports(t, results["legacy/dgemm.f"], map[string]lang.Target{
		"include 'params.inc'": {Local: "legacy/params.inc"},
		"use LEGACY_UTIL":      {}, // defined in the same file
		"use SHOP_CART":        {Local: "src/cart.F90"},
	})
	langtest.CheckImports(t, results["legacy/modern.f"], map[string]lang.Target{
		"use shop_cart": {Local: "src/cart.F90"},
	})
}

// A CMake project without fpm: modules still resolve across the project's
// files; what none defines goes to the curated table, a C library or an
// unresolved module.
//
// Verifies: REQ-FORTRAN-004, REQ-FORTRAN-007
func TestWithoutFpm(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/cmake")
	langtest.CheckImports(t, results["src/physics.f90"], map[string]lang.Target{
		"use constants":    {Local: "src/constants.f90"},
		"use stdlib_math":  {Ecosystem: ecosystemFpm, Package: "stdlib", Unresolved: true},
		"use mpi":          mpi,
		"use hdf5":         {Ecosystem: "c-external", Package: "hdf5", Unresolved: true},
		"use omp_lib":      std("openmp"),
		"use unknown_mod":  {Ecosystem: ecosystemExternal, Package: "unknown_mod", Unresolved: true},
		"include 'mpif.h'": mpi,
	})
}

// Verifies: REQ-FORTRAN-005
func TestManifest(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["fpm.toml"], map[string]lang.Target{
		"stdlib":                        stdlibMeta,
		"openmp":                        std("openmp"),
		"mpi":                           mpi,
		"json-fortran":                  jsonF, // a tag: neither pinned nor floating; cache.toml's rev shown
		"toml-f":                        tomlF, // a rev pins
		"fancy":                         fancy, // a branch floats; a server of its own is the origin
		"widgets":                       {Local: "libs/widgets/fpm.toml"},
		"gone":                          {}, // a path outside the repository
		"plotter":                       plotter,
		"test-drive":                    testDrive,
		"library: src":                  {Local: "src"},
		"executable shop: app/main.f90": {Local: "app/main.f90"},
		"test unit: test/check.f90":     {Local: "test/check.f90"},
		"external-modules: netcdf":      netcdf,
		"external-modules: vendor_mod":  {Ecosystem: ecosystemExternal, Package: "vendor_mod"},
		"link: lapack":                  {Ecosystem: "c-external", Package: "lapack", Unresolved: true},
	})
	langtest.CheckSymbols(t, results["fpm.toml"], map[string]string{"shop": "package"})
	lines := map[string]int{}
	for _, imported := range results["fpm.toml"].Imports {
		lines[imported.Spec] = imported.Line
	}
	want := map[string]int{"stdlib": 13, "toml-f": 17, "fancy": 19, "test-drive": 25, "executable shop: app/main.f90": 27, "external-modules: netcdf": 9}
	for spec, l := range want {
		if lines[spec] != l {
			t.Errorf("%s: line %d, want %d", spec, lines[spec], l)
		}
	}
}

// Verifies: REQ-FORTRAN-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, results["src/shop.f90"], map[string]string{
		"shop":            "module",
		"order_t":         "type",
		"total":           "interface",
		"shop.total_int":  "function", // pure elemental integer function
		"shop.total_real": "function", // real(dp) function
		"shop.walk":       "function", // a recursive subroutine
		"shop.walk.inner": "function",
		"shop.new_order":  "function", // type(order_t) function
	})
	langtest.CheckSymbols(t, results["src/cart.F90"], map[string]string{
		"Shop_Cart":             "module",
		"cart_t":                "type",
		"operator(+)":           "interface",
		"Shop_Cart.add":         "function",
		"Shop_Cart.merge_carts": "function",
	})
	langtest.CheckSymbols(t, results["src/cart_impl.f90"], map[string]string{
		"cart_impl":          "submodule",
		"cart_impl.checkout": "function", // module procedure checkout
	})
	langtest.CheckSymbols(t, results["src/cart_more.f90"], map[string]string{
		"cart_more": "submodule", "cart_more.audit": "function",
	})
	langtest.CheckSymbols(t, results["app/main.f90"], map[string]string{
		"shop_app": "program", "shop_app.run": "function",
	})
	langtest.CheckSymbols(t, results["legacy/dgemm.f"], map[string]string{
		"DGEMM": "function", "DDOT": "function", "LABEL": "function",
		"SHOPINIT": "block data", "LEGACY_UTIL": "module", "LEGACY": "program",
	})
	langtest.CheckSymbols(t, results["legacy/tabbed.for"], map[string]string{"TABBED": "function"})
	langtest.CheckSymbols(t, results["legacy/modern.f"], map[string]string{"modern_f": "module"})
}

// Forth's .f files and what fpm wrote into build/ are not read.
//
// Verifies: REQ-FORTRAN-001
func TestClaims(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	for _, p := range []string{"legacy/words.f", "build/cache.toml", "build/dependencies/fancy/fpm.toml",
		"build/dependencies/fancy/src/fancy_core.f90", "src/shop.inc", "src/config.h"} {
		if _, ok := results[p]; ok {
			t.Errorf("%s claimed", p)
		}
	}
	for _, p := range []string{"legacy/dgemm.f", "legacy/tabbed.for", "src/cart.F90", "fpm.toml", "libs/widgets/fpm.toml"} {
		if _, ok := results[p]; !ok {
			t.Errorf("%s not claimed", p)
		}
	}
	for _, f := range []*scan.File{{Path: "gleam.toml"}, {Path: "x.fs"}, {Path: "a.f", Language: "Forth"}, {Path: "a.f90", Binary: true}} {
		if (Plugin{}).Claims(f) {
			t.Errorf("%s claimed", f.Path)
		}
	}
	if (Plugin{}).Class(&scan.File{Path: "fpm.toml"}) == (Plugin{}).Class(&scan.File{Path: "x.toml"}) {
		t.Error("fpm.toml not told apart from other TOML files")
	}
}

// What fpm fetched into build/dependencies/ says what it depends on, versioned
// by the project's cache.toml where it fetched them too.
//
// Verifies: REQ-FORTRAN-008
func TestTransitive(t *testing.T) {
	files := langtest.Files(t, "testdata/repo")
	root, _ := filepath.Abs("testdata/repo")
	r, err := Plugin{}.Resolver(root, files)
	if err != nil {
		t.Fatal(err)
	}
	transitive := r.(lang.Transitive)
	got := transitive.Dependencies(fancy)
	want := []lang.Target{
		jsonF,
		{Ecosystem: ecosystemFpm, Package: "leftpad", Version: "fedcba9876543210fedcba9876543210fedcba98"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fancy depends on %+v, want %+v", got, want)
	}
	in := r.(lang.Installed)
	if !in.Installed(fancy) || in.Installed(plotter) {
		t.Error("installed dependencies not told apart")
	}
	if transitive.Dependencies(plotter) != nil {
		t.Error("a dependency fpm did not fetch has dependencies")
	}
}

// Verifies: REQ-FORTRAN-002
func TestSourceForms(t *testing.T) {
	cases := []struct {
		source string
		fixed  bool
		want   []string
	}{
		// free form: & continuation (a comment between), ; separated statements
		{"use a, &\n  ! note\n  & only: x; use b\n", false, []string{"use a", "use b"}},
		// a string continued across lines hides what it holds
		{"s = 'x &\n  &use c'\nuse d\n", false, []string{"use d"}},
		// fixed form: column 6 continuation, C/*/! comments, labels
		{"C     USE X\n*     USE Y\n      USE\n     &  Z\n  100 CONTINUE\n", true, []string{"use Z"}},
		// a doubled quote does not end a string
		{"      S = 'IT''S USE W'\n      USE V\n", true, []string{"use V"}},
		// OpenMP conditional compilation, but not a directive
		{"!$ use omp_lib\n!$omp parallel\n", false, []string{"use omp_lib"}},
		{"!$    USE OMP_LIB\nC$OMP PARALLEL\n", true, []string{"use OMP_LIB"}},
		// use statement forms
		{"use :: m1\nuse, intrinsic :: ieee_arithmetic\nuse = 3\nused = 4\n", false, []string{"use m1", "use, intrinsic :: ieee_arithmetic"}},
		// preprocessor lines, fypp's include
		{"#include \"a.h\"\n#:include \"common.fypp\"\n#define X \\\n  use nope\n", false, []string{`#include "a.h"`, `#:include "common.fypp"`}},
	}
	for _, c := range cases {
		extraction := extractSource([]byte(c.source), c.fixed)
		var got []string
		for _, rawImport := range extraction.Imports {
			got = append(got, rawImport.Spec)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: imports %q, want %q", c.source, got, c.want)
		}
	}
}

// Every prefix of every fixture file extracts without a panic, and runs of
// what opens scopes, strings and continuations stay linear.
//
// Verifies: REQ-FORTRAN-010
func TestTruncated(t *testing.T) {
	var files []string
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	for _, p := range files {
		source, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f := &scan.File{Path: filepath.ToSlash(p)}
		for i := 0; i <= len(source); i++ {
			if _, err := (Plugin{}).Extract(f, source[:i]); err != nil {
				t.Fatal(err)
			}
			extractSource(source[:i], true)
			extractSource(source[:i], false)
			readManifest(source[:i])
			readCache(source[:i])
		}
	}
	for _, unit := range []string{"module m\n", "submodule (a:b) c\n", "function f()\n", "end\n", "end function\n",
		"type t\n", "interface\n", "interface x\n", "'", "\"", "&\n", "'&\n", "!", ";", "#include \"x\"\n", "#define x \\\n",
		"(", "use ", "use a, only: ", "include '", "     &", "\t1", "C\n", "type, extends(", "real function ", "é",
		"module procedure x\n", "& ", "&", "&!", "   ", "pure elemental real(dp) function f(x)\n", "!$ use omp_lib\n", "${x}$", "x = [" + "\n"} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		// the same run inside a string that starts before it
		for _, s := range [][]byte{source, append([]byte("'"), source...)} {
			start := time.Now()
			extractSource(s, true)
			extractSource(s, false)
			if d := time.Since(start); d > langtest.TimeLimit(5*time.Second) {
				t.Errorf("%q x %d: %v", unit, len(s)/len(unit), d)
			}
		}
	}
}

// Verifies: REQ-FORTRAN-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids[ecosystemStd]; !ok || !std || ids[ecosystemFpm] || ids[ecosystemExternal] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for _, root := range []string{"testdata/repo", "testdata/cmake"} {
		for f, r := range langtest.Analyze(t, Plugin{}, root) {
			for _, imported := range r.Imports {
				if e := imported.Target.Ecosystem; e != "" {
					if _, ok := ids[e]; !ok {
						t.Errorf("%s: %s -> %s, not declared", f, imported.Spec, e)
					}
				}
			}
		}
	}
}

// A system include of a path above the repository is not the project's, a bare
// ".." no more than "../x.h", even with the layout listing both: the check does
// not rely on the layout never holding a path above the root.
//
// Verifies: REQ-LANG-031
func TestIncludeAboveTheRepository(t *testing.T) {
	root := langtest.Write(t, map[string]string{"main.f90": "program main\nend program main\n"})
	r := newResolver(root, langtest.Files(t, root))
	r.Files[".."], r.Files["../x.h"] = true, true
	for _, spec := range []string{"..", "../x.h"} {
		if got := r.Resolve("main.f90", lang.RawImport{Module: spec, Name: kindCppSys}); got.Local != "" {
			t.Errorf("%s: got %+v, want no project file", spec, got)
		}
	}
}
