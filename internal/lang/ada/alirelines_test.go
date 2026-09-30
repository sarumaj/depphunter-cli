package ada

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// Verifies: REQ-LANG-033
func TestAlireDependencyLines(t *testing.T) {
	source := "name = \"app\"\n\n[[depends-on]]\naunit.version = \"^24\"\n'case(os)'.windows.win32ada = \"*\"\n"
	want := map[string]int{"aunit": 4, "win32ada": 5}
	if got := langtest.ImportLines(extractManifest([]byte(source))); !reflect.DeepEqual(got, want) {
		t.Errorf("lines %v, want %v", got, want)
	}
}
