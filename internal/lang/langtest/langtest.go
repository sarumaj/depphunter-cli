// Package langtest holds the helpers shared by the language plugin tests: run a plugin
// over a fixture project and compare its resolved imports and symbols.
package langtest

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	results, err := lang.Analyze(context.Background(), p, root, files)
	if err != nil {
		t.Fatal(err)
	}
	return results
}

// Files scans a fixture project, for tests that build a resolver directly.
func Files(t *testing.T, root string) []*scan.File {
	t.Helper()
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// Imports maps each import's spec to its target.
func Imports(t *testing.T, result *lang.FileResult) map[string]lang.Target {
	t.Helper()
	if result == nil {
		t.Fatal("file not analyzed")
	}
	out := map[string]lang.Target{}
	for _, imported := range result.Imports {
		out[imported.Spec] = imported.Target
	}
	return out
}

// ImportLines maps the module of each import an extraction found to its line.
func ImportLines(extraction *lang.Extraction) map[string]int {
	out := map[string]int{}
	for _, rawImport := range extraction.Imports {
		out[rawImport.Module] = rawImport.Line
	}
	return out
}

// CheckImports asserts that res imports exactly want (spec -> target).
func CheckImports(t *testing.T, result *lang.FileResult, want map[string]lang.Target) {
	t.Helper()
	got := Imports(t, result)
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
func Symbols(t *testing.T, result *lang.FileResult) map[string]string {
	t.Helper()
	if result == nil {
		t.Fatal("file not analyzed")
	}
	out := map[string]string{}
	for _, s := range result.Symbols {
		out[s.Name] = s.Kind
	}
	return out
}

// CheckSymbols asserts that res defines exactly want (name -> kind).
func CheckSymbols(t *testing.T, result *lang.FileResult, want map[string]string) {
	t.Helper()
	if got := Symbols(t, result); !reflect.DeepEqual(got, want) {
		t.Errorf("symbols: got %v, want %v", got, want)
	}
}

// Write puts files (slash-separated path -> content) under a new temporary root.
func Write(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, content := range files {
		absolute := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// Without is files less the ones at paths: what a scan lists when git ignores them.
func Without(files []*scan.File, paths ...string) []*scan.File {
	return slices.DeleteFunc(slices.Clone(files), func(f *scan.File) bool { return slices.Contains(paths, f.Path) })
}

// Notes lists what a resolver noted for --explain as "file code" lines, sorted;
// nil when it implements no lang.Noter.
func Notes(r lang.Resolver) []string {
	n, ok := r.(lang.Noter)
	if !ok {
		return nil
	}
	var out []string
	for _, note := range n.Notes() {
		out = append(out, note.File+" "+note.Code)
	}
	slices.Sort(out)
	return out
}
