package nim

import (
	"encoding/json"
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

// The fixture is a package "shop" (srcDir src, bin shop) whose shop.nimble
// requires packages by name, range, exact version, #commit, #head of a URL, a
// forge alias with a tag, a private git server, a file:// directory, inside a
// when branch, a feature and taskRequires; nimble.lock pins some of them (one
// only through a task, one only transitively) and nimbledeps/pkgs2 holds two
// installed packages. config.nims and nim.cfg add search paths, tests/ has its
// own config.nims. atlasapp/ is an Atlas project with atlas.lock and its
// checkouts in deps/. testdata/nimble-home is the nimble directory
// (NIMBLE_DIR): two versions of sdl2_nim (which ships the module sdl2), zippy,
// and a package no manifest names. testdata/nimrepo is a tiny copy of Nim's
// own repository layout (lib/system.nim, lib/pure, lib/std).
func TestMain(m *testing.M) {
	home, _ := filepath.Abs("testdata/nimble-home")
	os.Setenv("NIMBLE_DIR", home)
	os.Setenv("HOME", os.TempDir())
	os.Exit(m.Run())
}

var (
	chronos  = lang.Target{Ecosystem: ecoNimble, Package: "chronos", Version: "4.0.3", Pinned: true}
	jester   = lang.Target{Ecosystem: ecoNimble, Package: "jester", Version: ">= 0.5", Floating: true}
	widgets  = lang.Target{Ecosystem: ecoNimble, Package: "github.com/acme/nim-widgets", Version: "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567", Requested: "#head", Pinned: true}
	stew     = lang.Target{Ecosystem: ecoNimble, Package: "stew", Version: "0123abcd", Pinned: true}
	results  = lang.Target{Ecosystem: ecoNimble, Package: "results", Version: "0.4.0", Pinned: true}
	bearssl  = lang.Target{Ecosystem: ecoNimble, Package: "bearssl", Version: "0.2.1", Pinned: true}
	zippy    = lang.Target{Ecosystem: ecoNimble, Package: "zippy", Version: "^= 0.10", Floating: true}
	pixie    = lang.Target{Ecosystem: ecoNimble, Package: "github.com/treeform/pixie", Version: "v5.0.0"}
	vault    = lang.Target{Ecosystem: ecoNimble, Package: "git.acme.internal/shop/vault", Version: "abcdef0123456789abcdef0123456789abcdef01", Pinned: true, Origin: "https://git.acme.internal/shop/vault.git"}
	sdl2     = lang.Target{Ecosystem: ecoNimble, Package: "sdl2_nim", Floating: true}
	winim    = lang.Target{Ecosystem: ecoNimble, Package: "winim", Version: ">= 3.9", Floating: true}
	unittest = lang.Target{Ecosystem: ecoNimble, Package: "unittest2", Version: "0.2.2", Requested: ">= 0.2", Pinned: true}
	openssl  = lang.Target{Ecosystem: ecoNimble, Package: "openssl_evp", Floating: true}
	httputil = lang.Target{Ecosystem: ecoNimble, Package: "httputils", Version: "0.3.0"}
	malebolg = lang.Target{Ecosystem: ecoNimble, Package: "malebolgia", Version: "1.3.2", Requested: ">= 1.3", Pinned: true}
	sat      = lang.Target{Ecosystem: ecoNimble, Package: "github.com/nim-lang/sat", Version: "faf1617f44d7632ee9601ebc13887644925dcc01", Pinned: true}
)

func std(name string) lang.Target { return lang.Target{Ecosystem: ecoStd, Package: name} }

// Verifies: REQ-NIM-002, REQ-NIM-004, REQ-NIM-007, REQ-NIM-008
func TestImports(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["src/shop.nim"], map[string]lang.Target{
		"std/os":            std("os"),
		"std/strutils":      std("strutils"),
		"tables":            std("tables"),
		"json":              std("json"),
		"pkg/jester":        jester,
		"chronos":           chronos, // installed in nimbledeps/, locked
		"chronos/asyncloop": chronos,
		"widgets/button":    widgets,  // a URL requirement's repository, locked
		"stew/byteutils":    stew,     // #commit
		"results":           results,  // == and locked
		"zippy":             zippy,    // installed in the nimble directory, a range
		"pixie":             pixie,    // gh: alias, #tag
		"vault/client":      vault,    // private git server
		"httputils":         httputil, // installed, required by nothing
		"sdl2":              sdl2,     // the nimble directory's sdl2_nim ships sdl2.nim
		"winim/lean":        winim,    // required in a when branch
		"./shop/cart":       {Local: "src/shop/cart.nim"},
		"shop/models/user":  {Local: "src/shop/models/user.nim"},
		"shop/private/util": {Local: "src/shop/private/util.nim"},
		"vlib":              {Local: "vendor/lib/vlib.nim"}, // config.nims --path
		"generated":         {Local: "gen/generated.nim"},   // switch("path", thisDir() / "gen")
		"helper":            {Local: "extra/helper.nim"},    // nim.cfg path =
		"missing/thing":     {Ecosystem: ecoNimble, Package: "missing", Unresolved: true},
		"./nothere":         {},
		"db_sqlite":         std("db_sqlite"), // left the library in Nim 2; db_connector is not required
		"std/sha1":          std("sha1"),
		"strutils":          std("strutils"),
		"sequtils":          std("sequtils"),
		"include shop/inc":  {Local: "src/shop/inc.nim"},
		"winlean":           std("winlean"), // every when branch is read
		"jsffi":             std("jsffi"),
		"posix":             std("posix"),
	})
	langtest.CheckImports(t, res["tests/tshop.nim"], map[string]lang.Target{
		"unittest2":        unittest,                // taskRequires, locked by the task
		"shop":             {Local: "src/shop.nim"}, // the package's srcDir
		"../src/shop/cart": {Local: "src/shop/cart.nim"},
		"shop/cart":        {Local: "src/shop/cart.nim"},
		"localdep":         {Local: "libs/localdep/localdep.nim"}, // a file:// requirement in the repository
	})
	langtest.CheckImports(t, res["atlasapp/app.nim"], map[string]lang.Target{
		"malebolgia":         malebolg, // Atlas's checkout in deps/ (srcDir src), atlas.lock
		"malebolgia/lockers": malebolg,
		"sat/sat":            sat, // a URL requirement; atlas.lock pins the commit
	})
}

