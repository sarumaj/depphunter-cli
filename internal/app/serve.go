package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cli/browser"

	"github.com/sarumaj/depphunter-cli/internal/analyze"
	"github.com/sarumaj/depphunter-cli/internal/editor"
	"github.com/sarumaj/depphunter-cli/internal/findings"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/history"
	"github.com/sarumaj/depphunter-cli/internal/server"
	"github.com/sarumaj/depphunter-cli/internal/watch"
	"github.com/sarumaj/depphunter-cli/web"
)

// listening is the map's server once it holds its listener: what serve still needs
// to log the address and to answer on it.
type listening struct {
	mapServer  *server.Server
	httpServer *http.Server
	listener   net.Listener
	url        string
}

// serve serves the map of g until ctx is canceled: the server first, then the
// background loaders, then (--watch) the watcher, and only then the log line with the
// address and the browser.
//
// Implements: REQ-SRV-001
func (a *app) serve(ctx context.Context, g *graph.Graph) error {
	if a.settings.Editor == "" {
		a.settings.Editor = editor.Detect(os.Getenv, exec.LookPath)
	}
	s, err := a.startServer(ctx, g)
	if err != nil {
		return err
	}
	l := a.startLoaders(ctx, s.mapServer, g)
	if a.settings.Watch {
		if err := a.startWatch(ctx, s.mapServer, g, l); err != nil {
			return err
		}
	}

	log.Printf("serving at %s (Ctrl+C to stop)", s.url)
	// Implements: REQ-CLI-001, REQ-DIST-016
	if a.settings.Open {
		if err := browser.OpenURL(s.url); err != nil {
			log.Printf("could not open browser: %v", err)
		}
	}
	if err := s.httpServer.Serve(s.listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// startServer builds the map's server over g and the first resolution report, and
// binds its listener. The server shuts down when ctx is canceled.
func (a *app) startServer(ctx context.Context, g *graph.Graph) (*listening, error) {
	mapServer, err := server.New(a.settings, g, web.Assets())
	if err != nil {
		return nil, err
	}
	mapServer.SetResolution(a.options.Trace)
	listener, url, err := mapServer.Listen(a.settings.Address)
	if err != nil {
		return nil, err
	}
	httpServer := &http.Server{Handler: mapServer.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		mapServer.Close()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdown)
	}()
	return &listening{mapServer: mapServer, httpServer: httpServer, listener: listener, url: url}, nil
}

// startWatch watches the analyzed directories, the git directories and the scanner
// reports, and analyzes again after every change.
//
// Implements: REQ-WATCH-001, REQ-WATCH-003, REQ-WATCH-007, REQ-HIST-009, REQ-FND-023
func (a *app) startWatch(ctx context.Context, mapServer *server.Server, g *graph.Graph, l *loaders) error {
	w, err := watch.New()
	if err != nil {
		return fmt.Errorf("watch: %w", err)
	}
	gitDirectories := history.GitDirectories(ctx, a.settings.Root) // commits change only these
	// A scanner writing its report again is news too: the backpack in the UI marks a
	// caught finding fixed when it stops being reported, and that only works if the
	// report is re-read when it is written.
	reportDirectories, reportFiles := findings.Watched(a.settings.Root, a.settings.Findings)
	resync := func(g *graph.Graph) {
		w.Sync(append(append(watchDirectories(a.settings.Root, g), gitDirectories...), reportDirectories...), reportFiles)
	}
	resync(g)
	go w.Run(ctx, 300*time.Millisecond, func() { a.reanalyze(ctx, mapServer, resync, l) })
	return nil
}

// reanalyze runs one analysis after a change, with a resolution report of its own,
// re-syncs the watched set, and updates the map and the loaders.
//
// Implements: REQ-TRC-016
func (a *app) reanalyze(ctx context.Context, mapServer *server.Server, resync func(*graph.Graph), l *loaders) {
	start := time.Now()
	a.options.Trace = a.newReport()
	g, stats, err := analyze.Run(ctx, a.settings.Root, a.options)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("re-analysis failed: %v", err)
		}
		return
	}
	mapServer.SetResolution(stats.Resolution)
	if err := a.cache.Save(); err != nil {
		log.Printf("cache not saved: %v", err)
	}
	resync(g)
	changed, err := mapServer.Update(g, stats.ParsedFiles)
	if err != nil {
		log.Printf("update failed: %v", err)
	} else if changed {
		log.Printf("updated: %d files re-parsed in %s", stats.Parsed, time.Since(start).Round(time.Millisecond))
		// Only when the map moved. A report after every saved file would bury the one
		// that belongs to the change being looked at.
		explain(a.settings, stats.Resolution)
	}
	a.runLoaders(l, g, changed)
}

// watchDirectories lists the directories holding analyzed files: ignored trees are not watched.
//
// Implements: REQ-WATCH-001
func watchDirectories(root string, g *graph.Graph) []string {
	var directories []string
	for n := range g.Of(graph.KindDirectory) {
		directories = append(directories, filepath.Join(root, filepath.FromSlash(n.Path)))
	}
	return directories
}
