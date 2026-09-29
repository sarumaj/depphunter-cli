package d

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

// The fixture is a dub package "shop" (dub.json) with source/, views/ and a
// dub.selections.json, declaring packages by ~>, ==, >=, a bare exact version,
// optional, a path (libs/widgets, with a dub.sdl of its own), a git repository
// at a commit and a sub-package of a registry package (arsd-official:dom); an
// inline sub-package "cli" (cli/) and a sub-package directory tools/ (dub.sdl,
// string imports from res/). scripts/hello.d is a single-file package;
// deps/app.d is a make dependency file and probes/ holds DTrace scripts, none of
// them D. .dub/ is what dub fetched with --cache=local.

var (
	vibe      = lang.Target{Ecosystem: ecosystemDub, Package: "vibe-d", Version: "0.9.7", Requested: "~>0.9.5", Pinned: true}
	dyaml     = lang.Target{Ecosystem: ecosystemDub, Package: "dyaml", Version: "0.9.2", Requested: "==0.9.2", Pinned: true}
	mirAlgo   = lang.Target{Ecosystem: ecosystemDub, Package: "mir-algorithm", Version: "3.20.0", Requested: ">=3.0.0", Pinned: true}
	mirCore   = lang.Target{Ecosystem: ecosystemDub, Package: "mir-core", Version: "1.7.0", Pinned: true}
	fancy     = lang.Target{Ecosystem: ecosystemDub, Package: "fancy", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://github.com/acme/fancy.git"}
	unitT     = lang.Target{Ecosystem: ecosystemDub, Package: "unit-threaded", Version: "2.1.0", Pinned: true}
	arsd      = lang.Target{Ecosystem: ecosystemDub, Package: "arsd-official", Version: "~>11.0", Floating: true}
	requests  = lang.Target{Ecosystem: ecosystemDub, Package: "requests", Version: "~>2.0", Floating: true}
	eventcore = lang.Target{Ecosystem: ecosystemDub, Package: "eventcore", Version: "~master", Requested: "*", Floating: true}
	silly     = lang.Target{Ecosystem: ecosystemDub, Package: "silly", Version: "~>1.1", Floating: true}
	darg      = lang.Target{Ecosystem: ecosystemDub, Package: "darg", Version: "~>0.1", Floating: true}
	colorize  = lang.Target{Ecosystem: ecosystemDub, Package: "colorize", Version: "~>1.0", Floating: true}
)

func std(name string) lang.Target { return lang.Target{Ecosystem: ecosystemStd, Package: name} }

// analyze runs the plugin over the fixture with no dub packages installed.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Setenv("DUB_HOME", t.TempDir())
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-DLANG-002, REQ-DLANG-004, REQ-DLANG-006, REQ-DLANG-007, REQ-DLANG-011
func TestImports(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["source/shop/app.d"], map[string]lang.Target{
		"std.stdio":             std("std.stdio"),
		"core.thread":           std("core.thread"),
		"object":                std("object"),
		"shop.cart":             {Local: "source/shop/cart.d"},
		"shop.models":           {Local: "source/shop/models/package.d"}, // a package.d module
		"std.format":            std("std.format"),                       // import fmt = std.format
		"std.algorithm":         std("std.algorithm"),
		"etc.c.zlib":            std("etc.c"),
		"vibe.http.server":      vibe, // the curated table, among the declared packages
		"vibe.core.log":         vibe,
		"dyaml":                 dyaml,
		"mir.ndslice":           mirAlgo,
		"mir.math.common":       mirCore, // only selected
		"widgets.button":        {Local: "libs/widgets/source/widgets/button.d"},
		"fancy.thing":           fancy, // spelled by the declared name
		"unit_threaded":         unitT,
		"arsd.dom":              arsd,
		"requests":              requests, // a configuration's dependency
		"eventcore.core":        eventcore,
		"commands":              {Local: "cli/commands.d"}, // the inline sub-package's directory
		"nothere.x":             {Ecosystem: ecosystemDub, Package: "nothere", Unresolved: true},
		"shop.gone":             {}, // the package's own module, missing
		"std.conv":              std("std.conv"),
		`import("banner.txt")`:  {Local: "views/banner.txt"},
		`import("missing.txt")`: {},
	})
	langtest.CheckImports(t, results["source/shop/cart.d"], map[string]lang.Target{
		"shop.models.user": {Local: "source/shop/models/user.d"},
		"std.exception":    std("std.exception"), // in a unittest
	})
	langtest.CheckImports(t, results["libs/widgets/source/widgets/button.d"], map[string]lang.Target{
		"dyaml":           {Ecosystem: ecosystemDub, Package: "dyaml", Version: "~>0.9", Floating: true},
		"bindbc.sdl":      {Ecosystem: ecosystemDub, Package: "bindbc-sdl", Version: "~>1.4", Floating: true},
		"taggedalgebraic": {Ecosystem: ecosystemDub, Package: "taggedalgebraic", Version: ">=0.11.0 <0.12.0", Floating: true},
	})
	langtest.CheckImports(t, results["cli/commands.d"], map[string]lang.Target{
		"shop.cart": {Local: "source/shop/cart.d"}, // the sub-package depends on shop
		"darg":      darg,
	})
	langtest.CheckImports(t, results["tools/source/tool.d"], map[string]lang.Target{
		"shop.cart":             {Local: "source/shop/cart.d"}, // a path dependency on ..
		"colorize":              colorize,
		`import("config.json")`: {Local: "tools/res/config.json"},
	})
	langtest.CheckImports(t, results["scripts/hello.d"], map[string]lang.Target{
		"scriptlike": {Ecosystem: ecosystemDub, Package: "scriptlike", Version: "~>0.10.3", Floating: true},
		"std.stdio":  std("std.stdio"),
	})
}

