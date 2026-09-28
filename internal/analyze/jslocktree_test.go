package analyze

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
)

// The same install written in each JavaScript lock format walks offline to the
// same graph: the nested (or duplicate) d@2 that a loads is the version d's
// node takes, aliases reach the real package (h) or keep the name the project
// imports (c2), and neither the workspace, the Berry portal nor an optional
// package the install left out becomes an npm node. The fixtures are the
// JavaScript plugin's testdata/locktree.
//
// Verifies: REQ-SUP-009, REQ-SUP-013
func TestJavaScriptLockTreesOffline(t *testing.T) {
	for _, c := range []struct {
		directory string
		fsevents  bool
	}{
		{"npm-v3", false}, {"npm-v1", false}, {"berry", true},
		{"pnpm-v9", true}, {"pnpm-v6", true}, {"pnpm-v5", true},
	} {
		t.Run(c.directory, func(t *testing.T) {
			root := "../lang/javascript/testdata/locktree/" + c.directory
			g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{javascript.Plugin{}}, ResolveDepth: -1})
			if err != nil {
				t.Fatal(err)
			}
			wantNodes := map[string]string{
				"a": "1.0.0", "b": "1.0.0", "c2": "2.0.0", "d": "2.0.0", "e": "1.0.0", "f": "1.0.0", "h": "1.0.0",
			}
			wantEdges := []string{"a>d", "a>e", "b>d", "b>f", "b>h", "c2>d", "d>e"}
			if c.fsevents {
				wantNodes["fsevents"] = "2.3.3"
				wantEdges = append(wantEdges, "b>fsevents")
			}
			nodes := map[string]string{}
			for _, n := range g.Nodes {
				if n.Kind == graph.KindPackage {
					nodes[n.Name] = n.Version
				}
			}
			if !reflect.DeepEqual(nodes, wantNodes) {
				t.Errorf("packages %v, want %v", nodes, wantNodes)
			}
			prefix := graph.PackageID("npm", "")
			var edges []string
			for _, e := range g.Edges {
				if e.Kind == graph.EdgeDepends {
					edges = append(edges, strings.TrimPrefix(e.From, prefix)+">"+strings.TrimPrefix(e.To, prefix))
				}
			}
			sort.Strings(edges)
			sort.Strings(wantEdges)
			if !reflect.DeepEqual(edges, wantEdges) {
				t.Errorf("edges %v, want %v", edges, wantEdges)
			}
		})
	}
}
