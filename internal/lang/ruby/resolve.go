package ruby

import (
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	files    map[string]bool
	dirs     map[string]bool
	gemspecs map[string][]string // directory -> its *.gemspec files
	projects []*project          // shallowest first
	loadPath []string            // every project's gem lib directories
	rails    []*railsApp         // deepest first
	own      map[string]string   // gems the repository builds -> gemspec (or directory)
}

// railsApp is a Rails application: the directory holding config/application.rb, and
// what Zeitwerk would autoload in it - every directory under app/ (and app/*/concerns)
// is a root, lib/ too when the application says autoload_lib.
type railsApp struct {
	dir       string
	constants map[string]string // folded constant path ("admin::userscontroller") -> file or directory
	appDirs   []string          // app/models, app/controllers, ...: on the load path before Rails 7.1
}

// Implements: REQ-RUBY-005, REQ-RUBY-007, REQ-RUBY-010
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, gemspecs: map[string][]string{}, own: map[string]string{}}
	abs := map[string]string{}
	sorted := append([]*scan.File(nil), all...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	gemfiles := map[string]string{} // directory -> Gemfile name
	for _, f := range sorted {
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		switch base := path.Base(f.Path); {
		case base == "Gemfile" || base == "gems.rb":
			if _, ok := gemfiles[path.Dir(f.Path)]; !ok || base == "Gemfile" {
				gemfiles[path.Dir(f.Path)] = base
			}
		case strings.HasSuffix(base, ".gemspec"):
			r.gemspecs[path.Dir(f.Path)] = append(r.gemspecs[path.Dir(f.Path)], f.Path)
		}
	}
	read := func(rel string) (string, bool) {
		rel = path.Clean(rel)
		if a, ok := abs[rel]; ok {
			return readFile(a)
		}
		if root == "" || !inside(rel) {
			return "", false
		}
		return readFile(filepath.Join(root, filepath.FromSlash(rel))) // a lock the scan left out
	}
	dirs := map[string]bool{}
	for d := range gemfiles {
		dirs[d] = true
	}
	for d := range r.gemspecs {
		dirs[d] = true
	}
	// A gemspec a Gemfile takes in belongs to that Gemfile's project, not one of its own.
	claimed := map[string]bool{}
	for d, name := range gemfiles {
		src, _ := read(path.Join(d, name))
		_, specDirs := readGemfile(src, nil)
		for _, sd := range specDirs {
			if sd = path.Join(d, sd); sd != d {
				claimed[sd] = true
			}
		}
	}
	for _, d := range sortedKeys(dirs) {
		if gemfiles[d] == "" && claimed[d] {
			continue
		}
		p := readProject(d, gemfiles[d], r.gemspecs, read)
		r.projects = append(r.projects, p)
		r.loadPath = append(r.loadPath, p.loadPath...)
		for name, at := range p.own {
			if old, ok := r.own[name]; !ok || strings.HasSuffix(at, ".gemspec") && !strings.HasSuffix(old, ".gemspec") {
				r.own[name] = at
			}
		}
	}
	sort.SliceStable(r.projects, func(i, j int) bool { return depth(r.projects[i].dir) < depth(r.projects[j].dir) })
	for _, f := range sorted {
		if strings.HasSuffix(f.Path, "config/application.rb") {
			app := path.Dir(path.Dir(f.Path))
			src, _ := read(f.Path)
			r.rails = append(r.rails, r.zeitwerk(app, src))
		}
	}
	sort.SliceStable(r.rails, func(i, j int) bool { return depth(r.rails[i].dir) > depth(r.rails[j].dir) })
	return r
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

var autoloadLib = regexp.MustCompile(`autoload_lib\b|autoload_paths\b.*\blib\b`)

// zeitwerk indexes a Rails application's autoloaded constants the way Zeitwerk names
// them: a file's path under its root, camelized (admin/users_controller.rb is
// Admin::UsersController), and a directory without a file of its own as a namespace.
// Names are compared folded - lower case, underscores dropped - so an acronym
// inflection (API, not Api) needs no configuration.
//
// Implements: REQ-RUBY-010
func (r *resolver) zeitwerk(app, config string) *railsApp {
	a := &railsApp{dir: app, constants: map[string]string{}}
	prefix := path.Join(app, "app") + "/"
	if app == "." {
		prefix = "app/"
	}
	var roots []string
	for d := range r.dirs {
		rest, ok := strings.CutPrefix(d, prefix)
		if !ok {
			continue
		}
		if parts := strings.Split(rest, "/"); len(parts) == 1 || len(parts) == 2 && parts[1] == "concerns" {
			roots = append(roots, d)
			if len(parts) == 1 {
				a.appDirs = append(a.appDirs, d)
			}
		}
	}
	if autoloadLib.MatchString(config) {
		roots = append(roots, path.Join(app, "lib"))
	}
	sort.Strings(roots)
	sort.Strings(a.appDirs)
	var namespaces []string
	for f := range r.files {
		if !strings.HasSuffix(f, ".rb") {
			continue
		}
		for _, root := range roots {
			rel, ok := strings.CutPrefix(f, root+"/")
			if !ok || strings.HasPrefix(rel, "tasks/") || strings.HasPrefix(rel, "assets/") {
				continue
			}
			key := constKey(strings.TrimSuffix(rel, ".rb"))
			if old, ok := a.constants[key]; !ok || f < old {
				a.constants[key] = f
			}
			for d := path.Dir(rel); d != "."; d = path.Dir(d) {
				namespaces = append(namespaces, root+"/"+d)
			}
		}
	}
	sort.Strings(namespaces)
	for _, d := range namespaces {
		for _, root := range roots {
			if rel, ok := strings.CutPrefix(d, root+"/"); ok {
				if key := constKey(rel); a.constants[key] == "" {
					a.constants[key] = d
				}
			}
		}
	}
	return a
}

// constKey folds a file path under an autoload root, or a constant path, to what both
// have in common: admin/users_controller and Admin::UsersController are
// admin::userscontroller.
func constKey(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "/", "::")
	return strings.ReplaceAll(s, "_", "")
}

