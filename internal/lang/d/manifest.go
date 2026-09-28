package d

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one entry of a recipe's dependencies: a registry package by
// version specification, a path, or a git repository at a commit.
type dependency struct {
	name       string // as written: vibe-d, vibe-d:http, :sub
	version    string
	path       string
	repository string
	optional   bool
	line       int
}

// subPackage is an entry of a recipe's subPackages: a directory with a recipe
// of its own, or a recipe written inline.
type subPackage struct {
	path   string
	inline *recipe
	line   int
}

// recipe is what dub.json or dub.sdl says about a package (or an inline
// sub-package): its name, its dependencies (the configurations' too), its
// sub-packages and where its sources and string imports are. A nil path list
// means dub's default.
type recipe struct {
	name           string
	dependencyList []*dependency
	subs           []subPackage
	sourcePaths    []string
	importPaths    []string
	stringPaths    []string
	sourcesSet     bool
	importsSet     bool
	stringSet      bool
	nameLine       int
	dependencies   map[string]*dependency // by name as written, first entry wins
}

func (r *recipe) addDependency(d *dependency) {
	if r.dependencies == nil {
		r.dependencies = map[string]*dependency{}
	}
	if d.name == "" || r.dependencies[d.name] != nil {
		return
	}
	r.dependencies[d.name] = d
	r.dependencyList = append(r.dependencyList, d)
}

// selection is an entry of dub.selections.json.
type selection struct {
	version, path, repository string
	line                      int
}

// lineOf is the line of the first occurrence of needle in source at or after
// from, else 1.
func lineOf(source []byte, needle string, from int) (int, int) {
	if from > len(source) {
		from = len(source)
	}
	i := bytes.Index(source[from:], []byte(needle))
	if i < 0 {
		return 1, from
	}
	return bytes.Count(source[:from+i], []byte("\n")) + 1, from + i + len(needle)
}

// keyLine is the line of the first "key": in a JSON document, else 1.
func keyLine(source []byte, key string) int {
	if span := regexp.MustCompile(regexp.QuoteMeta(`"`+key+`"`) + `\s*:`).FindIndex(source); span != nil {
		return bytes.Count(source[:span[0]], []byte("\n")) + 1
	}
	return 1
}

// readJSONRecipe reads dub.json.
//
// Implements: REQ-DLANG-005
func readJSONRecipe(source []byte) *recipe {
	var doc map[string]json.RawMessage
	if json.Unmarshal(source, &doc) != nil {
		return &recipe{}
	}
	return jsonRecipe(doc, source)
}

