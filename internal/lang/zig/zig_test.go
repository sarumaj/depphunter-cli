package zig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a package shop: build.zig wires modules with the current API
// (b.addModule, b.createModule with .imports, addImport, a dependency's module, a
// lazy dependency's capture, a function returning a module, options) and
// build.zig.zon declares GitHub, git+https, Codeberg, GitLab, branch and mirror
// dependencies with and without hashes and a local package (libs/utils). legacy/ is
// a package on Zig 0.11's build API; examples/demo depends on the root package by
// path; zig-pkg/ holds one fetched dependency's build.zig.zon.

var (
	knownFolders = lang.Target{Ecosystem: ecosystemZig, Package: "github.com/ziglibs/known-folders", Version: "6da0d0c41b78b9ed2d34fa364fcb81b5ebece6c4", Pinned: true}
	vaxis        = lang.Target{Ecosystem: ecosystemZig, Package: "github.com/rockorager/libvaxis", Version: "dc0a228a5544988d4a920cfb40be9cd28db41423", Requested: "v0.5.1", Pinned: true}
	tracy        = lang.Target{Ecosystem: ecosystemZig, Package: "deps.example.org/tracy", Version: "N-V-__8AAOncKwEm1F9c5LrT7HMNmRMYX8-fAoqpc6YyTu9X", Pinned: true}
	zf           = lang.Target{Ecosystem: ecosystemZig, Package: "codeberg.org/natecraddock/zf", Version: "0.10.3"}
	clap         = lang.Target{Ecosystem: ecosystemZig, Package: "github.com/Hejsil/zig-clap", Version: "0.7.0", Pinned: true}
	std          = lang.Target{Ecosystem: ecosystemStd, Package: "std"}
)

// Verifies: REQ-ZIG-002, REQ-ZIG-004, REQ-ZIG-005, REQ-ZIG-006, REQ-ZIG-010, REQ-ZIG-011
func TestSourceImports(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["src/main.zig"], map[string]lang.Target{
		`@import("std")`:                  std,
		`@import("builtin")`:              {Ecosystem: ecosystemStd, Package: "builtin"},
		`@import("shop")`:                 {Local: "src/shop.zig"},         // .imports, b.addModule
		`@import("known-folders")`:        knownFolders,                    // dependency().module()
		`@import("vaxis")`:                vaxis,                           // a variable's .module()
		`@import("config")`:               {},                              // options: generated
		`@import("utils")`:                {Local: "libs/utils/utils.zig"}, // a path dependency's module
		`@import("model")`:                {Local: "src/model.zig"},        // a function's return
		`@import("tracy")`:                tracy,                           // a lazy dependency's capture
		`@import("zf")`:                   zf,                              // not wired: the dependency of that name
		`@import("cli/args.zig")`:         {Local: "src/cli/args.zig"},
		`@import("missing.zig")`:          {},
		`@import("nowhere")`:              {},
		`@embedFile("assets/banner.txt")`: {Local: "src/assets/banner.txt"},
		`@cInclude("stdio.h")`:            {Ecosystem: "c-std", Package: "stdio.h"},
		`@cInclude("shop.h")`:             {Local: "include/shop.h"},
	})
	// root is the compilation's root source file: src/main.zig reaches both (the
	// test root tests/all.zig and the example's root reach shop.zig too).
	langtest.CheckImports(t, results["src/shop.zig"], map[string]lang.Target{
		`@import("std")`:  std,
		`@import("root")`: {Local: "src/main.zig"},
	})
	langtest.CheckImports(t, results["src/cli/args.zig"], map[string]lang.Target{
		`@import("../shop.zig")`: {Local: "src/shop.zig"},
		`@import("root")`:        {Local: "src/main.zig"},
	})
	langtest.CheckImports(t, results["tests/all.zig"], map[string]lang.Target{
		`@import("std")`:  std,
		`@import("shop")`: {Local: "src/shop.zig"},
		`@import("root")`: {}, // itself
	})
	// The example's build.zig wires the root package's exported module.
	langtest.CheckImports(t, results["examples/demo/main.zig"], map[string]lang.Target{
		`@import("shop")`: {Local: "src/shop.zig"},
		`@import("root")`: {},
	})
	// Zig 0.11: step.addModule and a module's .dependencies.
	langtest.CheckImports(t, results["legacy/main.zig"], map[string]lang.Target{
		`@import("helper")`:   {Local: "legacy/helper.zig"},
		`@import("fmt")`:      {Local: "legacy/fmt.zig"},
		`@import("zig-clap")`: clap,
	})
	langtest.CheckImports(t, results["legacy/fmt.zig"], map[string]lang.Target{`@import("helper")`: {Local: "legacy/helper.zig"}})
}

