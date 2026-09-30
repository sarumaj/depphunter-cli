package julia

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds of a Project.toml and a Manifest.toml.
const (
	kindDependency = "dep"      // [deps], [weakdeps], [extras]: Name = "uuid"
	kindMember     = "member"   // [workspace] projects
	kindExtension  = "ext"      // [extensions]
	kindManifest   = "manifest" // a Manifest.toml entry
)

// dependencySections are the Project.toml tables that name packages by UUID: what the
// package needs, what its extensions need, and what its tests need (the old
// [extras] + [targets] layout).
var dependencySections = []string{"deps", "weakdeps", "extras"}

// project is a Project.toml (or JuliaProject.toml).
type project struct {
	file, directory string
	name            string
	uuid            string
	dependencies    map[string]string // name -> uuid, every section
	section         map[string]string // name -> the first section naming it
	compat          map[string]string
	// sources are Pkg 1.11's [sources]: a dependency taken from a directory or a
	// repository rather than a registry.
	sources    map[string]packageSource
	extensions map[string][]string // extension -> the weak dependencies it needs
	workspace  []string            // [workspace] projects, Pkg 1.12
	lines      *lang.KeyLines
	manifest   *manifest // the manifest resolving it, if any
}

type packageSource struct{ path, url, rev string }

// readProject reads a Project.toml; nil when it is not TOML.
func readProject(source []byte) *project {
	var doc map[string]any
	if _, err := toml.Decode(string(source), &doc); err != nil {
		return nil
	}
	p := &project{dependencies: map[string]string{}, section: map[string]string{}, compat: map[string]string{},
		sources: map[string]packageSource{}, extensions: map[string][]string{}, lines: lang.TOMLKeyLines(source)}
	p.name, _ = doc["name"].(string)
	p.uuid, _ = doc["uuid"].(string)
	for _, section := range dependencySections {
		m, _ := doc[section].(map[string]any)
		for name, v := range m {
			uuid, _ := v.(string)
			if _, ok := p.dependencies[name]; !ok {
				p.dependencies[name] = uuid
				p.section[name] = section
			}
		}
	}
	if m, ok := doc["compat"].(map[string]any); ok {
		for name, v := range m {
			switch v := v.(type) {
			case string:
				p.compat[name] = strings.TrimSpace(v)
			case []any: // not what Pkg writes, but some projects do
				var parts []string
				for _, x := range v {
					if s, ok := x.(string); ok {
						parts = append(parts, s)
					}
				}
				p.compat[name] = strings.Join(parts, ", ")
			}
		}
	}
	if m, ok := doc["sources"].(map[string]any); ok {
		for name, v := range m {
			if t, ok := v.(map[string]any); ok {
				s := packageSource{}
				s.path, _ = t["path"].(string)
				s.url, _ = t["url"].(string)
				s.rev, _ = t["rev"].(string)
				p.sources[name] = s
			}
		}
	}
	if m, ok := doc["extensions"].(map[string]any); ok {
		for extension, v := range m {
			switch v := v.(type) {
			case string:
				p.extensions[extension] = []string{v}
			case []any:
				for _, x := range v {
					if s, ok := x.(string); ok {
						p.extensions[extension] = append(p.extensions[extension], s)
					}
				}
			}
		}
	}
	if w, ok := doc["workspace"].(map[string]any); ok {
		if list, ok := w["projects"].([]any); ok {
			for _, x := range list {
				if s, ok := x.(string); ok {
					p.workspace = append(p.workspace, s)
				}
			}
		}
	}
	return p
}

// extractProject makes a Project.toml's dependencies (every section), workspace
// projects and extensions imports; the package's name is its symbol.
//
// Implements: REQ-JULIA-006
func extractProject(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{Symbols: []lang.Symbol{}}
	p := readProject(source)
	if p == nil {
		return extraction
	}
	var set lang.SymbolSet
	if p.name != "" {
		set.Add(p.name, "package", p.lines.Line("", "name"))
	}
	for _, section := range dependencySections {
		for _, name := range lang.SortedKeys(p.section) {
			if p.section[name] == section {
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: section + "." + name, Module: name, Name: kindDependency + "\n" + section, Line: p.lines.Line(section, name)})
			}
		}
	}
	// A dependency named in two sections (a weak dependency also a test extra) is
	// an import of each, as written.
	for _, section := range dependencySections[1:] {
		for name := range p.dependencies {
			if p.section[name] != section && p.lines.Line(section, name) > 0 {
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: section + "." + name, Module: name, Name: kindDependency + "\n" + section, Line: p.lines.Line(section, name)})
			}
		}
	}
	for _, w := range p.workspace {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "workspace.projects " + w, Module: w, Name: kindMember, Line: p.lines.Line("workspace", "projects")})
	}
	for _, extension := range lang.SortedKeys(p.extensions) {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "extensions." + extension, Module: extension, Name: kindExtension, Line: p.lines.Line("extensions", extension)})
		set.Add(extension, "extension", p.lines.Line("extensions", extension))
	}
	extraction.Symbols = append(extraction.Symbols, set.List()...)
	sort.SliceStable(extraction.Imports, func(i, j int) bool { return extraction.Imports[i].Line < extraction.Imports[j].Line })
	return extraction
}

