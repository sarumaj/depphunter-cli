package gleam

import (
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// A manifest.toml the scan lists says nothing; one read from disk because the scan
// left it out (a library's git-ignored lock) is noted.
//
// Verifies: REQ-TRC-017
func TestManifestNotes(t *testing.T) {
	root := "testdata/repo"
	files := langtest.Files(t, root)
	if got := langtest.Notes(newResolver(root, files)); len(got) != 0 {
		t.Errorf("listed: notes %q", got)
	}
	got := langtest.Notes(newResolver(root, langtest.Without(files, "manifest.toml")))
	if want := []string{"manifest.toml lock-ignored"}; !slices.Equal(got, want) {
		t.Errorf("ignored: notes %q, want %q", got, want)
	}
}
