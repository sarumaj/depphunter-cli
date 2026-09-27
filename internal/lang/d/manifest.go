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
	name     string // as written: vibe-d, vibe-d:http, :sub
	version  string
	path     string
	repo     string
	optional bool
	line     int
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
	name         string
	deps         []*dependency
	subs         []subPackage
	sourcePaths  []string
	importPaths  []string
	stringPaths  []string
	sourcesSet   bool
	importsSet   bool
	stringSet    bool
	nameLine     int
	dependencies map[string]*dependency // by name as written, first entry wins
}

func (r *recipe) addDep(d *dependency) {
	if r.dependencies == nil {
		r.dependencies = map[string]*dependency{}
	}
	if d.name == "" || r.dependencies[d.name] != nil {
		return
	}
	r.dependencies[d.name] = d
	r.deps = append(r.deps, d)
}

// selection is an entry of dub.selections.json.
type selection struct {
	version, path, repo string
	line                int
}

// lineOf is the line of the first occurrence of needle in src at or after
// from, else 1.
func lineOf(src []byte, needle string, from int) (int, int) {
	if from > len(src) {
		from = len(src)
	}
	i := bytes.Index(src[from:], []byte(needle))
	if i < 0 {
		return 1, from
	}
	return bytes.Count(src[:from+i], []byte("\n")) + 1, from + i + len(needle)
}

// keyLine is the line of the first "key": in a JSON document, else 1.
func keyLine(src []byte, key string) int {
	if loc := regexp.MustCompile(regexp.QuoteMeta(`"`+key+`"`) + `\s*:`).FindIndex(src); loc != nil {
		return bytes.Count(src[:loc[0]], []byte("\n")) + 1
	}
	return 1
}

// readJSONRecipe reads dub.json.
//
// Implements: REQ-DLANG-005
func readJSONRecipe(src []byte) *recipe {
	var doc map[string]json.RawMessage
	if json.Unmarshal(src, &doc) != nil {
		return &recipe{}
	}
	return jsonRecipe(doc, src)
}

func jsonRecipe(doc map[string]json.RawMessage, src []byte) *recipe {
	r := &recipe{}
	str := func(raw json.RawMessage) string {
		var s string
		json.Unmarshal(raw, &s)
		return s
	}
	r.name = str(doc["name"])
	if r.name != "" {
		r.nameLine, _ = lineOf(src, `"`+r.name+`"`, 0)
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
	readDeps := func(raw json.RawMessage) {
		var deps map[string]json.RawMessage
		if json.Unmarshal(raw, &deps) != nil {
			return
		}
		for _, name := range sortedKeys(deps) {
			d := &dependency{name: name, line: keyLine(src, name)}
			var spec string
			if json.Unmarshal(deps[name], &spec) == nil {
				d.version = strings.TrimSpace(spec)
			} else {
				var o struct {
					Version    string `json:"version"`
					Path       string `json:"path"`
					Repository string `json:"repository"`
					Optional   bool   `json:"optional"`
				}
				json.Unmarshal(deps[name], &o)
				d.version, d.path, d.repo, d.optional = strings.TrimSpace(o.Version), o.Path, o.Repository, o.Optional
			}
			r.addDep(d)
		}
	}
	readDeps(doc["dependencies"])
	var configs []map[string]json.RawMessage
	if json.Unmarshal(doc["configurations"], &configs) == nil {
		for _, c := range configs {
			readDeps(c["dependencies"])
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
				line, _ := lineOf(src, `"`+p+`"`, 0)
				r.subs = append(r.subs, subPackage{path: p, line: line})
				continue
			}
			var sub map[string]json.RawMessage
			if json.Unmarshal(raw, &sub) == nil {
				in := jsonRecipe(sub, src)
				r.subs = append(r.subs, subPackage{inline: in, line: in.nameLine})
			}
		}
	}
	return r
}

// readSDLRecipe reads dub.sdl.
//
// Implements: REQ-DLANG-005
func readSDLRecipe(src []byte) *recipe {
	return sdlRecipe(readSDL(src))
}

func sdlRecipe(tags []*sdlTag) *recipe {
	r := &recipe{}
	for _, t := range tags {
		switch t.name {
		case "name":
			r.name, r.nameLine = t.value(), t.line
		case "dependency":
			d := &dependency{name: t.value(), version: strings.TrimSpace(t.attr("version")), path: t.attr("path"),
				repo: t.attr("repository"), optional: t.attr("optional") == "true", line: t.line}
			r.addDep(d)
		case "sourcePaths":
			r.sourcePaths, r.sourcesSet = append(r.sourcePaths, t.values...), true
		case "importPaths":
			r.importPaths, r.importsSet = append(r.importPaths, t.values...), true
		case "stringImportPaths":
			r.stringPaths, r.stringSet = append(r.stringPaths, t.values...), true
		case "configuration":
			// A configuration's dependencies and paths add to the package's.
			c := sdlRecipe(t.children)
			for _, d := range c.deps {
				r.addDep(d)
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
func readSelections(src []byte) map[string]*selection {
	var doc struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	out := map[string]*selection{}
	if json.Unmarshal(src, &doc) != nil {
		return out
	}
	for _, name := range sortedKeys(doc.Versions) {
		s := &selection{line: keyLine(src, name)}
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
			s.version, s.path, s.repo = strings.TrimSpace(o.Version), o.Path, o.Repository
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
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	var walk func(r *recipe)
	walk = func(r *recipe) {
		for _, d := range r.deps {
			if seen[d.name] {
				continue
			}
			seen[d.name] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: d.name, Module: d.name, Name: kindDep, Line: d.line})
		}
		for _, s := range r.subs {
			if s.inline != nil {
				walk(s.inline)
			} else if p := strings.TrimRight(s.path, "/"); p != "" && !seen["\x00"+p] {
				seen["\x00"+p] = true
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: s.path, Module: p, Name: kindSubPath, Line: s.line})
			}
		}
	}
	walk(r)
	return ex
}

// Implements: REQ-DLANG-006
func extractSelections(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	sel := readSelections(src)
	for _, name := range sortedKeys(sel) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kindSelected, Line: sel[name].line})
	}
	sort.SliceStable(ex.Imports, func(i, j int) bool { return ex.Imports[i].Line < ex.Imports[j].Line })
	return ex
}

// pinRule is dub's pinning of a version specification without a selection: an
// exact version - "==1.2.3" or a bare "1.2.3", which dub reads as ==1.2.3 -
// pins; ~>, ^, >=, ranges, * and ~branch float. A git repository pins by a
// commit only.
//
// Implements: REQ-DLANG-006
func pinRule(t *lang.Target, spec string, repo bool) {
	spec = strings.TrimSpace(spec)
	switch {
	case repo && lang.Commit(spec):
		t.Version, t.Pinned = spec, true
	case repo:
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
func singleFile(src []byte) (*recipe, int) {
	s := string(bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")))
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
	offset := strings.Count(string(src[:min(len(src), skipped+2+(len(rest)-len(strings.TrimLeft(rest, " \t\r\n")))+colon)]), "\n")
	text := []byte(body[:end])
	var rec *recipe
	if format == "sdl" {
		rec = readSDLRecipe(text)
	} else {
		rec = readJSONRecipe(text)
	}
	return rec, offset
}