func jsonRecipe(doc map[string]json.RawMessage, source []byte) *recipe {
	r := &recipe{}
	stringOf := func(raw json.RawMessage) string {
		var s string
		json.Unmarshal(raw, &s)
		return s
	}
	r.name = stringOf(doc["name"])
	if r.name != "" {
		r.nameLine, _ = lineOf(source, `"`+r.name+`"`, 0)
	}
	paths := func(key string) ([]string, bool) {
		var out []string
		set := false
		for _, k := range sortedKeys(doc) {
			if k != key && !strings.HasPrefix(k, key+"-") {
				continue // sourcePaths-windows and the like add to the list
			}
			var list []string
			if json.Unmarshal(doc[k], &list) == nil {
				out = append(out, list...)
				set = true
			}
		}
		return out, set
	}
	r.sourcePaths, r.sourcesSet = paths("sourcePaths")
	r.importPaths, r.importsSet = paths("importPaths")
	r.stringPaths, r.stringSet = paths("stringImportPaths")
	readDependencies := func(raw json.RawMessage) {
		var dependencies map[string]json.RawMessage
		if json.Unmarshal(raw, &dependencies) != nil {
			return
		}
		for _, name := range sortedKeys(dependencies) {
			d := &dependency{name: name, line: keyLine(source, name)}
			var spec string
			if json.Unmarshal(dependencies[name], &spec) == nil {
				d.version = strings.TrimSpace(spec)
			} else {
				var o struct {
					Version    string `json:"version"`
					Path       string `json:"path"`
					Repository string `json:"repository"`
					Optional   bool   `json:"optional"`
				}
				json.Unmarshal(dependencies[name], &o)
				d.version, d.path, d.repository, d.optional = strings.TrimSpace(o.Version), o.Path, o.Repository, o.Optional
			}
			r.addDependency(d)
		}
	}
	readDependencies(doc["dependencies"])
	var configs []map[string]json.RawMessage
	if json.Unmarshal(doc["configurations"], &configs) == nil {
		for _, c := range configs {
			readDependencies(c["dependencies"])
			// A configuration's paths add to the package's.
			for _, k := range sortedKeys(c) {
				var list []string
				if json.Unmarshal(c[k], &list) != nil {
					continue
				}
				switch {
				case k == "sourcePaths" || strings.HasPrefix(k, "sourcePaths-"):
					r.sourcePaths, r.sourcesSet = append(r.sourcePaths, list...), true
				case k == "importPaths" || strings.HasPrefix(k, "importPaths-"):
					r.importPaths, r.importsSet = append(r.importPaths, list...), true
				case k == "stringImportPaths" || strings.HasPrefix(k, "stringImportPaths-"):
					r.stringPaths, r.stringSet = append(r.stringPaths, list...), true
				}
			}
		}
	}
	var subs []json.RawMessage
	if json.Unmarshal(doc["subPackages"], &subs) == nil {
		for _, raw := range subs {
			var p string
			if json.Unmarshal(raw, &p) == nil {
				line, _ := lineOf(source, `"`+p+`"`, 0)
				r.subs = append(r.subs, subPackage{path: p, line: line})
				continue
			}
			var inlineRecipe map[string]json.RawMessage
			if json.Unmarshal(raw, &inlineRecipe) == nil {
				in := jsonRecipe(inlineRecipe, source)
				r.subs = append(r.subs, subPackage{inline: in, line: in.nameLine})
			}
		}
	}
	return r
}

// readSDLRecipe reads dub.sdl.
//
// Implements: REQ-DLANG-005
func readSDLRecipe(source []byte) *recipe {
	return sdlRecipe(readSDL(source))
}

func sdlRecipe(tags []*sdlTag) *recipe {
	r := &recipe{}
	for _, t := range tags {
		switch t.name {
		case "name":
			r.name, r.nameLine = t.value(), t.line
		case "dependency":
			d := &dependency{name: t.value(), version: strings.TrimSpace(t.attribute("version")), path: t.attribute("path"),
				repository: t.attribute("repository"), optional: t.attribute("optional") == "true", line: t.line}
			r.addDependency(d)
		case "sourcePaths":
			r.sourcePaths, r.sourcesSet = append(r.sourcePaths, t.values...), true
		case "importPaths":
			r.importPaths, r.importsSet = append(r.importPaths, t.values...), true
		case "stringImportPaths":
			r.stringPaths, r.stringSet = append(r.stringPaths, t.values...), true
		case "configuration":
			// A configuration's dependencies and paths add to the package's.
			c := sdlRecipe(t.children)
			for _, d := range c.dependencyList {
				r.addDependency(d)
			}
			if c.sourcesSet {
				r.sourcePaths, r.sourcesSet = append(r.sourcePaths, c.sourcePaths...), true
			}
			if c.importsSet {
				r.importPaths, r.importsSet = append(r.importPaths, c.importPaths...), true
			}
			if c.stringSet {
				r.stringPaths, r.stringSet = append(r.stringPaths, c.stringPaths...), true
			}
		case "subPackage":
			if len(t.children) > 0 {
				in := sdlRecipe(t.children)
				r.subs = append(r.subs, subPackage{inline: in, line: t.line})
			} else if p := t.value(); p != "" {
				r.subs = append(r.subs, subPackage{path: p, line: t.line})
			}
		}
	}
	return r
}

