package graph

import (
	"slices"
	"testing"
)

// Of yields the nodes of one kind in the graph's order, and stops when the loop
// does.
func TestOfYieldsOneKindInOrder(t *testing.T) {
	g := &Graph{Nodes: []*Node{
		{ID: FileID("b.go"), Kind: KindFile},
		{ID: DirectoryID("src"), Kind: KindDirectory},
		{ID: FileID("a.go"), Kind: KindFile},
		{ID: SymbolID("a.go", "F"), Kind: KindSymbol},
		{ID: FileID("c.go"), Kind: KindFile},
	}}
	var ids []string
	for n := range g.Of(KindFile) {
		ids = append(ids, n.ID)
	}
	if want := []string{"f:b.go", "f:a.go", "f:c.go"}; !slices.Equal(ids, want) {
		t.Errorf("Of(KindFile) = %q, want %q", ids, want)
	}
	ids = nil
	for n := range g.Of(KindFile) {
		ids = append(ids, n.ID)
		break
	}
	if want := []string{"f:b.go"}; !slices.Equal(ids, want) {
		t.Errorf("Of(KindFile) with break = %q, want %q", ids, want)
	}
	for n := range g.Of(KindPackage) {
		t.Errorf("Of(KindPackage) yielded %q from a graph without packages", n.ID)
	}
}

// EcosystemOf and FileOf undo EcosystemID and FileID, and name nothing for the
// other kinds of ID.
//
// Verifies: REQ-MOD-003
func TestIDInverses(t *testing.T) {
	for _, ecosystem := range []string{"go", "npm", "go-std", ""} {
		if got := EcosystemOf(EcosystemID(ecosystem)); got != ecosystem {
			t.Errorf("EcosystemOf(EcosystemID(%q)) = %q", ecosystem, got)
		}
	}
	for _, path := range []string{"main.go", "src/a:b.ts", ""} {
		if got := FileOf(FileID(path)); got != path {
			t.Errorf("FileOf(FileID(%q)) = %q", path, got)
		}
	}
	for _, id := range []string{DirectoryID("src"), SymbolID("main.go", "main"), PackageID("go", "example.com/m"), "go", ""} {
		if got := EcosystemOf(id); got != "" {
			t.Errorf("EcosystemOf(%q) = %q, want \"\"", id, got)
		}
		if got := FileOf(id); got != "" {
			t.Errorf("FileOf(%q) = %q, want \"\"", id, got)
		}
	}
}
