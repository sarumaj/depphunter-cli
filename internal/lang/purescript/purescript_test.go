package purescript

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/dhall"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a spago.yaml workspace at the root with two packages - shop
// (src/, test/) and packages/widgets - a spago.lock, a package set, extra
// packages from git, the registry and a path, and two packages spago installed
// in .spago/p/; a legacy spago 0.20 project under legacy/ (spago.dhall, a
// test.dhall extending it, and a packages.dhall overriding its package set);
// and a bower.json.

const set = "registry 60.0.0"
const psc = "psc-0.15.0-20220507"

func pinned(name, version string) lang.Target {
	return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Version: version, Pinned: true}
}

func inSet(name, set string) lang.Target {
	return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Version: set}
}

var (
	registryLibrary = lang.Target{Ecosystem: ecosystemPureScript, Package: "github.com/purescript/registry-dev/lib",
		Version: "3e8af27822b9d1ac8bfb158cdad08bcf6e6ec8be", Pinned: true, Origin: "https://github.com/purescript/registry-dev.git"}
	glitter = lang.Target{Ecosystem: ecosystemPureScript, Package: "github.com/acme/purescript-glitter", Version: "v2.0.0",
		Origin: "https://github.com/acme/purescript-glitter.git"}
	domIndexed = lang.Target{Ecosystem: ecosystemPureScript, Package: "github.com/purescript-halogen/purescript-dom-indexed",
		Version: "v11.0.0", Origin: "https://github.com/purescript-halogen/purescript-dom-indexed.git"}
	forked = lang.Target{Ecosystem: ecosystemPureScript, Package: "github.com/acme/purescript-forked",
		Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://github.com/acme/purescript-forked.git"}
	prelude = pinned("prelude", "6.0.1")
	maybe   = pinned("maybe", "6.0.0")
	effect  = pinned("effect", "4.0.0")
)

// Verifies: REQ-PURESCRIPT-002, REQ-PURESCRIPT-004, REQ-PURESCRIPT-007, REQ-PURESCRIPT-011
func TestModules(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["src/Main.purs"], map[string]lang.Target{
		"Prelude":              prelude,
		"Data.Argonaut.Core":   {Ecosystem: ecosystemPureScript, Package: "argonaut-core", Unresolved: true}, // table, not listed
		"Data.Map":             inSet("ordered-collections", set),                                            // table
		"Data.Maybe":           maybe,
		"Effect":               effect,
		"Effect.Aff":           {Ecosystem: ecosystemPureScript, Package: "aff", Version: "7.1.0", Requested: ">=7.0.0 <8.0.0", Pinned: true},
		"Effect.Console":       inSet("console", set),
		"Glitter.Sparkle":      glitter, // only the package installed in .spago/p says so
		"Halogen.HTML":         pinned("halogen", "7.0.0"),
		"Mystery.Thing":        {},                                                                                               // nothing names its package
		"Node.FS.Aff":          {Ecosystem: ecosystemPureScript, Package: "node-fs", Version: ">=9.0.0 <10.0.0", Floating: true}, // spelled
		"Prim.Row":             {Ecosystem: ecosystemStd, Package: prim},
		"Registry.PackageName": registryLibrary, // installed from git
		"Shop.Cart":            {Local: "src/Shop/Cart.purs"},
		"Widgets.Button":       {Local: "packages/widgets/src/Widgets/Button.purs"}, // a workspace package
		"foreign import":       {Local: "src/Main.js"},
	})
	langtest.CheckImports(t, results["src/Shop/Cart.purs"], map[string]lang.Target{
		"Prelude": prelude, "Data.Array": pinned("arrays", "7.3.0"), "Data.Maybe": maybe,
	})
	langtest.CheckImports(t, results["test/Test/Main.purs"], map[string]lang.Target{
		"Prelude": prelude, "Effect": effect,
		"Shop.Cart":    {Local: "src/Shop/Cart.purs"},
		"Test.Helpers": {Local: "test/Test/Helpers.purs"},
		"Test.Spec":    pinned("spec", "7.6.0"), // a test dependency
	})
	langtest.CheckImports(t, results["packages/widgets/src/Widgets/Button.purs"], map[string]lang.Target{
		"Prelude": prelude, "Web.HTML": inSet("web-html", set), "Data.Maybe": maybe,
	})
	langtest.CheckImports(t, results["legacy/src/Legacy/Main.purs"], map[string]lang.Target{
		"Prelude":             inSet("prelude", psc),
		"Control.Monad.State": inSet("transformers", psc),
		"DOM.HTML.Indexed":    domIndexed, // a packages.dhall override: a tag is shown, neither pinned nor floating
		"Data.Maybe":          inSet("maybe", psc),
		"Forked.Thing":        forked, // an override at a commit pins
		"Legacy.Util":         {Local: "legacy/src/Legacy/Util.purs"},
		"Widgets.Button":      {Local: "packages/widgets/src/Widgets/Button.purs"}, // a local repo in the set
	})
	langtest.CheckImports(t, results["legacy/test/Legacy/Test.purs"], map[string]lang.Target{
		"Legacy.Util": {Local: "legacy/src/Legacy/Util.purs"},
		"Test.Assert": inSet("assert", psc), // test.dhall adds assert
	})
}

