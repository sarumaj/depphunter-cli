package fortran

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// A top-level dotted key names its dependency whole: foobar is not foo.
//
// Verifies: REQ-LANG-033
func TestFpmDottedDependencyLines(t *testing.T) {
	source := "name = \"app\"\ndependencies.foobar.git = \"https://example.com/foobar\"\ndependencies.foo.git = \"https://example.com/foo\"\n"
	lines := langtest.ImportLines(extractManifest([]byte(source)))
	if lines["foo"] != 3 || lines["foobar"] != 2 {
		t.Errorf("lines %v, want foo on 3 and foobar on 2", lines)
	}
}
