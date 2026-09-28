package javascript

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo: a TypeScript app with tsconfig paths (inherited baseUrl, JSONC),
// a workspace package, a lockfile, and a CommonJS corner.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-JS-002, REQ-JS-003, REQ-JS-004, REQ-JS-005, REQ-JS-006, REQ-JS-007, REQ-JS-010
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

// Verifies: REQ-JS-001
func TestCommonJSAndDynamicImports(t *testing.T) {
	got := langtest.Imports(t, analyze(t)["legacy/app.js"])
	if got["fs"] != (lang.Target{Ecosystem: "node", Package: "fs"}) {
		t.Errorf("require('fs'): %+v", got["fs"])
	}
	if got["./lazy.mjs"] != (lang.Target{Local: "legacy/lazy.mjs"}) {
		t.Errorf("import('./lazy.mjs'): %+v", got["./lazy.mjs"])
	}
}

// Verifies: REQ-LANG-023, REQ-LANG-024, REQ-JS-011
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

// Verifies: REQ-JS-008, REQ-JS-009, REQ-SUP-003, REQ-SUP-007
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
//
// Verifies: REQ-SUP-009
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
	// Lock files contain cycles; the walk has to survive one.
	if next := deps("js-tokens"); next["cyclic-a"].Version != "1.0.0" {
		t.Errorf("js-tokens depends on %+v, want cyclic-a", next)
	}
	if next := deps("cyclic-b"); next["cyclic-a"].Version != "1.0.0" {
		t.Errorf("cyclic-b depends on %+v, want cyclic-a back again", next)
	}
	// Another ecosystem's packages are not this resolver's business.
	if n := len(tr.Dependencies(lang.Target{Ecosystem: "pypi", Package: "react"})); n != 0 {
		t.Errorf("answered for %d pypi dependencies", n)
	}
}

