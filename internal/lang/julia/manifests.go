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
	kindDep      = "dep"      // [deps], [weakdeps], [extras]: Name = "uuid"
	kindMember   = "member"   // [workspace] projects
	kindExt      = "ext"      // [extensions]
	kindManifest = "manifest" // a Manifest.toml entry
)

// depSections are the Project.toml tables that name packages by UUID: what the
// package needs, what its extensions need, and what its tests need (the old
// [extras] + [targets] layout).
var depSections = []string{"deps", "weakdeps", "extras"}

// project is a Project.toml (or JuliaProject.toml).
type project struct {
	file, dir string
	name      string
	uuid      string
	deps      map[string]string // name -> uuid, every section
	section   map[string]string // name -> the first section naming it
	compat    map[string]string
	// sources are Pkg 1.11's [sources]: a dependency taken from a directory or a
	// repository rather than a registry.
	sources    map[string]pkgSource
	extensions map[string][]string // extension -> the weak dependencies it needs
	workspace  []string            // [workspace] projects, Pkg 1.12
	lines      map[string]int      // "section.key" -> line
	manifest   *manifest           // the manifest resolving it, if any
}

type pkgSource struct{ path, url, rev string }

// readProject reads a Project.toml; nil when it is not TOML.
func readProject(src []byte) *project {
	var doc map[string]any
	if _, err := toml.Decode(string(src), &doc); err != nil {
		return nil
	}
	p := &project{deps: map[string]string{}, section: map[string]string{}, compat: map[string]string{},
		sources: map[string]pkgSource{}, extensions: map[string][]string{}, lines: keyLines(src)}
	p.name, _ = doc["name"].(string)
	p.uuid, _ = doc["uuid"].(string)
	for _, sec := range depSections {
		m, _ := doc[sec].(map[string]any)
		for name, v := range m {
			uuid, _ := v.(string)
			if _, ok := p.deps[name]; !ok {
				p.deps[name] = uuid
				p.section[name] = sec
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
				s := pkgSource{}
				s.path, _ = t["path"].(string)
				s.url, _ = t["url"].(string)
				s.rev, _ = t["rev"].(string)
				p.sources[name] = s
			}
		}
	}
	if m, ok := doc["extensions"].(map[string]any); ok {
		for ext, v := range m {
			switch v := v.(type) {
			case string:
				p.extensions[ext] = []string{v}
			case []any:
				for _, x := range v {
					if s, ok := x.(string); ok {
						p.extensions[ext] = append(p.extensions[ext], s)
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

var (
	tableHeader = regexp.MustCompile(`^\s*\[\[?\s*([^\]]+?)\s*\]\]?\s*(#.*)?$`)
	keyLine     = regexp.MustCompile(`^\s*("([^"]+)"|[A-Za-z0-9_\-]+)\s*=`)
)

// keyLines maps "table.key" to the line of each key of a TOML file (1-based), and
// each table header to its line; the first occurrence wins.
func keyLines(src []byte) map[string]int {
	out := map[string]int{}
	table := ""
	for i, l := range strings.Split(string(src), "\n") {
		if m := tableHeader.FindStringSubmatch(l); m != nil {
			table = strings.ReplaceAll(m[1], `"`, "")
			if _, ok := out[table]; !ok {
				out[table] = i + 1
			}
			continue
		}
		if m := keyLine.FindStringSubmatch(l); m != nil {
			k := m[1]
			if m[2] != "" {
				k = m[2]
			}
			key := k
			if table != "" {
				key = table + "." + k
			}
			if _, ok := out[key]; !ok {
				out[key] = i + 1
			}
		}
	}
	return out
}

// extractProject makes a Project.toml's dependencies (every section), workspace
// projects and extensions imports; the package's name is its symbol.
//
// Implements: REQ-JULIA-006
func extractProject(src []byte) *lang.Extraction {
	ex := &lang.Extraction{Symbols: []lang.Symbol{}}
	p := readProject(src)
	if p == nil {
		return ex
	}
	var set lang.SymbolSet
	if p.name != "" {
		set.Add(p.name, "package", p.lines["name"])
	}
	for _, sec := range depSections {
		for _, name := range sortedKeys(p.section) {
			if p.section[name] == sec {
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: sec + "." + name, Module: name, Name: kindDep + "\n" + sec, Line: p.lines[sec+"."+name]})
			}
		}
	}
	// A dependency named in two sections (a weak dependency also a test extra) is
	// an import of each, as written.
	for _, sec := range depSections[1:] {
		for name := range p.deps {
			if p.section[name] != sec && p.lines[sec+"."+name] > 0 {
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: sec + "." + name, Module: name, Name: kindDep + "\n" + sec, Line: p.lines[sec+"."+name]})
			}
		}
	}
	for _, w := range p.workspace {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "workspace.projects " + w, Module: w, Name: kindMember, Line: p.lines["workspace.projects"]})
	}
	for _, ext := range sortedKeys(p.extensions) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "extensions." + ext, Module: ext, Name: kindExt, Line: p.lines["extensions."+ext]})
		set.Add(ext, "extension", p.lines["extensions."+ext])
	}
	ex.Symbols = append(ex.Symbols, set.List()...)
	sort.SliceStable(ex.Imports, func(i, j int) bool { return ex.Imports[i].Line < ex.Imports[j].Line })
	return ex
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// manifest is a Manifest.toml: every package of the environment, resolved.
type manifest struct {
	file, dir string
	byName    map[string][]*entry
	order     []*entry
}

