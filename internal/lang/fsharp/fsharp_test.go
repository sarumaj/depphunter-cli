package fsharp

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

// testdata/repo: a Paket root (paket.dependencies with groups, github, git and
// http dependencies; paket.lock; paket.references per project) with central
// package versions (Directory.Packages.props); Shop.Domain.fsproj compiles
// Types.fs, Pricing.fsi, Pricing.fs, Cart.fs and Later.fs in that order, with a
// Paket-linked file from paket-files/ and an unexpandable item; Shop.Web.fsproj
// references Shop.Domain and a C# project; tests/Shop.Tests references neither;
// build.fsx is a script with #r and #load; a GLSL shader and a Forth file use .fs.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-FSHARP-002, REQ-FSHARP-004, REQ-FSHARP-008, REQ-FSHARP-012
func TestOpensAndReferences(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["src/Shop.Domain/Cart.fs"], map[string]lang.Target{
		"open Shop.Domain": {Local: "src/Shop.Domain/Types.fs"},
		// Later.fs comes after Cart.fs in the compile order.
		"open Shop.Domain.Later": {},
		"open Newtonsoft.Json":   {Ecosystem: "nuget", Package: "Newtonsoft.Json", Version: "13.0.3", Pinned: true},
		// A module in two files (signature and implementation): an edge to each.
		"Pricing (src/Shop.Domain/Pricing.fs)":  {Local: "src/Shop.Domain/Pricing.fs"},
		"Pricing (src/Shop.Domain/Pricing.fsi)": {Local: "src/Shop.Domain/Pricing.fsi"},
		"JsonConvert.SerializeObject":           {},
		// Declared by the file itself.
		"Cart.Empty": {},
		"Later":      {},
	})
	langtest.CheckImports(t, res["src/Shop.Web/Program.fs"], map[string]lang.Target{
		"open System":                       {Ecosystem: "dotnet", Package: "System"},
		"open System.Text.Json":             {Ecosystem: "dotnet", Package: "System.Text"},
		"open Microsoft.FSharp.Collections": {Ecosystem: "nuget", Package: "FSharp.Core", Version: "8.0.400", Requested: "~> 8.0", Pinned: true},
		"open FSharp.Control":               {Ecosystem: "nuget", Package: "FSharp.Core", Version: "8.0.400", Requested: "~> 8.0", Pinned: true},
		"open Argu":                         {Ecosystem: "nuget", Package: "Argu", Version: "6.1.1", Requested: ">= 6.1", Pinned: true},
		"open Newtonsoft.Json.Linq":         {Ecosystem: "nuget", Package: "Newtonsoft.Json", Version: "13.0.3", Pinned: true},
		"open Shop.Domain":                  {Local: "src/Shop.Domain/Types.fs"},
		"open type Shop.Domain.Cart.Cart":   {Local: "src/Shop.Domain/Cart.fs"},
		// A module of Fake.Core.Target, not of the FAKE package whose id prefixes it.
		"open Fake.Core.TargetOperators":                               {Ecosystem: "nuget", Package: "Fake.Core.Target", Version: "6.0.0", Pinned: true},
		"open Mystery.Lib":                                             {Ecosystem: "nuget", Package: "Mystery.Lib", Unresolved: true},
		"module P = Shop.Domain.Pricing (src/Shop.Domain/Pricing.fs)":  {Local: "src/Shop.Domain/Pricing.fs"},
		"module P = Shop.Domain.Pricing (src/Shop.Domain/Pricing.fsi)": {Local: "src/Shop.Domain/Pricing.fsi"},
		"Cart": {Local: "src/Shop.Domain/Cart.fs"},
		// A type of an [<AutoOpen>] module is known by its namespace.
		"Currency.EUR": {Local: "src/Shop.Domain/Types.fs"},
		"Views":        {Local: "src/Shop.Web/Views.fs"},
		"P":            {},
	})
	langtest.CheckImports(t, res["src/Shop.Domain/Types.fs"], map[string]lang.Target{
		"open System": {Ecosystem: "dotnet", Package: "System"},
	})
	// The tests project references no project declaring Shop.Domain: its open is
	// dropped rather than made a package.
	langtest.CheckImports(t, res["tests/Shop.Tests/Tests.fs"], map[string]lang.Target{
		"open Expecto":     {Ecosystem: "nuget", Package: "Expecto", Version: "10.2.1", Pinned: true},
		"open FsCheck":     {Ecosystem: "nuget", Package: "FsCheck", Version: "2.16.6", Pinned: true},
		"open Shop.Domain": {},
	})
}

