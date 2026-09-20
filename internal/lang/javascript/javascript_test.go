package javascript

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: a TypeScript app with tsconfig paths (inherited baseUrl, JSONC),
// a workspace package, a lockfile, and a CommonJS corner.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

func TestTypeScriptResolution(t *testing.T) {
	langtest.CheckImports(t, analyze(t)["src/index.ts"], map[string]lang.Target{
		"./util.js":        {Local: "src/util.ts"},
		"./util":           {Local: "src/util.ts"},
		"./components":     {Local: "src/components/index.tsx"},
		"@app/lib/math":    {Local: "src/lib/math.ts"},
		"react":            {Ecosystem: "npm", Package: "react", Version: "18.3.1", Requested: "^18.2.0", Pinned: true},
		"react-dom/client": {Ecosystem: "npm", Package: "react-dom", Unresolved: true},
		"node:fs":          {Ecosystem: "node", Package: "fs"},
		"path":             {Ecosystem: "node", Package: "path"},
		"@scope/tool/sub":  {Ecosystem: "npm", Package: "@scope/tool", Version: "1.2.0", Requested: "^1.0.0", Pinned: true},
		// Declared but absent from the lock file: the range is all there is.
		"chalk":        {Ecosystem: "npm", Package: "chalk", Version: "^5.3.0"},
		"@acme/shared": {Local: "packages/shared"},
		"./styles.css": {Local: "src/styles.css"},
		"../outside":   {},
		"~/alias":      {},
	})
}

func TestCommonJSAndDynamicImports(t *testing.T) {
	got := langtest.Imports(t, analyze(t)["legacy/app.js"])
	if got["fs"] != (lang.Target{Ecosystem: "node", Package: "fs"}) {
		t.Errorf("require('fs'): %+v", got["fs"])
	}
	if got["./lazy.mjs"] != (lang.Target{Local: "legacy/lazy.mjs"}) {
		t.Errorf("import('./lazy.mjs'): %+v", got["./lazy.mjs"])
	}
}

func TestSymbols(t *testing.T) {
	res := analyze(t)
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

func TestLockfiles(t *testing.T) {
	// Every lock file pins; what package.json asked for stays visible beside it.
	npm := func(pkg, version, requested string) lang.Target {
		return lang.Target{Ecosystem: "npm", Package: pkg, Version: version, Requested: requested, Pinned: true}
	}
	for _, c := range []struct {
		root, file string
		want       map[string]lang.Target
	}{
		// left-pad is locked twice; the range package.json declares picks 1.3.0.
		{"testdata/yarn-classic", "index.js", map[string]lang.Target{"lodash": npm("lodash", "4.17.21", "^4.17.0"), "left-pad": npm("left-pad", "1.3.0", "1.x")}},
		{"testdata/yarn-berry", "index.js", map[string]lang.Target{"react": npm("react", "18.2.0", "^18.2.0")}},
		{"testdata/pnpm", "index.js", map[string]lang.Target{"react": npm("react", "18.3.1", "^18.2.0")}},
		// Per-importer versions; the peer-dependency suffix is dropped.
		{"testdata/pnpm", "packages/ui/index.js", map[string]lang.Target{"react-dom/client": npm("react-dom", "18.3.1", "^18.2.0")}},
	} {
		t.Run(c.root+"/"+c.file, func(t *testing.T) {
			langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, c.root)[c.file], c.want)
		})
	}
}

// TestLockTree checks what the lock file says the packages themselves need: this is
// what --resolve-depth walks, and it must come out of the file alone.
func TestLockTree(t *testing.T) {
	r, err := (Plugin{}).Resolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := r.(lang.Transitive)
	if !ok {
		t.Fatal("the resolver cannot answer for transitive dependencies")
	}
	deps := func(pkg string) map[string]lang.Target {
		out := map[string]lang.Target{}
		for _, d := range tr.Dependencies(lang.Target{Ecosystem: "npm", Package: pkg}) {
			out[d.Package] = d
		}
		return out
	}
	react := deps("react")
	if got, ok := react["loose-envify"]; !ok || got.Version != "1.4.0" || !got.Pinned {
		t.Errorf("react depends on %+v, want loose-envify 1.4.0 pinned", react)
	}
	if next := deps("loose-envify"); next["js-tokens"].Version != "4.0.0" {
		t.Errorf("loose-envify depends on %+v, want js-tokens 4.0.0", next)
	}
	if n := len(deps("js-tokens")); n != 0 {
		t.Errorf("js-tokens has %d dependencies, want none", n)
	}
	// Another ecosystem's packages are not this resolver's business.
	if n := len(tr.Dependencies(lang.Target{Ecosystem: "pypi", Package: "react"})); n != 0 {
		t.Errorf("answered for %d pypi dependencies", n)
	}
}
