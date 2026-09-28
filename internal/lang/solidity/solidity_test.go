package solidity

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture has three projects. foundry/ is a Foundry project: foundry.toml
// remaps @oz/ into the openzeppelin-contracts submodule, util/ by context
// (src/ files get src/utils/, test/ files test/helpers/) and gone/ to a
// directory that does not exist; a second profile adds ci-only/. Its
// remappings.txt (with a comment and a blank line) maps forge-std/ into the
// checked-out forge-std submodule, ds-test/ into forge-std's own submodule,
// local/ into src/ and hh/ into node_modules. .gitmodules lists forge-std (a
// branch), openzeppelin-contracts (an scp-style URL) and a private vault (a
// branch); vault/ has no remapping, so Foundry's inferred vault/=lib/vault/
// applies. [dependencies] declares Soldeer packages (an exact version, a git
// source with a rev, a range and a private git source on a branch) and
// soldeer.lock pins two of them. lib/, dependencies/, out/ and cache/ hold
// files that are not the project's. src/Counter.sol uses every import form,
// hides fake imports in NatSpec, a block comment, a string and an assembly
// block, and declares every kind of member. hardhat/ is a Hardhat project
// whose package.json and package-lock.json declare and pin
// @openzeppelin/contracts, hardhat and solady; artifacts/ and
// typechain-types/ are Hardhat's. plain/ has no manifest at all.

var (
	forgeStd = lang.Target{Ecosystem: ecoGit, Package: "github.com/foundry-rs/forge-std", Version: "v1", Floating: true}
	oz       = lang.Target{Ecosystem: ecoGit, Package: "github.com/openzeppelin/openzeppelin-contracts", Floating: true}
	dsTest   = lang.Target{Ecosystem: ecoGit, Package: "github.com/dapphub/ds-test", Floating: true}
	vault    = lang.Target{Ecosystem: ecoGit, Package: "git.example.com/team/vault", Version: "develop", Floating: true,
		Origin: "https://git.example.com/team/vault.git"}
	ozSoldeer = lang.Target{Ecosystem: ecoSoldeer, Package: "@openzeppelin-contracts", Version: "5.0.2", Pinned: true}
	solady    = lang.Target{Ecosystem: ecoSoldeer, Package: "solady", Version: "0123456789abcdef0123456789abcdef01234567",
		Requested: "0.0.200", Pinned: true}
	ozNPM = lang.Target{Ecosystem: ecoNPM, Package: "@openzeppelin/contracts", Version: "5.0.2", Requested: "^5.0.0", Pinned: true}
)

func local(p string) lang.Target { return lang.Target{Local: p} }

func unresolved(eco, name string) lang.Target {
	return lang.Target{Ecosystem: eco, Package: name, Unresolved: true}
}

func analyze(t *testing.T) map[string]*lang.FileResult {
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-SOLIDITY-002, REQ-SOLIDITY-004, REQ-SOLIDITY-005, REQ-SOLIDITY-006, REQ-SOLIDITY-007
func TestFoundryImports(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["foundry/src/Counter.sol"], map[string]lang.Target{
		"./utils/Math.sol":          local("foundry/src/utils/Math.sol"),
		"local/Local.sol":           local("foundry/src/local/Local.sol"),
		"util/Math.sol":             local("foundry/src/utils/Math.sol"), // the src/ context
		"forge-std/Test.sol":        forgeStd,
		"@oz/token/ERC20/ERC20.sol": oz,
		"@openzeppelin-contracts-5.0.2/token/ERC20/ERC20.sol": ozSoldeer, // Soldeer's generated remapping
		"solady-0.0.200/src/utils/LibString.sol":              solady,
		"ds-test/test.sol":                                    dsTest, // forge-std's own submodule, from its .gitmodules
		"vault/Vault.sol":                                     vault,  // Foundry's inferred remapping of lib/vault
		"gone/Nothing.sol":                                    {},     // remapped to a file that does not exist
		"../../outside.sol":                                   {},
		"unknown-lib/Unknown.sol":                             unresolved(ecoGit, "unknown-lib"),
		"@scope/pkg/Scoped.sol":                               unresolved(ecoNPM, "@scope/pkg"),
		"src/local/Local.sol":                                 local("foundry/src/local/Local.sol"), // from the project's root
	})
	langtest.CheckImports(t, res["foundry/test/Counter.t.sol"], map[string]lang.Target{
		"forge-std/Test.sol":    forgeStd,
		"util/Helper.sol":       local("foundry/test/helpers/Helper.sol"), // the test/ context
		"../src/Counter.sol":    local("foundry/src/Counter.sol"),
		"solmate/test/Mock.sol": unresolved(ecoGit, "solmate"),
	})
	// A missing ';' does not lose the next import; a missing file of one of
	// the project's directories is dropped, not a package.
	langtest.CheckImports(t, res["foundry/script/Deploy.s.sol"], map[string]lang.Target{
		"src/Counter.sol":    local("foundry/src/Counter.sol"),
		"src/utils/Math.sol": local("foundry/src/utils/Math.sol"),
		"script/Missing.sol": {},
	})
}

