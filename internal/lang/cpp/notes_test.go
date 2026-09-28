package cpp

import (
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// A Conan 2 lock (conan-txt) pins without edges and is noted; a Conan 1 lock
// (conan-py), whose graph the walk follows, is not.
//
// Verifies: REQ-CPP-011, REQ-TRC-017
func TestConanLockNotes(t *testing.T) {
	r := newResolver(t.TempDir(), langtest.Files(t, "testdata/pkgs"))
	if got, want := langtest.Notes(r), []string{"conan-txt/conan.lock lock-flat"}; !slices.Equal(got, want) {
		t.Errorf("notes %q, want %q", got, want)
	}
}