// Verifies: REQ-FSHARP-002, REQ-FSHARP-008, REQ-FSHARP-012
func TestScriptDirectives(t *testing.T) {
	langtest.CheckImports(t, analyze(t)["build.fsx"], map[string]lang.Target{
		"#r \"nuget: Fake.Core.Process, 6.0.0\"":           {Ecosystem: "nuget", Package: "Fake.Core.Process", Version: "6.0.0", Pinned: true},
		"#r \"nuget: FSharp.Data\"":                        {Ecosystem: "nuget", Package: "FSharp.Data", Floating: true},
		"#r \"nuget: Serilog, 3.*\"":                       {Ecosystem: "nuget", Package: "Serilog", Version: "3.*"},
		"#load \"helpers.fsx\"":                            {Local: "scripts/helpers.fsx"},
		"#load \"missing.fsx\"":                            {},
		"#r \"packages/Argu/lib/netstandard2.0/Argu.dll\"": {Ecosystem: "nuget", Package: "Argu", Version: "6.1.1", Requested: ">= 6.1", Pinned: true},
		"#r \"System.Xml.Linq\"":                           {Ecosystem: "dotnet", Package: "System.Xml"},
		"#r \"bin/Debug/Shop.dll\"":                        {},
		"#r \"nuget: Expecto\"":                            {Ecosystem: "nuget", Package: "Expecto", Version: "10.2.1", Pinned: true},
		// The shortest #r package below the namespace.
		"open Fake.Core":   {Ecosystem: "nuget", Package: "Fake.Core.Process", Version: "6.0.0", Pinned: true},
		"open FSharp.Data": {Ecosystem: "nuget", Package: "FSharp.Data", Floating: true},
		"open Serilog":     {Ecosystem: "nuget", Package: "Serilog", Version: "3.*"},
		"Helpers":          {Local: "scripts/helpers.fsx"},
	})
}

// Verifies: REQ-FSHARP-005
func TestProjectFiles(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["src/Shop.Domain/Shop.Domain.fsproj"], map[string]lang.Target{
		"..\\..\\paket-files\\fsharp\\FAKE\\src\\app\\FakeLib\\Globbing\\Globbing.fs": {Ecosystem: "paket", Package: "github.com/fsharp/FAKE", Version: "0341a2e614eb2a7f34607cec914eb0ed83ce9add", Pinned: true},
		"Types.fs":                    {Local: "src/Shop.Domain/Types.fs"},
		"Pricing.fsi":                 {Local: "src/Shop.Domain/Pricing.fsi"},
		"Pricing.fs":                  {Local: "src/Shop.Domain/Pricing.fs"},
		"Cart.fs":                     {Local: "src/Shop.Domain/Cart.fs"},
		"Later.fs":                    {Local: "src/Shop.Domain/Later.fs"},
		"$(GeneratedDir)\\Version.fs": {},
		"Newtonsoft.Json":             {Ecosystem: "nuget", Package: "Newtonsoft.Json", Version: "13.0.3", Pinned: true},
		"FSharp.Core":                 {Ecosystem: "nuget", Package: "FSharp.Core", Version: "8.0.400", Requested: "~> 8.0", Pinned: true},
	})
	langtest.CheckImports(t, res["src/Shop.Web/Shop.Web.fsproj"], map[string]lang.Target{
		"Views.fs":                            {Local: "src/Shop.Web/Views.fs"},
		"Program.fs":                          {Local: "src/Shop.Web/Program.fs"},
		"..\\Shop.Domain\\Shop.Domain.fsproj": {Local: "src/Shop.Domain/Shop.Domain.fsproj"},
		"..\\Legacy\\Legacy.csproj":           {Local: "src/Legacy/Legacy.csproj"},
		"..\\Missing\\Missing.fsproj":         {},
		"FSharp.Core (implicit)":              {Ecosystem: "nuget", Package: "FSharp.Core", Version: "8.0.400", Requested: "~> 8.0", Pinned: true},
	})
	// DisableImplicitFSharpCoreReference: no implicit FSharp.Core.
	langtest.CheckImports(t, res["tests/Shop.Tests/Shop.Tests.fsproj"], map[string]lang.Target{
		"Tests.fs": {Local: "tests/Shop.Tests/Tests.fs"},
	})
}