// Verifies: REQ-NIM-004, REQ-NIM-005, REQ-NIM-006, REQ-NIM-011
func TestManifests(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["shop.nimble"], map[string]lang.Target{
		"nim >= 2.0.0":  {}, // the compiler
		"jester >= 0.5": jester,
		"chronos":       chronos,
		"https://github.com/acme/nim-widgets.git#head": widgets,
		"stew#0123abcd":            stew,
		"results == 0.4.0":         results,
		"zippy ^= 0.10":            zippy,
		"gh:treeform/pixie#v5.0.0": pixie,
		"https://git.acme.internal/shop/vault.git#abcdef0123456789abcdef0123456789abcdef01": vault,
		"sdl2_nim":               sdl2,
		"file://libs/localdep":   {Local: "libs/localdep"},
		"winim >= 3.9":           winim,
		"test: unittest2 >= 0.2": unittest,
		"openssl_evp":            openssl,
		"bin: shop":              {Local: "src/shop.nim"},
	})
	langtest.CheckImports(t, res["nimble.lock"], map[string]lang.Target{
		"results":   results,
		"bearssl":   bearssl, // locked, required by chronos only
		"chronos":   chronos,
		"widgets":   widgets,
		"unittest2": unittest,
	})
	langtest.CheckImports(t, res["atlasapp/atlasapp.nimble"], map[string]lang.Target{
		"malebolgia >= 1.3":               malebolg,
		"https://github.com/nim-lang/sat": sat,
	})
	langtest.CheckImports(t, res["atlasapp/atlas.lock"], map[string]lang.Target{"malebolgia": malebolg, "sat": sat})
	langtest.CheckImports(t, res["atlasapp/nim.cfg"], map[string]lang.Target{
		"--path:deps/malebolgia/src": malebolg, // Atlas's generated paths are its packages
		"--path:deps/sat":            sat,
	})
	langtest.CheckImports(t, res["config.nims"], map[string]lang.Target{
		"include nimble.paths":   {}, // generated by nimble setup, not in the repository
		"--path:vendor/lib":      {Local: "vendor/lib"},
		"--path:$projectDir/gen": {Local: "gen"},
	})
	langtest.CheckImports(t, res["nim.cfg"], map[string]lang.Target{
		"--path:extra":         {Local: "extra"},
		"--path:$home/winlibs": {},
	})
	langtest.CheckImports(t, res["tests/config.nims"], map[string]lang.Target{"--path:../src": {Local: "src"}})
	langtest.CheckSymbols(t, res["shop.nimble"], map[string]string{"shop": "package", "test": "task"})
	langtest.CheckSymbols(t, res["libs/localdep/localdep.nimble"], map[string]string{"localdep": "package"})
}

