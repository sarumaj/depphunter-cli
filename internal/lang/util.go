package lang

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
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

// ForEachFile reads each file and calls function from NumCPU goroutines. Files that cannot
// be read, are too large, or look minified are skipped. function must be safe for
// concurrent use; the returned map collects its non-nil results.
//
// Implements: REQ-DIST-016, REQ-LANG-010, REQ-LANG-012, REQ-LANG-021, REQ-LANG-022
func ForEachFile(ctx context.Context, files []*scan.File, function func(f *scan.File, source []byte) *FileResult) map[string]*FileResult {
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
			source, err := os.ReadFile(f.AbsolutePath)
			if err != nil || !Parseable(f, source) {
				return nil
			}
			if result := function(f, source); result != nil {
				mu.Lock()
				results[f.Path] = result
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
func Parseable(f *scan.File, source []byte) bool {
	if len(source) > MaxParseSize || f.Binary {
		return false
	}
	lines := max(f.LOC, 1)
	return !(len(source) > 20_000 && len(source)/lines > 250)
}

// RepositoryName names a repository or download by its URL: without scheme, user and
// ".git", the host in lower case and without a port -
// https://github.com/apple/swift-nio.git and git@github.com:apple/swift-nio are
// both github.com/apple/swift-nio. SwiftPM packages and CMake's fetched content are
// named so.
//
// Implements: REQ-SWIFT-006, REQ-CMAKE-007
func RepositoryName(url string) string {
	s := strings.TrimSpace(url)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	} else if at, rest, ok := strings.Cut(s, "@"); ok && !strings.Contains(at, "/") {
		s = strings.Replace(rest, ":", "/", 1) // scp-like git@host:owner/repo
	}
	if at, rest, ok := strings.Cut(s, "@"); ok && !strings.Contains(at, "/") {
		s = rest // https://user@host/...
	}
	s = strings.TrimSuffix(strings.TrimRight(s, "/"), ".git")
	host, rest, _ := strings.Cut(s, "/")
	if h, _, ok := strings.Cut(host, ":"); ok { // a port
		host = h
	}
	if rest == "" {
		return strings.ToLower(host)
	}
	return strings.ToLower(host) + "/" + rest
}
