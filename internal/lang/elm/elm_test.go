package elm

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

// The fixture is an Elm application with two source directories (src,
// lib/shared), elm-test tests under tests/, direct, indirect and test
// dependencies at exact versions; a package (packages/ui-kit) with ranged
// dependencies, grouped exposed-modules and kernel JavaScript; and an examples/
// application whose source directories reach into the package.

func pinned(name, version string) lang.Target {
	return lang.Target{Ecosystem: ecosystemElm, Package: name, Version: version, Pinned: true}
}

func ranged(name, versionRange string) lang.Target {
	return lang.Target{Ecosystem: ecosystemElm, Package: name, Version: versionRange, Floating: true}
}

// analyze runs the plugin over the fixture with an empty ELM_HOME, so what the
// machine running the tests has installed does not matter.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Setenv("ELM_HOME", t.TempDir())
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-ELM-002, REQ-ELM-004, REQ-ELM-007, REQ-ELM-011
func TestModules(t *testing.T) {
	results := analyze(t)
	html := pinned("elm/html", "1.0.0")
	langtest.CheckImports(t, results["src/Main.elm"], map[string]lang.Target{
		"Browser":              {Ecosystem: ecosystemElm, Package: "elm/browser", Unresolved: true}, // not in elm.json
		"Dict":                 pinned("elm/core", "1.0.5"),
		"Glitter.Sparkle":      pinned("acme/elm-glitter-effects", "3.1.0"), // the name starts like the module
		"Html":                 html,
		"Html.Attributes":      html,
		"Html.Styled":          pinned("rtfeldman/elm-css", "18.0.0"), // the longer table entry
		"Json.Decode":          pinned("elm/json", "1.1.3"),
		"Json.Decode.Pipeline": pinned("NoRedInk/elm-json-decode-pipeline", "1.0.1"),
		"List.Extra":           pinned("elm-community/list-extra", "8.7.0"),
		"Markdown":             {Ecosystem: ecosystemElm, Package: "elm-explorations/markdown", Unresolved: true},
		"Mystery.Thing":        {}, // nothing names its package
		"Shared.Format":        {Local: "lib/shared/Shared/Format.elm"},
		"Shop.Cart":            {Local: "src/Shop/Cart.elm"},
		"Widget.Button":        {}, // acme/elm-toolkit's, but only its installed elm.json says so
	})
	test := pinned("elm-explorations/test", "2.1.1")
	langtest.CheckImports(t, results["tests/CartTest.elm"], map[string]lang.Target{
		"Expect": test, "Fuzz": test, "Test": test,
		"Helpers":   {Local: "tests/Helpers.elm"},
		"Shop.Cart": {Local: "src/Shop/Cart.elm"},
	})
	langtest.CheckImports(t, results["src/Shop/Cart.elm"], map[string]lang.Target{"Shared.Format": {Local: "lib/shared/Shared/Format.elm"}})
	// The package's own directory wins over examples/, which lists it too.
	core := ranged("elm/core", "1.0.0 <= v < 2.0.0")
	langtest.CheckImports(t, results["packages/ui-kit/src/UiKit/Button.elm"], map[string]lang.Target{
		"Elm.Kernel.Kit":   {Local: "packages/ui-kit/src/Elm/Kernel/Kit.js"},
		"Elm.Kernel.Nope":  {},
		"Elm.Kernel.Utils": core,
		"Html":             ranged("elm/html", "1.0.0 <= v < 2.0.0"),
		"UiKit.Missing":    {}, // exposed by the package itself, but missing
	})
	langtest.CheckImports(t, results["examples/Demo.elm"], map[string]lang.Target{
		"Html":         html,
		"UiKit.Button": {Local: "packages/ui-kit/src/UiKit/Button.elm"},
	})
}

// Verifies: REQ-ELM-003, REQ-ELM-010
func TestSymbols(t *testing.T) {
	results := analyze(t)
	langtest.CheckSymbols(t, results["src/Main.elm"], map[string]string{
		"sendMessage": "port", "messageReceiver": "port",
		"help": "func", "shader": "func", "quote": "func", "main": "func",
	})
	langtest.CheckSymbols(t, results["src/Shop/Cart.elm"], map[string]string{
		"Item": "type", "Item.Book": "constructor", "Item.Toy": "constructor", "Item.Gift": "constructor",
		"Cart": "type", "empty": "func", "total": "func", "andThen": "func", "|>>": "operator",
	})
	langtest.CheckSymbols(t, results["packages/ui-kit/elm.json"], map[string]string{"acme/ui-kit": "package"})
	// An annotation and its definition are one symbol, on the annotation's line.
	for _, s := range results["lib/shared/Shared/Format.elm"].Symbols {
		if s.Name == "price" && s.Line != 4 {
			t.Errorf("price on line %d, want 4", s.Line)
		}
	}
}