// Implements: REQ-RUBY-002, REQ-RUBY-004, REQ-RUBY-005, REQ-RUBY-006, REQ-RUBY-007, REQ-RUBY-009, REQ-RUBY-010, REQ-RUBY-012
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch kind, nest, _ := strings.Cut(imp.Name, ":"); kind {
	case kindConst:
		return r.constant(file, imp.Module, nest)
	case kindRelative:
		return r.rubyFile(r.relative(file, imp.Module))
	case kindRequire:
		return r.require(file, imp.Module)
	case kindLoad:
		return r.load(file, imp.Module)
	case kindGem:
		return r.gem(file, imp.Module, r.projectsOf(file))
	case kindGemPath, kindGemspec:
		dir := r.relative(file, imp.Module)
		if specs := r.gemspecs[dir]; len(specs) > 0 {
			return lang.Target{Local: specs[0]}
		}
		if imp.Name == kindGemPath {
			return r.localPath(dir)
		}
	}
	return lang.Target{}
}

// relative turns "__DIR__/..." into a path relative to the repository root.
func (r *resolver) relative(file, p string) string {
	if rest, ok := strings.CutPrefix(p, "__DIR__"); ok {
		return path.Clean(path.Dir(file) + rest)
	}
	return path.Clean(p)
}

// rubyFile finds the project file a path without its extension names.
func (r *resolver) rubyFile(p string) lang.Target {
	if !inside(p) {
		return lang.Target{}
	}
	for _, candidate := range []string{p + ".rb", p} {
		if r.files[candidate] {
			return lang.Target{Local: candidate}
		}
	}
	return lang.Target{}
}

