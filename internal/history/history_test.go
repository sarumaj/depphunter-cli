package history

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repo creates a git repository with four commits by two authors at fixed times:
//
//	t=1000 ann  app/a.go (+2)                 t=3000 ann  app/a.go (+1 -1), app/logo.png (binary)
//	t=2000 bob  app/a.go (+1), app/b.go (+1)  t=4000 bob  README.md (+1)
//	t=5000 bob  renames app/b.go to app/core/b2.go
func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	empty := filepath.Join(t.TempDir(), "gitconfig")
	os.WriteFile(empty, nil, 0o644)
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// An empty config file isolates the test from the user's and system's git settings
		// (/dev/null is not portable to Windows).
		cmd.Env = append(os.Environ(), append([]string{"GIT_CONFIG_GLOBAL=" + empty, "GIT_CONFIG_SYSTEM=" + empty}, env...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(p, content string) {
		os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), []byte(content), 0o644)
	}
	commit := func(when, name, email string) {
		env := []string{"GIT_AUTHOR_DATE=@" + when + " +0000", "GIT_COMMITTER_DATE=@" + when + " +0000",
			"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + email}
		run(env, "add", "-A")
		run(env, "commit", "-q", "-m", "c"+when)
	}
	run(nil, "init", "-q", "-b", "main")
	write("app/a.go", "1\n2\n")
	commit("1000", "Ann", "ann@example.com")
	write("app/a.go", "1\n2\n3\n")
	write("app/b.go", "x\n")
	commit("2000", "Bob", "bob@example.com")
	write("app/a.go", "1\n2\nthree\n")
	write("app/logo.png", "\x89PNG\x00\x01")
	commit("3000", "Ann Smith", "ANN@example.com") // same person, new display name
	write("README.md", "hi\n")
	commit("4000", "Bob", "bob@example.com")
	os.MkdirAll(filepath.Join(dir, "app", "core"), 0o755)
	run(nil, "mv", "app/b.go", "app/core/b2.go")
	commit("5000", "Bob", "bob@example.com")
	return dir
}

// Verifies: REQ-HIST-001, REQ-HIST-003, REQ-HIST-004, REQ-HIST-006
func TestCollect(t *testing.T) {
	dir := repo(t)
	h, err := Collect(context.Background(), dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if h.Commits != 5 || h.Truncated || len(h.Authors) != 2 {
		t.Fatalf("commits %d truncated %v authors %v", h.Commits, h.Truncated, h.Authors)
	}
	a := h.Files["app/a.go"]
	if len(a) != 3 {
		t.Fatalf("app/a.go changes: %v", a)
	}
	// Newest first: t=3000 by Ann (+1 -1), commit index 2 (the rename is 0, README 1).
	ann := a[0][1]
	if a[0] != (Change{3000, ann, 1, 1, 2}) || h.Authors[ann] != "Ann Smith" && h.Authors[ann] != "Ann" {
		t.Errorf("latest change %v by %q", a[0], h.Authors[ann])
	}
	if a[2] != (Change{1000, ann, 2, 0, 4}) {
		t.Errorf("first change %v", a[2])
	}
	if png := h.Files["app/logo.png"]; len(png) != 1 || png[0][2] != 0 || png[0][3] != 0 {
		t.Errorf("binary file changes: %v", png)
	}
	// The renamed file keeps the history of its old name.
	if b := h.Files["app/core/b2.go"]; len(b) != 2 || b[1][0] != 2000 {
		t.Errorf("renamed file history: %v", b)
	}
	if _, ok := h.Files["app/b.go"]; ok {
		t.Error("history left under the old name")
	}
}

// Verifies: REQ-HIST-006
func TestRenamePaths(t *testing.T) {
	for in, want := range map[string][2]string{
		"a.go":                    {"", "a.go"},
		"old.go => new.go":        {"old.go", "new.go"},
		"src/{a => b}/f.go":       {"src/a/f.go", "src/b/f.go"},
		"src/{ => sub}/f.go":      {"src/f.go", "src/sub/f.go"},
		"{lib => pkg}/x/y.go":     {"lib/x/y.go", "pkg/x/y.go"},
		"docs/{old.md => new.md}": {"docs/old.md", "docs/new.md"},
	} {
		if o, n := renamePaths(in); o != want[0] || n != want[1] {
			t.Errorf("%q: got %q, %q; want %q, %q", in, o, n, want[0], want[1])
		}
	}
}

// Verifies: REQ-HIST-001
func TestCollectFromSubdirectory(t *testing.T) {
	h, err := Collect(context.Background(), filepath.Join(repo(t), "app"), 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Files["a.go"]; !ok || len(h.Files) != 3 { // a.go, core/b2.go, logo.png
		t.Errorf("paths should be relative to the subdirectory and limited to it: %v", keys(h.Files))
	}
	if h.Commits != 4 {
		t.Errorf("commits touching app/: %d, want 4", h.Commits)
	}
}

// Verifies: REQ-HIST-002
func TestCollectTruncates(t *testing.T) {
	h, err := Collect(context.Background(), repo(t), 2)
	if err != nil {
		t.Fatal(err)
	}
	if h.Commits != 2 || !h.Truncated {
		t.Errorf("commits %d truncated %v", h.Commits, h.Truncated)
	}
	if _, ok := h.Files["app/core/b2.go"]; !ok || len(h.Files) != 2 {
		t.Errorf("only the rename and README commits were read: %v", keys(h.Files))
	}
}

func TestNoHistory(t *testing.T) {
	if _, err := Collect(context.Background(), t.TempDir(), 10); !errors.Is(err, ErrNoHistory) {
		t.Errorf("plain directory: %v", err)
	}
	empty := t.TempDir()
	exec.Command("git", "-C", empty, "init", "-q").Run()
	if _, err := Collect(context.Background(), empty, 10); !errors.Is(err, ErrNoHistory) {
		t.Errorf("repository without commits: %v", err)
	}
}

// Verifies: REQ-HIST-005
func TestCached(t *testing.T) {
	dir, cache := repo(t), t.TempDir()
	h1, err := Cached(context.Background(), cache, dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(cache, "*-history-*.json.gz"))
	if len(files) != 1 || !strings.Contains(files[0], h1.Head) {
		t.Fatalf("cache files: %v", files)
	}
	h2, err := Cached(context.Background(), cache, dir, 100)
	if err != nil || h2.Commits != h1.Commits || len(h2.Files) != len(h1.Files) {
		t.Errorf("cached history differs: %v", err)
	}
	if only := h1.Only(map[string]bool{"README.md": true}); len(only.Files) != 1 || len(h1.Files) != 4 { // a, b2, logo, README
		t.Errorf("Only: %v (original %d files)", keys(only.Files), len(h1.Files))
	}
}

func keys(m map[string][]Change) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
