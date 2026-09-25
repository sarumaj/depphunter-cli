// Package watch reports file-system changes in the analyzed directories.
//
// Only directories that contain analyzed files are watched, so ignored trees
// (node_modules, build output, .git) cost nothing. The set is re-synced after every
// analysis; a new directory is picked up because its creation is an event in its
// (watched) parent, which triggers the re-analysis that lists its files.
package watch

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	w       *fsnotify.Watcher
	watched map[string]bool
	dirs    map[string]bool // directories in which every change counts
	files   map[string]bool // single files that count in the directories holding them
	warned  bool
}

func New() (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		w:       w,
		watched: map[string]bool{},
		dirs:    map[string]bool{},
		files:   map[string]bool{},
	}, nil
}

// Sync makes the watched set equal to dirs, plus the directories holding files (all
// absolute paths). A change anywhere in dirs is a change; in a directory watched only
// because it holds one of files, only those files are.
//
// Implements: REQ-WATCH-001
func (w *Watcher) Sync(dirs, files []string) {
	want := make(map[string]bool, len(dirs)+len(files))
	w.dirs = make(map[string]bool, len(dirs))
	w.files = make(map[string]bool, len(files))
	for _, f := range files {
		w.files[f] = true
		want[filepath.Dir(f)] = true
	}
	for _, d := range dirs {
		w.dirs[d] = true
		want[d] = true
	}
	for d := range want {
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

// counts reports whether an event is one of the changes that were asked for. A watch
// is on a whole directory even when only one file in it was asked for, so a scanner's
// report is watched together with everything else written beside it - and a directory
// a tool writes a report into usually holds a good deal else. Re-analyzing the
// repository for those is work nobody asked for.
func (w *Watcher) counts(name string) bool {
	if scratch(filepath.Base(name)) {
		return false
	}
	return w.dirs[filepath.Dir(name)] || w.files[name]
}

// scratch reports whether a file name is one an editor writes beside the file being
// edited and removes again - a swap file, a backup, a lock, vim's write test - which
// changes nothing that could be on the map.
func scratch(base string) bool {
	switch {
	case base == "4913", strings.HasSuffix(base, "~"), strings.HasPrefix(base, ".#"):
		return true
	case strings.HasPrefix(base, "#") && strings.HasSuffix(base, "#"):
		return true
	}
	switch filepath.Ext(base) {
	case ".swp", ".swo", ".swx", ".tmp":
		return true
	}
	return false
}

// maxWait bounds, in debounces, how long a stream of changes can defer onChange.
const maxWait = 10

// Run calls onChange once changes have been quiet for debounce, until ctx ends.
// Changes that arrive while onChange runs trigger another call afterwards.
//
// Changes that never go quiet - a build writing into a watched directory, a log
// growing in one - do not put the call off indefinitely: it comes at the latest
// maxWait debounces after the first of them.
//
// Implements: REQ-WATCH-002
func (w *Watcher) Run(ctx context.Context, debounce time.Duration, onChange func()) {
	defer w.w.Close()
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	var first time.Time // of the changes not yet handed on; zero when there are none
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
			if !w.counts(ev.Name) {
				continue
			}
			if first.IsZero() {
				first = time.Now()
			}
			wait := debounce
			if left := time.Until(first.Add(maxWait * debounce)); left < wait {
				wait = max(left, 0)
			}
			timer.Reset(wait)
		case err, ok := <-w.w.Errors:
			if !ok {
				return
			}
			log.Printf("watch: %v", err)
		case <-timer.C:
			first = time.Time{}
			onChange()
		}
	}
}
