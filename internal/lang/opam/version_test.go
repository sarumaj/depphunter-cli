package opam

import (
	"reflect"
	"testing"
)

// opam's version order: digits as numbers, `~` before everything (the end
// included), letters before other characters.
//
// Verifies: REQ-SUP-054
func TestCompareVersions(t *testing.T) {
	for _, tt := range [][2]string{
		{"1.9", "1.10"}, {"1.0~beta1", "1.0"}, {"1.0~alpha", "1.0~beta"}, {"1.0", "1.0.1"},
		{"1.0", "1.0a"}, {"1.0a", "1.0+b"}, {"v0.16.0", "v0.17.0"}, {"5.9.1", "5.10.0"}, {"2.0~~", "2.0~"},
	} {
		if CompareVersions(tt[0], tt[1]) >= 0 || CompareVersions(tt[1], tt[0]) <= 0 {
			t.Errorf("%s should come before %s", tt[0], tt[1])
		}
	}
	if CompareVersions("1.01", "1.1") != 0 || CompareVersions("1.0", "1.0") != 0 {
		t.Error("equal versions")
	}
}

// A constraint as Dep.Constraint writes it: `&` binds tighter than `|`, a
// variable is met, and the newest version admitted is chosen.
//
// Verifies: REQ-SUP-054
func TestSatisfies(t *testing.T) {
	for _, tt := range []struct {
		version, constraint string
		want                bool
	}{
		{"5.7", ">= 5.6 & < 6", true},
		{"6.0", ">= 5.6 & < 6", false},
		{"6~beta", "< 6", true},
		{"6.0~beta", "< 6", false},
		{"0.16", ">= 0.17 | = 0.16", true},
		{"0.15", ">= 0.17 | = 0.16", false},
		{"1.2", "!= 1.2", false},
		{"1.2", "= version", true},
		{"1.2", "", true},
		{"1.2", "> 1.1 & <= 1.2", true},
	} {
		if got := Satisfies(tt.version, tt.constraint); got != tt.want {
			t.Errorf("Satisfies(%q, %q) = %v", tt.version, tt.constraint, got)
		}
	}
	if got := Newest([]string{"1.2", "1.10", "1.9", "2.0~rc"}, "< 2.0~rc"); got != "1.10" {
		t.Errorf("Newest %q", got)
	}
	if got := Newest([]string{"1.2"}, "> 2"); got != "" {
		t.Errorf("Newest of none %q", got)
	}
}

// opam's configuration files: repos-config's names with URLs (trust anchors
// after the URL ignored), the priority lists of config and switch-config (one
// string or a list), and a one-string field.
//
// Verifies: REQ-SUP-054
func TestRepositories(t *testing.T) {
	src := []byte(`opam-version: "2.0"
repositories: [
  "default" {"https://opam.ocaml.org"}
  "corp" {"git+ssh://git.corp/r.git" ["fp1" "fp2"] 2}
]
switch: "5.1" # the default switch
`)
	want := []Repository{{"default", "https://opam.ocaml.org"}, {"corp", "git+ssh://git.corp/r.git"}}
	if got := Repositories(src); !reflect.DeepEqual(got, want) {
		t.Errorf("repos-config %+v", got)
	}
	if got := Repositories([]byte(`repositories: "default"` + "\n" + `depends: ["x"]`)); !reflect.DeepEqual(got, []Repository{{Name: "default"}}) {
		t.Errorf("one name %+v", got)
	}
	if got := Repositories([]byte(`repositories: ["a" "b"]`)); !reflect.DeepEqual(got, []Repository{{Name: "a"}, {Name: "b"}}) {
		t.Errorf("list %+v", got)
	}
	if got := String(src, "switch"); got != "5.1" {
		t.Errorf("switch %q", got)
	}
	if got := String(src, "missing"); got != "" {
		t.Errorf("missing %q", got)
	}
}
