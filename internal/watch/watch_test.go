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
	w.Sync([]string{dir})

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
	w.Sync(nil)
	os.WriteFile(filepath.Join(dir, "b.go"), nil, 0o644)
	time.Sleep(300 * time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Fatalf("change in unwatched dir triggered onChange (%d calls)", n)
	}
}
