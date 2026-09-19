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
func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), append([]string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null"}, env...)...)
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
	return dir
}

func TestCollect(t *testing.T) {
	dir := repo(t)
	h, err := Collect(context.Background(), dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	if h.Commits != 4 || h.Truncated || len(h.Authors) != 2 {
		t.Fatalf("commits %d truncated %v authors %v", h.Commits, h.Truncated, h.Authors)
	}
	a := h.Files["app/a.go"]
	if len(a) != 3 {
		t.Fatalf("app/a.go changes: %v", a)
	}
	// Newest first: t=3000 by Ann (+1 -1), commit index 1 (README's commit is 0).
	ann := a[0][1]
	if a[0] != (Change{3000, ann, 1, 1, 1}) || h.Authors[ann] != "Ann Smith" && h.Authors[ann] != "Ann" {
		t.Errorf("latest change %v by %q", a[0], h.Authors[ann])
	}
	if a[2] != (Change{1000, ann, 2, 0, 3}) {
		t.Errorf("first change %v", a[2])
	}
	if png := h.Files["app/logo.png"]; len(png) != 1 || png[0][2] != 0 || png[0][3] != 0 {
		t.Errorf("binary file changes: %v", png)
	}
}

func TestCollectFromSubdirectory(t *testing.T) {
	h, err := Collect(context.Background(), filepath.Join(repo(t), "app"), 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Files["a.go"]; !ok || len(h.Files) != 3 {
		t.Errorf("paths should be relative to the subdirectory and limited to it: %v", keys(h.Files))
	}
	if h.Commits != 3 {
		t.Errorf("commits touching app/: %d, want 3", h.Commits)
	}
}

func TestCollectTruncates(t *testing.T) {
	h, err := Collect(context.Background(), repo(t), 2)
	if err != nil {
		t.Fatal(err)
	}
	if h.Commits != 2 || !h.Truncated {
		t.Errorf("commits %d truncated %v", h.Commits, h.Truncated)
	}
	if _, ok := h.Files["app/b.go"]; ok {
		t.Error("app/b.go was changed in the third-newest commit only")
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
	if only := h1.Only(map[string]bool{"README.md": true}); len(only.Files) != 1 || len(h1.Files) != 4 {
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