// Verifies: REQ-DLANG-005, REQ-DLANG-006
func TestManifests(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["dub.json"], map[string]lang.Target{
		"vibe-d":            vibe,
		"dyaml":             dyaml,
		"mir-algorithm":     mirAlgo,
		"silly":             silly, // optional
		"widgets":           {Local: "libs/widgets"},
		"fancy":             fancy,
		"unit-threaded":     unitT,
		"arsd-official:dom": arsd, // the base package's node
		"eventcore":         eventcore,
		":cli":              {}, // its own inline sub-package
		"requests":          requests,
		"darg":              darg,
		"shop":              {Local: "."}, // cli depends on its package
		"tools/":            {Local: "tools"},
	})
	langtest.CheckImports(t, results["dub.selections.json"], map[string]lang.Target{
		"vibe-d":        vibe,
		"dyaml":         dyaml,
		"mir-algorithm": mirAlgo,
		"mir-core":      mirCore,
		"widgets":       {Local: "libs/widgets"},
		"fancy":         fancy,
		"eventcore":     eventcore,
	})
	langtest.CheckImports(t, results["libs/widgets/dub.sdl"], map[string]lang.Target{
		"dyaml":           {Ecosystem: ecosystemDub, Package: "dyaml", Version: "~>0.9", Floating: true},
		"bindbc-sdl":      {Ecosystem: ecosystemDub, Package: "bindbc-sdl", Version: "~>1.4", Floating: true},
		"taggedalgebraic": {Ecosystem: ecosystemDub, Package: "taggedalgebraic", Version: ">=0.11.0 <0.12.0", Floating: true},
	})
	langtest.CheckImports(t, results["tools/dub.sdl"], map[string]lang.Target{
		"shop":     {Local: "."},
		"colorize": colorize,
	})
	lines := map[string]int{}
	for _, imported := range results["dub.json"].Imports {
		lines[imported.Spec] = imported.Line
	}
	if lines["vibe-d"] != 5 || lines["darg"] != 21 || lines["tools/"] != 23 || lines["requests"] != 27 {
		t.Errorf("lines %v", lines)
	}
	for _, imported := range results["libs/widgets/dub.sdl"].Imports {
		if imported.Spec == "taggedalgebraic" && imported.Line != 8 {
			t.Errorf("taggedalgebraic on line %d", imported.Line)
		}
	}
}

// Verifies: REQ-DLANG-006
func TestPinRule(t *testing.T) {
	for spec, want := range map[string]lang.Target{
		"==1.2.3":        {Version: "1.2.3", Pinned: true},
		"1.2.3":          {Version: "1.2.3", Pinned: true}, // dub reads a bare version as ==
		"~>1.2.3":        {Version: "~>1.2.3", Floating: true},
		"^1.2.3":         {Version: "^1.2.3", Floating: true},
		">=1.0.0 <2.0.0": {Version: ">=1.0.0 <2.0.0", Floating: true},
		"*":              {Version: "*", Floating: true},
		"~master":        {Version: "~master", Floating: true},
		"":               {Floating: true},
	} {
		var got lang.Target
		pinRule(&got, spec, false)
		if got != want {
			t.Errorf("%q: got %+v, want %+v", spec, got, want)
		}
	}
	var got lang.Target
	if pinRule(&got, "main", true); got.Pinned || !got.Floating {
		t.Errorf("a branch of a repository: %+v", got)
	}
}