// Verifies: REQ-PURESCRIPT-003, REQ-PURESCRIPT-010
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, results["src/Main.purs"], map[string]string{
		"greeting": "func", "raw": "func", "greet": "foreign", "Handle": "type", "main": "func", "<+>": "operator",
	})
	langtest.CheckSymbols(t, results["src/Shop/Cart.purs"], map[string]string{
		"Item": "type", "Item.Book": "constructor", "Item.Toy": "constructor", "Item.Gift": "constructor",
		"Cart": "type", "Cart.Cart": "constructor", "Total": "type",
		"Priced": "class", "Priced.price": "method", "Priced.discount": "method",
		"pricedItem": "instance", "eqItem": "instance", "showCart": "instance",
		"empty": "func", "total": "func",
	})
	langtest.CheckSymbols(t, results["spago.yaml"], map[string]string{"shop": "package"})
	langtest.CheckSymbols(t, results["legacy/spago.dhall"], map[string]string{"legacy": "package"})
	// A signature and its definition are one symbol, on the signature's line.
	for _, s := range results["src/Shop/Cart.purs"].Symbols {
		if s.Name == "empty" && s.Line != 27 {
			t.Errorf("empty on line %d, want 27", s.Line)
		}
	}
}

// Verifies: REQ-PURESCRIPT-005, REQ-PURESCRIPT-006
func TestManifests(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["spago.yaml"], map[string]lang.Target{
		"aff":                 {Ecosystem: ecosystemPureScript, Package: "aff", Version: "7.1.0", Requested: ">=7.0.0 <8.0.0", Pinned: true},
		"arrays":              pinned("arrays", "7.3.0"),
		"console":             inSet("console", set), // the package set decides, offline unknown
		"effect":              effect,
		"halogen":             pinned("halogen", "7.0.0"),
		"maybe":               maybe,
		"node-fs":             {Ecosystem: ecosystemPureScript, Package: "node-fs", Version: ">=9.0.0 <10.0.0", Floating: true},
		"ordered-collections": inSet("ordered-collections", set),
		"prelude":             prelude,
		"registry-lib":        registryLibrary,
		"acme-glitter":        glitter, // a git tag: shown, neither pinned nor floating
		"widgets":             {Local: "packages/widgets/spago.yaml"},
		"spec":                pinned("spec", "7.6.0"),
		"json-codecs":         pinned("json-codecs", "4.0.0"), // an extra registry version
		"vendored":            {},                             // a path that is not there
	})
	langtest.CheckImports(t, results["spago.lock"], map[string]lang.Target{
		"aff": pinned("aff", "7.1.0"), "arrays": pinned("arrays", "7.3.0"), "effect": effect,
		"halogen": pinned("halogen", "7.0.0"), "maybe": maybe, "prelude": prelude,
		"registry-lib": registryLibrary, "spec": pinned("spec", "7.6.0"),
	})
	langtest.CheckImports(t, results["packages/widgets/spago.yaml"], map[string]lang.Target{
		"prelude": prelude, "web-html": inSet("web-html", set),
	})
	langtest.CheckImports(t, results["legacy/spago.dhall"], map[string]lang.Target{
		"console": inSet("console", psc), "dom-indexed": domIndexed, "effect": inSet("effect", psc),
		"forked": forked, "maybe": inSet("maybe", psc), "prelude": inSet("prelude", psc),
		"transformers": inSet("transformers", psc), "widgets": {Local: "packages/widgets/spago.yaml"},
	})
	langtest.CheckImports(t, results["legacy/packages.dhall"], map[string]lang.Target{
		"forked": forked, "dom-indexed": domIndexed, "widgets": {Local: "packages/widgets/spago.yaml"},
	})
	langtest.CheckImports(t, results["legacy/test.dhall"], map[string]lang.Target{"assert": inSet("assert", psc)})
	langtest.CheckImports(t, results["bower.json"], map[string]lang.Target{
		"purescript-prelude": {Ecosystem: ecosystemPureScript, Package: "prelude", Version: "^v6.0.0", Floating: true},
		"purescript-halogen-subscriptions": {Ecosystem: ecosystemPureScript, Package: "github.com/purescript-halogen/purescript-halogen-subscriptions",
			Version: "^2.0.0", Floating: true, Origin: "https://github.com/purescript-halogen/purescript-halogen-subscriptions.git"},
		"purescript-spec": pinned("spec", "7.0.0"),
	})
	for _, imported := range results["spago.yaml"].Imports {
		if imported.Spec == "node-fs" && imported.Line != 10 {
			t.Errorf("node-fs on line %d, want 10", imported.Line)
		}
	}
	// Not a manifest spago or bower would read.
	for name, source := range map[string]string{
		"spago.yaml": "other: 1\n", "spago.lock": "[1]", "bower.json": `{"name": "left-pad", "dependencies": {"x": "1"}}`,
		"spago.dhall": `{ foo = 1 }`, "packages.dhall": `https://example.com/packages.dhall`,
	} {
		if extraction, _ := (Plugin{}).Extract(&scan.File{Path: name}, []byte(source)); len(extraction.Imports) != 0 || len(extraction.Symbols) != 0 {
			t.Errorf("%s: %+v", name, extraction)
		}
	}
}

