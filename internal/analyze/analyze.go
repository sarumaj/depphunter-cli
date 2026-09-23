// Package analyze turns a scanned project and plugin results into a graph.
package analyze

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

type Options struct {
	Scan    scan.Options
	Plugins []lang.Plugin
	Cache   *cache.Cache // nil disables caching
	// ResolveDepth is how many levels of an external package's own dependencies to
	// add, from the project's lock files: 0 none, -1 as far as they reach.
	ResolveDepth int
	// Registry answers for packages whose own dependencies the repository does not
	// record - Go modules, say - when --online allows asking an index. nil keeps
	// the analysis offline.
	Registry lang.Transitive
	// Indexes answers where an external package comes from. It is built from the
	// scanned files, since a repository configures its indexes in its own files;
	// nil leaves the question unanswered.
	Indexes func(files []*scan.File) Indexes
	// Private reports whether a package is the organization's own (internal/scope).
	// Such a package is marked on the graph, and what is marked is never named to a
	// public index or to the vulnerability database. nil makes everything public,
	// which is what a repository of open-source dependencies is.
	Private func(eco, pkg string) bool
	// Trace is the report this run writes its account of itself into: which index
	// answered for what, and what each level of the walk added (internal/trace). One
	// report belongs to one run, so --watch hands a fresh one to every re-analysis.
	// nil records nothing.
	Trace *trace.Report
}

// Traced is the optional half of a Registry that can say how it answered each
// question. internal/index's client implements it; a registry that does not is
// simply not heard from in the report.
type Traced interface{ Trace(*trace.Report) }

// Indexes says which package index serves a package, and whether anything on this
// machine vouches for that index (see internal/index).
type Indexes interface {
	For(eco, pkg string) (index string, known bool)
}

// Discovered is the optional half of Indexes: every index the run knew about and
// where it was learned from, which is what the resolution report opens with.
// internal/index's Config implements it.
type Discovered interface{ Report() []trace.Source }

// Stats describes one run: how many files were parsed and how many came from the cache.
type Stats struct {
	Files, Parsed, Cached int
	ParsedFiles           []string // paths whose contents were (re-)parsed
	// Resolution is Options.Trace, filled in, for callers that would rather not keep
	// hold of what they passed.
	Resolution *trace.Report
}

func Run(ctx context.Context, root string, opts Options) (*graph.Graph, Stats, error) {
	var stats Stats
	files, err := scan.Scan(ctx, root, opts.Scan)
	if err != nil {
		return nil, stats, fmt.Errorf("scanning %s: %w", root, err)
	}
	stats.Files, stats.Resolution = len(files), opts.Trace
	if t, ok := opts.Registry.(Traced); ok {
		t.Trace(opts.Trace)
	}
	opts.Cache.BeginRun()
	var parsed, cached atomic.Int64
	var mu sync.Mutex

	b := &builder{
		g:        &graph.Graph{Root: filepath.Base(root), GeneratedAt: time.Now().UTC(), Edges: []*graph.Edge{}},
		nodes:    map[string]*graph.Node{},
		edges:    map[[2]string]bool{},
		files:    map[string]bool{},
		packages: map[string]lang.Target{},
		private:  opts.Private,
		rep:      opts.Trace,
	}
	if opts.Indexes != nil {
		b.indexes = opts.Indexes(files)
		if d, ok := b.indexes.(Discovered); ok {
			opts.Trace.SetSources(d.Report())
		}
	}
	b.add(&graph.Node{ID: graph.DirID("."), Kind: graph.KindDir, Name: b.g.Root, Path: "."})
	for _, f := range files {
		b.files[f.Path] = true
		b.add(&graph.Node{
			ID: graph.FileID(f.Path), Kind: graph.KindFile, Name: path.Base(f.Path), Path: f.Path,
			Parent: b.dir(path.Dir(f.Path)), Lang: f.Lang, LOC: f.LOC,
		})
	}

	for _, p := range opts.Plugins {
		claimed := lang.Claimed(p, files)
		if len(claimed) == 0 {
			continue
		}
		r, err := p.Resolver(root, files)
		if err != nil {
			return nil, stats, fmt.Errorf("%s plugin: %w", p.Name(), err)
		}
		results := lang.ForEachFile(ctx, claimed, func(f *scan.File, src []byte) *lang.FileResult {
			key := cache.Key(p.Name(), p.Version(), f.Path, src)
			if ex, ok := opts.Cache.Get(key); ok {
				cached.Add(1)
				return lang.Apply(r, f.Path, ex)
			}
			parsed.Add(1)
			mu.Lock()
			stats.ParsedFiles = append(stats.ParsedFiles, f.Path)
			mu.Unlock()
			ex, err := p.Extract(f, src)
			if err != nil {
				return nil
			}
			opts.Cache.Put(key, ex)
			return lang.Apply(r, f.Path, ex)
		})
		if err := ctx.Err(); err != nil {
			return nil, stats, err
		}
		ecosystems := map[string]lang.Ecosystem{}
		for _, e := range p.Ecosystems() {
			ecosystems[e.ID] = e
		}
		for _, f := range claimed {
			if res := results[f.Path]; res != nil {
				b.fileResult(f.Path, res, ecosystems)
			}
		}
		// Only now, with every direct package of this plugin on the graph, is there
		// something to walk out from.
		local, _ := r.(lang.Transitive)
		switch {
		case opts.ResolveDepth == 0:
		case local != nil || opts.Registry != nil:
			b.expand(ctx, chain{local: local, remote: opts.Registry, rep: opts.Trace},
				p.Name(), ecosystems, opts.ResolveDepth)
			if err := ctx.Err(); err != nil {
				return nil, stats, err
			}
		default:
			// Neither half of the answer is available: this ecosystem keeps its
			// dependency graph outside the repository (Go modules, NuGet, Maven,
			// containers) and nothing may be asked. Silence here is not "no
			// dependencies", and the report is where the difference is kept.
			opts.Trace.Skip(p.Name(),
				"the repository records no dependency graph for it, and --online was not given")
		}
	}
	stats.Parsed, stats.Cached = int(parsed.Load()), int(cached.Load())
	opts.Trace.Summarize(b.g)
	opts.Trace.Finish()
	opts.Cache.EndRun()
	return b.g, stats, nil
}

