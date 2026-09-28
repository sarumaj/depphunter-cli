package gleam

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

// The fixture is a Gleam package "shop" with src/, test/ and dev/ modules, a
// manifest.toml locking Hex packages, a git package and a local one, a path
// dependency in the repository (libs/inventory) and Erlang and JavaScript FFI
// files. tools/cli is a second package without a manifest, depending on shop by
// path, on Hex packages by exact, bare and ranged requirements and on git
// packages by commit and by branch.

var (
	stdlib = hex("gleam_stdlib", "0.40.0", ">= 0.34.0 and < 2.0.0")
	erlang = hex("gleam_erlang", "0.25.0", "~> 0.25")
	otp    = hex("gleam_otp", "0.10.0", ">= 0.10.0 and < 1.0.0")
	glitch = lang.Target{Ecosystem: ecosystemHex, Package: "glitch", Version: "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
		Requested: "main", Origin: "https://github.com/acme/glitch.git", Pinned: true}
)

func hex(name, version, requested string) lang.Target {
	return lang.Target{Ecosystem: ecosystemHex, Package: name, Version: version, Requested: requested, Pinned: true}
}

// Verifies: REQ-GLEAM-002, REQ-GLEAM-004, REQ-GLEAM-011
func TestModules(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["src/shop.gleam"], map[string]lang.Target{
		"gleam":                {}, // the prelude
		"gleam/dict":           stdlib,
		"gleam/erlang/process": erlang,
		"gleam/http/request":   hex("gleam_http", "3.6.0", ""),
		"gleam/io":             stdlib,
		"gleam/json":           hex("gleam_json", "1.0.1", "~> 1.0"),
		"gleam/list":           stdlib,
		"gleam/otp/actor":      otp,
		"glitch":               glitch,
		"inventory/stock":      {Local: "libs/inventory/src/inventory/stock.gleam"},
		"lustre/element":       hex("lustre", "4.3.0", ">= 4.0.0 and < 5.0.0"),
		"mist":                 hex("mist", "1.2.0", ""),
		"nothere/thing":        {Ecosystem: ecosystemHex, Package: "nothere", Unresolved: true},
		"gleam/community/ansi": {Ecosystem: ecosystemHex, Package: "gleam_community_ansi", Unresolved: true},
		"shop/cart":            {Local: "src/shop/cart.gleam"},
		"shop/missing":         {}, // its own namespace
	})
	langtest.CheckImports(t, results["test/shop_test.gleam"], map[string]lang.Target{
		"gleeunit":         hex("gleeunit", "1.2.0", "~> 1.0"),
		"gleeunit/should":  hex("gleeunit", "1.2.0", "~> 1.0"),
		"helpers":          {Local: "test/helpers.gleam"},
		"shop":             {Local: "src/shop.gleam"},
		"shop/cart":        {Local: "src/shop/cart.gleam"},
		"support/fixtures": {Local: "dev/support/fixtures.gleam"},
	})
	langtest.CheckImports(t, results["dev/support/fixtures.gleam"], map[string]lang.Target{"shop/cart": {Local: "src/shop/cart.gleam"}})
	// A path dependency's own modules; its gleam_stdlib is not locked.
	langtest.CheckImports(t, results["libs/inventory/src/inventory/stock.gleam"], map[string]lang.Target{
		"gleam/int": {Ecosystem: ecosystemHex, Package: "gleam_stdlib", Version: ">= 0.34.0 and < 2.0.0"},
		"inventory": {Local: "libs/inventory/src/inventory.gleam"},
	})
	pinnedStdlib := hex("gleam_stdlib", "0.40.0", "")
	langtest.CheckImports(t, results["tools/cli/src/cli.gleam"], map[string]lang.Target{
		"argv":          {Ecosystem: ecosystemHex, Package: "argv", Version: "~> 1.0"},
		"birl/duration": hex("birl", "1.7.1", ""),
		"gleam/io":      pinnedStdlib,
		"gleam/string":  pinnedStdlib,
		"shop":          {Local: "src/shop.gleam"},
		"shop/cart":     {Local: "src/shop/cart.gleam"},
	})
}