// Verifies: REQ-PURESCRIPT-005
func TestDhall(t *testing.T) {
	files := map[string]string{
		"base.dhall": `let x = "a" in { list = [ x, "b" ] : List Text, name = "base" }`,
	}
	load := func(f string) *dhall.Value {
		if source, ok := files[f]; ok {
			return dhall.Eval([]byte(source), ".", nil)
		}
		return nil
	}
	for source, want := range map[string][]string{
		// Imports, `//`, `#`, selection.
		`let b = ./base.dhall in b // { list = b.list # [ "c" ] }`: {"a", "b", "c"},
		// A projection keeps what is known; `?` takes the first import that loads.
		`(./missing.dhall ? ./base.dhall).{ list }.list`: {"a", "b"},
		// Some, nested with, text interpolation (unknown), comments.
		`{- {- nested -} -} (Some { list = [ "x", "${y}" ] } with list = [ "w" ]).list -- tail`: {"w"},
		// A record type is not a value; a lambda is not understood.
		`{ list : List Text }`:            nil,
		`\(x : Text) -> { list = [ x ] }`: nil,
		`''
      multi ${"line"}
      '' `: nil,
	} {
		v := dhall.Eval([]byte(source), ".", load)
		if v.Kind == dhall.KindRecord {
			v = v.Field("list")
		}
		var got []string
		for _, e := range v.Texts() {
			got = append(got, e.Text)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, want %v", source, got, want)
		}
	}
	// Older package sets build their records with mkPackage.
	v := dhall.Eval([]byte(`let mkPackage = https://example.com/mkPackage.dhall sha256:00
let upstream = https://github.com/purescript/package-sets/releases/download/psc-0.13.8-20200822/packages.dhall
in upstream // { foo = mkPackage [ "prelude" ] "https://github.com/acme/foo.git" "v1.0.0" }
  with bar.version = "v2.0.0"`), ".", nil)
	extras := dhallExtras(v, ".")
	if len(extras) != 2 || extras[0].git != "https://github.com/acme/foo.git" || extras[0].version != "v1.0.0" ||
		!reflect.DeepEqual(extras[0].dependencies, []string{"prelude"}) || extras[1].name != "bar" || extras[1].version != "v2.0.0" {
		t.Errorf("extras %+v", extras)
	}
	if n := setName(v); n != "psc-0.13.8-20200822" {
		t.Errorf("set name %q", n)
	}
}

