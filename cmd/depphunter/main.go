// Command depphunter opens an interactive map of the code base in the current directory.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/analyze"
	"github.com/sarumaj/depphunter-cli/internal/browser"
	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/server"
	"github.com/sarumaj/depphunter-cli/web"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, config.ErrHelp) {
			fmt.Fprintln(os.Stderr, "depphunter:", err)
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

	start := time.Now()
	g, err := analyze.Run(ctx, cfg.Root, analyze.Options{
		Scan:    scan.Options{Exclude: cfg.Exclude, MaxFileSize: cfg.MaxFileSize},
		Plugins: []lang.Plugin{golang.Plugin{}},
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Analysed %s: %d nodes, %d edges in %s\n", cfg.Root, len(g.Nodes), len(g.Edges), time.Since(start).Round(time.Millisecond))

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
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdown)
	}()

	fmt.Fprintf(os.Stderr, "Serving at %s (Ctrl+C to stop)\n", url)
	if cfg.Open {
		if err := browser.Open(url); err != nil {
			fmt.Fprintln(os.Stderr, "could not open browser:", err)
		}
	}
	if err := httpSrv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
