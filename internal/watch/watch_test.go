package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncedChanges(t *testing.T) {
	dir := t.TempDir()
	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	w.Sync([]string{dir}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	go w.Run(ctx, 100*time.Millisecond, func() { calls.Add(1) })

	// A burst of writes is one change.
	for i := range 5 {
		os.WriteFile(filepath.Join(dir, "a.go"), []byte{byte(i)}, 0o644)
		time.Sleep(10 * time.Millisecond)
	}
	deadline := time.Now().Add(3 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Fatalf("onChange called %d times, want 1", n)
	}

	// Unwatched directories are silent.
	w.Sync(nil, nil)
	os.WriteFile(filepath.Join(dir, "b.go"), nil, 0o644)
	time.Sleep(300 * time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Fatalf("change in unwatched dir triggered onChange (%d calls)", n)
	}
}

// A report a scanner writes shares its directory with whatever else is written there,
// and fsnotify watches directories; only the report itself is a change.
func TestOnlyTheNamedFilesInTheirDirectories(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	w, err := New()
	if err != nil {
		t.Fatal(err)
	}
	w.Sync(nil, []string{report})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	go w.Run(ctx, 100*time.Millisecond, func() { calls.Add(1) })

	os.WriteFile(filepath.Join(dir, "screenshot.png"), []byte("x"), 0o644)
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
