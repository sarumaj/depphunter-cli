package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
