package r

import (
	"cmp"
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// field is one field of a Debian control file (DCF) record - DESCRIPTION, packrat.lock
// - with the lines its value spans, so a dependency can point at its own line.
type field struct {
	name     string
	segments []segment
}

type segment struct {
	text string
	line int
}

func (f field) value() string {
	parts := make([]string, len(f.segments))
	for i, s := range f.segments {
		parts[i] = strings.TrimSpace(s.text)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// record is one DCF record, its fields by name.
type record map[string]field

func (r record) get(name string) string { return r[name].value() }

// parseDCF reads the records of a DCF file: "Name: value" lines, continued by
// indented lines, records separated by blank lines.
func parseDCF(source []byte) []record {
	var out []record
	current := record{}
	last := ""
	for i, line := range strings.Split(string(source), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.TrimSpace(line) == "":
			if len(current) > 0 {
				out = append(out, current)
				current, last = record{}, ""
			}
		case line[0] == ' ' || line[0] == '\t':
			if f, ok := current[last]; ok {
				f.segments = append(f.segments, segment{text: line, line: i + 1})
				current[last] = f
			}
		default:
			name, value, ok := strings.Cut(line, ":")
			if !ok || strings.ContainsAny(name, " \t") {
				continue
			}
			last = name
			current[name] = field{name: name, segments: []segment{{text: value, line: i + 1}}}
		}
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

// dependency is one entry of a DESCRIPTION dependency field: "dplyr (>= 1.1.0)".
type dependency struct {
	name, constraint, text string
	line                   int
}

var dependencyEntry = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9.]*)\s*(?:\(\s*([^)]*?)\s*\))?$`)

// dependencies splits a dependency field on its commas, each entry on the line it starts.
func dependencies(f field) []dependency {
	var out []dependency
	var current strings.Builder
	line := 0
	flush := func() {
		text := strings.Join(strings.Fields(current.String()), " ")
		current.Reset()
		if m := dependencyEntry.FindStringSubmatch(text); m != nil {
			out = append(out, dependency{name: m[1], constraint: normalizeConstraint(m[2]), text: text, line: line})
		}
		line = 0
	}
	for _, s := range f.segments {
		for i, part := range strings.Split(s.text, ",") {
			if i > 0 {
				flush()
			}
			if strings.TrimSpace(part) != "" && line == 0 {
				line = s.line
			}
			current.WriteString(" " + part)
		}
	}
	flush()
	return out
}

// normalizeConstraint writes a version requirement as ">= 1.2.0", and an exact one
// ("== 1.2.0") as the bare version, which is what lang.Pinned reads as pinned.
func normalizeConstraint(c string) string {
	c = strings.Join(strings.Fields(c), "")
	for _, operator := range []string{">=", "<=", "==", ">", "<"} {
		if v, ok := strings.CutPrefix(c, operator); ok {
			if operator == "==" {
				return v
			}
			return operator + " " + v
		}
	}
	return c
}

// dependencyFields are the DESCRIPTION fields that name packages, in the order R reads them.
var dependencyFields = []string{"Depends", "Imports", "LinkingTo", "Suggests", "Enhances"}

// description is what a DESCRIPTION file says about a package, or about a project
// that uses one only to declare dependencies (no Package field).
type description struct {
	name         string
	dependencies map[string]dependency
	remotes      map[string]remote
}

func readDescription(source []byte) *description {
	records := parseDCF(source)
	if len(records) == 0 {
		return nil
	}
	record := records[0]
	d := &description{name: record.get("Package"), dependencies: map[string]dependency{}, remotes: map[string]remote{}}
	for _, name := range dependencyFields {
		for _, dependency := range dependencies(record[name]) {
			if _, ok := d.dependencies[dependency.name]; !ok && dependency.name != "R" {
				d.dependencies[dependency.name] = dependency
			}
		}
	}
	for _, e := range strings.Split(record.get("Remotes"), ",") {
		if name, remote, ok := parseRemote(strings.TrimSpace(e)); ok {
			d.remotes[name] = remote
		}
	}
	return d
}

// remote is where a DESCRIPTION's Remotes field says a package comes from.
type remote struct {
	origin    string // a repository URL, or "path:<dir>"
	reference string
	bioc      bool
}

// parseRemote reads one Remotes entry as the remotes package does: [name=][type::]
// spec, where a GitHub spec is user/repo[/subdir][@ref|#pr].
func parseRemote(e string) (string, remote, bool) {
	if e == "" {
		return "", remote{}, false
	}
	name := ""
	if n, rest, ok := strings.Cut(e, "="); ok && !strings.Contains(n, "::") && !strings.Contains(n, "/") {
		name, e = strings.TrimSpace(n), strings.TrimSpace(rest)
	}
	typeName := "github"
	if t, rest, ok := strings.Cut(e, "::"); ok {
		typeName, e = strings.ToLower(t), rest
	}
	spec, reference, _ := strings.Cut(e, "@")
	spec, _, _ = strings.Cut(spec, "#")
	last := func(s string) string { return path.Base(strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git")) }
	var declaredRemote remote
	switch typeName {
	case "github", "gitlab", "bitbucket":
		host := map[string]string{"github": "github.com", "gitlab": "gitlab.com", "bitbucket": "bitbucket.org"}[typeName]
		segments := strings.Split(spec, "/")
		if len(segments) < 2 {
			return "", remote{}, false
		}
		declaredRemote = remote{origin: "https://" + host + "/" + segments[0] + "/" + segments[1], reference: reference}
	case "git", "url":
		declaredRemote = remote{origin: spec, reference: reference}
	case "local":
		declaredRemote = remote{origin: "path:" + spec}
	case "bioc":
		declaredRemote = remote{bioc: true}
		reference = ""
	default:
		return "", remote{}, false
	}
	if name == "" {
		name = last(spec)
		if typeName == "url" {
			name, _, _ = strings.Cut(name, "_") // pkg_1.0.tar.gz
		}
	}
	return name, declaredRemote, validPackage(name)
}

// extractDescription turns a DESCRIPTION's dependency fields into imports of what
// they name ("R (>= 4.1)" is the interpreter, not a package).
//
// Implements: REQ-R-005
func extractDescription(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	records := parseDCF(source)
	if len(records) == 0 {
		return extraction
	}
	for _, name := range dependencyFields {
		for _, d := range dependencies(records[0][name]) {
			if d.name != "R" {
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name + ": " + d.text, Module: d.name, Name: kindDependency + ":" + name, Line: d.line})
			}
		}
	}
	sort.SliceStable(extraction.Imports, func(i, j int) bool { return extraction.Imports[i].Line < extraction.Imports[j].Line })
	return extraction
}

// extractNamespace reads a NAMESPACE file's import(), importFrom(),
// importClassesFrom() and importMethodsFrom() directives, once per package.
// NAMESPACE is R syntax, so it goes through the lexer (directives may sit in if()).
//
// Implements: REQ-R-005
func extractNamespace(source []byte) *lang.Extraction {
	tokens, _ := lex(source, 1)
	c := newCode(tokens)
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	for i := range c.tokens {
		name, namespace, ok := c.callAt(i)
		if !ok || namespace != "" {
			continue
		}
		var packages []string
		arguments := c.arguments(i + 1)
		switch name {
		case "import":
			for _, a := range arguments {
				if v, one := c.single(a); one && a.name == "" && (v.k == tIdentifier || v.k == tString) {
					packages = append(packages, v.s)
				}
			}
		case "importFrom", "importClassesFrom", "importMethodsFrom":
			if a, ok := positional(arguments, 0); ok {
				if v, one := c.single(a); one && (v.k == tIdentifier || v.k == tString) {
					packages = append(packages, v.s)
				}
			}
		}
		for _, p := range packages {
			if key := name + " " + p; !seen[key] && validPackage(p) {
				seen[key] = true
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name + "(" + p + ")", Module: p, Name: kindNSFile, Line: c.tokens[i].line})
			}
		}
	}
	return extraction
}

// locked is one package of renv.lock or packrat.lock.
type locked struct {
	name, version string
	bioc          bool
	origin        string // repository URL of a GitHub, GitLab or git package; "path:" local
	sha           string // the commit a remote package was installed from
	reference     string // the branch or tag asked for, when no commit is recorded
	requires      []string
}

// lockfile is a project's lock: renv.lock, or packrat/packrat.lock.
type lockfile struct {
	directory string // the project directory
	packages  map[string]*locked
	// repositories are the repositories renv.lock names, by name: a package's Repository
	// field refers to one of them.
	repositories map[string]string
}

type renvPackage struct {
	Package, Version, Source, Repository   string
	RemoteType, RemoteHost, RemoteUsername string
	RemoteRepo, RemoteRef, RemoteSha       string
	RemoteUrl                              string
	Requirements                           []string
	Depends, Imports, LinkingTo            json.RawMessage
}

// readRenvLock reads renv.lock: every package with its version and source, and what
// it requires - Requirements in renv before 1.1, Depends/Imports/LinkingTo (lists
// of DESCRIPTION entries) after.
//
// Implements: REQ-R-008
func readRenvLock(source []byte, directory string) *lockfile {
	var doc struct {
		R struct {
			Repositories []struct{ Name, URL string }
		}
		Packages map[string]renvPackage
	}
	if json.Unmarshal(source, &doc) != nil {
		return nil
	}
	l := &lockfile{directory: directory, packages: map[string]*locked{}, repositories: map[string]string{}}
	for _, r := range doc.R.Repositories {
		l.repositories[r.Name] = r.URL
	}
	for key, p := range doc.Packages {
		name := cmp.Or(p.Package, key)
		lockedPackage := &locked{name: name, version: p.Version}
		source := strings.ToLower(p.Source)
		switch {
		case source == "bioconductor" || source == "repository" && strings.Contains(strings.ToLower(p.Repository), "bioc"):
			lockedPackage.bioc = true
		case source == "github" || source == "gitlab" || source == "bitbucket" || source == "git":
			host := strings.TrimPrefix(strings.TrimPrefix(p.RemoteHost, "api."), "https://")
			if host == "" || host == "github.com/api/v3" {
				host = map[string]string{"github": "github.com", "gitlab": "gitlab.com", "bitbucket": "bitbucket.org"}[source]
			}
			switch {
			case p.RemoteUrl != "":
				lockedPackage.origin = p.RemoteUrl
			case p.RemoteUsername != "" && p.RemoteRepo != "":
				lockedPackage.origin = "https://" + host + "/" + p.RemoteUsername + "/" + p.RemoteRepo
			}
			lockedPackage.sha, lockedPackage.reference = p.RemoteSha, p.RemoteRef
		case source == "local" || source == "url" || source == "cellar":
			lockedPackage.origin = "path:" + p.RemoteUrl
			if source == "url" {
				lockedPackage.origin = p.RemoteUrl
			}
		}
		requirements := p.Requirements
		for _, raw := range []json.RawMessage{p.Depends, p.Imports, p.LinkingTo} {
			requirements = append(requirements, descriptionList(raw)...)
		}
		for _, r := range requirements {
			if m := dependencyEntry.FindStringSubmatch(strings.TrimSpace(r)); m != nil && m[1] != "R" {
				lockedPackage.requires = append(lockedPackage.requires, m[1])
			}
		}
		sort.Strings(lockedPackage.requires)
		lockedPackage.requires = compact(lockedPackage.requires)
		l.packages[name] = lockedPackage
	}
	return l
}

// descriptionList reads a DESCRIPTION field as renv 1.1 writes it into renv.lock: a list
// of entries, or one comma-separated string.
func descriptionList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		out := strings.Split(s, ",")
		for i := range out {
			out[i] = strings.Join(strings.Fields(out[i]), " ")
		}
		return out
	}
	return nil
}

func compact(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// readPackratLock reads packrat/packrat.lock: DCF records after the header, with
// Source CRAN, Bioconductor, github (GithubUsername, GithubRepo, GithubSHA1) or
// source, and Requires.
//
// Implements: REQ-R-008
func readPackratLock(source []byte, directory string) *lockfile {
	l := &lockfile{directory: directory, packages: map[string]*locked{}, repositories: map[string]string{}}
	for _, record := range parseDCF(source) {
		name := record.get("Package")
		if name == "" {
			continue
		}
		lockedPackage := &locked{name: name, version: record.get("Version")}
		switch strings.ToLower(record.get("Source")) {
		case "bioconductor":
			lockedPackage.bioc = true
		case "github":
			lockedPackage.origin = "https://github.com/" + record.get("GithubUsername") + "/" + record.get("GithubRepo")
			lockedPackage.sha, lockedPackage.reference = record.get("GithubSHA1"), record.get("GithubRef")
		case "source":
			lockedPackage.origin = "path:" + name
		}
		for _, d := range dependencies(record["Requires"]) {
			lockedPackage.requires = append(lockedPackage.requires, d.name)
		}
		l.packages[name] = lockedPackage
	}
	return l
}
