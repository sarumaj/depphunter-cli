package lang

import (
	"os"
	"path/filepath"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The helpers below read the files resolvers parse: manifests, lock files and
// the metadata of installed packages. Each keeps the bound its callers had, so
// they differ on purpose:
//
//   - ReadScanned reads a file the scan listed, which Readable admits or not
//     before anything is loaded.
//   - ReadBounded measures a file on disk first (os.Stat, which follows a
//     symbolic link) and refuses a directory or one over MaxParseSize.
//   - ReadCapped reads a file on disk whole and drops it when it turns out over
//     MaxParseSize.
//   - ReadWhole reads a file on disk whole, however large.
//
// All four report failure as false rather than an error: a file that is gone,
// unreadable or too large is skipped by every resolver alike.

// ReadScanned reads a scanned file, unless it is binary, over the scan's size
// limit or over MaxParseSize (see Readable).
//
// Implements: REQ-LANG-012
func ReadScanned(f *scan.File) ([]byte, bool) {
	if !Readable(f) {
		return nil, false
	}
	return ReadWhole(f.AbsolutePath)
}

// ReadBounded reads a file on disk that is no larger than MaxParseSize,
// measuring it before reading it; a directory is refused as well. Like
// os.ReadFile it returns what it read even when reading fails part way.
//
// Implements: REQ-LANG-012
func ReadBounded(absolute string) ([]byte, bool) {
	fileInfo, err := os.Stat(absolute)
	if err != nil || fileInfo.IsDir() || fileInfo.Size() > MaxParseSize {
		return nil, false
	}
	return ReadWhole(absolute)
}

// ReadCapped reads a file on disk and drops it when its content is larger than
// MaxParseSize, for the callers that measure what they read rather than the
// file.
//
// Implements: REQ-LANG-012
func ReadCapped(absolute string) ([]byte, bool) {
	data, err := os.ReadFile(absolute)
	if err != nil || len(data) > MaxParseSize {
		return nil, false
	}
	return data, true
}

// ReadWhole reads a file on disk without a bound. Like os.ReadFile it returns
// what it read even when reading fails part way.
func ReadWhole(absolute string) ([]byte, bool) {
	data, err := os.ReadFile(absolute)
	return data, err == nil
}

// SourceOptions says how a Source reads.
type SourceOptions struct {
	// Confined refuses, rather than looks for on disk, a path that Inside
	// rejects: one that climbs out of the root with ".." or is absolute. The
	// check is on the path alone; a symbolic link under the root is followed.
	Confined bool
	// Bounded reads every file, listed or not, through ReadBounded; otherwise
	// through ReadWhole.
	Bounded bool
}

// Source reads a repository's files by their path relative to the root: the
// scanned copy of a file the scan listed, else the file on disk under the
// root. Lock files are ignored by git as often as not, yet what is on disk
// beside a manifest is what the package manager resolved with; Listed tells a
// resolver which files came from disk, for NoteList.NoteIgnored.
type Source struct {
	root     string
	absolute map[string]string // listed path -> its absolute path
	options  SourceOptions
}

// NewSource makes a Source for the repository at root, which lists no file
// until Add is called. With root empty, only listed files are read.
func NewSource(root string, options SourceOptions) *Source {
	return &Source{root: root, absolute: map[string]string{}, options: options}
}

// Add lists a scanned file, whose copy Read then prefers to the disk.
func (s *Source) Add(f *scan.File) {
	s.absolute[f.Path] = f.AbsolutePath
}

// Listed reports whether Add listed the file at relative.
func (s *Source) Listed(relative string) bool {
	_, ok := s.absolute[relative]
	return ok
}

// Absolute is the absolute path of a listed file.
func (s *Source) Absolute(relative string) (string, bool) {
	a, ok := s.absolute[relative]
	return a, ok
}

// Read reads the file at relative, a cleaned path such as path.Join gives,
// from its scanned copy when it is listed, else from disk under the root. It
// fails without a root, or, when the Source is Confined, for a path outside
// the root.
func (s *Source) Read(relative string) ([]byte, bool) {
	a, ok := s.absolute[relative]
	if !ok {
		if s.root == "" || s.options.Confined && !Inside(relative) {
			return nil, false
		}
		a = filepath.Join(s.root, filepath.FromSlash(relative))
	}
	if s.options.Bounded {
		return ReadBounded(a)
	}
	return ReadWhole(a)
}