// readSelections reads dub.selections.json: package -> "version", or an object
// with a version, a path or a repository and its commit.
//
// Implements: REQ-DLANG-006
func readSelections(source []byte) map[string]*selection {
	var doc struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	out := map[string]*selection{}
	if json.Unmarshal(source, &doc) != nil {
		return out
	}
	for _, name := range sortedKeys(doc.Versions) {
		s := &selection{line: keyLine(source, name)}
		var v string
		if json.Unmarshal(doc.Versions[name], &v) == nil {
			s.version = strings.TrimSpace(v)
		} else {
			var o struct {
				Version    string `json:"version"`
				Path       string `json:"path"`
				Repository string `json:"repository"`
			}
			json.Unmarshal(doc.Versions[name], &o)
			s.version, s.path, s.repository = strings.TrimSpace(o.Version), o.Path, o.Repository
		}
		out[name] = s
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

// extractRecipe lists a recipe's dependencies, its configurations' and its
// inline sub-packages', and its sub-package directories, as imports.
//
// Implements: REQ-DLANG-005
func extractRecipe(r *recipe) *lang.Extraction {
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	var walk func(r *recipe)
	walk = func(r *recipe) {
		for _, d := range r.dependencyList {
			if seen[d.name] {
				continue
			}
			seen[d.name] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: d.name, Module: d.name, Name: kindDependency, Line: d.line})
		}
		for _, s := range r.subs {
			if s.inline != nil {
				walk(s.inline)
			} else if p := strings.TrimRight(s.path, "/"); p != "" && !seen["\x00"+p] {
				seen["\x00"+p] = true
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: s.path, Module: p, Name: kindSubPath, Line: s.line})
			}
		}
	}
	walk(r)
	return extraction
}

// Implements: REQ-DLANG-006
func extractSelections(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	selections := readSelections(source)
	for _, name := range sortedKeys(selections) {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kindSelected, Line: selections[name].line})
	}
	sort.SliceStable(extraction.Imports, func(i, j int) bool { return extraction.Imports[i].Line < extraction.Imports[j].Line })
	return extraction
}

// pinRule is dub's pinning of a version specification without a selection: an
// exact version - "==1.2.3" or a bare "1.2.3", which dub reads as ==1.2.3 -
// pins; ~>, ^, >=, ranges, * and ~branch float. A git repository pins by a
// commit only.
//
// Implements: REQ-DLANG-006
func pinRule(t *lang.Target, spec string, repository bool) {
	spec = strings.TrimSpace(spec)
	switch {
	case repository && lang.Commit(spec):
		t.Version, t.Pinned = spec, true
	case repository:
		t.Version, t.Floating = spec, true
	case strings.HasPrefix(spec, "=="):
		v := strings.TrimSpace(spec[2:])
		t.Version = v
		t.Pinned = lang.Pinned(v)
		t.Floating = !t.Pinned
	case spec != "" && spec[0] >= '0' && spec[0] <= '9' && lang.Pinned(spec):
		t.Version, t.Pinned = spec, true
	default:
		t.Version, t.Floating = spec, true
	}
}

// singleFile reads the recipe a single-file package embeds in a comment at the
// top of its module - /+ dub.sdl: ... +/ or /+ dub.json: ... +/, after an
// optional #! line - and the number of lines before the recipe's text; nil
// when the module has none.
//
// Implements: REQ-DLANG-005
func singleFile(source []byte) (*recipe, int) {
	s := string(bytes.TrimPrefix(source, []byte("\xef\xbb\xbf")))
	skipped := 0
	if strings.HasPrefix(s, "#!") {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return nil, 0
		}
		s, skipped = s[i+1:], i+1
	}
	t := strings.TrimLeft(s, " \t\r\n")
	skipped += len(s) - len(t)
	rest, ok := strings.CutPrefix(t, "/+")
	if !ok {
		return nil, 0
	}
	body := strings.TrimLeft(rest, " \t\r\n")
	format := ""
	switch {
	case strings.HasPrefix(body, "dub.sdl:"):
		format = "sdl"
	case strings.HasPrefix(body, "dub.json:"):
		format = "json"
	default:
		return nil, 0
	}
	colon := strings.IndexByte(body, ':') + 1
	body = body[colon:]
	end := strings.Index(body, "+/")
	if end < 0 {
		end = len(body)
	}
	offset := strings.Count(string(source[:min(len(source), skipped+2+(len(rest)-len(strings.TrimLeft(rest, " \t\r\n")))+colon)]), "\n")
	text := []byte(body[:end])
	var current *recipe
	if format == "sdl" {
		current = readSDLRecipe(text)
	} else {
		current = readJSONRecipe(text)
	}
	return current, offset
}