// In Nim's own repository the standard library is its lib/ directory.
//
// Verifies: REQ-NIM-008
func TestStdlibRepository(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/nimrepo")
	langtest.CheckImports(t, res["compiler/ast.nim"], map[string]lang.Target{
		"std/os":      {Local: "lib/pure/os.nim"},
		"std/syncio":  {Local: "lib/std/syncio.nim"},
		"strutils":    {Local: "lib/pure/strutils.nim"},
		"tables":      std("tables"), // not in this copy
		"std/nothere": std("nothere"),
	})
	langtest.CheckImports(t, res["lib/system.nim"], map[string]lang.Target{"include system/basic_types": {Local: "lib/system/basic_types.nim"}})
	langtest.CheckImports(t, res["tools/tool.nim"], map[string]lang.Target{"compiler/ast": {Local: "compiler/ast.nim"}}) // path = "$nim"
}

// Verifies: REQ-NIM-003
func TestSymbols(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, res["src/shop.nim"], map[string]string{
		"s": "var", "r": "var", "t": "var", "f": "var", "c": "var", "n": "var",
		"native":     "func", // in a when branch
		"Money":      "type",
		"Item":       "class",
		"Kind":       "enum",
		"Shape":      "class",
		"Comparable": "interface",
		"Callback":   "type",
		"Pair":       "type",
		"Cart":       "class",
		"Version":    "const",
		"A":          "const",
		"B":          "const",
		"counter":    "var",
		"grand":      "var",
		"x":          "var",
		"y":          "var",
		"tls":        "var",
		"total":      "func",
		"$":          "func",
		"draw":       "method",
		"items":      "iterator",
		"toInt":      "converter",
		"check":      "template",
		"route":      "macro",
		"forward":    "func",
		"main":       "func", // when isMainModule
	})
}

// What looks like an import inside comments, doc comments, nested block
// comments, strings of every kind, character literals and routine bodies is not
// read.
//
// Verifies: REQ-NIM-002, REQ-NIM-010
func TestLiteralsHideCode(t *testing.T) {
	src := `## import docfake
#[ import blockfake #[ nested ]# import stillfake ]#
##[ import docblockfake ]##
let a = "import strfake \" still"
let b = r"import rawfake "" still"
let c = """import triplefake
import triplefake2
"""
let d = sql"import genfake"
let e = '"'
let g = '\''
let h = 1'u8
proc p() =
  import bodyfake
type T = object
  import: int
import real1, real2 # import commentfake
import "real3.nim"; import real4
`
	s := scanSource([]byte(src))
	var got []string
	for _, im := range s.imports {
		got = append(got, im.Spec)
	}
	if want := []string{"real1", "real2", "real3", "real4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("imports: %v, want %v", got, want)
	}
	if line := s.imports[0].Line; line != 17 {
		t.Errorf("line: %d", line)
	}
}

