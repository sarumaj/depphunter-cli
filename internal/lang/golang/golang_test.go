package golang

import (
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

func TestCgoPseudoPackageIgnored(t *testing.T) {
	if n := len(analyze(t)["lib/sub/sub.go"].Imports); n != 0 {
		t.Errorf(`import "C" should be skipped, got %d imports`, n)
	}
}

func TestBrokenFileStillResolves(t *testing.T) {
	r := analyze(t)["tools/broken.go"]
	if r == nil || len(r.Imports) != 1 || r.Imports[0].Target.Local != "app/internal/util" {
		t.Errorf("partial parse should keep imports, got %+v", r)
	}
}

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
func TestAVanishedGoModIsSkipped(t *testing.T) {
	gone := &scan.File{Path: "sub/go.mod", Abs: filepath.Join(t.TempDir(), "go.mod")}
	if _, err := (Plugin{}).Resolver(t.TempDir(), []*scan.File{gone}); err != nil {
		t.Errorf("a go.mod that could not be read failed the analysis: %v", err)
	}
}
