package export

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"reflect"
	"regexp"
	"sort"
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
	// Node ids are generated; compare edges by label.
	labels := map[string]string{}
	for _, m := range regexp.MustCompile(`(n\d+)\[[^\]]*label="((?:[^"\\]|\\.)*)"`).FindAllStringSubmatch(out, -1) {
		labels[m[1]] = m[2]
	}
	var edges []string
	for _, m := range regexp.MustCompile(`(n\d+)->(n\d+)`).FindAllStringSubmatch(out, -1) {
		edges = append(edges, labels[m[1]]+" -> "+labels[m[2]])
	}
	sort.Strings(edges)
	want := []string{`a.go -> x.io/<z>`, `main.go -> pkg/`, `main.go -> x.io/y\nv1.0.0`}
	if !reflect.DeepEqual(edges, want) {
		t.Errorf("edges %q, want %q", edges, want)
	}
	for _, s := range []string{`label="odd \"repo\""`, `label="Go modules"`, `label="pkg/"`, `shape="folder"`, `style="rounded,dashed"`} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s in\n%s", s, out)
		}
	}
	for _, s := range []string{"README.md", "fmt"} {
		if strings.Contains(out, s) {
			t.Errorf("%s should be omitted (no edges / standard library)", s)
		}
	}
}

func TestUnknownFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, sample(), "svg"); err == nil {
		t.Error("expected an error")
	}
}

func TestReferencesInExports(t *testing.T) {
	g := WithEdges(sample(), []*graph.Edge{{From: "s:main.go#main", To: "f:pkg/a.go", Kind: graph.EdgeReference}})
	if len(sample().Edges) != 4 || len(g.Edges) != 5 {
		t.Fatalf("WithEdges must copy: %d edges", len(g.Edges))
	}
	var gml, dot bytes.Buffer
	Write(&gml, g, "graphml")
	Write(&dot, g, "dot")
	if !strings.Contains(gml.String(), `<data key="edgeKind">reference</data>`) {
		t.Error("GraphML should carry reference edges")
	}
	if strings.Contains(dot.String(), `label="main"`) {
		t.Error("DOT should leave symbol references out")
	}
}
