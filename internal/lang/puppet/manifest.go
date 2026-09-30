package puppet

import (
	"bytes"
	"cmp"
	"encoding/json"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
)

// dependency is a module a Puppetfile, a metadata.json or a .fixtures.yml names.
type dependency struct {
	key         string // unique within its file: what the manifest's import carries
	name        string // the module's short name: stdlib
	packageName string // the package: a Forge slug (puppetlabs-stdlib) or a repository
	version     string
	pinned      bool
	floating    bool
	origin      string // the git URL of a module installed from git
	forge       bool   // packageName is a Forge slug
	line        int
}

func (d *dependency) target() lang.Target {
	return lang.Target{Ecosystem: ecosystemForge, Package: d.packageName, Version: d.version, Pinned: d.pinned, Floating: d.floating, Origin: d.origin}
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

// gitReference pins a commit (a full or abbreviated hash), leaves a tag - a
// version - neither pinned nor floating, and floats a branch.
func gitReference(d *dependency, reference string) {
	d.version = reference
	switch {
	case reference == "":
		d.floating = true
	case hexReference(reference):
		d.pinned = true
	case !lang.Pinned(reference):
		d.floating = true
	}
}

func hexReference(s string) bool {
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
	dependencies    []*dependency
	moduleDirectory string // where the modules are installed, relative to the Puppetfile
}

// readPuppetfile reads a Puppetfile's `mod` and `moduledir` statements. A
// Forge module is `mod 'author-name'` with an optional version or :latest;
// a git module is `mod 'name', :git => url` with :commit, :ref, :tag or
// :branch (hash-rocket or `git:` keyword style); :local modules are part of
// the repository.
//
// Implements: REQ-PUPPET-005
func readPuppetfile(source []byte) *puppetfile {
	parsed := &puppetfile{moduleDirectory: "modules"}
	for _, statement := range rubyStatements(source) {
		if len(statement) < 2 || statement[0].kind != rIdentifier || statement[1].kind != rString {
			continue
		}
		switch statement[0].text {
		case "moduledir":
			parsed.moduleDirectory = strings.Trim(statement[1].text, "/")
			continue
		case "mod":
		default:
			continue
		}
		name := statement[1].text
		d := &dependency{key: name, name: short(name), line: statement[1].line}
		options := map[string]string{}
		version, latest := "", false
		for i := 2; i < len(statement); i++ {
			switch t := statement[i]; {
			case t.kind == rString && i == 3:
				version = t.text
			case t.kind == rSymbol && i == 3 && t.text == "latest":
				latest = true
			case t.kind == rSymbol || t.kind == rLabel:
				// :key => value or key: value
				j := i + 1
				if j < len(statement) && statement[j].kind == rArrow {
					j++
				}
				if j < len(statement) && (statement[j].kind == rString || statement[j].kind == rSymbol || statement[j].kind == rIdentifier) {
					options[t.text] = statement[j].text
					i = j
				}
			}
		}
		switch {
		case options["local"] == "true":
			continue
		case options["git"] != "" || options["svn"] != "":
			url := options["git"] + options["svn"]
			d.packageName, d.origin = lang.RepositoryName(url), url
			switch {
			case options["commit"] != "":
				d.version, d.pinned = options["commit"], true
			case options["tag"] != "":
				d.version = options["tag"]
			case options["branch"] != "":
				d.version, d.floating = options["branch"], true
			case options["rev"] != "": // svn
				d.version, d.pinned = options["rev"], true
			default:
				gitReference(d, options["ref"])
			}
			if d.version == "control_branch" {
				d.pinned = false
				d.floating = true
			}
		default:
			d.packageName, d.forge = slug(name), true
			version = cmp.Or(version, options["version"])
			d.version = version
			d.pinned = !latest && lang.Pinned(version)
			d.floating = !d.pinned
		}
		parsed.dependencies = append(parsed.dependencies, d)
	}
	return parsed
}

type rKind uint8

const (
	rIdentifier rKind = iota
	rString
	rSymbol // :git
	rLabel  // git: (keyword argument)
	rArrow  // =>
	rPunctuation
)

type rToken struct {
	kind rKind
	text string
	line int
}

// rubyStatements splits the Ruby of a Puppetfile into statements: a line
// ending in a comma, an arrow or an open bracket continues on the next.
func rubyStatements(source []byte) [][]rToken {
	s := string(source)
	var out [][]rToken
	var current []rToken
	line, depth := 1, 0
	flush := func() {
		if len(current) > 0 {
			out = append(out, current)
			current = nil
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
			if depth <= 0 && (len(current) == 0 || current[len(current)-1].kind != rArrow && (current[len(current)-1].kind != rPunctuation || current[len(current)-1].text != ",")) {
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
			current = append(current, rToken{rString, b.String(), line})
			i = min(j+1, len(s))
		case c == ':' && isWord(at(s, i+1)):
			j := i + 1
			for j < len(s) && isWord(s[j]) {
				j++
			}
			current = append(current, rToken{rSymbol, s[i+1 : j], line})
			i = j
		case isWord(c):
			j := i
			for j < len(s) && (isWord(s[j]) || s[j] == '?' || s[j] == '!') {
				j++
			}
			if at(s, j) == ':' && at(s, j+1) != ':' {
				current = append(current, rToken{rLabel, s[i:j], line})
				i = j + 1
				continue
			}
			current = append(current, rToken{rIdentifier, s[i:j], line})
			i = j
		case c == '=' && at(s, i+1) == '>':
			current = append(current, rToken{rArrow, "=>", line})
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
			current = append(current, rToken{rPunctuation, string(c), line})
			i++
		}
	}
	flush()
	return out
}

// metadata is what a Puppet module's metadata.json says of it.
type metadata struct {
	name         string // author-name
	version      string
	dependencies []*dependency
	line         int
}

var moduleName = regexp.MustCompile(`^[A-Za-z0-9]+[-/][a-z][a-z0-9_]*$`)

// readMetadata reads a module's metadata.json, or returns nil for a
// metadata.json that is not a Puppet module's: it names the module
// author-name and says what it depends on, which platforms it supports or
// where its source is. A dependency's version_requirement is a range, which
// floats; an exact version pins.
//
// Implements: REQ-PUPPET-005
func readMetadata(source []byte) *metadata {
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
	if json.Unmarshal(source, &raw) != nil || !moduleName.MatchString(raw.Name) {
		return nil
	}
	if raw.Dependencies == nil && raw.Support == nil && raw.Source == "" && raw.Requirements == nil {
		return nil
	}
	m := &metadata{name: slug(raw.Name), version: raw.Version, line: max(lang.LineOf(source, `"`+raw.Name+`"`, 0), 1)}
	from := 0
	for _, d := range raw.Dependencies {
		if !moduleName.MatchString(d.Name) {
			continue
		}
		line := max(lang.LineOf(source, `"`+d.Name+`"`, from), 1)
		from = max(from, bytes.Index(source, []byte(`"`+d.Name+`"`)))
		v := strings.TrimSpace(d.Version)
		pinned := lang.Pinned(v)
		m.dependencies = append(m.dependencies, &dependency{key: slug(d.Name), name: short(d.Name), packageName: slug(d.Name), version: v, pinned: pinned, floating: !pinned, forge: true, line: line})
	}
	return m
}

// readFixtures reads puppetlabs_spec_helper's .fixtures.yml: the modules its
// forge_modules and repositories install for the module's tests (a string,
// or a map with repository, reference and branch). A Forge module pins by an exact reference;
// a repository as a Puppetfile's git :ref does, and floats on a branch.
//
// Implements: REQ-PUPPET-005
func readFixtures(source []byte) []*dependency {
	fixtures := yamlnode.Get(yamlnode.Parse(source), "fixtures")
	var out []*dependency
	for _, section := range []string{"forge_modules", "repositories"} {
		for _, entry := range yamlnode.Pairs(yamlnode.Get(fixtures, section)) {
			k, v := entry.Key, entry.Value
			repository, reference, branch := v.Value, "", ""
			if v.Kind == yaml.MappingNode {
				repository, reference, branch = yamlnode.Scalar(v, "repo"), yamlnode.Scalar(v, "ref"), yamlnode.Scalar(v, "branch")
			}
			if repository == "" {
				continue
			}
			d := &dependency{key: section + ":" + k.Value, name: strings.ToLower(k.Value), line: k.Line}
			if section == "forge_modules" {
				d.packageName, d.forge, d.version = slug(repository), true, reference
				d.pinned = lang.Pinned(reference)
				d.floating = !d.pinned
			} else {
				d.packageName, d.origin = lang.RepositoryName(repository), repository
				if branch != "" && reference == "" {
					d.version, d.floating = branch, true
				} else {
					gitReference(d, reference)
				}
			}
			out = append(out, d)
		}
	}
	return out
}
