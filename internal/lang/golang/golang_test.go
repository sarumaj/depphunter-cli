package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo holds two modules: app (requires cobra, replaces lib with ../lib) and
// lib, plus a file outside any module with a syntax error.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-LANG-002, REQ-LANG-004, REQ-GO-003, REQ-GO-004, REQ-GO-005, REQ-GO-006
func TestImportResolution(t *testing.T) {
	res := analyze(t)
	main := res["app/main.go"]
	if main == nil {
		t.Fatal("app/main.go not analyzed")
	}
	langtest.CheckImports(t, main, map[string]lang.Target{
		"fmt":                           {Ecosystem: "go-std", Package: "fmt"},
		"net/http":                      {Ecosystem: "go-std", Package: "net/http"},
		"example.com/app/internal/util": {Local: "app/internal/util"},
		"example.com/lib/sub":           {Local: "lib/sub"},
		// A require line is the version the build selects, so Go modules are pinned.
		"github.com/spf13/cobra/doc":  {Ecosystem: "go", Package: "github.com/spf13/cobra", Version: "v1.8.0", Pinned: true},
		"github.com/undeclared/thing": {Ecosystem: "go", Package: "github.com/undeclared/thing", Unresolved: true},
	})
}

// Verifies: REQ-GO-001
func TestCgoPseudoPackageIgnored(t *testing.T) {
	if n := len(analyze(t)["lib/sub/sub.go"].Imports); n != 0 {
		t.Errorf(`import "C" should be skipped, got %d imports`, n)
	}
}

// Verifies: REQ-GO-001
func TestBrokenFileStillResolves(t *testing.T) {
	r := analyze(t)["tools/broken.go"]
	if r == nil || len(r.Imports) != 1 || r.Imports[0].Target.Local != "app/internal/util" {
		t.Errorf("partial parse should keep imports, got %+v", r)
	}
}

// Verifies: REQ-LANG-003, REQ-LANG-024, REQ-GO-002
func TestSymbols(t *testing.T) {
	got := map[string]string{}
	for _, s := range analyze(t)["app/main.go"].Symbols {
		got[s.Name] = s.Kind
	}
	for name, kind := range map[string]string{
		"Server": "type", "Server.Start": "method", "init": "func", "init@18": "func",
		"Version": "const", "main": "func",
	} {
		if got[name] != kind {
			t.Errorf("symbol %s: got kind %q, want %q (all: %v)", name, got[name], kind, got)
		}
	}
}

// A go.mod the scan listed can be gone by the time it is read - a branch switch, an
// editor saving by rename - and that is no reason to fail the whole analysis.
//
// Verifies: REQ-GO-003
func TestAVanishedGoModIsSkipped(t *testing.T) {
	gone := &scan.File{Path: "sub/go.mod", Abs: filepath.Join(t.TempDir(), "go.mod")}
	if _, err := (Plugin{}).Resolver(t.TempDir(), []*scan.File{gone}); err != nil {
		t.Errorf("a go.mod that could not be read failed the analysis: %v", err)
	}
}

func TestReplaceDirectives(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod": `module example.com/app

go 1.22

require (
	example.com/forked v1.0.0
	example.com/sibling v0.0.0-00010101000000-000000000000
	example.com/pinned v1.2.0
	example.com/other v1.5.0
	example.com/nested v1.0.0
	example.com/nested/deeper v1.0.0
)

replace (
	example.com/forked => github.com/me/forked v1.0.1-fix
	example.com/sibling => ../sibling
	example.com/pinned v1.1.0 => github.com/me/pinned v1.1.1
	example.com/nested => github.com/me/nested v1.0.1
	example.com/nested/deeper => github.com/me/deeper v2.0.0
)
`,
		"main.go": `package main

import (
	_ "example.com/forked/pkg"
	_ "example.com/sibling"
	_ "example.com/pinned"
	_ "example.com/nested/deeper/x"
)
`,
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.WriteFile(abs, []byte(c), 0o644)
	}
	res := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, res["main.go"], map[string]lang.Target{
		// A module replaced by another module: the replacement is what is built.
		"example.com/forked/pkg": {Ecosystem: "go", Package: "github.com/me/forked", Version: "v1.0.1-fix", Requested: "v1.0.0", Pinned: true},
		// A directory outside the project has no version worth looking up.
		"example.com/sibling": {Ecosystem: "go", Package: "example.com/sibling"},
		// A replacement of another version than the one required does not apply.
		"example.com/pinned": {Ecosystem: "go", Package: "example.com/pinned", Version: "v1.2.0", Pinned: true},
		// The longest replaced path wins.
		"example.com/nested/deeper/x": {Ecosystem: "go", Package: "github.com/me/deeper", Version: "v2.0.0", Requested: "v1.0.0", Pinned: true},
	})
}