// Verifies: REQ-DLANG-003, REQ-DLANG-011
func TestSymbols(t *testing.T) {
	results := analyze(t)
	langtest.CheckSymbols(t, results["source/shop/cart.d"], map[string]string{
		"shop.cart":       "module",
		"TaxRate":         "const",
		"MaxItems":        "const",
		"MinItems":        "const",
		"isItem":          "const",
		"Currency":        "enum",
		"Price":           "alias",
		"Quantity":        "alias",
		"Priced":          "interface",
		"Priced.price":    "method",
		"Cart":            "class",
		"Cart.Line":       "struct",
		"Cart.Line.total": "method",
		"Cart.this":       "method",
		"Cart.~this":      "method",
		"Cart.length":     "method",
		"Cart.price":      "method",
		"Cart.add":        "method",
		"Amount":          "union",
		"Box":             "template",
		"Box.Box":         "struct",
		"Logged":          "mixin template",
		"Logged.log":      "method",
		"platform":        "func", // version (Windows) { }
		"platform@76":     "func", // else { }
		"zero":            "func", // static if
		"total":           "func",
		"shop_version":    "func",
	})
	langtest.CheckSymbols(t, results["source/shop/app.d"], map[string]string{
		"shop.app": "module",
		"code":     "const",
		"main":     "func",
	})
	langtest.CheckSymbols(t, results["dub.json"], map[string]string{})
}

// Nothing in a comment, a string of any kind, a token string or a character
// literal is read as code; an import inside a function is.
//
// Verifies: REQ-DLANG-002, REQ-DLANG-010, REQ-DLANG-011
func TestLiteralsHideCode(t *testing.T) {
	source := "module m;\n" +
		"/+ /+ nested +/ import a1; +/\n" +
		"/* import a2; */ // import a3;\n" +
		"enum t = q{ import a4; void f() { string s = \"}\"; } };\n" +
		"string s1 = \"import a5; \\\" still\";\n" +
		"string s2 = `import a6;`;\n" +
		"string s3 = r\"import a7;\\\";\n" +
		"string s4 = q\"[import [a8];]\";\n" +
		"string s5 = q\"EOS\nimport a9;\nEOS\";\n" +
		"string s6 = q\"/import a10;/\";\n" +
		"char c1 = '\"'; char c2 = '\\''; char c3 = 'é';\n" +
		"auto x = x\"0A 0B\";\n" +
		"void f() {\n\timport real.one : a, b = c;\n\tstatic import real.two, alias3 = real.three;\n}\n" +
		"__EOF__\nimport after.eof;\n"
	extraction := extractSource([]byte(source))
	var got []string
	for _, rawImport := range extraction.Imports {
		got = append(got, rawImport.Module)
	}
	if want := []string{"real.one", "real.two", "real.three"}; !reflect.DeepEqual(got, want) {
		t.Errorf("imports %v, want %v", got, want)
	}
	if extraction.Imports[0].Line != 16 {
		t.Errorf("line %d", extraction.Imports[0].Line)
	}
}

// Verifies: REQ-DLANG-001
func TestClaims(t *testing.T) {
	claimed := map[string]bool{}
	for _, f := range langtest.Files(t, "testdata/repo") {
		if (Plugin{}).Claims(f) {
			claimed[f.Path] = true
		}
	}
	want := []string{"cli/commands.d", "dub.json", "dub.selections.json", "libs/widgets/dub.sdl",
		"libs/widgets/source/widgets/button.d", "scripts/hello.d", "source/shop/app.d", "source/shop/cart.d",
		"source/shop/models/package.d", "source/shop/models/user.d", "tools/dub.sdl", "tools/source/tool.d"}
	if got := sortedKeys(claimed); !reflect.DeepEqual(got, want) {
		t.Errorf("claimed %v, want %v", got, want)
	}
	for _, f := range []*scan.File{
		{Path: "import/shop.di"},
		{Path: "a/.dub/packages/x/1.0.0/x/source/x.d", Language: "D"},
		{Path: "deps/app.d", Language: "Make"},
		{Path: "probes/trace.d", Language: "DTrace"},
		{Path: "src/x.d", Language: "D", Binary: true},
	} {
		if got, want := (Plugin{}).Claims(f), f.Path == "import/shop.di"; got != want {
			t.Errorf("%s: claimed %v", f.Path, got)
		}
	}
	if Plugin.Class(Plugin{}, &scan.File{Path: "x/dub.sdl"}) != classSDL || Plugin.Class(Plugin{}, &scan.File{Path: "x/app.d"}) != "" {
		t.Error("class")
	}
}