func (r *resolver) localPath(p string) lang.Target {
	if inside(p) && (r.files[p] || r.dirs[p]) {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// require resolves a require path as Ruby would under Bundler: a file on the load
// path, then Ruby's own library (unless the project declares it as a gem), then a
// gem. The load path is guessed: the lib, test and spec directories of the file's
// directory and every one above it (Rake and RSpec add test/ and spec/), the lib
// directories of the gems whose code is in the repository (gemspecs, path gems),
// then a Rails application's app/ directories.
//
// Implements: REQ-RUBY-005
func (r *resolver) require(file, p string) lang.Target {
	if strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") {
		if t := r.rubyFile(path.Join(path.Dir(file), p)); t.Local != "" {
			return t
		}
		return r.rubyFile(path.Clean(p))
	}
	if path.IsAbs(p) {
		return lang.Target{}
	}
	p = strings.TrimSuffix(p, ".rb")
	for _, dir := range r.loadPathOf(file) {
		if t := r.rubyFile(path.Join(dir, p)); t.Local != "" && t.Local != file {
			return t
		}
	}
	projects := r.projectsOf(file)
	if name, ok := r.gemFor(p, projects, true); ok {
		return r.gem(file, name, projects)
	}
	if pkg, ok := stdLibrary(p); ok {
		for _, proj := range projects {
			if proj.has(pkg) { // yaml is psych, and the lock has psych
				return r.gem(file, pkg, projects)
			}
		}
		return lang.Target{Ecosystem: ecoStd, Package: pkg}
	}
	name, _ := r.gemFor(p, projects, false)
	return r.gem(file, name, projects)
}

// gem resolves a gem name: a gem the repository builds itself is its gemspec;
// another goes through the file's projects, nearest first: the nearest that locks
// it, else the nearest that declares it (a monorepo's gemspecs declare, its root
// Gemfile.lock pins).
func (r *resolver) gem(file, name string, projects []*project) lang.Target {
	if at, ok := r.own[strings.ToLower(name)]; ok && at != file {
		return lang.Target{Local: at}
	}
	for _, p := range projects {
		if p.locked[strings.ToLower(name)] != nil {
			return p.target(name)
		}
	}
	for _, p := range projects {
		if p.has(name) {
			return p.target(name)
		}
	}
	return lang.Target{Ecosystem: ecoGems, Package: name, Unresolved: true}
}

// loadPathOf lists the directories a require from file is looked up in.
func (r *resolver) loadPathOf(file string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(d string) {
		if !seen[d] && (r.dirs[d] || d == ".") {
			seen[d] = true
			out = append(out, d)
		}
	}
	for d := path.Dir(file); ; d = path.Dir(d) {
		for _, sub := range []string{"lib", "test", "spec"} {
			add(path.Join(d, sub))
		}
		if d == "." {
			break
		}
	}
	for _, d := range r.loadPath {
		add(d)
	}
	for _, app := range r.rails {
		if app.dir == "." || strings.HasPrefix(file, app.dir+"/") {
			for _, d := range app.appDirs {
				add(d)
			}
		}
	}
	return out
}

// gemFor names the gem a require path belongs to. With declared set it only answers
// with a gem the projects declare or lock: an alias (active_support is
// activesupport), then the path's leading segments joined with dashes or run
// together, longest first (rspec/core is rspec-core, net/http/persistent is
// net-http-persistent), compared with case, dashes and underscores ignored; a
// first segment also names the gem it is suffixed or prefixed with ruby (yajl is
// yajl-ruby).
// Otherwise it guesses: the alias, or the path's first segment (the first two
// joined with a dash under a shared namespace such as net/).
//
// Implements: REQ-RUBY-006
func (r *resolver) gemFor(p string, projects []*project, declared bool) (string, bool) {
	segments := strings.Split(p, "/")
	for k := len(segments); k >= 1; k-- {
		names, ok := aliases[strings.Join(segments[:k], "/")]
		if !ok {
			continue
		}
		if !declared {
			for _, name := range names {
				for _, proj := range projects {
					if proj.has(name) {
						return name, true
					}
				}
			}
			return names[0], true
		}
		for _, name := range names {
			for _, proj := range projects {
				if proj.has(name) {
					return name, true
				}
			}
		}
	}
	if !declared {
		if namespaces[segments[0]] && len(segments) > 1 {
			return segments[0] + "-" + segments[1], true
		}
		return segments[0], true
	}
	for k := len(segments); k >= 1; k-- {
		want := fold(strings.Join(segments[:k], ""))
		for _, proj := range projects {
			for _, name := range proj.gemNames() {
				if f := fold(name); f == want || k == 1 && (f == want+"ruby" || f == "ruby"+want) {
					return name, true
				}
			}
		}
	}
	return "", false
}

// namespaces are first segments many gems share: net/ssh is net-ssh, dry/types is
// dry-types.
var namespaces = map[string]bool{"net": true, "dry": true}

// fold is a name reduced to what require paths and gem names share: lower case,
// letters and digits only.
func fold(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// load resolves a file Kernel#load reads: relative to the file, a project's
// directory, the load path or the repository root. load never reaches a gem.
//
// Implements: REQ-RUBY-005
func (r *resolver) load(file, p string) lang.Target {
	if strings.HasPrefix(p, "__DIR__") {
		return r.rubyFile(r.relative(file, p))
	}
	if path.IsAbs(p) {
		return lang.Target{}
	}
	bases := []string{path.Dir(file)}
	for _, proj := range r.projectsOf(file) {
		bases = append(bases, proj.dir)
	}
	bases = append(append(bases, r.loadPathOf(file)...), ".")
	for _, b := range bases {
		if t := r.rubyFile(path.Join(b, p)); t.Local != "" {
			return t
		}
	}
	return lang.Target{}
}

// projectsOf lists the Bundler projects a file belongs to, nearest first; a file under
// none takes every project, the shallowest first.
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

// constant resolves a constant in a Rails application to the file Zeitwerk loads it
// from, trying the modules around the reference innermost first, as Ruby looks it up.
// A constant nothing autoloads - Ruby's, a gem's, one the file defines itself - is
// dropped.
//
// Implements: REQ-RUBY-010
func (r *resolver) constant(file, name, nest string) lang.Target {
	var app *railsApp
	for _, a := range r.rails {
		if a.dir == "." || strings.HasPrefix(file, a.dir+"/") {
			app = a
			break
		}
	}
	if app == nil {
		return lang.Target{}
	}
	var candidates []string
	if abs, ok := strings.CutPrefix(name, "::"); ok {
		candidates = []string{abs}
	} else {
		outer := strings.Split(nest, "::")
		if nest == "" {
			outer = nil
		}
		for k := len(outer); k >= 0; k-- {
			candidates = append(candidates, strings.Join(append(append([]string{}, outer[:k]...), name), "::"))
		}
	}
	for _, c := range candidates {
		if f := app.constants[constKey(strings.ReplaceAll(c, " ", ""))]; f != "" {
			return lang.Target{Local: f}
		}
	}
	return lang.Target{}
}

// Dependencies implements lang.Transitive from Gemfile.lock: every locked gem lists
// what it depends on, and the lock holds those too.
//
// Implements: REQ-RUBY-011
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoGems {
		return nil
	}
	var p *project
	var s *spec
	for _, proj := range r.projects {
		if k := proj.locked[strings.ToLower(t.Package)]; k != nil {
			if p == nil || k.version == t.Version {
				p, s = proj, k
			}
			if k.version == t.Version {
				break
			}
		}
	}
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.deps))
	for name := range s.deps {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]lang.Target, 0, len(names))
	for _, name := range names {
		if dep := p.locked[strings.ToLower(name)]; dep != nil {
			d := p.target(name)
			d.Requested = s.deps[name]
			if d.Requested == "" || d.Requested == d.Version || d.Requested == "= "+d.Version {
				d.Requested = ""
			}
			out = append(out, d)
			continue
		}
		v, pin := exactVersion(s.deps[name])
		out = append(out, lang.Target{Ecosystem: ecoGems, Package: name, Version: v, Pinned: pin})
	}
	return out
}