// writeTree writes files under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// spago 0.20 installs packages in .spago/<name>/<version>/, bower in
// bower_components/purescript-<name>/: what is on disk says which package a
// module is.
//
// Verifies: REQ-PURESCRIPT-007
func TestInstalledLegacy(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"spago.dhall":    `{ name = "app", dependencies = [ "odd-name", "maybe" ], packages = ./packages.dhall, sources = [ "src/**/*.purs" ] }`,
		"packages.dhall": `https://github.com/purescript/package-sets/releases/download/psc-0.15.4-20220901/packages.dhall`,
		"src/Main.purs":  "module Main where\nimport Unusual.Module\nimport Data.Maybe\nimport Web.Thing\n",
		".spago/odd-name/v3.0.0/src/Unusual/Module.purs": "module Unusual.Module where\n",
		"lib/bower.json":   `{"name": "purescript-lib", "dependencies": {"purescript-web-thing": "^1.0.0"}}`,
		"lib/src/Lib.purs": "module Lib where\nimport Web.Thing\n",
		"lib/bower_components/purescript-web-thing/src/Web/Thing.purs": "module Web.Thing where\n",
	})
	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["src/Main.purs"], map[string]lang.Target{
		"Unusual.Module": inSet("odd-name", "psc-0.15.4-20220901"),
		"Data.Maybe":     inSet("maybe", "psc-0.15.4-20220901"),
		"Web.Thing":      {}, // installed for lib/ only, and not listed here
	})
	langtest.CheckImports(t, results["lib/src/Lib.purs"], map[string]lang.Target{
		"Web.Thing": {Ecosystem: ecosystemPureScript, Package: "web-thing", Version: "^1.0.0", Floating: true},
	})
	if _, ok := results["lib/bower_components/purescript-web-thing/src/Web/Thing.purs"]; ok {
		t.Error("bower_components/ analyzed")
	}
}

// Verifies: REQ-PURESCRIPT-008
func TestDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	for _, c := range []struct {
		t    lang.Target
		want []lang.Target
	}{
		// spago.lock's dependencies, pinned by the lock.
		{pinned("aff", "7.1.0"), []lang.Target{effect, prelude}},
		{registryLibrary, []lang.Target{pinned("aff", "7.1.0"), prelude}},
		// A packages.dhall override's dependencies, as its workspace decides them.
		{forked, []lang.Target{inSet("prelude", psc), inSet("maybe", psc)}},
		{inSet("console", set), nil},
		{lang.Target{Ecosystem: "npm", Package: "aff"}, nil},
	} {
		if got := r.Dependencies(c.t); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.t.Package, got, c.want)
		}
	}
}

