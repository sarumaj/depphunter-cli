package ada

import "testing"

// Alire's semantic versions: numbers, a pre-release before its release, build
// metadata ignored.
//
// Verifies: REQ-SUP-061
func TestCompareVersions(t *testing.T) {
	for _, test := range [][2]string{
		{"1.9.0", "1.10.0"}, {"1.0.0-rc1", "1.0.0"}, {"1.0.0-alpha", "1.0.0-alpha.1"}, {"1.0.0-alpha.2", "1.0.0-alpha.10"},
		{"1.0.0-2", "1.0.0-alpha"}, {"1.0", "1.0.1"}, {"24.0.0", "25.0.0-rc"},
	} {
		if CompareVersions(test[0], test[1]) >= 0 || CompareVersions(test[1], test[0]) <= 0 {
			t.Errorf("%s should come before %s", test[0], test[1])
		}
	}
	if CompareVersions("1.0.0+build", "1.0.0") != 0 || CompareVersions("1.0", "1.0.0") != 0 {
		t.Error("equal versions")
	}
}

// Alire's constraints: operators (Unicode ones too), ^ up to the next major
// (0.x included) and ~ up to the next minor, & and |, parentheses; a commit or a
// branch is no constraint.
//
// Verifies: REQ-SUP-061
func TestSatisfies(t *testing.T) {
	for _, test := range []struct {
		version, constraint string
		want                bool
	}{
		{"1.4.2", "^1.0", true},
		{"2.0.0", "^1.0", false},
		{"2.0.0-rc1", "^1.0", false},
		{"0.9.0", "^0.2", true},
		{"1.2.9", "~1.2.3", true},
		{"1.3.0", "~1.2.3", false},
		{"1.2.2", "~1.2.3", false},
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "=1.2.4", false},
		{"1.2.3", "/=1.2.3", false},
		{"1.2.3", "≠1.2.4", true},
		{"1.2.3", "≥1.2 & ≤1.3", true},
		{"3.0.0", "(^1.0 | ^3.0) & /=3.0.1", true},
		{"3.0.1", "(^1.0 | ^3.0) & /=3.0.1", false},
		{"5.0.0", "*", true},
		{"5.0.0", "any", true},
		{"5.0.0", "", true},
		{"5.0.0", "main", false},
	} {
		if got := Satisfies(test.version, test.constraint); got != test.want {
			t.Errorf("Satisfies(%q, %q) = %v", test.version, test.constraint, got)
		}
	}
	if got := Newest([]string{"25.1.0", "26.0.0-rc1", "25.2.0", "external"}, "*"); got != "25.2.0" {
		t.Errorf("Newest %q", got)
	}
	if got := Newest([]string{"25.1.0", "26.0.0-rc1"}, ">25.1"); got != "26.0.0-rc1" {
		t.Errorf("Newest pre-release %q", got)
	}
	for c, want := range map[string]bool{"^1.0 & /=1.2": true, "*": true, "": true, "(>=1 | <0.5)": true,
		"73d99ae1ff2f5210dc41c2ea7afebe600f9e9916": false, "main": false, "^x": false} {
		if ValidConstraint(c) != want {
			t.Errorf("ValidConstraint(%q) != %v", c, want)
		}
	}
}