// Verifies: REQ-ELM-005, REQ-ELM-006
func TestManifests(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["elm.json"], map[string]lang.Target{
		"NoRedInk/elm-json-decode-pipeline": pinned("NoRedInk/elm-json-decode-pipeline", "1.0.1"),
		"acme/elm-glitter-effects":          pinned("acme/elm-glitter-effects", "3.1.0"),
		"acme/elm-toolkit":                  pinned("acme/elm-toolkit", "2.0.0"),
		"elm/core":                          pinned("elm/core", "1.0.5"),
		"elm/html":                          pinned("elm/html", "1.0.0"),
		"elm/json":                          pinned("elm/json", "1.1.3"),
		"elm-community/list-extra":          pinned("elm-community/list-extra", "8.7.0"),
		"rtfeldman/elm-css":                 pinned("rtfeldman/elm-css", "18.0.0"),
		"elm/time":                          pinned("elm/time", "1.0.0"),        // indirect
		"elm/virtual-dom":                   pinned("elm/virtual-dom", "1.0.3"), // indirect
		"elm-explorations/test":             pinned("elm-explorations/test", "2.1.1"),
		"elm/random":                        pinned("elm/random", "1.0.0"), // indirect test dependency
		"src":                               {Local: "src"},
		"lib/shared":                        {Local: "lib/shared"},
	})
	langtest.CheckImports(t, results["packages/ui-kit/elm.json"], map[string]lang.Target{
		"elm/core":              ranged("elm/core", "1.0.0 <= v < 2.0.0"),
		"elm/html":              ranged("elm/html", "1.0.0 <= v < 2.0.0"),
		"elm-explorations/test": ranged("elm-explorations/test", "2.0.0 <= v < 3.0.0"),
	})
	langtest.CheckImports(t, results["examples/elm.json"], map[string]lang.Target{
		"elm/core":               pinned("elm/core", "1.0.5"),
		"elm/html":               pinned("elm/html", "1.0.0"),
		"elm/json":               pinned("elm/json", "1.1.3"),
		"elm/virtual-dom":        pinned("elm/virtual-dom", "1.0.3"),
		".":                      {Local: "examples"},
		"../packages/ui-kit/src": {Local: "packages/ui-kit/src"},
		"../../outside":          {}, // outside the repository
	})
	for _, imported := range results["elm.json"].Imports {
		if imported.Spec == "elm/random" && imported.Line != 29 {
			t.Errorf("elm/random on line %d, want 29", imported.Line)
		}
	}
	// Not an elm.json the compiler would read.
	for _, source := range []string{"{}", "[1]", `{"type": "library"}`, "not json"} {
		if extraction, _ := (Plugin{}).Extract(&scan.File{Path: "elm.json"}, []byte(source)); len(extraction.Imports) != 0 || len(extraction.Symbols) != 0 {
			t.Errorf("%s: %+v", source, extraction)
		}
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

// Packages the compiler downloaded into ELM_HOME say which modules they expose
// and what they depend on.
//
// Verifies: REQ-ELM-007, REQ-ELM-008
func TestInstalledPackages(t *testing.T) {
	home := t.TempDir()
	writeTree(t, home, map[string]string{
		"0.19.1/packages/acme/elm-toolkit/2.0.0/elm.json": `{"type": "package", "name": "acme/elm-toolkit",
"exposed-modules": {"Widgets": ["Widget.Button", "Widget.Card"]},
"dependencies": {"elm/core": "1.0.0 <= v < 2.0.0", "elm/html": "1.0.0 <= v < 2.0.0", "acme/elm-colors": "1.0.0 <= v < 2.0.0"},
"test-dependencies": {"elm-explorations/test": "2.0.0 <= v < 3.0.0"}}`,
		// A package project's range picks the newest installed version it admits.
		"0.19.1/packages/elm/html/1.0.0/elm.json": `{"type": "package", "name": "elm/html", "exposed-modules": ["Html"],
"dependencies": {"elm/core": "1.0.0 <= v < 2.0.0"}}`,
		"0.19.1/packages/elm/html/1.0.1/elm.json": `{"type": "package", "name": "elm/html", "exposed-modules": ["Html", "Html.Keyed"],
"dependencies": {"elm/core": "1.0.0 <= v < 2.0.0", "elm/virtual-dom": "1.0.0 <= v < 2.0.0"}}`,
		"0.19.1/packages/elm/html/2.0.0/elm.json": `{"type": "package", "name": "elm/html", "exposed-modules": ["Html2"]}`,
	})
	t.Setenv("ELM_HOME", home)
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["src/Main.elm"], map[string]lang.Target{
		"Browser":              {Ecosystem: ecosystemElm, Package: "elm/browser", Unresolved: true},
		"Dict":                 pinned("elm/core", "1.0.5"),
		"Glitter.Sparkle":      pinned("acme/elm-glitter-effects", "3.1.0"),
		"Html":                 pinned("elm/html", "1.0.0"),
		"Html.Attributes":      pinned("elm/html", "1.0.0"),
		"Html.Styled":          pinned("rtfeldman/elm-css", "18.0.0"),
		"Json.Decode":          pinned("elm/json", "1.1.3"),
		"Json.Decode.Pipeline": pinned("NoRedInk/elm-json-decode-pipeline", "1.0.1"),
		"List.Extra":           pinned("elm-community/list-extra", "8.7.0"),
		"Markdown":             {Ecosystem: ecosystemElm, Package: "elm-explorations/markdown", Unresolved: true},
		"Mystery.Thing":        {},
		"Shared.Format":        {Local: "lib/shared/Shared/Format.elm"},
		"Shop.Cart":            {Local: "src/Shop/Cart.elm"},
		"Widget.Button":        pinned("acme/elm-toolkit", "2.0.0"), // its installed elm.json exposes it
	})

	files := langtest.Files(t, "testdata/repo")
	r := newResolver("testdata/repo", files, home)
	// An application of the repository pins what it lists; the rest float as ranged.
	got := r.Dependencies(pinned("acme/elm-toolkit", "2.0.0"))
	want := []lang.Target{ranged("acme/elm-colors", "1.0.0 <= v < 2.0.0"), pinned("elm/core", "1.0.5"), pinned("elm/html", "1.0.0")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("acme/elm-toolkit depends on %+v, want %+v", got, want)
	}
	if !r.Installed(pinned("acme/elm-toolkit", "2.0.0")) || r.Installed(pinned("acme/elm-colors", "1.0.0")) {
		t.Error("Installed: want only what ELM_HOME holds")
	}
	got = r.Dependencies(ranged("elm/html", "1.0.0 <= v < 2.0.0"))
	want = []lang.Target{pinned("elm/core", "1.0.5"), pinned("elm/virtual-dom", "1.0.3")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("elm/html in range depends on %+v, want %+v (1.0.1)", got, want)
	}
	if got := r.Dependencies(lang.Target{Ecosystem: "npm", Package: "acme/elm-toolkit", Version: "2.0.0"}); got != nil {
		t.Errorf("npm: %+v", got)
	}
	if got := r.Dependencies(pinned("../../etc", "1.0.0")); got != nil {
		t.Errorf("path in a name: %+v", got)
	}
}