// Verifies: REQ-ZIG-004, REQ-ZIG-006
func TestBuildFiles(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["build.zig"], map[string]lang.Target{
		`@import("std")`:                std,
		`@import("utils")`:              {Local: "libs/utils/build.zig"}, // a dependency's build.zig
		`b.path("src/shop.zig")`:        {Local: "src/shop.zig"},
		`b.path("src/main.zig")`:        {Local: "src/main.zig"},
		`b.path("src/model.zig")`:       {Local: "src/model.zig"},
		`b.path("tests/all.zig")`:       {Local: "tests/all.zig"},
		`b.path("src/native.c")`:        {Local: "src/native.c"},
		`b.path("include")`:             {Local: "include"},
		`b.dependency("known_folders")`: knownFolders,
		`b.dependency("vaxis")`:         vaxis,
		`b.dependency("utils")`:         {Local: "libs/utils"},
		`b.lazyDependency("tracy")`:     tracy,
	})
	langtest.CheckImports(t, results["legacy/build.zig"], map[string]lang.Target{
		`@import("std")`:            std,
		`.{ .path = "helper.zig" }`: {Local: "legacy/helper.zig"},
		`.{ .path = "main.zig" }`:   {Local: "legacy/main.zig"},
		`.{ .path = "fmt.zig" }`:    {Local: "legacy/fmt.zig"},
	})
	langtest.CheckImports(t, results["examples/demo/build.zig"], map[string]lang.Target{
		`@import("std")`:       std,
		`b.dependency("shop")`: {Local: "build.zig.zon"}, // the repository's root package
		`b.path("main.zig")`:   {Local: "examples/demo/main.zig"},
	})
}

// Verifies: REQ-ZIG-005, REQ-ZIG-007, REQ-ZIG-008
func TestManifest(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["build.zig.zon"], map[string]lang.Target{
		".minimum_zig_version": {Ecosystem: ecosystemStd, Package: "zig", Version: ">= 0.14.0"},
		"known_folders":        knownFolders,
		"vaxis":                vaxis,
		"zf":                   zf,
		"zeit":                 {Ecosystem: ecosystemZig, Package: "github.com/rockorager/zeit", Version: "main", Floating: true},
		"ziglyph":              {Ecosystem: ecosystemZig, Package: "gitlab.com/dude_the_builder/ziglyph", Version: "v0.11.1", Pinned: true},
		"tracy":                tracy,
		"utils":                {Local: "libs/utils"},
	})
	langtest.CheckImports(t, results["legacy/build.zig.zon"], map[string]lang.Target{"zig-clap": clap})
	langtest.CheckImports(t, results["examples/demo/build.zig.zon"], map[string]lang.Target{"shop": {Local: "build.zig.zon"}})
	langtest.CheckImports(t, results["data/config.zon"], map[string]lang.Target{})
	langtest.CheckSymbols(t, results["build.zig.zon"], map[string]string{"shop": "package"})
	langtest.CheckSymbols(t, results["legacy/build.zig.zon"], map[string]string{"legacy": "package"})
	langtest.CheckSymbols(t, results["data/config.zon"], map[string]string{})
}