// Verifies: REQ-GLEAM-007, REQ-GLEAM-011
func TestExternals(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["src/shop/ffi.gleam"], map[string]lang.Target{
		"erlang:shop_ffi":                   {Local: "src/shop_ffi.erl"},
		"./ffi_helpers.mjs":                 {Local: "src/shop/ffi_helpers.mjs"},
		"erlang:os":                         {Ecosystem: ecosystemOTP, Package: "os"},
		"erlang:gleam_stdlib":               stdlib,
		"erlang:gleam@list":                 stdlib, // a compiled Gleam module
		"erlang:hpack":                      hex("hpack_erl", "0.3.0", ""),
		"erlang:gleam_otp_external":         otp,
		"erlang:Elixir.Jason":               {},
		"erlang:made_up_nif":                {Ecosystem: ecosystemHex, Package: "made_up_nif", Unresolved: true},
		"../../gleam_stdlib/gleam/list.mjs": stdlib, // out of the package: another package's output
		"react":                             {Ecosystem: ecosystemNPM, Package: "react", Version: "18.3.1", Pinned: true},
		"left-pad":                          {Ecosystem: ecosystemNPM, Package: "left-pad", Unresolved: true},
	})
	langtest.CheckImports(t, results["src/shop/cart.gleam"], map[string]lang.Target{
		"gleam/option":    stdlib,
		"../shop_ffi.mjs": {Local: "src/shop_ffi.mjs"},
	})
	// Gleam before 0.30: external fn with the module as a string.
	langtest.CheckImports(t, results["src/legacy.gleam"], map[string]lang.Target{
		"erlang:erlang":    {Ecosystem: ecosystemOTP, Package: "erlang"},
		"./legacy_ffi.mjs": {Local: "src/legacy_ffi.mjs"},
	})
}

// Verifies: REQ-GLEAM-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	for file, want := range map[string]map[string]string{
		"src/shop.gleam": {"Order": "type", "Order.Order": "constructor", "Order.Cancelled": "constructor",
			"Token": "type", "Token.Token": "constructor", "Stock": "type", "version": "const", "greeting": "const",
			"main": "func", "helper": "func"},
		"src/shop/cart.gleam": {"Cart": "type", "Cart.Cart": "constructor", "new": "func", "total": "func"},
		"src/legacy.gleam":    {"Pid": "type", "system_time": "func", "log": "func"},
		"gleam.toml":          {"shop": "package"},
	} {
		if got := langtest.Symbols(t, results[file]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: symbols %v, want %v", file, got, want)
		}
	}
	lines := map[string]int{}
	for _, s := range results["src/shop.gleam"].Symbols {
		lines[s.Name] = s.Line
	}
	if lines["Order"] != 23 || lines["Order.Cancelled"] != 26 || lines["main"] != 40 {
		t.Errorf("lines %v", lines)
	}
}

