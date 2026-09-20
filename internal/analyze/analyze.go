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
)

type Options struct {
	Scan    scan.Options
	Plugins []lang.Plugin
	Cache   *cache.Cache // nil disables caching
	// ResolveDepth is how many levels of an external package's own dependencies to
	// add, from the project's lock files: 0 none, -1 as far as they reach.
	ResolveDepth int
}

// Stats describes one run: how many files were parsed and how many came from the cache.
type Stats struct {
	Files, Parsed, Cached int
	ParsedFiles           []string // paths whose contents were (re-)parsed
}

func Run(ctx context.Context, root string, opts Options) (*graph.Graph, Stats, error) {
	var stats Stats
	files, err := scan.Scan(ctx, root, opts.Scan)
	if err != nil {
		return nil, stats, fmt.Errorf("scanning %s: %w", root, err)
	}
	stats.Files = len(files)
	opts.Cache.BeginRun()
	var parsed, cached atomic.Int64
	var mu sync.Mutex

	b := &builder{
		g:        &graph.Graph{Root: filepath.Base(root), GeneratedAt: time.Now().UTC(), Edges: []*graph.Edge{}},
		nodes:    map[string]*graph.Node{},
		edges:    map[[2]string]bool{},
		files:    map[string]bool{},
		packages: map[string]lang.Target{},
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
		if tr, ok := r.(lang.Transitive); ok && opts.ResolveDepth != 0 {
			b.expand(tr, ecosystems, opts.ResolveDepth)
		}
	}
	stats.Parsed, stats.Cached = int(parsed.Load()), int(cached.Load())
	return b.g, stats, nil
}

type builder struct {
	g     *graph.Graph
	nodes map[string]*graph.Node
	edges map[[2]string]bool
	files map[string]bool
	// packages remembers what each package node was built from, so the transitive
	// walk can ask a resolver about it again.
	packages map[string]lang.Target
}

// expand walks out from the packages already on the graph, adding what the project's
// lock files say they depend on. depth counts levels past the direct dependencies;
// -1 walks until nothing new appears.
func (b *builder) expand(tr lang.Transitive, ecosystems map[string]lang.Ecosystem, depth int) {
	type step struct {
		id    string
		level int
	}
	var queue []step
	for id, t := range b.packages {
		if _, ok := ecosystems[t.Ecosystem]; ok {
			queue = append(queue, step{id, 0})
		}
	}
	// A stable order keeps the graph (and the tests reading it) the same run to run.
	sort.Slice(queue, func(i, j int) bool { return queue[i].id < queue[j].id })

	seen := map[string]bool{}
	for i := 0; i < len(queue); i++ {
		cur := queue[i]
		if depth >= 0 && cur.level >= depth {
			continue
		}
		for _, dep := range tr.Dependencies(b.packages[cur.id]) {
			_, known := b.nodes[graph.PackageID(dep.Ecosystem, dep.Package)]
			id := b.target(dep, ecosystems)
			if id == "" || id == cur.id {
				continue
			}
			if !known {
				b.nodes[id].Transitive = true
			}
			b.edge(cur.id, id, graph.EdgeDepends, 0)
			if !seen[id] {
				seen[id] = true
				queue = append(queue, step{id, cur.level + 1})
			}
		}
	}
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