// chain asks what the repository records before it asks an index: a lock file is
// both faster and more truthful about this project than a registry can be.
type chain struct {
	local, remote lang.Transitive
	rep           *trace.Report
}

func (c chain) Dependencies(t lang.Target) []lang.Target {
	if c.local != nil {
		if deps := c.local.Dependencies(t); len(deps) > 0 {
			c.rep.Add(trace.Lookup{
				Ecosystem: t.Ecosystem, Package: t.Package, Version: t.Version,
				Answer: trace.FromLock, Deps: len(deps),
			})
			return deps
		}
	}
	if c.remote != nil {
		return c.remote.Dependencies(t) // which records its own answer
	}
	// Offline, and the repository's own files said nothing. Whether that means the
	// package has no dependencies or that no lock file covers it cannot be told
	// apart from here, and the report says as much rather than implying the first.
	c.rep.Add(trace.Lookup{
		Ecosystem: t.Ecosystem, Package: t.Package, Version: t.Version,
		Answer: trace.NoAnswer, Reason: trace.ReasonOffline,
	})
	return nil
}

type builder struct {
	g     *graph.Graph
	nodes map[string]*graph.Node
	edges map[[2]string]bool
	files map[string]bool
	// packages remembers what each package node was built from, so the transitive
	// walk can ask a resolver about it again.
	packages map[string]lang.Target
	indexes  Indexes
	private  func(eco, pkg string) bool
	rep      *trace.Report
}

// transitiveWorkers is how many packages are asked about at once. A lock file
// answers from memory, but an index is a round trip each, and walking the queue one
// package at a time made --resolve-depth with --online take as long as the requests
// laid end to end. Bounded, because the other end is somebody's registry.
const transitiveWorkers = 12

// expand walks out from the packages already on the graph, adding what the project's
// lock files - and, with --online, the package indexes - say they depend on. depth
// counts levels past the direct dependencies; -1 walks until nothing new appears.
//
// A whole level is asked at once and its answers applied in the level's own order, so
// what lands on the graph does not depend on which request came back first.
//
// It stops early when ctx is cancelled - with --online a walk is hundreds of requests,
// and Ctrl+C or a newer --watch change should not wait for all of them - leaving the
// graph partial; the caller checks ctx and discards it.
func (b *builder) expand(ctx context.Context, tr lang.Transitive, plugin string, ecosystems map[string]lang.Ecosystem, depth int) {
	var level []string
	for id, t := range b.packages {
		// A standard library is not walked: nothing publishes what "fs" or "os"
		// depends on, so asking produces a round of questions nobody can answer and
		// a report full of them.
		if e, ok := ecosystems[t.Ecosystem]; ok && !e.Std {
			level = append(level, id)
		}
	}
	// A stable order keeps the graph (and the tests reading it) the same run to run.
	sort.Strings(level)

	// The first level is seen already: a direct dependency that another one also
	// depends on (react-dom -> react) is not asked about a second time.
	seen := map[string]bool{}
	for _, id := range level {
		seen[id] = true
	}
	for n := 0; len(level) > 0 && (depth < 0 || n < depth) && ctx.Err() == nil; n++ {
		b.rep.Enter(plugin, n)
		start := time.Now()
		answers := b.ask(ctx, tr, level)
		var next []string
		answered, added, edges := 0, 0, 0
		for i, from := range level {
			if len(answers[i]) > 0 {
				answered++
			}
			for _, dep := range answers[i] {
				_, known := b.nodes[graph.PackageID(dep.Ecosystem, dep.Package)]
				id := b.target(dep, ecosystems)
				if id == "" || id == from {
					continue
				}
				if !known {
					b.nodes[id].Transitive = true
					added++
				}
				before := len(b.g.Edges)
				b.edge(from, id, graph.EdgeDepends, 0)
				edges += len(b.g.Edges) - before
				if !seen[id] {
					seen[id] = true
					next = append(next, id)
				}
			}
		}
		b.rep.Done(len(level), answered, added, edges, time.Since(start))
		level = next
	}
}

