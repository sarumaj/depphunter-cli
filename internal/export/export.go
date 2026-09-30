// Package export writes a graph as JSON (the UI's own format), GraphML (every node
// and attribute, for Gephi, yEd, NetworkX) or Graphviz DOT (the dependency graph).
package export

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/emicklei/dot"

	"github.com/sarumaj/depphunter-cli/internal/graph"
)

// Format is a way to write a graph out, with what a download of it is served as.
type Format struct {
	Name, ContentType, Extension string
	write                        func(io.Writer, *graph.Graph) error
}

// formats is what Write produces, in the order they are offered; "html" (the static
// page) is written by package web.
var formats = []Format{
	{"json", "application/json", ".json", func(w io.Writer, g *graph.Graph) error { return json.NewEncoder(w).Encode(g) }},
	{"graphml", "application/graphml+xml", ".graphml", writeGraphML},
	{"dot", "text/vnd.graphviz", ".dot", writeDOT},
}

// Lookup returns the format called name.
func Lookup(name string) (Format, bool) {
	for _, f := range formats {
		if f.Name == name {
			return f, true
		}
	}
	return Format{}, false
}

// Names lists the formats' names, comma-separated, for an error message.
func Names() string {
	names := make([]string, len(formats))
	for i, f := range formats {
		names[i] = f.Name
	}
	return strings.Join(names, ", ")
}

// WithEdges returns a copy of g that also holds extra edges (e.g. symbol references).
func WithEdges(g *graph.Graph, extra []*graph.Edge) *graph.Graph {
	if len(extra) == 0 {
		return g
	}
	c := *g
	c.Edges = append(append([]*graph.Edge{}, g.Edges...), extra...)
	return &c
}

// Implements: REQ-EXP-001
func Write(w io.Writer, g *graph.Graph, format string) error {
	f, ok := Lookup(format)
	if !ok {
		return fmt.Errorf("unknown export format %q (want one of %s)", format, Names())
	}
	return f.write(w, g)
}

// ---------------------------------------------------------------- GraphML

type gmlKey struct {
	ID   string `xml:"id,attr"`
	For  string `xml:"for,attr"`
	Name string `xml:"attr.name,attr"`
	Type string `xml:"attr.type,attr"`
}

type gmlData struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

type gmlNode struct {
	ID   string    `xml:"id,attr"`
	Data []gmlData `xml:"data"`
}

type gmlEdge struct {
	ID     string    `xml:"id,attr"`
	Source string    `xml:"source,attr"`
	Target string    `xml:"target,attr"`
	Data   []gmlData `xml:"data"`
}

type gmlDoc struct {
	XMLName   xml.Name `xml:"graphml"`
	Namespace string   `xml:"xmlns,attr"`
	Keys      []gmlKey `xml:"key"`
	Graph     struct {
		ID          string    `xml:"id,attr"`
		EdgeDefault string    `xml:"edgedefault,attr"`
		Nodes       []gmlNode `xml:"node"`
		Edges       []gmlEdge `xml:"edge"`
	} `xml:"graph"`
}

// A node attribute in GraphML: its key, its type, and its value ("" for none, which
// leaves it out).
type gmlField struct{ id, typeName, value string }

// nodeFields lists a node's attributes in key order; any node, the zero one included,
// gives every key.
func nodeFields(n *graph.Node) []gmlField {
	number := func(v int) string {
		if v == 0 {
			return ""
		}
		return strconv.Itoa(v)
	}
	flag := func(v bool) string {
		if !v {
			return ""
		}
		return "true"
	}
	return []gmlField{
		{"kind", "string", string(n.Kind)}, {"name", "string", n.Name}, {"path", "string", n.Path},
		{"parent", "string", n.Parent}, {"lang", "string", n.Language}, {"loc", "int", number(n.LOC)},
		{"symbolKind", "string", n.SymbolKind}, {"line", "int", number(n.Line)}, {"version", "string", n.Version},
		// Implements: REQ-SUP-006
		{"requested", "string", n.Requested}, {"floating", "boolean", flag(n.Floating)},
		{"transitive", "boolean", flag(n.Transitive)}, {"index", "string", n.Index},
		{"indexUnknown", "boolean", flag(n.IndexUnknown)}, {"private", "boolean", flag(n.Private)},
		{"std", "boolean", flag(n.Std)}, {"unresolved", "boolean", flag(n.Unresolved)},
		{"origin", "string", n.Origin}, {"git", "string", n.Git}, {"platform", "string", n.Platform},
	}
}