// TestYarnEntryKeys checks the two places a classic yarn.lock is read: a dependency
// whose name begins with "version" must not be mistaken for the entry's own version,
// which is only told apart by how deep it is indented.
//
// Verifies: REQ-SUP-009, REQ-JS-008
func TestYarnEntryKeys(t *testing.T) {
	lock := "lodash@^4.17.0:\n  version \"4.17.21\"\n  resolved \"https://registry.npmjs.org/lodash\"\n" +
		"  dependencies:\n    version-guard \"^1.1.1\"\n    js-tokens \"^4\"\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "yarn.lock")
	if err := os.WriteFile(path, []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readYarnLock([]byte(lock))["lodash@^4.17.0"]; got != "4.17.21" {
		t.Errorf("locked version %q, want 4.17.21", got)
	}
	tr := newTree()
	tr.addYarnTree([]byte(lock))
	if got := tr.locked["lodash"]; got != "4.17.21" {
		t.Errorf("tree version %q, want 4.17.21", got)
	}
	for _, dep := range []string{"version-guard", "js-tokens"} {
		if !tr.deps["lodash"][dep] {
			t.Errorf("%s is not among lodash's dependencies: %v", dep, tr.deps["lodash"])
		}
	}
}

// TestPnpmKeys covers both spellings: version 5 put the version after a slash,
// version 6 and later after an @, with peer context in parentheses.
//
// Verifies: REQ-SUP-009
func TestPnpmKeys(t *testing.T) {
	for _, c := range []struct{ key, name, version string }{
		{"/lodash/4.17.21", "lodash", "4.17.21"},
		{"/@scope/pkg/1.2.3", "@scope/pkg", "1.2.3"},
		{"/lodash@4.17.21", "lodash", "4.17.21"},
		{"/@scope/pkg@1.2.3", "@scope/pkg", "1.2.3"},
		{"react-dom@18.3.1(react@18.3.1)", "react-dom", "18.3.1"},
		// A path that names no version is a name.
		{"/@scope/pkg", "@scope/pkg", ""},
	} {
		name, version := pnpmKey(c.key)
		if name != c.name || version != c.version {
			t.Errorf("%s: got %q %q, want %q %q", c.key, name, version, c.name, c.version)
		}
	}
}

// Verifies: REQ-JS-002, REQ-JS-006, REQ-JS-012, REQ-JS-013
func TestComponentImports(t *testing.T) {
	res := analyze(t)
	vue := lang.Target{Ecosystem: "npm", Package: "vue", Unresolved: true}
	// Both script blocks count; none of the <script> text in the template, the
	// comment, the mustache or the style does.
	langtest.CheckImports(t, res["src/ui/Counter.vue"], map[string]lang.Target{
		"vue":           vue,
		"./Child.vue":   {Local: "src/ui/Child.vue"},
		"@app/lib/math": {Local: "src/lib/math.ts"},
		"chalk":         {Ecosystem: "npm", Package: "chalk", Version: "^5.3.0"},
	})
	langtest.CheckImports(t, res["src/ui/Child.vue"], map[string]lang.Target{
		"./child.ts": {Local: "src/ui/child.ts"},
	})
	// The module script and the instance script; not the scripts in <svelte:head>,
	// in an attribute or in an expression.
	langtest.CheckImports(t, res["src/ui/Widget.svelte"], map[string]lang.Target{
		"./types":       {Local: "src/ui/types.ts"},
		"./Counter.vue": {Local: "src/ui/Counter.vue"},
		"svelte":        {Ecosystem: "npm", Package: "svelte", Unresolved: true},
	})
	// The frontmatter and the processed template scripts; inline and data scripts
	// are left as written by Astro.
	langtest.CheckImports(t, res["src/pages/index.astro"], map[string]lang.Target{
		"../ui/Widget.svelte": {Local: "src/ui/Widget.svelte"},
		"node:path":           {Ecosystem: "node", Package: "path"},
		"@app/lib/math":       {Local: "src/lib/math.ts"},
		"astro:content":       {},
		"react":               {Ecosystem: "npm", Package: "react", Version: "18.3.1", Requested: "^18.2.0", Pinned: true},
		"../ui/child.ts":      {Local: "src/ui/child.ts"},
	})
	// Components are imported by their full name from JavaScript and TypeScript.
	langtest.CheckImports(t, res["src/ui/main.ts"], map[string]lang.Target{
		"./Counter.vue":        {Local: "src/ui/Counter.vue"},
		"./Widget.svelte":      {Local: "src/ui/Widget.svelte"},
		"../pages/index.astro": {Local: "src/pages/index.astro"},
		"./Counter":            {Local: "src/ui/Counter.vue"},
	})
}

// TestComponentLines checks that what a block declares and imports is placed on
// the component's own lines, not the block's.
//
// Verifies: REQ-JS-012, REQ-JS-014
func TestComponentLines(t *testing.T) {
	res := analyze(t)
	lines := map[string]int{}
	for _, file := range []string{"src/ui/Counter.vue", "src/ui/Child.vue", "src/pages/index.astro"} {
		for _, im := range res[file].Imports {
			lines[file+" "+im.Spec] = im.Line
		}
	}
	for key, want := range map[string]int{
		"src/ui/Counter.vue ./Child.vue":       18,
		"src/ui/Counter.vue chalk":             20,
		"src/ui/Child.vue ./child.ts":          2,
		"src/pages/index.astro node:path":      3,
		"src/pages/index.astro react":          18,
		"src/pages/index.astro ../ui/child.ts": 22,
	} {
		if lines[key] != want {
			t.Errorf("%s: line %d, want %d", key, lines[key], want)
		}
	}
	symbolLines := map[string]int{}
	for _, s := range res["src/ui/Counter.vue"].Symbols {
		symbolLines[s.Name] = s.Line
	}
	if symbolLines["increment"] != 24 || symbolLines["Counter"] != 1 {
		t.Errorf("Counter.vue symbols at %v, want increment on 24 and the component on 1", symbolLines)
	}
}

// Verifies: REQ-JS-014
func TestComponentSymbols(t *testing.T) {
	res := analyze(t)
	for file, want := range map[string]map[string]string{
		"src/ui/Counter.vue":   {"Counter": "component", "count": "var", "closing": "var", "increment": "func"},
		"src/ui/Child.vue":     {"Child": "component"},
		"src/ui/Widget.svelte": {"Widget": "component", "preload": "func", "name": "var", "greet": "func"},
		// Destructured names (const { title } = Astro.props) are not symbols, as
		// in any script.
		"src/pages/index.astro": {"index": "component", "heading": "func"},
	} {
		if got := langtest.Symbols(t, res[file]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: symbols %v, want %v", file, got, want)
		}
	}
}

// Verifies: REQ-JS-015, REQ-JS-006
func TestSvelteKitAliases(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/sveltekit")
	langtest.CheckImports(t, res["src/routes/+page.svelte"], map[string]lang.Target{
		"$lib":                {Local: "src/lib/index.ts"},
		"$lib/format":         {Local: "src/lib/format.ts"},
		"$app/navigation":     {},
		"$env/dynamic/public": {},
		"@sveltejs/kit":       {Ecosystem: "npm", Package: "@sveltejs/kit", Version: "^2.0.0"},
	})
}

// TestComponentScanner covers blocks the fixtures do not: other script languages,
// data blocks, and components cut off anywhere - the scanner must neither panic
// nor read past the end.
//
// Verifies: REQ-JS-012, REQ-JS-013
func TestComponentScanner(t *testing.T) {
	blocks := func(ext, src string) []string {
		var out []string
		for _, s := range componentScripts(ext, []byte(src)) {
			out = append(out, src[s.start:s.end]+"|"+s.src)
		}
		return out
	}
	for _, c := range []struct {
		ext, src string
		want     []string
	}{
		{".vue", `<script lang="coffee">x = 1</script><script>a</script>`, []string{"a|"}},
		{".vue", `<script type="text/x-template">t</script><SCRIPT LANG="TS">b</Script >`, []string{"b|"}},
		{".vue", `<script setup lang="ts" generic="T extends Record<string, any>">c</script>`, []string{"c|"}},
		{".svelte", `<script>d</scripts></script>`, []string{"d</scripts>|"}},
		{".svelte", "<p>{'}'}</p><script>e</script>", []string{"e|"}},
		{".astro", "\n---\nf\n---\n<script src=\"./g.ts\" />", []string{"f\n|", "|./g.ts"}},
		{".astro", "<p>no frontmatter</p><script>h</script>", []string{"h|"}},
	} {
		if got := blocks(c.ext, c.src); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %q: blocks %q, want %q", c.ext, c.src, got, c.want)
		}
	}
	for _, file := range []string{"testdata/repo/src/ui/Counter.vue", "testdata/repo/src/ui/Widget.svelte", "testdata/repo/src/pages/index.astro"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for n := range src {
			for _, s := range componentScripts(filepath.Ext(file), src[:n]) {
				if s.start > s.end || s.end > n {
					t.Fatalf("%s cut at %d: block %d..%d", file, n, s.start, s.end)
				}
			}
		}
	}
}

