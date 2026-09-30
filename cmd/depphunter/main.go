// Command depphunter opens an interactive map of the code base in the current directory.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cli/browser"
	"github.com/spf13/cobra"

	anal "github.com/sarumaj/depphunter-cli/internal/analyze"
	"github.com/sarumaj/depphunter-cli/internal/auth"
	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/editor"
	"github.com/sarumaj/depphunter-cli/internal/export"
	"github.com/sarumaj/depphunter-cli/internal/findings"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/history"
	"github.com/sarumaj/depphunter-cli/internal/index"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/all"
	"github.com/sarumaj/depphunter-cli/internal/lsp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/server"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
	"github.com/sarumaj/depphunter-cli/internal/watch"
	"github.com/sarumaj/depphunter-cli/web"
)

// How long an index's answer stays usable, and how long one request may take.
const (
	// Implements: REQ-SUP-032
	indexCacheTTL = 24 * time.Hour
	indexTimeout  = 30 * time.Second
	// Advisories are published against versions that are already released, so what the
	// database said yesterday is almost always still true - but not for long enough to
	// keep for a week.
	findingsCacheTTL = 6 * time.Hour
	// Link rot is slow, and asking a hundred hosts on every run is the kind of thing
	// that gets a tool blocked: a link's answer is kept for a day.
	//
	// Implements: REQ-MD-014
	linkCacheTTL = 24 * time.Hour
)

// version is set at release builds: -ldflags "-X main.version=v1.2.3".
var version = "dev"

// Implements: REQ-CLI-008, REQ-CLI-011
func main() {
	log.SetFlags(0)
	log.SetPrefix("depphunter: ")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := newCommand().ExecuteContext(ctx)
	stop()
	if err != nil {
		// A failure is not part of the output anybody asked for, wherever the log
		// was going by then (logOutput).
		log.SetOutput(os.Stderr)
		log.Println(err)
		os.Exit(1)
	}
}

// logOutput is where depphunter's own log goes: stdout, like any other output of a
// command - except where stdout is already carrying an export with no file to go to,
// since "analyzed …" in the middle of a JSON graph is no use to anyone.
//
// Implements: REQ-CLI-009, REQ-CLI-010
func logOutput(settings config.Config) io.Writer {
	if settings.Export != "" && settings.Output == "" {
		return os.Stderr
	}
	return os.Stdout
}

// newCommand is the depphunter command: flags are declared by the config package,
// which layers them over the config files and environment (viper).
//
// Implements: REQ-CLI-001, REQ-CLI-002, REQ-CLI-003, REQ-CLI-005, REQ-CLI-006, REQ-CLI-007, REQ-CLI-008
func newCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "depphunter [path]",
		Short: "Browse a code base as an interactive isometric map",
		Long: `Opens an interactive map of the code base at path (default: the current
directory) in the browser, or writes the dependency graph with --export.

Settings come from, in increasing precedence: defaults, the user config
(<user config dir>/depphunter/config.yaml), the project config
(<path>/` + config.ProjectFile + ` or --config), DEPPHUNTER_* environment
variables, and flags.`,
		Example: `  depphunter                          # the current directory
  depphunter ~/src/app --watch        # keep the map in sync while you edit
  depphunter --findings trivy.json    # put what a scanner reported on the map
  depphunter --export html -o map.html`,
		Args:          cobra.MaximumNArgs(1),
		Version:       version,
		SilenceUsage:  true, // errors are about the input, not the syntax
		SilenceErrors: true, // main logs them
		RunE: func(command *cobra.Command, arguments []string) error {
			userDirectory := ""
			if d, err := os.UserConfigDir(); err == nil {
				userDirectory = filepath.Join(d, "depphunter")
			}
			settings, err := config.Load(command.Flags(), arguments, userDirectory)
			if err != nil {
				return err
			}
			return run(command.Context(), settings)
		},
	}
	command.SetVersionTemplate("depphunter {{.Version}}\n")
	command.Flags().SortFlags = false
	config.RegisterFlags(command.Flags())
	return command
}

