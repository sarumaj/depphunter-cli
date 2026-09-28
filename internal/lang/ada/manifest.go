package ada

import (
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// manifest is what an alire.toml says: the crate's name and version, its
// dependencies (every alternative of a case(...) expression), its pins and
// its project files.
type manifest struct {
	name, version string
	dependencies  map[string]*dependency
	pins          map[string]*pin
	projectFiles  []item
}

type dependency struct {
	name, constraint string
	line             int
}

// pin is an entry of [[pins]]: a local directory, a git repository (at a
// commit or a branch) or a version.
type pin struct {
	name                               string
	path, url, commit, branch, version string
	subdirectory                       string
	line                               int
}

// readManifest reads an alire.toml.
//
// Implements: REQ-ADA-006
func readManifest(source []byte) *manifest {
	m := &manifest{dependencies: map[string]*dependency{}, pins: map[string]*pin{}}
	var raw map[string]any
	if _, err := toml.Decode(string(source), &raw); err != nil {
		return m
	}
	lines := keyLines(source)
	m.name, _ = raw["name"].(string)
	m.name = strings.ToLower(strings.TrimSpace(m.name))
	m.version, _ = raw["version"].(string)
	eachTable(raw["depends-on"], func(t map[string]any) {
		dependsOn(t, func(name, constraint string) {
			name = strings.ToLower(name)
			if _, ok := m.dependencies[name]; !ok {
				m.dependencies[name] = &dependency{name: name, constraint: strings.TrimSpace(constraint), line: lines.find("depends-on", name)}
			}
		})
	})
	eachTable(raw["pins"], func(t map[string]any) {
		for name, v := range t {
			name = strings.ToLower(name)
			if _, ok := m.pins[name]; ok {
				continue
			}
			p := &pin{name: name, line: lines.find("pins", name)}
			switch v := v.(type) {
			case map[string]any:
				stringField := func(k string) string {
					s, _ := v[k].(string)
					return strings.TrimSpace(s)
				}
				p.path, p.url, p.commit, p.branch, p.version, p.subdirectory = stringField("path"), stringField("url"), stringField("commit"), stringField("branch"), stringField("version"), stringField("subdir")
			case string:
				p.version = strings.TrimSpace(v)
			}
			m.pins[name] = p
		}
	})
	strings_(raw["project-files"], func(s string) {
		m.projectFiles = append(m.projectFiles, item{s, lines.find("", "project-files")})
	})
	return m
}

// eachTable calls function for a table or each table of an array of tables.
func eachTable(v any, function func(map[string]any)) {
	switch v := v.(type) {
	case map[string]any:
		function(v)
	case []map[string]any:
		for _, t := range v {
			function(t)
		}
	case []any:
		for _, x := range v {
			if t, ok := x.(map[string]any); ok {
				function(t)
			}
		}
	}
}

// dependsOn walks a depends-on table: crate = "constraint" entries, and every
// alternative of 'case(os)' and similar expressions.
func dependsOn(t map[string]any, function func(name, constraint string)) {
	for _, k := range sortedKeys(t) {
		switch v := t[k].(type) {
		case string:
			function(k, v)
		case map[string]any:
			if strings.HasPrefix(k, "case(") {
				for _, alternative := range sortedKeys(v) {
					if at, ok := v[alternative].(map[string]any); ok {
						dependsOn(at, function)
					}
				}
			} else if s, ok := v["version"].(string); ok {
				function(k, s)
			}
		}
	}
}

// strings_ calls function for every string in v, looking into arrays and case(...)
// tables.
func strings_(v any, function func(string)) {
	switch v := v.(type) {
	case string:
		function(v)
	case []any:
		for _, x := range v {
			strings_(x, function)
		}
	case map[string]any:
		for _, k := range sortedKeys(v) {
			strings_(v[k], function)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// lineIndex finds the line of a key: BurntSushi's decoder keeps no positions.
type lineIndex []keyLine

type keyLine struct {
	section, key string
	line         int
}

var (
	headerRe = regexp.MustCompile(`^\s*\[\[?\s*([^\]]+?)\s*\]\]?`)
	keyRe    = regexp.MustCompile(`^\s*("[^"]*"|'[^']*'|[A-Za-z0-9_.\-]+)\s*=`)
)

func keyLines(source []byte) lineIndex {
	var out lineIndex
	section := ""
	for n, line := range strings.Split(string(source), "\n") {
		if m := headerRe.FindStringSubmatch(line); m != nil {
			section = strings.ReplaceAll(strings.ReplaceAll(m[1], `"`, ""), "'", "")
			out = append(out, keyLine{section: section, line: n + 1})
			continue
		}
		if m := keyRe.FindStringSubmatch(line); m != nil {
			k := strings.Trim(m[1], `"'`)
			out = append(out, keyLine{section: section, key: k, line: n + 1})
		}
	}
	return out
}

// find is the line of key (case-insensitive) in a section starting with
// prefix, or of a [prefix.key] header.
func (x lineIndex) find(prefix, key string) int {
	key = strings.ToLower(key)
	for _, k := range x {
		if !strings.HasPrefix(k.section, prefix) {
			continue
		}
		if strings.ToLower(k.key) == key || k.key == "" && prefix != "" && strings.ToLower(k.section) == prefix+"."+key {
			return k.line
		}
	}
	return 1
}

// extractManifest turns an alire.toml into imports: its dependencies, the
// crates it pins without depending on them, and its project files.
//
// Implements: REQ-ADA-006
func extractManifest(source []byte) *lang.Extraction {
	m := readManifest(source)
	extraction := &lang.Extraction{}
	for _, name := range sortedKeys(m.dependencies) {
		d := m.dependencies[name]
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kindDependency, Line: d.line})
	}
	for _, name := range sortedKeys(m.pins) {
		if _, ok := m.dependencies[name]; !ok {
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kindPin, Line: m.pins[name].line})
		}
	}
	for _, f := range m.projectFiles {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: f.s, Module: f.s, Name: kindProjectFile, Line: f.line})
	}
	sort.SliceStable(extraction.Imports, func(i, j int) bool { return extraction.Imports[i].Line < extraction.Imports[j].Line })
	if m.name != "" {
		var symbols lang.SymbolSet
		symbols.Add(m.name, "crate", lines1(source, "name"))
		extraction.Symbols = symbols.List()
	}
	return extraction
}

