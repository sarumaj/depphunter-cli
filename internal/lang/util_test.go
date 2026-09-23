package lang

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

func TestForEachFileSkipsWhatItWouldNotParse(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.go")
	os.WriteFile(small, []byte("package a\n"), 0o644)
	// Measured as larger than MaxParseSize, but not on disk: a file that is read
	// anyway would be found and passed to fn.
	big := filepath.Join(dir, "big.go")
	os.WriteFile(big, []byte("package b\n"), 0o644)
	files := []*scan.File{
		{Path: "small.go", Abs: small, LOC: 1, Size: 10},
		{Path: "big.go", Abs: big, LOC: 1, Size: MaxParseSize + 1},
		{Path: "bin.go", Abs: small, Binary: true, Size: 10},
		{Path: "over.go", Abs: small, LOC: 1, Size: 10, TooLarge: true}, // over --max-file-size
	}
	got := ForEachFile(context.Background(), files, func(f *scan.File, _ []byte) *FileResult {
		return &FileResult{}
	})
	if len(got) != 1 || got["small.go"] == nil {
		t.Errorf("parsed %v, want small.go alone", got)
	}
}
