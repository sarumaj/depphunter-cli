package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Verifies: REQ-WATCH-002
func TestDebouncedChanges(t *testing.T) {
	directory := t.TempDir()
	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	w.Sync([]string{directory}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	// The writes are 10 ms apart, so each is its own event, and the debounce is far
	// longer than that: only a stall of most of a debounce between two of them - not
	// a slow runner's usual hiccup - could split the burst in two.
	const debounce = 300 * time.Millisecond
	go w.Run(ctx, debounce, func() { calls.Add(1) })

	// A burst of writes is one change.
	for i := range 5 {
		os.WriteFile(filepath.Join(directory, "a.go"), []byte{byte(i)}, 0o644)
		time.Sleep(10 * time.Millisecond)
	}
	deadline := time.Now().Add(3 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(3 * debounce)
	if n := calls.Load(); n != 1 {
		t.Fatalf("onChange called %d times, want 1", n)
	}

	// Unwatched directories are silent.
	w.Sync(nil, nil)
	os.WriteFile(filepath.Join(directory, "b.go"), nil, 0o644)
	time.Sleep(3 * debounce)
	if n := calls.Load(); n != 1 {
		t.Fatalf("change in unwatched dir triggered onChange (%d calls)", n)
	}
}

// A report a scanner writes shares its directory with whatever else is written there,
// and fsnotify watches directories; only the report itself is a change.
//
// Verifies: REQ-WATCH-001, REQ-FND-023
func TestOnlyTheNamedFilesInTheirDirectories(t *testing.T) {
	directory := t.TempDir()
	report := filepath.Join(directory, "report.json")
	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	w.Sync(nil, []string{report})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	go w.Run(ctx, 100*time.Millisecond, func() { calls.Add(1) })

	os.WriteFile(filepath.Join(directory, "screenshot.png"), []byte("x"), 0o644)
	time.Sleep(400 * time.Millisecond)
	if n := calls.Load(); n != 0 {
		t.Fatalf("a file beside the report triggered onChange (%d calls)", n)
	}

	os.WriteFile(report, []byte("{}"), 0o644)
	deadline := time.Now().Add(3 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("writing the report called onChange %d times, want 1", n)
	}
}

// Writes that never pause - a build, a growing log - still get the map updated.
//
// Verifies: REQ-WATCH-002
func TestChangesThatNeverGoQuiet(t *testing.T) {
	directory := t.TempDir()
	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	w.Sync([]string{directory}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	debounce := 50 * time.Millisecond
	go w.Run(ctx, debounce, func() { calls.Add(1) })

	// A write every 20ms for well past maxWait debounces: never quiet for one.
	stop := time.Now().Add(3 * maxWait * debounce)
	for i := 0; time.Now().Before(stop); i++ {
		os.WriteFile(filepath.Join(directory, "a.go"), []byte{byte(i)}, 0o644)
		time.Sleep(20 * time.Millisecond)
	}
	if calls.Load() == 0 {
		t.Error("a steady stream of changes put the update off for good")
	}
}

func TestEditorScratchFilesAreNotChanges(t *testing.T) {
	for _, name := range []string{".main.go.swp", "main.go~", "4913", ".#main.go", "#main.go#", "x.tmp"} {
		if !scratch(name) {
			t.Errorf("%s counts as a change", name)
		}
	}
	for _, name := range []string{"main.go", "go.mod", "report.json"} {
		if scratch(name) {
			t.Errorf("%s is ignored", name)
		}
	}
}