// Without a spago.lock, the manifests of what spago installed answer: halogen's
// spago.yaml (not its test dependencies), aff's purs.json, a git package's
// spago.yaml under its ref, and in a spago 0.20 project console's spago.dhall.
// Each dependency spago installed too is pinned to what it installed; one it
// did not is left to the package set. A manifest that is not one says nothing.
//
// Verifies: REQ-PURESCRIPT-008
func TestInstalledDependencies(t *testing.T) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	files := map[string]string{
		"spago.yaml": "package:\n  name: app\n  dependencies:\n    - halogen\n" +
			"workspace:\n  packageSet:\n    registry: 60.0.0\n",
		".spago/p/halogen-7.0.0/spago.yaml": "package:\n  name: halogen\n  dependencies:\n    - aff\n    - prelude: \">=6.0.0 <7.0.0\"\n" +
			"    - dom-indexed\n  test:\n    main: Test.Main\n    dependencies:\n      - spec\n",
		".spago/p/aff-7.1.0/purs.json":             `{"name": "aff", "version": "7.1.0", "dependencies": {"prelude": ">=6.0.0 <7.0.0"}}`,
		".spago/p/prelude-6.0.1/purs.json":         `{"name": "prelude", "version": "6.0.1", "dependencies": {}}`,
		".spago/p/hooks/" + commit + "/spago.yaml": "package:\n  name: hooks\n  dependencies:\n    - halogen\n",
		"legacy/spago.dhall":                       `{ name = "legacy", dependencies = [ "console" ], packages = ./packages.dhall, sources = [ "src/**/*.purs" ] }`,
		"legacy/.spago/console/v6.1.0/spago.dhall": `{ name = "console", dependencies = [ "effect", "prelude" ], packages = ./packages.dhall, sources = [ "src/**/*.purs" ] }`,
	}
	root := langtest.Write(t, files)
	r := newResolver(root, langtest.Files(t, root))
	halogen := pinned("halogen", "7.0.0")
	for _, c := range []struct {
		t    lang.Target
		want []lang.Target
	}{
		{halogen, []lang.Target{pinned("aff", "7.1.0"), pinned("prelude", "6.0.1"), inSet("dom-indexed", set)}},
		{pinned("aff", "7.1.0"), []lang.Target{pinned("prelude", "6.0.1")}},
		{lang.Target{Ecosystem: ecosystemPureScript, Package: "hooks"}, []lang.Target{halogen}},
		{lang.Target{Ecosystem: ecosystemPureScript, Package: "console"}, []lang.Target{
			{Ecosystem: ecosystemPureScript, Package: "effect", Floating: true}, {Ecosystem: ecosystemPureScript, Package: "prelude", Floating: true}}},
	} {
		if got := r.Dependencies(c.t); !reflect.DeepEqual(got, c.want) || !r.Installed(c.t) {
			t.Errorf("%s: got %+v, want %+v", c.t.Package, got, c.want)
		}
	}
	if r.Installed(inSet("dom-indexed", set)) {
		t.Error("dom-indexed is not installed")
	}

	files[".spago/p/halogen-7.0.0/spago.yaml"] = "package: [[[ \x00"
	root = langtest.Write(t, files)
	if got := newResolver(root, langtest.Files(t, root)).Dependencies(halogen); got != nil {
		t.Errorf("garbage spago.yaml: %+v", got)
	}
}

// Verifies: REQ-PURESCRIPT-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"src/Main.purs": true, "spago.yaml": true, "spago.lock": true, "spago.dhall": true, "packages.dhall": true,
		"test.dhall": true, "examples/spago-dev.dhall": true, "bower.json": true, "config/app.dhall": false,
		".spago/p/prelude-6.0.1/src/Prelude.purs": false, "a/.spago/x/v1/src/X.purs": false,
		"bower_components/purescript-maybe/bower.json": false, "src/Main.js": false, "package.json": false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: claimed %v, want %v", p, got, want)
		}
	}
	for p, want := range map[string]string{"spago.yaml": classYAML, "a/spago.lock": classLock, "packages.dhall": classSet,
		"spago.dhall": classDhall, "bower.json": classBower, "src/A.purs": ""} {
		if got := (Plugin{}).Class(&scan.File{Path: p}); got != want {
			t.Errorf("%s: class %q, want %q", p, got, want)
		}
	}
}