// Qt projects keep their translations in ".ts" files (Qt Linguist XML). Parsed as
// TypeScript, each one ran into the per-file parse bound (REQ-LANG-011) and
// yielded nothing: flameshot's 49 translations took 39 s of a 40 s analysis. They
// must not reach the plugin at all.
//
// Verifies: REQ-JS-001
func TestQtLinguistTranslationsNotClaimed(t *testing.T) {
	root := t.TempDir()
	var ts strings.Builder
	ts.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<!DOCTYPE TS>\n<TS version=\"2.1\" language=\"de\">\n<context>\n    <name>Main</name>\n")
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&ts, "    <message>\n        <location filename=\"../../src/main.cpp\" line=\"%d\"/>\n        <source>Save &amp; close</source>\n        <translation>Speichern &amp; schließen</translation>\n    </message>\n", i)
	}
	ts.WriteString("</context>\n</TS>\n")
	for p, c := range map[string]string{
		"data/translations/app_de.ts": ts.String(),
		"src/app.ts":                  "import './util';\n",
		"src/util.ts":                 "export const x = 1;\n",
	} {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range lang.Claimed(Plugin{}, files) {
		got = append(got, f.Path)
	}
	sort.Strings(got)
	if want := []string{"src/app.ts", "src/util.ts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("claimed %v, want %v", got, want)
	}
}

// testdata/bun: a Bun workspace whose bun.lock holds registry packages (scoped
// and not), a react that the ui workspace has installed under itself at another
// major version, a GitHub dependency at a commit, an npm alias, a tarball, a
// workspace entry, trailing commas and a comment.
//
// Verifies: REQ-JS-016
func TestBunLock(t *testing.T) {
	npm := func(pkg, version, requested string) lang.Target {
		return lang.Target{Ecosystem: "npm", Package: pkg, Version: version, Requested: requested, Pinned: true}
	}
	res := langtest.Analyze(t, Plugin{}, "testdata/bun")
	langtest.CheckImports(t, res["index.js"], map[string]lang.Target{
		"react":           npm("react", "18.3.1", "^18.2.0"),
		"@scope/tool/sub": npm("@scope/tool", "1.2.0", "^1.0.0"),
		// The lock names the commit Bun fetched for the tag package.json asks for.
		"forge-std": npm("forge-std", "github:foundry-rs/forge-std#1eea5ba", "github:foundry-rs/forge-std#v1.9.4"),
		"aliased":   npm("aliased", "3.0.1", "npm:real-name@^3.0.0"),
		// A tarball names no version: the URL package.json gives is all there is.
		"tarball-pkg": {Ecosystem: "npm", Package: "tarball-pkg", Version: "https://example.com/tarball-pkg-1.0.0.tgz"},
		"ui":          {Local: "packages/ui"},
		// Hoisted in the lock but declared by nobody.
		"object-assign": {Ecosystem: "npm", Package: "object-assign", Unresolved: true},
	})
	langtest.CheckImports(t, res["packages/ui/index.js"], map[string]lang.Target{
		// "ui/react" is installed under the workspace: it wins over the hoisted 18.
		"react":       npm("react", "17.0.2", "^17.0.0"),
		"@scope/tool": npm("@scope/tool", "1.2.0", "^1.1.0"),
	})
}