func run(ctx context.Context, settings config.Config) error {
	log.SetOutput(logOutput(settings))
	var c *cache.Cache
	cacheDirectory := "" // also holds git histories; "" disables caching
	if settings.Cache {
		if d, err := os.UserCacheDir(); err == nil {
			cacheDirectory = filepath.Join(d, "depphunter")
			c = cache.Open(cacheDirectory, settings.Root)
		}
	}
	options := anal.Options{
		Scan:         scan.Options{Exclude: settings.Exclude, MaxFileSize: settings.MaxFileSize},
		Plugins:      all.Plugins(all.Options{Python: settings.Python, Getenv: os.Getenv}),
		Cache:        c,
		ResolveDepth: settings.ResolveDepth,
	}
	home, _ := os.UserHomeDir()
	// What this machine already holds for its registries and indexes. Read once: the
	// index client, the container registries and the link check all send from it, and
	// each credential goes only to the host it was written for. The files Yarn and
	// NuGet read in the directories above a project are this machine's above the
	// analyzed checkout.
	credentials := auth.ReadFor(home, settings.Root, os.Getenv)
	// What this organization owns: what was declared, plus what the machine already
	// says about private Go modules (internal/scope), in the environment or in the
	// file `go env -w` writes, as the go command reads them.
	// Implements: REQ-SUP-036
	private := scope.New(append(append([]string{}, settings.Private...), scope.FromGoEnvironment(userconf.New(home, os.Getenv).GoEnvironment)...))
	if !private.Empty() {
		log.Printf("private: %s", strings.Join(private.Patterns(), ", "))
	}
	options.Private = private.Match
	indexes := index.NewDiscoverer(os.Getenv, home)
	indexes.Root(settings.Root)
	indexes.Config().Credentials(credentials)
	indexes.Config().Trust(settings.TrustIndexes)
	indexes.Config().Private(private.Match)
	options.Indexes = func(files []*scan.File) anal.Indexes { return indexes.Discover(files) }
	// Implements: REQ-DIST-003, REQ-SUP-010, REQ-SUP-020
	if settings.Online {
		// The configuration is filled while the scan runs; the client only reads it
		// afterwards, when the walk starts asking about packages. Without a cache
		// directory (--no-cache) the answers are kept for this run only, rather than
		// written to a relative path inside the analyzed project.
		store := ""
		if cacheDirectory != "" {
			store = filepath.Join(cacheDirectory, "index")
		}
		options.Registry = index.NewClient(indexes.Config(), store, indexCacheTTL, indexTimeout, credentials)
	}
	// One report per analysis: --watch analyzes again on every change, and a report
	// that accumulated over a morning's editing describes no run in particular.
	// Implements: REQ-TRC-001, REQ-TRC-016
	newReport := func() *trace.Report {
		return trace.New(settings.ResolveDepth, options.Registry != nil, private.Patterns(), settings.TrustIndexes)
	}
	options.Trace = newReport()
	g, err := analyze(ctx, settings, options, c)
	if err != nil {
		return err
	}

	// Implements: REQ-EXP-004, REQ-EXP-010, REQ-HIST-015
	if settings.Export != "" {
		extra := map[string]any{}
		references := loadReferences(ctx, settings, cacheDirectory, g)
		if settings.Export == "html" {
			if h := loadHistory(ctx, settings, cacheDirectory, g); h != nil {
				extra["history"] = h
			}
			if references != nil {
				extra["references"] = references
			}
			if f := loadFindings(ctx, settings, cacheDirectory, g, credentials, private); !f.Empty() {
				extra["findings"] = f
			}
		} else if references != nil {
			g = export.WithEdges(g, references.Edges)
		}
		return writeExport(g, settings, extra, settings.Export, settings.Output)
	}
	return serve(ctx, settings, g, options, c, cacheDirectory, newReport, credentials, private)
}

// explain writes the resolution report when --explain asked for it, wherever the log
// goes (logOutput) - which is where the VS Code extension reads it from too.
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

// loadHistory reads (or loads from the cache) the git history of the graph's files;
// nil when disabled or unavailable.
func loadHistory(ctx context.Context, settings config.Config, cacheDirectory string, g *graph.Graph) *history.History {
	if !settings.History {
		return nil
	}
	start := time.Now()
	h, err := history.Cached(ctx, cacheDirectory, settings.Root, settings.HistoryCommits)
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
	for _, n := range g.Nodes {
		if n.Kind == graph.KindFile {
			files[n.Path] = true
		}
	}
	return h.Only(files)
}

