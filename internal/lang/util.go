package lang

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// MaxParseSize bounds the files plugins read; larger files are almost always generated.
//
// Implements: REQ-LANG-012
const MaxParseSize = 1 << 20

// SymbolSet collects a file's symbols, keeping names unique within the file
// (graph ids are derived from them) by suffixing repeats with their line.
//
// Implements: REQ-LANG-003
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
//
// Implements: REQ-DIST-016, REQ-LANG-010, REQ-LANG-012, REQ-LANG-021, REQ-LANG-022
func ForEachFile(ctx context.Context, files []*scan.File, fn func(f *scan.File, src []byte) *FileResult) map[string]*FileResult {
	results := make(map[string]*FileResult, len(files))
	var mu sync.Mutex
	var g errgroup.Group
	g.SetLimit(runtime.NumCPU())
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		// What Parseable would reject on its size alone is not read at all: a
		// generated bundle can run to megabytes, and this runs on every analysis.
		if f.Binary || f.TooLarge || f.Size > MaxParseSize {
			continue
		}
		g.Go(func() error {
			src, err := os.ReadFile(f.Abs)
			if err != nil || !Parseable(f, src) {
				return nil
			}
			if res := fn(f, src); res != nil {
				mu.Lock()
				results[f.Path] = res
				mu.Unlock()
			}
			return nil
		})
	}
	g.Wait()
	return results
}

// Parseable rejects oversized and minified sources: parsing them is slow and their
// structure (bundled code) says nothing about the project.
//
// Implements: REQ-LANG-012
func Parseable(f *scan.File, src []byte) bool {
	if len(src) > MaxParseSize || f.Binary {
		return false
	}
	lines := max(f.LOC, 1)
	return !(len(src) > 20_000 && len(src)/lines > 250)
}
