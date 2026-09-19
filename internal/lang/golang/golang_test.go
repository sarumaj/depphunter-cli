package golang

import (
	"context"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo holds two modules: app (requires cobra, replaces lib with ../lib) and
// lib, plus a file outside any module with a syntax error.
func analyse(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	const root = "testdata/repo"
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var claimed []*scan.File
	for _, f := range files {
		if (Plugin{}).Claims(f) {
			claimed = append(claimed, f)
		}
	}
	res, err := Plugin{}.Analyze(context.Background(), root, files, claimed)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestImportResolution(t *testing.T) {
	res := analyse(t)
	main := res["app/main.go"]
	if main == nil {
		t.Fatal("app/main.go not analysed")
	}
	got := map[string]lang.Target{}
	for _, im := range main.Imports {
		got[im.Spec] = im.Target
	}
	want := map[string]lang.Target{
		"fmt":                           {Ecosystem: "go-std", Package: "fmt"},
		"net/http":                      {Ecosystem: "go-std", Package: "net/http"},
		"example.com/app/internal/util": {Local: "app/internal/util"},
		"example.com/lib/sub":           {Local: "lib/sub"},
		"github.com/spf13/cobra/doc":    {Ecosystem: "go", Package: "github.com/spf13/cobra", Version: "v1.8.0"},
		"github.com/undeclared/thing":   {Ecosystem: "go", Package: "github.com/undeclared/thing", Unresolved: true},
	}
	for spec, w := range want {
		if got[spec] != w {
			t.Errorf("%s: got %+v, want %+v", spec, got[spec], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d imports, want %d: %+v", len(got), len(want), got)
	}
}

func TestCgoPseudoPackageIgnored(t *testing.T) {
	if n := len(analyse(t)["lib/sub/sub.go"].Imports); n != 0 {
		t.Errorf(`import "C" should be skipped, got %d imports`, n)
	}
}

func TestBrokenFileStillResolves(t *testing.T) {
	r := analyse(t)["tools/broken.go"]
	if r == nil || len(r.Imports) != 1 || r.Imports[0].Target.Local != "app/internal/util" {
		t.Errorf("partial parse should keep imports, got %+v", r)
	}
}

func TestSymbols(t *testing.T) {
	got := map[string]string{}
	for _, s := range analyse(t)["app/main.go"].Symbols {
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
