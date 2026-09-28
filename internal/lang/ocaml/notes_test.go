package ocaml

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// opam's lock pins without edges: noted unless dune's lock directory, which has
// them, is on disk.
//
// Verifies: REQ-OCAML-009, REQ-TRC-017
func TestOpamLockNotes(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"app.opam":        "opam-version: \"2.0\"\ndepends: [ \"lwt\" ]\n",
		"app.opam.locked": "opam-version: \"2.0\"\ndepends: [ \"lwt\" {= \"5.7.0\"} ]\n",
		"bin/main.ml":     "let () = Lwt_main.run (Lwt.return ())\n",
	})
	files := langtest.Files(t, root)
	if got, want := langtest.Notes(newResolver(root, files)), []string{"app.opam.locked lock-flat"}; !slices.Equal(got, want) {
		t.Errorf("notes %q, want %q", got, want)
	}
	lock := filepath.Join(root, "dune.lock")
	if err := os.MkdirAll(lock, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lock, "lwt.pkg"), []byte("(version 5.7.0)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := langtest.Notes(newResolver(root, files)); len(got) != 0 {
		t.Errorf("with dune.lock: notes %q", got)
	}
}
