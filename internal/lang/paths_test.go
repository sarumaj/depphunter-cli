package lang

import (
	"reflect"
	"slices"
	"testing"
)

func TestAncestorsWalkUpToTheRoot(t *testing.T) {
	for _, testCase := range []struct {
		walk string
		p    string
		want []string
	}{
		{"Ancestors", "a/b/c.go", []string{"a/b", "a", "."}},
		{"Ancestors", "c.go", []string{"."}},
		{"Ancestors", "a", []string{"."}},
		{"Ancestors", ".", nil},
		{"Ancestors", "/a/b", []string{"/a", "/"}},
		{"Ancestors", "/", nil},
		{"DirectoryAndAncestors", "a/b", []string{"a/b", "a", "."}},
		{"DirectoryAndAncestors", ".", []string{"."}},
		{"DirectoryAndAncestors", "/a", []string{"/a", "/"}},
	} {
		walk := Ancestors
		if testCase.walk == "DirectoryAndAncestors" {
			walk = DirectoryAndAncestors
		}
		if got := slices.Collect(walk(testCase.p)); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("%s(%q) = %q, want %q", testCase.walk, testCase.p, got, testCase.want)
		}
	}
}

func TestNearestFindsTheInnermostEntry(t *testing.T) {
	m := map[string]string{".": "root", "a": "a", "a/b/c": "leaf"}
	for _, testCase := range []struct {
		p    string
		want string
		ok   bool
	}{
		{"a/b/c/x.go", "leaf", true},
		{"a/b/x.go", "a", true},
		{"x.go", "root", true},
		{"a/b/c", "a", true}, // strictly above: a directory is not its own ancestor
	} {
		if got, ok := Nearest(m, testCase.p); got != testCase.want || ok != testCase.ok {
			t.Errorf("Nearest(%q) = %q, %v, want %q, %v", testCase.p, got, ok, testCase.want, testCase.ok)
		}
	}
	if got, ok := NearestAtOrAbove(m, "a/b/c"); got != "leaf" || !ok {
		t.Errorf("NearestAtOrAbove(a/b/c) = %q, %v, want leaf, true", got, ok)
	}
	if got, ok := Nearest(map[string]string{"b": "b"}, "a/x.go"); got != "" || ok {
		t.Errorf("Nearest outside every entry = %q, %v, want \"\", false", got, ok)
	}
	if got, want := Chain(m, "a/b/c/x.go"), []string{"leaf", "a", "root"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Chain = %q, want %q", got, want)
	}
}

func TestShallowestFirstPutsTheRootFirst(t *testing.T) {
	directories := []string{"b/c", "-a", "b", ".", "a"}
	slices.SortFunc(directories, func(a, b string) int {
		if ShallowestFirst(a, b) {
			return -1
		}
		if ShallowestFirst(b, a) {
			return 1
		}
		return 0
	})
	if want := []string{".", "-a", "a", "b", "b/c"}; !reflect.DeepEqual(directories, want) {
		t.Errorf("sorted %q, want %q", directories, want)
	}
}
