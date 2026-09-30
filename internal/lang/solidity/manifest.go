package solidity

import (
	"bufio"
	"bytes"
	"cmp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds of the manifests, carried in RawImport.Name.
const (
	kindRemap      = "remap"     // a remapping, in foundry.toml or remappings.txt
	kindDependency = "dep"       // a Soldeer dependency in foundry.toml
	kindLock       = "lock"      // an entry of soldeer.lock
	kindSubmodule  = "submodule" // a submodule of .gitmodules
)

// remapping is solc's `context:prefix=target`.
type remapping struct {
	context, prefix, target string
}

// parseRemapping reads one remapping; ok is false for a line that is none.
func parseRemapping(s string) (remapping, bool) {
	s = strings.TrimSpace(s)
	lhs, target, ok := strings.Cut(s, "=")
	if !ok || lhs == "" {
		return remapping{}, false
	}
	var r remapping
	if ctx, prefix, ok := strings.Cut(lhs, ":"); ok {
		r.context, r.prefix = strings.TrimSpace(ctx), strings.TrimSpace(prefix)
	} else {
		r.prefix = strings.TrimSpace(lhs)
	}
	r.target = strings.TrimSpace(target)
	if r.prefix == "" {
		return remapping{}, false
	}
	return r, true
}

// remappingLines reads remappings.txt: one remapping per line; blank lines
// and # or // comments are skipped.
func remappingLines(source []byte) (out []string, lines []int) {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	scanner.Buffer(make([]byte, 0, 4096), lang.MaxParseSize+1)
	for n := 1; scanner.Scan(); n++ {
		l := strings.TrimSpace(scanner.Text())
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "//") {
			continue
		}
		if _, ok := parseRemapping(l); ok {
			out, lines = append(out, l), append(lines, n)
		}
	}
	return out, lines
}

// soldeerDependency is a dependency of foundry.toml's [dependencies] table: a
// version string, or a table with version and a git source or a url.
type soldeerDependency struct {
	name, version         string
	git, rev, tag, branch string
	url                   string
}

// foundryConfig is what the plugin reads of foundry.toml.
type foundryConfig struct {
	remappings   []string // every profile's, the default profile's first
	libraries    []string // the default profile's libs ("lib" when unset)
	source       string
	dependencies []soldeerDependency
	// Soldeer's remapping settings: a prefix before each generated
	// remapping's name and whether the version is part of it.
	soldeerPrefix    string
	soldeerNoVersion bool
	autoDetect       bool // auto_detect_remappings (true unless switched off)
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}

