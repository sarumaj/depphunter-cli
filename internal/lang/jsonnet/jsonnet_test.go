package jsonnet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a jsonnet-bundler project: jsonnetfile.json declares
// ksonnet-util (a branch, locked, installed in vendor/ with its legacy
// link vendor/ksonnet-util), k8s-libsonnet (named k8s, locked, not
// installed), node-mixin (a tag, not locked), a private repository pinned
// by commit, a branch nobody locked and a local directory; the lock adds
// doc-util, installed under vendor/ only by its full path. lib/ is on the
// library path, third_party/jpath through JSONNET_PATH. mixin/ is a nested
// project without a lock of its own.
func TestMain(m *testing.M) {
	os.Setenv("JSONNET_PATH", "third_party/jpath")
	os.Exit(m.Run())
}

var (
	ksonnet = lang.Target{Ecosystem: ecosystemJB, Package: "github.com/grafana/jsonnet-libs/ksonnet-util",
		Version: "10b0fbc6f6bffbf774067de9c07982bf8454a211", Requested: "master", Pinned: true}
	k8s = lang.Target{Ecosystem: ecosystemJB, Package: "github.com/jsonnet-libs/k8s-libsonnet/1.29",
		Version: "f8efa81cf15257bd151b97e31599e20b2ba5311b", Requested: "main", Pinned: true}
	docUtil = lang.Target{Ecosystem: ecosystemJB, Package: "github.com/jsonnet-libs/docsonnet/doc-util",
		Version: "6ac6c69685b8c29c54515448eaca583da2d88150", Pinned: true}
	nodeMixin = lang.Target{Ecosystem: ecosystemJB, Package: "github.com/prometheus/node_exporter/docs/node-mixin", Version: "v1.8.0"}
	acme      = lang.Target{Ecosystem: ecosystemJB, Package: "git.acme.internal/ops/libs",
		Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "git@git.acme.internal:ops/libs.git"}
	branchy = lang.Target{Ecosystem: ecosystemJB, Package: "github.com/acme/branchy", Version: "main", Floating: true}
)

// Verifies: REQ-JSONNET-002, REQ-JSONNET-004, REQ-JSONNET-006, REQ-JSONNET-010
func TestImports(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["environments/prod/main.jsonnet"], map[string]lang.Target{
		"ksonnet-util/kausal.libsonnet":                               ksonnet, // legacy link in vendor/
		"github.com/grafana/jsonnet-libs/ksonnet-util/util.libsonnet": ksonnet, // full path in vendor/
		"github.com/jsonnet-libs/k8s-libsonnet/1.29/main.libsonnet":   k8s,     // not installed: the manifest
		"k8s/main.libsonnet": k8s, // the name field is the legacy name
		"github.com/jsonnet-libs/docsonnet/doc-util/main.libsonnet": docUtil, // installed, only locked
		"node-mixin/mixin.libsonnet":                                nodeMixin,
		"utils.libsonnet":                                           {Local: "lib/utils.libsonnet"},
		"./params.libsonnet":                                        {Local: "environments/prod/params.libsonnet"},
		"shared/lib.libsonnet":                                      {Local: "libs/shared/lib.libsonnet"}, // a local source
		"libs/secrets.libsonnet":                                    acme,
		"branchy/b.libsonnet":                                       branchy,
		"github.com/unknown/thing/x.libsonnet":                      {Ecosystem: ecosystemJB, Package: "github.com/unknown/thing", Unresolved: true},
		"mystery/x.libsonnet":                                       {Ecosystem: ecosystemJB, Package: "mystery", Unresolved: true},
		"missing.libsonnet":                                         {},
		"./gone.libsonnet":                                          {},
		"extra.libsonnet":                                           {Local: "third_party/jpath/extra.libsonnet"},
		"importstr files/config.yaml":                               {Local: "environments/prod/files/config.yaml"},
		"importbin files/logo.png":                                  {Local: "environments/prod/files/logo.png"},
	})
	// With the legacy link installed, mixin/ reaches the root project's
	// vendor/; where the checkout has no symlinks (git on Windows writes the
	// link as a plain file) nothing is installed for it, and its own
	// manifest answers.
	viaVendor := ksonnet
	if fileInfo, err := os.Lstat("testdata/repo/vendor/ksonnet-util"); err != nil || fileInfo.Mode()&os.ModeSymlink == 0 {
		viaVendor.Requested = "v1.0"
	}
	langtest.CheckImports(t, results["mixin/mixin.libsonnet"], map[string]lang.Target{
		"ksonnet-util/kausal.libsonnet": viaVendor,
	})
	for p := range results {
		if strings.HasPrefix(p, "vendor/") {
			t.Errorf("%s: what jb installed was analyzed", p)
		}
	}
}

