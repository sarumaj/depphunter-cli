package puppet

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dep is a module a Puppetfile, a metadata.json or a .fixtures.yml names.
type dep struct {
	key      string // unique within its file: what the manifest's import carries
	name     string // the module's short name: stdlib
	pkg      string // the package: a Forge slug (puppetlabs-stdlib) or a repository
	version  string
	pinned   bool
	floating bool
	origin   string // the git URL of a module installed from git
	forge    bool   // pkg is a Forge slug
	line     int
}

func (d *dep) target() lang.Target {
	return lang.Target{Ecosystem: ecoForge, Package: d.pkg, Version: d.version, Pinned: d.pinned, Floating: d.floating, Origin: d.origin}
}

// slug is the Forge's name for a module: author-name, lower case
// (puppetlabs/stdlib is written too).
func slug(s string) string {
	return strings.ToLower(strings.Replace(strings.TrimSpace(s), "/", "-", 1))
}

// short is a module's name without its author: stdlib.
func short(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, "-/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// gitRef pins a commit (a full or abbreviated hash), leaves a tag - a
// version - neither pinned nor floating, and floats a branch.
func gitRef(d *dep, ref string) {
	d.version = ref
	switch {
	case ref == "":
		d.floating = true
	case hexRef(ref):
		d.pinned = true
	case !lang.Pinned(ref):
		d.floating = true
	}
}

func hexRef(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	letter := false
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'f':
			letter = true
		case c < '0' || c > '9':
			return false
		}
	}
	return letter || len(s) == 40
}

// puppetfile is what an r10k (or Code Manager, g10k, librarian-puppet)
// Puppetfile declares.
type puppetfile struct {
	deps      []*dep
	moduledir string // where the modules are installed, relative to the Puppetfile
}

// readPuppetfile reads a Puppetfile's `mod` and `moduledir` statements. A
// Forge module is `mod 'author-name'` with an optional version or :latest;
// a git module is `mod 'name', :git => url` with :commit, :ref, :tag or
// :branch (hash-rocket or `git:` keyword style); :local modules are part of
// the repository.
//
// Implements: REQ-PUPPET-005
func readPuppetfile(src []byte) *puppetfile {
	pf := &puppetfile{moduledir: "modules"}
	for _, st := range rubyStatements(src) {
		if len(st) < 2 || st[0].kind != rIdent || st[1].kind != rString {
			continue
		}
		switch st[0].text {
		case "moduledir":
			pf.moduledir = strings.Trim(st[1].text, "/")
			continue
		case "mod":
		default:
			continue
		}
		name := st[1].text
		d := &dep{key: name, name: short(name), line: st[1].line}
		opts := map[string]string{}
		version, latest := "", false
		for i := 2; i < len(st); i++ {
			switch t := st[i]; {
			case t.kind == rString && i == 3:
				version = t.text
			case t.kind == rSymbol && i == 3 && t.text == "latest":
				latest = true
			case t.kind == rSymbol || t.kind == rLabel:
				// :key => value or key: value
				j := i + 1
				if j < len(st) && st[j].kind == rArrow {
					j++
				}
				if j < len(st) && (st[j].kind == rString || st[j].kind == rSymbol || st[j].kind == rIdent) {
					opts[t.text] = st[j].text
					i = j
				}
			}
		}
		switch {
		case opts["local"] == "true":
			continue
		case opts["git"] != "" || opts["svn"] != "":
			url := opts["git"] + opts["svn"]
			d.pkg, d.origin = lang.RepoName(url), url
			switch {
			case opts["commit"] != "":
				d.version, d.pinned = opts["commit"], true
			case opts["tag"] != "":
				d.version = opts["tag"]
			case opts["branch"] != "":
				d.version, d.floating = opts["branch"], true
			case opts["rev"] != "": // svn
				d.version, d.pinned = opts["rev"], true
			default:
				gitRef(d, opts["ref"])
			}
			if d.version == "control_branch" {
				d.pinned = false
				d.floating = true
			}
		default:
			d.pkg, d.forge = slug(name), true
			if version == "" {
				version = opts["version"]
			}
			d.version = version
			d.pinned = !latest && lang.Pinned(version)
			d.floating = !d.pinned
		}
		pf.deps = append(pf.deps, d)
	}
	return pf
}

type rKind uint8

const (
	rIdent rKind = iota
	rString
	rSymbol // :git
	rLabel  // git: (keyword argument)
	rArrow  // =>
	rPunct
)

type rTok struct {
	kind rKind
	text string
	line int
}

