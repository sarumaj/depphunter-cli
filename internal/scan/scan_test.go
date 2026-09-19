package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
