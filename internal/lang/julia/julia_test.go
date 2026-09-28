package julia

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a package Shop (src/Shop.jl including a submodule file and a file
// under src/internal) with a Project.toml declaring registry packages, standard
// libraries (Statistics upgraded from the registry), a local package by UUID
// (lib/Utils, developed by the manifest's path), a package added by URL, a weak
// dependency with its extension and a test extra; a format 2.0 Manifest.toml; a test
// environment and a docs environment with [sources]; a tools environment with a plain
// and a versioned manifest; a script outside every environment; Artifacts.toml.

// uuids are the fixture's registry packages' UUIDs, which their targets carry for
// the registries to find them by.
var uuids = map[string]string{
	"Aqua":       "4c88cf16-eb10-579e-8560-4a9242c79595",
	"CairoPlot":  "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d",
	"DataFrames": "a93c6f00-e57d-5684-b7b6-d8193f3e46c0",
	"Documenter": "e30172f5-a6a5-5a46-863b-614d45cd2de4",
	"HTTP":       "cd3eb016-35fb-5094-929b-558a96fad6f3",
	"JSON":       "682c06a0-de6a-54ab-a142-c8b1cf79cde6",
	"Parsers":    "69de0a69-1ddd-5017-9359-2bf0b02dc9f0",
	"Plots":      "91a5bcdd-55d7-5caf-9e0b-520d859cae80",
	"Statistics": "10745b16-79ce-11e8-11f9-7d13ad32a3b2",
}

func packageName(ecosystem, name, version string, pinned bool) lang.Target {
	return lang.Target{Ecosystem: ecosystem, Package: name, Version: version, Pinned: pinned, Registry: uuids[name]}
}

// Verifies: REQ-JULIA-002, REQ-JULIA-004, REQ-JULIA-005, REQ-JULIA-008, REQ-JULIA-010
func TestSourceImports(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	json := lang.Target{Ecosystem: ecosystemJulia, Package: "JSON", Version: "0.21.4", Requested: "0.21", Pinned: true, Registry: uuids["JSON"]}
	langtest.CheckImports(t, results["src/Shop.jl"], map[string]lang.Target{
		"using JSON":          json,
		"using HTTP":          packageName(ecosystemJulia, "HTTP", "1.10.8", true),
		"using LinearAlgebra": {Ecosystem: ecosystemStd, Package: "LinearAlgebra"},
		"import Statistics":   packageName(ecosystemJulia, "Statistics", "1.11.1", true),
		"using Base.Threads":  {Ecosystem: ecosystemStd, Package: "Base"},
		"import Base":         {Ecosystem: ecosystemStd, Package: "Base"},
		`include("cart.jl")`:  {Local: "src/cart.jl"},
		`include(joinpath(@__DIR__, "internal", "pricing.jl"))`: {Local: "src/internal/pricing.jl"},
		"using .Pricing": {Local: "src/internal/pricing.jl"},
		"using .Cart":    {Local: "src/cart.jl"},
	})
	langtest.CheckImports(t, results["src/cart.jl"], map[string]lang.Target{
		"using ..Shop":    {Local: "src/Shop.jl"},
		"using ..Pricing": {Local: "src/internal/pricing.jl"},
	})
	langtest.CheckImports(t, results["src/internal/pricing.jl"], map[string]lang.Target{
		"using Utils":         {Local: "lib/Utils/src/Utils.jl"},
		"using Utils.Strings": {Local: "lib/Utils/src/sub/Strings.jl"},
		"import Parsers":      packageName(ecosystemJulia, "Parsers", "2.8.1", true),
	})
	langtest.CheckImports(t, results["ext/ShopPlotsExt.jl"], map[string]lang.Target{
		"using Shop":  {Local: "src/Shop.jl"},
		"using Plots": packageName(ecosystemJulia, "Plots", "1", false),
	})
	langtest.CheckImports(t, results["test/runtests.jl"], map[string]lang.Target{
		"using Test":                        {Ecosystem: ecosystemStd, Package: "Test"},
		"using Shop":                        {Local: "src/Shop.jl"},
		"using Aqua":                        packageName(ecosystemJulia, "Aqua", "0.8", false),
		"using Utils.Strings":               {Local: "lib/Utils/src/sub/Strings.jl"},
		"import Shop.Cart":                  {Local: "src/cart.jl"},
		"using Missing1":                    {Ecosystem: ecosystemJulia, Package: "Missing1", Unresolved: true, Floating: true},
		`includet("helpers.jl")`:            {Local: "test/helpers.jl"},
		`include("$(@__DIR__)/helpers.jl")`: {Local: "test/helpers.jl"},
		`include(joinpath(dirname(@__FILE__), "..", "scripts", "report.jl"))`: {Local: "scripts/report.jl"},
		"include(f)": {},
	})
	langtest.CheckImports(t, results["docs/make.jl"], map[string]lang.Target{
		"using Documenter": packageName(ecosystemJulia, "Documenter", "~1.2", false),
		"using Shop":       {Local: "src/Shop.jl"},
	})
	// Outside every environment: any project's declaration; Main.X is a module a
	// script defined at its top.
	langtest.CheckImports(t, results["scripts/report.jl"], map[string]lang.Target{
		"using DataFrames": packageName(ecosystemJulia, "DataFrames", "1.7.0", true),
		"using Main.Shop":  {Local: "src/Shop.jl"},
	})
	// A versioned manifest wins over the plain one beside it.
	langtest.CheckImports(t, results["tools/clean.jl"], map[string]lang.Target{
		"using DataFrames": packageName(ecosystemJulia, "DataFrames", "1.7.0", true),
	})
	langtest.CheckImports(t, results["lib/Utils/src/Utils.jl"], map[string]lang.Target{
		`include("sub/Strings.jl")`: {Local: "lib/Utils/src/sub/Strings.jl"},
	})
}

// Verifies: REQ-JULIA-001, REQ-JULIA-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, results["src/Shop.jl"], map[string]string{
		"Shop": "module", "VERSION_TAG": "const", "A": "const", "B": "const",
		"AbstractItem": "type", "Money": "type", "Item": "type", "Basket": "type",
		"Color": "enum", "audit": "macro", "checkout": "function",
		"Base.show": "function", "Base.+": "function", "discount": "function",
		"scale": "function", "fast": "function", "winonly": "function",
	})
	langtest.CheckSymbols(t, results["ext/ShopPlotsExt.jl"], map[string]string{"ShopPlotsExt": "module", "Shop.plot": "function"})
	langtest.CheckSymbols(t, results["lib/Utils/src/sub/Strings.jl"], map[string]string{"Strings": "module", "slug": "function"})
	langtest.CheckSymbols(t, results["Project.toml"], map[string]string{"Shop": "package", "ShopPlotsExt": "extension"})
	langtest.CheckSymbols(t, results["Artifacts.toml"], map[string]string{"libshop": "artifact", "fonts": "artifact"})
}

