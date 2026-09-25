package lang

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Verifies: REQ-LANG-012, REQ-LANG-021, REQ-LANG-022
func TestForEachFileSkipsWhatItWouldNotParse(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.go")
	os.WriteFile(small, []byte("package a\n"), 0o644)
	// Measured as larger than MaxParseSize, but not on disk: a file that is read
	// anyway would be found and passed to fn.
	big := filepath.Join(dir, "big.go")
	os.WriteFile(big, []byte("package b\n"), 0o644)
	files := []*scan.File{
		{Path: "small.go", Abs: small, LOC: 1, Size: 10},
		{Path: "big.go", Abs: big, LOC: 1, Size: MaxParseSize + 1},
		{Path: "bin.go", Abs: small, Binary: true, Size: 10},
		{Path: "over.go", Abs: small, LOC: 1, Size: 10, TooLarge: true}, // over --max-file-size
	}
	got := ForEachFile(context.Background(), files, func(f *scan.File, _ []byte) *FileResult {
		return &FileResult{}
	})
	if len(got) != 1 || got["small.go"] == nil {
		t.Errorf("parsed %v, want small.go alone", got)
	}
}

// Verifies: REQ-LANG-010
func TestForEachFileRunsAtMostNumCPUAtOnce(t *testing.T) {
	// More files than workers, each held open briefly so that the pool fills up: the
	// peak of callbacks running together must never exceed the number of CPUs, and
	// every file must still come back.
	dir := t.TempDir()
	n := 4*runtime.NumCPU() + 3
	files := make([]*scan.File, n)
	for i := range files {
		name := fmt.Sprintf("f%d.go", i)
		abs := filepath.Join(dir, name)
		if err := os.WriteFile(abs, []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		files[i] = &scan.File{Path: name, Abs: abs, LOC: 1, Size: 10}
	}
	var running, peak atomic.Int64
	got := ForEachFile(context.Background(), files, func(f *scan.File, _ []byte) *FileResult {
		now := running.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		running.Add(-1)
		return &FileResult{}
	})
	if p := peak.Load(); p > int64(runtime.NumCPU()) {
		t.Errorf("%d callbacks ran at once on %d CPUs", p, runtime.NumCPU())
	}
	if len(got) != n {
		t.Errorf("got %d results, want %d", len(got), n)
	}

	// A cancelled analysis schedules nothing more.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := ForEachFile(ctx, files, func(*scan.File, []byte) *FileResult { return &FileResult{} }); len(got) != 0 {
		t.Errorf("a cancelled run still parsed %d files", len(got))
	}
}

// recorder is a plugin that claims .rec files by extension and writes down what the
// pipeline hands it, so the contract between them can be checked from the outside.
type recorder struct {
	mu        sync.Mutex
	extracted map[string]string // path -> content passed to Extract
	resolver  []string          // the files the resolver was built from
}

func (*recorder) Name() string             { return "rec" }
func (*recorder) Version() int             { return 1 }
func (*recorder) Claims(f *scan.File) bool { return path.Ext(f.Path) == ".rec" }
func (*recorder) Ecosystems() []Ecosystem  { return []Ecosystem{{ID: "rec", Name: "Recorded"}} }
func (r *recorder) Extract(f *scan.File, src []byte) (*Extraction, error) {
	r.mu.Lock()
	r.extracted[f.Path] = string(src)
	r.mu.Unlock()
	return &Extraction{Imports: []RawImport{{Spec: "dep", Module: "dep", Line: 1}}}, nil
}
func (r *recorder) Resolver(root string, all []*scan.File) (Resolver, error) {
	for _, f := range all {
		r.resolver = append(r.resolver, f.Path)
	}
	return resolverFunc(func(file string, imp RawImport) Target {
		return Target{Ecosystem: "rec", Package: imp.Module}
	}), nil
}

type resolverFunc func(string, RawImport) Target

func (f resolverFunc) Resolve(file string, imp RawImport) Target { return f(file, imp) }

// A plugin sees extraction one claimed file at a time, with nothing but that file and
// its content, while its resolver is built once from every file of the project - that
// split is what lets extractions be cached and resolution be redone.
//
// Verifies: REQ-LANG-001, REQ-LANG-025
func TestPluginsExtractOnlyWhatTheyClaim(t *testing.T) {
	dir := t.TempDir()
	var all []*scan.File
	for name, content := range map[string]string{"a.rec": "alpha\n", "b.rec": "beta\n", "c.txt": "gamma\n"} {
		abs := filepath.Join(dir, name)
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		all = append(all, &scan.File{Path: name, Abs: abs, LOC: 1, Size: int64(len(content))})
	}
	p := &recorder{extracted: map[string]string{}}
	res, err := Analyze(context.Background(), p, dir, all)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a.rec": "alpha\n", "b.rec": "beta\n"}
	if !reflect.DeepEqual(p.extracted, want) {
		t.Errorf("extracted %v, want the two claimed files with their content", p.extracted)
	}
	if res["c.txt"] != nil {
		t.Error("a file the plugin does not claim has a result")
	}
	if got := res["a.rec"]; got == nil || len(got.Imports) != 1 || got.Imports[0].Target.Package != "dep" {
		t.Errorf("a.rec: %+v, want its import resolved", got)
	}
	sort.Strings(p.resolver)
	if !reflect.DeepEqual(p.resolver, []string{"a.rec", "b.rec", "c.txt"}) {
		t.Errorf("resolver built from %v, want every project file", p.resolver)
	}
}

// classified reads a file differently depending on its directory, so it says so.
type classified struct{ *recorder }

func (classified) Class(f *scan.File) string { return path.Dir(f.Path) }

// Verifies: REQ-LANG-025
func TestAClassBecomesPartOfTheCacheKey(t *testing.T) {
	plain := &recorder{}
	// Without a class only the extension counts: two directories share an entry.
	if ClassOf(plain, &scan.File{Path: "x/a.rec"}) != ClassOf(plain, &scan.File{Path: "y/a.rec"}) {
		t.Error("a plugin that declares no class is keyed by more than the extension")
	}
	c := classified{plain}
	if ClassOf(c, &scan.File{Path: "x/a.rec"}) == ClassOf(c, &scan.File{Path: "y/a.rec"}) {
		t.Error("the declared class is not part of the key")
	}
	if ClassOf(c, &scan.File{Path: "x/a.rec"}) == ClassOf(c, &scan.File{Path: "x/a.other"}) {
		t.Error("the extension is no longer part of the key")
	}
}
