package lua

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// Verifies: REQ-LANG-033
func TestWallyDependencyLines(t *testing.T) {
	for _, testCase := range []struct {
		name, source string
		want         map[string]int
	}{
		{"tab before =", "[dependencies]\nRoact\t= \"roblox/roact@1.4.0\"\n", map[string]int{"roblox/roact": 2}},
		{"comment after the header", "[package]\nname = \"a/b\"\n\n[dependencies] # runtime\nRoact = \"roblox/roact@1.4.0\"\n", map[string]int{"roblox/roact": 5}},
		{"literal key", "[dependencies]\n'Roact' = \"roblox/roact@1.4.0\"\n", map[string]int{"roblox/roact": 2}},
	} {
		if got := langtest.ImportLines(extractWally([]byte(testCase.source))); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s: lines %v, want %v", testCase.name, got, testCase.want)
		}
	}
}
