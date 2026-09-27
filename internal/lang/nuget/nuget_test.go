package nuget

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Verifies: REQ-FSHARP-006
func TestParseDependencies(t *testing.T) {
	src := "\xef\xbb\xbfsource https://api.nuget.org/v3/index.json\n" +
		"# a comment\n" +
		"storage: none\n" +
		"nuget Argu >= 6.1 redirects: force // trailing\n" +
		"nuget Expecto\n" +
		"clitool dotnet-fake 5.0.0\n" +
		"github fsharp/FAKE:0341a2e614eb2a7f34607cec914eb0ed83ce9add src/Globbing.fs\n" +
		"gist Thorium/1972308 gistfile1.fs\n" +
		"git https://github.com/fsprojects/Chessie.git >= 1.0\n" +
		"http http://www.fssnip.net/raw/1M/test1.fs\n" +
		"group Build\n" +
		"  source https://feed.example/v3/index.json\n" +
		"  nuget FAKE @~> 4.0 prerelease\n"
	deps, sources := ParseDependencies([]byte(src))
	want := []Dependency{
		{Group: "Main", Kind: "nuget", Name: "Argu", Constraint: ">= 6.1", Line: 4, Text: "nuget Argu >= 6.1 redirects: force"},
		{Group: "Main", Kind: "nuget", Name: "Expecto", Line: 5, Text: "nuget Expecto"},
		{Group: "Main", Kind: "nuget", Name: "dotnet-fake", Constraint: "5.0.0", Line: 6, Text: "clitool dotnet-fake 5.0.0"},
		{Group: "Main", Kind: "github", Name: "fsharp/FAKE", Constraint: "0341a2e614eb2a7f34607cec914eb0ed83ce9add", File: "src/Globbing.fs", Line: 7, Text: "github fsharp/FAKE:0341a2e614eb2a7f34607cec914eb0ed83ce9add src/Globbing.fs"},
		{Group: "Main", Kind: "gist", Name: "Thorium/1972308", File: "gistfile1.fs", Line: 8, Text: "gist Thorium/1972308 gistfile1.fs"},
		{Group: "Main", Kind: "git", Name: "https://github.com/fsprojects/Chessie.git", Constraint: ">= 1.0", Line: 9, Text: "git https://github.com/fsprojects/Chessie.git >= 1.0"},
		{Group: "Main", Kind: "http", Name: "http://www.fssnip.net/raw/1M/test1.fs", Line: 10, Text: "http http://www.fssnip.net/raw/1M/test1.fs"},
		{Group: "Build", Kind: "nuget", Name: "FAKE", Constraint: "~> 4.0 prerelease", Line: 13, Text: "nuget FAKE @~> 4.0 prerelease"},
	}
	if !reflect.DeepEqual(deps, want) {
		t.Errorf("deps\n got %+v\nwant %+v", deps, want)
	}
	if w := []Source{{"Main", "https://api.nuget.org/v3/index.json"}, {"Build", "https://feed.example/v3/index.json"}}; !reflect.DeepEqual(sources, w) {
		t.Errorf("sources %v", sources)
	}
}

// Paket pins `= x`, `== x` and a bare version; ranges and nothing float.
//
// Verifies: REQ-FSHARP-008
func TestPaketVersion(t *testing.T) {
	for c, want := range map[string]struct {
		v      string
		pinned bool
	}{
		"":                   {"", false},
		"1.2.3":              {"1.2.3", true},
		"= 1.2.3":            {"1.2.3", true},
		"== 1.2.3":           {"1.2.3", true},
		"1.2.3-alpha001":     {"1.2.3-alpha001", true},
		"~> 1.2":             {"~> 1.2", false},
		">= 1.0 < 2.0":       {">= 1.0 < 2.0", false},
		"< 5":                {"< 5", false},
		"prerelease":         {"", false},
		"~> 4.0 prerelease":  {"~> 4.0", false},
		"= 2.0.0 prerelease": {"2.0.0", true},
		"master":             {"master", false},
	} {
		if v, pinned := PaketVersion(c); v != want.v || pinned != want.pinned {
			t.Errorf("%q: got %q %v, want %q %v", c, v, pinned, want.v, want.pinned)
		}
	}
}

