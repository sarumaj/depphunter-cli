package solidity

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// Verifies: REQ-LANG-033
func TestFoundryDependencyLines(t *testing.T) {
	for _, testCase := range []struct {
		name, source string
		want         map[string]int
	}{
		{"comment after the header", "[profile.default]\nsrc = \"src\"\n\n[dependencies] # soldeer\nforge-std = \"1.9.1\"\n", map[string]int{"forge-std": 5}},
		{"top-level dotted key", "# soldeer\n\ndependencies.forge-std = \"1.9.1\"\n", map[string]int{"forge-std": 3}},
	} {
		if got := langtest.ImportLines(extractFoundry([]byte(testCase.source))); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s: lines %v, want %v", testCase.name, got, testCase.want)
		}
	}
}
