package analyze

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
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

// Verifies: REQ-MOD-003, REQ-MOD-008, REQ-LANG-026, REQ-LANG-028
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
//
// Verifies: REQ-SUP-001, REQ-SUP-007, REQ-MOD-005
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

// Verifies: REQ-SUP-008, REQ-SUP-012, REQ-MOD-009
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

// Verifies: REQ-SUP-013, REQ-MOD-006, REQ-MOD-010
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

// Verifies: REQ-SUP-014, REQ-SUP-018
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

// Verifies: REQ-SUP-037, REQ-MOD-011
func TestPackagesAreMarkedAsTheOrganizationsOwn(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "corp.example/lib\nreact\n"})
	p := fakePlugin{targets: map[string]lang.Target{
		"corp.example/lib": {Ecosystem: "fake-eco", Package: "corp.example/lib", Version: "v1.0.0", Pinned: true},
		"react":            {Ecosystem: "fake-eco", Package: "react", Version: "18.3.1", Pinned: true},
	}}
	g, _, err := Run(context.Background(), root, Options{
		Plugins: []lang.Plugin{p},
		Private: func(eco, pkg string) bool { return eco == "fake-eco" && pkg == "corp.example/lib" },
	})
	if err != nil {
		t.Fatal(err)
	}
	marked := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.KindPackage {
			marked[n.Name] = n.Private
		}
	}
	if !marked["corp.example/lib"] {
		t.Error("the organization's own package was not marked")
	}
	if marked["react"] {
		t.Error("a public package was marked private")
	}
}

// Verifies: REQ-SUP-037
func TestNothingIsPrivateWithoutBeingDeclared(t *testing.T) {
	// The ordinary case: a repository of open-source dependencies, no configuration,
	// and nothing held back from the index or the vulnerability database.
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "react\n"})
	p := fakePlugin{targets: map[string]lang.Target{"react": {Ecosystem: "fake-eco", Package: "react"}}}
	g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{p}})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Private {
			t.Errorf("%s was marked private with nothing declared", n.ID)
		}
	}
}

// stdPlugin claims .std files for an ecosystem that is a standard library, which the
// walk must not ask about: nothing publishes what "fs" depends on.
type stdPlugin struct{ fakePlugin }

func (stdPlugin) Claims(f *scan.File) bool     { return strings.HasSuffix(f.Path, ".std") }
func (stdPlugin) Ecosystems() []lang.Ecosystem { return []lang.Ecosystem{{ID: "std-eco", Std: true}} }
func (p stdPlugin) Resolver(string, []*scan.File) (lang.Resolver, error) {
	return fakeResolver(p.fakePlugin), nil
}

// TestResolutionReport checks the account the walk gives of itself: who answered for
// each package, how far each level got, and - the part no other test can see - what
// was never asked at all.
//
// Verifies: REQ-TRC-001, REQ-TRC-004, REQ-TRC-005, REQ-SUP-010, REQ-SUP-030
func TestResolutionReport(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "direct\n"})
	p := fakePlugin{targets: map[string]lang.Target{
		"direct": {Ecosystem: "fake-eco", Package: "direct", Version: "2.0.0", Pinned: true},
	}}
	rep := trace.New(-1, false, []string{"corp.example/*"}, nil)
	if _, st, err := Run(context.Background(), root, Options{
		Plugins: []lang.Plugin{p}, ResolveDepth: -1, Trace: rep,
	}); err != nil {
		t.Fatal(err)
	} else if st.Resolution != rep {
		t.Error("the stats did not hand back the report the run was given")
	}

	if len(rep.Levels) != 3 {
		t.Fatalf("levels: %+v", rep.Levels)
	}
	// One package known to start with, which answers with one more, which answers
	// with one more again - and a last round that finds the end of the chain.
	for i, want := range []struct{ asked, answered, added int }{{1, 1, 1}, {1, 1, 1}, {1, 0, 0}} {
		got := rep.Levels[i]
		if got.Asked != want.asked || got.Answered != want.answered || got.Added != want.added {
			t.Errorf("level %d: %+v, want asked %d answered %d added %d", i, got, want.asked, want.answered, want.added)
		}
	}
	answered := map[string]trace.Answer{}
	for _, l := range rep.Lookups {
		answered[l.Package] = l.Answer
	}
	// The resolver is the repository's own answer, and "deep" needs nothing - which
	// offline is indistinguishable from nothing being recorded about it.
	if answered["direct"] != trace.FromLock || answered["middle"] != trace.FromLock {
		t.Errorf("answers: %+v", answered)
	}
	if answered["deep"] != trace.NoAnswer {
		t.Errorf("a package nothing could answer for is recorded as %q", answered["deep"])
	}
	if rep.Totals.FromLock != 2 || rep.Totals.Unanswered != 1 {
		t.Errorf("totals: %+v", rep.Totals)
	}
}