// Verifies: REQ-ELM-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"src/Main.elm": true, "elm.json": true, "tests/elm.json": true, "src/A/B.elm": true,
		"elm-stuff/0.19.1/i.dat": false, "elm-stuff/generated-code/x/Main.elm": false,
		"app/elm-stuff/elm.json": false, "package.json": false, "src/main.js": false, "src/elm-stuffing.elm": true,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: claimed %v, want %v", p, got, want)
		}
	}
	for p, want := range map[string]string{"elm.json": classManifest, "a/elm.json": classManifest, "src/A.elm": ""} {
		if got := (Plugin{}).Class(&scan.File{Path: p}); got != want {
			t.Errorf("%s: class %q, want %q", p, got, want)
		}
	}
}

// Verifies: REQ-ELM-002, REQ-ELM-010
func TestLexer(t *testing.T) {
	source := "module A exposing (..)\n" +
		"import B.C as D exposing (e, F(..))\n" +
		"{- {- import No.Nested -} import No.Block -}\n" +
		"s = \"import No.String \\\" -- still\"\n" +
		"m = \"\"\"\nimport No.Multi \\\"\"\" \n\"\"\"\n" +
		"c = '\\''\n" +
		"g = [glsl|\nimport No.Glsl\n|]\n" +
		"-- import No.Comment\n" +
		"import E\n" +
		"import B.C\n" +
		"x=1 --comment\n" +
		"record = { a = 1 }.a\n" +
		"  import No.Indented\n"
	extraction := extractSource([]byte(source))
	var specs []string
	for _, rawImport := range extraction.Imports {
		specs = append(specs, rawImport.Spec)
	}
	if want := []string{"B.C", "E"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %v, want %v", specs, want)
	}
	want := []lang.Symbol{
		{Name: "s", Kind: "func", Line: 4}, {Name: "m", Kind: "func", Line: 5}, {Name: "c", Kind: "func", Line: 8},
		{Name: "g", Kind: "func", Line: 9}, {Name: "x", Kind: "func", Line: 15}, {Name: "record", Kind: "func", Line: 16},
	}
	if !reflect.DeepEqual(extraction.Symbols, want) {
		t.Errorf("symbols %+v, want %+v", extraction.Symbols, want)
	}
}

