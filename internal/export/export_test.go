package export

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/graph"
)

func sample() *graph.Graph {
	return &graph.Graph{
		Root: `odd "repo"`,
		Nodes: []*graph.Node{
			{ID: "d:.", Kind: graph.KindDir, Name: "repo", Path: "."},
			{ID: "d:pkg", Kind: graph.KindDir, Name: "pkg", Path: "pkg", Parent: "d:."},
			{ID: "f:main.go", Kind: graph.KindFile, Name: "main.go", Path: "main.go", Parent: "d:.", Lang: "Go", LOC: 10},
			{ID: "f:pkg/a.go", Kind: graph.KindFile, Name: "a.go", Path: "pkg/a.go", Parent: "d:pkg", Lang: "Go", LOC: 3},
			{ID: "f:README.md", Kind: graph.KindFile, Name: "README.md", Path: "README.md", Parent: "d:.", Lang: "Markdown", LOC: 1},
			{ID: "s:main.go#main", Kind: graph.KindSymbol, Name: "main", SymbolKind: "func", Line: 3, Parent: "f:main.go"},
			{ID: "e:go", Kind: graph.KindEcosystem, Name: "Go modules"},
			{ID: "e:go-std", Kind: graph.KindEcosystem, Name: "Go standard library", Std: true},
			{ID: "p:go-std:fmt", Kind: graph.KindPackage, Name: "fmt", Parent: "e:go-std"},
			{ID: "p:go:x.io/y", Kind: graph.KindPackage, Name: "x.io/y", Version: "v1.0.0", Parent: "e:go"},
			{ID: "p:go:x.io/<z>", Kind: graph.KindPackage, Name: "x.io/<z>", Parent: "e:go", Unresolved: true},
		},
		Edges: []*graph.Edge{
			{From: "f:main.go", To: "d:pkg", Kind: graph.EdgeImport, Line: 4},
			{From: "f:main.go", To: "p:go:x.io/y", Kind: graph.EdgeImport, Line: 5},
			{From: "f:pkg/a.go", To: "p:go:x.io/<z>", Kind: graph.EdgeImport},
			{From: "f:pkg/a.go", To: "p:go-std:fmt", Kind: graph.EdgeImport},
		},
	}
}

func TestJSONRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample(), "json"); err != nil {
		t.Fatal(err)
	}
	var g graph.Graph
	if err := json.Unmarshal(buf.Bytes(), &g); err != nil || len(g.Nodes) != 11 || len(g.Edges) != 4 {
		t.Fatalf("round trip: %v, %d nodes, %d edges", err, len(g.Nodes), len(g.Edges))
	}
}

func TestGraphML(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample(), "graphml"); err != nil {
		t.Fatal(err)
	}
	var doc gmlDoc
	if err := xml.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("not well-formed: %v\n%s", err, buf.String())
	}
	if len(doc.Graph.Nodes) != 11 || len(doc.Graph.Edges) != 4 {
		t.Fatalf("%d nodes, %d edges", len(doc.Graph.Nodes), len(doc.Graph.Edges))
	}
	var loc string
	for _, n := range doc.Graph.Nodes {
		if n.ID == "f:main.go" {
			for _, d := range n.Data {
				if d.Key == "loc" {
					loc = d.Value
				}
			}
		}
	}
	if loc != "10" {
		t.Errorf("main.go loc = %q", loc)
	}
	if !strings.Contains(buf.String(), "x.io/&lt;z&gt;") {
		t.Error("names must be XML-escaped")
	}
}

func TestDOT(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample(), "dot"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`digraph "odd \"repo\"" {`,
		`subgraph "cluster_d:pkg" {`,
		`subgraph "cluster_e:go" {`,
		`"d:pkg" [label="pkg/", shape=folder];`,
		`"p:go:x.io/y" [label="x.io/y\nv1.0.0", shape=component];`,
		`style="rounded,dashed"`,
		`"f:main.go" -> "p:go:x.io/y";`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	if strings.Contains(out, "README.md") {
		t.Error("files without edges should be omitted")
	}
	if strings.Contains(out, "go-std") {
		t.Error("standard-library packages should be omitted")
	}
	if strings.Count(out, "{") != strings.Count(out, "}") {
		t.Error("unbalanced braces")
	}
}

func TestUnknownFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, sample(), "svg"); err == nil {
		t.Error("expected an error")
	}
}