// Verifies: REQ-JSONNET-005, REQ-JSONNET-006
func TestManifests(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["jsonnetfile.json"], map[string]lang.Target{
		"github.com/grafana/jsonnet-libs/ksonnet-util":        ksonnet,
		"github.com/jsonnet-libs/k8s-libsonnet/1.29":          k8s,
		"github.com/prometheus/node_exporter/docs/node-mixin": nodeMixin,
		"git.acme.internal/ops/libs":                          acme,
		"github.com/acme/branchy":                             branchy,
		"libs/shared":                                         {Local: "libs/shared"},
	})
	langtest.CheckImports(t, results["jsonnetfile.lock.json"], map[string]lang.Target{
		"github.com/grafana/jsonnet-libs/ksonnet-util": ksonnet,
		"github.com/jsonnet-libs/k8s-libsonnet/1.29":   k8s,
		"github.com/jsonnet-libs/docsonnet/doc-util":   docUtil,
		"git.acme.internal/ops/libs":                   acme,
		"libs/shared":                                  {Local: "libs/shared"},
	})
	// A nested project without a lock is installed by the one above it.
	nested := ksonnet
	nested.Requested = "v1.0"
	langtest.CheckImports(t, results["mixin/jsonnetfile.json"], map[string]lang.Target{
		"github.com/grafana/jsonnet-libs/ksonnet-util": nested,
	})
	dependencies, legacy := readJsonnetfile([]byte(`{"dependencies": [{"source": {"git": {"remote": "https://github.com/a/b"}}}], "legacyImports": false}`))
	if len(dependencies) != 1 || legacy || dependencies[0].legacy() != "b" || dependencies[0].packageName() != "github.com/a/b" {
		t.Errorf("readJsonnetfile: %+v %v", dependencies, legacy)
	}
}

// Verifies: REQ-JSONNET-006
func TestPinRule(t *testing.T) {
	r := &resolver{}
	p := &project{directory: "."}
	for v, want := range map[string]lang.Target{
		"0123456789abcdef0123456789abcdef01234567": {Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true},
		"v1.2.3":       {Version: "v1.2.3"},
		"2.0":          {Version: "2.0"},
		"v0.1.0-rc.1":  {Version: "v0.1.0-rc.1"},
		"main":         {Version: "main", Floating: true},
		"release-0.14": {Version: "release-0.14", Floating: true},
		"":             {Floating: true},
	} {
		d := &dependency{remote: "https://github.com/a/b", version: v}
		p.dependencies = []*dependency{d}
		want.Ecosystem, want.Package = ecosystemJB, "github.com/a/b"
		if got := r.target(p, d); got != want {
			t.Errorf("%q: got %+v, want %+v", v, got, want)
		}
	}
}

// Verifies: REQ-JSONNET-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, results["environments/prod/main.jsonnet"], map[string]string{
		"kausal": "var", "util": "var", "k": "var", "k8s": "var", "doc": "var", "node": "var",
		"utils": "var", "params": "var", "shared": "var", "secrets": "var", "branchy": "var",
		"unknown": "var", "mystery": "var", "missing": "var", "gone": "var", "extra": "var",
		"fake": "var", "verbatim": "var", "text": "var", "labels": "func", "mk": "func",
		"config": "field", "logo": "field", "deployment": "field", "quoted-name": "field",
		"hidden": "field", "plus": "field", "inner": "field", "fn": "func", "asFunction": "func",
		"again": "field", "extended": "field",
	})
	langtest.CheckSymbols(t, results["environments/prod/app.libsonnet"], map[string]string{"service": "field"})
	langtest.CheckSymbols(t, results["jsonnetfile.json"], map[string]string{})
}