func stringList(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, x := range list {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// readFoundry decodes foundry.toml; ok is false when it is not TOML.
//
// Implements: REQ-SOLIDITY-005, REQ-SOLIDITY-007
func readFoundry(source []byte) (foundryConfig, bool) {
	var m map[string]any
	if _, err := toml.Decode(string(source), &m); err != nil {
		return foundryConfig{}, false
	}
	c := foundryConfig{libraries: []string{"lib"}, autoDetect: true}
	profiles, _ := m["profile"].(map[string]any)
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		if name != "default" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var tables []map[string]any
	if d, ok := profiles["default"].(map[string]any); ok {
		tables = append(tables, d)
	} else if d, ok := m["default"].(map[string]any); ok { // the old [default] section
		tables = append(tables, d)
	}
	defaults := len(tables) > 0
	for _, name := range names {
		if t, ok := profiles[name].(map[string]any); ok {
			tables = append(tables, t)
		}
	}
	seen := map[string]bool{}
	for i, t := range tables {
		for _, r := range stringList(t["remappings"]) {
			if !seen[r] {
				seen[r] = true
				c.remappings = append(c.remappings, r)
			}
		}
		if i == 0 && defaults {
			if libraries, ok := t["libs"].([]any); ok {
				c.libraries = nil
				for _, l := range libraries {
					if s, ok := l.(string); ok {
						c.libraries = append(c.libraries, strings.Trim(strings.TrimPrefix(s, "./"), "/"))
					}
				}
			}
			c.source = strings.Trim(strings.TrimPrefix(stringOf(t["src"]), "./"), "/")
			if v, ok := t["auto_detect_remappings"].(bool); ok {
				c.autoDetect = v
			}
		}
	}
	if s, ok := m["soldeer"].(map[string]any); ok {
		c.soldeerPrefix = stringOf(s["remappings_prefix"])
		if v, ok := s["remappings_version"].(bool); ok {
			c.soldeerNoVersion = !v
		}
	}
	dependencies, _ := m["dependencies"].(map[string]any)
	for name, v := range dependencies {
		d := soldeerDependency{name: name}
		switch v := v.(type) {
		case string:
			d.version = v
		case map[string]any:
			d.version, d.git, d.rev = stringOf(v["version"]), stringOf(v["git"]), stringOf(v["rev"])
			d.tag, d.branch, d.url = stringOf(v["tag"]), stringOf(v["branch"]), stringOf(v["url"])
		default:
			continue
		}
		c.dependencies = append(c.dependencies, d)
	}
	sort.Slice(c.dependencies, func(i, j int) bool { return c.dependencies[i].name < c.dependencies[j].name })
	return c, true
}

// lockEntry is an entry of soldeer.lock.
type lockEntry struct {
	name, version, url, git, rev, checksum string
}

// readSoldeerLock decodes soldeer.lock's [[dependencies]] entries.
//
// Implements: REQ-SOLIDITY-007
func readSoldeerLock(source []byte) []lockEntry {
	var m struct {
		Dependencies []map[string]any `toml:"dependencies"`
	}
	if _, err := toml.Decode(string(source), &m); err != nil {
		return nil
	}
	var out []lockEntry
	for _, d := range m.Dependencies {
		e := lockEntry{name: stringOf(d["name"]), version: stringOf(d["version"]), url: stringOf(d["url"]),
			git: stringOf(d["git"]), rev: stringOf(d["rev"]), checksum: stringOf(d["checksum"])}
		e.url = cmp.Or(e.url, stringOf(d["source"])) // the lock files of Soldeer before 0.3
		if e.name != "" {
			out = append(out, e)
		}
	}
	return out
}

// submodule is a [submodule] section of .gitmodules.
type submodule struct {
	name, path, url, branch string
	line                    int
}

// readGitmodules reads .gitmodules, which is git-config syntax.
//
// Implements: REQ-SOLIDITY-006
func readGitmodules(source []byte) []submodule {
	var out []submodule
	var current *submodule
	scanner := bufio.NewScanner(bytes.NewReader(source))
	scanner.Buffer(make([]byte, 0, 4096), lang.MaxParseSize+1)
	for n := 1; scanner.Scan(); n++ {
		l := strings.TrimSpace(scanner.Text())
		if l == "" || l[0] == '#' || l[0] == ';' {
			continue
		}
		if l[0] == '[' {
			current = nil
			head := strings.TrimSpace(strings.Trim(l, "[]"))
			if keyword, name, ok := strings.Cut(head, " "); ok && strings.EqualFold(keyword, "submodule") {
				out = append(out, submodule{name: strings.Trim(strings.TrimSpace(name), `"`), line: n})
				current = &out[len(out)-1]
			}
			continue
		}
		if current == nil {
			continue
		}
		key, value, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if i := strings.IndexAny(value, "#;"); i >= 0 && !strings.HasPrefix(value, `"`) {
			value = strings.TrimSpace(value[:i])
		}
		value = strings.Trim(value, `"`)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "path":
			current.path = strings.Trim(strings.TrimPrefix(value, "./"), "/")
		case "url":
			current.url = value
		case "branch":
			current.branch = value
		}
	}
	kept := out[:0]
	for _, s := range out {
		if s.path != "" {
			kept = append(kept, s)
		}
	}
	return kept
}

// tomlLines finds where a TOML file writes each of its keys, which the
// decoder does not say: the line of `key =` (or `"key" =`) inside a section,
// or of a `[section.key]` header, and the line of a quoted string anywhere.
type tomlLines struct {
	lines []string
}

func newTOMLLines(source []byte) tomlLines {
	return tomlLines{lines: strings.Split(string(source), "\n")}
}

// key returns the line of key in section (1-based), 0 when not found.
func (t tomlLines) key(section, key string) int {
	current := ""
	for i, l := range t.lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "[") {
			h := strings.TrimSpace(strings.Trim(l, "[]"))
			if h == section+"."+key || h == section+`."`+key+`"` {
				return i + 1
			}
			current = h
			continue
		}
		if current != section {
			continue
		}
		k, _, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		if k = strings.TrimSpace(k); k == key || k == `"`+key+`"` || k == "'"+key+"'" {
			return i + 1
		}
	}
	return 0
}

// quoted returns the line of the first "s" or 's' at or after from.
func (t tomlLines) quoted(s string, from int) int {
	for i := max(from, 0); i < len(t.lines); i++ {
		if strings.Contains(t.lines[i], `"`+s+`"`) || strings.Contains(t.lines[i], "'"+s+"'") {
			return i + 1
		}
	}
	return 0
}

// extractFoundry makes every remapping and every Soldeer dependency of a
// foundry.toml an import.
//
// Implements: REQ-SOLIDITY-005, REQ-SOLIDITY-007
func extractFoundry(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	c, ok := readFoundry(source)
	if !ok {
		return extraction
	}
	lines := newTOMLLines(source)
	for _, r := range c.remappings {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: r, Module: r, Name: kindRemap, Line: max(lines.quoted(r, 0), 1)})
	}
	for _, d := range c.dependencies {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: d.name, Module: d.name, Name: kindDependency, Line: max(lines.key("dependencies", d.name), 1)})
	}
	return extraction
}

// extractRemappings makes every line of remappings.txt an import.
func extractRemappings(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	list, lines := remappingLines(source)
	for i, r := range list {
		if !seen[r] {
			seen[r] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: r, Module: r, Name: kindRemap, Line: lines[i]})
		}
	}
	return extraction
}

// extractLock makes every entry of soldeer.lock an import.
func extractLock(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	lines := newTOMLLines(source)
	seen := map[string]bool{}
	from := 0
	for _, e := range readSoldeerLock(source) {
		if seen[e.name] {
			continue
		}
		seen[e.name] = true
		line := lines.quoted(e.name, from)
		if line > 0 {
			from = line
		}
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: e.name, Module: e.name, Name: kindLock, Line: max(line, 1)})
	}
	return extraction
}

// extractGitmodules makes every submodule an import.
func extractGitmodules(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	for _, s := range readGitmodules(source) {
		if !seen[s.path] {
			seen[s.path] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: s.path, Module: s.path, Name: kindSubmodule, Line: s.line})
		}
	}
	return extraction
}