// Verifies: REQ-GLEAM-005, REQ-GLEAM-006, REQ-GLEAM-008
func TestManifests(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["gleam.toml"], map[string]lang.Target{
		"gleam_stdlib": stdlib,
		"gleam_erlang": erlang,
		"gleam_otp":    otp,
		"gleam_http":   hex("gleam_http", "3.6.0", ""),
		"gleam_json":   hex("gleam_json", "1.0.1", "~> 1.0"),
		"lustre":       hex("lustre", "4.3.0", ">= 4.0.0 and < 5.0.0"),
		"mist":         hex("mist", "1.2.0", ""),
		"inventory":    {Local: "libs/inventory/gleam.toml"},
		"glitch":       glitch,
		"gleeunit":     hex("gleeunit", "1.2.0", "~> 1.0"),
	})
	// Without a manifest: == and a bare version pin, ranges are kept as written,
	// a git commit pins and a branch floats, a path is an edge.
	langtest.CheckImports(t, results["tools/cli/gleam.toml"], map[string]lang.Target{
		"gleam_stdlib": hex("gleam_stdlib", "0.40.0", ""),
		"argv":         {Ecosystem: ecosystemHex, Package: "argv", Version: "~> 1.0"},
		"birl":         hex("birl", "1.7.1", ""),
		"shop":         {Local: "gleam.toml"},
		"ghost": {Ecosystem: ecosystemHex, Package: "ghost", Version: "0123456789abcdef0123456789abcdef01234567",
			Origin: "https://github.com/acme/ghost.git", Pinned: true},
		"branchy":  {Ecosystem: ecosystemHex, Package: "branchy", Version: "main", Origin: "https://github.com/acme/branchy.git", Floating: true},
		"gleeunit": {Ecosystem: ecosystemHex, Package: "gleeunit", Version: "~> 1.0"}, // inline dev-dependencies
	})
	langtest.CheckImports(t, results["manifest.toml"], map[string]lang.Target{
		"gleam_stdlib": stdlib,
		"gleam_erlang": erlang,
		"gleam_otp":    otp,
		"gleam_http":   hex("gleam_http", "3.6.0", ""),
		"gleam_json":   hex("gleam_json", "1.0.1", "~> 1.0"),
		"lustre":       hex("lustre", "4.3.0", ">= 4.0.0 and < 5.0.0"),
		"mist":         hex("mist", "1.2.0", ""),
		"inventory":    {Local: "libs/inventory/gleam.toml"},
		"glitch":       glitch,
		"gleeunit":     hex("gleeunit", "1.2.0", "~> 1.0"),
		"glisten":      hex("glisten", "5.0.0", ""),
		"hpack_erl":    hex("hpack_erl", "0.3.0", ""),
		"thoas":        hex("thoas", "1.2.1", ""),
	})
	for _, imported := range results["manifest.toml"].Imports {
		if imported.Spec == "mist" && imported.Line != 16 {
			t.Errorf("mist on line %d, want 16", imported.Line)
		}
	}
	for _, imported := range results["gleam.toml"].Imports {
		if imported.Spec == "gleeunit" && imported.Line != 20 {
			t.Errorf("gleeunit on line %d, want 20", imported.Line)
		}
	}
}

// Verifies: REQ-GLEAM-006
func TestTransitive(t *testing.T) {
	files := langtest.Files(t, "testdata/repo")
	r, err := Plugin{}.Resolver("testdata/repo", files)
	if err != nil {
		t.Fatal(err)
	}
	transitive := r.(lang.Transitive)
	got := transitive.Dependencies(lang.Target{Ecosystem: ecosystemHex, Package: "mist", Version: "1.2.0"})
	want := []lang.Target{
		hex("gleam_erlang", "0.25.0", ""), hex("gleam_http", "3.6.0", ""), hex("gleam_otp", "0.10.0", ""),
		hex("gleam_stdlib", "0.40.0", ""), hex("glisten", "5.0.0", ""), hex("hpack_erl", "0.3.0", ""),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mist depends on %+v, want %+v", got, want)
	}
	if got := transitive.Dependencies(lang.Target{Ecosystem: ecosystemHex, Package: "gleam_stdlib"}); len(got) != 0 {
		t.Errorf("gleam_stdlib depends on %+v", got)
	}
	if got := transitive.Dependencies(lang.Target{Ecosystem: "npm", Package: "mist"}); got != nil {
		t.Errorf("npm mist: %+v", got)
	}
}

