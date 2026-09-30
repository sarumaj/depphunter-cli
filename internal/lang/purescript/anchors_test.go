package purescript

import "testing"

// A dependency's range given by an anchor is read as the range.
//
// Verifies: REQ-LANG-032
func TestSpagoAnchors(t *testing.T) {
	m := readSpagoYAML([]byte("package:\n  name: app\n  dependencies:\n    - prelude: &range \">=6.0.0 <7.0.0\"\n    - effect: *range\n"))
	if m == nil || len(m.dependencies) != 2 {
		t.Fatalf("readSpagoYAML: %+v", m)
	}
	if d := m.dependencies[1]; d.name != "effect" || d.versionRange != ">=6.0.0 <7.0.0" {
		t.Errorf("effect: %+v", d)
	}
}