// Verifies: REQ-DLANG-005
func TestSDL(t *testing.T) {
	source := "name \"a\" // c\n# c\n-- c\n/* block\n */dependency \"b\" version=\"~>1\" \\\n  optional=true; dependency `c` path=\"../c\"\n" +
		"configuration \"x\" {\n\tdependency \"d\" version=\"1.0.0\"\n\tsubConfiguration \"b\" \"y\"\n}\nsourcePaths \"s1\" \"s2\" platform=\"posix\"\n" +
		"subPackage {\n\tname \"inner\"\n\tdependency \"e\" version=\"*\"\n}\nsubPackage \"./sub/\"\nstringImportPaths \"views\" \"more\"\n" +
		"description \"a \\\"quoted\\\" \\\n    text\"\n"
	r := readSDLRecipe([]byte(source))
	var dependencies []string
	for _, d := range r.dependencyList {
		dependencies = append(dependencies, d.name+"@"+d.version+"@"+d.path)
	}
	if want := []string{"b@~>1@", "c@@../c", "d@1.0.0@"}; !reflect.DeepEqual(dependencies, want) {
		t.Errorf("deps %v", dependencies)
	}
	if r.name != "a" || !r.dependencyList[0].optional || r.dependencyList[0].line != 5 || r.dependencyList[1].line != 6 {
		t.Errorf("recipe %+v %+v", r, r.dependencyList[0])
	}
	if !reflect.DeepEqual(r.sourcePaths, []string{"s1", "s2"}) || !reflect.DeepEqual(r.stringPaths, []string{"views", "more"}) {
		t.Errorf("paths %v %v", r.sourcePaths, r.stringPaths)
	}
	if len(r.subs) != 2 || r.subs[0].inline == nil || r.subs[0].inline.name != "inner" || r.subs[1].path != "./sub/" {
		t.Errorf("subs %+v", r.subs)
	}
	tags := readSDL([]byte("description \"a \\\"quoted\\\" \\\n    text\"\n"))
	if len(tags) != 1 || tags[0].value() != `a "quoted" text` {
		t.Errorf("continued string %+v", tags[0])
	}
}

// installDub lays out a DUB_HOME with ddata 1.2.0 (dub 1.31's layout), an
// older ddata 1.1.0 (the old layout) and its dependency tree as dub fetched it.
func installDub(t *testing.T) string {
	home := t.TempDir()
	write := func(p, content string) {
		absolute := filepath.Join(home, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(absolute), 0o755)
		os.WriteFile(absolute, []byte(content), 0o644)
	}
	write("packages/ddata/1.2.0/ddata/dub.json", `{"name": "ddata", "importPaths": ["lib"], "sourcePaths": ["lib"],
"dependencies": {"dyaml": "~>0.9", "vibe-d:data": "*", ":internal": "*", "extra": {"version": "*", "optional": true}},
"subPackages": ["internal"]}`)
	write("packages/ddata/1.2.0/ddata/lib/datastructs/tree.d", "module datastructs.tree;\n")
	write("packages/ddata/1.2.0/ddata/lib/datastructs/package.d", "module datastructs;\n")
	write("packages/ddata/1.2.0/ddata/internal/dub.sdl", "name \"internal\"\ndependency \"taggedalgebraic\" version=\"~>0.11\"\n")
	write("packages/ddata/1.2.0/ddata/internal/source/ddinternal/util.d", "module ddinternal.util;\n")
	write("packages/ddata-1.1.0/ddata/dub.json", `{"name": "ddata", "dependencies": {"old": "*"}}`)
	write("packages/ddata-1.1.0/ddata/source/datastructs/old.d", "module datastructs.old;\n")
	return home
}