// Verifies: REQ-FSHARP-007
func TestParseLock(t *testing.T) {
	src := "RESTRICTION: == net8.0\n" +
		"NUGET\n" +
		"  remote: https://api.nuget.org/v3/index.json\n" +
		"    Argu (6.1.1)\n" +
		"      FSharp.Core (>= 4.3.2) - restriction: >= netstandard2.0\n" +
		"      NETStandard.Library  - restriction: < netstandard1.6\n" +
		"    FSharp.Core (8.0.400) - redirects: force\n" +
		"GITHUB\n" +
		"  remote: fsharp/FAKE\n" +
		"    modules/Octokit/Octokit.fsx (a25c2f256a99242c1106b5a3478aae6bb68c7a93)\n" +
		"      Octokit (>= 0)\n" +
		"GIT\n" +
		"  remote: https://github.com/fsprojects/Paket.git\n" +
		"     (60b2a1f9ed22b54c9cf2ee2dca6f1c1d4a1f6b43)\n" +
		"      build: \"build.cmd\", OS: windows\n" +
		"\n" +
		"GROUP Build\n" +
		"NUGET\n" +
		"  remote: https://nuget.org/api/v2\n" +
		"  specs:\n" +
		"    FAKE (4.64.18)\n"
	got := ParseLock([]byte(src))
	for i := range got {
		got[i].indent = 0
	}
	want := []Locked{
		{Group: "Main", Kind: "nuget", Remote: "https://api.nuget.org/v3/index.json", Name: "Argu", Version: "6.1.1", Line: 4,
			Deps: []LockDep{{"FSharp.Core", ">= 4.3.2"}, {"NETStandard.Library", ""}}},
		{Group: "Main", Kind: "nuget", Remote: "https://api.nuget.org/v3/index.json", Name: "FSharp.Core", Version: "8.0.400", Line: 7},
		{Group: "Main", Kind: "github", Remote: "fsharp/FAKE", Name: "modules/Octokit/Octokit.fsx", Version: "a25c2f256a99242c1106b5a3478aae6bb68c7a93", Line: 10,
			Deps: []LockDep{{"Octokit", ">= 0"}}},
		{Group: "Main", Kind: "git", Remote: "https://github.com/fsprojects/Paket.git", Version: "60b2a1f9ed22b54c9cf2ee2dca6f1c1d4a1f6b43", Line: 14},
		{Group: "Build", Kind: "nuget", Remote: "https://nuget.org/api/v2", Name: "FAKE", Version: "4.64.18", Line: 21},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lock\n got %+v\nwant %+v", got, want)
	}
}

