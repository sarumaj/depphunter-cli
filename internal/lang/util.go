package lang

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// MaxParseSize bounds the files plugins read; larger files are almost always generated.
const MaxParseSize = 1 << 20

// SymbolSet collects a file's symbols, keeping names unique within the file
// (graph ids are derived from them) by suffixing repeats with their line.
type SymbolSet struct {
	list []Symbol
	seen map[string]bool
}

func (s *SymbolSet) Add(name, kind string, line int) {
	if name == "" || name == "_" {
		return
	}
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	if s.seen[name] {
		name = fmt.Sprintf("%s@%d", name, line)
		if s.seen[name] {
			return // same name captured twice at the same place
		}
	}
	s.seen[name] = true
	s.list = append(s.list, Symbol{Name: name, Kind: kind, Line: line})
}

func (s *SymbolSet) List() []Symbol { return s.list }

// ForEachFile reads each file and calls fn from NumCPU goroutines. Files that cannot
// be read, are too large, or look minified are skipped. fn must be safe for
// concurrent use; the returned map collects its non-nil results.
func ForEachFile(ctx context.Context, files []*scan.File, fn func(f *scan.File, src []byte) *FileResult) map[string]*FileResult {
	results := make(map[string]*FileResult, len(files))
	var mu sync.Mutex
	var wg sync.WaitGroup
	work := make(chan *scan.File)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range work {
				src, err := os.ReadFile(f.Abs)
				if err != nil || !Parseable(f, src) {
					continue
				}
				if res := fn(f, src); res != nil {
					mu.Lock()
					results[f.Path] = res
					mu.Unlock()
				}
			}
		}()
	}
	for _, f := range files {
		select {
		case work <- f:
		case <-ctx.Done():
		}
	}
	close(work)
	wg.Wait()
	return results
}

// Parseable rejects oversized and minified sources: parsing them is slow and their
// structure (bundled code) says nothing about the project.
func Parseable(f *scan.File, src []byte) bool {
	if len(src) > MaxParseSize || f.Binary {
		return false
	}
	lines := max(f.LOC, 1)
	return !(len(src) > 20_000 && len(src)/lines > 250)
}