// Verifies: REQ-FSHARP-006, REQ-FSHARP-007, REQ-FSHARP-008
func TestPaketFiles(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["paket.dependencies"], map[string]lang.Target{
		"nuget FSharp.Core ~> 8.0":           {Ecosystem: "nuget", Package: "FSharp.Core", Version: "8.0.400", Requested: "~> 8.0", Pinned: true},
		"nuget Argu >= 6.1 redirects: force": {Ecosystem: "nuget", Package: "Argu", Version: "6.1.1", Requested: ">= 6.1", Pinned: true},
		"nuget Expecto 10.2.1":               {Ecosystem: "nuget", Package: "Expecto", Version: "10.2.1", Pinned: true},
		"nuget FsCheck = 2.16.6":             {Ecosystem: "nuget", Package: "FsCheck", Version: "2.16.6", Pinned: true},
		"nuget Fake.Core.Target":             {Ecosystem: "nuget", Package: "Fake.Core.Target", Version: "6.0.0", Pinned: true},
		"github fsharp/FAKE:0341a2e614eb2a7f34607cec914eb0ed83ce9add src/app/FakeLib/Globbing/Globbing.fs": {Ecosystem: "paket", Package: "github.com/fsharp/FAKE", Version: "0341a2e614eb2a7f34607cec914eb0ed83ce9add", Pinned: true},
		"github forki/FsUnit FsUnit.fs":                        {Ecosystem: "paket", Package: "github.com/forki/FsUnit", Version: "7623fc13439f0e60bd05c1ed3b5f6dcb937fe468", Pinned: true},
		"git https://github.com/fsprojects/Chessie.git master": {Ecosystem: "paket", Package: "github.com/fsprojects/Chessie", Version: "1f23b1caeb1f87e750abc96a25109376771dd090", Requested: "master", Pinned: true},
		"http http://www.fssnip.net/raw/1M/test1.fs":           {Ecosystem: "paket", Package: "www.fssnip.net/raw/1M/test1.fs", Floating: true},
		"nuget FAKE < 5": {Ecosystem: "nuget", Package: "FAKE", Version: "4.64.18", Requested: "< 5", Pinned: true},
	})
	langtest.CheckImports(t, res["paket.lock"], map[string]lang.Target{
		"Argu":             {Ecosystem: "nuget", Package: "Argu", Version: "6.1.1", Requested: ">= 6.1", Pinned: true},
		"Expecto":          {Ecosystem: "nuget", Package: "Expecto", Version: "10.2.1", Pinned: true},
		"Fake.Core.Target": {Ecosystem: "nuget", Package: "Fake.Core.Target", Version: "6.0.0", Pinned: true},
		"FsCheck":          {Ecosystem: "nuget", Package: "FsCheck", Version: "2.16.6", Pinned: true},
		"FSharp.Core":      {Ecosystem: "nuget", Package: "FSharp.Core", Version: "8.0.400", Requested: "~> 8.0", Pinned: true},
		"Mono.Cecil":       {Ecosystem: "nuget", Package: "Mono.Cecil", Version: "0.11.5", Pinned: true},
		"System.Configuration.ConfigurationManager":        {Ecosystem: "nuget", Package: "System.Configuration.ConfigurationManager", Version: "8.0.0", Pinned: true},
		"fsharp/FAKE/src/app/FakeLib/Globbing/Globbing.fs": {Ecosystem: "paket", Package: "github.com/fsharp/FAKE", Version: "0341a2e614eb2a7f34607cec914eb0ed83ce9add", Pinned: true},
		"forki/FsUnit/FsUnit.fs":                           {Ecosystem: "paket", Package: "github.com/forki/FsUnit", Version: "7623fc13439f0e60bd05c1ed3b5f6dcb937fe468", Pinned: true},
		"https://github.com/fsprojects/Chessie.git":        {Ecosystem: "paket", Package: "github.com/fsprojects/Chessie", Version: "1f23b1caeb1f87e750abc96a25109376771dd090", Requested: "master", Pinned: true},
		"http://www.fssnip.net/raw/1M/test1.fs":            {Ecosystem: "paket", Package: "www.fssnip.net/raw/1M/test1.fs", Floating: true},
		"Build/FAKE":                                       {Ecosystem: "nuget", Package: "FAKE", Version: "4.64.18", Requested: "< 5", Pinned: true},
	})
	langtest.CheckImports(t, res["src/Shop.Web/paket.references"], map[string]lang.Target{
		"FSharp.Core":             {Ecosystem: "nuget", Package: "FSharp.Core", Version: "8.0.400", Requested: "~> 8.0", Pinned: true},
		"Argu":                    {Ecosystem: "nuget", Package: "Argu", Version: "6.1.1", Requested: ">= 6.1", Pinned: true},
		"Unknown.Package":         {Ecosystem: "nuget", Package: "Unknown.Package", Unresolved: true},
		"Build/FAKE":              {Ecosystem: "nuget", Package: "FAKE", Version: "4.64.18", Requested: "< 5", Pinned: true},
		"Build/File: Globbing.fs": {Ecosystem: "paket", Package: "github.com/fsharp/FAKE", Version: "0341a2e614eb2a7f34607cec914eb0ed83ce9add", Pinned: true},
	})
}

