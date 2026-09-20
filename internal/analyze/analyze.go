// Package analyze turns a scanned project and plugin results into a graph.
package analyze

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
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
		g:     &graph.Graph{Root: filepath.Base(root), GeneratedAt: time.Now().UTC(), Edges: []*graph.Edge{}},
		nodes: map[string]*graph.Node{},
		edges: map[[2]string]bool{},
		files: map[string]bool{},
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
	}
	stats.Parsed, stats.Cached = int(parsed.Load()), int(cached.Load())
	return b.g, stats, nil
}

type builder struct {
	g     *graph.Graph
	nodes map[string]*graph.Node
	edges map[[2]string]bool
	files map[string]bool
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
		key := [2]string{fid, to}
		if b.edges[key] {
			continue
		}
		b.edges[key] = true
		b.g.Edges = append(b.g.Edges, &graph.Edge{From: fid, To: to, Kind: graph.EdgeImport, Line: im.Line})
	}
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
