package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type answer struct {
	Name string   `json:"name"`
	Deps []string `json:"deps"`
}

func TestRoundTrip(t *testing.T) {
	s := New(t.TempDir(), time.Hour)
	want := answer{Name: "lodash", Deps: []string{"a", "b"}}
	s.Put("npm|lodash@4", want)

	got, ok := Get[answer](s, "npm|lodash@4")
	if !ok || got.Name != want.Name || len(got.Deps) != 2 {
		t.Errorf("got %+v (%v), want %+v", got, ok, want)
	}
	// A key nothing was written under is a miss, not an empty answer.
	if _, ok := Get[answer](s, "npm|other@1"); ok {
		t.Error("a key that was never written answered")
	}
	// The key is hashed, so anything the caller finds distinguishing will do.
	if _, ok := Get[answer](New(s.dir, time.Hour), "npm|lodash@4"); !ok {
		t.Error("a second store over the same directory did not read the answer")
	}
}

// Verifies: REQ-FND-012
func TestStaleAnswersAreAMiss(t *testing.T) {
	dir := t.TempDir()
	New(dir, time.Hour).Put("k", answer{Name: "x"})
	// Aged rather than given a shorter life: Windows reads a clock that ticks every
	// fifteen milliseconds, so an answer written and read inside one test is the same
	// instant there and no non-zero life expires it.
	found, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(found) != 1 {
		t.Fatalf("%d files written, want 1", len(found))
	}
	data, err := os.ReadFile(found[0])
	if err != nil {
		t.Fatal(err)
	}
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatal(err)
	}
	e.At = e.At.Add(-2 * time.Hour)
	if data, err = json.Marshal(e); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(found[0], data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Get[answer](New(dir, time.Hour), "k"); ok {
		t.Error("a stale answer was served")
	}
}

// A store is an optimization, so everything that could go wrong with one is a miss
// rather than a failure - and a caller that has to ask which has a store it cannot
// use from the branch where there is no cache directory.
func TestNothingToKeepIsNotAFailure(t *testing.T) {
	// A directory that cannot be made, because a file is standing where one of its
	// parents would go.
	blocked := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		s    *Store
	}{
		{"no directory", New("", time.Hour)},
		{"no time to live", New(t.TempDir(), 0)},
		{"a directory that cannot be made", New(filepath.Join(blocked, "under"), time.Hour)},
	} {
		if c.s != nil {
			t.Errorf("%s: a store was opened anyway", c.name)
			continue
		}
		c.s.Put("k", answer{Name: "x"}) // must not panic
		if _, ok := Get[answer](c.s, "k"); ok {
			t.Errorf("%s: a store that keeps nothing answered", c.name)
		}
	}
}

func TestAnAnswerOfTheWrongShapeIsAMiss(t *testing.T) {
	s := New(t.TempDir(), time.Hour)
	s.Put("k", answer{Name: "x"})
	// What was kept no longer decodes into what the caller now asks for - an older
	// build's answer, read by a newer one.
	if got, ok := Get[[]string](s, "k"); ok {
		t.Errorf("decoded as %v", got)
	}
	// Nor does a file that is not an entry at all.
	if err := os.WriteFile(s.path("junk"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Get[answer](s, "junk"); ok {
		t.Error("unreadable content answered")
	}
}

// Two writers on one key publish one whole answer or none: the scratch file is named
// per write, so neither can overwrite the other's half and have it renamed into place.
func TestConcurrentWritersDoNotPublishHalfAnAnswer(t *testing.T) {
	s := New(t.TempDir(), time.Hour)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Put("k", answer{Name: "x", Deps: []string{"a", "b", "c", "d"}})
			if got, ok := Get[answer](s, "k"); ok && len(got.Deps) != 4 {
				t.Errorf("writer %d read a torn answer: %+v", i, got)
			}
		}(i)
	}
	wg.Wait()
	got, ok := Get[answer](s, "k")
	if !ok || len(got.Deps) != 4 {
		t.Errorf("after 16 writers: %+v (%v)", got, ok)
	}
	// Every scratch file was renamed into place or removed; none was left behind.
	left, _ := filepath.Glob(filepath.Join(s.dir, "put-*"))
	if len(left) != 0 {
		t.Errorf("scratch files left behind: %v", left)
	}
}

func TestStaleAnswersAreSweptAway(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, time.Hour)
	s.Put("fresh", 1)
	s.Put("stale", 2)
	old := time.Now().Add(-2 * time.Hour)
	os.Chtimes(s.path("stale"), old, old)
	other := filepath.Join(dir, "README")
	os.WriteFile(other, nil, 0o644)
	os.Chtimes(other, old, old)

	New(dir, time.Hour)
	if _, err := os.Stat(s.path("stale")); !os.IsNotExist(err) {
		t.Error("an answer past its time to live was kept")
	}
	if _, ok := Get[int](s, "fresh"); !ok {
		t.Error("a fresh answer was swept")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("a file the store did not write was removed")
	}
}