// Verifies: REQ-JULIA-005, REQ-JULIA-006, REQ-JULIA-007, REQ-JULIA-008
func TestManifests(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	cairo := lang.Target{Ecosystem: ecosystemJulia, Package: "CairoPlot", Version: "0.2.0", Pinned: true, Origin: "https://github.com/example/CairoPlot.jl", Registry: uuids["CairoPlot"]}
	langtest.CheckImports(t, results["Project.toml"], map[string]lang.Target{
		"deps.HTTP":               packageName(ecosystemJulia, "HTTP", "1.10.8", true),
		"deps.JSON":               {Ecosystem: ecosystemJulia, Package: "JSON", Version: "0.21.4", Requested: "0.21", Pinned: true, Registry: uuids["JSON"]},
		"deps.LinearAlgebra":      {Ecosystem: ecosystemStd, Package: "LinearAlgebra"},
		"deps.Statistics":         packageName(ecosystemJulia, "Statistics", "1.11.1", true),
		"deps.Utils":              {Local: "lib/Utils/src/Utils.jl"},
		"deps.CairoPlot":          cairo,
		"weakdeps.Plots":          packageName(ecosystemJulia, "Plots", "1", false),
		"extensions.ShopPlotsExt": {Local: "ext/ShopPlotsExt.jl"},
		"extras.Test":             {Ecosystem: ecosystemStd, Package: "Test"},
	})
	langtest.CheckImports(t, results["Manifest.toml"], map[string]lang.Target{
		"[[deps.CairoPlot]]":     cairo,
		"[[deps.Dates]]":         {Ecosystem: ecosystemStd, Package: "Dates"},
		"[[deps.HTTP]]":          packageName(ecosystemJulia, "HTTP", "1.10.8", true),
		"[[deps.JSON]]":          packageName(ecosystemJulia, "JSON", "0.21.4", true),
		"[[deps.LinearAlgebra]]": {Ecosystem: ecosystemStd, Package: "LinearAlgebra"},
		"[[deps.Mmap]]":          {Ecosystem: ecosystemStd, Package: "Mmap"},
		"[[deps.Parsers]]":       packageName(ecosystemJulia, "Parsers", "2.8.1", true),
		"[[deps.Statistics]]":    packageName(ecosystemJulia, "Statistics", "1.11.1", true),
		"[[deps.Utils]]":         {Local: "lib/Utils/src/Utils.jl"},
	})
	langtest.CheckImports(t, results["test/Project.toml"], map[string]lang.Target{
		"deps.Aqua": packageName(ecosystemJulia, "Aqua", "0.8", false),
		"deps.Shop": {Local: "src/Shop.jl"},
		"deps.Test": {Ecosystem: ecosystemStd, Package: "Test"},
	})
	langtest.CheckImports(t, results["docs/Project.toml"], map[string]lang.Target{
		"deps.Documenter": packageName(ecosystemJulia, "Documenter", "~1.2", false),
		"deps.Shop":       {Local: "src/Shop.jl"},
	})
	langtest.CheckImports(t, results["tools/Manifest.toml"], map[string]lang.Target{"[[deps.DataFrames]]": packageName(ecosystemJulia, "DataFrames", "1.6.0", true)})
	langtest.CheckImports(t, results["tools/Manifest-v1.11.toml"], map[string]lang.Target{"[[deps.DataFrames]]": packageName(ecosystemJulia, "DataFrames", "1.7.0", true)})
}