// TestBunLockTree checks the edges --resolve-depth follows out of bun.lock, and
// that a package installed under its dependent answers for that dependent.
//
// Verifies: REQ-SUP-009, REQ-JS-016
func TestBunLockTree(t *testing.T) {
	r, err := (Plugin{}).Resolver("testdata/bun", langtest.Files(t, "testdata/bun"))
	if err != nil {
		t.Fatal(err)
	}
	tr := r.(lang.Transitive)
	deps := func(pkg string) map[string]lang.Target {
		out := map[string]lang.Target{}
		for _, d := range tr.Dependencies(lang.Target{Ecosystem: "npm", Package: pkg}) {
			out[d.Package] = d
		}
		return out
	}
	pinned := func(pkg, version string) lang.Target {
		return lang.Target{Ecosystem: "npm", Package: pkg, Version: version, Pinned: true}
	}
	for _, c := range []struct {
		pkg  string
		want map[string]lang.Target
	}{
		// ui's react 17 adds object-assign to the one react node.
		{"react", map[string]lang.Target{"loose-envify": pinned("loose-envify", "1.4.0"), "object-assign": pinned("object-assign", "4.1.1")}},
		// @scope/dep 1.0.3 sits under @scope/tool; the hoisted one is 2.1.0.
		{"@scope/tool", map[string]lang.Target{"@scope/dep": pinned("@scope/dep", "1.0.3"), "loose-envify": pinned("loose-envify", "1.4.0")}},
		{"loose-envify", map[string]lang.Target{"js-tokens": pinned("js-tokens", "4.0.0")}},
		{"tarball-pkg", map[string]lang.Target{"js-tokens": pinned("js-tokens", "4.0.0")}},
		{"forge-std", map[string]lang.Target{}},
	} {
		if got := deps(c.pkg); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s depends on %+v, want %+v", c.pkg, got, c.want)
		}
	}
}

// TestBunLockPrecedence: beside package-lock.json, bun.lock only answers for what
// the other lock does not hold, so reading it never changes an earlier answer.
//
// Verifies: REQ-JS-016
func TestBunLockPrecedence(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"package.json":      `{"dependencies":{"react":"^18.2.0","chalk":"^5.3.0"}}`,
		"package-lock.json": `{"lockfileVersion":3,"packages":{"":{},"node_modules/react":{"version":"18.2.0"}}}`,
		"bun.lock": `{"lockfileVersion":1,"workspaces":{"":{"dependencies":{"react":"^18.2.0","chalk":"^5.3.0",},},},
			"packages":{"react":["react@18.3.1","",{},"sha512-a"],"chalk":["chalk@5.3.0","",{},"sha512-b"],},}`,
		"index.js": "import 'react';\nimport 'chalk';\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, dir)["index.js"], map[string]lang.Target{
		"react": {Ecosystem: "npm", Package: "react", Version: "18.2.0", Requested: "^18.2.0", Pinned: true},
		"chalk": {Ecosystem: "npm", Package: "chalk", Version: "5.3.0", Requested: "^5.3.0", Pinned: true},
	})
}

