package elm

import (
	"encoding/json"
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
	kind              string // "application" or "package"
	name              string // a package's author/name
	elmVersion        string
	sourceDirectories []string // an application's source-directories, as written
	dependencies      map[string]*dependency
	exposed           []string // a package's exposed-modules
}

// readManifest reads elm.json: an application (dependencies.direct/indirect and
// test-dependencies.direct/indirect, exact versions) or a package (dependencies
// and test-dependencies, ranges; exposed-modules as a list or grouped). Anything
// else - not JSON, no "type" - is nil.
//
// Implements: REQ-ELM-005
func readManifest(source []byte) *manifest {
	var raw struct {
		Type              string          `json:"type"`
		Name              string          `json:"name"`
		ElmVersion        string          `json:"elm-version"`
		SourceDirectories []string        `json:"source-directories"`
		Dependencies      json.RawMessage `json:"dependencies"`
		TestDependencies  json.RawMessage `json:"test-dependencies"`
		ExposedModules    json.RawMessage `json:"exposed-modules"`
	}
	if json.Unmarshal(source, &raw) != nil || raw.Type != "application" && raw.Type != "package" {
		return nil
	}
	m := &manifest{kind: raw.Type, name: raw.Name, elmVersion: raw.ElmVersion, sourceDirectories: raw.SourceDirectories,
		dependencies: map[string]*dependency{}}
	for _, section := range []struct {
		raw  json.RawMessage
		test bool
	}{{raw.Dependencies, false}, {raw.TestDependencies, true}} {
		if m.kind == "application" {
			var split struct {
				Direct   map[string]string `json:"direct"`
				Indirect map[string]string `json:"indirect"`
			}
			json.Unmarshal(section.raw, &split)
			m.add(split.Direct, false, section.test)
			m.add(split.Indirect, true, section.test)
		} else {
			var flat map[string]string
			json.Unmarshal(section.raw, &flat)
			m.add(flat, false, section.test)
		}
	}
	m.exposed = exposedModules(raw.ExposedModules)
	return m
}

func (m *manifest) add(dependencies map[string]string, indirect, test bool) {
	for name, v := range dependencies {
		if name = strings.TrimSpace(name); name != "" && m.dependencies[name] == nil {
			m.dependencies[name] = &dependency{name: name, version: strings.TrimSpace(v), indirect: indirect, test: test}
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
	for _, g := range lang.SortedKeys(groups) {
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
func extractManifest(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	m := readManifest(source)
	if m == nil {
		return extraction
	}
	lines := lang.JSONStringLines(source)
	for _, name := range lang.SortedKeys(m.dependencies) {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kindDependency, Line: lines[name]})
	}
	for _, d := range m.sourceDirectories {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: d, Module: d, Name: kindSourceDirectory, Line: lines[d]})
	}
	if m.kind == "package" && m.name != "" {
		extraction.Symbols = []lang.Symbol{{Name: m.name, Kind: "package", Line: lines[m.name]}}
	}
	return extraction
}