// Verifies: REQ-SOLIDITY-004, REQ-SOLIDITY-008
func TestHardhatImports(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["hardhat/contracts/Token.sol"], map[string]lang.Target{
		"@openzeppelin/contracts/token/ERC20/ERC20.sol": ozNPM,
		"hardhat/console.sol":                           {Ecosystem: ecoNPM, Package: "hardhat", Version: "2.22.3", Requested: "^2.22.0", Pinned: true},
		"solady/src/utils/LibString.sol":                {Ecosystem: ecoNPM, Package: "solady", Version: "0.0.228", Requested: "0.0.228", Pinned: true},
		"./lib/Lib.sol":                                 local("hardhat/contracts/lib/Lib.sol"),
		"contracts/lib/Lib.sol":                         local("hardhat/contracts/lib/Lib.sol"),
		"@unknown/pkg/Missing.sol":                      unresolved(ecoNPM, "@unknown/pkg"),
		"openzeppelin-solidity/contracts/Old.sol":       unresolved(ecoNPM, "openzeppelin-solidity"),
	})
	langtest.CheckImports(t, res["plain/A.sol"], map[string]lang.Target{
		"./B.sol": local("plain/B.sol"),
		"openzeppelin-solidity/contracts/math/SafeMath.sol": unresolved(ecoNPM, "openzeppelin-solidity"),
	})
}

// Verifies: REQ-SOLIDITY-005, REQ-SOLIDITY-006, REQ-SOLIDITY-007
func TestManifests(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["foundry/foundry.toml"], map[string]lang.Target{
		"@oz/=lib/openzeppelin-contracts/contracts/": oz,
		"src/:util/=src/utils/":                      local("foundry/src/utils"),
		"test/:util/=test/helpers/":                  local("foundry/test/helpers"),
		"gone/=src/missing/":                         {},
		"ci-only/=lib/ci-only/":                      unresolved(ecoGit, "ci-only"),
		"@openzeppelin-contracts":                    ozSoldeer,
		"solady":                                     solady,
		"@uniswap-v3":                                {Ecosystem: ecoSoldeer, Package: "@uniswap-v3", Version: "^1.0.0", Floating: true},
		"internal-kit": {Ecosystem: ecoSoldeer, Package: "internal-kit", Version: "main", Floating: true,
			Origin: "https://git.example.com/team/kit.git"},
	})
	langtest.CheckImports(t, res["foundry/remappings.txt"], map[string]lang.Target{
		"forge-std/=lib/forge-std/src/":           forgeStd,
		"ds-test/=lib/forge-std/lib/ds-test/src/": dsTest,
		"local/=src/local/":                       local("foundry/src/local"),
		"hh/=node_modules/hardhat/":               unresolved(ecoNPM, "hardhat"),
	})
	langtest.CheckImports(t, res["foundry/soldeer.lock"], map[string]lang.Target{
		"@openzeppelin-contracts": ozSoldeer,
		"solady":                  solady,
	})
	langtest.CheckImports(t, res["foundry/.gitmodules"], map[string]lang.Target{
		"lib/forge-std":              forgeStd,
		"lib/openzeppelin-contracts": oz,
		"lib/vault":                  vault,
	})
	lines := map[string]int{}
	for _, im := range res["foundry/foundry.toml"].Imports {
		lines[im.Spec] = im.Line
	}
	if lines["test/:util/=test/helpers/"] != 8 || lines["solady"] != 17 || lines["ci-only/=lib/ci-only/"] != 13 {
		t.Errorf("lines %v", lines)
	}
}

