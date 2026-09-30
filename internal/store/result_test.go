package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// report stands for a cached result: large enough that a torn write would show.
type report struct {
	Writer int      `json:"writer"`
	Lines  []string `json:"lines"`
}

func reportOf(writer int) *report {
	r := &report{Writer: writer}
	for i := range 2000 {
		r.Lines = append(r.Lines, fmt.Sprintf("writer %d, line %d", writer, i))
	}
	return r
}

// whole reports whether r is one writer's report, all of it.
func whole(r *report) bool {
	return r != nil && len(r.Lines) == 2000 && r.Lines[0] == fmt.Sprintf("writer %d, line 0", r.Writer) &&
		r.Lines[1999] == fmt.Sprintf("writer %d, line 1999", r.Writer)
}

// names lists the files in directory.
func names(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// A result is kept under a file named for the project, the kind and the key, and a
// save replaces the project's result for another key but not another project's or
// another kind's.
func TestResultKeepsOneKeyPerProjectAndKind(t *testing.T) {
	directory := t.TempDir()
	results := NewResult[report](directory, "/src/app", "history")
	if _, ok := results.Load("a"); ok {
		t.Fatal("a result that was never saved was loaded")
	}
	other := NewResult[report](directory, "/src/other", "history")
	references := NewResult[report](directory, "/src/app", "references")
	other.Save("a", reportOf(8))
	references.Save("a", reportOf(9))

	results.Save("a", reportOf(1))
	if got, ok := results.Load("a"); !ok || got.Writer != 1 || !whole(got) {
		t.Fatalf("Load(a) = %v, %v", got != nil, ok)
	}
	if !strings.HasSuffix(results.File("a"), "-history-a.json.gz") || filepath.Dir(results.File("a")) != directory {
		t.Errorf("File(a) = %s", results.File("a"))
	}
	results.Save("b", reportOf(2))
	if _, ok := results.Load("a"); ok {
		t.Error("the result for an earlier key was kept")
	}
	if got, ok := results.Load("b"); !ok || got.Writer != 2 {
		t.Error("the result just saved was not loaded")
	}
	if got, ok := other.Load("a"); !ok || got.Writer != 8 {
		t.Error("another project's result was removed")
	}
	if got, ok := references.Load("a"); !ok || got.Writer != 9 {
		t.Error("another kind of result was removed")
	}
	if n := len(names(t, directory)); n != 3 {
		t.Errorf("%d files in the directory, want 3: %v", n, names(t, directory))
	}
}

// Without a directory - --no-cache - nothing is written, not even relative to the
// working directory, and nothing is read.
func TestResultWithoutADirectoryKeepsNothing(t *testing.T) {
	working := t.TempDir()
	t.Chdir(working)
	results := NewResult[report]("", "/src/app", "history")
	results.Save("a", reportOf(1))
	if n := len(names(t, working)); n != 0 {
		t.Errorf("files written to the working directory: %v", names(t, working))
	}
	if err := os.WriteFile(results.File("a"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := results.Load("a"); ok {
		t.Error("a result was loaded without a directory")
	}
}

// Concurrent writers of one result each publish a whole file: the scratch file is
// named per write. With one scratch name for all (file+".tmp", as history and lsp
// once had), one writer truncates and rewrites the file another is about to rename,
// and the result published is torn.
func TestConcurrentResultWritersDoNotCorruptEachOther(t *testing.T) {
	directory := t.TempDir()
	results := NewResult[report](directory, "/src/app", "references")
	var wg sync.WaitGroup
	for writer := range 16 {
		wg.Go(func() {
			for range 10 {
				results.Save("k", reportOf(writer))
				// A reader may find no file, between another writer's sweep and its
				// rename, but never a part of one.
				if got, ok := results.Load("k"); ok && !whole(got) {
					t.Errorf("writer %d read a torn result", writer)
				}
			}
		})
	}
	wg.Wait()
	got, ok := results.Load("k")
	if !ok || !whole(got) {
		t.Fatalf("after 16 writers the result is not whole (%v)", ok)
	}
	if left := names(t, directory); !slices.Equal(left, []string{filepath.Base(results.File("k"))}) {
		t.Errorf("files left behind: %v", left)
	}
}

// WriteAtomic leaves the previous file in place, and nothing else behind, when the
// write fails.
func TestWriteAtomicKeepsThePreviousFileOnFailure(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "f")
	if err := WriteAtomic(file, func(w io.Writer) error { _, err := io.WriteString(w, "old"); return err }); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("disk full")
	err := WriteAtomic(file, func(w io.Writer) error {
		io.WriteString(w, "half")
		return failure
	})
	if !errors.Is(err, failure) {
		t.Errorf("WriteAtomic returned %v, want the write's error", err)
	}
	if data, _ := os.ReadFile(file); string(data) != "old" {
		t.Errorf("file holds %q after a failed write", data)
	}
	if left := names(t, directory); !slices.Equal(left, []string{"f"}) {
		t.Errorf("files left behind: %v", left)
	}
}
