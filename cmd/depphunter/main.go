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
	"github.com/sarumaj/depphunter-cli/internal/lang/ci"
	"github.com/sarumaj/depphunter-cli/internal/lang/csharp"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
	"github.com/sarumaj/depphunter-cli/internal/lang/markdown"
	"github.com/sarumaj/depphunter-cli/internal/lang/powershell"
	"github.com/sarumaj/depphunter-cli/internal/lang/python"
	"github.com/sarumaj/depphunter-cli/internal/lang/rust"
	"github.com/sarumaj/depphunter-cli/internal/lsp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/scope"
	"github.com/sarumaj/depphunter-cli/internal/server"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/watch"
	"github.com/sarumaj/depphunter-cli/web"
)

// How long an index's answer stays usable, and how long one request may take.
const (
	indexCacheTTL = 24 * time.Hour
	indexTimeout  = 30 * time.Second
	// Advisories are published against versions that are already released, so what the
	// database said yesterday is almost always still true - but not for long enough to
	// keep for a week.
	findingsCacheTTL = 6 * time.Hour
)

// version is set at release builds: -ldflags "-X main.version=v1.2.3".
var version = "dev"

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
func logOutput(cfg config.Config) io.Writer {
	if cfg.Export != "" && cfg.Output == "" {
		return os.Stderr
	}
	return os.Stdout
}

// newCommand is the depphunter command: flags are declared by the config package,
// which layers them over the config files and environment (viper).
func newCommand() *cobra.Command {
	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			userDir := ""
			if d, err := os.UserConfigDir(); err == nil {
				userDir = filepath.Join(d, "depphunter")
			}
			cfg, err := config.Load(cmd.Flags(), args, userDir)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cfg)
		},
	}
	cmd.SetVersionTemplate("depphunter {{.Version}}\n")
	cmd.Flags().SortFlags = false
	config.RegisterFlags(cmd.Flags())
	return cmd
}

func run(ctx context.Context, cfg config.Config) error {
	log.SetOutput(logOutput(cfg))
	var c *cache.Cache
	cacheDir := "" // also holds git histories; "" disables caching
	if cfg.Cache {
		if d, err := os.UserCacheDir(); err == nil {
			cacheDir = filepath.Join(d, "depphunter")
			c = cache.Open(cacheDir, cfg.Root)
		}
	}
	opts := anal.Options{
		Scan: scan.Options{Exclude: cfg.Exclude, MaxFileSize: cfg.MaxFileSize},
		Plugins: []lang.Plugin{
			golang.Plugin{}, javascript.Plugin{}, python.Plugin{}, rust.Plugin{}, java.Plugin{},
			csharp.Plugin{}, powershell.Plugin{}, ci.Plugin{}, markdown.Plugin{},
		},
		Cache:        c,
		ResolveDepth: cfg.ResolveDepth,
	}
	home, _ := os.UserHomeDir()
	// What this machine already holds for its registries and indexes. Read once: the
	// index client, the container registries and the link check all send from it, and
	// each credential goes only to the host it was written for.
	credentials := auth.Read(home, os.Getenv)
	// What this organization owns: what was declared, plus what the machine already
	// says about private Go modules (internal/scope).
	private := scope.New(append(append([]string{}, cfg.Private...), scope.FromGoEnv(os.Getenv)...))
	if !private.Empty() {
		log.Printf("private: %s", strings.Join(private.Patterns(), ", "))
	}
	opts.Private = private.Match
	indexes := index.NewDiscoverer(os.Getenv, home)
	indexes.Config().Credentials(credentials)
	indexes.Config().Trust(cfg.TrustIndexes)
	opts.Indexes = func(files []*scan.File) anal.Indexes { return indexes.Discover(files) }
	if cfg.Online {
		// The configuration is filled while the scan runs; the client only reads it
		// afterwards, when the walk starts asking about packages. Without a cache
		// directory (--no-cache) the answers are kept for this run only, rather than
		// written to a relative path inside the analyzed project.
		store := ""
		if cacheDir != "" {
			store = filepath.Join(cacheDir, "index")
		}
		opts.Registry = index.NewClient(indexes.Config(), store, indexCacheTTL, indexTimeout, credentials, private)
	}
	// One report per analysis: --watch analyzes again on every change, and a report
	// that accumulated over a morning's editing describes no run in particular.
	newReport := func() *trace.Report {
		return trace.New(cfg.ResolveDepth, opts.Registry != nil, private.Patterns(), cfg.TrustIndexes)
	}
	opts.Trace = newReport()
	g, err := analyze(ctx, cfg, opts, c)
	if err != nil {
		return err
	}

	if cfg.Export != "" {
		extra := map[string]any{}
		refs := loadReferences(ctx, cfg, cacheDir, g)
		if cfg.Export == "html" {
			if h := loadHistory(ctx, cfg, cacheDir, g); h != nil {
				extra["history"] = h
			}
			if refs != nil {
				extra["references"] = refs
			}
			if f := loadFindings(ctx, cfg, cacheDir, g, credentials); !f.Empty() {
				extra["findings"] = f
			}
		} else if refs != nil {
			g = export.WithEdges(g, refs.Edges)
		}
		return writeExport(g, cfg, extra, cfg.Export, cfg.Output)
	}
	return serve(ctx, cfg, g, opts, c, cacheDir, newReport, credentials)
}