// ask resolves one level of packages at once and hands back their answers in the
// order they were asked, each sorted: a resolver may answer out of a map, and the
// graph must not come out differently for it.
func (b *builder) ask(ctx context.Context, tr lang.Transitive, level []string) [][]lang.Target {
	answers := make([][]lang.Target, len(level))
	workers := min(transitiveWorkers, len(level))
	var wg sync.WaitGroup
	work := make(chan int)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				if ctx.Err() != nil {
					continue // cancelled: what is left is not asked
				}
				deps := tr.Dependencies(b.packages[level[i]])
				sort.Slice(deps, func(a, c int) bool {
					if deps[a].Ecosystem != deps[c].Ecosystem {
						return deps[a].Ecosystem < deps[c].Ecosystem
					}
					return deps[a].Package < deps[c].Package
				})
				answers[i] = deps
			}
		}()
	}
	for i := range level {
		work <- i
	}
	close(work)
	wg.Wait()
	return answers
}

func (b *builder) add(n *graph.Node) *graph.Node {
	if old, ok := b.nodes[n.ID]; ok {
		return old
	}
	b.nodes[n.ID] = n
	b.g.Nodes = append(b.g.Nodes, n)
	return n
}

// dir ensures the directory node and all its ancestors exist and returns its id.
func (b *builder) dir(p string) string {
	id := graph.DirID(p)
	if _, ok := b.nodes[id]; !ok {
		b.add(&graph.Node{ID: id, Kind: graph.KindDir, Name: path.Base(p), Path: p, Parent: b.dir(path.Dir(p))})
	}
	return id
}

func (b *builder) fileResult(file string, res *lang.FileResult, ecosystems map[string]lang.Ecosystem) {
	fid := graph.FileID(file)
	for _, s := range res.Symbols {
		b.add(&graph.Node{ID: graph.SymbolID(file, s.Name), Kind: graph.KindSymbol, Name: s.Name, SymbolKind: s.Kind, Line: s.Line, Parent: fid})
	}
	for _, im := range res.Imports {
		to := b.target(im.Target, ecosystems)
		if to == "" || to == fid {
			continue
		}
		b.edge(fid, to, graph.EdgeImport, im.Line)
	}
}

// edge adds one edge, at most once per pair.
func (b *builder) edge(from, to string, kind graph.EdgeKind, line int) {
	key := [2]string{from, to}
	if b.edges[key] {
		return
	}
	b.edges[key] = true
	b.g.Edges = append(b.g.Edges, &graph.Edge{From: from, To: to, Kind: kind, Line: line})
}

func (b *builder) target(t lang.Target, ecosystems map[string]lang.Ecosystem) string {
	if t.Local != "" {
		if b.files[t.Local] {
			return graph.FileID(t.Local)
		}
		if _, ok := b.nodes[graph.DirID(t.Local)]; ok {
			return graph.DirID(t.Local)
		}
		return ""
	}
	if t.Ecosystem == "" || t.Package == "" {
		return ""
	}
	eco := ecosystems[t.Ecosystem]
	name := eco.Name
	if name == "" {
		name = t.Ecosystem
	}
	eid := b.add(&graph.Node{ID: graph.EcosystemID(t.Ecosystem), Kind: graph.KindEcosystem, Name: name, Std: eco.Std}).ID
	n := b.add(&graph.Node{ID: graph.PackageID(t.Ecosystem, t.Package), Kind: graph.KindPackage, Name: t.Package, Parent: eid, Unresolved: t.Unresolved})
	if _, ok := b.packages[n.ID]; !ok {
		b.packages[n.ID] = t
	}
	if b.indexes != nil && n.Index == "" {
		if idx, known := b.indexes.For(t.Ecosystem, t.Package); idx != "" {
			n.Index, n.IndexUnknown = idx, !known
		}
	}
	if b.private != nil && !n.Private {
		n.Private = b.private(t.Ecosystem, t.Package)
	}
	if n.Version == "" {
		n.Version = t.Version
	}
	if n.Requested == "" {
		n.Requested = t.Requested
	}
	// A package one manifest pins and another leaves open is only as fixed as its
	// loosest requester, which is what a supply chain answers to. A version has to be
	// known before it can be called floating, unless the plugin says the reference
	// moves although it names none.
	if t.Floating || (!t.Pinned && (t.Version != "" || t.Requested != "")) {
		n.Floating = true
	}
	return n.ID
}
