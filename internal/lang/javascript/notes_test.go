package javascript

import (
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// bun.lockb is not read, and says so - unless a bun.lock or a yarn.lock beside it
// is read in its place.
//
// Verifies: REQ-JS-017, REQ-TRC-017
func TestBunLockbNotes(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"package.json":      `{"dependencies": {"react": "^18"}}`,
		"bun.lockb":         "\x00",
		"yarn/package.json": `{"dependencies": {"react": "^18"}}`,
		"yarn/bun.lockb":    "\x00",
		"yarn/yarn.lock":    "react@^18:\n  version \"18.3.1\"\n",
		"text/package.json": `{"dependencies": {"react": "^18"}}`,
		"text/bun.lockb":    "\x00",
		"text/bun.lock":     `{"lockfileVersion": 1, "workspaces": {"": {"name": "text"}}, "packages": {}}`,
	})
	got := langtest.Notes(newResolver(langtest.Files(t, root)))
	if want := []string{"bun.lockb lock-unread"}; !slices.Equal(got, want) {
		t.Errorf("notes %q, want %q", got, want)
	}
}
