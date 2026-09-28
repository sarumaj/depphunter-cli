package fortran

import (
	"path"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Manifest import kinds, carried in RawImport.Name.
const (
	kindDep      = "dep"      // [dependencies], [features.*.dependencies]
	kindDevDep   = "dev"      // [dev-dependencies]
	kindDir      = "dir"      // [library] source-dir
	kindMain     = "main"     // [[executable]], [[test]], [[example]] main file
	kindExternal = "external" // [build] external-modules
	kindLink     = "link"     // [build] link
)

// dependency is one entry of fpm.toml's dependency tables: a git repository
// (with a tag, branch or rev), a path, a registry package (namespace and v), or
// a metapackage written as a string (`stdlib = "*"`).
type dependency struct {
	name                  string
	git, tag, branch, rev string
	path                  string
	namespace, v          string
	meta                  string
	metaSet               bool
	dev                   bool
	line                  int
}

// program is an [[executable]], [[test]] or [[example]] entry.
type program struct {
	kind, name, dir, main string
	line                  int
}

// manifest is what an fpm.toml says.
type manifest struct {
	name            string
	nameLine        int
	deps            map[string]*dependency
	sourceDir       string // [library] source-dir as written, "" when not written
	sourceLine      int
	includeDirs     []string // [library] include-dir, "include" by default
	externalModules []string // [build] external-modules, lower case
	link            []string // [build] link
	buildLine       int
	programs        []program
}

// Implements: REQ-FORTRAN-005
func readManifest(src []byte) *manifest {
	m := &manifest{deps: map[string]*dependency{}}
	var raw map[string]any
	if _, err := toml.Decode(string(src), &raw); err != nil {
		return m
	}
	lines := newLines(src)
	m.name, _ = raw["name"].(string)
	m.nameLine = lines.key("", "name")
	readDeps(raw["dependencies"], false, "dependencies", lines, m.deps)
	if feats, ok := raw["features"].(map[string]any); ok {
		for _, f := range sortedKeys(feats) {
			if ft, ok := feats[f].(map[string]any); ok {
				readDeps(ft["dependencies"], false, "features."+f+".dependencies", lines, m.deps)
			}
		}
	}
	readDeps(raw["dev-dependencies"], true, "dev-dependencies", lines, m.deps)
	m.includeDirs = []string{"include"}
	if lib, ok := raw["library"].(map[string]any); ok {
		if s, ok := lib["source-dir"].(string); ok && s != "" {
			m.sourceDir = s
			m.sourceLine = lines.key("library", "source-dir")
		}
		if inc := stringList(lib["include-dir"]); len(inc) > 0 {
			m.includeDirs = inc
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
			p := program{kind: kind, dir: map[string]string{"executable": "app", "test": "test", "example": "example"}[kind], main: "main.f90"}
			p.name, _ = e["name"].(string)
			if s, ok := e["source-dir"].(string); ok && s != "" {
				p.dir = s
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
			readDeps(e["dependencies"], kind != "executable", kind+".dependencies", lines, m.deps)
		}
	}
	return m
}

func readDeps(v any, dev bool, section string, lines *tomlLines, into map[string]*dependency) {
	tbl, ok := v.(map[string]any)
	if !ok {
		return
	}
	for _, name := range sortedKeys(tbl) {
		if name == "" || into[name] != nil {
			continue
		}
		d := &dependency{name: name, dev: dev, line: lines.key(section, name)}
		switch e := tbl[name].(type) {
		case string:
			d.meta, d.metaSet = e, true
		case map[string]any:
			str := func(k string) string {
				s, _ := e[k].(string)
				return strings.TrimSpace(s)
			}
			d.git, d.tag, d.branch, d.rev = str("git"), str("tag"), str("branch"), str("rev")
			d.path, d.namespace, d.v = str("path"), str("namespace"), str("v")
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

func newLines(src []byte) *tomlLines { return &tomlLines{lines: strings.Split(string(src), "\n")} }

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
	cur := ""
	for i, l := range tl.lines {
		if h, ok := header(l); ok {
			cur = h
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
		if cur == section && strings.HasPrefix(t, key) {
			rest := strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(t[len(key):], `"`), `'`), " \t")
			if strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, ".") {
				return i + 1
			}
		}
		if section != "" && cur == "" && strings.HasPrefix(t, section+".") {
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
func extractManifest(src []byte) *lang.Extraction {
	m := readManifest(src)
	ex := &lang.Extraction{}
	for _, name := range sortedKeys(m.deps) {
		d := m.deps[name]
		kind := kindDep
		if d.dev {
			kind = kindDevDep
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kind, Line: max(d.line, 1)})
	}
	if m.sourceDir != "" {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "library: " + m.sourceDir, Module: path.Clean(m.sourceDir), Name: kindDir, Line: max(m.sourceLine, 1)})
	}
	seen := map[string]bool{}
	for _, p := range m.programs {
		file := path.Join(p.dir, p.main)
		spec := p.kind + " " + p.name + ": " + file
		if seen[spec] {
			continue
		}
		seen[spec] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: file, Name: kindMain, Line: max(p.line, 1)})
	}
	for _, e := range m.externalModules {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "external-modules: " + e, Module: e, Name: kindExternal, Line: max(m.buildLine, 1)})
	}
	for _, l := range m.link {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "link: " + l, Module: l, Name: kindLink, Line: max(m.buildLine, 1)})
	}
	if m.name != "" {
		ex.Symbols = []lang.Symbol{{Name: m.name, Kind: "package", Line: max(m.nameLine, 1)}}
	}
	return ex
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
