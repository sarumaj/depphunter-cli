package elm

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one package elm.json lists: an application's exact version
// ("1.0.5") or a package's range ("1.0.0 <= v < 2.0.0").
type dependency struct {
	name, version string
	indirect      bool // an application's indirect dependency: installed for another
	test          bool // test-dependencies
}

// manifest is what elm.json says about a project.
type manifest struct {
	kind       string // "application" or "package"
	name       string // a package's author/name
	elmVersion string
	srcDirs    []string // an application's source-directories, as written
	deps       map[string]*dependency
	exposed    []string // a package's exposed-modules
}

// readManifest reads elm.json: an application (dependencies.direct/indirect and
// test-dependencies.direct/indirect, exact versions) or a package (dependencies
// and test-dependencies, ranges; exposed-modules as a list or grouped). Anything
// else - not JSON, no "type" - is nil.
//
// Implements: REQ-ELM-005
func readManifest(src []byte) *manifest {
	var raw struct {
		Type           string          `json:"type"`
		Name           string          `json:"name"`
		ElmVersion     string          `json:"elm-version"`
		SourceDirs     []string        `json:"source-directories"`
		Dependencies   json.RawMessage `json:"dependencies"`
		TestDeps       json.RawMessage `json:"test-dependencies"`
		ExposedModules json.RawMessage `json:"exposed-modules"`
	}
	if json.Unmarshal(src, &raw) != nil || raw.Type != "application" && raw.Type != "package" {
		return nil
	}
	m := &manifest{kind: raw.Type, name: raw.Name, elmVersion: raw.ElmVersion, srcDirs: raw.SourceDirs,
		deps: map[string]*dependency{}}
	for _, sec := range []struct {
		raw  json.RawMessage
		test bool
	}{{raw.Dependencies, false}, {raw.TestDeps, true}} {
		if m.kind == "application" {
			var split struct {
				Direct   map[string]string `json:"direct"`
				Indirect map[string]string `json:"indirect"`
			}
			json.Unmarshal(sec.raw, &split)
			m.add(split.Direct, false, sec.test)
			m.add(split.Indirect, true, sec.test)
		} else {
			var flat map[string]string
			json.Unmarshal(sec.raw, &flat)
			m.add(flat, false, sec.test)
		}
	}
	m.exposed = exposedModules(raw.ExposedModules)
	return m
}

func (m *manifest) add(deps map[string]string, indirect, test bool) {
	for name, v := range deps {
		if name = strings.TrimSpace(name); name != "" && m.deps[name] == nil {
			m.deps[name] = &dependency{name: name, version: strings.TrimSpace(v), indirect: indirect, test: test}
		}
	}
}

// exposedModules reads a package's exposed-modules: a list, or lists grouped
// under headings for the documentation.
func exposedModules(raw json.RawMessage) []string {
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var groups map[string][]string
	json.Unmarshal(raw, &groups)
	for _, g := range sortedKeys(groups) {
		list = append(list, groups[g]...)
	}
	return list
}

// extractManifest makes each package elm.json lists - direct, indirect and test
// dependencies alike - an import of that package, so what the build installs is
// on the map even when no module imports it, and each source directory an edge to
// that directory.
//
// Implements: REQ-ELM-005
func extractManifest(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	m := readManifest(src)
	if m == nil {
		return ex
	}
	lines := stringLines(src)
	for _, name := range sortedKeys(m.deps) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kindDep, Line: lines[name]})
	}
	for _, d := range m.srcDirs {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: d, Module: d, Name: kindSrcDir, Line: lines[d]})
	}
	if m.kind == "package" && m.name != "" {
		ex.Symbols = []lang.Symbol{{Name: m.name, Kind: "package", Line: lines[m.name]}}
	}
	return ex
}

var jsonString = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

// stringLines is the first line each JSON string is written on.
func stringLines(src []byte) map[string]int {
	out := map[string]int{}
	for i, l := range strings.Split(string(src), "\n") {
		for _, m := range jsonString.FindAllStringSubmatch(l, -1) {
			if _, ok := out[m[1]]; !ok {
				out[m[1]] = i + 1
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
