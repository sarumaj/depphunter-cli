package cache

import (
	"os"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

func TestAnUnfinishedRunDoesNotPrune(t *testing.T) {
	c := Open(t.TempDir(), "/project")
	c.BeginRun()
	c.Put("a", &lang.Extraction{})
	c.Put("b", &lang.Extraction{})
	c.EndRun()

	// Cancelled after reaching one file of two: b is still the project's.
	c.BeginRun()
	c.Get("a")
	c.BeginRun()
	if _, ok := c.Get("b"); !ok {
		t.Error("an entry was dropped because a run that failed did not reach it")
	}

	// A run that finishes without it is what says it is gone.
	c.BeginRun()
	c.Get("a")
	c.EndRun()
	c.BeginRun()
	if _, ok := c.Get("b"); ok {
		t.Error("an entry no finished run used was kept")
	}
}

func TestSaveWritesOnlyWhatChanged(t *testing.T) {
	dir := t.TempDir()
	c := Open(dir, "/project")
	c.BeginRun()
	c.Put("a", &lang.Extraction{})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(c.path, old, old); err != nil {
		t.Fatal(err)
	}

	// A --watch re-analysis that parsed nothing.
	c.BeginRun()
	c.Get("a")
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(c.path); !st.ModTime().Equal(old) {
		t.Error("the cache was written again with nothing new in it")
	}

	c.Put("b", &lang.Extraction{})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(c.path); st.ModTime().Equal(old) {
		t.Error("a new entry was not written")
	}
	if _, ok := Open(dir, "/project").Get("b"); !ok {
		t.Error("the new entry is not on disk")
	}
}
