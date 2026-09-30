package julia

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// Verifies: REQ-LANG-033
func TestProjectDependencyLines(t *testing.T) {
	source := "name = \"App\"\ndeps.Foo = \"7876af07-990d-54b4-ab0e-23690620f79a\"\n\n[weakdeps]\n'Bar' = \"8f4d0f93-b110-5947-807f-2305c1781a2d\"\n"
	want := map[string]int{"Foo": 2, "Bar": 5}
	if got := langtest.ImportLines(extractProject([]byte(source))); !reflect.DeepEqual(got, want) {
		t.Errorf("lines %v, want %v", got, want)
	}
}
