package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/findings"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/history"
	"github.com/sarumaj/depphunter-cli/internal/latest"
	"github.com/sarumaj/depphunter-cli/internal/lsp"
	"github.com/sarumaj/depphunter-cli/internal/server"
)

// loaders are the served map's background loaders, each run with the newest graph
// and never twice at once.
type loaders struct {
	history, references, findings *latest.Runner[*graph.Graph]
}

// startLoaders builds the loaders that feed mapServer and runs the enabled ones on g.
// The map is served at once; history and references follow in the background and,
// in watch mode, are refreshed after changes (run).
//
// Implements: REQ-HIST-007, REQ-LSP-005, REQ-FND-022
func (a *app) startLoaders(ctx context.Context, mapServer *server.Server, g *graph.Graph) *loaders {
	historyHead, historyRead := "", false
	l := &loaders{
		history: latest.New(func(g *graph.Graph) {
			head, _ := history.Head(ctx, a.settings.Root) // "" without commits
			if historyRead && head == historyHead {
				return
			}
			historyHead, historyRead = head, true
			published("history", mapServer.SetHistory(a.loadHistory(ctx, g)))
		}),
		references: latest.New(func(g *graph.Graph) {
			published("references", mapServer.SetReferences(a.loadReferences(ctx, g)))
		}),
		findings: latest.New(func(g *graph.Graph) {
			published("findings", mapServer.SetFindings(a.loadFindings(ctx, g)))
		}),
	}
	a.runLoaders(l, g, true)
	return l
}

// published logs a dataset the map could not be given: it goes on without it.
func published(name string, err error) {
	if err != nil {
		log.Printf("%s not published: %v", name, err)
	}
}

// runLoaders runs the enabled loaders on g in the background. changed says whether
// the graph moved since the last run: only the references depend on that alone.
func (a *app) runLoaders(l *loaders, g *graph.Graph, changed bool) {
	if a.settings.History {
		go l.history.Run(g) // a commit moves HEAD
	}
	if a.settings.LSP && changed {
		go l.references.Run(g)
	}
	if a.settings.FindingsEnabled() {
		// Not only when the graph changed: a linter complains about the text of a
		// file, and fixing one changes neither its imports nor its size.
		go l.findings.Run(g)
	}
}

// loadHistory reads (or loads from the cache) the git history of the graph's files;
// nil when disabled or unavailable.
func (a *app) loadHistory(ctx context.Context, g *graph.Graph) *history.History {
	settings := a.settings
	if !settings.History {
		return nil
	}
	start := time.Now()
	h, err := history.Cached(ctx, a.cacheDirectory, settings.Root, settings.HistoryCommits)
	if err != nil {
		if !errors.Is(err, history.ErrNoHistory) && ctx.Err() == nil {
			log.Printf("git history unavailable: %v", err)
		}
		return nil
	}
	note := ""
	if h.Truncated {
		note = fmt.Sprintf(" (the newest %d; raise --history-commits for more)", h.Commits)
	}
	log.Printf("git history: %d commits%s in %s", h.Commits, note, time.Since(start).Round(time.Millisecond))
	files := map[string]bool{}
	for n := range g.Of(graph.KindFile) {
		files[n.Path] = true
	}
	return h.Only(files)
}

// loadReferences asks the installed language servers for symbol references; nil when
// disabled or when no server could answer.
//
// Implements: REQ-LSP-001, REQ-LSP-009
func (a *app) loadReferences(ctx context.Context, g *graph.Graph) *server.References {
	settings := a.settings
	if !settings.LSP {
		return nil
	}
	start := time.Now()
	r, err := lsp.Cached(ctx, a.cacheDirectory, g, lsp.Options{Root: settings.Root, Timeout: settings.LSPTimeout, Logf: log.Printf})
	if r == nil || len(r.Servers) == 0 && len(r.Edges) == 0 {
		if err != nil && ctx.Err() == nil {
			log.Printf("references unavailable: %v", err)
		}
		return nil
	}
	log.Printf("references: %d from %s in %s", len(r.Edges), strings.Join(r.Servers, ", "), time.Since(start).Round(time.Millisecond))
	return &server.References{Edges: r.Edges, Servers: r.Servers, Partial: r.Partial}
}

// loadFindings reads the scanner reports the user named and, with --online, asks the
// OSV database about every external package the map pins to a version. nil when
// nothing was asked for or nothing was found.
//
// Implements: REQ-FND-014, REQ-FND-015, REQ-MD-010, REQ-MD-012, REQ-MD-016
func (a *app) loadFindings(ctx context.Context, g *graph.Graph) *findings.Set {
	if !a.settings.FindingsEnabled() {
		return nil
	}
	start := time.Now()
	set := findings.Collect(ctx, a.findingsOptions(g))
	if set.Empty() {
		if set.Partial && ctx.Err() == nil {
			log.Print("findings: nothing could be read")
		}
		return nil
	}
	note := ""
	if set.Partial {
		note = " (incomplete)"
	}
	log.Printf("findings: %d from %s%s in %s", len(set.Findings), strings.Join(set.Sources, ", "), note,
		time.Since(start).Round(time.Millisecond))
	return set
}

// findingsOptions says which sources the findings come from. --no-vulns and
// --no-links turn off one source each, so each is asked for on its own: a run that
// wants only the link check reads no reports.
func (a *app) findingsOptions(g *graph.Graph) findings.Options {
	settings := a.settings
	options := findings.Options{Root: settings.Root, Logf: log.Printf}
	if settings.Vulnerabilities {
		options.Reports = settings.Findings
	}
	if settings.Links {
		options.Docs = findings.Documents(g)
	}
	if !settings.Online {
		return options
	}
	if settings.Vulnerabilities {
		options.OSV = findings.NewOSV(a.subdirectory("osv"), FindingsCacheTTL, indexTimeout)
		options.Packages = findings.Pinned(g, a.private.Match)
	}
	if settings.Links {
		options.Web = findings.NewWeb(a.subdirectory("links"), LinkCacheTTL, indexTimeout, a.credentials)
	}
	return options
}
