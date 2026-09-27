package r

import (
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
	segments []seg
}

type seg struct {
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
func parseDCF(src []byte) []record {
	var out []record
	cur := record{}
	last := ""
	for i, line := range strings.Split(string(src), "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.TrimSpace(line) == "":
			if len(cur) > 0 {
				out = append(out, cur)
				cur, last = record{}, ""
			}
		case line[0] == ' ' || line[0] == '\t':
			if f, ok := cur[last]; ok {
				f.segments = append(f.segments, seg{text: line, line: i + 1})
				cur[last] = f
			}
		default:
			name, value, ok := strings.Cut(line, ":")
			if !ok || strings.ContainsAny(name, " \t") {
				continue
			}
			last = name
			cur[name] = field{name: name, segments: []seg{{text: value, line: i + 1}}}
		}
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// dependency is one entry of a DESCRIPTION dependency field: "dplyr (>= 1.1.0)".
type dependency struct {
	name, constraint, text string
	line                   int
}

var depEntry = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9.]*)\s*(?:\(\s*([^)]*?)\s*\))?$`)

// deps splits a dependency field on its commas, each entry on the line it starts.
func deps(f field) []dependency {
	var out []dependency
	var cur strings.Builder
	line := 0
	flush := func() {
		text := strings.Join(strings.Fields(cur.String()), " ")
		cur.Reset()
		if m := depEntry.FindStringSubmatch(text); m != nil {
			out = append(out, dependency{name: m[1], constraint: normConstraint(m[2]), text: text, line: line})
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
			cur.WriteString(" " + part)
		}
	}
	flush()
	return out
}

// normConstraint writes a version requirement as ">= 1.2.0", and an exact one
// ("== 1.2.0") as the bare version, which is what lang.Pinned reads as pinned.
func normConstraint(c string) string {
	c = strings.Join(strings.Fields(c), "")
	for _, op := range []string{">=", "<=", "==", ">", "<"} {
		if v, ok := strings.CutPrefix(c, op); ok {
			if op == "==" {
				return v
			}
			return op + " " + v
		}
	}
	return c
}

// depFields are the DESCRIPTION fields that name packages, in the order R reads them.
var depFields = []string{"Depends", "Imports", "LinkingTo", "Suggests", "Enhances"}

// description is what a DESCRIPTION file says about a package, or about a project
// that uses one only to declare dependencies (no Package field).
type description struct {
	name    string
	deps    map[string]dependency
	remotes map[string]remote
}

func readDescription(src []byte) *description {
	recs := parseDCF(src)
	if len(recs) == 0 {
		return nil
	}
	rec := recs[0]
	d := &description{name: rec.get("Package"), deps: map[string]dependency{}, remotes: map[string]remote{}}
	for _, name := range depFields {
		for _, dep := range deps(rec[name]) {
			if _, ok := d.deps[dep.name]; !ok && dep.name != "R" {
				d.deps[dep.name] = dep
			}
		}
	}
	for _, e := range strings.Split(rec.get("Remotes"), ",") {
		if name, rm, ok := parseRemote(strings.TrimSpace(e)); ok {
			d.remotes[name] = rm
		}
	}
	return d
}

// remote is where a DESCRIPTION's Remotes field says a package comes from.
type remote struct {
	origin string // a repository URL, or "path:<dir>"
	ref    string
	bioc   bool
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
	typ := "github"
	if t, rest, ok := strings.Cut(e, "::"); ok {
		typ, e = strings.ToLower(t), rest
	}
	spec, ref, _ := strings.Cut(e, "@")
	spec, _, _ = strings.Cut(spec, "#")
	last := func(s string) string { return path.Base(strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git")) }
	var rm remote
	switch typ {
	case "github", "gitlab", "bitbucket":
		host := map[string]string{"github": "github.com", "gitlab": "gitlab.com", "bitbucket": "bitbucket.org"}[typ]
		segments := strings.Split(spec, "/")
		if len(segments) < 2 {
			return "", remote{}, false
		}
		rm = remote{origin: "https://" + host + "/" + segments[0] + "/" + segments[1], ref: ref}
	case "git", "url":
		rm = remote{origin: spec, ref: ref}
	case "local":
		rm = remote{origin: "path:" + spec}
	case "bioc":
		rm = remote{bioc: true}
		ref = ""
	default:
		return "", remote{}, false
	}
	if name == "" {
		name = last(spec)
		if typ == "url" {
			name, _, _ = strings.Cut(name, "_") // pkg_1.0.tar.gz
		}
	}
	return name, rm, validPkg(name)
}

// extractDescription turns a DESCRIPTION's dependency fields into imports of what
// they name ("R (>= 4.1)" is the interpreter, not a package).
//
// Implements: REQ-R-005
func extractDescription(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	recs := parseDCF(src)
	if len(recs) == 0 {
		return ex
	}
	for _, name := range depFields {
		for _, d := range deps(recs[0][name]) {
			if d.name != "R" {
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: name + ": " + d.text, Module: d.name, Name: kindDep + ":" + name, Line: d.line})
			}
		}
	}
	sort.SliceStable(ex.Imports, func(i, j int) bool { return ex.Imports[i].Line < ex.Imports[j].Line })
	return ex
}

// extractNamespace reads a NAMESPACE file's import(), importFrom(),
// importClassesFrom() and importMethodsFrom() directives, once per package.
// NAMESPACE is R syntax, so it goes through the lexer (directives may sit in if()).
//
// Implements: REQ-R-005
func extractNamespace(src []byte) *lang.Extraction {
	tokens, _ := lex(src, 1)
	c := newCode(tokens)
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	for i := range c.tokens {
		name, ns, ok := c.callAt(i)
		if !ok || ns != "" {
			continue
		}
		var pkgs []string
		args := c.args(i + 1)
		switch name {
		case "import":
			for _, a := range args {
				if v, one := c.single(a); one && a.name == "" && (v.k == tIdent || v.k == tStr) {
					pkgs = append(pkgs, v.s)
				}
			}
		case "importFrom", "importClassesFrom", "importMethodsFrom":
			if a, ok := positional(args, 0); ok {
				if v, one := c.single(a); one && (v.k == tIdent || v.k == tStr) {
					pkgs = append(pkgs, v.s)
				}
			}
		}
		for _, p := range pkgs {
			if key := name + " " + p; !seen[key] && validPkg(p) {
				seen[key] = true
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: name + "(" + p + ")", Module: p, Name: kindNSFile, Line: c.tokens[i].line})
			}
		}
	}
	return ex
}

// locked is one package of renv.lock or packrat.lock.
type locked struct {
	name, version string
	bioc          bool
	origin        string // repository URL of a GitHub, GitLab or git package; "path:" local
	sha           string // the commit a remote package was installed from
	ref           string // the branch or tag asked for, when no commit is recorded
	requires      []string
}

// lockfile is a project's lock: renv.lock, or packrat/packrat.lock.
type lockfile struct {
	dir  string // the project directory
	pkgs map[string]*locked
	// repos are the repositories renv.lock names, by name: a package's Repository
	// field refers to one of them.
	repos map[string]string
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
func readRenvLock(src []byte, dir string) *lockfile {
	var doc struct {
		R struct {
			Repositories []struct{ Name, URL string }
		}
		Packages map[string]renvPackage
	}
	if json.Unmarshal(src, &doc) != nil {
		return nil
	}
	l := &lockfile{dir: dir, pkgs: map[string]*locked{}, repos: map[string]string{}}
	for _, r := range doc.R.Repositories {
		l.repos[r.Name] = r.URL
	}
	for key, p := range doc.Packages {
		name := p.Package
		if name == "" {
			name = key
		}
		lk := &locked{name: name, version: p.Version}
		src := strings.ToLower(p.Source)
		switch {
		case src == "bioconductor" || src == "repository" && strings.Contains(strings.ToLower(p.Repository), "bioc"):
			lk.bioc = true
		case src == "github" || src == "gitlab" || src == "bitbucket" || src == "git":
			host := strings.TrimPrefix(strings.TrimPrefix(p.RemoteHost, "api."), "https://")
			if host == "" || host == "github.com/api/v3" {
				host = map[string]string{"github": "github.com", "gitlab": "gitlab.com", "bitbucket": "bitbucket.org"}[src]
			}
			switch {
			case p.RemoteUrl != "":
				lk.origin = p.RemoteUrl
			case p.RemoteUsername != "" && p.RemoteRepo != "":
				lk.origin = "https://" + host + "/" + p.RemoteUsername + "/" + p.RemoteRepo
			}
			lk.sha, lk.ref = p.RemoteSha, p.RemoteRef
		case src == "local" || src == "url" || src == "cellar":
			lk.origin = "path:" + p.RemoteUrl
			if src == "url" {
				lk.origin = p.RemoteUrl
			}
		}
		req := p.Requirements
		for _, raw := range []json.RawMessage{p.Depends, p.Imports, p.LinkingTo} {
			req = append(req, descList(raw)...)
		}
		for _, r := range req {
			if m := depEntry.FindStringSubmatch(strings.TrimSpace(r)); m != nil && m[1] != "R" {
				lk.requires = append(lk.requires, m[1])
			}
		}
		sort.Strings(lk.requires)
		lk.requires = compact(lk.requires)
		l.pkgs[name] = lk
	}
	return l
}

// descList reads a DESCRIPTION field as renv 1.1 writes it into renv.lock: a list
// of entries, or one comma-separated string.
func descList(raw json.RawMessage) []string {
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
func readPackratLock(src []byte, dir string) *lockfile {
	l := &lockfile{dir: dir, pkgs: map[string]*locked{}, repos: map[string]string{}}
	for _, rec := range parseDCF(src) {
		name := rec.get("Package")
		if name == "" {
			continue
		}
		lk := &locked{name: name, version: rec.get("Version")}
		switch strings.ToLower(rec.get("Source")) {
		case "bioconductor":
			lk.bioc = true
		case "github":
			lk.origin = "https://github.com/" + rec.get("GithubUsername") + "/" + rec.get("GithubRepo")
			lk.sha, lk.ref = rec.get("GithubSHA1"), rec.get("GithubRef")
		case "source":
			lk.origin = "path:" + name
		}
		for _, d := range deps(rec["Requires"]) {
			lk.requires = append(lk.requires, d.name)
		}
		l.pkgs[name] = lk
	}
	return l
}