// Every prefix of every fixture file, and long runs of each construct, extract
// without a panic and in time linear in their size.
//
// Verifies: REQ-ELM-010
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
			readManifest(source[:i])
		}
	}
	for _, unit := range []string{"{-", "-}", "{", "(", "[", "\"", "\"\"\"", "'", "\\", "--", "[glsl|", "import ",
		"import A.", "type T = ", "| A ", "type alias ", "port ", "infix ", "f a b = ", "x : ", "\nx", "\nA.B.c",
		"A.", "1.0e", "0x", "é", "\n"} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extractSource(source)
		if d := time.Since(start); d > langtest.TimeLimit(5*time.Second) {
			t.Errorf("%q x %d: %v", unit, len(source)/len(unit), d)
		}
	}
}

// Every package target the fixture resolves is in the plugin's one island.
//
// Verifies: REQ-ELM-009
func TestEcosystems(t *testing.T) {
	ecosystems := (Plugin{}).Ecosystems()
	if len(ecosystems) != 1 || ecosystems[0].ID != ecosystemElm || ecosystems[0].Std {
		t.Fatalf("ecosystems %+v", ecosystems)
	}
	for f, r := range analyze(t) {
		for _, imported := range r.Imports {
			if imported.Target.Package != "" && imported.Target.Ecosystem != ecosystemElm {
				t.Errorf("%s %s: %+v", f, imported.Spec, imported.Target)
			}
		}
	}
}

// elm-tooling.json pins the compiler a package project installs with: its
// ELM_HOME directory is searched first for a ranged dependency. Its other tools
// are not dependencies. A garbage or missing file, or one pinning no exact
// version, leaves the default order (0.19.1 first).
//
// Verifies: REQ-ELM-005
func TestElmTooling(t *testing.T) {
	home := langtest.Write(t, map[string]string{
		"0.19.0/packages/elm/json/1.1.2/elm.json": `{"type": "package", "name": "elm/json", "exposed-modules": ["Json.Old"]}`,
		"0.19.1/packages/elm/json/1.1.3/elm.json": `{"type": "package", "name": "elm/json", "exposed-modules": ["Json.New"]}`,
	})
	files := map[string]string{
		"pkg/elm.json": `{"type": "package", "name": "acme/pkg", "elm-version": "0.19.0 <= v < 0.20.0",
			"exposed-modules": [], "dependencies": {"elm/json": "1.0.0 <= v < 2.0.0"}}`,
		"elm-tooling.json": `{"tools": {"elm": "0.19.0", "elm-format": "0.8.5", "elm-json": "0.2.13"}}`,
	}
	for _, c := range []struct{ tooling, want string }{
		{files["elm-tooling.json"], "1.1.2"},
		{"{{ not json", "1.1.3"},
		{`{"tools": {"elm": "^0.19.0"}}`, "1.1.3"},
		{`{"tools": ["elm"]}`, "1.1.3"},
		{"", "1.1.3"}, // missing
	} {
		garbage, want := c.tooling, c.want
		files["elm-tooling.json"] = garbage
		if garbage == "" {
			delete(files, "elm-tooling.json")
		}
		root := langtest.Write(t, files)
		r := newResolver(root, langtest.Files(t, root), home)
		if len(r.projects) != 1 {
			t.Fatalf("%q: projects %v", garbage, r.projects)
		}
		m := r.projects[0].m
		if got := r.installedVersion(m, m.dependencies["elm/json"]); got != want {
			t.Errorf("%q: elm/json %s, want %s", garbage, got, want)
		}
	}
}
