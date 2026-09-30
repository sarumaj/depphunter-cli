package fortran

import (
	"path"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Manifest import kinds, carried in RawImport.Name.
const (
	kindDependency    = "dep"      // [dependencies], [features.*.dependencies]
	kindDevDependency = "dev"      // [dev-dependencies]
	kindDirectory     = "dir"      // [library] source-dir
	kindMain          = "main"     // [[executable]], [[test]], [[example]] main file
	kindExternal      = "external" // [build] external-modules
	kindLink          = "link"     // [build] link
)

// dependency is one entry of fpm.toml's dependency tables: a git repository
// (with a tag, branch or rev), a path, a registry package (namespace and v), or
// a metapackage written as a string (`stdlib = "*"`).
type dependency struct {
	name                  string
	git, tag, branch, rev string
	path                  string
	namespace, v          string
	metadata              string
	metaSet               bool
	dev                   bool
	line                  int
}

// program is an [[executable]], [[test]] or [[example]] entry.
type program struct {
	kind, name, directory, main string
	line                        int
}

// manifest is what an fpm.toml says.
type manifest struct {
	name               string
	nameLine           int
	dependencies       map[string]*dependency
	sourceDirectory    string // [library] source-dir as written, "" when not written
	sourceLine         int
	includeDirectories []string // [library] include-dir, "include" by default
	externalModules    []string // [build] external-modules, lower case
	link               []string // [build] link
	buildLine          int
	programs           []program
}

// Implements: REQ-FORTRAN-005
func readManifest(source []byte) *manifest {
	m := &manifest{dependencies: map[string]*dependency{}}
	var raw map[string]any
	if _, err := toml.Decode(string(source), &raw); err != nil {
		return m
	}
	lines := newLines(source)
	m.name, _ = raw["name"].(string)
	m.nameLine = lines.key("", "name")
	readDependencies(raw["dependencies"], false, "dependencies", lines, m.dependencies)
	if feats, ok := raw["features"].(map[string]any); ok {
		for _, f := range lang.SortedKeys(feats) {
			if ft, ok := feats[f].(map[string]any); ok {
				readDependencies(ft["dependencies"], false, "features."+f+".dependencies", lines, m.dependencies)
			}
		}
	}
	readDependencies(raw["dev-dependencies"], true, "dev-dependencies", lines, m.dependencies)
	m.includeDirectories = []string{"include"}
	if library, ok := raw["library"].(map[string]any); ok {
		if s, ok := library["source-dir"].(string); ok && s != "" {
			m.sourceDirectory = s
			m.sourceLine = lines.key("library", "source-dir")
		}
		if include := stringList(library["include-dir"]); len(include) > 0 {
			m.includeDirectories = include
		}
	}
	if b, ok := raw["build"].(map[string]any); ok {
		for _, e := range stringList(b["external-modules"]) {
			m.externalModules = append(m.externalModules, strings.ToLower(e))
		}
		m.link = stringList(b["link"])
		m.buildLine = lines.key("build", "external-modules")
		if m.buildLine == 0 {
			m.buildLine = lines.key("build", "link")
		}
	}
	for _, kind := range []string{"executable", "test", "example"} {
		list, _ := raw[kind].([]map[string]any)
		for n, e := range list {
			p := program{kind: kind, directory: map[string]string{"executable": "app", "test": "test", "example": "example"}[kind], main: "main.f90"}
			p.name, _ = e["name"].(string)
			if s, ok := e["source-dir"].(string); ok && s != "" {
				p.directory = s
			}
			if s, ok := e["main"].(string); ok && s != "" {
				p.main = s
			}
			p.line = lines.array(kind, n)
			m.programs = append(m.programs, p)
		}
	}
	// A program's own dependencies: [[test]] ... [test.dependencies]; a test's
	// or an example's are development dependencies.
	for _, kind := range []string{"executable", "example", "test"} {
		list, _ := raw[kind].([]map[string]any)
		for _, e := range list {
			readDependencies(e["dependencies"], kind != "executable", kind+".dependencies", lines, m.dependencies)
		}
	}
	return m
}

func readDependencies(v any, dev bool, section string, lines *tomlLines, into map[string]*dependency) {
	table, ok := v.(map[string]any)
	if !ok {
		return
	}
	for _, name := range lang.SortedKeys(table) {
		if name == "" || into[name] != nil {
			continue
		}
		d := &dependency{name: name, dev: dev, line: lines.key(section, name)}
		switch e := table[name].(type) {
		case string:
			d.metadata, d.metaSet = e, true
		case map[string]any:
			stringField := func(k string) string {
				s, _ := e[k].(string)
				return strings.TrimSpace(s)
			}
			d.git, d.tag, d.branch, d.rev = stringField("git"), stringField("tag"), stringField("branch"), stringField("rev")
			d.path, d.namespace, d.v = stringField("path"), stringField("namespace"), stringField("v")
		default:
			continue
		}
		into[name] = d
	}
}

func stringList(v any) []string {
	switch e := v.(type) {
	case string:
		if e != "" {
			return []string{e}
		}
	case []any:
		var out []string
		for _, x := range e {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// tomlLines finds where keys of a TOML file are written, for import lines:
// BurntSushi/toml does not report positions of decoded keys.
type tomlLines struct {
	lines []string
}

func newLines(source []byte) *tomlLines {
	return &tomlLines{lines: strings.Split(string(source), "\n")}
}

// header is a table header line's name: [a.b] -> a.b, [[a]] -> a.
func header(l string) (string, bool) {
	t := strings.TrimSpace(l)
	if !strings.HasPrefix(t, "[") {
		return "", false
	}
	if i := strings.Index(t, "#"); i > 0 {
		t = strings.TrimSpace(t[:i])
	}
	t = strings.Trim(t, "[]")
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(t), " ", ""), `"`, ""), true
}

// key is the first line that writes key in table section ("" for the top
// level), as `key = ...`, `key.x = ...` or its own [section.key] header; 0 when
// not found.
func (tl *tomlLines) key(section, key string) int {
	current := ""
	for i, l := range tl.lines {
		if h, ok := header(l); ok {
			current = h
			if section != "" && h == section+"."+key || section == "" && h == key {
				return i + 1
			}
			if section != "" && strings.HasPrefix(h, section+"."+key+".") {
				return i + 1
			}
			continue
		}
		t := strings.TrimSpace(l)
		t = strings.TrimPrefix(t, `"`)
		t = strings.TrimPrefix(t, `'`)
		if current == section && strings.HasPrefix(t, key) {
			rest := strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(t[len(key):], `"`), `'`), " \t")
			if strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, ".") {
				return i + 1
			}
		}
		if section != "" && current == "" && strings.HasPrefix(t, section+".") {
			if rest := strings.TrimPrefix(t, section+"."); strings.HasPrefix(rest, key) {
				return i + 1 // dependencies.x = { ... } at the top level
			}
		}
	}
	return 0
}