// Verifies: REQ-ZIG-008
func TestPackageNames(t *testing.T) {
	r := &resolver{files: map[string]bool{}, directories: map[string]bool{}}
	for _, c := range []struct {
		url, hash string
		want      lang.Target
	}{
		{"https://github.com/o/r/archive/refs/tags/v1.2.0.tar.gz", "", lang.Target{Package: "github.com/o/r", Version: "v1.2.0"}},
		{"https://github.com/o/r/tarball/0123456789abcdef0123456789abcdef01234567", "", lang.Target{Package: "github.com/o/r", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true}},
		{"https://github.com/o/r/releases/download/v2.0/r-linux.tar.xz", "1220aa", lang.Target{Package: "github.com/o/r", Version: "v2.0", Pinned: true}},
		{"https://git.sr.ht/~o/r/archive/v0.3.tar.gz", "", lang.Target{Package: "git.sr.ht/~o/r", Version: "v0.3"}},
		{"git+https://codeberg.org/o/r#0123456789abcdef0123456789abcdef01234567", "", lang.Target{Package: "codeberg.org/o/r", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true}},
		{"git+https://github.com/o/r.git#main", "", lang.Target{Package: "github.com/o/r", Version: "main"}},
		{"https://example.org/files/zlib-1.3.1.tar.gz", "zlib-1.3.1-ZZZ", lang.Target{Package: "example.org/files/zlib", Version: "1.3.1", Pinned: true}},
		{"https://example.org/snapshot.tar.gz", "", lang.Target{Package: "example.org/snapshot", Floating: true}},
		{"https://example.org/snapshot.tar.gz", "pkg-0.2.0-dev.3-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", lang.Target{Package: "example.org/snapshot", Version: "0.2.0-dev.3", Pinned: true}},
	} {
		c.want.Ecosystem = ecosystemZig
		if got := r.zonTarget(".", zonDependency{key: "k", url: c.url, hash: c.hash}); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.url, got, c.want)
		}
	}
	if got := r.zonTarget(".", zonDependency{key: "broken"}); got != (lang.Target{Ecosystem: ecosystemZig, Package: "broken", Unresolved: true, Floating: true}) {
		t.Errorf("a dependency without url or path: %+v", got)
	}
}

// Verifies: REQ-ZIG-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, results["src/shop.zig"], map[string]string{
		"Allocator": "const", "Cart": "type", "Cart.Item": "type", "Cart.Item.price": "method",
		"Cart.add": "method", "Cart.nested": "method", "Color": "type", "Shape": "type",
		"Shape.area": "method", "Error": "type", "Handle": "type", "Packed": "type",
		"counter": "var", "abs": "func", "last": "var", "checkout": "func", "Pair": "func",
		"cart adds items": "test", "checkout@73": "test", "weird name": "const",
	})
	langtest.CheckSymbols(t, results["src/main.zig"], map[string]string{"banner": "const", "main": "func"})
	langtest.CheckSymbols(t, results["build.zig"], map[string]string{"build": "func", "modelModule": "func"})
}

// Verifies: REQ-ZIG-001
func TestClaims(t *testing.T) {
	for _, p := range []string{"src/main.zig", "build.zig", "build.zig.zon", "data/config.zon"} {
		if !(Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s not claimed", p)
		}
	}
	for _, p := range []string{".zig-cache/o/1/cimport.zig", "zig-cache/h/x.zig", "zig-out/lib/x.zig", "zig-pkg/x-0.1.0-AAAA/build.zig.zon", "main.c"} {
		if (Plugin{}).Claims(&scan.File{Path: p}) {
			t.Errorf("%s claimed", p)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "a.zig", Binary: true}) {
		t.Error("a binary file claimed")
	}
	if lang.ClassOf(Plugin{}, &scan.File{Path: "build.zig.zon"}) == lang.ClassOf(Plugin{}, &scan.File{Path: "data/config.zon"}) {
		t.Error("build.zig.zon and a data .zon share a cache class")
	}
}

