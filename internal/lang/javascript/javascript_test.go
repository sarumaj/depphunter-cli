package javascript

import (
	"context"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo: a TypeScript app with tsconfig paths (inherited baseUrl, JSONC),
// a workspace package, a lockfile, and a CommonJS corner.
func analyse(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	const root = "testdata/repo"
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := lang.Analyze(context.Background(), Plugin{}, root, files)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func imports(t *testing.T, res *lang.FileResult) map[string]lang.Target {
	t.Helper()
	if res == nil {
		t.Fatal("file not analysed")
	}
	out := map[string]lang.Target{}
	for _, im := range res.Imports {
		out[im.Spec] = im.Target
	}
	return out
}

func TestTypeScriptResolution(t *testing.T) {
	got := imports(t, analyse(t)["src/index.ts"])
	want := map[string]lang.Target{
		"./util.js":        {Local: "src/util.ts"},
		"./util":           {Local: "src/util.ts"},
		"./components":     {Local: "src/components/index.tsx"},
		"@app/lib/math":    {Local: "src/lib/math.ts"},
		"react":            {Ecosystem: "npm", Package: "react", Version: "18.3.1"},
		"react-dom/client": {Ecosystem: "npm", Package: "react-dom", Unresolved: true},
		"node:fs":          {Ecosystem: "node", Package: "fs"},
		"path":             {Ecosystem: "node", Package: "path"},
		"@scope/tool/sub":  {Ecosystem: "npm", Package: "@scope/tool", Version: "1.2.0"},
		"@acme/shared":     {Local: "packages/shared"},
		"./styles.css":     {Local: "src/styles.css"},
		"../outside":       {},
		"~/alias":          {},
	}
	for spec, w := range want {
		g, ok := got[spec]
		if !ok {
			t.Errorf("%s: not captured", spec)
		} else if g != w {
			t.Errorf("%s: got %+v, want %+v", spec, g, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d imports, want %d: %v", len(got), len(want), got)
	}
}

func TestCommonJSAndDynamicImports(t *testing.T) {
	got := imports(t, analyse(t)["legacy/app.js"])
	if got["fs"] != (lang.Target{Ecosystem: "node", Package: "fs"}) {
		t.Errorf("require('fs'): %+v", got["fs"])
	}
	if got["./lazy.mjs"] != (lang.Target{Local: "legacy/lazy.mjs"}) {
		t.Errorf("import('./lazy.mjs'): %+v", got["./lazy.mjs"])
	}
}

func TestSymbols(t *testing.T) {
	res := analyse(t)
	check := func(file string, want map[string]string) {
		t.Helper()
		got := map[string]string{}
		for _, s := range res[file].Symbols {
			got[s.Name] = s.Kind
		}
		for name, kind := range want {
			if got[name] != kind {
				t.Errorf("%s: symbol %s kind %q, want %q (all: %v)", file, name, got[name], kind, got)
			}
		}
	}
	check("src/index.ts", map[string]string{
		"main": "func", "App": "class", "App.render": "method", "Props": "interface",
		"T": "type", "E": "enum", "handler": "func", "X": "var",
	})
	check("src/components/index.tsx", map[string]string{"Button": "func"})
	check("legacy/app.js", map[string]string{"Legacy": "class", "Legacy.start": "method", "lazy": "func", "fs": "var"})
}

func TestStripJSONC(t *testing.T) {
	in := `{"a": "// not a comment", /* c */ "b": [1, 2,], // x
	}`
	if got := string(stripJSONC([]byte(in))); got != "{\"a\": \"// not a comment\",  \"b\": [1, 2], \n\t}" {
		t.Errorf("got %q", got)
	}
}
