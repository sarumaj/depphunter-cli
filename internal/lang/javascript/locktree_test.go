package javascript

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// lockTrees are the fixtures under testdata/locktree: one project written in
// each lock format, with the same install. a needs d@^2, which only a gets (a
// nested or duplicate copy); b needs the hoisted d@1, a patched f, h under the
// alias h-alias and, optionally, fsevents; the project imports c under the alias
// c2; a workspace (or local directory) ws-lib, and in Berry a portal g, are the
// project's own and never npm packages. fsevents is how the formats differ: npm
// v3 leaves it out of the install, the npm v1 fixture does not ask for it, and
// the others install 2.3.3.
var lockTrees = []struct {
	dir      string
	fsevents bool
}{
	{"npm-v3", false}, {"npm-v1", false}, {"berry", true},
	{"pnpm-v9", true}, {"pnpm-v6", true}, {"pnpm-v5", true},
}

// TestLockTreeCopies checks that each package's dependencies come at the version
// of the copy that package loads, read from node_modules paths, yarn descriptors
// and pnpm's exact references.
//
// Verifies: REQ-SUP-009, REQ-JS-007, REQ-JS-008, REQ-JS-009
func TestLockTreeCopies(t *testing.T) {
	for _, c := range lockTrees {
		t.Run(c.dir, func(t *testing.T) {
			root := "testdata/locktree/" + c.dir
			npm := func(pkg, version, requested string) lang.Target {
				return lang.Target{Ecosystem: "npm", Package: pkg, Version: version, Requested: requested, Pinned: true}
			}
			langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, root)["index.js"], map[string]lang.Target{
				"a":  npm("a", "1.0.0", "^1.0.0"),
				"b":  npm("b", "1.0.0", "^1.0.0"),
				"c2": npm("c2", "2.0.0", "npm:c@^2.0.0"),
			})
			r, err := (Plugin{}).Resolver(root, langtest.Files(t, root))
			if err != nil {
				t.Fatal(err)
			}
			tr := r.(lang.Transitive)
			// Nothing of the project's own is a package: not Berry's __metadata,
			// the root workspace, ws-lib or the portal g.
			for _, n := range []string{"__metadata", "root", "ws-lib", "g"} {
				if v, ok := r.(*resolver).tree.locked[n]; ok {
					t.Errorf("%s locked at %q", n, v)
				}
			}
			// Asked by name alone, npm's d is the hoisted copy.
			if strings.HasPrefix(c.dir, "npm-") {
				for _, d := range tr.Dependencies(lang.Target{Ecosystem: "npm", Package: "b"}) {
					if d.Package == "d" && d.Version != "1.0.0" {
						t.Errorf("b by name needs d %q, want the hoisted 1.0.0", d.Version)
					}
				}
			}
			bDeps := map[string]string{"d": "1.0.0", "f": "1.0.0", "h": "1.0.0"}
			if c.fsevents {
				bDeps["fsevents"] = "2.3.3"
			}
			for _, q := range []struct {
				pkg, version string
				want         map[string]string
			}{
				{"a", "1.0.0", map[string]string{"d": "2.0.0", "e": "1.0.0"}},
				{"b", "1.0.0", bDeps},
				// Imported under its alias, or reached as the real package.
				{"c2", "2.0.0", map[string]string{"d": "1.0.0"}},
				{"c", "2.0.0", map[string]string{"d": "1.0.0"}},
				{"d", "2.0.0", map[string]string{"e": "1.0.0"}},
				// One node stands for every copy of d: the edge the other copy has
				// is kept, at the version the name is locked to.
				{"d", "1.0.0", map[string]string{"e": "1.0.0"}},
				{"e", "1.0.0", map[string]string{}},
			} {
				got := map[string]string{}
				for _, d := range tr.Dependencies(lang.Target{Ecosystem: "npm", Package: q.pkg, Version: q.version}) {
					if d.Pinned != (d.Version != "") {
						t.Errorf("%s@%s -> %s: pinned %v with version %q", q.pkg, q.version, d.Package, d.Pinned, d.Version)
					}
					got[d.Package] = d.Version
				}
				if !reflect.DeepEqual(got, q.want) {
					t.Errorf("%s@%s depends on %v, want %v", q.pkg, q.version, got, q.want)
				}
			}
		})
	}
}

