package lang

import (
	"maps"
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// TestLayoutListsFilesAndTheDirectoriesAboveThem checks the one rule every
// resolver's layout follows: every directory above a file, at any depth, is
// in Directories, and the top level "." is not.
func TestLayoutListsFilesAndTheDirectoriesAboveThem(t *testing.T) {
	l := LayoutOf([]*scan.File{{Path: "a/b/c.go"}, {Path: "a/d.go"}, {Path: "top.go"}})
	if got, want := slices.Sorted(maps.Keys(l.Files)), []string{"a/b/c.go", "a/d.go", "top.go"}; !slices.Equal(got, want) {
		t.Errorf("Files = %v, want %v", got, want)
	}
	if got, want := slices.Sorted(maps.Keys(l.Directories)), []string{"a", "a/b"}; !slices.Equal(got, want) {
		t.Errorf("Directories = %v, want %v", got, want)
	}
	for p, want := range map[string]bool{"a": true, "a/b/c.go": true, ".": false, "a/c": false} {
		if got := l.Has(p); got != want {
			t.Errorf("Has(%q) = %v, want %v", p, got, want)
		}
	}
}

// TestClimbsOut checks the escape test that Inside builds on: only ".." as a
// whole first segment climbs out.
func TestClimbsOut(t *testing.T) {
	for p, want := range map[string]bool{"..": true, "../a": true, "..a": false, "a/..": false, ".": false, "/a": false} {
		if got := ClimbsOut(p); got != want {
			t.Errorf("ClimbsOut(%q) = %v, want %v", p, got, want)
		}
	}
}