// Verifies: REQ-PURESCRIPT-002, REQ-PURESCRIPT-010
func TestLexer(t *testing.T) {
	source := "module A\n( x\n, y\n) where\n" +
		"import B.C as D\n" +
		"{- import No.Block {- -}\n" +
		"s = \"import No.String \\\" -- still\"\n" +
		"g = \"gap \\\n   \\import No.Gap\"\n" +
		"m = \"\"\"\nimport No.Raw \\\n\"\"\"\n" +
		"c = '\\''\n" +
		"-- import No.Comment\n" +
		"x' = foldl' f 'y' --> z\n" +
		"import E hiding (f)\n" +
		"import B.C\n" +
		"y ∷ Int\n" +
		"y = 1\n" +
		"  import No.Indented\n" +
		"type role T nominal\n"
	extraction := extractSource([]byte(source))
	var specs []string
	for _, rawImport := range extraction.Imports {
		specs = append(specs, rawImport.Spec)
	}
	if want := []string{"B.C", "E"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %v, want %v", specs, want)
	}
	want := []lang.Symbol{
		{Name: "s", Kind: "func", Line: 7}, {Name: "g", Kind: "func", Line: 8}, {Name: "m", Kind: "func", Line: 10},
		{Name: "c", Kind: "func", Line: 13}, {Name: "x'", Kind: "func", Line: 15}, {Name: "y", Kind: "func", Line: 18},
	}
	if !reflect.DeepEqual(extraction.Symbols, want) {
		t.Errorf("symbols %+v, want %+v", extraction.Symbols, want)
	}
	if moduleName([]byte("-- | doc\n{- x -}\nmodule Data.Shop (a) where\n")) != "Data.Shop" || moduleName([]byte("x = 1")) != "" {
		t.Error("moduleName")
	}
}

// Every prefix of every fixture file, and long runs of each construct, extract
// without a panic and in time linear in their size.
//
// Verifies: REQ-PURESCRIPT-010
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
			dhall.Eval(source[:i], ".", nil)
		}
	}
	for _, unit := range []string{"{-", "-}", "{", "(", "[", "\"", "\"\"\"", "'", "\\", "--", "import ", "import A.",
		"data T = ", "| A ", "class ", "instance x :: ", "foreign import ", "infixl 5 f as ", "f a b = ", "x :: ",
		"\nx", "\nA.B.c", "A.", "1.0e", "0x", "é", "∷", "\n", "module M where\n",
		// Dhall
		"let x = ", "{ a = ", "[ ", "( ", "x // ", "x # ", "x with a = ", "x.", "''", "${", "< A | ", "\\(x : T) -> ",
		"https://x ", "./a ", "Some ", "mkPackage [] \"\" \"\" "} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extractSource(source)
		dhall.Eval(source, ".", nil)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(source)/len(unit), d)
		}
	}
}

// Every package target the fixture resolves is in one of the plugin's islands.
//
// Verifies: REQ-PURESCRIPT-009
func TestEcosystems(t *testing.T) {
	ecosystems := (Plugin{}).Ecosystems()
	if len(ecosystems) != 2 || ecosystems[0].ID != ecosystemPureScript || ecosystems[0].Std || ecosystems[1].ID != ecosystemStd || !ecosystems[1].Std {
		t.Fatalf("ecosystems %+v", ecosystems)
	}
	for f, r := range langtest.Analyze(t, Plugin{}, "testdata/repo") {
		for _, imported := range r.Imports {
			if imported.Target.Package != "" && imported.Target.Ecosystem != ecosystemPureScript && imported.Target.Ecosystem != ecosystemStd {
				t.Errorf("%s %s: %+v", f, imported.Spec, imported.Target)
			}
		}
	}
}