// Verifies: REQ-FSHARP-003
func TestSymbols(t *testing.T) {
	res := analyze(t)
	langtest.CheckSymbols(t, res["src/Shop.Domain/Cart.fs"], map[string]string{
		"banner":        "value",
		"verbatim":      "value",
		"triple":        "value",
		"interpolated":  "func",
		"quote":         "value",
		"brace":         "value",
		"id":            "func",
		"Cart":          "class",
		"Cart.Items":    "method",
		"Cart.Total":    "method",
		"Cart.Empty":    "method",
		"Cart.ToString": "method",
		"empty":         "value",
		"summed":        "value",
		"(|Big|Small|)": "func",
		"later":         "value",
	})
	langtest.CheckSymbols(t, res["src/Shop.Domain/Types.fs"], map[string]string{
		"Item":           "type",
		"Status":         "type",
		"Money":          "module",
		"Money.add":      "func",
		"Money.zero":     "value",
		"Money.Currency": "type",
		"IPriced":        "interface",
		"IPriced.Price":  "method",
		"OutOfStock":     "exception",
	})
	langtest.CheckSymbols(t, res["src/Shop.Web/Views.fs"], map[string]string{
		"Views":      "module",
		"Views.page": "func",
	})
	langtest.CheckSymbols(t, res["src/Shop.Domain/Pricing.fsi"], map[string]string{
		"total": "func",
	})
}

// A GLSL shader and a Forth file share .fs; scan labels them and they are not
// claimed, nor is the C# file, nor what Paket and FAKE downloaded.
//
// Verifies: REQ-FSHARP-001
func TestClaims(t *testing.T) {
	res := analyze(t)
	for _, p := range []string{"shaders/blur.fs", "forth/hello.fs", "src/Legacy/Thing.cs", "src/Legacy/Legacy.csproj", "Directory.Packages.props"} {
		if _, ok := res[p]; ok {
			t.Errorf("%s claimed", p)
		}
	}
	for _, p := range []string{"build.fsx", "src/Shop.Domain/Pricing.fsi", "paket.lock", "src/Shop.Web/paket.references", "src/Shop.Web/Shop.Web.fsproj"} {
		if _, ok := res[p]; !ok {
			t.Errorf("%s not claimed", p)
		}
	}
	for _, f := range []*scan.File{
		{Path: "paket-files/fsharp/FAKE/src/Globbing.fs", Lang: "F#"},
		{Path: ".fake/build.fsx/intellisense.fsx"},
		{Path: "src/App.fs", Lang: "GLSL"},
		{Path: "src/App.fs"},
	} {
		if (Plugin{}).Claims(f) {
			t.Errorf("%s (%q) claimed", f.Path, f.Lang)
		}
	}
	if !(Plugin{}).Claims(&scan.File{Path: "src/App.fs", Lang: "F#"}) {
		t.Error("an F# .fs file not claimed")
	}
}

// Comments (nested, and holding strings with "*)"), strings of every form, char
// literals and type variables hide or keep what they should.
//
// Verifies: REQ-FSHARP-002, REQ-FSHARP-011
func TestLiteralsHideCode(t *testing.T) {
	src := "module M\n" +
		"(* open A (* open B *) \"*)\" open C *)\n" +
		"// open D\n" +
		"let a = \"open E \\\" open F\"\n" +
		"let b = @\"C:\\ \"\"open G\"\"\"\n" +
		"let c = \"\"\"open H \"quoted\" \"\"\"\n" +
		"let d = $\"x {(if true then \"}\" else \"{\")} open I {{not}}\"\n" +
		"let e = $$\"\"\"{{a}} {b} open J\"\"\"\n" +
		"let f = '\"'\n" +
		"let g = '\\''\n" +
		"let h<'T> (x: 'T) = x\n" +
		"let i' = 1\n" +
		"let j = (*) 2 3\n" +
		"let k = @\"\"\"open K\"\n" +
		"let l = \"bytes\"B\n" +
		"open Real.One\n" +
		"let m = $\"{x'}\" // a primed name in a hole\n" +
		"open Real.Two\n"
	in := scanSource(src)
	var opens []string
	for _, im := range in.imports {
		opens = append(opens, im.Module)
	}
	if want := []string{"Real.One", "Real.Two"}; !reflect.DeepEqual(opens, want) {
		t.Errorf("opens %v, want %v", opens, want)
	}
	var names []string
	for _, s := range in.symbols {
		names = append(names, s.Name)
	}
	if want := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i'", "j", "k", "l", "m"}; !reflect.DeepEqual(names, want) {
		t.Errorf("symbols %v, want %v", names, want)
	}
}