// TestBunLockShapes covers the pieces of a bun.lock read one at a time, and
// input that is not a bun.lock at all.
//
// Verifies: REQ-JS-016
func TestBunLockShapes(t *testing.T) {
	for _, c := range []struct{ key, parent, name string }{
		{"react", "", "react"},
		{"@scope/tool", "", "@scope/tool"},
		{"ui/react", "ui", "react"},
		{"@scope/tool/@scope/dep", "@scope/tool", "@scope/dep"},
		{"@a/b/c/@d/e", "@a/b/c", "@d/e"},
	} {
		if parent, name := bunKey(c.key); parent != c.parent || name != c.name {
			t.Errorf("bunKey(%q) = %q %q, want %q %q", c.key, parent, name, c.parent, c.name)
		}
	}
	for _, c := range []struct{ ident, version string }{
		{"react@18.3.1", "18.3.1"},
		{"@scope/tool@1.2.0-beta.1", "1.2.0-beta.1"},
		{"forge-std@github:foundry-rs/forge-std#1eea5ba", "github:foundry-rs/forge-std#1eea5ba"},
		{"x@git+https://github.com/o/x.git#0123456789abcdef0123456789abcdef01234567", "git+https://github.com/o/x.git#0123456789abcdef0123456789abcdef01234567"},
		// A git resolution to anything but a commit, and the project's own packages, pin nothing.
		{"x@github:o/x#main", ""},
		{"x@github:o/x", ""},
		{"ui@workspace:packages/ui", ""},
		{"linked@link:linked", ""},
		{"folder@file:../folder", ""},
		{"@solidjs/start@https://pkg.pr.new/@solidjs/start@dfb2020", ""},
		{"@x", ""},
		{"", ""},
	} {
		if got := bunVersion(bunResolution(c.ident)); got != c.version {
			t.Errorf("%q: version %q, want %q", c.ident, got, c.version)
		}
	}
	for _, bad := range []string{"", "{", `{"packages":[]}`, `{"workspaces":{"":{"dependencies":{"a":1}}}}`} {
		if _, err := readBunLock([]byte(bad)); err == nil {
			t.Errorf("%q read as a bun.lock", bad)
		}
	}
	// Entries of another shape are skipped, not guessed at.
	lock, err := readBunLock([]byte(`{"packages":{"a":"a@1.0.0","b":[],"c":[1],"d":["d@1.0.0",[],"x",{"dependencies":{"e":"^1"}}],"":["@1"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	got := lock.entries()
	want := map[string]bunEntry{"d": {name: "d", version: "1.0.0", deps: []string{"e"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entries %+v, want %+v", got, want)
	}
}

// TestBunLockTruncated feeds every prefix of the fixture's bun.lock through the
// reader: a cut file is an error or a smaller lock, never a panic.
//
// Verifies: REQ-JS-016
func TestBunLockTruncated(t *testing.T) {
	data, err := os.ReadFile("testdata/bun/bun.lock")
	if err != nil {
		t.Fatal(err)
	}
	for i := range data {
		lock, err := readBunLock(data[:i])
		if err != nil {
			continue
		}
		entries := lock.entries()
		lock.versions(entries)
		newTree().addBunTree(entries)
	}
}

// TestNpmShrinkwrap: npm-shrinkwrap.json is read as package-lock.json is - its
// versions pin, its tree is walked - and beside one in the same directory npm reads
// only it, so package-lock.json there says nothing. Another directory's
// package-lock.json still answers for its own package.
//
// Verifies: REQ-JS-007, REQ-SUP-009
func TestNpmShrinkwrap(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"package.json":          `{"dependencies":{"react":"^18.2.0","chalk":"^5.3.0"}}`,
		"npm-shrinkwrap.json":   `{"lockfileVersion":3,"packages":{"":{},"node_modules/react":{"version":"18.3.1","dependencies":{"loose-envify":"^1.1.0"}},"node_modules/loose-envify":{"version":"1.4.0"}}}`,
		"package-lock.json":     `{"lockfileVersion":3,"packages":{"":{},"node_modules/react":{"version":"18.2.0"},"node_modules/chalk":{"version":"5.3.0"}}}`,
		"index.js":              "import 'react';\nimport 'chalk';\n",
		"app/package.json":      `{"dependencies":{"chalk":"^5.0.0"}}`,
		"app/package-lock.json": `{"lockfileVersion":2,"packages":{"":{},"node_modules/chalk":{"version":"5.2.0"}}}`,
		"app/main.js":           "import 'chalk';\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, filepath.FromSlash(name))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res := langtest.Analyze(t, Plugin{}, dir)
	langtest.CheckImports(t, res["index.js"], map[string]lang.Target{
		"react": {Ecosystem: "npm", Package: "react", Version: "18.3.1", Requested: "^18.2.0", Pinned: true},
		"chalk": {Ecosystem: "npm", Package: "chalk", Version: "^5.3.0"},
	})
	langtest.CheckImports(t, res["app/main.js"], map[string]lang.Target{
		"chalk": {Ecosystem: "npm", Package: "chalk", Version: "5.2.0", Requested: "^5.0.0", Pinned: true},
	})
	r, err := (Plugin{}).Resolver(dir, langtest.Files(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	got := r.(lang.Transitive).Dependencies(lang.Target{Ecosystem: "npm", Package: "react"})
	if len(got) != 1 || got[0].Package != "loose-envify" || got[0].Version != "1.4.0" {
		t.Errorf("react depends on %+v, want loose-envify 1.4.0 from the shrinkwrap", got)
	}
}