// Verifies: REQ-NIM-002
func TestImportForms(t *testing.T) {
	src := `import
  std / [os, strutils],
  compiler/[ast, idents as id],
  ../up/x,
  $lib/pure/os,
  a/b/[c, d/e]
import std/os as myos, other except foo, bar
from pkg/p import nil
from ./q as qq import z
include incl1, "incl2.nim"
export std/tables
when defined(js): import jsffi
`
	var got []string
	for _, im := range scanSource([]byte(src)).imports {
		got = append(got, im.Name+" "+im.Module)
	}
	want := []string{
		"import std/os", "import std/strutils", "import compiler/ast", "import compiler/idents", "import ../up/x",
		"import $lib/pure/os", "import a/b/c", "import a/b/d/e", "import other", "import pkg/p", "import ./q",
		"include incl1", "include incl2", "import jsffi",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// Verifies: REQ-NIM-005
func TestRequirements(t *testing.T) {
	src := `requires "a >= 1", "b"
requires("c#head")
requires @["d", "e"]
requires "f",
  "g ^= 1.2"
when defined(linux): requires "h"
taskRequires "bench", "i", "j"
feature "x":
  requires "k[extra] == 1.0"
dev:
  requires "l"
echo "requires m"
let requires = 1
proc f() = exec "requires n"
`
	var got []string
	for _, r := range scanSource([]byte(src)).requirements() {
		got = append(got, r.task+":"+r.text)
	}
	want := []string{":a >= 1", ":b", ":c#head", ":d", ":e", ":f", ":g ^= 1.2", ":h", "bench:i", "bench:j", ":k[extra] == 1.0", ":l"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	for _, c := range []struct {
		text, name, url, ver string
	}{
		{"jester >= 0.5", "jester", "", ">= 0.5"},
		{"pkg#abc123", "pkg", "", "#abc123"},
		{"pkg == 1.2.3", "pkg", "", "== 1.2.3"},
		{"k[ssl,dev] >= 1 & < 2", "k", "", ">= 1 & < 2"},
		{"https://github.com/x/y.git#head", "https://github.com/x/y.git", "https://github.com/x/y.git", "#head"},
		{"git@github.com:x/y", "git@github.com:x/y", "git@github.com:x/y", ""},
		{"gl:grp/proj", "gl:grp/proj", "https://gitlab.com/grp/proj", ""},
		{"pkg@#head", "pkg", "", "#head"},
	} {
		d, ok := parseDep(c.text, 1, "")
		if !ok || d.name != c.name || d.url != c.url || d.ver != c.ver {
			t.Errorf("%q: %+v", c.text, d)
		}
	}
	if d, _ := parseDep("https://github.com/status-im/nim-chronos.git", 1, ""); d.pkg() != "github.com/status-im/nim-chronos" {
		t.Errorf("pkg: %s", d.pkg())
	}
}

// Verifies: REQ-NIM-006
func TestPinRule(t *testing.T) {
	for ver, want := range map[string]lang.Target{
		"":                  {Floating: true},
		"== 1.2.3":          {Version: "1.2.3", Pinned: true},
		"==1.2":             {Version: "1.2", Pinned: true},
		"1.2.3":             {Version: "1.2.3", Pinned: true},
		"#abc123":           {Version: "abc123", Pinned: true},
		"#head":             {Version: "#head", Floating: true},
		"#main":             {Version: "#main", Floating: true},
		"#v1.2.0":           {Version: "v1.2.0"},
		"#123456":           {Version: "123456"}, // all digits: a version tag
		">= 0.5":            {Version: ">= 0.5", Floating: true},
		"^= 1.2":            {Version: "^= 1.2", Floating: true},
		"~= 1.2":            {Version: "~= 1.2", Floating: true},
		">= 1.0 & < 2.0":    {Version: ">= 1.0 & < 2.0", Floating: true},
		"== 1.0 & feature":  {Version: "== 1.0 & feature", Floating: true},
		"any":               {Floating: true},
		"#cafebabe00112233": {Version: "cafebabe00112233", Pinned: true},
	} {
		var got lang.Target
		pinRule(&got, ver)
		if got != want {
			t.Errorf("%q: got %+v, want %+v", ver, got, want)
		}
	}
}

// Verifies: REQ-NIM-001
func TestClaims(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	for _, p := range []string{
		"nimbledeps/pkgs2/chronos-4.0.3-9f8e7d6c5b4a39281706f5e4d3c2b1a098765432/chronos.nim",
		"nimbledeps/pkgs2/chronos-4.0.3-9f8e7d6c5b4a39281706f5e4d3c2b1a098765432/chronos.nimble",
		"atlasapp/deps/malebolgia/src/malebolgia.nim", "atlasapp/deps/sat/sat.nimble", "nimcache/stale.nim",
	} {
		if res[p] != nil {
			t.Errorf("%s: analyzed", p)
		}
	}
	for _, p := range []string{"src/shop.nim", "config.nims", "nim.cfg", "shop.nimble", "nimble.lock", "atlasapp/atlas.lock", "atlasapp/nim.cfg"} {
		if res[p] == nil {
			t.Errorf("%s: not analyzed", p)
		}
	}
	for p, want := range map[string]string{
		"shop.nimble": "nimble:shop", "a/nimble.lock": classLock, "atlas.lock": classAtlas, "nim.cfg": classCfg,
		"app.nim.cfg": classCfg, "setup.cfg": "", "src/x.nim": "", "config.nims": "", ".nimble": "",
	} {
		if got := (Plugin{}).Class(&scan.File{Path: p}); got != want {
			t.Errorf("Class(%s) = %q, want %q", p, got, want)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "setup.cfg"}) || !(Plugin{}).Claims(&scan.File{Path: "deps/x.nim"}) {
		t.Error("claims")
	}
}

// Verifies: REQ-NIM-006, REQ-NIM-007
func TestDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"), os.Getenv)
	if got, want := r.Dependencies(chronos), []lang.Target{results, stew, bearssl}; !reflect.DeepEqual(got, want) {
		t.Errorf("chronos: got %+v, want %+v", got, want)
	}
	if r.Installed(chronos) {
		t.Error("chronos: its dependencies come from the lock")
	}
	// httputils is installed, not locked: its .nimble file's requirements.
	if got, want := r.Dependencies(httputil), []lang.Target{stew}; !reflect.DeepEqual(got, want) || !r.Installed(httputil) {
		t.Errorf("httputils: got %+v, want %+v", got, want)
	}
	if deps := r.Dependencies(jester); deps != nil || r.Installed(jester) {
		t.Errorf("jester: %v", deps)
	}
	if deps := r.Dependencies(std("os")); deps != nil {
		t.Errorf("std: %v", deps)
	}
}

// The nimble directory's packages are read only when a manifest names them,
// the newest version of each.
//
// Verifies: REQ-NIM-007
func TestNimbleDirectory(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"), os.Getenv)
	got := map[string]string{}
	for _, ip := range r.global {
		got[ip.name] = ip.version
	}
	if want := map[string]string{"sdl2_nim": "2.0.14.3", "zippy": "0.10.4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	empty := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"), func(string) string { return "" })
	if len(empty.global) != 0 {
		t.Error("no nimble directory")
	}
	for base, want := range map[string][3]string{
		"chronos-4.0.3-9f8e7d6c5b4a39281706f5e4d3c2b1a098765432": {"chronos", "4.0.3", "true"},
		"json_serialization-0.2.9-abc":                           {"json_serialization", "0.2.9", "true"},
		"stew-#head-abc":                                         {"stew", "#head", "true"},
		"nope":                                                   {"", "", "false"},
	} {
		name, version, ok := pkgDir(base, true)
		if name != want[0] || version != want[1] || (ok != (want[2] == "true")) {
			t.Errorf("%s: %s %s %v", base, name, version, ok)
		}
	}
}

// An Atlas checkout's repository comes from its .git/config.
//
// Verifies: REQ-NIM-007
func TestAtlasCheckoutOrigin(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "nim-chronos")
	if err := os.MkdirAll(filepath.Join(pkg, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(pkg, "chronos.nimble"), []byte("version = \"4.0.0\"\n"), 0o644)
	os.WriteFile(filepath.Join(pkg, "chronos.nim"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(pkg, ".git", "config"), []byte("[remote \"origin\"]\n\turl = https://github.com/status-im/nim-chronos\n"), 0o644)
	got := readAtlas(dir)
	if len(got) != 1 || got[0].name != "chronos" || got[0].url != "https://github.com/status-im/nim-chronos" || !got[0].modules["chronos"] {
		t.Fatalf("%+v", got)
	}
	if !names(got[0], "github.com/status-im/nim-chronos") {
		t.Error("names")
	}
}

// nimble.paths (written by nimble setup) names the installed packages' source
// directories.
//
// Verifies: REQ-NIM-007
func TestNimblePaths(t *testing.T) {
	home, _ := filepath.Abs("testdata/nimble-home/pkgs2/sdl2_nim-2.0.14.3-1111111111111111111111111111111111111111")
	got := fromPaths(readCfg([]byte("--noNimblePath\n--path:\"" + filepath.ToSlash(home) + "\"\n--path:\"/elsewhere/x\"\n")))
	if len(got) != 1 || got[0].name != "sdl2_nim" || got[0].version != "2.0.14.3" || !got[0].modules["sdl2"] {
		t.Fatalf("%+v", got)
	}
}

// Every prefix of every fixture file, and long runs of what opens something,
// are read without a panic and in linear time.
//
// Verifies: REQ-NIM-010
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
		f := &scan.File{Path: filepath.ToSlash(p)}
		for i := 0; i <= len(src); i++ {
			if _, err := (Plugin{}).Extract(f, src[:i]); err != nil {
				t.Fatal(err)
			}
			s := scanSource(src[:i])
			s.requirements()
			s.paths()
			readNimbleLock(src[:i])
			readAtlasLock(src[:i])
			readCfg(src[:i])
		}
	}
	for _, unit := range []string{"(", "[", "{", ")", "]", "}", "\"", "\"\"\"", "r\"", "'", "`", "#[", "]#", "##[", "\\",
		"import ", "import a, ", "import a/[", "from x import ", "include ", "type\n  ", "type\n  A = object\n", "const\n  ",
		"when x:\n", "when x: ", "else:\n", "proc f() =\n", "  ", "\n  ", "requires \"a\", ", "taskRequires ",
		"--path:", "switch(\"path\", ", "var a, ", "x = @[\"", "é", ";", "1'", "{\"packages\": {"} {
		src := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		s := scanSource(src)
		s.requirements()
		s.paths()
		readNimbleLock(src)
		readCfg(src)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(src)/len(unit), d)
		}
	}
}

