// Package watch reports file-system changes in the analysed directories.
//
// Only directories that contain analysed files are watched, so ignored trees
// (node_modules, build output, .git) cost nothing. The set is re-synced after every
// analysis; a new directory is picked up because its creation is an event in its
// (watched) parent, which triggers the re-analysis that lists its files.
package watch

import (
	"context"
	"log"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	w       *fsnotify.Watcher
	watched map[string]bool
	warned  bool
}

func New() (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{w: w, watched: map[string]bool{}}, nil
}

// Sync makes the watched set equal to dirs (absolute paths).
func (w *Watcher) Sync(dirs []string) {
	want := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		want[d] = true
		if w.watched[d] {
			continue
		}
		if err := w.w.Add(d); err != nil {
			// Usually the inotify watch limit; say so once and keep going.
			if !w.warned {
				log.Printf("watch: %s: %v (further errors suppressed)", d, err)
				w.warned = true
			}
			continue
		}
		w.watched[d] = true
	}
	for d := range w.watched {
		if !want[d] {
			w.w.Remove(d)
			delete(w.watched, d)
		}
	}
}

// Run calls onChange once changes have been quiet for debounce, until ctx ends.
// Changes that arrive while onChange runs trigger another call afterwards.
func (w *Watcher) Run(ctx context.Context, debounce time.Duration, onChange func()) {
	defer w.w.Close()
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.w.Events:
			if !ok {
				return
			}
			if ev.Op == fsnotify.Chmod {
				continue // metadata only; contents did not change
			}
			timer.Reset(debounce)
		case err, ok := <-w.w.Errors:
			if !ok {
				return
			}
			log.Printf("watch: %v", err)
		case <-timer.C:
			onChange()
		}
	}
}