// Nested modules, types with members, mutually recursive types and lets, and a
// namespace with several modules: the offside rule without a parser.
//
// Verifies: REQ-FSHARP-003
func TestOffside(t *testing.T) {
	src := `namespace Shop.Orders

open System

[<RequireQualifiedAccess>]
module Order =
    type Line = { Sku: string }

    let create sku = { Sku = sku }

    module Rules =
        let valid (l: Line) = l.Sku <> ""

    let total lines =
        let inner x = x
        lines |> List.length

type Service(repo: obj) =
    let cache = ()
    member this.Place(o) = ()
    member val Count = 0 with get, set
    abstract Cancel: unit -> unit
    default this.Cancel() = ()
    static member (+) (a: Service, b: Service) = a
    interface IDisposable with
        member _.Dispose() = ()

type A =
    | A of B
and B =
    | B of A

module Tail =
    let rec even n = n = 0 || odd (n - 1)
    and odd n = n <> 0 && even (n - 1)
`
	got := map[string]string{}
	for _, s := range scanSource(src).symbols {
		got[s.Name] = s.Kind
	}
	want := map[string]string{
		"Order": "module", "Order.Line": "type", "Order.create": "func", "Order.Rules": "module",
		"Order.Rules.valid": "func", "Order.total": "func",
		"Service": "class", "Service.Place": "method", "Service.Count": "method", "Service.Cancel": "method",
		"Service.(+)": "method", "Service.Dispose": "method",
		"A": "type", "B": "type", "Tail": "module", "Tail.even": "func", "Tail.odd": "func",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("symbols\n got %v\nwant %v", got, want)
	}
	var decls []string
	for _, d := range scanSource(src).decls {
		decls = append(decls, string(d.kind)+" "+d.name)
	}
	if w := []string{"n Shop.Orders", "m Shop.Orders.Order", "t Shop.Orders.Order.Line", "m Shop.Orders.Order.Rules",
		"t Shop.Orders.Service", "t Shop.Orders.A", "t Shop.Orders.B", "m Shop.Orders.Tail"}; !reflect.DeepEqual(decls, w) {
		t.Errorf("decls %v, want %v", decls, w)
	}
}

// Every prefix of every fixture file goes through Extract without a panic, and
// pathological inputs stay linear.
//
// Verifies: REQ-FSHARP-011
func TestTruncated(t *testing.T) {
	var files []string
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	for _, p := range files {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f := &scan.File{Path: strings.TrimPrefix(filepath.ToSlash(p), "testdata/repo/")}
		for i := 0; i <= len(src); i++ {
			if _, err := (Plugin{}).Extract(f, src[:i]); err != nil {
				t.Fatal(err)
			}
			scanSource(string(src[:i]))
		}
	}
	for _, unit := range []string{"(*", "*)", "(", "[<", ">]", "\"", "@\"", "\"\"\"", "$\"{", "$$\"\"\"{{", "{", "'", "'a", "``",
		"#load \"", "#r @\"", "open ", "open type ", "module M =\n", "module ", "type T =\n ", "let ", "namespace N\n",
		"A.", "A.B.c ", " member x.", "and ", "[<AutoOpen>]\n", "\\", "é", "0x1e-", "let (|", "exception "} {
		src := strings.Repeat(unit, 200_000/len(unit)+1)
		start := time.Now()
		scanSource(src)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(src)/len(unit), d)
		}
	}
}

// Verifies: REQ-FSHARP-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids["dotnet"]; !ok || !std || len(ids) != 3 || ids["nuget"] || ids["paket"] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for f, r := range analyze(t) {
		for _, im := range r.Imports {
			if e := im.Target.Ecosystem; e != "" && !(e == "nuget" || e == "dotnet" || e == "paket") {
				t.Errorf("%s: %s -> %s", f, im.Spec, e)
			}
		}
	}
}