// array is the header line of the n-th [[name]] entry.
func (tl *tomlLines) array(name string, n int) int {
	for i, l := range tl.lines {
		if strings.TrimSpace(l) == "[["+name+"]]" || strings.HasPrefix(strings.TrimSpace(l), "[["+name+"]]") {
			if n == 0 {
				return i + 1
			}
			n--
		}
	}
	return 0
}

// extractManifest makes each dependency of fpm.toml an import of the package it
// names, an explicit library source directory and each program's main file an
// import of that directory or file, and the external modules and libraries of
// [build] imports of what provides them.
//
// Implements: REQ-FORTRAN-005
func extractManifest(source []byte) *lang.Extraction {
	m := readManifest(source)
	extraction := &lang.Extraction{}
	for _, name := range lang.SortedKeys(m.dependencies) {
		d := m.dependencies[name]
		kind := kindDependency
		if d.dev {
			kind = kindDevDependency
		}
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kind, Line: max(d.line, 1)})
	}
	if m.sourceDirectory != "" {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "library: " + m.sourceDirectory, Module: path.Clean(m.sourceDirectory), Name: kindDirectory, Line: max(m.sourceLine, 1)})
	}
	seen := map[string]bool{}
	for _, p := range m.programs {
		file := path.Join(p.directory, p.main)
		spec := p.kind + " " + p.name + ": " + file
		if seen[spec] {
			continue
		}
		seen[spec] = true
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: file, Name: kindMain, Line: max(p.line, 1)})
	}
	for _, e := range m.externalModules {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "external-modules: " + e, Module: e, Name: kindExternal, Line: max(m.buildLine, 1)})
	}
	for _, l := range m.link {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "link: " + l, Module: l, Name: kindLink, Line: max(m.buildLine, 1)})
	}
	if m.name != "" {
		extraction.Symbols = []lang.Symbol{{Name: m.name, Kind: "package", Line: max(m.nameLine, 1)}}
	}
	return extraction
}