// Verifies: REQ-SOLIDITY-003
func TestSymbols(t *testing.T) {
	res := analyze(t)
	langtest.CheckSymbols(t, res["foundry/src/Counter.sol"], map[string]string{
		"Counter": "class", "Counter.MAX": "const", "Counter.Incremented": "event", "Counter.TooLarge": "error",
		"Counter.small": "modifier", "Counter.constructor": "method", "Counter.increment": "method",
		"Counter.increment@47": "method", "Counter.hash": "method", "Counter.receive": "method",
		"Counter.fallback": "method",
	})
	langtest.CheckSymbols(t, res["foundry/src/utils/Math.sol"], map[string]string{
		"Rounding": "enum", "WAD": "const", "Price": "type", "MathOverflow": "error", "Computed": "event",
		"Pair": "struct", "mulWad": "func", "Math": "class", "Math.Frac": "struct", "Math.Kind": "enum",
		"Math.ONE": "const", "Math.max": "method",
	})
	langtest.CheckSymbols(t, res["foundry/src/local/Local.sol"], map[string]string{
		"ILocal": "interface", "ILocal.get": "method", "ILocal.Got": "event",
	})
	langtest.CheckSymbols(t, res["foundry/test/helpers/Helper.sol"], map[string]string{
		"Helper": "class", "Helper.help": "method",
	})
	lines := map[string]int{}
	for _, s := range res["foundry/src/utils/Math.sol"].Symbols {
		lines[s.Name] = s.Line
	}
	if lines["Math.max"] != 38 || lines["Pair"] != 17 {
		t.Errorf("lines %v", lines)
	}
}

// Verifies: REQ-SOLIDITY-002
func TestImportForms(t *testing.T) {
	for src, want := range map[string][]string{
		`import "a.sol";`:                                     {"a.sol"},
		`import "a.sol" as A;`:                                {"a.sol"},
		`import * as A from "a.sol";`:                         {"a.sol"},
		`import {A, B as C} from 'a.sol';`:                    {"a.sol"},
		"import {\n  A,\n  B\n} from \"a.sol\";":              {"a.sol"},
		`import "a.sol"; import "a.sol";`:                     {"a.sol"},
		`import "a\"b.sol";`:                                  {`a\"b.sol`},
		`import unicode"a.sol";`:                              {"a.sol"},
		`pragma solidity ^0.8.20; import "a.sol";`:            {"a.sol"},
		`// import "x.sol";` + "\n" + `import "a.sol";`:       {"a.sol"},
		`/// @dev import "x.sol";` + "\n" + `import "a.sol";`: {"a.sol"},
		`/* import "x.sol"; */ import "a.sol";`:               {"a.sol"},
		`contract C { string s = "import \"x.sol\";"; }`:      nil,
		`contract C { function f() { assembly { let x := "import" } } } import "a.sol";`: {"a.sol"},
		`import {A} from "a.sol"` + "\n" + `import "b.sol";`:                             {"a.sol", "b.sol"},
		`import "unterminated`: {"unterminated"},
	} {
		var got []string
		for _, im := range scanSource([]byte(src)).Imports {
			got = append(got, im.Module)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %q, want %q", src, got, want)
		}
	}
}

// Verifies: REQ-SOLIDITY-001
func TestClaims(t *testing.T) {
	files := langtest.Files(t, "testdata/repo")
	var claimed []string
	for _, f := range lang.Claimed(Plugin{}, files) {
		claimed = append(claimed, f.Path)
	}
	sort.Strings(claimed)
	want := []string{
		"foundry/.gitmodules", "foundry/foundry.toml", "foundry/remappings.txt", "foundry/script/Deploy.s.sol",
		"foundry/soldeer.lock", "foundry/src/Counter.sol", "foundry/src/local/Local.sol", "foundry/src/utils/Math.sol",
		"foundry/test/Counter.t.sol", "foundry/test/helpers/Helper.sol", "hardhat/contracts/Token.sol",
		"hardhat/contracts/lib/Lib.sol", "plain/A.sol", "plain/B.sol",
	}
	if !reflect.DeepEqual(claimed, want) {
		t.Errorf("claimed %v\nwant %v", claimed, want)
	}
	for _, f := range files {
		if c := (Plugin{}).Class(f); c != "" && c != filepath.Base(f.Path) {
			t.Errorf("%s: class %q", f.Path, c)
		}
	}
}

// Verifies: REQ-SOLIDITY-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = true
		if e.Std {
			t.Errorf("%s is not a standard library", e.ID)
		}
	}
	for _, id := range []string{ecoSoldeer, ecoGit, ecoNPM} {
		if !ids[id] {
			t.Errorf("%s missing", id)
		}
	}
}

// A checked-out submodule's own submodules are its dependencies; npm
// packages are answered by the lock files, as JavaScript's are.
//
// Verifies: REQ-SOLIDITY-006, REQ-SOLIDITY-008
func TestDependencies(t *testing.T) {
	root, _ := filepath.Abs("testdata/repo")
	r := newResolver(root, langtest.Files(t, "testdata/repo"))
	got := r.Dependencies(forgeStd)
	if len(got) != 1 || got[0] != dsTest {
		t.Errorf("forge-std depends on %+v", got)
	}
	if !r.Installed(forgeStd) || r.Installed(ozNPM) {
		t.Error("Installed")
	}
	if got := r.Dependencies(oz); len(got) != 0 {
		t.Errorf("a submodule that is not checked out: %+v", got)
	}
}

