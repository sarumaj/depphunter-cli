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
	"syscall"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/analyze"
	"github.com/sarumaj/depphunter-cli/internal/browser"
	"github.com/sarumaj/depphunter-cli/internal/cache"
	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/editor"
	"github.com/sarumaj/depphunter-cli/internal/export"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
	"github.com/sarumaj/depphunter-cli/internal/lang/python"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/server"
	"github.com/sarumaj/depphunter-cli/internal/watch"
	"github.com/sarumaj/depphunter-cli/web"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("depphunter: ")
	if err := run(); err != nil {
		if !errors.Is(err, config.ErrHelp) {
			log.Println(err)
			os.Exit(1)
		}
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
	if cfg.Cache {
		if d, err := os.UserCacheDir(); err == nil {
			c = cache.Open(filepath.Join(d, "depphunter"), cfg.Root)
		}
	}
	opts := analyze.Options{
		Scan:    scan.Options{Exclude: cfg.Exclude, MaxFileSize: cfg.MaxFileSize},
		Plugins: []lang.Plugin{golang.Plugin{}, javascript.Plugin{}, python.Plugin{}},
		Cache:   c,
	}
	g, err := analyse(ctx, cfg.Root, opts, c)
	if err != nil {
		return err
	}

	if cfg.Export != "" {
		return writeExport(g, cfg.Export, cfg.Output)
	}
	return serve(ctx, cfg, g, opts, c)
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

func writeExport(g *graph.Graph, format, output string) error {
	var w io.Writer = os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	return export.Write(w, g, format)
}

func serve(ctx context.Context, cfg config.Config, g *graph.Graph, opts analyze.Options, c *cache.Cache) error {
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

	if cfg.Watch {
		w, err := watch.New()
		if err != nil {
			return fmt.Errorf("watch: %w", err)
		}
		w.Sync(watchDirs(cfg.Root, g))
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
			w.Sync(watchDirs(cfg.Root, ng))
			if changed, err := srv.Update(ng, st.ParsedFiles); err != nil {
				log.Printf("update failed: %v", err)
			} else if changed {
				log.Printf("updated: %d files re-parsed in %s", st.Parsed, time.Since(start).Round(time.Millisecond))
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