// manifest is a Manifest.toml: every package of the environment, resolved.
type manifest struct {
	file, directory string
	byName          map[string][]*entry
	order           []*entry
}

// entry is one package of a manifest.
type entry struct {
	name, uuid, version          string
	tree                         string // git-tree-sha1: the exact content, which only registered and git packages have
	path                         string // a developed package's directory
	repositoryURL, repositoryRev string // a package added by URL
	dependencies                 []string
	line                         int
}

// find is the entry of a package, by UUID when one is known.
func (m *manifest) find(name, uuid string) *entry {
	if m == nil {
		return nil
	}
	for _, e := range m.byName[name] {
		if uuid == "" || e.uuid == "" || e.uuid == uuid {
			return e
		}
	}
	return nil
}

// manifestHeader is an entry's table header: [[deps.Name]] (format 2.0) or [[Name]]
// (format 1).
var manifestHeader = regexp.MustCompile(`^\s*\[\[\s*(?:deps\.)?"?([^"\]\s]+)"?\s*\]\]`)

// readManifest reads a Manifest.toml of either format: 2.0 (Julia 1.7 and later,
// entries under [[deps.Name]]) or 1.0 (entries as top-level [[Name]] arrays).
func readManifest(source []byte) *manifest {
	m := &manifest{byName: map[string][]*entry{}}
	var doc map[string]any
	if _, err := toml.Decode(string(source), &doc); err != nil {
		return m
	}
	tables := doc
	if d, ok := doc["deps"].(map[string]any); ok && doc["manifest_format"] != nil {
		tables = d
	}
	// Lines, by the order headers of a name appear in.
	lines := map[string][]int{}
	for i, l := range strings.Split(string(source), "\n") {
		if h := manifestHeader.FindStringSubmatch(l); h != nil {
			lines[h[1]] = append(lines[h[1]], i+1)
		}
	}
	for _, name := range lang.SortedKeys(tables) {
		list, ok := tables[name].([]map[string]any)
		if !ok {
			continue
		}
		for k, t := range list {
			e := &entry{name: name}
			e.uuid, _ = t["uuid"].(string)
			e.version, _ = t["version"].(string)
			e.tree, _ = t["git-tree-sha1"].(string)
			e.path, _ = t["path"].(string)
			e.repositoryURL, _ = t["repo-url"].(string)
			e.repositoryRev, _ = t["repo-rev"].(string)
			switch d := t["deps"].(type) {
			case []any:
				for _, x := range d {
					if s, ok := x.(string); ok {
						e.dependencies = append(e.dependencies, s)
					}
				}
			case map[string]any: // names that are ambiguous are written with their UUIDs
				e.dependencies = lang.SortedKeys(d)
			}
			if k < len(lines[name]) {
				e.line = lines[name][k]
			}
			m.byName[name] = append(m.byName[name], e)
			m.order = append(m.order, e)
		}
	}
	sort.SliceStable(m.order, func(i, j int) bool { return m.order[i].line < m.order[j].line })
	return m
}

// extractManifest makes each package of a manifest an import of it, pinned to the
// version the manifest resolved.
//
// Implements: REQ-JULIA-007
func extractManifest(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{Symbols: []lang.Symbol{}}
	count := map[string]int{}
	for _, e := range readManifest(source).order {
		count[e.name]++
		spec := "[[deps." + e.name + "]]"
		if count[e.name] > 1 {
			spec += fmt.Sprintf(" #%d", count[e.name])
		}
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: e.name, Name: kindManifest + "\n" + e.uuid, Line: e.line})
	}
	return extraction
}

// extractArtifacts names an Artifacts.toml's artifacts: binaries Pkg downloads by
// content hash, which are no package of any registry.
//
// Implements: REQ-JULIA-001
func extractArtifacts(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{Symbols: []lang.Symbol{}}
	var doc map[string]any
	if _, err := toml.Decode(string(source), &doc); err != nil {
		return extraction
	}
	lines := lang.TOMLKeyLines(source)
	var set lang.SymbolSet
	for _, name := range lang.SortedKeys(doc) {
		set.Add(name, "artifact", lines.Line("", name))
	}
	extraction.Symbols = set.List()
	return extraction
}
