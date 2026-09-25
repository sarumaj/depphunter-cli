package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/graph"
)

// Verifies: REQ-EXP-006, REQ-EXP-007
func TestWriteStatic(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // </script><script>alert(1)</script>\n"), 0o644)
	os.WriteFile(filepath.Join(root, "big.txt"), bytes.Repeat([]byte("x"), staticPerFile+1), 0o644)
	os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0, 1, 2}, 0o644)
	g := &graph.Graph{Root: "demo", Nodes: []*graph.Node{
		{ID: "f:a.go", Kind: graph.KindFile, Path: "a.go"},
		{ID: "f:big.txt", Kind: graph.KindFile, Path: "big.txt"},
		{ID: "f:bin.dat", Kind: graph.KindFile, Path: "bin.dat"},
	}}
	var buf bytes.Buffer
	if err := WriteStatic(&buf, g, config.Default().UI, root, nil); err != nil {
		t.Fatal(err)
	}
	page := buf.String()

	// The embedded source must not be able to close the data <script>.
	if strings.Count(page, "</script>") != 3 {
		t.Errorf("expected exactly the three page scripts to close, got %d", strings.Count(page, "</script>"))
	}

	data := regexp.MustCompile(`(?s)<script type="application/json" id="depphunter-data">(.*?)</script>`).FindStringSubmatch(page)
	var payload struct {
		Config  map[string]any
		Sources map[string]string
	}
	if data == nil || json.Unmarshal([]byte(data[1]), &payload) != nil {
		t.Fatal("data payload missing or invalid")
	}
	if payload.Config["static"] != true {
		t.Error("config must mark the page static")
	}
	if _, ok := payload.Sources["a.go"]; !ok || len(payload.Sources) != 1 {
		t.Errorf("sources should hold only a.go, got %d entries", len(payload.Sources))
	}

	// Every module is inlined and none keeps a relative import (they cannot resolve
	// from a data: URL).
	m := regexp.MustCompile(`<script type="importmap">(.*?)</script>`).FindStringSubmatch(page)
	var im struct{ Imports map[string]string }
	if m == nil || json.Unmarshal([]byte(m[1]), &im) != nil {
		t.Fatal("import map missing or invalid")
	}
	for _, name := range []string{"app", "model", "layout", "scene", "panel", "filter", "colors", "data", "three.module.min", "OrbitControls", "highlight.min"} {
		url, ok := im.Imports["depphunter/"+name]
		if !ok {
			t.Errorf("module %s missing", name)
			continue
		}
		src, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, "data:text/javascript;base64,"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if loc := regexp.MustCompile(`(?:from|import)\s*\(?\s*['"]\.\.?/`).FindIndex(src); loc != nil {
			t.Errorf("%s still has a relative import: %.60s", name, src[loc[0]:])
		}
	}
	if strings.Contains(page, `href="style.css"`) || strings.Contains(page, `src="app.js"`) {
		t.Error("page still references external assets")
	}
}
