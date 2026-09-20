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

// Formats lists what Write produces; "html" (the static page) is written by package web.
var Formats = []string{"json", "graphml", "dot"}

// ContentType and extension per format, for downloads.
var (
	ContentTypes = map[string]string{"json": "application/json", "graphml": "application/graphml+xml", "dot": "text/vnd.graphviz"}
	Extensions   = map[string]string{"json": ".json", "graphml": ".graphml", "dot": ".dot"}
)

// WithEdges returns a copy of g that also holds extra edges (e.g. symbol references).
func WithEdges(g *graph.Graph, extra []*graph.Edge) *graph.Graph {
	if len(extra) == 0 {
		return g
	}
	c := *g
	c.Edges = append(append([]*graph.Edge{}, g.Edges...), extra...)
	return &c
}

func Write(w io.Writer, g *graph.Graph, format string) error {
	switch format {
	case "json":
		return json.NewEncoder(w).Encode(g)
	case "graphml":
		return writeGraphML(w, g)
	case "dot":
		return writeDOT(w, g)
	}
	return fmt.Errorf("unknown export format %q (want one of %s)", format, strings.Join(Formats, ", "))
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
	XMLName xml.Name `xml:"graphml"`
	NS      string   `xml:"xmlns,attr"`
	Keys    []gmlKey `xml:"key"`
	Graph   struct {
		ID          string    `xml:"id,attr"`
		EdgeDefault string    `xml:"edgedefault,attr"`
		Nodes       []gmlNode `xml:"node"`
		Edges       []gmlEdge `xml:"edge"`
	} `xml:"graph"`
}

func writeGraphML(w io.Writer, g *graph.Graph) error {
	doc := gmlDoc{NS: "http://graphml.graphdrawing.org/xmlns"}
	for _, k := range []struct{ id, typ string }{
		{"kind", "string"}, {"name", "string"}, {"path", "string"}, {"parent", "string"},
		{"lang", "string"}, {"loc", "int"}, {"symbolKind", "string"}, {"line", "int"},
		{"version", "string"}, {"requested", "string"}, {"floating", "boolean"}, {"transitive", "boolean"},
		{"std", "boolean"}, {"unresolved", "boolean"},
	} {
		doc.Keys = append(doc.Keys, gmlKey{ID: k.id, For: "node", Name: k.id, Type: k.typ})
	}
	doc.Keys = append(doc.Keys,
		gmlKey{ID: "edgeKind", For: "edge", Name: "kind", Type: "string"},
		gmlKey{ID: "edgeLine", For: "edge", Name: "line", Type: "int"})
	doc.Graph.ID = g.Root
	doc.Graph.EdgeDefault = "directed"

	for _, n := range g.Nodes {
		var data []gmlData
		add := func(key, v string) {
			if v != "" {
				data = append(data, gmlData{key, v})
			}
		}
		num := func(key string, v int) {
			if v != 0 {
				add(key, strconv.Itoa(v))
			}
		}
		flag := func(key string, v bool) {
			if v {
				add(key, "true")
			}
		}
		add("kind", string(n.Kind))
		add("name", n.Name)
		add("path", n.Path)
		add("parent", n.Parent)
		add("lang", n.Lang)
		num("loc", n.LOC)
		add("symbolKind", n.SymbolKind)
		num("line", n.Line)
		add("version", n.Version)
		add("requested", n.Requested)
		flag("floating", n.Floating)
		flag("transitive", n.Transitive)
		flag("std", n.Std)
		flag("unresolved", n.Unresolved)
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
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
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
		if n.Kind == graph.KindDir {
			key = n.ID
		}
		clusters[key] = append(clusters[key], n)
	}
	keys := make([]string, 0, len(clusters))
	for k := range clusters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

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
		sub := d.Subgraph(key, dot.ClusterOption{})
		if c := byID[key]; c != nil && c.Kind == graph.KindDir {
			sub.Attr("label", c.Path+"/")
		} else if c != nil {
			sub.Attr("label", c.Name)
			sub.Attr("style", "filled")
			sub.Attr("fillcolor", "#e3e9ec")
		}
		members := clusters[key]
		sort.Slice(members, func(i, j int) bool { return members[i].ID < members[j].ID })
		for _, n := range members {
			node := sub.Node(n.ID).Label(n.Name)
			switch n.Kind {
			case graph.KindDir:
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
func Sources(root string, g *graph.Graph, perFile, total int64) map[string]string {
	out := map[string]string{}
	var used int64
	for _, n := range g.Nodes {
		if n.Kind != graph.KindFile {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(n.Path))
		st, err := os.Stat(abs)
		if err != nil || st.Size() > perFile || used+st.Size() > total {
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		out[n.Path] = string(data)
		used += int64(len(data))
	}
	return out
}
