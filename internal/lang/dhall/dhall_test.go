package dhall

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

const hash = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// The fixture: config/app.dhall imports in every way Dhall can (the Prelude
// with a hash, a GitHub branch, the Prelude from dhall-lang's repository, an
// alternative of a local file and a hashed URL, env:, absolute and home
// paths, `as Text`, `as Location`, `using` headers); package.dhall is a
// record of imports; spago.dhall, packages.dhall and test/test.dhall are
// spago's.
//
// Verifies: REQ-DHALL-002, REQ-DHALL-004, REQ-DHALL-005, REQ-DHALL-008
func TestImports(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, results["config/app.dhall"], map[string]lang.Target{
		"https://prelude.dhall-lang.org/v23.0.0/package.dhall":                                    {Ecosystem: ecosystemDhall, Package: prelude, Version: "v23.0.0", Pinned: true},
		"https://raw.githubusercontent.com/dhall-lang/dhall-kubernetes/master/1.25/package.dhall": {Ecosystem: ecosystemDhall, Package: "github.com/dhall-lang/dhall-kubernetes", Version: "master", Floating: true},
		"https://raw.githubusercontent.com/dhall-lang/dhall-lang/v17.0.0/Prelude/Map/Type":        {Ecosystem: ecosystemDhall, Package: prelude, Version: "v17.0.0"},
		"../lib/util.dhall":                           {Local: "lib/util.dhall"},
		"https://example.com/dhall/v1.2.0/util.dhall": {Ecosystem: ecosystemDhall, Package: "example.com/dhall", Version: "v1.2.0", Pinned: true},
		"./schema.dhall":                              {Local: "config/schema.dhall"},
		"env:DHALL_LOCAL":                             {},
		"./missing.dhall":                             {},
		"/etc/dhall/x.dhall":                          {},
		"~/x.dhall":                                   {},
		"../README.md as Text":                        {Local: "README.md"},
		"./schema.dhall as Location":                  {Local: "config/schema.dhall"},
		"https://example.org/private/config.dhall":    {Ecosystem: ecosystemDhall, Package: "example.org/private", Floating: true},
		"./headers.dhall":                             {Local: "config/headers.dhall"},
	})
	langtest.CheckImports(t, results["package.dhall"], map[string]lang.Target{
		"./config/app.dhall": {Local: "config/app.dhall"},
		"./lib/util.dhall":   {Local: "lib/util.dhall"},
	})
	for _, p := range []string{"spago.dhall", "packages.dhall", "test/test.dhall", "README.md"} {
		if results[p] != nil {
			t.Errorf("%s claimed", p)
		}
	}
}

// Verifies: REQ-DHALL-003
func TestSymbols(t *testing.T) {
	results := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, results["config/app.dhall"], map[string]string{
		"Prelude": "type", "k8s": "let", "Map": "type", "Util": "type", "Schema": "type", "fromEnv": "let",
		"readme": "let", "here": "let", "private": "let", "text": "let", "quoted": "let", "mk": "function",
		"Config": "type", "app": "field", "version": "field",
	})
	langtest.CheckSymbols(t, results["package.dhall"], map[string]string{"app": "field", "util": "field"})
	langtest.CheckSymbols(t, results["lib/util.dhall"], map[string]string{"version": "let"})
}

// Verifies: REQ-DHALL-001, REQ-PURESCRIPT-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"a.dhall": true, "types/Deployment.dhall": true, "package.dhall": true,
		"spago.dhall": false, "packages.dhall": false, "test/test.dhall": false, "spago-test.dhall": false, "a.purs": false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: claimed %v, want %v", p, got, want)
		}
		if Spago(p) == want && strings.HasSuffix(p, ".dhall") {
			t.Errorf("%s: Spago %v", p, Spago(p))
		}
	}
}

// Verifies: REQ-DHALL-005
func TestRemote(t *testing.T) {
	for url, want := range map[string]lang.Target{
		"https://prelude.dhall-lang.org/package.dhall":                                                    {Ecosystem: ecosystemDhall, Package: prelude, Floating: true},
		"https://github.com/dhall-lang/dhall-lang/raw/0123456789abcdef0123456789abcdef01234567/Prelude/x": {Ecosystem: ecosystemDhall, Package: prelude, Version: "0123456789abcdef0123456789abcdef01234567"},
		"https://cdn.jsdelivr.net/gh/Acme/Dhall-Lib@v2.1.0/package.dhall":                                 {Ecosystem: ecosystemDhall, Package: "github.com/Acme/Dhall-Lib", Version: "v2.1.0"},
		"https://gitlab.com/group/sub/proj/-/raw/main/package.dhall":                                      {Ecosystem: ecosystemDhall, Package: "gitlab.com/group/sub/proj", Version: "main", Floating: true},
		"https://user@Example.COM:8443/a/b/3.1/c/d.dhall?x=1":                                             {Ecosystem: ecosystemDhall, Package: "example.com/a/b", Version: "3.1"},
		"http://example.com/x.dhall":                                                                      {Ecosystem: ecosystemDhall, Package: "example.com", Floating: true},
	} {
		if got := remote(url, ""); got != want {
			t.Errorf("%s: got %+v, want %+v", url, got, want)
		}
	}
	if got := remote("https://example.com/x.dhall", hash); got.Version != "sha256:0123456789ab" || !got.Pinned {
		t.Errorf("hashed without version: %+v", got)
	}
}

// Every prefix of the fixtures, and pathological inputs, are read in time
// without a panic.
//
// Verifies: REQ-DHALL-007
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
		}
	}
	for _, unit := range []string{"{", "[", "(", "let x = ", "let x = let y = ", "{- ", "''", "\"${", "./a ", "https://a ", "\\(x : T) -> ", "x with a = ", "< A | ", "a.", "{ a = ", "{ a.b.c = 1, "} {
		source := []byte(strings.Repeat(unit, 200_000/len(unit)))
		start := time.Now()
		extractSource(source)
		if d := time.Since(start); d > langtest.TimeLimit(2*time.Second) {
			t.Errorf("%q x %d: %v", unit, 200_000/len(unit), d)
		}
	}
}

// Verifies: REQ-DHALL-006
func TestEcosystems(t *testing.T) {
	if e := (Plugin{}).Ecosystems(); len(e) != 1 || e[0].ID != ecosystemDhall || e[0].Std {
		t.Errorf("%+v", e)
	}
}
