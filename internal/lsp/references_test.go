package lsp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/analyze"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
)

// Verifies: REQ-LSP-002, REQ-LSP-003, REQ-LSP-004
func TestGoplsReferences(t *testing.T) {
	gopls := Servers[0].command(exec.LookPath)
	if gopls == nil {
		t.Skip("gopls not installed")
	}
	root := t.TempDir()
	for p, c := range map[string]string{
		"go.mod": "module example.com/m\n\ngo 1.22\n",
		"a.go": "package m\n\ntype Store struct{}\n\nfunc (s *Store) Get() int { return helper() }\n\n" +
			"func helper() int { return 1 }\n\nfunc Use() int {\n\ts := &Store{}\n\treturn s.Get()\n}\n\n" +
			"var registry = map[string]func() int{\"use\": Use}\n",
		"b/b.go": "package b\n\nimport \"example.com/m\"\n\n// B uses m.\nfunc B() int { return m.Use() }\n",
	} {
		os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte(c), 0o644)
	}
	g, _, err := analyze.Run(context.Background(), root, analyze.Options{Plugins: []lang.Plugin{golang.Plugin{}}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := References(context.Background(), g, Options{
		Root: root, Timeout: 2 * time.Minute,
		LookPath: func(string) (string, error) { return gopls[0], nil },
		Logf:     t.Logf,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[[2]string]bool{}
	for _, e := range res.Edges {
		if e.Kind != graph.EdgeReference {
			t.Errorf("edge kind %q", e.Kind)
		}
		got[[2]string{e.From, e.To}] = true
	}
	for _, want := range [][2]string{
		{"s:a.go#Store.Get", "s:a.go#helper"}, // call inside a method
		{"s:a.go#Use", "s:a.go#Store"},        // composite literal
		{"s:a.go#Use", "s:a.go#Store.Get"},    // method call
		{"s:b/b.go#B", "s:a.go#Use"},          // across packages
		{"s:a.go#registry", "s:a.go#Use"},     // inside a var declaration
	} {
		if !got[want] {
			t.Errorf("missing reference %s -> %s", want[0], want[1])
		}
	}
	// Use's body ends before registry: the extent, not the nearest preceding
	// definition, decides where a reference sits.
	if got[[2]string{"s:a.go#Use", "s:a.go#Use"}] {
		t.Error("self reference")
	}
	if len(res.Servers) != 1 || res.Servers[0] != "gopls" || res.Partial {
		t.Errorf("servers %v, partial %v", res.Servers, res.Partial)
	}
	t.Logf("%d references: %v", len(res.Edges), got)
}

// Verifies: REQ-LSP-008
func TestNameColumn(t *testing.T) {
	for _, c := range []struct {
		line, name string
		col        int
		ok         bool
	}{
		{"func (s *Store) Get() int {", "Get", 16, true},
		{"func GetAll() {}", "Get", 0, false},             // whole words only
		{"const π = 3; func Area() {}", "Area", 18, true}, // π is one UTF-16 unit
		{"let 😀 = 1; function f() {}", "f", 21, true},     // 😀 is two UTF-16 units
	} {
		col, ok := nameColumn(c.line, c.name)
		if col != c.col || ok != c.ok {
			t.Errorf("%q in %q: got %d %v, want %d %v", c.name, c.line, col, ok, c.col, c.ok)
		}
	}
}

func TestURIs(t *testing.T) {
	root := filepath.FromSlash("/work/repo")
	if rel, ok := relPath(root, fileURI(filepath.Join(root, "a b", "c.go"))); !ok || rel != "a b/c.go" {
		t.Errorf("round trip: %q %v", rel, ok)
	}
	if _, ok := relPath(root, "file:///usr/lib/go/src/fmt/print.go"); ok {
		t.Error("paths outside the root must be rejected")
	}
}
