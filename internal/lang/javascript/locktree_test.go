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
// project's own and never npm packages. Where the formats differ: fsevents, a
// macOS-only optional dependency, is one the npm v1 fixture does not ask for and
// the others install at 2.3.3; the pnpm v5 and v6 fixtures have no workspace for
// a to depend on; and the npm v3 fixture has peer dependencies, an optional one
// that is installed (b's e), one that is not, and a required one (e's d).
var lockTrees = []struct {
	directory                  string
	fsevents, workspace, peers bool
}{
	{"npm-v3", true, true, true}, {"npm-v1", false, true, false}, {"berry", true, true, false},
	{"pnpm-v9", true, true, false}, {"pnpm-v6", true, false, false}, {"pnpm-v5", true, false, false},
}

// TestLockTreeCopies checks that each package's dependencies come at the version
// of the copy that package loads, read from node_modules paths, yarn descriptors
// and pnpm's exact references; that a dependency on the workspace is its
// directory, npm's peer dependencies are edges, and fsevents says where it
// installs.
//
// Verifies: REQ-SUP-009, REQ-JS-004, REQ-JS-007, REQ-JS-008, REQ-JS-009, REQ-JS-018
func TestLockTreeCopies(t *testing.T) {
	for _, c := range lockTrees {
		t.Run(c.directory, func(t *testing.T) {
			root := "testdata/locktree/" + c.directory
			npm := func(packageName, version, requested string) lang.Target {
				return lang.Target{Ecosystem: "npm", Package: packageName, Version: version, Requested: requested, Pinned: true}
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
			transitive := r.(lang.Transitive)
			// Nothing of the project's own is a package: not Berry's __metadata,
			// the root workspace, ws-lib or the portal g.
			for _, n := range []string{"__metadata", "root", "ws-lib", "g"} {
				if v, ok := r.(*resolver).tree.locked[n]; ok {
					t.Errorf("%s locked at %q", n, v)
				}
			}
			// Asked by name alone, npm's d is the hoisted copy.
			if strings.HasPrefix(c.directory, "npm-") {
				for _, d := range transitive.Dependencies(lang.Target{Ecosystem: "npm", Package: "b"}) {
					if d.Package == "d" && d.Version != "1.0.0" {
						t.Errorf("b by name needs d %q, want the hoisted 1.0.0", d.Version)
					}
				}
			}
			bDependencies := map[string]string{"d": "1.0.0", "f": "1.0.0", "h": "1.0.0"}
			if c.fsevents {
				bDependencies["fsevents"] = "2.3.3"
			}
			// A workspace package is the project's directory, not an npm package.
			aDependencies := map[string]string{"d": "2.0.0", "e": "1.0.0"}
			if c.workspace {
				aDependencies["local:packages/ws"] = ""
			}
			eDependencies := map[string]string{}
			if c.peers {
				bDependencies["e"] = "1.0.0"
				eDependencies["d"] = "1.0.0"
			}
			for _, q := range []struct {
				packageName, version string
				want                 map[string]string
			}{
				{"a", "1.0.0", aDependencies},
				{"b", "1.0.0", bDependencies},
				// Imported under its alias, or reached as the real package.
				{"c2", "2.0.0", map[string]string{"d": "1.0.0"}},
				{"c", "2.0.0", map[string]string{"d": "1.0.0"}},
				{"d", "2.0.0", map[string]string{"e": "1.0.0"}},
				// One node stands for every copy of d: the edge the other copy has
				// is kept, at the version the name is locked to.
				{"d", "1.0.0", map[string]string{"e": "1.0.0"}},
				{"e", "1.0.0", eDependencies},
			} {
				got := map[string]string{}
				for _, d := range transitive.Dependencies(lang.Target{Ecosystem: "npm", Package: q.packageName, Version: q.version}) {
					if d.Pinned != (d.Version != "") {
						t.Errorf("%s@%s -> %s: pinned %v with version %q", q.packageName, q.version, d.Package, d.Pinned, d.Version)
					}
					// Only the macOS binary says where it installs.
					if want := map[bool]string{true: "os=darwin"}[d.Package == "fsevents"]; d.Platform != want {
						t.Errorf("%s@%s -> %s: platform %q, want %q", q.packageName, q.version, d.Package, d.Platform, want)
					}
					if d.Local != "" {
						got["local:"+d.Local] = ""
						continue
					}
					got[d.Package] = d.Version
				}
				if !reflect.DeepEqual(got, q.want) {
					t.Errorf("%s@%s depends on %v, want %v", q.packageName, q.version, got, q.want)
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
		name, versionRange string
		want               []string
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
		if got := yarnCandidates(c.name, c.versionRange); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %s: got %q, want %q", c.name, c.versionRange, got, c.want)
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
		if name, versionRange := yarnDependency(line); name != want[0] || versionRange != want[1] {
			t.Errorf("%s: got %q %q, want %q", line, name, versionRange, want)
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
	tree := newTree()
	tree.addYarnTree([]byte(lock))
	for key, want := range map[string]map[string]string{
		"a@1.0.0":  {"d": "2.0.0"},
		"c@2.0.0":  {"d": "1.0.0"},
		"c2@2.0.0": {"d": "1.0.0"},
	} {
		if got := tree.exact[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s needs %v, want %v", key, got, want)
		}
	}
	if !tree.dependencies["c"]["d"] || !tree.dependencies["c2"]["d"] || tree.dependencies["a"]["local"] {
		t.Errorf("edges %v", tree.dependencies)
	}
	if !tree.local["a"]["local"] {
		t.Errorf("a's local dependency is not recorded: %v", tree.local)
	}
}

// TestPlatformConditions writes a manifest's os, cpu and libc lists as Yarn
// Berry's conditions, and reads Berry's own from a yarn.lock entry: the
// platform binaries esbuild lists as optional dependencies.
//
// Verifies: REQ-JS-018
func TestPlatformConditions(t *testing.T) {
	for _, c := range []struct {
		operatingSystems, processors, cLibraries []string
		want                                     string
	}{
		{nil, nil, nil, ""},
		{[]string{"linux"}, []string{"x64"}, nil, "os=linux & cpu=x64"},
		{[]string{"darwin", "linux"}, nil, []string{"glibc"}, "(os=darwin | os=linux) & libc=glibc"},
		// npm's negation, and a doubled one, as Berry writes them.
		{[]string{"!win32"}, []string{"!!arm64"}, nil, "!os=win32 & cpu=arm64"},
		{[]string{"", "!"}, nil, nil, ""},
	} {
		if got := platformCondition(c.operatingSystems, c.processors, c.cLibraries); got != c.want {
			t.Errorf("%q %q %q: got %q, want %q", c.operatingSystems, c.processors, c.cLibraries, got, c.want)
		}
	}
	lock := `__metadata:
  version: 8

"esbuild@npm:^0.21.5":
  version: 0.21.5
  resolution: "esbuild@npm:0.21.5"
  dependencies:
    "@esbuild/linux-x64": "npm:0.21.5"
    "@esbuild/win32-x64": "npm:0.21.5"
  dependenciesMeta:
    "@esbuild/linux-x64":
      optional: true
    "@esbuild/win32-x64":
      optional: true
  languageName: node
  linkType: hard

"@esbuild/linux-x64@npm:0.21.5":
  version: 0.21.5
  resolution: "@esbuild/linux-x64@npm:0.21.5"
  conditions: os=linux & cpu=x64
  languageName: node
  linkType: hard

"@esbuild/win32-x64@npm:0.21.5":
  version: 0.21.5
  resolution: "@esbuild/win32-x64@npm:0.21.5"
  conditions: os=win32 & cpu=x64
  languageName: node
  linkType: hard
`
	tree := newTree()
	tree.addYarnTree([]byte(lock))
	want := map[string]string{"@esbuild/linux-x64": "os=linux & cpu=x64", "@esbuild/win32-x64": "os=win32 & cpu=x64"}
	if !reflect.DeepEqual(tree.platform, want) {
		t.Errorf("platforms %v, want %v", tree.platform, want)
	}
	// Both stay edges: which one an install gets is the platform's business.
	if got := tree.exact["esbuild@0.21.5"]; len(got) != 2 {
		t.Errorf("esbuild needs %v, want both binaries", got)
	}
}

// TestPnpmPackageManagerDocument reads the project's lock from a pnpm-lock.yaml
// that recent pnpm opens with a document locking the package manager
// itself: that one's packages are not the project's. pnpm writes a single libc
// without a list; a platform field of no known shape costs nothing else.
//
// Verifies: REQ-JS-009, REQ-JS-018
func TestPnpmPackageManagerDocument(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"package.json": `{"dependencies": {"a": "^1.0.0"}}`,
		"index.js":     `import 'a';`,
		"pnpm-lock.yaml": `---
lockfileVersion: '9.0'

importers:

  .:
    configDependencies: {}
    packageManagerDependencies:
      pnpm:
        specifier: 12.6.0
        version: 12.6.0

packages:

  '@pnpm/exe.linux-x64@12.6.0':
    resolution: {integrity: sha512-x}
    cpu: [x64]
    os: [linux]

  pnpm@12.6.0:
    resolution: {integrity: sha512-p}

snapshots:

  pnpm@12.6.0:
    optionalDependencies:
      '@pnpm/exe.linux-x64': 12.6.0

---
lockfileVersion: '9.0'

importers:

  .:
    dependencies:
      a:
        specifier: ^1.0.0
        version: 1.0.0

packages:

  a@1.0.0:
    resolution: {integrity: sha512-a}

  a-linux-x64@1.0.0:
    resolution: {integrity: sha512-l}
    cpu: [x64]
    os: [linux]
    libc: glibc

  a-other@1.0.0:
    resolution: {integrity: sha512-o}
    os: {not: a list}

snapshots:

  a@1.0.0:
    optionalDependencies:
      a-linux-x64: 1.0.0

  a-linux-x64@1.0.0:
    optional: true
`,
	})
	langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, root)["index.js"], map[string]lang.Target{
		"a": {Ecosystem: "npm", Package: "a", Version: "1.0.0", Requested: "^1.0.0", Pinned: true},
	})
	r, err := (Plugin{}).Resolver(root, langtest.Files(t, root))
	if err != nil {
		t.Fatal(err)
	}
	tree := r.(*resolver).tree
	if _, ok := tree.locked["pnpm"]; ok {
		t.Errorf("the package manager's own lock was read: %v", tree.locked)
	}
	got := r.(lang.Transitive).Dependencies(lang.Target{Ecosystem: "npm", Package: "a", Version: "1.0.0"})
	want := []lang.Target{{Ecosystem: "npm", Package: "a-linux-x64", Version: "1.0.0", Pinned: true, Platform: "os=linux & cpu=x64 & libc=glibc"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a depends on %+v, want %+v", got, want)
	}
}