// Verifies: REQ-GLEAM-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"src/app.gleam": true, "gleam.toml": true, "x/manifest.toml": true, "src/a/b.gleam": true,
		"Manifest.toml": false, "Cargo.toml": false, "src/app.erl": false,
		"build/packages/gleam_stdlib/src/gleam/list.gleam": false, "build/packages/packages.toml": false,
		"build/dev/erlang/app/_gleam_artefacts/app.gleam": false, "x/build/packages/y/gleam.toml": false,
		"src/build/tool.gleam": true,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: claimed %v, want %v", p, got, want)
		}
	}
	for p, want := range map[string]string{"gleam.toml": classConfig, "a/manifest.toml": classManifest, "src/a.gleam": ""} {
		if got := (Plugin{}).Class(&scan.File{Path: p}); got != want {
			t.Errorf("%s: class %q, want %q", p, got, want)
		}
	}
	// A manifest.toml that Gleam did not write gives nothing.
	extraction, _ := Plugin{}.Extract(&scan.File{Path: "manifest.toml"}, []byte("[package]\nname = \"x\"\n"))
	if len(extraction.Imports) != 0 {
		t.Errorf("foreign manifest.toml: %+v", extraction.Imports)
	}
}

// Modules of packages gleam downloaded into build/packages name their package even
// when the module path does not; the build directory itself is not analyzed.
//
// Verifies: REQ-GLEAM-001, REQ-GLEAM-004
func TestInstalledPackages(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"gleam.toml":    "name = \"app\"\n\n[dependencies]\npretty_print = \"~> 1.0\"\n",
		"src/app.gleam": "import pp/doc\n",
		".gitignore":    "build\n",
		"build/packages/pretty_print/src/pp/doc.gleam": "pub fn x() { 1 }\n",
		"build/packages/packages.toml":                 "[packages]\npretty_print = \"1.0.2\"\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["src/app.gleam"], map[string]lang.Target{
		"pp/doc": {Ecosystem: ecosystemHex, Package: "pretty_print", Version: "~> 1.0"},
	})
	for f := range results {
		if strings.HasPrefix(f, "build/") {
			t.Errorf("%s analyzed", f)
		}
	}
}

// Verifies: REQ-GLEAM-002, REQ-GLEAM-010
func TestLexer(t *testing.T) {
	source := "import a/b.{type T, f as g} as c\nimport d/e\n  .{x}\n" +
		"const s = \"import no/pe \\\" still string\n// import no/comment\"\n" +
		"// import commented/out\nfn f() { \"@external(erlang, \\\"no\\\", \\\"x\\\")\" }\nimport z\n"
	extraction := extractSource([]byte(source))
	var specs []string
	for _, rawImport := range extraction.Imports {
		specs = append(specs, rawImport.Spec)
	}
	if want := []string{"a/b", "d/e", "z"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %v, want %v", specs, want)
	}
	if want := []lang.Symbol{{Name: "s", Kind: "const", Line: 4}, {Name: "f", Kind: "func", Line: 7}}; !reflect.DeepEqual(extraction.Symbols, want) {
		t.Errorf("symbols %+v", extraction.Symbols)
	}
}

// Every prefix of every fixture file, and long runs of each construct, extract
// without a panic and in time linear in their size.
//
// Verifies: REQ-GLEAM-010
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
		f := &scan.File{Path: strings.TrimPrefix(filepath.ToSlash(p), "testdata/repo/")}
		for i := 0; i <= len(source); i++ {
			if _, err := (Plugin{}).Extract(f, source[:i]); err != nil {
				t.Fatal(err)
			}
			extractSource(source[:i])
		}
	}
	for _, unit := range []string{"{", "(", "[", "}", ")", "\"", "\\", "//", "import ", "import a/", "import a.{",
		"import a.{b, ", "@external(", "@external(erlang, \"m\", ", "pub type T(", "pub type T { A(", "type ",
		"fn ", "pub external fn f(", "external fn f() = ", "const ", "-> ", ".. ", "1.0e", "é"} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extractSource(source)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(source)/len(unit), d)
		}
	}
}