// Verifies: REQ-NIM-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids[ecoStd]; !ok || !std || len(ids) != 2 || ids[ecoNimble] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for _, root := range []string{"testdata/repo", "testdata/nimrepo"} {
		for f, r := range langtest.Analyze(t, Plugin{}, root) {
			for _, im := range r.Imports {
				if e := im.Target.Ecosystem; e != "" && e != ecoNimble && e != ecoStd {
					t.Errorf("%s: %s -> %s", f, im.Spec, e)
				}
			}
		}
	}
}

// nimble.develop develops foo (and, through the develop file it includes, bar)
// from directories of the repository: their requirements and modules resolve
// there. A path outside the repository and a garbage or missing develop file
// leave the packages as required.
//
// Verifies: REQ-NIM-005, REQ-NIM-007
func TestDevelop(t *testing.T) {
	files := map[string]string{
		"app/app.nimble": "requires \"foo >= 1.0\"\nrequires \"https://github.com/acme/nim-bar\"\nrequires \"baz\"\nrequires \"qux\"\n",
		"app/nimble.develop": `{"version": 1, "includes": ["../shared.develop", "/nowhere/x.develop", "../qux.develop"],
			"dependencies": ["../foo", "../../outside/baz"]}`,
		"shared.develop":       `{"version": 1, "dependencies": ["bar"]}`,
		"app/main.nim":         "import foo, foo/util, bar, baz\n",
		"foo/foo.nimble":       "srcDir = \"src\"\n",
		"foo/src/foo.nim":      "proc f*() = discard\n",
		"foo/src/foo/util.nim": "proc u*() = discard\n",
		"bar/bar.nimble":       "",
		"bar/bar.nim":          "proc b*() = discard\n",
	}
	baz := lang.Target{Ecosystem: ecoNimble, Package: "baz", Floating: true}
	root := langtest.Write(t, files)
	// An absolute path inside the repository develops qux.
	for p, content := range map[string]string{"qux/qux.nimble": "", "qux/qux.nim": ""} {
		if err := os.MkdirAll(filepath.Join(root, "qux"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	qux, _ := json.Marshal(map[string][]string{"dependencies": {filepath.Join(root, "qux")}})
	if err := os.WriteFile(filepath.Join(root, "qux.develop"), qux, 0o644); err != nil {
		t.Fatal(err)
	}
	res := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, res["app/main.nim"], map[string]lang.Target{
		"foo":      {Local: "foo/src/foo.nim"},
		"foo/util": {Local: "foo/src/foo/util.nim"},
		"bar":      {Local: "bar/bar.nim"},
		"baz":      baz,
	})
	langtest.CheckImports(t, res["app/app.nimble"], map[string]lang.Target{
		"foo >= 1.0":                      {Local: "foo"},
		"https://github.com/acme/nim-bar": {Local: "bar"},
		"baz":                             baz,
		"qux":                             {Local: "qux"},
	})
	for _, garbage := range []string{"not json {", `{"dependencies": 3}`, ""} {
		files["app/nimble.develop"] = garbage
		files["shared.develop"] = garbage
		if garbage == "" {
			delete(files, "app/nimble.develop")
		}
		res := langtest.Analyze(t, Plugin{}, langtest.Write(t, files))
		langtest.CheckImports(t, res["app/main.nim"], map[string]lang.Target{
			"foo":      {Ecosystem: ecoNimble, Package: "foo", Version: ">= 1.0", Floating: true},
			"foo/util": {Ecosystem: ecoNimble, Package: "foo", Version: ">= 1.0", Floating: true},
			"bar":      {Ecosystem: ecoNimble, Package: "github.com/acme/nim-bar", Floating: true},
			"baz":      baz,
		})
	}
}