// A manifest entry's deps are the package's dependencies: standard libraries by
// name, the others pinned by the same manifest.
//
// Verifies: REQ-JULIA-009
func TestDependencies(t *testing.T) {
	files := langtest.Files(t, "testdata/repo")
	r := newResolver("testdata/repo", files)
	got := r.Dependencies(packageName(ecosystemJulia, "JSON", "0.21.4", true))
	want := []lang.Target{
		{Ecosystem: ecosystemStd, Package: "Dates"},
		{Ecosystem: ecosystemStd, Package: "Mmap"},
		packageName(ecosystemJulia, "Parsers", "2.8.1", true),
		{Ecosystem: ecosystemStd, Package: "Unicode"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON: got %+v, want %+v", got, want)
	}
	if got := r.Dependencies(packageName(ecosystemJulia, "JSON", "0.20.0", true)); got != nil {
		t.Errorf("another version: %+v", got)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: ecosystemStd, Package: "Dates"}); got != nil {
		t.Errorf("std: %+v", got)
	}
}

// A format 1.0 manifest (Julia before 1.7) keeps its entries at the top level;
// dependency lists written as a table (for names that are ambiguous) are read too.
//
// Verifies: REQ-JULIA-007
func TestManifestFormats(t *testing.T) {
	m := readManifest([]byte(`# This file is machine-generated - editing it directly is not advised

[[Base64]]
uuid = "2a0f44e3-6c83-55bd-87e4-b1978d98bd5f"

[[JSON]]
deps = ["Dates", "Mmap"]
git-tree-sha1 = "31e996f0a15c7b280ba9f76636b3ff9e2ae58c9a"
uuid = "682c06a0-de6a-54ab-a142-c8b1cf79cde6"
version = "0.21.4"

[[Foo]]
git-tree-sha1 = "1111111111111111111111111111111111111111"
uuid = "11111111-1111-1111-1111-111111111111"
version = "1.0.0"
    [Foo.deps]
    Bar = "22222222-2222-2222-2222-222222222222"
`))
	if e := m.find("JSON", ""); e == nil || e.version != "0.21.4" || !reflect.DeepEqual(e.dependencies, []string{"Dates", "Mmap"}) || e.line != 6 {
		t.Errorf("JSON: %+v", e)
	}
	if e := m.find("Foo", "11111111-1111-1111-1111-111111111111"); e == nil || !reflect.DeepEqual(e.dependencies, []string{"Bar"}) {
		t.Errorf("Foo: %+v", e)
	}
	if e := m.find("Base64", ""); e == nil || e.tree != "" {
		t.Errorf("Base64: %+v", e)
	}
	if e := m.find("JSON", "00000000-0000-0000-0000-000000000000"); e != nil {
		t.Errorf("another UUID matched: %+v", e)
	}
}

// Verifies: REQ-JULIA-008
func TestCompatPins(t *testing.T) {
	for compat, want := range map[string]lang.Target{
		"=1.2.3":       {Ecosystem: ecosystemJulia, Package: "X", Version: "1.2.3", Pinned: true},
		"1.2.3":        {Ecosystem: ecosystemJulia, Package: "X", Version: "1.2.3"},
		"=1.2":         {Ecosystem: ecosystemJulia, Package: "X", Version: "=1.2"},
		"~1.2, 2":      {Ecosystem: ecosystemJulia, Package: "X", Version: "~1.2, 2"},
		"1.2 - 1.5":    {Ecosystem: ecosystemJulia, Package: "X", Version: "1.2 - 1.5"},
		">= 1":         {Ecosystem: ecosystemJulia, Package: "X", Version: ">= 1"},
		"=1.2.3, =1.3": {Ecosystem: ecosystemJulia, Package: "X", Version: "=1.2.3, =1.3"},
		"":             {Ecosystem: ecosystemJulia, Package: "X", Floating: true},
	} {
		if got := compatTarget("X", "", compat); got != want {
			t.Errorf("%q: got %+v, want %+v", compat, got, want)
		}
	}
}