// TestYarnCandidates covers the ways a yarn.lock dependency's range is keyed.
//
// Verifies: REQ-SUP-009
func TestYarnCandidates(t *testing.T) {
	for _, c := range []struct {
		name, rng string
		want      []string
	}{
		{"d", "^1.0.0", []string{"d@^1.0.0", "d@npm:^1.0.0"}},
		{"d", "npm:^1.0.0", []string{"d@npm:^1.0.0"}},
		{"h-alias", "npm:h@^1.0.0", []string{"h-alias@npm:h@^1.0.0"}},
		{"ws", "workspace:^", []string{"ws@workspace:^"}},
		{"x", "https://example.com/x.tgz", []string{"x@https://example.com/x.tgz"}},
		// A patch the lock keys with its locator falls back to the range it patches.
		{"f", "patch:f@npm%3A^1.0.0#./.yarn/patches/f.patch", []string{
			"f@patch:f@npm%3A^1.0.0#./.yarn/patches/f.patch", "f@npm:^1.0.0",
		}},
		{"@s/f", "patch:@s/f@^1#~builtin<compat/f>", []string{
			"@s/f@patch:@s/f@^1#~builtin<compat/f>", "@s/f@^1", "@s/f@npm:^1",
		}},
	} {
		if got := yarnCandidates(c.name, c.rng); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %s: got %q, want %q", c.name, c.rng, got, c.want)
		}
	}
}

// TestYarnDependencyLines reads both spellings of a dependency line.
//
// Verifies: REQ-SUP-009
func TestYarnDependencyLines(t *testing.T) {
	for line, want := range map[string][2]string{
		`lodash "^4.17.0"`:            {"lodash", "^4.17.0"},
		`"@babel/core" "^7.0.0"`:      {"@babel/core", "^7.0.0"},
		`lodash: ^4.17.0`:             {"lodash", "^4.17.0"},
		`"@babel/core": "npm:^7.0.0"`: {"@babel/core", "npm:^7.0.0"},
		`either "^1.0.0 || ^2.0.0"`:   {"either", "^1.0.0 || ^2.0.0"},
		`"broken`:                     {"", ""},
	} {
		if name, rng := yarnDep(line); name != want[0] || rng != want[1] {
			t.Errorf("%s: got %q %q, want %q", line, name, rng, want)
		}
	}
}

// TestYarnClassicCopies reads a classic yarn.lock the same way: an alias's
// descriptor names the real package, and each range picks its own entry.
//
// Verifies: REQ-SUP-009, REQ-JS-008
func TestYarnClassicCopies(t *testing.T) {
	lock := `# yarn lockfile v1


a@^1.0.0:
  version "1.0.0"
  resolved "https://registry.yarnpkg.com/a/-/a-1.0.0.tgz"
  dependencies:
    d "^2.0.0"
    local "file:../local"

"c2@npm:c@^2.0.0":
  version "2.0.0"
  dependencies:
    d "^1.0.0"

d@^1.0.0:
  version "1.0.0"

d@^2.0.0:
  version "2.0.0"
`
	tr := newTree()
	tr.addYarnTree([]byte(lock))
	for key, want := range map[string]map[string]string{
		"a@1.0.0":  {"d": "2.0.0"},
		"c@2.0.0":  {"d": "1.0.0"},
		"c2@2.0.0": {"d": "1.0.0"},
	} {
		if got := tr.exact[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s needs %v, want %v", key, got, want)
		}
	}
	if !tr.deps["c"]["d"] || !tr.deps["c2"]["d"] || tr.deps["a"]["local"] {
		t.Errorf("edges %v", tr.deps)
	}
}
