// Package history reads a project's git history into per-file change lists, from
// which the UI computes churn, age and authorship for any time range.
package history

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNoHistory means the project is not in a git work tree or has no commits.
var ErrNoHistory = errors.New("no git history")

// Change is one commit's change to one file, as a compact JSON array:
// [unix time, author index, lines added, lines deleted, commit index].
type Change [5]int64

type History struct {
	Head      string              `json:"head"`
	Commits   int                 `json:"commits"`   // commits read, newest first
	Truncated bool                `json:"truncated"` // the limit was reached; older commits are missing
	Authors   []string            `json:"authors"`   // display names, indexed by Change[1]
	Files     map[string][]Change `json:"files"`     // project-relative path -> changes, newest first
}

// Head returns the commit hash HEAD points to, or ErrNoHistory.
func Head(ctx context.Context, root string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--verify", "-q", "HEAD").Output()
	if err != nil {
		return "", ErrNoHistory
	}
	return strings.TrimSpace(string(out)), nil
}

// GitDirs returns the directories whose changes signal a new HEAD: the git directory
// (HEAD, packed-refs) and refs/heads (branch tips). Nil outside a work tree.
func GitDirs(ctx context.Context, root string) []string {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return nil
	}
	dir := strings.TrimSpace(string(out))
	return []string{dir, filepath.Join(dir, "refs", "heads")}
}

// Collect reads at most maxCommits non-merge commits reachable from HEAD. Paths are
// relative to root (which may be a subdirectory of the repository). Renames are
// followed: changes made under an old name are filed under the file's current path.
func Collect(ctx context.Context, root string, maxCommits int) (*History, error) {
	head, err := Head(ctx, root)
	if err != nil {
		return nil, err
	}
	// \x1e starts a commit header; \x1f separates its fields.
	cmd := exec.CommandContext(ctx, "git", "-C", root, "log", "--no-merges", "-M", "--relative",
		"--numstat", "--format=%x1e%at%x1f%aN%x1f%aE", "-n", strconv.Itoa(maxCommits+1), "HEAD",
		"--", ".") // only commits touching root; --relative alone still lists the others
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	h := &History{Head: head, Files: map[string][]Change{}}
	authors := map[string]int{} // lower-case e-mail (or name) -> index
	// renamed maps an older path to the path it has at HEAD. The log runs newest
	// first, so a rename is seen before the changes made under the old name.
	renamed := map[string]string{}
	current := func(p string) string {
		if c, ok := renamed[p]; ok {
			return c
		}
		return p
	}
	var when, author int64
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "\x1e") {
			if h.Commits == maxCommits {
				h.Truncated = true
				break
			}
			h.Commits++
			fields := strings.SplitN(line[1:], "\x1f", 3)
			if len(fields) != 3 {
				continue
			}
			when, _ = strconv.ParseInt(fields[0], 10, 64)
			key := strings.ToLower(fields[2])
			if key == "" {
				key = fields[1]
			}
			idx, ok := authors[key]
			if !ok {
				idx = len(h.Authors)
				authors[key] = idx
				h.Authors = append(h.Authors, fields[1])
			}
			author = int64(idx)
			continue
		}
		// numstat: "added<TAB>deleted<TAB>path"; binary files show "-".
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 || h.Commits == 0 {
			continue
		}
		added, _ := strconv.ParseInt(parts[0], 10, 64)
		deleted, _ := strconv.ParseInt(parts[1], 10, 64)
		oldPath, newPath := renamePaths(unquote(parts[2]))
		path := current(filepath.ToSlash(newPath))
		if oldPath != "" {
			renamed[filepath.ToSlash(oldPath)] = path
		}
		h.Files[path] = append(h.Files[path], Change{when, author, added, deleted, int64(h.Commits - 1)})
	}
	if err := sc.Err(); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return nil, err
	}
	if h.Truncated {
		cmd.Process.Kill() // we stopped reading early on purpose
		cmd.Wait()
		return h, nil
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("git log: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return h, nil
}

// renamePaths splits a numstat rename, "old => new" or "dir/{old => new}/file",
// into its paths; old is "" for a plain path.
func renamePaths(p string) (old, new string) {
	if !strings.Contains(p, " => ") {
		return "", p
	}
	open, close := strings.Index(p, "{"), strings.LastIndex(p, "}")
	if open < 0 || close < open {
		o, n, _ := strings.Cut(p, " => ")
		return o, n
	}
	prefix, suffix := p[:open], p[close+1:]
	o, n, _ := strings.Cut(p[open+1:close], " => ")
	// "{ => sub}" leaves an empty side, hence path.Clean on "dir//file".
	return cleanPath(prefix + o + suffix), cleanPath(prefix + n + suffix)
}

func cleanPath(p string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(p)), "./")
}

// unquote decodes git's C-style quoting of unusual paths ("a\tb.go").
func unquote(p string) string {
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		if s, err := strconv.Unquote(p); err == nil {
			return s
		}
	}
	return p
}

// Only returns a copy holding the history of the given paths.
func (h *History) Only(paths map[string]bool) *History {
	c := *h
	c.Files = make(map[string][]Change, len(paths))
	for p, ch := range h.Files {
		if paths[p] {
			c.Files[p] = ch
		}
	}
	return &c
}

// ---------------------------------------------------------------- cache

// cacheFile names the cached history of root at head, read with a commit limit.
func cacheFile(dir, root, head string, maxCommits int) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(dir, fmt.Sprintf("%s-history-%s-%d.json.gz", hex.EncodeToString(sum[:12]), head, maxCommits))
}

// Cached returns the history of root at HEAD, collecting and caching it in dir when
// needed. Histories of older HEADs of the same project are removed.
func Cached(ctx context.Context, dir, root string, maxCommits int) (*History, error) {
	head, err := Head(ctx, root)
	if err != nil {
		return nil, err
	}
	file := cacheFile(dir, root, head, maxCommits)
	if f, err := os.Open(file); err == nil {
		defer f.Close()
		if zr, err := gzip.NewReader(f); err == nil {
			var h History
			if json.NewDecoder(zr).Decode(&h) == nil && h.Head == head {
				return &h, nil
			}
		}
	}
	h, err := Collect(ctx, root, maxCommits)
	if err != nil {
		return nil, err
	}
	if dir != "" {
		save(dir, root, file, h)
	}
	return h, nil
}

func save(dir, root, file string, h *History) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	old, _ := filepath.Glob(strings.Replace(cacheFile(dir, root, "*", 0), "-0.json.gz", "-*.json.gz", 1))
	for _, o := range old {
		os.Remove(o)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if json.NewEncoder(zw).Encode(h) != nil || zw.Close() != nil {
		return
	}
	tmp := file + ".tmp"
	if os.WriteFile(tmp, buf.Bytes(), 0o644) == nil {
		os.Rename(tmp, file)
	}
}
