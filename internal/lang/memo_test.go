package lang

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestMemoRemembersPerKey checks that Get computes a missing key and then
// answers from memory, and that LoadOrStore hands every goroutine the value
// stored first, as the resolvers that fill a sync.Once in it rely on.
func TestMemoRemembersPerKey(t *testing.T) {
	var memo Memo[string, int]
	calls := 0
	compute := func(key string) int { calls++; return len(key) }
	if got := memo.Get("abc", compute); got != 3 {
		t.Errorf("Get(abc) = %d, want 3", got)
	}
	if got := memo.Get("abc", compute); got != 3 || calls != 1 {
		t.Errorf("Get(abc) again = %d after %d computations, want 3 after 1", got, calls)
	}
	if _, ok := memo.Load("missing"); ok {
		t.Error("Load(missing) found a value")
	}

	var shared Memo[string, *int]
	values := make([]*int, 16)
	var group sync.WaitGroup
	for i := range values {
		group.Go(func() { values[i] = shared.LoadOrStore("key", new(int)) })
	}
	group.Wait()
	for i, v := range values {
		if v != values[0] {
			t.Fatalf("LoadOrStore gave goroutine %d another value than goroutine 0", i)
		}
	}
}

// TestMarkerMemoDoesNotFollowALink checks that a marker counts when present,
// even as a symbolic link to nothing, and that the answer is remembered.
func TestMarkerMemoDoesNotFollowALink(t *testing.T) {
	with, without, linked := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(with, "marker.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	hasMarker := MarkerMemo("marker.toml")
	if !hasMarker(with) || hasMarker(without) {
		t.Errorf("hasMarker(with, without) = %v, %v, want true, false", hasMarker(with), hasMarker(without))
	}
	if err := os.Symlink(filepath.Join(linked, "nowhere"), filepath.Join(linked, "marker.toml")); err == nil && !hasMarker(linked) {
		t.Error("a marker committed as a dangling link does not count")
	}
	if err := os.Remove(filepath.Join(with, "marker.toml")); err != nil {
		t.Fatal(err)
	}
	if !hasMarker(with) {
		t.Error("the answer for a directory was not remembered")
	}
}