// entry is one package of a manifest.
type entry struct {
	name, uuid, version string
	tree                string // git-tree-sha1: the exact content, which only registered and git packages have
	path                string // a developed package's directory
	repoURL, repoRev    string // a package added by URL
	deps                []string
	line                int
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
func readManifest(src []byte) *manifest {
	m := &manifest{byName: map[string][]*entry{}}
	var doc map[string]any
	if _, err := toml.Decode(string(src), &doc); err != nil {
		return m
	}
	tables := doc
	if d, ok := doc["deps"].(map[string]any); ok && doc["manifest_format"] != nil {
		tables = d
	}
	// Lines, by the order headers of a name appear in.
	lines := map[string][]int{}
	for i, l := range strings.Split(string(src), "\n") {
		if h := manifestHeader.FindStringSubmatch(l); h != nil {
			lines[h[1]] = append(lines[h[1]], i+1)
		}
	}
	for _, name := range sortedKeys(tables) {
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
			e.repoURL, _ = t["repo-url"].(string)
			e.repoRev, _ = t["repo-rev"].(string)
			switch d := t["deps"].(type) {
			case []any:
				for _, x := range d {
					if s, ok := x.(string); ok {
						e.deps = append(e.deps, s)
					}
				}
			case map[string]any: // names that are ambiguous are written with their UUIDs
				e.deps = sortedKeys(d)
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
func extractManifest(src []byte) *lang.Extraction {
	ex := &lang.Extraction{Symbols: []lang.Symbol{}}
	count := map[string]int{}
	for _, e := range readManifest(src).order {
		count[e.name]++
		spec := "[[deps." + e.name + "]]"
		if count[e.name] > 1 {
			spec += fmt.Sprintf(" #%d", count[e.name])
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: e.name, Name: kindManifest + "\n" + e.uuid, Line: e.line})
	}
	return ex
}

// extractArtifacts names an Artifacts.toml's artifacts: binaries Pkg downloads by
// content hash, which are no package of any registry.
//
// Implements: REQ-JULIA-001
func extractArtifacts(src []byte) *lang.Extraction {
	ex := &lang.Extraction{Symbols: []lang.Symbol{}}
	var doc map[string]any
	if _, err := toml.Decode(string(src), &doc); err != nil {
		return ex
	}
	lines := keyLines(src)
	var set lang.SymbolSet
	for _, name := range sortedKeys(doc) {
		set.Add(name, "artifact", lines[name])
	}
	ex.Symbols = set.List()
	return ex
}
