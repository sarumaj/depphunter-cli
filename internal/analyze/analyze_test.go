package analyze

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

func writeProject(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		abs := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// canonical renders what a graph contains, without its timestamp.
func canonical(t *testing.T, g *graph.Graph) string {
	t.Helper()
	b, err := json.Marshal([]any{g.Nodes, g.Edges})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCacheSkipsUnchangedFiles(t *testing.T) {
	root, cacheDir := t.TempDir(), t.TempDir()
	writeProject(t, root, map[string]string{
		"go.mod":       "module example.com/m\n\ngo 1.22\n",
		"main.go":      "package main\n\nimport \"example.com/m/util\"\n\nfunc main() { util.Do() }\n",
		"util/util.go": "package util\n\nfunc Do() {}\n",
	})
	run := func() (*graph.Graph, Stats) {
		t.Helper()
		c := cache.Open(cacheDir, root)
		g, st, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{golang.Plugin{}}, Cache: c})
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Save(); err != nil {
			t.Fatal(err)
		}
		return g, st
	}

	g1, st := run()
	if st.Parsed != 2 || st.Cached != 0 {
		t.Fatalf("cold run: %+v", st)
	}
	g2, st := run()
	if st.Parsed != 0 || st.Cached != 2 {
		t.Fatalf("warm run: %+v", st)
	}
	if canonical(t, g1) != canonical(t, g2) {
		t.Error("cached run produced a different graph")
	}

	writeProject(t, root, map[string]string{"util/util.go": "package util\n\nimport \"fmt\"\n\nfunc Do() { fmt.Println() }\n"})
	g3, st := run()
	if st.Parsed != 1 || st.Cached != 1 {
		t.Fatalf("after edit: %+v", st)
	}
	found := false
	for _, e := range g3.Edges {
		found = found || (e.From == "f:util/util.go" && e.To == "p:go-std:fmt")
	}
	if !found {
		t.Error("edit not reflected in the graph")
	}
}

func TestNilCache(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.go": "package a\n"})
	_, st, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{golang.Plugin{}}})
	if err != nil || st.Parsed != 1 {
		t.Fatalf("stats %+v, err %v", st, err)
	}
}

// fakePlugin resolves each import to the target its spec names, so the builder can be
// asked what it does with versions without going through a real ecosystem.
type fakePlugin struct{ targets map[string]lang.Target }

func (fakePlugin) Name() string             { return "fake" }
func (fakePlugin) Version() int             { return 1 }
func (fakePlugin) Claims(f *scan.File) bool { return filepath.Ext(f.Path) == ".fake" }
func (fakePlugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{{ID: "fake-eco", Name: "Fake"}}
}

func (fakePlugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	e := &lang.Extraction{}
	for i, line := range strings.Split(string(src), "\n") {
		if spec := strings.TrimSpace(line); spec != "" {
			e.Imports = append(e.Imports, lang.RawImport{Spec: spec, Module: spec, Line: i + 1})
		}
	}
	return e, nil
}

func (p fakePlugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return fakeResolver(p), nil
}

type fakeResolver fakePlugin

func (r fakeResolver) Resolve(file string, imp lang.RawImport) lang.Target {
	return r.targets[imp.Module]
}

// TestPackageVersions checks what the builder makes of the versions its plugins
// report: the first version wins, and a package is floating as soon as one importer
// leaves it open - a lock file elsewhere does not fix what this manifest lets drift.
func TestPackageVersions(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{
		"a.fake": "pinned\nfloating\nboth\nunknown\n",
	})
	p := fakePlugin{targets: map[string]lang.Target{
		"pinned":   {Ecosystem: "fake-eco", Package: "pinned", Version: "1.2.3", Requested: "^1.2.0", Pinned: true},
		"floating": {Ecosystem: "fake-eco", Package: "floating", Version: "^2.0.0"},
		"both":     {Ecosystem: "fake-eco", Package: "both", Version: "3.0.0", Pinned: true},
		"unknown":  {Ecosystem: "fake-eco", Package: "unknown"},
	}}
	g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{p}})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	for _, c := range []struct {
		pkg, version, requested string
		floating                bool
	}{
		{"pinned", "1.2.3", "^1.2.0", false},
		{"floating", "^2.0.0", "", true},
		{"both", "3.0.0", "", false},
		// Nothing is known about it, which is not the same as knowing it floats.
		{"unknown", "", "", false},
	} {
		n := byID[graph.PackageID("fake-eco", c.pkg)]
		if n == nil {
			t.Errorf("%s: no package node", c.pkg)
			continue
		}
		if n.Version != c.version || n.Requested != c.requested || n.Floating != c.floating {
			t.Errorf("%s: version %q requested %q floating %v, want %q %q %v",
				c.pkg, n.Version, n.Requested, n.Floating, c.version, c.requested, c.floating)
		}
	}
}
