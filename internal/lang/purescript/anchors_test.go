package purescript

import (
	"slices"
	"testing"
)

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

// An extra package's dependencies are read as a package's own are: names and
// name: range entries alike, an aliased entry resolved.
//
// Verifies: REQ-LANG-032, REQ-PURESCRIPT-008
func TestExtraPackageDependencies(t *testing.T) {
	m := readSpagoYAML([]byte(`workspace:
  extraPackages:
    widgets:
      git: https://github.com/example/widgets.git
      ref: v1.2.0
      dependencies:
        - prelude
        - &effect effect: ">=4.0.0 <5.0.0"
        - *effect
        - arrays: "*"
`))
	if m == nil || len(m.extra) != 1 {
		t.Fatalf("readSpagoYAML: %+v", m)
	}
	want := []string{"prelude", "effect", "effect", "arrays"}
	if got := m.extra[0].dependencies; !slices.Equal(got, want) {
		t.Errorf("dependencies %q, want %q", got, want)
	}
}
