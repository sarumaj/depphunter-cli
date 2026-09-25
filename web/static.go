package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/config"
	"github.com/sarumaj/depphunter-cli/internal/export"
	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/minify"
)

// Limits for source text embedded in a static page.
//
// Implements: REQ-EXP-007
const (
	staticPerFile = 256 << 10
	staticTotal   = 24 << 20
)

// relativeImport matches `from './x.js'` and `import('./vendor/x.js')`.
var relativeImport = regexp.MustCompile(`((?:from|import)\s*\(?\s*)(['"])\./(?:vendor/)?([\w.\-]+)\.js(['"])`)

// WriteStatic writes the UI as one self-contained HTML file that opens without a
// server: every ES module is inlined as a data: URL behind an import map (relative
// imports are rewritten to the map's names), the stylesheet is inlined, and the graph,
// UI settings and source texts (within size limits) are embedded as JSON.
// extra holds optional datasets ("history", "references", "findings") embedded as they are.
//
// Implements: REQ-EXP-006, REQ-EXP-007, REQ-HIST-015
func WriteStatic(w io.Writer, g *graph.Graph, ui config.UI, root string, extra map[string]any) error {
	assets := Assets()
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return err
	}
	css, err := fs.ReadFile(assets, "style.css")
	if err != nil {
		return err
	}
	css = []byte(minify.CSS(string(css)))
	index = []byte(minify.HTML(string(index)))

	imports := map[string]string{}
	err = fs.WalkDir(assets, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".js" {
			return err
		}
		src, err := fs.ReadFile(assets, p)
		if err != nil {
			return err
		}
		src = relativeImport.ReplaceAll(src, []byte(`$1"depphunter/$3"`))
		// An export is one file with everything in it and no server to compress it,
		// so the comments come out here too (internal/minify).
		src = []byte(minify.JS(string(src)))
		imports["depphunter/"+strings.TrimSuffix(path.Base(p), ".js")] = "data:text/javascript;base64," + base64.StdEncoding.EncodeToString(src)
		return nil
	})
	if err != nil {
		return err
	}
	importMap, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		return err
	}
	// The models are binary assets the UI fetches; a page with no server to fetch
	// from carries them inline instead.
	models := map[string]string{}
	for key, name := range map[string]string{"hand": "hand.glb", "props": "props.glb", "bug": "bug.glb"} {
		b, err := fs.ReadFile(assets, name)
		if err != nil {
			return err
		}
		models[key] = "data:model/gltf-binary;base64," + base64.StdEncoding.EncodeToString(b)
	}
	// The introduction's pictures, likewise: a first visit to an exported map is still
	// a first visit, and a card whose picture is a broken link is worse than a card
	// that never had one. Read from the directory rather than from a list, so that
	// taking a new picture is one command and not two edits (scripts/tour-shots.mjs).
	tour := map[string]string{}
	shots, err := fs.ReadDir(assets, "tour")
	if err == nil {
		for _, shot := range shots {
			name := shot.Name()
			if !strings.HasSuffix(name, ".webp") {
				continue
			}
			b, err := fs.ReadFile(assets, "tour/"+name)
			if err != nil {
				return err
			}
			tour[strings.TrimSuffix(name, ".webp")] = "data:image/webp;base64," + base64.StdEncoding.EncodeToString(b)
		}
	}
	// json.Marshal escapes <, > and &, so the payload cannot close its <script> element.
	data := map[string]any{
		"hand":  models["hand"],
		"props": models["props"],
		"bug":   models["bug"],
		"tour":  tour,
		"graph": g,
		"config": struct {
			config.UI
			Static bool `json:"static"`
		}{ui, true},
		"sources": export.Sources(root, g, staticPerFile, staticTotal),
	}
	for k, v := range extra {
		data[k] = v
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	page := string(index)
	for old, repl := range map[string]string{
		`<link rel="stylesheet" href="style.css">`: "<style>\n" + string(css) + "</style>",
		`<script type="module" src="app.js"></script>`: `<script type="application/json" id="depphunter-data">` + string(payload) + "</script>\n" +
			`  <script type="importmap">` + string(importMap) + "</script>\n" +
			`  <script type="module">import "depphunter/app";</script>`,
	} {
		if !strings.Contains(page, old) {
			return fmt.Errorf("static export: index.html no longer contains %s", old)
		}
		page = strings.Replace(page, old, repl, 1)
	}
	_, err = io.Copy(w, bytes.NewBufferString(page))
	return err
}
