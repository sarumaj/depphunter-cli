package php

import (
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	files    map[string]bool
	dirs     map[string]bool
	projects []*project // shallowest first
	local    []mapping  // every project's own autoload rules, longest prefix first
	// declared maps a lower-case fully qualified class, function or constant name to
	// the project file declaring it, and a namespace to the directory of its first
	// file: the classes of a classmap, of a project without composer.json, and those
	// PSR-4 cannot place.
	declared   map[string]string
	namespaces map[string]string
}

// Implements: REQ-PHP-005, REQ-PHP-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, declared: map[string]string{}, namespaces: map[string]string{}}
	sorted := append([]*scan.File(nil), all...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	for _, f := range sorted {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		switch {
		case path.Base(f.Path) == "composer.json":
			if p, local := readProject(root, f.Path, f.Abs); p != nil {
				r.projects = append(r.projects, p)
				r.local = append(r.local, local...)
			}
		case exts[strings.ToLower(path.Ext(f.Path))] && !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize:
			r.readDeclarations(f)
		}
	}
	sort.SliceStable(r.projects, func(i, j int) bool { return depth(r.projects[i].dir) < depth(r.projects[j].dir) })
	sort.SliceStable(r.local, func(i, j int) bool {
		a, b := r.local[i], r.local[j]
		if len(a.prefix) != len(b.prefix) {
			return len(a.prefix) > len(b.prefix)
		}
		return a.prefix+strings.Join(a.dirs, ",") < b.prefix+strings.Join(b.dirs, ",")
	})
	return r
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

var (
	nsDecl    = regexp.MustCompile(`^\s*namespace\s+([A-Za-z_\\][A-Za-z0-9_\\]*)\s*[;{]`)
	classDecl = regexp.MustCompile(`^\s*(?:#\[[^\]]*\]\s*)?(?:(?:abstract|final|readonly)\s+)*(?:class|interface|trait|enum)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	funcDecl  = regexp.MustCompile(`^function\s+&?\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	constDecl = regexp.MustCompile(`^const\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`)
)

// readDeclarations reads the namespace and the names a PHP file declares, from its
// text: the resolver is rebuilt on every run and must stay cheap. Classes may be
// indented (a braced namespace); a function or constant counts only at the first
// column, where a method never is.
//
// Implements: REQ-PHP-005
func (r *resolver) readDeclarations(f *scan.File) {
	src, err := os.ReadFile(f.Abs)
	if err != nil {
		return
	}
	ns, comment := "", false
	add := func(name string) {
		key := strings.ToLower(join(ns, name))
		if _, ok := r.declared[key]; !ok {
			r.declared[key] = f.Path
		}
	}
	for _, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if comment {
			comment = !strings.Contains(trimmed, "*/")
			continue
		}
		if strings.HasPrefix(trimmed, "/*") {
			comment = !strings.Contains(trimmed[2:], "*/")
			continue
		}
		if m := nsDecl.FindStringSubmatch(line); m != nil {
			ns = strings.Trim(m[1], `\`)
			if _, ok := r.namespaces[strings.ToLower(ns)]; !ok {
				r.namespaces[strings.ToLower(ns)] = path.Dir(f.Path)
			}
		} else if m := classDecl.FindStringSubmatch(line); m != nil {
			add(m[1])
		} else if m := funcDecl.FindStringSubmatch(line); m != nil {
			add(m[1])
		} else if m := constDecl.FindStringSubmatch(line); m != nil {
			add(m[1])
		}
	}
}

// Implements: REQ-PHP-004, REQ-PHP-005, REQ-PHP-006, REQ-PHP-007, REQ-PHP-009, REQ-PHP-010, REQ-PHP-012
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindInclude:
		return r.include(file, imp.Module)
	case kindLocal:
		t, _ := r.localFile(imp.Module, kindClass)
		return t
	}
	fqn, kind := strings.TrimPrefix(imp.Module, `\`), imp.Name
	if t, ok := r.localFile(fqn, kind); ok {
		return t
	}
	if ext, ok := builtin(fqn, kind); ok {
		return lang.Target{Ecosystem: ecoStd, Package: ext}
	}
	projects := r.projectsOf(file)
	lookup := fqn
	if kind != kindClass { // a function or constant is found by its namespace
		lookup = fqn[:max(0, strings.LastIndex(fqn, `\`))]
	}
	for _, p := range projects {
		for _, pp := range p.byPrefix {
			if matchPrefix(lookup+`\`, pp.prefix) || matchPrefix(lookup, pp.prefix) {
				return p.target(pp.pkg.name)
			}
		}
	}
	if !strings.Contains(fqn, `\`) {
		return lang.Target{} // a global name neither PHP nor the project defines
	}
	if t, ok := r.localNamespace(lookup); ok {
		return t
	}
	return r.guess(fqn, projects)
}

// projectsOf lists the composer.json projects a file belongs to, nearest first; a file
// under none takes every project, the shallowest first.
func (r *resolver) projectsOf(file string) []*project {
	var out []*project
	for i := len(r.projects) - 1; i >= 0; i-- {
		if p := r.projects[i]; p.dir == "." || strings.HasPrefix(file, p.dir+"/") {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return r.projects
	}
	return out
}

// localFile finds the project file declaring a name: by what the files declare, then
// by the autoload rules (PSR-4: the rest of the name under the prefix's directories;
// PSR-0: the whole name, `_` in the class name read as a directory).
//
// Implements: REQ-PHP-005
func (r *resolver) localFile(fqn, kind string) (lang.Target, bool) {
	if p, ok := r.declared[strings.ToLower(fqn)]; ok {
		return lang.Target{Local: p}, true
	}
	if kind != kindClass {
		return lang.Target{}, false
	}
	for _, m := range r.local {
		if m.prefix != "" && !matchPrefix(fqn, m.prefix) {
			continue
		}
		rel := fqn[len(m.prefix):]
		if m.psr0 {
			ns, class := "", fqn
			if i := strings.LastIndex(fqn, `\`); i >= 0 {
				ns, class = fqn[:i+1], fqn[i+1:]
			}
			rel = ns + strings.ReplaceAll(class, "_", "/")
		}
		rel = strings.ReplaceAll(strings.TrimPrefix(rel, `\`), `\`, "/") + ".php"
		for _, d := range m.dirs {
			if p := path.Join(d, rel); r.files[p] {
				return lang.Target{Local: p}, true
			}
		}
	}
	return lang.Target{}, false
}

// localNamespace places a name under one of the project's own namespaces that no file
// declares - `use App\Models;` names a namespace, not a class - at the namespace's
// directory, or else at the directory its autoload rule names. A catch-all rule (the
// empty prefix) claims nothing here.
//
// Implements: REQ-PHP-005
func (r *resolver) localNamespace(fqn string) (lang.Target, bool) {
	if dir, ok := r.namespaces[strings.ToLower(fqn)]; ok {
		return lang.Target{Local: dir}, true
	}
	for _, m := range r.local {
		if m.prefix == "" || !matchPrefix(fqn+`\`, m.prefix) && !matchPrefix(fqn, m.prefix) {
			continue
		}
		rel := strings.ReplaceAll(strings.Trim(fqn[min(len(fqn), len(m.prefix)):], `\`), `\`, "/")
		if m.psr0 {
			rel = strings.ReplaceAll(fqn, `\`, "/")
		}
		for _, d := range m.dirs {
			if p := path.Join(d, rel); r.dirs[p] {
				return lang.Target{Local: p}, true
			}
		}
		if d := m.dirs[0]; d == "." || r.dirs[d] {
			return lang.Target{Local: d}, true
		}
	}
	return lang.Target{}, false
}

var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// kebab turns a namespace segment into Composer's package-name style, splitting where
// a lower-case letter or digit meets an upper-case one: HttpFoundation ->
// http-foundation, while an acronym stays whole (PHPUnit -> phpunit, OAuth2 ->
// oauth2).
func kebab(s string) string {
	return strings.ToLower(camelBoundary.ReplaceAllString(s, "$1-$2"))
}

// fold is a name reduced to what namespace and package names share: lower case,
// letters and digits only (GuzzleHttp, guzzlehttp; HttpFoundation, http-foundation).
func fold(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// guess attributes a namespaced name no autoload rule claims. A declared package
// whose vendor and name are the namespace's first two segments (case and dashes
// aside), whose name is the first segment (PHPUnit is phpunit/phpunit), the only
// package of a vendor named like the first segment, or the vendor's package named by
// the following segments together (Psr\Http\Message is psr/http-message) takes it.
// Otherwise the name goes to the package its first two segments would name (the
// first lower-cased, the second hyphenated where its words meet) - Symfony's
// Component\X, Bundle\X and Bridge\X to symfony/x, symfony/x and symfony/x-bridge -
// marked unresolved.
//
// Implements: REQ-PHP-010
func (r *resolver) guess(fqn string, projects []*project) lang.Target {
	segments := strings.Split(fqn, `\`)
	s1, s2 := segments[0], ""
	if len(segments) > 1 {
		s2 = segments[1]
	}
	for _, p := range projects {
		var byVendor []string
		names := p.names()
		for _, name := range names {
			vendor, pkgName, _ := strings.Cut(name, "/")
			switch {
			case fold(vendor) == fold(s1) && fold(pkgName) == fold(s2):
				return p.target(name)
			case fold(vendor) == fold(s1):
				byVendor = append(byVendor, name)
			}
		}
		for _, name := range names {
			if _, pkgName, _ := strings.Cut(name, "/"); fold(pkgName) == fold(s1) {
				return p.target(name)
			}
		}
		if len(byVendor) == 1 {
			return p.target(byVendor[0])
		}
		// Several packages of the vendor: the one named by more segments
		// (Psr\Http\Message is psr/http-message, not psr/http-factory).
		for k := len(segments) - 1; k > 2; k-- {
			for _, name := range byVendor {
				if _, pkgName, _ := strings.Cut(name, "/"); fold(pkgName) == fold(strings.Join(segments[1:k], "")) {
					return p.target(name)
				}
			}
		}
	}
	name := strings.ToLower(s1) + "/" + kebab(s2) // vendors are rarely hyphenated: GuzzleHttp
	if strings.EqualFold(s1, "Symfony") && len(segments) > 2 {
		switch {
		case strings.EqualFold(s2, "Component"), strings.EqualFold(s2, "Bundle"):
			name = "symfony/" + kebab(segments[2])
		case strings.EqualFold(s2, "Bridge"):
			name = "symfony/" + kebab(segments[2]) + "-bridge"
		}
	}
	for _, p := range projects {
		if _, ok := p.require[name]; ok {
			return p.target(name)
		}
	}
	return lang.Target{Ecosystem: ecoComposer, Package: name, Unresolved: true}
}

// names lists a project's packages, required and locked, sorted.
func (p *project) names() []string {
	seen := map[string]bool{}
	var out []string
	for name := range p.require {
		seen[name] = true
		out = append(out, name)
	}
	for name := range p.locked {
		if !seen[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// include resolves a required or included path: "__DIR__/..." against the including
// file's directory; any other relative path as PHP's include path would find it -
// the including file's directory, the projects' roots, then the repository root. An
// absolute path, or one outside the repository, is dropped.
//
// Implements: REQ-PHP-004
func (r *resolver) include(file, p string) lang.Target {
	if rest, ok := strings.CutPrefix(p, "__DIR__"); ok {
		return r.localPath(path.Dir(file) + rest)
	}
	if path.IsAbs(p) || strings.Contains(p, "://") {
		return lang.Target{}
	}
	bases := []string{path.Dir(file)}
	for _, proj := range r.projectsOf(file) {
		bases = append(bases, proj.dir)
	}
	for _, base := range append(bases, ".") {
		if t := r.localPath(path.Join(base, p)); t.Local != "" {
			return t
		}
	}
	return lang.Target{}
}

func (r *resolver) localPath(p string) lang.Target {
	p = path.Clean(p)
	if p == ".." || strings.HasPrefix(p, "../") {
		return lang.Target{}
	}
	if r.files[p] || r.dirs[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// Dependencies implements lang.Transitive from composer.lock (or installed.json):
// every locked package lists what it requires, and the lock holds those too.
//
// Implements: REQ-PHP-011
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoComposer {
		return nil
	}
	p, k := r.locked(t)
	if k == nil {
		return nil
	}
	names := make([]string, 0, len(k.require))
	for name := range k.require {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]lang.Target, 0, len(names))
	for _, name := range names {
		d := lang.Target{Ecosystem: ecoComposer, Package: name, Version: k.require[name], Pinned: pinned(k.require[name])}
		if dep := p.locked[name]; dep != nil && dep.version != "" {
			d.Version, d.Pinned = dep.version, true
		}
		out = append(out, d)
	}
	return out
}

// Installed implements lang.Installed: the answer came from vendor/composer/installed.json.
func (r *resolver) Installed(t lang.Target) bool {
	_, k := r.locked(t)
	return k != nil && k.installed
}

// locked finds a package in the projects' locks, the one with the target's version
// first.
func (r *resolver) locked(t lang.Target) (*project, *pkg) {
	var fp *project
	var fk *pkg
	for _, p := range r.projects {
		if k := p.locked[strings.ToLower(t.Package)]; k != nil {
			if k.version == t.Version {
				return p, k
			}
			if fk == nil {
				fp, fk = p, k
			}
		}
	}
	return fp, fk
}