func analyze(ctx context.Context, settings config.Config, options anal.Options, c *cache.Cache) (*graph.Graph, error) {
	start := time.Now()
	g, stats, err := anal.Run(ctx, settings.Root, options)
	if err != nil {
		return nil, err
	}
	log.Printf("analyzed %s: %d files (%d parsed, %d cached), %d nodes, %d edges in %s",
		settings.Root, stats.Files, stats.Parsed, stats.Cached, len(g.Nodes), len(g.Edges), time.Since(start).Round(time.Millisecond))
	explain(settings, stats.Resolution)
	if err := c.Save(); err != nil {
		log.Printf("cache not saved: %v", err)
	}
	return g, nil
}

// loadReferences asks the installed language servers for symbol references; nil when
// disabled or when no server could answer.
//
// Implements: REQ-LSP-001, REQ-LSP-009
func loadReferences(ctx context.Context, settings config.Config, cacheDirectory string, g *graph.Graph) *server.References {
	if !settings.LSP {
		return nil
	}
	start := time.Now()
	r, err := lsp.Cached(ctx, cacheDirectory, g, lsp.Options{Root: settings.Root, Timeout: settings.LSPTimeout, Logf: log.Printf})
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
func loadFindings(ctx context.Context, settings config.Config, cacheDirectory string, g *graph.Graph,
	credentials *auth.Store, private *scope.Private) *findings.Set {
	if !settings.FindingsEnabled() {
		return nil
	}
	start := time.Now()
	// --no-vulns and --no-links turn off one source each, so each is asked for on
	// its own: a run that wants only the link check reads no reports.
	options := findings.Options{Root: settings.Root, Logf: log.Printf}
	if settings.Vulnerabilities {
		options.Reports = settings.Findings
	}
	if settings.Links {
		options.Docs = documents(g)
	}
	if settings.Online {
		store := ""
		if cacheDirectory != "" {
			store = filepath.Join(cacheDirectory, "osv")
		}
		if settings.Vulnerabilities {
			options.OSV = findings.NewOSV(store, findingsCacheTTL, indexTimeout)
			options.Packages = pinned(g, private.Match)
		}
		if settings.Links {
			links := ""
			if cacheDirectory != "" {
				links = filepath.Join(cacheDirectory, "links")
			}
			options.Web = findings.NewWeb(links, linkCacheTTL, indexTimeout, credentials)
		}
	}
	set := findings.Collect(ctx, options)
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

// documents is the repository's own Markdown, which is where the links worth
// following are. The map has already read which files those are, so nothing is
// scanned twice to find them.
//
// Two kinds of Markdown are left out, because a finding against either would be a
// defect nobody is meant to fix: a vendored README links to the parts of its own
// repository that vendoring does not copy, and a fixture under testdata is wrong on
// purpose - a link that leads nowhere is what a link check is tested against.
//
// Implements: REQ-MD-015
func documents(g *graph.Graph) []string {
	var out []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindFile && n.Language == "Markdown" && !fixed(n.Path) {
			out = append(out, n.Path)
		}
	}
	return out
}

// notProse names the directories whose Markdown is not this repository's own writing.
//
// Implements: REQ-MD-015
var notProse = map[string]bool{
	"vendor": true, "node_modules": true, "third_party": true, "thirdparty": true,
	"site-packages": true, ".venv": true, "venv": true, "testdata": true,
}

// fixed reports whether a path lies under one of them, at any depth: a Go module's
// vendor/ is at the root, a workspace's node_modules and a package's testdata are not.
func fixed(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if notProse[segment] {
			return true
		}
	}
	return false
}

