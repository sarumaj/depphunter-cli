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
		if paths, err = walkFiles(root); err != nil {
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
func gitFiles(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		// Deleted-but-tracked files and submodule entries are listed but are not regular files.
		if st, err := os.Stat(filepath.Join(root, string(p))); err == nil && st.Mode().IsRegular() {
			paths = append(paths, string(p))
		}
	}
	return paths, nil
}

func walkFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
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
	if err != nil || (maxSize > 0 && st.Size() > maxSize) {
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