// Verifies: REQ-JSONNET-002, REQ-JSONNET-009
func TestLiteralsHideCode(t *testing.T) {
	source := []byte(`// import "a.libsonnet"
# import "b.libsonnet"
/* import "c.libsonnet"
   import "d.libsonnet" */
local s = "import 'e.libsonnet' \" import 'f.libsonnet'";
local v = @'import "g.libsonnet" '' import "h.libsonnet"';
local t = |||-
  import "i.libsonnet"
    import "j.libsonnet"
|||;
local real = import "real.libsonnet";
local str = importstr @'dir\file.txt';
{ x: import "inside.libsonnet" }
`)
	extraction := extractSource(source)
	var got []string
	for _, rawImport := range extraction.Imports {
		got = append(got, rawImport.Spec)
	}
	want := []string{"real.libsonnet", `importstr dir\file.txt`, "inside.libsonnet"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
	if extraction.Imports[0].Line != 11 {
		t.Errorf("line: got %d, want 11", extraction.Imports[0].Line)
	}
}

// Verifies: REQ-JSONNET-001
func TestClaims(t *testing.T) {
	root := t.TempDir()
	for p, c := range map[string]string{
		"app/jsonnetfile.json":                       "{}",
		"app/vendor/github.com/a/b/x.libsonnet":      "{}",
		"other/vendor/y.libsonnet":                   "{}",
		"app/main.jsonnet":                           "{}",
		"app/jsonnetfile.lock.json":                  "{}",
		"app/package.json":                           "{}",
		"app/README.md":                              "x",
		"app/vendor/github.com/a/b/jsonnetfile.json": "{}",
	} {
		absolute := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for p, want := range map[string]bool{
		"app/jsonnetfile.json":                       true,
		"app/jsonnetfile.lock.json":                  true,
		"app/main.jsonnet":                           true,
		"other/vendor/y.libsonnet":                   true, // no jsonnetfile.json beside this vendor/
		"app/vendor/github.com/a/b/x.libsonnet":      false,
		"app/vendor/github.com/a/b/jsonnetfile.json": false,
		"app/package.json":                           false,
		"app/README.md":                              false,
	} {
		f := &scan.File{Path: p, AbsolutePath: filepath.Join(root, filepath.FromSlash(p))}
		if got := (Plugin{}).Claims(f); got != want {
			t.Errorf("%s: got %v, want %v", p, got, want)
		}
	}
	if c := (Plugin{}).Class(&scan.File{Path: "a/jsonnetfile.lock.json"}); c != classLock {
		t.Errorf("class: %q", c)
	}
}

// Verifies: REQ-JSONNET-007
func TestDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"), func(string) string { return "" })
	got := r.Dependencies(ksonnet)
	locked := docUtil // pinned by the installing project's lock, asked for by ksonnet-util
	locked.Requested = "master"
	want := []lang.Target{
		locked,
		{Ecosystem: ecosystemJB, Package: "github.com/jsonnet-libs/xtd", Version: "master", Floating: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if !r.Installed(ksonnet) || r.Installed(k8s) || r.Dependencies(k8s) != nil {
		t.Error("only what vendor/ holds is installed")
	}
}

// Verifies: REQ-JSONNET-008
func TestIslands(t *testing.T) {
	ecosystems := (Plugin{}).Ecosystems()
	if len(ecosystems) != 1 || ecosystems[0].ID != ecosystemJB || ecosystems[0].Std {
		t.Errorf("got %+v", ecosystems)
	}
}

// Every prefix of every fixture file and pathological inputs go through the
// lexer and the readers without panicking, quickly.
//
// Verifies: REQ-JSONNET-009
func TestTruncated(t *testing.T) {
	var sources [][]byte
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if b, err := os.ReadFile(p); err == nil {
				sources = append(sources, b)
			}
		}
		return nil
	})
	for _, source := range sources {
		for i := 0; i <= len(source); i++ {
			extractSource(source[:i])
			readJsonnetfile(source[:i])
		}
	}
	for _, unit := range []string{"{", "[", "(", "}", ")", "local a = ", "local f(", "{ a: ", "|||\n  x\n", "'", "\"", "@'", "/*", "import ", "{ [x]: ", "a+: ", "\\"} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)))
		start := time.Now()
		extractSource(source)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%q x %d: %v", unit, 200_000/len(unit), d)
		}
	}
}
