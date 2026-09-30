package lang

import (
	"os"
	"path/filepath"
	"sync"
)

// Memo remembers a value per key for the resolvers and classifiers that
// goroutines share: the zero Memo is empty and ready, and it must not be
// copied once used. It is sync.Map with the key and value types spelled out.
type Memo[K comparable, V any] struct {
	m sync.Map
}

// Get returns the value remembered for key, or computes, remembers and
// returns it. Two goroutines missing the same key at once may both compute it,
// and the later store wins, which suits values that come out the same
// whichever goroutine computes them.
func (memo *Memo[K, V]) Get(key K, compute func(K) V) V {
	if v, ok := memo.Load(key); ok {
		return v
	}
	v := compute(key)
	memo.m.Store(key, v)
	return v
}

// Load returns the value remembered for key, if any.
func (memo *Memo[K, V]) Load(key K) (V, bool) {
	v, ok := memo.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

// LoadOrStore returns the value remembered for key, or remembers and returns
// value. The first store wins, so every caller gets the same value: the one
// pointer that a sync.Once in it then fills once.
func (memo *Memo[K, V]) LoadOrStore(key K, value V) V {
	v, _ := memo.m.LoadOrStore(key, value)
	return v.(V)
}

// Present reports whether anything exists at an absolute path, without
// following a symbolic link there (os.Lstat): a marker file committed as a
// link says nothing of its target.
func Present(absolute string) bool {
	_, err := os.Lstat(absolute)
	return err == nil
}

// MarkerMemo returns a check of whether a directory, given as an absolute
// path, holds a file named marker (see Present), remembered per directory for
// the life of the process: a classifier asks it for every file of a directory.
func MarkerMemo(marker string) func(directory string) bool {
	var memo Memo[string, bool]
	present := func(directory string) bool { return Present(filepath.Join(directory, marker)) }
	return func(directory string) bool { return memo.Get(directory, present) }
}
