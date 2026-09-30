// Package app is what the depphunter command runs once its flags are parsed: it wires
// the cache, the plugins, the credentials, the private scope, the index discovery and
// the index client into one analysis, and then either writes the export or serves the
// map (serve.go), with the background loaders (loaders.go) and, with --watch, the
// re-analysis after every change.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/analyze"
	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/index"
	"github.com/sarumaj/depphunter-cli/internal/lang/all"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// How long an index's answer stays usable, and how long one request may take.
const (
	// Implements: REQ-SUP-032
	indexCacheTTL = 24 * time.Hour
	indexTimeout  = 30 * time.Second
	// FindingsCacheTTL is how long the OSV database's answers are kept. Advisories are
	// published against versions that are already released, so what the database said
	// yesterday is almost always still true - but not for long enough to keep for a
	// week.
	FindingsCacheTTL = 6 * time.Hour
	// LinkCacheTTL is how long a link's answer is kept. Link rot is slow, and asking a
	// hundred hosts on every run is the kind of thing that gets a tool blocked: a
	// link's answer is kept for a day.
	//
	// Implements: REQ-MD-014
	LinkCacheTTL = 24 * time.Hour
)

// app is one run of the command: the settings, and what is built from them once and
// shared by the first analysis, the export or the served map, and every re-analysis.
type app struct {
	settings config.Config
	// cache is the extraction cache; nil with --no-cache.
	cache *cache.Cache
	// cacheDirectory also holds git histories, references and index answers; ""
	// disables caching.
	cacheDirectory string
	options        analyze.Options
	credentials    *auth.Store
	private        *scope.Private
}

// Run analyzes settings.Root once, and then writes the export --export asked for, or
// serves the map until ctx is canceled. The log's output is the caller's to choose
// before calling Run.
func Run(ctx context.Context, settings config.Config) error {
	a := newApp(settings)
	a.wireIndexes()
	a.options.Trace = a.newReport()
	g, err := a.analyze(ctx)
	if err != nil {
		return err
	}
	if settings.Export != "" {
		return a.export(ctx, g)
	}
	return a.serve(ctx, g)
}

// newApp opens the extraction cache (unless --no-cache) and assembles the analysis
// options that depend on nothing but the settings.
func newApp(settings config.Config) *app {
	a := &app{settings: settings}
	if settings.Cache {
		if d, err := os.UserCacheDir(); err == nil {
			a.cacheDirectory = filepath.Join(d, "depphunter")
			a.cache = cache.Open(a.cacheDirectory, settings.Root)
		}
	}
	a.options = analyze.Options{
		Scan:         scan.Options{Exclude: settings.Exclude, MaxFileSize: settings.MaxFileSize},
		Plugins:      all.Plugins(all.Options{Python: settings.Python, Getenv: os.Getenv}),
		Cache:        a.cache,
		ResolveDepth: settings.ResolveDepth,
	}
	return a
}

// wireIndexes reads the machine's credentials, settles the private scope, and hands
// both to the index discovery and, with --online, to the index client. The order is
// the point: the private scope is owned by the discovery's configuration before any
// index is discovered, and the client is built over that same configuration.
func (a *app) wireIndexes() {
	settings := a.settings
	home, _ := os.UserHomeDir()
	// What this machine already holds for its registries and indexes. Read once: the
	// index client, the container registries and the link check all send from it, and
	// each credential goes only to the host it was written for. The files Yarn and
	// NuGet read in the directories above a project are this machine's above the
	// analyzed checkout.
	a.credentials = auth.ReadFor(home, settings.Root, os.Getenv)
	// What this organization owns: what was declared, plus what the machine already
	// says about private Go modules (internal/scope), in the environment or in the
	// file `go env -w` writes, as the go command reads them.
	// Implements: REQ-SUP-036
	a.private = scope.New(append(append([]string{}, settings.Private...), scope.FromGoEnvironment(userconf.New(home, os.Getenv).GoEnvironment)...))
	if !a.private.Empty() {
		log.Printf("private: %s", strings.Join(a.private.Patterns(), ", "))
	}
	a.options.Private = a.private.Match
	indexes := index.NewDiscoverer(os.Getenv, home)
	indexes.Root(settings.Root)
	indexes.Config().Credentials(a.credentials)
	indexes.Config().Trust(settings.TrustIndexes)
	indexes.Config().Private(a.private.Match)
	a.options.Indexes = func(files []*scan.File) analyze.Indexes { return indexes.Discover(files) }
	// Implements: REQ-DIST-003, REQ-SUP-010, REQ-SUP-020
	if !settings.Online {
		return
	}
	// The configuration is filled while the scan runs; the client only reads it
	// afterwards, when the walk starts asking about packages. Without a cache
	// directory (--no-cache) the answers are kept for this run only, rather than
	// written to a relative path inside the analyzed project.
	a.options.Registry = index.NewClient(indexes.Config(), a.subdirectory("index"), indexCacheTTL, indexTimeout, a.credentials)
}

// subdirectory is name inside the cache directory, or "" when there is none
// (--no-cache): the stores that take it then keep their answers for this run only.
func (a *app) subdirectory(name string) string {
	if a.cacheDirectory == "" {
		return ""
	}
	return filepath.Join(a.cacheDirectory, name)
}

// newReport starts the resolution report of one analysis. One report per analysis:
// --watch analyzes again on every change, and a report that accumulated over a
// morning's editing describes no run in particular.
//
// Implements: REQ-TRC-001, REQ-TRC-016
func (a *app) newReport() *trace.Report {
	return trace.New(a.settings.ResolveDepth, a.options.Registry != nil, a.private.Patterns(), a.settings.TrustIndexes)
}

// analyze runs the first analysis, logs it and the resolution report, and saves the
// extraction cache.
func (a *app) analyze(ctx context.Context) (*graph.Graph, error) {
	start := time.Now()
	g, stats, err := analyze.Run(ctx, a.settings.Root, a.options)
	if err != nil {
		return nil, err
	}
	log.Printf("analyzed %s: %d files (%d parsed, %d cached), %d nodes, %d edges in %s",
		a.settings.Root, stats.Files, stats.Parsed, stats.Cached, len(g.Nodes), len(g.Edges), time.Since(start).Round(time.Millisecond))
	explain(a.settings, stats.Resolution)
	if err := a.cache.Save(); err != nil {
		log.Printf("cache not saved: %v", err)
	}
	return g, nil
}

// explain writes the resolution report when --explain asked for it, wherever the log
// goes - which is where the VS Code extension reads it from too.
//
// Implements: REQ-TRC-010, REQ-TRC-013
func explain(settings config.Config, r *trace.Report) {
	if !settings.Explain || r == nil {
		return
	}
	w := log.Writer()
	fmt.Fprintln(w)
	if err := r.Text(w); err != nil {
		log.Printf("resolution report: %v", err)
	}
	fmt.Fprintln(w)
}