// pinned is every external package the map fixes to one version: the only ones a
// vulnerability database can answer about, since a floating range resolves to
// something else on the next install. A package fixed to a git commit on a public
// forge carries the commit too (lang.GitPin), which OSV answers for whatever the
// ecosystem.
//
// Implements: REQ-FND-013, REQ-SUP-040, REQ-FND-026
func pinned(g *graph.Graph, private func(ecosystem, name string) bool) []findings.Package {
	if private == nil {
		private = func(string, string) bool { return false }
	}
	var out []findings.Package
	for _, n := range g.Nodes {
		if n.Kind != graph.KindPackage || n.Version == "" || n.Floating {
			continue
		}
		ecosystem := n.Parent
		if i := strings.LastIndex(ecosystem, ":"); i >= 0 {
			ecosystem = ecosystem[i+1:]
		}
		p := findings.Package{Ecosystem: ecosystem, Name: n.Name, Version: n.Version}
		commit, repository, public := lang.GitPin(ecosystem, n.Name, n.Version, n.Origin, n.Git)
		// A version that is itself a git reference (npm's github:owner/repo#<sha>, a
		// Python "@ git+<url>@<sha>") is no release of the ecosystem's index: only
		// the commit is a question, and the reference names the repository.
		reference := commit != "" && !strings.EqualFold(n.Version, commit) &&
			strings.Contains(strings.ToLower(n.Version), commit)
		// The commit of a private repository is not sent: its repository is on a
		// public forge (a private host would be the disclosure), the package is not
		// private by name (--private, GOPRIVATE: it is only private for having been
		// installed from outside every index), and neither the package nor its
		// repository matches a private pattern.
		if commit != "" && public && !(n.Private && n.Origin == "") &&
			!private(ecosystem, n.Name) && (repository == "" || !private(ecosystem, repository)) {
			p.Commit, p.Repository = commit, repository
		}
		// An organization's own package is not asked about: the question hands the
		// name and version of internal code to somebody else's server.
		p.CommitOnly = n.Private || reference
		if p.CommitOnly && p.Commit == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Implements: REQ-EXP-004
func writeExport(g *graph.Graph, settings config.Config, extra map[string]any, format, output string) (err error) {
	var w io.Writer = os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		// Close is where a full disk or a network share reports that the write did
		// not happen; ignoring it would leave a truncated export and exit 0.
		defer func() {
			if cErr := f.Close(); err == nil {
				err = cErr
			}
		}()
		w = f
	}
	if format == "html" {
		return web.WriteStatic(w, g, settings.UI, settings.Root, extra)
	}
	return export.Write(w, g, format)
}

// Implements: REQ-SRV-001
func serve(ctx context.Context, settings config.Config, g *graph.Graph, options anal.Options, c *cache.Cache,
	cacheDirectory string, newReport func() *trace.Report, credentials *auth.Store, private *scope.Private) error {
	if settings.Editor == "" {
		settings.Editor = editor.Detect(os.Getenv, exec.LookPath)
	}
	mapServer, err := server.New(settings, g, web.Assets())
	if err != nil {
		return err
	}
	mapServer.SetResolution(options.Trace)
	line, url, err := mapServer.Listen(settings.Address)
	if err != nil {
		return err
	}
	httpServer := &http.Server{Handler: mapServer.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		mapServer.Close()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdown)
	}()

	// The map is served at once; history and references follow in the background and,
	// in watch mode, are refreshed after changes.
	// Implements: REQ-HIST-007, REQ-LSP-005, REQ-FND-022
	historyHead, historyRead := "", false
	historyRun := newLatest(func(g *graph.Graph) {
		head, _ := history.Head(ctx, settings.Root) // "" without commits
		if historyRead && head == historyHead {
			return
		}
		historyHead, historyRead = head, true
		mapServer.SetHistory(loadHistory(ctx, settings, cacheDirectory, g))
	})
	referencesRun := newLatest(func(g *graph.Graph) {
		mapServer.SetReferences(loadReferences(ctx, settings, cacheDirectory, g))
	})
	findingsRun := newLatest(func(g *graph.Graph) {
		mapServer.SetFindings(loadFindings(ctx, settings, cacheDirectory, g, credentials, private))
	})
	if settings.History {
		go historyRun.Run(g)
	}
	if settings.LSP {
		go referencesRun.Run(g)
	}
	if settings.FindingsEnabled() {
		go findingsRun.Run(g)
	}

	// Implements: REQ-WATCH-001, REQ-WATCH-003, REQ-WATCH-007, REQ-HIST-009, REQ-FND-023
	if settings.Watch {
		w, err := watch.New()
		if err != nil {
			return fmt.Errorf("watch: %w", err)
		}
		gitDirectories := history.GitDirectories(ctx, settings.Root) // commits change only these
		// A scanner writing its report again is news too: the backpack in the UI
		// marks a caught finding fixed when it stops being reported, and that only
		// works if the report is re-read when it is written.
		reportDirectories, reportFiles := findingWatch(settings)
		watched := func(g *graph.Graph) []string {
			return append(append(watchDirectories(settings.Root, g), gitDirectories...), reportDirectories...)
		}
		w.Sync(watched(g), reportFiles)
		go w.Run(ctx, 300*time.Millisecond, func() {
			start := time.Now()
			// Implements: REQ-TRC-016
			options.Trace = newReport()
			ng, stats, err := anal.Run(ctx, settings.Root, options)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("re-analysis failed: %v", err)
				}
				return
			}
			mapServer.SetResolution(stats.Resolution)
			if err := c.Save(); err != nil {
				log.Printf("cache not saved: %v", err)
			}
			w.Sync(watched(ng), reportFiles)
			changed, err := mapServer.Update(ng, stats.ParsedFiles)
			if err != nil {
				log.Printf("update failed: %v", err)
			} else if changed {
				log.Printf("updated: %d files re-parsed in %s", stats.Parsed, time.Since(start).Round(time.Millisecond))
				// Only when the map moved. A report after every saved file would
				// bury the one that belongs to the change being looked at.
				explain(settings, stats.Resolution)
			}
			if settings.History {
				go historyRun.Run(ng) // a commit moves HEAD
			}
			if settings.LSP && changed {
				go referencesRun.Run(ng)
			}
			if settings.FindingsEnabled() {
				// Not only when the graph changed: a linter complains about the text
				// of a file, and fixing one changes neither its imports nor its size.
				go findingsRun.Run(ng)
			}
		})
	}

	log.Printf("serving at %s (Ctrl+C to stop)", url)
	// Implements: REQ-CLI-001, REQ-DIST-016
	if settings.Open {
		if err := browser.OpenURL(url); err != nil {
			log.Printf("could not open browser: %v", err)
		}
	}
	if err := httpServer.Serve(line); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// findingWatch says what to watch for the scanner reports, so that rewriting one is a
