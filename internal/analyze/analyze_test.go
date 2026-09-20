package analyze

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
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
		"a.fake": "pinned\nfloating\nboth\nunknown\nmoving\n",
	})
	p := fakePlugin{targets: map[string]lang.Target{
		"pinned":   {Ecosystem: "fake-eco", Package: "pinned", Version: "1.2.3", Requested: "^1.2.0", Pinned: true},
		"floating": {Ecosystem: "fake-eco", Package: "floating", Version: "^2.0.0"},
		"both":     {Ecosystem: "fake-eco", Package: "both", Version: "3.0.0", Pinned: true},
		"unknown":  {Ecosystem: "fake-eco", Package: "unknown"},
		// A reference with no version that still moves: a served template, a URL.
		"moving": {Ecosystem: "fake-eco", Package: "moving", Floating: true},
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
		{"moving", "", "", true},
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

// fakeResolver answers for transitive dependencies too, so the walk can be tested
// without a lock file: direct -> middle -> deep, and deep needs nothing.
func (r fakeResolver) Dependencies(t lang.Target) []lang.Target {
	next := map[string]string{"direct": "middle", "middle": "deep"}
	if dep, ok := next[t.Package]; ok {
		return []lang.Target{{Ecosystem: "fake-eco", Package: dep, Version: "1.0.0", Pinned: true}}
	}
	return nil
}

func TestResolveDepth(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "direct\n"})
	p := fakePlugin{targets: map[string]lang.Target{
		"direct": {Ecosystem: "fake-eco", Package: "direct", Version: "2.0.0", Pinned: true},
	}}

	for _, c := range []struct {
		depth int
		want  []string // package nodes, in the order the walk reaches them
	}{
		{0, []string{"direct"}},
		{1, []string{"direct", "middle"}},
		{2, []string{"direct", "middle", "deep"}},
		{-1, []string{"direct", "middle", "deep"}}, // as far as the lock files reach
	} {
		g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{p}, ResolveDepth: c.depth})
		if err != nil {
			t.Fatal(err)
		}
		var packages []string
		transitive := map[string]bool{}
		for _, n := range g.Nodes {
			if n.Kind == graph.KindPackage {
				packages = append(packages, n.Name)
				transitive[n.Name] = n.Transitive
			}
		}
		sort.Strings(packages)
		want := append([]string(nil), c.want...)
		sort.Strings(want)
		if strings.Join(packages, ",") != strings.Join(want, ",") {
			t.Errorf("depth %d: packages %v, want %v", c.depth, packages, want)
		}
		// What the project imports itself is not transitive, whatever the depth.
		if transitive["direct"] {
			t.Errorf("depth %d: the imported package is marked transitive", c.depth)
		}
		if len(c.want) > 1 && !transitive["middle"] {
			t.Errorf("depth %d: a package only a dependency needs is not marked transitive", c.depth)
		}
	}
}

func TestResolveDepthEdges(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "direct\n"})
	p := fakePlugin{targets: map[string]lang.Target{
		"direct": {Ecosystem: "fake-eco", Package: "direct", Version: "2.0.0", Pinned: true},
	}}
	g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{p}, ResolveDepth: -1})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]graph.EdgeKind{}
	for _, e := range g.Edges {
		kinds[e.From+" -> "+e.To] = e.Kind
	}
	// A file imports a package; a package depends on a package.
	if got := kinds[graph.FileID("a.fake")+" -> "+graph.PackageID("fake-eco", "direct")]; got != graph.EdgeImport {
		t.Errorf("file edge is %q, want import", got)
	}
	for _, pair := range [][2]string{{"direct", "middle"}, {"middle", "deep"}} {
		key := graph.PackageID("fake-eco", pair[0]) + " -> " + graph.PackageID("fake-eco", pair[1])
		if got := kinds[key]; got != graph.EdgeDepends {
			t.Errorf("%s edge is %q, want depends", key, got)
		}
	}
}

// fakeIndexes answers for one ecosystem, so the builder's attribution can be checked
// without a real configuration on disk.
type fakeIndexes struct{ index string }

func (f fakeIndexes) For(eco, pkg string) (string, bool) {
	if eco != "fake-eco" {
		return "", false
	}
	if pkg == "public" {
		return "https://public.example", true
	}
	return f.index, false
}

func TestPackagesCarryTheirIndex(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "public\nprivate\n"})
	p := fakePlugin{targets: map[string]lang.Target{
		"public":  {Ecosystem: "fake-eco", Package: "public"},
		"private": {Ecosystem: "fake-eco", Package: "private"},
	}}
	g, _, err := Run(context.Background(), root, Options{
		Plugins: []lang.Plugin{p},
		Indexes: func([]*scan.File) Indexes { return fakeIndexes{index: "https://internal.example"} },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind != graph.KindPackage {
			continue
		}
		switch n.Name {
		case "public":
			if n.Index != "https://public.example" || n.IndexUnknown {
				t.Errorf("public: index %q unknown %v", n.Index, n.IndexUnknown)
			}
		case "private":
			// Nothing on this machine vouches for it, which is what the map shows.
			if n.Index != "https://internal.example" || !n.IndexUnknown {
				t.Errorf("private: index %q unknown %v", n.Index, n.IndexUnknown)
			}
		}
	}
}