// Implements: REQ-EXP-002, REQ-EXP-013
func writeGraphML(w io.Writer, g *graph.Graph) error {
	doc := gmlDoc{Namespace: "http://graphml.graphdrawing.org/xmlns"}
	for _, f := range nodeFields(&graph.Node{}) {
		doc.Keys = append(doc.Keys, gmlKey{ID: f.id, For: "node", Name: f.id, Type: f.typeName})
	}
	doc.Keys = append(doc.Keys,
		gmlKey{ID: "edgeKind", For: "edge", Name: "kind", Type: "string"},
		gmlKey{ID: "edgeLine", For: "edge", Name: "line", Type: "int"})
	doc.Graph.ID = g.Root
	doc.Graph.EdgeDefault = "directed"

	for _, n := range g.Nodes {
		var data []gmlData
		for _, f := range nodeFields(n) {
			if f.value != "" {
				data = append(data, gmlData{f.id, f.value})
			}
		}
		doc.Graph.Nodes = append(doc.Graph.Nodes, gmlNode{ID: n.ID, Data: data})
	}
	for i, e := range g.Edges {
		data := []gmlData{{"edgeKind", string(e.Kind)}}
		if e.Line != 0 {
			data = append(data, gmlData{"edgeLine", strconv.Itoa(e.Line)})
		}
		doc.Graph.Edges = append(doc.Graph.Edges, gmlEdge{ID: "e" + strconv.Itoa(i), Source: e.From, Target: e.To, Data: data})
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// ---------------------------------------------------------------- DOT

// writeDOT emits the dependency graph between files, import-target directories (Go
// packages) and external packages, with one flat cluster per directory and per
// ecosystem. Standard-library packages and files without edges are left out: they
// dominate the drawing without saying much. Nested clusters were tried and make
// Graphviz stack them into very tall layouts. JSON and GraphML keep everything.
//
// Implements: REQ-EXP-003
func writeDOT(w io.Writer, g *graph.Graph) error {
	byID := map[string]*graph.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	isStd := func(id string) bool {
		n := byID[id]
		return n != nil && n.Kind == graph.KindPackage && byID[n.Parent] != nil && byID[n.Parent].Std
	}
	var edges []*graph.Edge
	used := map[string]bool{}
	for _, e := range g.Edges {
		// Symbol references would swamp a file-level drawing: DOT shows imports only.
		if e.Kind != graph.EdgeImport || byID[e.From] == nil || byID[e.To] == nil || isStd(e.To) {
			continue
		}
		edges = append(edges, e)
		used[e.From], used[e.To] = true, true
	}

	// Cluster key: the containing directory for files, the directory itself for
	// directory targets, the ecosystem for packages.
	clusters := map[string][]*graph.Node{}
	for id := range used {
		n := byID[id]
		key := n.Parent
		if n.Kind == graph.KindDirectory {
			key = n.ID
		}
		clusters[key] = append(clusters[key], n)
	}
	keys := make([]string, 0, len(clusters))
	for k := range clusters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Implements: REQ-DIST-016
	d := dot.NewGraph(dot.Directed)
	d.Attr("label", g.Root)
	d.Attr("rankdir", "LR")
	d.Attr("newrank", "true")
	d.Attr("fontname", "Helvetica")
	d.Attr("fontsize", "11")
	d.Attr("color", "#c3c2b7")
	d.NodeInitializer(func(n dot.Node) {
		n.Attr("shape", "box").Attr("style", "rounded,filled").Attr("fillcolor", "#f0efec").
			Attr("color", "#c3c2b7").Attr("fontname", "Helvetica").Attr("fontsize", "10")
	})
	d.EdgeInitializer(func(e dot.Edge) { e.Attr("color", "#898781").Attr("arrowsize", "0.6") })

	nodes := map[string]dot.Node{}
	for _, key := range keys {
		subgraph := d.Subgraph(key, dot.ClusterOption{})
		if c := byID[key]; c != nil && c.Kind == graph.KindDirectory {
			subgraph.Attr("label", c.Path+"/")
		} else if c != nil {
			subgraph.Attr("label", c.Name)
			subgraph.Attr("style", "filled")
			subgraph.Attr("fillcolor", "#e3e9ec")
		}
		members := clusters[key]
		sort.Slice(members, func(i, j int) bool { return members[i].ID < members[j].ID })
		for _, n := range members {
			node := subgraph.Node(n.ID).Label(n.Name)
			switch n.Kind {
			case graph.KindDirectory:
				node.Label(n.Path+"/").Attr("shape", "folder")
			case graph.KindPackage:
				label := n.Name
				if n.Version != "" {
					label += "\n" + n.Version
				}
				node.Label(label).Attr("shape", "component")
				if n.Unresolved {
					node.Attr("style", "rounded,dashed")
				}
			}
			nodes[n.ID] = node
		}
	}
	for _, e := range edges {
		d.Edge(nodes[e.From], nodes[e.To])
	}
	_, err := io.WriteString(w, d.String())
	return err
}

// Sources reads the text of the graph's files for a static export. Files over perFile
// bytes and binary files are skipped; reading stops once total bytes are collected.
//
// Implements: REQ-EXP-007
func Sources(root string, g *graph.Graph, perFile, total int64) map[string]string {
	out := map[string]string{}
	var used int64
	for n := range g.Of(graph.KindFile) {
		absolute := filepath.Join(root, filepath.FromSlash(n.Path))
		fileInfo, err := os.Stat(absolute)
		if err != nil || fileInfo.Size() > perFile || used+fileInfo.Size() > total {
			continue
		}
		data, err := os.ReadFile(absolute)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		out[n.Path] = string(data)
		used += int64(len(data))
	}
	return out
}