func lines1(source []byte, key string) int {
	return keyLines(source).find("", key)
}

// lockState is a crate of an Alire lock file's solution.
type lockState struct {
	crate, versions, version string // versions: the constraint solved for
	linkPath, linkURL        string
	linkCommit, linkBranch   string
	dependencies             []string // the crates its release depends on
	constraints              map[string]string
}

// readLock reads alire.lock ([solution] with [[solution.state]] entries: each
// crate, the versions asked for, and the release chosen with its own
// depends-on, or the link a pin made). Alire 1.1 and later write it to
// alire/alire.lock; earlier versions wrote it beside alire.toml.
//
// Implements: REQ-ADA-006, REQ-ADA-008
func readLock(source []byte) map[string]*lockState {
	out := map[string]*lockState{}
	var raw map[string]any
	if _, err := toml.Decode(string(source), &raw); err != nil {
		return out
	}
	sol, _ := raw["solution"].(map[string]any)
	eachTable(sol["state"], func(t map[string]any) {
		stringField := func(m map[string]any, k string) string {
			s, _ := m[k].(string)
			return strings.TrimSpace(s)
		}
		state := &lockState{crate: strings.ToLower(stringField(t, "crate")), versions: stringField(t, "versions"), constraints: map[string]string{}}
		if state.crate == "" {
			return
		}
		if link, ok := t["link"].(map[string]any); ok {
			state.linkPath, state.linkURL, state.linkCommit, state.linkBranch = stringField(link, "path"), stringField(link, "url"), stringField(link, "commit"), stringField(link, "branch")
		}
		if relative, ok := t["release"].(map[string]any); ok {
			if _, named := relative["version"]; !named {
				// [solution.state.release.<crate>]
				if inner, ok := relative[state.crate].(map[string]any); ok {
					relative = inner
				}
			}
			state.version = stringField(relative, "version")
			eachTable(relative["depends-on"], func(d map[string]any) {
				dependsOn(d, func(name, c string) {
					name = strings.ToLower(name)
					if _, ok := state.constraints[name]; !ok {
						state.constraints[name] = strings.TrimSpace(c)
						state.dependencies = append(state.dependencies, name)
					}
				})
			})
		}
		out[state.crate] = state
	})
	return out
}

// exactVersion is the version an Alire constraint names alone: "=1.2.3" or a
// bare "1.2.3" (Semantic_Versioning reads it as exactly that version).
func exactVersion(c string) (string, bool) {
	c = strings.TrimSpace(c)
	c = strings.TrimSpace(strings.TrimPrefix(c, "="))
	if c == "" || strings.ContainsAny(c, "^~<>=/&|*() ") {
		return "", false
	}
	if c[0] < '0' || c[0] > '9' || !validSemver(c) {
		return "", false
	}
	return c, true
}

// ExactVersion is the release an Alire version or constraint names alone
// ("1.2.3", "=1.2.3"); a range, a commit or a branch names none.
func ExactVersion(c string) (string, bool) { return exactVersion(c) }

// Dependencies lists what a crate manifest (an alire.toml, or a release's
// manifest in the community index) depends on: each crate with its
// constraint, every alternative of a case(...) expression counted.
func Dependencies(source []byte) map[string]string {
	out := map[string]string{}
	for name, d := range readManifest(source).dependencies {
		out[name] = d.constraint
	}
	return out
}

// validSemver accepts what Semantic_Versioning parses relaxed: digits and dots,
// then a pre-release or build part.
func validSemver(v string) bool {
	core := v
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		core = v[:i]
	}
	for _, part := range strings.Split(core, ".") {
		if part == "" {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}
