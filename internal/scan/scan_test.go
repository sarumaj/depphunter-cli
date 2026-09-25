package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Verifies: REQ-LANG-015, REQ-LANG-016, REQ-LANG-018, REQ-LANG-019, REQ-LANG-021
func TestScanMeasuresAndExcludes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"a.go":                "package a\n\nfunc A() {}\n",
		"no_newline.py":       "x = 1\ny = 2",
		"gen/skip.go":         "package gen\n",
		"node_modules/x/i.js": "ignored by default when git is unavailable\n",
		"img.bin":             "\x00\x01\x02",
	}
	for p, c := range files {
		abs := filepath.Join(root, p)
		os.MkdirAll(filepath.Dir(abs), 0o755)
		os.WriteFile(abs, []byte(c), 0o644)
	}

	got, err := Scan(context.Background(), root, Options{Exclude: []string{"gen"}})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]*File{}
	for _, f := range got {
		byPath[f.Path] = f
	}
	if len(byPath) != 3 {
		t.Fatalf("got files %v, want a.go, no_newline.py, img.bin", byPath)
	}
	if f := byPath["a.go"]; f.LOC != 3 || f.Lang != "Go" {
		t.Errorf("a.go: %+v", f)
	}
	if f := byPath["no_newline.py"]; f.LOC != 2 || f.Lang != "Python" {
		t.Errorf("no_newline.py: %+v", f)
	}
	if f := byPath["img.bin"]; !f.Binary || f.LOC != 0 {
		t.Errorf("img.bin: %+v", f)
	}
}

func TestScanSkipsSymlinksInGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(outside, []byte("not part of the repository\n"), 0o644)
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "notes.txt")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if f.Path == "notes.txt" {
			t.Errorf("a tracked symlink to %s was scanned as a file", outside)
		}
	}
	if len(got) != 1 {
		t.Errorf("got %d files, want a.go alone", len(got))
	}
}

// Verifies: REQ-LANG-018
func TestWalkSkipsUnreadableDirectories(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)
	locked := filepath.Join(root, "locked")
	os.MkdirAll(locked, 0o755)
	os.WriteFile(filepath.Join(locked, "b.go"), []byte("package b\n"), 0o644)
	if err := os.Chmod(locked, 0); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("the directory is readable anyway (running as root, or on Windows)")
	}

	got, err := walkFiles(context.Background(), root)
	if err != nil {
		t.Fatalf("one unreadable directory failed the walk: %v", err)
	}
	if len(got) != 1 || got[0] != "a.go" {
		t.Errorf("got %v, want [a.go]", got)
	}
}

// git decides what a repository contains: whatever .gitignore excludes is left out,
// an untracked file nothing ignores is kept, and a file in conflict - which git lists
// once per merge stage - comes back once.
//
// Verifies: REQ-LANG-017
func TestScanListsWhatGitDoes(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		// No user configuration is assumed: identity and signing are set here.
		base := []string{"-C", root, "-c", "user.name=test", "-c", "user.email=test@example.com",
			"-c", "commit.gpgsign=false", "-c", "core.autocrlf=false"}
		if out, err := exec.Command("git", append(base, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(p, content string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(abs), 0o755)
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write(".gitignore", "*.log\nout/\n")
	write("conflict.txt", "base\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "other")
	write("conflict.txt", "other\n")
	git("commit", "-q", "-am", "other")
	git("checkout", "-q", "-")
	write("conflict.txt", "mine\n")
	git("commit", "-q", "-am", "mine")
	// The merge fails by design, leaving conflict.txt in three stages.
	exec.Command("git", "-C", root, "-c", "user.name=test", "-c", "user.email=test@example.com",
		"merge", "-q", "other").Run()

	write("debug.log", "ignored\n")
	write("out/app.bin", "ignored\n")
	write("new.go", "package a\n") // untracked, and nothing ignores it

	got, err := Scan(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, f := range got {
		count[f.Path]++
	}
	want := map[string]int{".gitignore": 1, "conflict.txt": 1, "new.go": 1}
	if !reflect.DeepEqual(count, want) {
		t.Errorf("scanned %v, want %v", count, want)
	}
}

// Verifies: REQ-LANG-020
func TestScanMeasuresEveryFilesSize(t *testing.T) {
	// Bytes are measured whether or not the lines are counted: a text file, a binary
	// and a file over the size limit all carry their size on disk.
	root := t.TempDir()
	files := map[string]string{
		"a.go":    "package a\n\nfunc A() {}\n",
		"img.bin": "\x00\x01\x02\x03",
		"big.txt": strings.Repeat("x", 100) + "\n",
	}
	for p, c := range files {
		os.WriteFile(filepath.Join(root, p), []byte(c), 0o644)
	}
	got, err := Scan(context.Background(), root, Options{MaxFileSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(files) {
		t.Fatalf("got %d files, want %d", len(got), len(files))
	}
	for _, f := range got {
		st, err := os.Stat(f.Abs)
		if err != nil {
			t.Fatal(err)
		}
		if f.Size != st.Size() || f.Size != int64(len(files[f.Path])) {
			t.Errorf("%s: size %d, want %d", f.Path, f.Size, st.Size())
		}
	}
}
