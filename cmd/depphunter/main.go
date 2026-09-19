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

	"github.com/sarumaj/depphunter-cli/internal/analyze"
	"github.com/sarumaj/depphunter-cli/internal/browser"
	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/editor"
	"github.com/sarumaj/depphunter-cli/internal/export"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/history"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/csharp"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
	"github.com/sarumaj/depphunter-cli/internal/lang/powershell"
	"github.com/sarumaj/depphunter-cli/internal/lang/python"
	"github.com/sarumaj/depphunter-cli/internal/lang/rust"
	"github.com/sarumaj/depphunter-cli/internal/lsp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/server"
	"github.com/sarumaj/depphunter-cli/internal/watch"
	"github.com/sarumaj/depphunter-cli/web"
)

// version is set at release builds: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	log.SetFlags(0)
	log.SetPrefix("depphunter: ")
	switch err := run(); {
	case err == nil, errors.Is(err, config.ErrHelp):
	case errors.Is(err, config.ErrVersion):
		fmt.Println("depphunter", version)
	default:
		log.Println(err)
		os.Exit(1)
	}
}

func run() error {
	userDir := ""
	if d, err := os.UserConfigDir(); err == nil {
		userDir = filepath.Join(d, "depphunter")
	}
	cfg, err := config.Load(os.Args[1:], os.Getenv, userDir, os.Stderr)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var c *cache.Cache
	cacheDir := "" // also holds git histories; "" disables caching
	if cfg.Cache {
		if d, err := os.UserCacheDir(); err == nil {
			cacheDir = filepath.Join(d, "depphunter")
			c = cache.Open(cacheDir, cfg.Root)
		}
	}
	opts := analyze.Options{
		Scan: scan.Options{Exclude: cfg.Exclude, MaxFileSize: cfg.MaxFileSize},
		Plugins: []lang.Plugin{
			golang.Plugin{}, javascript.Plugin{}, python.Plugin{}, rust.Plugin{}, java.Plugin{},
			csharp.Plugin{}, powershell.Plugin{},
		},
		Cache: c,
	}
	g, err := analyse(ctx, cfg.Root, opts, c)
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
		} else if refs != nil {
			g = export.WithEdges(g, refs.Edges)
		}
		return writeExport(g, cfg, extra, cfg.Export, cfg.Output)
	}
	return serve(ctx, cfg, g, opts, c, cacheDir)
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

func analyse(ctx context.Context, root string, opts analyze.Options, c *cache.Cache) (*graph.Graph, error) {
	start := time.Now()
	g, st, err := analyze.Run(ctx, root, opts)
	if err != nil {
		return nil, err
	}
	log.Printf("analysed %s: %d files (%d parsed, %d cached), %d nodes, %d edges in %s",
		root, st.Files, st.Parsed, st.Cached, len(g.Nodes), len(g.Edges), time.Since(start).Round(time.Millisecond))
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

func writeExport(g *graph.Graph, cfg config.Config, extra map[string]any, format, output string) error {
	var w io.Writer = os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	if format == "html" {
		return web.WriteStatic(w, g, cfg.UI, cfg.Root, extra)
	}
	return export.Write(w, g, format)
}

func serve(ctx context.Context, cfg config.Config, g *graph.Graph, opts analyze.Options, c *cache.Cache, cacheDir string) error {
	if cfg.Editor == "" {
		cfg.Editor = editor.Detect(os.Getenv, exec.LookPath)
	}
	srv, err := server.New(cfg, g, web.Assets())
	if err != nil {
		return err
	}
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

	// The map is served at once; the history follows when git has been read. histMu
	// serialises reads so a watch-mode refresh cannot overtake the first one.
	var histMu sync.Mutex
	histHead, histRead := "", false
	refreshHistory := func(g *graph.Graph) {
		histMu.Lock()
		defer histMu.Unlock()
		head, _ := history.Head(ctx, cfg.Root) // "" without commits
		if histRead && head == histHead {
			return
		}
		histHead, histRead = head, true
		srv.SetHistory(loadHistory(ctx, cfg, cacheDir, g))
	}
	if cfg.History {
		go refreshHistory(g)
	}

	// References follow the history; in watch mode they are recomputed after changes,
	// one run at a time, the latest graph winning.
	var refsMu sync.Mutex
	var refsNext *graph.Graph
	refreshReferences := func(g *graph.Graph) {
		refsMu.Lock()
		running := refsNext != nil
		refsNext = g
		refsMu.Unlock()
		if running {
			return // the running loop picks up the newest graph
		}
		for {
			refsMu.Lock()
			g := refsNext
			refsMu.Unlock()
			srv.SetReferences(loadReferences(ctx, cfg, cacheDir, g))
			refsMu.Lock()
			if refsNext == g {
				refsNext = nil
				refsMu.Unlock()
				return
			}
			refsMu.Unlock()
		}
	}
	if cfg.LSP {
		go refreshReferences(g)
	}

	if cfg.Watch {
		w, err := watch.New()
		if err != nil {
			return fmt.Errorf("watch: %w", err)
		}
		gitDirs := history.GitDirs(ctx, cfg.Root) // commits change only these
		w.Sync(append(watchDirs(cfg.Root, g), gitDirs...))
		go w.Run(ctx, 300*time.Millisecond, func() {
			start := time.Now()
			ng, st, err := analyze.Run(ctx, cfg.Root, opts)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("re-analysis failed: %v", err)
				}
				return
			}
			c.Save()
			w.Sync(append(watchDirs(cfg.Root, ng), gitDirs...))
			changed, err := srv.Update(ng, st.ParsedFiles)
			if err != nil {
				log.Printf("update failed: %v", err)
			} else if changed {
				log.Printf("updated: %d files re-parsed in %s", st.Parsed, time.Since(start).Round(time.Millisecond))
			}
			if cfg.History {
				refreshHistory(ng) // a commit moved HEAD
			}
			if cfg.LSP && changed {
				go refreshReferences(ng)
			}
		})
	}

	log.Printf("serving at %s (Ctrl+C to stop)", url)
	if cfg.Open {
		if err := browser.Open(url); err != nil {
			log.Printf("could not open browser: %v", err)
		}
	}
	if err := httpSrv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// watchDirs lists the directories holding analysed files: ignored trees are not watched.
func watchDirs(root string, g *graph.Graph) []string {
	var dirs []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindDir {
			dirs = append(dirs, filepath.Join(root, filepath.FromSlash(n.Path)))
		}
	}
	return dirs
}
