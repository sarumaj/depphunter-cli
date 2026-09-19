// Package langtest holds the helpers shared by the language plugin tests: run a plugin
// over a fixture project and compare its resolved imports and symbols.
package langtest

import (
	"context"
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Analyze runs p over the project at root (usually testdata/repo).
func Analyze(t *testing.T, p lang.Plugin, root string) map[string]*lang.FileResult {
	t.Helper()
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := lang.Analyze(context.Background(), p, root, files)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// Imports maps each import's spec to its target.
func Imports(t *testing.T, res *lang.FileResult) map[string]lang.Target {
	t.Helper()
	if res == nil {
		t.Fatal("file not analyzed")
	}
	out := map[string]lang.Target{}
	for _, im := range res.Imports {
		out[im.Spec] = im.Target
	}
	return out
}

// CheckImports asserts that res imports exactly want (spec -> target).
func CheckImports(t *testing.T, res *lang.FileResult, want map[string]lang.Target) {
	t.Helper()
	got := Imports(t, res)
	for spec, w := range want {
		if g, ok := got[spec]; !ok {
			t.Errorf("%s: not captured", spec)
		} else if g != w {
			t.Errorf("%s: got %+v, want %+v", spec, g, w)
		}
	}
	for spec, g := range got {
		if _, ok := want[spec]; !ok {
			t.Errorf("unexpected import %s -> %+v", spec, g)
		}
	}
}

// Symbols maps each symbol's name to its kind.
func Symbols(t *testing.T, res *lang.FileResult) map[string]string {
	t.Helper()
	if res == nil {
		t.Fatal("file not analyzed")
	}
	out := map[string]string{}
	for _, s := range res.Symbols {
		out[s.Name] = s.Kind
	}
	return out
}

// CheckSymbols asserts that res defines exactly want (name -> kind).
func CheckSymbols(t *testing.T, res *lang.FileResult, want map[string]string) {
	t.Helper()
	if got := Symbols(t, res); !reflect.DeepEqual(got, want) {
		t.Errorf("symbols: got %v, want %v", got, want)
	}
}