// TestResolutionReportSkips checks the two silences the report has to tell apart: an
// ecosystem whose graph is nowhere to be read, and a standard library, which has no
// graph to read.
//
// Verifies: REQ-TRC-008, REQ-TRC-009, REQ-SUP-011
func TestResolutionReportSkips(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "direct\n", "b.std": "direct\n"})
	// A plugin with no Transitive half: offline, nothing can answer for it.
	silent := quietPlugin{}
	std := stdPlugin{fakePlugin{targets: map[string]lang.Target{
		"direct": {Ecosystem: "std-eco", Package: "direct", Version: "1.0.0", Pinned: true},
	}}}
	rep := trace.New(1, false, nil, nil)
	if _, _, err := Run(context.Background(), root, Options{
		Plugins: []lang.Plugin{silent, std}, ResolveDepth: 1, Trace: rep,
	}); err != nil {
		t.Fatal(err)
	}
	if len(rep.Skipped) != 1 || rep.Skipped[0].Plugin != "quiet" {
		t.Errorf("skipped: %+v", rep.Skipped)
	}
	// The standard library's plugin can answer, so its walk is not skipped - but the
	// walk finds nothing to ask about, so it asks nothing.
	if rep.Totals.Asked != 0 {
		t.Errorf("a standard library was asked about: %+v", rep.Lookups)
	}
}

// quietPlugin resolves imports but cannot say what a package depends on, which is the
// shape of every ecosystem whose graph lives outside the repository.
type quietPlugin struct{ fakePlugin }

func (quietPlugin) Name() string                 { return "quiet" }
func (quietPlugin) Claims(f *scan.File) bool     { return strings.HasSuffix(f.Path, ".fake") }
func (quietPlugin) Ecosystems() []lang.Ecosystem { return []lang.Ecosystem{{ID: "quiet-eco"}} }
func (quietPlugin) Resolver(string, []*scan.File) (lang.Resolver, error) {
	return quietResolver{}, nil
}

type quietResolver struct{}

func (quietResolver) Resolve(file string, imp lang.RawImport) lang.Target {
	return lang.Target{Ecosystem: "quiet-eco", Package: imp.Module, Version: "1.0.0", Pinned: true}
}

// counting answers like fakeResolver and counts how often each package is asked about.
type counting struct {
	mu    sync.Mutex
	asked map[string]int
}

func (c *counting) Dependencies(t lang.Target) []lang.Target {
	c.mu.Lock()
	c.asked[t.Package]++
	c.mu.Unlock()
	return fakeResolver{}.Dependencies(t)
}

// Verifies: REQ-SUP-008
func TestTheWalkAsksAboutEachPackageOnce(t *testing.T) {
	root := t.TempDir()
	// Both imported directly, and one also depends on the other.
	writeProject(t, root, map[string]string{"a.fake": "direct\nmiddle\n"})
	p := fakePlugin{targets: map[string]lang.Target{
		"direct": {Ecosystem: "fake-eco", Package: "direct", Version: "2.0.0", Pinned: true},
		"middle": {Ecosystem: "fake-eco", Package: "middle", Version: "1.0.0", Pinned: true},
	}}
	c := &counting{asked: map[string]int{}}
	if _, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{noLocks{p}}, Registry: c, ResolveDepth: -1}); err != nil {
		t.Fatal(err)
	}
	for pkg, n := range c.asked {
		if n != 1 {
			t.Errorf("%s was asked about %d times", pkg, n)
		}
	}
}

// Verifies: REQ-SUP-031
func TestTheWalkStopsWhenCancelled(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{"a.fake": "direct\n"})
	p := fakePlugin{targets: map[string]lang.Target{
		"direct": {Ecosystem: "fake-eco", Package: "direct", Version: "2.0.0", Pinned: true},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	// Cancelled while the first level is being asked about, as Ctrl+C would be.
	stop := transitiveFunc(func(t lang.Target) []lang.Target {
		cancel()
		return fakeResolver{}.Dependencies(t)
	})
	_, _, err := Run(ctx, root, Options{Plugins: []lang.Plugin{noLocks{p}}, Registry: stop, ResolveDepth: -1})
	if err != context.Canceled {
		t.Errorf("got %v, want the walk to stop with context.Canceled", err)
	}
}

type transitiveFunc func(lang.Target) []lang.Target

func (f transitiveFunc) Dependencies(t lang.Target) []lang.Target { return f(t) }

// noLocks is a fakePlugin whose resolver records no dependency graph, so what the
// walk asks goes to the Registry alone.
type noLocks struct{ fakePlugin }

func (p noLocks) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return onlyResolve{fakeResolver(p.fakePlugin)}, nil
}

type onlyResolve struct{ r fakeResolver }

func (o onlyResolve) Resolve(file string, imp lang.RawImport) lang.Target {
	return o.r.Resolve(file, imp)
}
