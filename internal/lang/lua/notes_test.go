package lua

import (
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// luarocks.lock pins without edges, which is noted, as is one read from disk that
// the scan left out.
//
// Verifies: REQ-LUA-007, REQ-TRC-017
func TestLuarocksLockNotes(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"app-1.0-1.rockspec": "package = \"app\"\nversion = \"1.0-1\"\ndependencies = { \"luasocket >= 3.0\" }\n",
		"luarocks.lock":      "return {\n  dependencies = {\n    luasocket = \"3.1.0-1\",\n  },\n}\n",
		"src/app.lua":        "local socket = require(\"socket\")\n",
	})
	files := langtest.Files(t, root)
	if got, want := langtest.Notes(newResolver(root, files)), []string{"luarocks.lock lock-flat"}; !slices.Equal(got, want) {
		t.Errorf("notes %q, want %q", got, want)
	}
	got := langtest.Notes(newResolver(root, langtest.Without(files, "luarocks.lock")))
	if want := []string{"luarocks.lock lock-flat", "luarocks.lock lock-ignored"}; !slices.Equal(got, want) {
		t.Errorf("ignored: notes %q, want %q", got, want)
	}
}