// Verifies: REQ-JULIA-001
func TestClaims(t *testing.T) {
	for p, class := range map[string]string{
		"src/Shop.jl": classSource, "Project.toml": classProject, "JuliaProject.toml": classProject,
		"Manifest.toml": classManifest, "JuliaManifest.toml": classManifest, "Manifest-v1.11.toml": classManifest,
		"JuliaManifest-v1.12.toml": classManifest, "Artifacts.toml": classArtifacts, "JuliaArtifacts.toml": classArtifacts,
	} {
		if !(Plugin{}).Claims(&scan.File{Path: p}) || fileClass(p) != class {
			t.Errorf("%s: not claimed as %s", p, class)
		}
	}
	for _, p := range []string{"Cargo.toml", "pyproject.toml", "Manifest-vx.toml", "Manifest.toml.bak", "project.toml"} {
		if (Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s claimed", p)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "a.jl", Binary: true}) {
		t.Error("binary claimed")
	}
}

// The lexer's traps: a string holding code, nested and line comments, character
// literals against the adjoint, symbols that are keywords (:end, :module), `end`
// and `begin` as indices, comprehensions' for/if, string macros, interpolation
// holding strings and parentheses.
//
// Verifies: REQ-JULIA-002, REQ-JULIA-003, REQ-JULIA-011
func TestReader(t *testing.T) {
	s := readSource([]byte(`module M
x = Expr(:module, :end, :function)
y = a[end] + a[begin:end-1]
z = [f(i) for i in 1:3 if isodd(i)]
g = (i for i in xs)
w = A'
c = '"'
d = '\''
e = "$(join(["a)", "b"], ")")) using NotHere"
r = raw"\d+ \" using NotHere2"
q = """
    using NotHere3
    """
#=
using NotHere4
=#
using Real1 # using NotHere5
f() = 1
@eval import Real2
try
    using Real3
catch
end
@static if Sys.iswindows()
    using Real4
end
quote
    using Real5
end
function outer()
    local inner() = 2
    nothing
end
h(x) = x
end
after() = 1
`))
	var specs []string
	for _, rawImport := range s.imports {
		specs = append(specs, rawImport.Spec)
	}
	if want := []string{"using Real1", "import Real2", "using Real3", "using Real4", "using Real5"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
	var names []string
	for _, symbol := range s.symbols {
		names = append(names, symbol.Name)
	}
	if want := []string{"M", "f", "outer", "h", "after"}; !reflect.DeepEqual(names, want) {
		t.Errorf("symbols %q, want %q", names, want)
	}
	if !reflect.DeepEqual(s.modules, []string{"M"}) {
		t.Errorf("modules %q", s.modules)
	}
	// The import's line and module path.
	if rawImport := s.imports[0]; rawImport.Line != 17 || rawImport.Name != kindUsing+"\nM" {
		t.Errorf("using Real1: %+v", rawImport)
	}
}

// Relative imports climb the module tree, and stop at a package's top module.
//
// Verifies: REQ-JULIA-004, REQ-JULIA-010
func TestRelativeImports(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	for spec, want := range map[string]lang.Target{
		".Cart":      {}, // the module itself
		"..Pricing":  {Local: "src/internal/pricing.jl"},
		"...Pricing": {Local: "src/internal/pricing.jl"},
		"..Shop":     {Local: "src/Shop.jl"},
		".Nothing":   {},
		"..":         {Local: "src/Shop.jl"},
	} {
		if got := r.Resolve("src/cart.jl", lang.RawImport{Module: spec, Name: kindUsing + "\nCart"}); got != want {
			t.Errorf("%s: got %+v, want %+v", spec, got, want)
		}
	}
}

// Every prefix of every fixture file, and pathological inputs, read without a
// panic and in bounded time.
//
// Verifies: REQ-JULIA-011
func TestTruncated(t *testing.T) {
	files := langtest.Files(t, "testdata/repo")
	for _, f := range files {
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			t.Fatal(err)
		}
		for n := 0; n <= len(source); n++ {
			extraction, err := (Plugin{}).Extract(f, source[:n])
			if err != nil || extraction == nil {
				t.Fatalf("%s[:%d]: %v", f.Path, n, err)
			}
		}
	}
	for _, s := range []string{
		strings.Repeat("(", 100000), strings.Repeat("[", 100000), strings.Repeat(`"$(`, 20000),
		strings.Repeat("begin ", 50000), strings.Repeat("end ", 50000), strings.Repeat("#=", 50000),
		strings.Repeat("f(x) = ", 20000), strings.Repeat("using ", 20000), strings.Repeat("include(", 20000),
		strings.Repeat("'", 50000), strings.Repeat(":", 50000), strings.Repeat(`"""`, 20000),
	} {
		readSource([]byte(s))
	}
}
