package lang

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The helpers below read the files resolvers parse: manifests, lock files and
// the metadata of installed packages. There are two kinds of such files:
//
//   - A file under the analyzed repository. The scan lists none that is a
//     symbolic link (a committed link may point anywhere on this machine), and
//     what the scan left out - lock files git ignores, installed trees - is
//     read through a Root, which refuses a path that leaves the repository
//     through ".." or through a link. A link that stays inside it (pnpm's
//     node_modules/.pnpm, a workspace's links) is followed.
//   - A file of this machine: the package managers' configuration, caches and
//     the trees they install under the user's home. Those are read through
//     Machine, which goes anywhere, as the user's own tools would.
//
// A read of either kind through ReadBounded (or ReadLimited, for a reader with
// a bound of its own) is bounded by MaxParseSize, measured on the file once it
// is open, so a link cannot pass off a large file as a small one.
// Failure is reported as false rather than an error: a file that is gone,
// unreadable, too large or outside the repository is skipped by every
// resolver alike.

// ErrOutside is the error of a read that would leave the repository, by its
// path or through a symbolic link.
var ErrOutside = errors.New("outside the repository")

// Root reads the files under one directory, the analyzed repository, and
// nothing outside it. Its methods take absolute paths as the resolvers build
// them (filepath.Join of the root and a relative path) and refuse one that
// does not lie under the root once cleaned, or that reaches outside it
// through a symbolic link.
//
// A link is followed when its target is under the root, relative or absolute
// (pnpm's node_modules/.pnpm, the link shards makes for a path dependency):
// the path is resolved, checked against the resolved root, and what it
// resolves to is then opened through an os.Root, which refuses anything
// swapped for a link out of the root in the meantime. The zero Root reads
// nothing.
//
// Implements: REQ-LANG-031
type Root struct {
	dir     string
	machine bool
}

// OpenRoot makes the Root of the repository at dir. With dir empty it reads
// nothing.
//
// Implements: REQ-LANG-031
func OpenRoot(dir string) Root {
	return Root{dir: dir}
}

// Machine reads anywhere on this machine: for configuration, caches and
// installed trees outside the repository, never for a path a repository
// names.
var Machine = Root{machine: true}

// Join is the absolute path of relative, a slash-separated path under the
// root. It is cleaned, and so may name a path outside the root, which the
// reads then refuse.
func (r Root) Join(relative string) string {
	return filepath.Join(r.dir, filepath.FromSlash(relative))
}

// under is p relative to directory, slash-separated as os.Root's fs.FS wants
// it, and false when p climbs out of directory. Either may be relative to the
// working directory.
func under(directory, p string) (string, bool) {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", false
	}
	if p, err = filepath.Abs(p); err != nil {
		return "", false
	}
	relative, err := filepath.Rel(directory, p)
	if err != nil {
		return "", false
	}
	relative = filepath.ToSlash(relative)
	if !Inside(relative) || !fs.ValidPath(relative) {
		return "", false
	}
	return relative, true
}

// resolve opens the root, once its own links are resolved, as an os.Root, and
// gives the path absolute resolves to under it: every symbolic link on the way
// followed, the last one only when followLast. It fails for a path outside the
// root, by its name or by where a link on it leads.
func (r Root) resolve(op, absolute string, followLast bool) (*os.Root, string, error) {
	outside := &fs.PathError{Op: op, Path: absolute, Err: ErrOutside}
	if r.dir == "" {
		return nil, "", outside
	}
	relative, ok := under(r.dir, absolute)
	if !ok {
		return nil, "", outside
	}
	dir, err := filepath.Abs(r.dir)
	if err != nil {
		return nil, "", err
	}
	realRoot, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, "", err
	}
	real := realRoot
	if relative != "." {
		target := filepath.Join(dir, filepath.FromSlash(relative))
		if followLast {
			real, err = filepath.EvalSymlinks(target)
		} else {
			real, err = filepath.EvalSymlinks(filepath.Dir(target))
			real = filepath.Join(real, filepath.Base(target))
		}
		if err != nil {
			return nil, "", err
		}
	}
	if relative, ok = under(realRoot, real); !ok {
		return nil, "", outside
	}
	root, err := os.OpenRoot(realRoot)
	if err != nil {
		return nil, "", err
	}
	return root, relative, nil
}

// Contains reports whether absolute lies under the root once cleaned (or is
// the root). It does not look at the disk; opening the file does that.
func (r Root) Contains(absolute string) bool {
	_, ok := under(r.dir, absolute)
	return r.dir != "" && ok
}

// Open opens a file for reading.
func (r Root) Open(absolute string) (*os.File, error) {
	if r.machine {
		return os.Open(absolute)
	}
	root, relative, err := r.resolve("open", absolute, true)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(filepath.FromSlash(relative))
}

// Stat describes a file, following a symbolic link (inside the root only).
func (r Root) Stat(absolute string) (fs.FileInfo, error) {
	if r.machine {
		return os.Stat(absolute)
	}
	root, relative, err := r.resolve("stat", absolute, true)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Stat(filepath.FromSlash(relative))
}