// Verifies: REQ-DLANG-008
func TestInstalledPackages(t *testing.T) {
	root := t.TempDir()
	write := func(p, content string) {
		absolute := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(absolute), 0o755)
		os.WriteFile(absolute, []byte(content), 0o644)
	}
	write("dub.sdl", "name \"app\"\ndependency \"ddata\" version=\"~>1.1\"\ndependency \"dyaml\" version=\"~>0.9\"\n")
	write("dub.selections.json", `{"fileVersion": 1, "versions": {"ddata": "1.2.0", "dyaml": "0.9.2", "vibe-d": "0.9.7"}}`)
	write("source/app.d", "import datastructs.tree;\nimport datastructs;\nimport ddinternal.util;\nimport datastructs.old;\n")
	for i, home := range []string{t.TempDir(), installDub(t)} {
		t.Setenv("DUB_HOME", home)
		files := langtest.Files(t, root)
		r := newResolver(root, files)
		resolveImport := func(m string) lang.Target { return r.Resolve("source/app.d", lang.RawImport{Module: m}) }
		ddata := lang.Target{Ecosystem: ecosystemDub, Package: "ddata", Version: "1.2.0", Requested: "~>1.1", Pinned: true}
		if i == 0 {
			// Nothing installed: the module names no declared package.
			if got := resolveImport("datastructs.tree"); got != (lang.Target{Ecosystem: ecosystemDub, Package: "datastructs", Unresolved: true}) {
				t.Errorf("not installed: %+v", got)
			}
			if r.Installed(ddata) || r.Dependencies(ddata) != nil {
				t.Error("nothing is installed")
			}
			continue
		}
		for _, m := range []string{"datastructs.tree", "datastructs", "ddinternal.util"} {
			if got := resolveImport(m); got != ddata {
				t.Errorf("%s: %+v", m, got)
			}
		}
		// Only the selected version is read.
		if got := resolveImport("datastructs.old"); got.Package != "datastructs" || !got.Unresolved {
			t.Errorf("datastructs.old: %+v", got)
		}
		want := []lang.Target{
			{Ecosystem: ecosystemDub, Package: "dyaml", Version: "0.9.2", Pinned: true},
			{Ecosystem: ecosystemDub, Package: "vibe-d", Version: "0.9.7", Pinned: true},
			{Ecosystem: ecosystemDub, Package: "taggedalgebraic", Version: "~>0.11", Floating: true},
		}
		if got := r.Dependencies(ddata); !reflect.DeepEqual(got, want) {
			t.Errorf("dependencies %+v, want %+v", got, want)
		}
		if !r.Installed(ddata) || r.Installed(lang.Target{Ecosystem: ecosystemDub, Package: "dyaml"}) {
			t.Error("installed")
		}
	}
}

// Every prefix of every fixture file extracts, and long runs of the tokens
// that open or close something stay linear.
//
// Verifies: REQ-DLANG-010
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
			readSDLRecipe(source[:i])
			singleFile(source[:i])
		}
	}
	for _, unit := range []string{"{", "(", "[", "}", ")", "]", "\"", "`", "'", "q{", "q\"(", "q\"EOS\n", "/+", "+/", "/*",
		"import ", "import(", "import a.", "import a :", "static ", "@", "@a(", "class A ", "enum ", "alias ", "private:",
		"version(X) ", "void f() ", "q{x\"", "q{r\"", "x = ", "a : ", "~this", "this(", "mixin template ", "é", "#line 1\n", "\\\n", "=", ","} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extractSource(source)
		readSDLRecipe(source)
		if d := time.Since(start); d > langtest.TimeLimit(5*time.Second) {
			t.Errorf("%q x %d: %v", unit, len(source)/len(unit), d)
		}
	}
}

// Verifies: REQ-DLANG-009
func TestIslands(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range (Plugin{}).Ecosystems() {
		ids[e.ID] = e.Std
	}
	if std, ok := ids[ecosystemStd]; !ok || !std || len(ids) != 2 || ids[ecosystemDub] {
		t.Fatalf("ecosystems: %v", ids)
	}
	for f, r := range analyze(t) {
		for _, imported := range r.Imports {
			if e := imported.Target.Ecosystem; e != "" && e != ecosystemDub && e != ecosystemStd {
				t.Errorf("%s: %s -> %s", f, imported.Spec, e)
			}
		}
	}
}
