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
//
// Implements: REQ-HIST-003
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

// GitDirectories returns the directories whose changes signal a new HEAD: the git directory
// (HEAD, packed-refs) and refs/heads (branch tips). Nil outside a work tree.
//
// Implements: REQ-HIST-009
func GitDirectories(ctx context.Context, root string) []string {
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return nil
	}
	directory := strings.TrimSpace(string(out))
	return []string{directory, filepath.Join(directory, "refs", "heads")}
}

// Collect reads at most maxCommits non-merge commits reachable from HEAD. Paths are
// relative to root (which may be a subdirectory of the repository). Renames are
// followed: changes made under an old name are filed under the file's current path.
//
// Implements: REQ-HIST-001, REQ-HIST-002, REQ-HIST-003, REQ-HIST-004, REQ-HIST-006
func Collect(ctx context.Context, root string, maxCommits int) (*History, error) {
	head, err := Head(ctx, root)
	if err != nil {
		return nil, err
	}
	// \x1e starts a commit header; \x1f separates its fields.
	command := exec.CommandContext(ctx, "git", "-C", root, "log", "--no-merges", "-M", "--relative",
		"--numstat", "--format=%x1e%at%x1f%aN%x1f%aE", "-n", strconv.Itoa(maxCommits+1), "HEAD",
		"--", ".") // only commits touching root; --relative alone still lists the others
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
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
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	for scanner.Scan() {
		line := scanner.Text()
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
			index, ok := authors[key]
			if !ok {
				index = len(h.Authors)
				authors[key] = index
				h.Authors = append(h.Authors, fields[1])
			}
			author = int64(index)
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
	if err := scanner.Err(); err != nil {
		command.Process.Kill()
		command.Wait()
		return nil, err
	}
	if h.Truncated {
		command.Process.Kill() // we stopped reading early on purpose
		command.Wait()
		return h, nil
	}
	if err := command.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("git log: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return h, nil
}

// renamePaths splits a numstat rename, "old => new" or "dir/{old => new}/file",
// into its paths; old is "" for a plain path.
//
// Implements: REQ-HIST-006
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
	for p, changes := range h.Files {
		if paths[p] {
			c.Files[p] = changes
		}
	}
	return &c
}

// ---------------------------------------------------------------- cache

// cacheFile names the cached history of root at head, read with a commit limit.
func cacheFile(directory, root, head string, maxCommits int) string {
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(directory, fmt.Sprintf("%s-history-%s-%d.json.gz", hex.EncodeToString(sum[:12]), head, maxCommits))
}

// Cached returns the history of root at HEAD, collecting and caching it in directory when
// needed. Histories of older HEADs of the same project are removed. An empty directory
// disables the cache: the history is always collected, and nothing is read or written.
//
// Implements: REQ-HIST-005
func Cached(ctx context.Context, directory, root string, maxCommits int) (*History, error) {
	head, err := Head(ctx, root)
	if err != nil {
		return nil, err
	}
	file := cacheFile(directory, root, head, maxCommits)
	if directory != "" {
		if f, err := os.Open(file); err == nil {
			defer f.Close()
			if gzipReader, err := gzip.NewReader(f); err == nil {
				var h History
				if json.NewDecoder(gzipReader).Decode(&h) == nil && h.Head == head {
					return &h, nil
				}
			}
		}
	}
	h, err := Collect(ctx, root, maxCommits)
	if err != nil {
		return nil, err
	}
	if directory != "" {
		save(directory, root, file, h)
	}
	return h, nil
}

func save(directory, root, file string, h *History) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return
	}
	old, _ := filepath.Glob(strings.Replace(cacheFile(directory, root, "*", 0), "-0.json.gz", "-*.json.gz", 1))
	for _, o := range old {
		os.Remove(o)
	}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	if json.NewEncoder(gzipWriter).Encode(h) != nil || gzipWriter.Close() != nil {
		return
	}
	temporary := file + ".tmp"
	if os.WriteFile(temporary, buffer.Bytes(), 0o644) == nil {
		os.Rename(temporary, file)
	}
}