// ReadDir lists a directory sorted by name, as os.ReadDir does.
func (r Root) ReadDir(absolute string) ([]fs.DirEntry, error) {
	if r.machine {
		return os.ReadDir(absolute)
	}
	root, relative, err := r.resolve("readdir", absolute, true)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return fs.ReadDir(root.FS(), relative)
}

// WalkDir walks the tree at absolute as filepath.WalkDir does, calling
// function with paths under absolute; like it, it follows no symbolic link in
// the tree, nor one at its top.
func (r Root) WalkDir(absolute string, function fs.WalkDirFunc) error {
	if r.machine {
		return filepath.WalkDir(absolute, function)
	}
	root, relative, err := r.resolve("lstat", absolute, false)
	if err != nil {
		return function(absolute, nil, err)
	}
	defer root.Close()
	info, err := root.Lstat(filepath.FromSlash(relative))
	if err != nil {
		return function(absolute, nil, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		// fs.WalkDir would descend into it; filepath.WalkDir does not.
		err := function(absolute, fs.FileInfoToDirEntry(info), nil)
		if err == filepath.SkipDir || err == filepath.SkipAll {
			return nil
		}
		return err
	}
	return fs.WalkDir(root.FS(), relative, func(p string, d fs.DirEntry, err error) error {
		below, _ := filepath.Rel(filepath.FromSlash(relative), filepath.FromSlash(p))
		return function(filepath.Join(absolute, below), d, err)
	})
}

// Glob lists the paths matching pattern as filepath.Glob does, leaving out
// those outside the root, by their name or through a link on the way.
func (r Root) Glob(pattern string) ([]string, error) {
	if r.machine {
		return filepath.Glob(pattern)
	}
	if !r.Contains(pattern) {
		return nil, nil
	}
	matches, err := filepath.Glob(pattern)
	kept := matches[:0]
	for _, m := range matches {
		if root, _, err := r.resolve("glob", m, false); err == nil {
			root.Close()
			kept = append(kept, m)
		}
	}
	return kept, err
}

// IsDirectory reports whether absolute is a directory, following a symbolic
// link that stays inside the root.
func (r Root) IsDirectory(absolute string) bool {
	info, err := r.Stat(absolute)
	return err == nil && info.IsDir()
}

// ReadBounded reads a regular file no larger than MaxParseSize, measuring it
// once it is open, so that the measure is of the file read even through a
// symbolic link; a directory or a device is refused. Like os.ReadFile it
// returns what it read even when reading fails part way.
//
// Implements: REQ-LANG-012, REQ-LANG-031
func (r Root) ReadBounded(absolute string) ([]byte, bool) {
	return r.ReadLimited(absolute, MaxParseSize)
}

// ReadLimited is ReadBounded with a bound of limit bytes, for the resolvers
// that allow a larger or smaller file than MaxParseSize.
func (r Root) ReadLimited(absolute string, limit int64) ([]byte, bool) {
	f, err := r.Open(absolute)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	fileInfo, err := f.Stat()
	if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Size() > limit {
		return nil, false
	}
	// A file that grows past the bound while it is read is refused as well.
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(data)) > limit {
		return nil, false
	}
	return data, err == nil
}

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

// ReadBounded reads a file of this machine through Machine.ReadBounded.
//
// Implements: REQ-LANG-012
func ReadBounded(absolute string) ([]byte, bool) {
	return Machine.ReadBounded(absolute)
}

// ReadWhole reads a file on disk without a bound, for a scanned file that
// Readable has admitted. Like os.ReadFile it returns what it read even when
// reading fails part way.
func ReadWhole(absolute string) ([]byte, bool) {
	data, err := os.ReadFile(absolute)
	return data, err == nil
}

// Source reads a repository's files by their path relative to the root: the
// scanned copy of a file the scan listed, else the file on disk under the
// root, through a Root. Lock files are ignored by git as often as not, yet
// what is on disk beside a manifest is what the package manager resolved
// with; Listed tells a resolver which files came from disk, for
// NoteList.NoteIgnored. Every read, listed or not, is bounded by
// MaxParseSize.
type Source struct {
	root     Root
	absolute map[string]string // listed path -> its absolute path
}

// NewSource makes a Source for the repository at root, which lists no file
// until Add is called. With root empty, only listed files are read.
func NewSource(root string) *Source {
	return &Source{root: OpenRoot(root), absolute: map[string]string{}}
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

// Read reads the file at relative, a slash-separated path, from its scanned
// copy when it is listed, else from disk under the root. It fails without a
// root and for a path that leaves the root, by ".." or through a symbolic
// link.
//
// Implements: REQ-LANG-031
func (s *Source) Read(relative string) ([]byte, bool) {
	if a, ok := s.absolute[relative]; ok {
		return ReadBounded(a)
	}
	return s.root.ReadBounded(s.root.Join(relative))
}
