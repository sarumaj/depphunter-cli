package gleam

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// Verifies: REQ-LANG-033
func TestGleamDependencyLines(t *testing.T) {
	for _, testCase := range []struct {
		name, source string
		want         map[string]int
	}{
		{"same key in another table", "name = \"app\"\nversion = \"1.0.0\"\n\n[dependencies]\nversion = \">= 1.0.0\"\n", map[string]int{"version": 5}},
		{"key inside a string", "name = \"app\"\ndescription = \"a, path = b\"\n\n[dependencies]\npath = \">= 1.0.0\"\n", map[string]int{"path": 5}},
		{"key inside an inline table", "name = \"app\"\n\n[dependencies]\nlocal = { path = \"../local\" }\npath = \">= 1.0.0\"\n", map[string]int{"local": 4, "path": 5}},
	} {
		if got := langtest.ImportLines(extractConfig([]byte(testCase.source))); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s: lines %v, want %v", testCase.name, got, testCase.want)
		}
	}
}