// rubyStatements splits the Ruby of a Puppetfile into statements: a line
// ending in a comma, an arrow or an open bracket continues on the next.
func rubyStatements(src []byte) [][]rTok {
	s := string(src)
	var out [][]rTok
	var cur []rTok
	line, depth := 1, 0
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
			cur = nil
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
			if depth <= 0 && (len(cur) == 0 || cur[len(cur)-1].kind != rArrow && (cur[len(cur)-1].kind != rPunct || cur[len(cur)-1].text != ",")) {
				flush()
			}
		case c == '#':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '\'' || c == '"':
			j := i + 1
			var b strings.Builder
			for ; j < len(s) && s[j] != c; j++ {
				if s[j] == '\\' && j+1 < len(s) {
					j++
				}
				if s[j] == '\n' {
					line++
				}
				b.WriteByte(s[j])
			}
			cur = append(cur, rTok{rString, b.String(), line})
			i = min(j+1, len(s))
		case c == ':' && isWord(at(s, i+1)):
			j := i + 1
			for j < len(s) && isWord(s[j]) {
				j++
			}
			cur = append(cur, rTok{rSymbol, s[i+1 : j], line})
			i = j
		case isWord(c):
			j := i
			for j < len(s) && (isWord(s[j]) || s[j] == '?' || s[j] == '!') {
				j++
			}
			if at(s, j) == ':' && at(s, j+1) != ':' {
				cur = append(cur, rTok{rLabel, s[i:j], line})
				i = j + 1
				continue
			}
			cur = append(cur, rTok{rIdent, s[i:j], line})
			i = j
		case c == '=' && at(s, i+1) == '>':
			cur = append(cur, rTok{rArrow, "=>", line})
			i += 2
		case c == ' ' || c == '\t' || c == '\r':
			i++
		default:
			switch c {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
			cur = append(cur, rTok{rPunct, string(c), line})
			i++
		}
	}
	flush()
	return out
}

// metadata is what a Puppet module's metadata.json says of it.
type metadata struct {
	name    string // author-name
	version string
	deps    []*dep
	line    int
}

var moduleName = regexp.MustCompile(`^[A-Za-z0-9]+[-/][a-z][a-z0-9_]*$`)

// readMetadata reads a module's metadata.json, or returns nil for a
// metadata.json that is not a Puppet module's: it names the module
// author-name and says what it depends on, which platforms it supports or
// where its source is. A dependency's version_requirement is a range, which
// floats; an exact version pins.
//
// Implements: REQ-PUPPET-005
func readMetadata(src []byte) *metadata {
	var raw struct {
		Name         string          `json:"name"`
		Version      string          `json:"version"`
		Source       string          `json:"source"`
		Support      json.RawMessage `json:"operatingsystem_support"`
		Requirements json.RawMessage `json:"requirements"`
		Dependencies []struct {
			Name    string `json:"name"`
			Version string `json:"version_requirement"`
		} `json:"dependencies"`
	}
	if json.Unmarshal(src, &raw) != nil || !moduleName.MatchString(raw.Name) {
		return nil
	}
	if raw.Dependencies == nil && raw.Support == nil && raw.Source == "" && raw.Requirements == nil {
		return nil
	}
	m := &metadata{name: slug(raw.Name), version: raw.Version, line: lineOf(src, raw.Name, 0)}
	from := 0
	for _, d := range raw.Dependencies {
		if !moduleName.MatchString(d.Name) {
			continue
		}
		line := lineOf(src, d.Name, from)
		from = max(from, bytes.Index(src, []byte(`"`+d.Name+`"`)))
		v := strings.TrimSpace(d.Version)
		pinned := lang.Pinned(v)
		m.deps = append(m.deps, &dep{key: slug(d.Name), name: short(d.Name), pkg: slug(d.Name), version: v, pinned: pinned, floating: !pinned, forge: true, line: line})
	}
	return m
}

// lineOf is the line of the first quoted s at or after byte from.
func lineOf(src []byte, s string, from int) int {
	from = max(from, 0)
	i := bytes.Index(src[from:], []byte(`"`+s+`"`))
	if i < 0 {
		return 1
	}
	return 1 + bytes.Count(src[:from+i], []byte("\n"))
}

// readFixtures reads puppetlabs_spec_helper's .fixtures.yml: the modules its
// forge_modules and repositories install for the module's tests (a string,
// or a map with repo, ref and branch). A Forge module pins by an exact ref;
// a repository as a Puppetfile's git :ref does, and floats on a branch.
//
// Implements: REQ-PUPPET-005
func readFixtures(src []byte) []*dep {
	var doc yaml.Node
	if yaml.Unmarshal(src, &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	fixtures := mapValue(doc.Content[0], "fixtures")
	var out []*dep
	for _, section := range []string{"forge_modules", "repositories"} {
		m := mapValue(fixtures, section)
		if m == nil || m.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], m.Content[i+1]
			repo, ref, branch := v.Value, "", ""
			if v.Kind == yaml.MappingNode {
				repo = mapValue(v, "repo").Value
				ref = mapValue(v, "ref").Value
				branch = mapValue(v, "branch").Value
			}
			if repo == "" {
				continue
			}
			d := &dep{key: section + ":" + k.Value, name: strings.ToLower(k.Value), line: k.Line}
			if section == "forge_modules" {
				d.pkg, d.forge, d.version = slug(repo), true, ref
				d.pinned = lang.Pinned(ref)
				d.floating = !d.pinned
			} else {
				d.pkg, d.origin = lang.RepoName(repo), repo
				if branch != "" && ref == "" {
					d.version, d.floating = branch, true
				} else {
					gitRef(d, ref)
				}
			}
			out = append(out, d)
		}
	}
	return out
}

var empty = &yaml.Node{}

// mapValue is the value of key in a YAML mapping, or an empty node.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n != nil && n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				return n.Content[i+1]
			}
		}
	}
	return empty
}