// explain writes the resolution report when --explain asked for it, wherever the log
// goes (logOutput) - which is where the VS Code extension reads it from too.
func explain(cfg config.Config, r *trace.Report) {
	if !cfg.Explain || r == nil {
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
func loadHistory(ctx context.Context, cfg config.Config, cacheDir string, g *graph.Graph) *history.History {
	if !cfg.History {
		return nil
	}
	start := time.Now()
	h, err := history.Cached(ctx, cacheDir, cfg.Root, cfg.HistoryCommits)
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

func analyze(ctx context.Context, cfg config.Config, opts anal.Options, c *cache.Cache) (*graph.Graph, error) {
	start := time.Now()
	g, st, err := anal.Run(ctx, cfg.Root, opts)
	if err != nil {
		return nil, err
	}
	log.Printf("analyzed %s: %d files (%d parsed, %d cached), %d nodes, %d edges in %s",
		cfg.Root, st.Files, st.Parsed, st.Cached, len(g.Nodes), len(g.Edges), time.Since(start).Round(time.Millisecond))
	explain(cfg, st.Resolution)
	if err := c.Save(); err != nil {
		log.Printf("cache not saved: %v", err)
	}
	return g, nil
}

// loadReferences asks the installed language servers for symbol references; nil when
// disabled or when no server could answer.
func loadReferences(ctx context.Context, cfg config.Config, cacheDir string, g *graph.Graph) *server.References {
	if !cfg.LSP {
		return nil
	}
	start := time.Now()
	r, err := lsp.Cached(ctx, cacheDir, g, lsp.Options{Root: cfg.Root, Timeout: cfg.LSPTimeout, Logf: log.Printf})
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
func loadFindings(ctx context.Context, cfg config.Config, cacheDir string, g *graph.Graph,
	credentials *auth.Store) *findings.Set {
	if !cfg.FindingsEnabled() {
		return nil
	}
	start := time.Now()
	// --no-vulns and --no-links turn off one source each, so each is asked for on
	// its own: a run that wants only the link check reads no reports.
	opts := findings.Options{Root: cfg.Root, Logf: log.Printf}
	if cfg.Vulns {
		opts.Reports = cfg.Findings
	}
	if cfg.Links {
		opts.Docs = documents(g)
	}
	if cfg.Online {
		store := ""
		if cacheDir != "" {
			store = filepath.Join(cacheDir, "osv")
		}
		if cfg.Vulns {
			opts.OSV = findings.NewOSV(store, findingsCacheTTL, indexTimeout)
			opts.Packages = pinned(g)
		}
		if cfg.Links {
			links := ""
			if cacheDir != "" {
				links = filepath.Join(cacheDir, "links")
			}
			opts.Web = findings.NewWeb(links, findingsCacheTTL, indexTimeout, credentials)
		}
	}
	set := findings.Collect(ctx, opts)
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
func documents(g *graph.Graph) []string {
	var out []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindFile && n.Lang == "Markdown" && !fixed(n.Path) {
			out = append(out, n.Path)
		}
	}
	return out
}

// notProse names the directories whose Markdown is not this repository's own writing.
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
// something else on the next install.
func pinned(g *graph.Graph) []findings.Package {
	var out []findings.Package
	for _, n := range g.Nodes {
		if n.Kind != graph.KindPackage || n.Version == "" || n.Floating {
			continue
		}
		// An organization's own package is not asked about: the question hands the
		// name and version of internal code to somebody else's server.
		if n.Private {
			continue
		}
		eco := n.Parent
		if i := strings.LastIndex(eco, ":"); i >= 0 {
			eco = eco[i+1:]
		}
		out = append(out, findings.Package{Ecosystem: eco, Name: n.Name, Version: n.Version})
	}
	return out
}

func writeExport(g *graph.Graph, cfg config.Config, extra map[string]any, format, output string) (err error) {
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
		return web.WriteStatic(w, g, cfg.UI, cfg.Root, extra)
	}
	return export.Write(w, g, format)
}

func serve(ctx context.Context, cfg config.Config, g *graph.Graph, opts anal.Options, c *cache.Cache,
	cacheDir string, newReport func() *trace.Report, credentials *auth.Store) error {
	if cfg.Editor == "" {
		cfg.Editor = editor.Detect(os.Getenv, exec.LookPath)
	}
	srv, err := server.New(cfg, g, web.Assets())
	if err != nil {
		return err
	}
	srv.SetResolution(opts.Trace)
	ln, url, err := srv.Listen(cfg.Addr)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		srv.Close()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdown)
	}()

	// The map is served at once; history and references follow in the background and,
	// in watch mode, are refreshed after changes.
	histHead, histRead := "", false
	historyRun := newLatest(func(g *graph.Graph) {
		head, _ := history.Head(ctx, cfg.Root) // "" without commits
		if histRead && head == histHead {
			return
		}
		histHead, histRead = head, true
		srv.SetHistory(loadHistory(ctx, cfg, cacheDir, g))
	})
	referencesRun := newLatest(func(g *graph.Graph) {
		srv.SetReferences(loadReferences(ctx, cfg, cacheDir, g))
	})
	findingsRun := newLatest(func(g *graph.Graph) {
		srv.SetFindings(loadFindings(ctx, cfg, cacheDir, g, credentials))
	})
	if cfg.History {
		go historyRun.Run(g)
	}
	if cfg.LSP {
		go referencesRun.Run(g)
	}
	if cfg.FindingsEnabled() {
		go findingsRun.Run(g)
	}

	if cfg.Watch {
		w, err := watch.New()
		if err != nil {
			return fmt.Errorf("watch: %w", err)
		}
		gitDirs := history.GitDirs(ctx, cfg.Root) // commits change only these
		// A scanner writing its report again is news too: the backpack in the UI
		// marks a caught finding fixed when it stops being reported, and that only
		// works if the report is re-read when it is written.
		reportDirs, reportFiles := findingWatch(cfg)
		watched := func(g *graph.Graph) []string {
			return append(append(watchDirs(cfg.Root, g), gitDirs...), reportDirs...)
		}
		w.Sync(watched(g), reportFiles)
		go w.Run(ctx, 300*time.Millisecond, func() {
			start := time.Now()
			opts.Trace = newReport()
			ng, st, err := anal.Run(ctx, cfg.Root, opts)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("re-analysis failed: %v", err)
				}
				return
			}
			srv.SetResolution(st.Resolution)
			if err := c.Save(); err != nil {
				log.Printf("cache not saved: %v", err)
			}
			w.Sync(watched(ng), reportFiles)
			changed, err := srv.Update(ng, st.ParsedFiles)
			if err != nil {
				log.Printf("update failed: %v", err)
			} else if changed {
				log.Printf("updated: %d files re-parsed in %s", st.Parsed, time.Since(start).Round(time.Millisecond))
				// Only when the map moved. A report after every saved file would
				// bury the one that belongs to the change being looked at.
				explain(cfg, st.Resolution)
			}
			if cfg.History {
				go historyRun.Run(ng) // a commit moves HEAD
			}
			if cfg.LSP && changed {
				go referencesRun.Run(ng)
			}
			if cfg.FindingsEnabled() {
				// Not only when the graph changed: a linter complains about the text
				// of a file, and fixing one changes neither its imports nor its size.
				go findingsRun.Run(ng)
			}
		})
	}

	log.Printf("serving at %s (Ctrl+C to stop)", url)
	if cfg.Open {
		if err := browser.OpenURL(url); err != nil {
			log.Printf("could not open browser: %v", err)
		}
	}
	if err := httpSrv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
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
func findingWatch(cfg config.Config) (dirs, files []string) {
	globbed := map[string]bool{}
	for _, p := range cfg.Findings {
		if !strings.ContainsAny(p, "*?[") {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(cfg.Root, p)
		}
		d := filepath.Dir(p)
		if !globbed[d] {
			globbed[d] = true
			dirs = append(dirs, d)
		}
	}
	for _, f := range findings.Files(cfg.Root, cfg.Findings) {
		if !globbed[filepath.Dir(f)] {
			files = append(files, f)
		}
	}
	return dirs, files
}

// watchDirs lists the directories holding analyzed files: ignored trees are not watched.
func watchDirs(root string, g *graph.Graph) []string {
	var dirs []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindDir {
			dirs = append(dirs, filepath.Join(root, filepath.FromSlash(n.Path)))
		}
	}
	return dirs
}

// latest runs fn one call at a time with the newest value it was given: values that
// arrive during a run are coalesced into a single follow-up run.
type latest[T any] struct {
	fn      func(T)
	mu      sync.Mutex
	running bool
	next    *T
}

func newLatest[T any](fn func(T)) *latest[T] { return &latest[T]{fn: fn} }

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
		l.fn(v)
		l.mu.Lock()
	}
	l.running = false
	l.mu.Unlock()
}
