// Package analyze turns a scanned project and plugin results into a graph.
package analyze

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/links"
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
	Private func(ecosystem, packageName string) bool
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
	For(ecosystem, packageName string) (index string, known bool)
}

// TargetIndexes is the optional half of Indexes that reads the whole target, for a
// package that names a registry of its own (lang.Target.Registry). internal/index's
// Config implements it.
type TargetIndexes interface {
	ForTarget(t lang.Target) (index string, known bool)
}

// Locator is the optional half of a Registry that says where a package it was asked
// about was found. Before anything is asked a package is attributed to its
// ecosystem's primary index; one found only on an index asked beside it (an extra
// pip index, a POM's repository) is moved there afterwards.
type Locator interface {
	Located(ecosystem, packageName string) (index string, known, ok bool)
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

// Implements: REQ-MOD-001, REQ-LANG-029, REQ-LANG-030
func Run(ctx context.Context, root string, options Options) (*graph.Graph, Stats, error) {
	var stats Stats
	files, err := scan.Scan(ctx, root, options.Scan)
	if err != nil {
		return nil, stats, fmt.Errorf("scanning %s: %w", root, err)
	}
	stats.Files, stats.Resolution = len(files), options.Trace
	if t, ok := options.Registry.(Traced); ok {
		t.Trace(options.Trace)
	}
	options.Cache.BeginRun()
	b := newBuilder(root, files, options)
	counts := &counts{}
	for _, p := range options.Plugins {
		if err := b.runPlugin(ctx, p, root, files, options, counts); err != nil {
			return nil, stats, err
		}
	}
	b.relocate(options.Registry)
	b.link()
	stats.Parsed, stats.Cached, stats.ParsedFiles = int(counts.parsed.Load()), int(counts.cached.Load()), counts.files
	options.Trace.Summarize(b.g)
	options.Trace.Finish()
	options.Cache.EndRun()
	return b.g, stats, nil
}

// newBuilder starts the graph: the root directory, and every scanned file under the
// directories that hold it.
//
// Implements: REQ-LANG-014, REQ-LANG-020
func newBuilder(root string, files []*scan.File, options Options) *builder {
	b := &builder{
		g:        &graph.Graph{Root: filepath.Base(root), GeneratedAt: time.Now().UTC(), Edges: []*graph.Edge{}},
		nodes:    map[string]*graph.Node{},
		edges:    map[[2]string]bool{},
		files:    map[string]bool{},
		packages: map[string]lang.Target{},
		private:  options.Private,
		report:   options.Trace,
	}
	if options.Indexes != nil {
		b.indexes = options.Indexes(files)
		if d, ok := b.indexes.(Discovered); ok {
			options.Trace.SetSources(d.Report())
		}
	}
	b.add(&graph.Node{ID: graph.DirectoryID("."), Kind: graph.KindDirectory, Name: b.g.Root, Path: "."})
	for _, f := range files {
		b.files[f.Path] = true
		b.add(&graph.Node{
			ID: graph.FileID(f.Path), Kind: graph.KindFile, Name: path.Base(f.Path), Path: f.Path,
			Parent: b.directory(path.Dir(f.Path)), Language: f.Language, LOC: f.LOC, Bytes: f.Size,
		})
	}
	return b
}

// counts is how many files a run parsed and how many it took from the cache, and
// which were parsed.
type counts struct {
	parsed, cached atomic.Int64
	mu             sync.Mutex
	files          []string
}

// cachedExtractor extracts with p, or takes the extraction from the cache when the
// file's content, p's version and its class of file are what they were.
//
// Implements: REQ-LANG-026, REQ-LANG-027, REQ-LANG-028
func cachedExtractor(p lang.Plugin, c *cache.Cache, n *counts) lang.Extractor {
	return func(f *scan.File, source []byte) (*lang.Extraction, error) {
		key := cache.Key(p.Name(), p.Version(), lang.ClassOf(p, f), source)
		if extraction, ok := c.Get(key); ok {
			n.cached.Add(1)
			return extraction, nil
		}
		n.parsed.Add(1)
		n.mu.Lock()
		n.files = append(n.files, f.Path)
		n.mu.Unlock()
		extraction, err := p.Extract(f, source)
		if err == nil {
			c.Put(key, extraction)
		}
		return extraction, err
	}
}

// runPlugin puts what p finds in its files on the graph, walks out from the packages
// they import as far as the options allow, and passes p's notes to the report.
func (b *builder) runPlugin(ctx context.Context, p lang.Plugin, root string, files []*scan.File, options Options, n *counts) error {
	claimed := lang.Claimed(p, files)
	if len(claimed) == 0 {
		return nil
	}
	r, err := p.Resolver(root, files)
	if err != nil {
		return fmt.Errorf("%s plugin: %w", p.Name(), err)
	}
	results := lang.Results(ctx, r, claimed, cachedExtractor(p, options.Cache, n))
	if err := ctx.Err(); err != nil {
		return err
	}
	ecosystems := map[string]lang.Ecosystem{}
	for _, e := range p.Ecosystems() {
		ecosystems[e.ID] = e
	}
	for _, f := range claimed {
		if result := results[f.Path]; result != nil {
			b.fileResult(f.Path, result, ecosystems)
		}
	}
	// Only now, with every direct package of this plugin on the graph, is there
	// something to walk out from.
	// Implements: REQ-SUP-011, REQ-TRC-008
	local, _ := r.(lang.Transitive)
	switch {
	case options.ResolveDepth == 0:
	case local != nil || options.Registry != nil:
		b.expand(ctx, chain{local: local, remote: options.Registry, report: options.Trace},
			p.Name(), ecosystems, options.ResolveDepth)
		if err := ctx.Err(); err != nil {
			return err
		}
	default:
		// Neither half of the answer is available: this ecosystem keeps its
		// dependency graph outside the repository (Go modules, NuGet, Maven,
		// containers) and nothing may be asked. Silence here is not "no
		// dependencies", and the report is where the difference is kept.
		options.Trace.Skip(p.Name(),
			"the repository records no dependency graph for it, and --online was not given")
	}
	// Implements: REQ-TRC-017
	if noter, ok := r.(lang.Noter); ok {
		for _, note := range noter.Notes() {
			note.Plugin = cmp.Or(note.Plugin, p.Name())
			options.Trace.Note(note)
		}
	}
	return nil
}

// relocate moves each package the registry found somewhere other than where it was
// attributed to that index, or marks it when only an index the repository names can
// have it.
//
// Implements: REQ-SUP-018, REQ-SUP-063
func (b *builder) relocate(r lang.Transitive) {
	locator, ok := r.(Locator)
	if !ok {
		return
	}
	for id, t := range b.packages {
		n := b.nodes[id]
		if n == nil || n.Index == "" || n.Origin != "" || t.Origin != "" {
			continue
		}
		if index, known, found := locator.Located(t.Ecosystem, t.Package); found && index != "" {
			n.Index, n.IndexUnknown = index, !known
		}
	}
}

// chain asks what the repository records before it asks an index: a lock file is
// both faster and more truthful about this project than a registry can be.
//
// Implements: REQ-SUP-010, REQ-SUP-030
type chain struct {
	local, remote lang.Transitive
	report        *trace.Report
}

// Implements: REQ-SUP-030, REQ-TRC-005, REQ-TRC-006
func (c chain) Dependencies(t lang.Target) []lang.Target {
	if c.local != nil {
		if dependencies := c.local.Dependencies(t); len(dependencies) > 0 {
			answer := trace.FromLock
			// Implements: REQ-PY-015
			if in, ok := c.local.(lang.Installed); ok && in.Installed(t) {
				answer = trace.FromInstalled
			}
			c.report.Add(trace.Lookup{
				Ecosystem: t.Ecosystem, Package: t.Package, Version: t.Version,
				Answer: answer, Dependencies: len(dependencies),
			})
			return dependencies
		}
	}
	if c.remote != nil {
		return c.remote.Dependencies(t) // which records its own answer
	}
	// Offline, and the repository's own files said nothing. Whether that means the
	// package has no dependencies or that no lock file covers it cannot be told
	// apart from here, and the report says as much rather than implying the first.
	c.report.Add(trace.Lookup{
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
	private  func(ecosystem, packageName string) bool
	report   *trace.Report
}

// transitiveWorkers is how many packages are asked about at once. A lock file
// answers from memory, but an index is a round trip each, and walking the queue one
// package at a time made --resolve-depth with --online take as long as the requests
// laid end to end. Bounded, because the other end is somebody's registry.
//
// Implements: REQ-SUP-031
const transitiveWorkers = 12

// expand walks out from the packages already on the graph, adding what the project's
// lock files - and, with --online, the package indexes - say they depend on. depth
// counts levels past the direct dependencies; -1 walks until nothing new appears.
//
// A whole level is asked at once and its answers applied in the level's own order, so
// what lands on the graph does not depend on which request came back first.
//
// It stops early when ctx is canceled - with --online a walk is hundreds of requests,
// and Ctrl+C or a newer --watch change should not wait for all of them - leaving the
// graph partial; the caller checks ctx and discards it.
//
// Implements: REQ-SUP-008, REQ-SUP-012, REQ-SUP-013, REQ-TRC-004, REQ-TRC-009
func (b *builder) expand(ctx context.Context, transitive lang.Transitive, plugin string, ecosystems map[string]lang.Ecosystem, depth int) {
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
		b.report.Enter(plugin, n)
		start := time.Now()
		answers := b.ask(ctx, transitive, level)
		var next []string
		answered, added, edges := 0, 0, 0
		for i, from := range level {
			if len(answers[i]) > 0 {
				answered++
			}
			for _, dependency := range answers[i] {
				_, known := b.nodes[graph.PackageID(dependency.Ecosystem, dependency.Package)]
				id := b.target(dependency, ecosystems)
				if id == "" || id == from {
					continue
				}
				// A directory or file of the project (a workspace package an npm
				// package depends on) is on the map already, and its own imports
				// are what it needs: it gets the edge and is not asked about.
				// Implements: REQ-MOD-010, REQ-JS-004
				if dependency.Local == "" && !known {
					b.nodes[id].Transitive = true
					added++
				}
				if b.edge(from, id, graph.EdgeDepends, 0) {
					edges++
				}
				if dependency.Local != "" {
					continue
				}
				if !seen[id] {
					seen[id] = true
					next = append(next, id)
				}
			}
		}
		b.report.Done(len(level), answered, added, edges, time.Since(start))
		level = next
	}
}

// ask resolves one level of packages at once and hands back their answers in the
// order they were asked, each sorted: a resolver may answer out of a map, and the
// graph must not come out differently for it.
//
// Implements: REQ-SUP-031
func (b *builder) ask(ctx context.Context, transitive lang.Transitive, level []string) [][]lang.Target {
	answers := make([][]lang.Target, len(level))
	var group errgroup.Group
	group.SetLimit(transitiveWorkers)
	for i, id := range level {
		group.Go(func() error {
			if ctx.Err() != nil {
				return nil // canceled: what is left is not asked
			}
			dependencies := transitive.Dependencies(b.packages[id])
			slices.SortFunc(dependencies, func(a, c lang.Target) int {
				return cmp.Or(cmp.Compare(a.Ecosystem, c.Ecosystem), cmp.Compare(a.Package, c.Package), cmp.Compare(a.Local, c.Local))
			})
			answers[i] = dependencies
			return nil
		})
	}
	group.Wait()
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

// directory ensures the directory node and all its ancestors exist and returns its id.
func (b *builder) directory(p string) string {
	id := graph.DirectoryID(p)
	if _, ok := b.nodes[id]; !ok {
		b.add(&graph.Node{ID: id, Kind: graph.KindDirectory, Name: path.Base(p), Path: p, Parent: b.directory(path.Dir(p))})
	}
	return id
}

// Implements: REQ-MOD-002, REQ-MOD-003
func (b *builder) fileResult(file string, result *lang.FileResult, ecosystems map[string]lang.Ecosystem) {
	fileID := graph.FileID(file)
	for _, s := range result.Symbols {
		b.add(&graph.Node{ID: graph.SymbolID(file, s.Name), Kind: graph.KindSymbol, Name: s.Name, SymbolKind: s.Kind, Line: s.Line, Parent: fileID})
	}
	for _, imported := range result.Imports {
		to := b.target(imported.Target, ecosystems)
		if to == "" || to == fileID {
			continue
		}
		b.edge(fileID, to, graph.EdgeImport, imported.Line)
	}
}

// edge adds one edge, at most once per pair, and reports whether it did.
//
// Implements: REQ-MOD-006
func (b *builder) edge(from, to string, kind graph.EdgeKind, line int) bool {
	key := [2]string{from, to}
	if b.edges[key] {
		return false
	}
	b.edges[key] = true
	b.g.Edges = append(b.g.Edges, &graph.Edge{From: from, To: to, Kind: kind, Line: line})
	return true
}

// target is the node t names: a file or directory of the project, or an external
// package, put on the graph with its ecosystem; "" for none.
//
// Implements: REQ-SUP-001, REQ-SUP-007, REQ-SUP-014, REQ-SUP-018, REQ-SUP-037, REQ-MOD-008
func (b *builder) target(t lang.Target, ecosystems map[string]lang.Ecosystem) string {
	if t.Local != "" {
		if b.files[t.Local] {
			return graph.FileID(t.Local)
		}
		if _, ok := b.nodes[graph.DirectoryID(t.Local)]; ok {
			return graph.DirectoryID(t.Local)
		}
		return ""
	}
	if t.Ecosystem == "" || t.Package == "" {
		return ""
	}
	n := b.packageNode(t, ecosystems)
	b.attribute(n, t)
	merge(n, t)
	return n.ID
}

// Public is the optional half of Indexes that tells an ecosystem's public index from
// every other. internal/index's Config implements it.
type Public interface {
	Public(ecosystem, index string) bool
}

// link says where each package can be looked at: its repository, and its page on
// the public index when that is where it resolves from. Without Public to say so,
// only a package attributed to no index is taken to be the public one's.
//
// Implements: REQ-MOD-014
func (b *builder) link() {
	public, _ := b.indexes.(Public)
	for n := range b.g.Of(graph.KindPackage) {
		ecosystem := graph.EcosystemOf(n.Parent)
		n.Repository = links.Repository(ecosystem, n.Name, n.Index, n.Origin, n.Git)
		if n.Private || n.IndexUnknown || n.Index != "" && (public == nil || !public.Public(ecosystem, n.Index)) {
			continue
		}
		n.Page = links.Page(ecosystem, n.Name, n.Index)
	}
}

// packageNode is t's package node, added under its ecosystem's the first time; the
// target that first named it is the one its own dependencies are asked with.
func (b *builder) packageNode(t lang.Target, ecosystems map[string]lang.Ecosystem) *graph.Node {
	ecosystem := ecosystems[t.Ecosystem]
	ecosystemID := b.add(&graph.Node{ID: graph.EcosystemID(t.Ecosystem), Kind: graph.KindEcosystem, Name: cmp.Or(ecosystem.Name, t.Ecosystem), Std: ecosystem.Std}).ID
	n := b.add(&graph.Node{ID: graph.PackageID(t.Ecosystem, t.Package), Kind: graph.KindPackage, Name: t.Package, Parent: ecosystemID, Unresolved: t.Unresolved})
	if _, ok := b.packages[n.ID]; !ok {
		b.packages[n.ID] = t
	}
	return n
}

// attribute says where a package comes from: the index that serves it, and whether it
// is the organization's own.
func (b *builder) attribute(n *graph.Node, t lang.Target) {
	if b.indexes != nil && n.Index == "" && n.Origin == "" && t.Origin == "" {
		index, known := "", false
		if ti, ok := b.indexes.(TargetIndexes); ok {
			index, known = ti.ForTarget(t)
		} else {
			index, known = b.indexes.For(t.Ecosystem, t.Package)
		}
		if index != "" {
			n.Index, n.IndexUnknown = index, !known
		}
	}
	if b.private != nil && !n.Private {
		n.Private = b.private(t.Ecosystem, t.Package)
	}
	// Installed from outside every index, it is as good as the organization's own:
	// asking an index or the vulnerability database about it could only disclose it.
	// It resolves from where it was installed, not from the ecosystem's index, so it
	// is attributed to no index and cannot be marked as coming from one nothing here
	// vouches for.
	// Implements: REQ-PY-015
	if t.Origin != "" {
		n.Private = true
		n.Index, n.IndexUnknown = "", false
		n.Origin = cmp.Or(n.Origin, t.Origin)
	}
}

// merge adds what another requester of the package says of it: the first version,
// source and platform named stand.
func merge(n *graph.Node, t lang.Target) {
	n.Version = cmp.Or(n.Version, t.Version)
	n.Git = cmp.Or(n.Git, t.Git)
	// Implements: REQ-JS-018
	n.Platform = cmp.Or(n.Platform, t.Platform)
	n.Requested = cmp.Or(n.Requested, t.Requested)
	// A package one manifest pins and another leaves open is only as fixed as its
	// loosest requester, which is what a supply chain answers to. A version has to be
	// known before it can be called floating, unless the plugin says the reference
	// moves although it names none.
	if t.Floating || (!t.Pinned && (t.Version != "" || t.Requested != "")) {
		n.Floating = true
	}
}
