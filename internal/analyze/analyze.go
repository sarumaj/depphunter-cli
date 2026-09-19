// Package analyze turns a scanned project and plugin results into a graph.
package analyze

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type Options struct {
	Scan    scan.Options
	Plugins []lang.Plugin
}

func Run(ctx context.Context, root string, opts Options) (*graph.Graph, error) {
	files, err := scan.Scan(ctx, root, opts.Scan)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", root, err)
	}

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
		var claimed []*scan.File
		for _, f := range files {
			if p.Claims(f) {
				claimed = append(claimed, f)
			}
		}
		if len(claimed) == 0 {
			continue
		}
		results, err := p.Analyze(ctx, root, files, claimed)
		if err != nil {
			return nil, fmt.Errorf("%s plugin: %w", p.Name(), err)
		}
		ecos := map[string]lang.Ecosystem{}
		for _, e := range p.Ecosystems() {
			ecos[e.ID] = e
		}
		for _, f := range claimed {
			if res := results[f.Path]; res != nil {
				b.fileResult(f.Path, res, ecos)
			}
		}
	}
	return b.g, nil
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

func (b *builder) fileResult(file string, res *lang.FileResult, ecos map[string]lang.Ecosystem) {
	fid := graph.FileID(file)
	for _, s := range res.Symbols {
		b.add(&graph.Node{ID: graph.SymbolID(file, s.Name), Kind: graph.KindSymbol, Name: s.Name, SymbolKind: s.Kind, Line: s.Line, Parent: fid})
	}
	for _, im := range res.Imports {
		to := b.target(im.Target, ecos)
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

func (b *builder) target(t lang.Target, ecos map[string]lang.Ecosystem) string {
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
	eco := ecos[t.Ecosystem]
	name := eco.Name
	if name == "" {
		name = t.Ecosystem
	}
	eid := b.add(&graph.Node{ID: graph.EcosystemID(t.Ecosystem), Kind: graph.KindEcosystem, Name: name, Std: eco.Std}).ID
	n := b.add(&graph.Node{ID: graph.PackageID(t.Ecosystem, t.Package), Kind: graph.KindPackage, Name: t.Package, Parent: eid, Unresolved: t.Unresolved})
	if n.Version == "" {
		n.Version = t.Version
	}
	return n.ID
}