// Verifies: REQ-ZIG-009
func TestDependencies(t *testing.T) {
	root := "testdata/repo"
	files := langtest.Files(t, root)
	cache := t.TempDir()
	t.Setenv("ZIG_GLOBAL_CACHE_DIR", cache)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", "")
	directory := filepath.Join(cache, "p", "known_folders-0.0.0-Fy-PJiDLAAB98m3uYUzatrTb2mO2fpvwx2zpSroEtfbO")
	os.MkdirAll(directory, 0o755)
	os.WriteFile(filepath.Join(directory, "build.zig.zon"), []byte(`.{ .name = .known_folders, .version = "0.0.0", .paths = .{""} }`), 0o644)

	r := newResolver(root, files)
	dependencies := r.Dependencies(vaxis)
	want := lang.Target{Ecosystem: ecosystemZig, Package: "github.com/zigimg/zigimg", Version: "3a667bdb3d7f0955a5a51c8468eac83210c1439e", Pinned: true}
	if len(dependencies) != 1 || dependencies[0] != want || !r.Installed(vaxis) {
		t.Errorf("vaxis from zig-pkg: %+v", dependencies)
	}
	if dependencies := r.Dependencies(knownFolders); len(dependencies) != 0 || !r.Installed(knownFolders) {
		t.Errorf("known_folders from the global cache: %+v, installed %v", dependencies, r.Installed(knownFolders))
	}
	if dependencies := r.Dependencies(tracy); len(dependencies) != 0 || r.Installed(tracy) {
		t.Errorf("a package not fetched: %+v", dependencies)
	}
}

// Verifies: REQ-ZIG-012
func TestLexer(t *testing.T) {
	s := readSource([]byte("const a = \"@import(\\\"no.zig\\\")\"; // @import(\"nor.zig\")\n" +
		"const q = '\"'; const m =\n    \\\\ @import(\"neither.zig\")\n;\n" +
		"const r = 0..5; const f = 1.5e-3; const x = @import(\"yes.zig\");\n" +
		"pub fn @\"odd name\"() void {}\n"))
	if len(s.imports) != 1 || s.imports[0].Module != "yes.zig" || s.imports[0].Line != 5 {
		t.Errorf("imports: %+v", s.imports)
	}
	names := map[string]bool{}
	for _, symbol := range s.symbols.List() {
		names[symbol.Name] = true
	}
	for _, n := range []string{"a", "q", "m", "r", "f", "odd name"} {
		if !names[n] {
			t.Errorf("%s missing from %v", n, names)
		}
	}
}

// Every prefix of every fixture file extracts without a panic, and pathological
// inputs finish (a run of unclosed brackets, nested containers, a lone quote).
//
// Verifies: REQ-ZIG-012
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
			evalBuild(source[:n], ".", newFacts())
			readZon(source[:n])
		}
	}
	for _, s := range []string{
		strings.Repeat("(", 100000), strings.Repeat("{", 100000), strings.Repeat(".{", 50000),
		strings.Repeat("const a = struct {", 20000), strings.Repeat("fn f() ", 20000),
		strings.Repeat("@import(", 20000), strings.Repeat(`"`, 50000), strings.Repeat("'", 50000),
		strings.Repeat("\\\\", 50000), strings.Repeat("b.dependency(\"x\").module(", 10000),
		strings.Repeat(".{ .name = \"x\", .module = ", 10000), strings.Repeat("x = y; y = x;", 10000),
		strings.Repeat("f(", 30000) + strings.Repeat(")", 30000),
		strings.Repeat(`b.addImport("x", b.createModule(.{ .root_source_file = b.path(`, 5000) + strings.Repeat(")})", 5000),
	} {
		readSource([]byte(s))
		evalBuild([]byte(s), ".", newFacts())
		readZon([]byte(s))
	}
}

// A relative XDG_CACHE_HOME is not this machine's cache: it would be read from the
// repository depphunter runs in, so a package there is not taken as fetched.
//
// Verifies: REQ-ZIG-009
func TestRelativeXDGCacheIgnored(t *testing.T) {
	root, err := filepath.Abs("testdata/repo")
	if err != nil {
		t.Fatal(err)
	}
	files := langtest.Files(t, root)
	t.Chdir(t.TempDir())
	t.Setenv("ZIG_GLOBAL_CACHE_DIR", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", "cache")
	directory := filepath.Join("cache", "zig", "p", "known_folders-0.0.0-Fy-PJiDLAAB98m3uYUzatrTb2mO2fpvwx2zpSroEtfbO")
	os.MkdirAll(directory, 0o755)
	os.WriteFile(filepath.Join(directory, "build.zig.zon"), []byte(`.{ .name = .known_folders, .version = "0.0.0", .paths = .{""} }`), 0o644)
	r := newResolver(root, files)
	r.Dependencies(knownFolders)
	if r.Installed(knownFolders) {
		t.Error("a package under a relative XDG_CACHE_HOME was taken as fetched")
	}
}