// change the watcher sees: named reports as single files, and the directory of every
// pattern that is a glob.
//
// A report usually sits in a directory holding a great deal besides - often the
// repository root - and a watch on the directory fires for every file written into
// it. A glob has no one file to watch, and a new file matching it is news, so there
// the whole directory stays watched.
//
// Implements: REQ-FND-023
func findingWatch(settings config.Config) (directories, files []string) {
	globbed := map[string]bool{}
	for _, p := range settings.Findings {
		if !strings.ContainsAny(p, "*?[") {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(settings.Root, p)
		}
		d := filepath.Dir(p)
		if !globbed[d] {
			globbed[d] = true
			directories = append(directories, d)
		}
	}
	for _, f := range findings.Files(settings.Root, settings.Findings) {
		if !globbed[filepath.Dir(f)] {
			files = append(files, f)
		}
	}
	return directories, files
}

// watchDirectories lists the directories holding analyzed files: ignored trees are not watched.
//
// Implements: REQ-WATCH-001
func watchDirectories(root string, g *graph.Graph) []string {
	var directories []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindDirectory {
			directories = append(directories, filepath.Join(root, filepath.FromSlash(n.Path)))
		}
	}
	return directories
}

// latest runs function one call at a time with the newest value it was given: values that
// arrive during a run are coalesced into a single follow-up run.
type latest[T any] struct {
	function func(T)
	mu       sync.Mutex
	running  bool
	next     *T
}

func newLatest[T any](function func(T)) *latest[T] { return &latest[T]{function: function} }

func (l *latest[T]) Run(v T) {
	l.mu.Lock()
	l.next = &v
	if l.running {
		l.mu.Unlock()
		return // the running loop picks v up
	}
	l.running = true
	for l.next != nil {
		v := *l.next
		l.next = nil
		l.mu.Unlock()
		l.function(v)
		l.mu.Lock()
	}
	l.running = false
	l.mu.Unlock()
}
