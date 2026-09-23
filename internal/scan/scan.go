// Package scan enumerates the files of a project and measures them.
package scan

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"
)

// File is a project file with paths relative to the project root, always slash-separated.
type File struct {
	Path   string
	Abs    string
	Lang   string
	LOC    int
	Binary bool
	// Size is the file's size in bytes when it was measured, so that a file too
	// large to be worth reading can be passed over without being read.
	Size int64
}

type Options struct {
	Exclude     []string // glob patterns matched against the relative path and each of its segments
	MaxFileSize int64    // files larger than this are listed but not read
}

// defaultIgnore applies when git is unavailable, so a plain walk does not descend into
// dependency caches and build output.
var defaultIgnore = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, "target": true, "bin": true, "obj": true,
	".venv": true, "venv": true, "__pycache__": true, ".idea": true, ".vscode": true,
	".next": true, ".cache": true, ".gradle": true, ".tox": true, ".mypy_cache": true,
}

func Scan(ctx context.Context, root string, opts Options) ([]*File, error) {
	paths, err := gitFiles(ctx, root)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if paths, err = walkFiles(ctx, root); err != nil {
			return nil, err
		}
	}

	files := make([]*File, 0, len(paths))
	for _, p := range paths {
		if !excluded(p, opts.Exclude) {
			files = append(files, &File{Path: p, Abs: filepath.Join(root, filepath.FromSlash(p)), Lang: Language(p)})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	var g errgroup.Group
	g.SetLimit(runtime.NumCPU())
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error { measure(f, opts.MaxFileSize); return nil })
	}
	g.Wait()
	return files, ctx.Err()
}

// gitFiles lists tracked and untracked-but-not-ignored files, which honours every
// .gitignore, .git/info/exclude and the global excludes file for free.
//
// Duplicates - a file with merge conflicts is listed once per stage - are dropped
// here rather than with --deduplicate, which needs git 2.31; an older git refuses
// the option, and the walk it would fall back to knows none of the ignore files.
func gitFiles(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	seen := map[string]bool{}
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) == 0 || seen[string(p)] {
			continue
		}
		seen[string(p)] = true
		// Deleted-but-tracked files and submodule entries are listed but are not regular
		// files. Nor is a symbolic link, which is not followed: one committed to the
		// repository may point anywhere on this machine, and what a file node holds is
		// served by /api/file and written into the HTML export. The walk below skips
		// them for the same reason.
		if st, err := os.Lstat(filepath.Join(root, string(p))); err == nil && st.Mode().IsRegular() {
			paths = append(paths, string(p))
		}
	}
	return paths, nil
}

func walkFiles(ctx context.Context, root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			// One directory that cannot be read - a volume owned by another user, say -
			// is left out rather than failing the scan of everything else.
			if p != root {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return err
		}
		if d.IsDir() {
			if p != root && defaultIgnore[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(root, p)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	return paths, err
}

func excluded(rel string, patterns []string) bool {
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, rel); ok {
			return true
		}
		for _, seg := range strings.Split(rel, "/") {
			if ok, _ := path.Match(pat, seg); ok {
				return true
			}
		}
	}
	return false
}

func measure(f *File, maxSize int64) {
	st, err := os.Stat(f.Abs)
	if err != nil {
		return
	}
	f.Size = st.Size()
	if maxSize > 0 && f.Size > maxSize {
		return
	}
	fh, err := os.Open(f.Abs)
	if err != nil {
		return
	}
	defer fh.Close()

	r := bufio.NewReader(fh)
	if head, _ := r.Peek(8000); bytes.IndexByte(head, 0) >= 0 {
		f.Binary = true
		return
	}
	var lines, last int
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		lines += bytes.Count(buf[:n], []byte{'\n'})
		if n > 0 {
			last = int(buf[n-1])
		}
		if err != nil {
			break
		}
	}
	if last != 0 && last != '\n' {
		lines++ // final line without trailing newline
	}
	f.LOC = lines
}