// git records the commit of each submodule (a gitlink); it pins the
// submodule, with the .gitmodules branch as requested.
//
// Verifies: REQ-SOLIDITY-006, REQ-SOLIDITY-011
func TestSubmoduleCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	for p, c := range map[string]string{
		"foundry.toml": "[profile.default]\n",
		".gitmodules":  "[submodule \"lib/forge-std\"]\n\tpath = lib/forge-std\n\turl = https://github.com/foundry-rs/forge-std\n\tbranch = v1\n[submodule \"lib/solmate\"]\n\tpath = lib/solmate\n\turl = https://github.com/transmissions11/solmate\n",
		"src/A.sol":    "import \"forge-std/Test.sol\";\nimport \"solmate/src/tokens/ERC20.sol\";\n",
	} {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		if err := os.WriteFile(abs, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const sha = "1801b0541f4fda118a10798fd3486bb7051c5dd6"
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "foundry.toml", ".gitmodules", "src/A.sol"},
		{"update-index", "--add", "--cacheinfo", "160000," + sha + ",lib/forge-std"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := lang.Analyze(context.Background(), Plugin{}, root, files)
	if err != nil {
		t.Fatal(err)
	}
	pinned := lang.Target{Ecosystem: ecoGit, Package: "github.com/foundry-rs/forge-std", Version: sha, Requested: "v1", Pinned: true}
	floating := lang.Target{Ecosystem: ecoGit, Package: "github.com/transmissions11/solmate", Floating: true}
	langtest.CheckImports(t, res["src/A.sol"], map[string]lang.Target{
		"forge-std/Test.sol":           pinned,
		"solmate/src/tokens/ERC20.sol": floating,
	})
	langtest.CheckImports(t, res[".gitmodules"], map[string]lang.Target{
		"lib/forge-std": pinned,
		"lib/solmate":   floating,
	})
}

// Verifies: REQ-SOLIDITY-005
func TestRemap(t *testing.T) {
	p := &project{dir: "c", remaps: []remapping{
		{prefix: "a/", target: "lib/a/src/"},
		{prefix: "a/b/", target: "lib/ab/"},
		{context: "test/", prefix: "a/", target: "test/a/"},
		{prefix: "x", target: "lib/x/src"},
	}}
	for _, c := range []struct{ file, spec, want string }{
		{"c/src/F.sol", "a/F.sol", "c/lib/a/src/F.sol"},
		{"c/src/F.sol", "a/b/F.sol", "c/lib/ab/F.sol"},
		{"c/test/F.sol", "a/b/F.sol", "c/test/a/b/F.sol"}, // the context beats the longer prefix
		{"c/src/F.sol", "x/F.sol", "c/lib/x/src/F.sol"},
		{"c/src/F.sol", "y/F.sol", ""},
	} {
		got, _ := p.remap(c.file, c.spec)
		if got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.file, c.spec, got, c.want)
		}
	}
	for s, want := range map[string]remapping{
		"ctx:pre/=tgt/": {"ctx", "pre/", "tgt/"},
		" a = b ":       {"", "a", "b"},
	} {
		if got, ok := parseRemapping(s); !ok || got != want {
			t.Errorf("%q: %+v", s, got)
		}
	}
	for _, s := range []string{"", "=x", "nothing", ":=x"} {
		if _, ok := parseRemapping(s); ok {
			t.Errorf("%q parsed", s)
		}
	}
}

// Every prefix of every fixture file, and long runs of what opens a
// construct, go through the scanner and the manifest readers without a
// panic and in linear time.
//
// Verifies: REQ-SOLIDITY-010
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
		for i := 0; i <= len(src); i++ {
			readAll(src[:i])
		}
	}
	for _, unit := range []string{"{", "}", "(", ")", "[", "\"", "'", "\\", "/*", "//", "/", "unicode\"", "hex'",
		"import ", "import {", "import \"a\" ", "contract C ", "contract C {", "abstract ", "function f(", "function (",
		"modifier m(", "event E(", "error E(", "struct S {", "enum E {", "type T is ", "constant ", "assembly {",
		"library L { function f() {", "pragma ", "using L for ", "a:b=c\n", "[submodule \"x\"]\npath=", "[[dependencies]]\n",
		"= [", "é"} {
		src := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		readAll(src)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(src)/len(unit), d)
		}
	}
}

func readAll(src []byte) {
	scanSource(src)
	extractFoundry(src)
	extractRemappings(src)
	extractLock(src)
	extractGitmodules(src)
}