// Verifies: REQ-FSHARP-006
func TestParseReferences(t *testing.T) {
	src := "FSharp.Core\nNewtonsoft.Json copy_local: false\n  exclude Newtonsoft.Json.dll\n// comment\n" +
		"File: Globbing.fs Common\nFile:FsUnit.fs\ngroup Build\n  FAKE\n  alias FakeLib.dll Fake\n"
	want := []Reference{
		{Group: "Main", Name: "FSharp.Core", Line: 1}, {Group: "Main", Name: "Newtonsoft.Json", Line: 2},
		{Group: "Main", Name: "Globbing.fs", File: true, Line: 5}, {Group: "Main", Name: "FsUnit.fs", File: true, Line: 6},
		{Group: "Build", Name: "FAKE", Line: 8},
	}
	if got := ParseReferences([]byte(src)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

// Verifies: REQ-FSHARP-005
func TestReadProject(t *testing.T) {
	src := "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"utf-8\"?>\n" +
		"<Project Sdk=\"Microsoft.NET.Sdk\">\n" +
		"  <PropertyGroup><DisableImplicitFSharpCoreReference>true</DisableImplicitFSharpCoreReference></PropertyGroup>\n" +
		"  <ItemGroup>\n" +
		"    <Compile Include=\"A.fs\" />\n" +
		"    <PackageReference Include=\"Serilog\">\n" +
		"      <Version>3.1.1</Version>\n" +
		"    </PackageReference>\n" +
		"    <PackageReference Update=\"FSharp.Core\" Version=\"8.0.400\" />\n" +
		"  </ItemGroup>\n" +
		"</Project>\n"
	p := ReadProject([]byte(src))
	want := []Item{
		{Kind: "Compile", Include: "A.fs", Line: 5},
		{Kind: "PackageReference", Include: "Serilog", Version: "3.1.1", Line: 6},
		{Kind: "PackageReference", Include: "FSharp.Core", Update: true, Version: "8.0.400", Line: 9},
	}
	if !reflect.DeepEqual(p.Items, want) || p.Sdk != "Microsoft.NET.Sdk" || p.Properties["DisableImplicitFSharpCoreReference"] != "true" {
		t.Errorf("got %+v", p)
	}
	for inc, want := range map[string]string{`..\Shared\X.fs`: "../Shared/X.fs", "$(MSBuildThisFileDirectory)X.fs": "X.fs", "$(Gen)/X.fs": "", "*.fs": ""} {
		if got, _ := ProjectPath(inc); got != want {
			t.Errorf("%s: %q, want %q", inc, got, want)
		}
	}
}

func files(t *testing.T, content map[string]string) []*scan.File {
	t.Helper()
	root := t.TempDir()
	var all []*scan.File
	for rel, body := range content {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		all = append(all, &scan.File{Path: rel, Abs: abs})
	}
	return all
}

// NuGet's packages.lock.json pins what restore resolved and lists what each
// package depends on; the lock's spelling of an id wins.
//
// Verifies: REQ-FSHARP-008, REQ-SUP-011
func TestPackagesLock(t *testing.T) {
	s := Read(files(t, map[string]string{
		"app/App.fsproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="serilog" Version="3.*" /></ItemGroup></Project>`,
		"app/packages.lock.json": `{"version": 1, "dependencies": {"net8.0": {
			"Serilog": {"type": "Direct", "requested": "[3.*, )", "resolved": "3.1.1", "dependencies": {"System.Memory": "4.5.5"}},
			"System.Memory": {"type": "Transitive", "resolved": "4.5.5"},
			"Shop.Core": {"type": "Project"}}}}`,
	}))
	got, ok := s.Package("app/Program.fs", "", "SERILOG")
	if want := (lang.Target{Ecosystem: "nuget", Package: "Serilog", Version: "3.1.1", Requested: "3.*", Pinned: true}); !ok || got != want {
		t.Errorf("Serilog: %+v", got)
	}
	deps := s.Dependencies(got)
	if want := []lang.Target{{Ecosystem: "nuget", Package: "System.Memory", Version: "4.5.5", Pinned: true}}; !reflect.DeepEqual(deps, want) {
		t.Errorf("deps %+v", deps)
	}
	// A lock-only package does not take a namespace.
	if _, ok := s.Declared("", "System.Memory"); ok {
		t.Error("a transitive package took its namespace")
	}
	if _, ok := s.Package("", "", "Shop.Core"); ok {
		t.Error("a project entry became a package")
	}
}

// Two Paket roots lock one package at different versions: each file sees its own
// root's, requested and resolved from the same root.
//
// Verifies: REQ-FSHARP-007, REQ-FSHARP-008
func TestPaketRoots(t *testing.T) {
	s := Read(files(t, map[string]string{
		"paket.dependencies":            "nuget Argu >= 6.0\nnuget Fake.Core.Target\n",
		"paket.lock":                    "NUGET\n  remote: https://api.nuget.org/v3/index.json\n    Argu (6.1.1)\n      FSharp.Core (>= 4.3.2)\n    Fake.Core.Target (6.0.0)\n    FSharp.Core (8.0.400)\n",
		"template/paket.dependencies":   "nuget Argu ~> 5\n",
		"template/paket.lock":           "NUGET\n  remote: https://api.nuget.org/v3/index.json\n    Argu (5.5.0)\n",
		"template/src/paket.references": "Argu\n",
	}))
	root, _ := s.Package("src/App.fs", "", "Argu")
	tmpl, _ := s.Package("template/src/App.fs", "", "argu")
	if root.Version != "6.1.1" || root.Requested != ">= 6.0" || tmpl.Version != "5.5.0" || tmpl.Requested != "~> 5" {
		t.Errorf("root %+v, template %+v", root, tmpl)
	}
	if deps := s.Dependencies(root); len(deps) != 1 || deps[0].Package != "FSharp.Core" || deps[0].Version != "8.0.400" || !deps[0].Pinned {
		t.Errorf("deps %+v", deps)
	}
	if got, ok := s.Under("src/App.fs", "Fake.Core"); !ok || got.Package != "Fake.Core.Target" {
		t.Errorf("under Fake.Core: %+v", got)
	}
	if _, ok := s.Under("src/App.fs", "Fake"); !ok {
		t.Error("nothing under Fake")
	}
}

// Verifies: REQ-FSHARP-007
func TestRemotes(t *testing.T) {
	s := Read(files(t, map[string]string{
		"paket.dependencies": "github fsharp/FAKE:master src/Globbing.fs\ngithub forki/FsUnit FsUnit.fs\ngit https://github.com/acme/tools.git v1.0\nhttp http://example.com/x.fs\n",
		"paket.lock":         "GITHUB\n  remote: fsharp/FAKE\n    src/Globbing.fs (0341a2e614eb2a7f34607cec914eb0ed83ce9add)\n",
	}))
	for _, tt := range []struct {
		kind, name string
		want       lang.Target
	}{
		{"github", "fsharp/FAKE", lang.Target{Ecosystem: "paket", Package: "github.com/fsharp/FAKE", Version: "0341a2e614eb2a7f34607cec914eb0ed83ce9add", Requested: "master", Pinned: true}},
		{"github", "forki/FsUnit", lang.Target{Ecosystem: "paket", Package: "github.com/forki/FsUnit", Floating: true}},
		{"git", "https://github.com/acme/tools.git", lang.Target{Ecosystem: "paket", Package: "github.com/acme/tools", Version: "v1.0"}},
		{"http", "http://example.com/x.fs", lang.Target{Ecosystem: "paket", Package: "example.com/x.fs", Floating: true}},
	} {
		if got, ok := s.Remote(tt.kind, tt.name, ""); !ok || got != tt.want {
			t.Errorf("%s %s: %+v", tt.kind, tt.name, got)
		}
	}
	if got, ok := s.RemoteFile(`..\paket-files\Globbing.fs`); !ok || got.Package != "github.com/fsharp/FAKE" {
		t.Errorf("File: Globbing.fs -> %+v", got)
	}
	if got, ok := s.RemoteRepo("forki/FsUnit/FsUnit.fs"); !ok || got.Package != "github.com/forki/FsUnit" {
		t.Errorf("paket-files/forki/FsUnit -> %+v", got)
	}
}
