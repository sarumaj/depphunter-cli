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
	"testing/fstest"

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
	var buffer bytes.Buffer
	if err := WriteStatic(&buffer, g, config.Default().UI, root, nil); err != nil {
		t.Fatal(err)
	}
	page := buffer.String()

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
	var importMap struct{ Imports map[string]string }
	if m == nil || json.Unmarshal([]byte(m[1]), &importMap) != nil {
		t.Fatal("import map missing or invalid")
	}
	for _, name := range []string{"app", "model", "layout", "scene", "panel", "filter", "colors", "data", "three.module.min", "OrbitControls", "highlight.min"} {
		url, ok := importMap.Imports["depphunter/"+name]
		if !ok {
			t.Errorf("module %s missing", name)
			continue
		}
		source, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, "data:text/javascript;base64,"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if span := regexp.MustCompile(`(?:from|import)\s*\(?\s*['"]\.\.?/`).FindIndex(source); span != nil {
			t.Errorf("%s still has a relative import: %.60s", name, source[span[0]:])
		}
	}
	if strings.Contains(page, `href="style.css"`) || strings.Contains(page, `src="app.js"`) {
		t.Error("page still references external assets")
	}
}

// Modules in subdirectories are inlined by their file names, their relative imports
// rewritten whichever way they climb; two modules of one name are refused rather
// than one quietly standing in for the other.
//
// Verifies: REQ-EXP-006
func TestInlineModules(t *testing.T) {
	file := func(source string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(source)} }
	imports, err := inlineModules(fstest.MapFS{
		"app.js":                     file(`import { walk } from './walk/walk.js';`),
		"walk/walk.js":               file(`import { clamp } from '../core/numbers.js';\nimport * as THREE from '../vendor/three.module.min.js';\nexport const walk = clamp;`),
		"core/numbers.js":            file(`export const clamp = 1;`),
		"vendor/three.module.min.js": file(`export const x = 1;`),
	})
	if err != nil {
		t.Fatal(err)
	}
	walk, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(imports["depphunter/walk"], "data:text/javascript;base64,"))
	for _, want := range []string{`"depphunter/numbers"`, `"depphunter/three.module.min"`} {
		if !strings.Contains(string(walk), want) {
			t.Errorf("walk.js does not import %s: %s", want, walk)
		}
	}
	if len(imports) != 4 {
		t.Errorf("%d modules inlined, want 4: %v", len(imports), imports)
	}
	_, err = inlineModules(fstest.MapFS{"map/labels.js": file(""), "walk/labels.js": file("")})
	if err == nil || !strings.Contains(err.Error(), "labels.js") {
		t.Errorf("two modules named labels.js: %v", err)
	}
}
