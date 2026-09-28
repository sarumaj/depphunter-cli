package beam

import (
	"cmp"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a Mix or rebar3 project of the repository: a directory with a mix.exs
// (using Mix.Project) or a rebar.config, or both.
type project struct {
	dir    string
	app    string
	deps   map[string]*dependency // by application
	lock   map[string]*locked     // its own lock, or the nearest enclosing project's
	parent *project               // the enclosing project (an umbrella)
	known  map[string]string      // fold(app) -> app: what it declares and what its lock holds
	mix    bool                   // a mix.exs using Mix.Project: Mix, not rebar3, fetches
	// rebar is the rebarRegistry of its rebar.config, for a rebar3 project.
	rebar *string
	// registry is the lang.Target.Registry of the Hex packages it gets from no
	// repository it names: rebar3's repositories, from the outermost rebar3
	// project it is part of. Mix's are hex.pm's unless a package names another.
	registry string
}

type resolver struct {
	files     map[string]bool
	dirs      map[string]bool
	exMods    map[string]string // Elixir module -> defining file
	exRoots   map[string]bool   // first segments of the project's Elixir modules
	erlMods   map[string]string // Erlang module -> file
	appFiles  map[string]string // application -> its .app.src, mix.exs or rebar.config
	appDirs   map[string]string // application -> its directory
	projects  []*project        // deepest first
	locks     []map[string]*locked
	depMods   map[string]string // module -> application, from deps/ and _build/ on disk
	injected  map[string]*injection
	gleamMods map[string]string // Gleam module (gleam/list) -> its .gleam file
}

// Implements: REQ-BEAM-006, REQ-BEAM-008, REQ-BEAM-010, REQ-BEAM-012
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, exMods: map[string]string{},
		exRoots: map[string]bool{}, erlMods: map[string]string{}, appFiles: map[string]string{},
		appDirs: map[string]string{}, depMods: map[string]string{}, injected: map[string]*injection{},
		gleamMods: map[string]string{}}
	abs := map[string]string{}
	var files []*scan.File
	for _, f := range all {
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		if m, ok := gleamModule(f.Path); ok {
			if prev, dup := r.gleamMods[m]; !dup || f.Path < prev {
				r.gleamMods[m] = f.Path
			}
		}
		if (Plugin{}).Claims(f) && !f.TooLarge && f.Size <= lang.MaxParseSize {
			files = append(files, f)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	read := func(rel string) ([]byte, bool) {
		if a, ok := abs[rel]; ok {
			data, err := os.ReadFile(a)
			return data, err == nil
		}
		if root == "" || !inside(rel) {
			return nil, false
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return data, err == nil
	}
	byDir := map[string]*project{}
	proj := func(dir string) *project {
		if p := byDir[dir]; p != nil {
			return p
		}
		p := &project{dir: dir, deps: map[string]*dependency{}}
		byDir[dir] = p
		r.projects = append(r.projects, p)
		return p
	}
	for _, f := range files {
		src, ok := read(f.Path)
		if !ok {
			continue
		}
		base := path.Base(f.Path)
		dir := path.Dir(f.Path)
		switch ext := strings.ToLower(path.Ext(f.Path)); {
		case ext == ".ex" || ext == ".exs":
			er := newExReader(src)
			er.read()
			for m := range er.defined {
				if er.implementations[m] {
					continue // Proto.Type is named by no reference
				}
				if prev, dup := r.exMods[m]; !dup || testPath(prev) && !testPath(f.Path) {
					r.exMods[m] = f.Path
				}
				r.exRoots[strings.SplitN(m, ".", 2)[0]] = true
			}
			for m, in := range er.injected {
				r.injected[m] = in
			}
			if base == "mix.exs" && er.mixProject() {
				p := proj(dir)
				p.mix = true
				app, _ := mixProjectInfo(er.tokens)
				if app != "" {
					p.app = app
					r.appDirs[app], r.appFiles[app] = dir, f.Path
				}
				for _, d := range mixDeps(er.tokens) {
					p.deps[d.app] = d
				}
			}
		case ext == ".erl" || ext == ".xrl" || ext == ".yrl":
			// leex and yecc sources compile to the module named after them.
			m := strings.TrimSuffix(base, path.Ext(base))
			if _, dup := r.erlMods[m]; !dup {
				r.erlMods[m] = f.Path
			}
		case base == "rebar.config":
			p := proj(dir)
			forms := erlForms(src)
			registry := rebarRegistry(forms)
			p.rebar = &registry
			for _, d := range rebarDeps(forms) {
				if p.deps[d.app] == nil {
					p.deps[d.app] = d
				}
			}
		case strings.HasSuffix(base, ".app.src"):
			if name, _, _ := appSrc(src); name != "" {
				appDir := dir
				if path.Base(dir) == "src" {
					appDir = path.Dir(dir)
				}
				r.appDirs[name], r.appFiles[name] = appDir, f.Path
			}
		}
	}
	// Enclosing projects first, so a child finds its parent's lock.
	sort.Slice(r.projects, func(i, j int) bool { return depth(r.projects[i].dir) < depth(r.projects[j].dir) })
	for _, p := range r.projects {
		for d := p.dir; d != "."; {
			d = path.Dir(d)
			if q := byDir[d]; q != nil {
				p.parent = q
				break
			}
		}
		// mix.lock and rebar.lock are committed by applications and git-ignored by
		// libraries; what is on disk is what was resolved.
		lock := map[string]*locked{}
		if src, ok := read(path.Join(p.dir, "rebar.lock")); ok {
			for k, v := range readRebarLock(src) {
				lock[k] = v
			}
		}
		if src, ok := read(path.Join(p.dir, "mix.lock")); ok {
			for k, v := range readMixLock(src) {
				lock[k] = v
			}
		}
		if len(lock) > 0 {
			p.lock = lock
			r.locks = append(r.locks, lock)
		} else if p.parent != nil {
			p.lock = p.parent.lock
		}
		p.known = map[string]string{}
		for q := p; q != nil; q = q.parent {
			for app := range q.deps {
				p.known[fold(app)] = app
			}
		}
		for app := range p.lock {
			if _, ok := p.known[fold(app)]; !ok {
				p.known[fold(app)] = app
			}
		}
		// rebar3 takes its repositories from the project it runs in, the outermost
		// one; a rebar.config beside a Mix project's mix.exs is Mix's to read.
		for q := p; q != nil; q = q.parent {
			if q.rebar != nil && !q.mix {
				p.registry = *q.rebar
			}
		}
		if p.mix {
			p.registry = ""
		}
		if root != "" {
			r.readInstalled(root, p.dir)
		}
	}
	sort.SliceStable(r.projects, func(i, j int) bool { return depth(r.projects[i].dir) > depth(r.projects[j].dir) })
	return r
}

var defmoduleLine = regexp.MustCompile(`(?m)^\s*defmodule\s+([A-Z][A-Za-z0-9_.]*)`)

// readInstalled learns which package defines which module from what Mix and rebar3
// fetched and built beside a project: deps/<app> sources and _build/*/lib/<app>/ebin
// module files. Neither is in the repository, so this is read from disk, and only
// when present.
//
// Implements: REQ-BEAM-008
func (r *resolver) readInstalled(root, dir string) {
	base := filepath.Join(root, filepath.FromSlash(dir))
	budget := 50_000
	add := func(module, app string) {
		if _, ok := r.depMods[module]; !ok {
			r.depMods[module] = app
		}
	}
	apps, _ := os.ReadDir(filepath.Join(base, "deps"))
	for _, a := range apps {
		if !a.IsDir() {
			continue
		}
		app := a.Name()
		filepath.WalkDir(filepath.Join(base, "deps", app), func(p string, d os.DirEntry, err error) error {
			if err != nil || budget <= 0 {
				return filepath.SkipDir
			}
			if d.IsDir() {
				if n := d.Name(); n == "test" || n == "_build" || n == "deps" || n == ".git" || n == "priv" {
					return filepath.SkipDir
				}
				return nil
			}
			budget--
			switch filepath.Ext(p) {
			case ".erl":
				add(strings.TrimSuffix(d.Name(), ".erl"), app)
			case ".ex":
				if src, err := os.ReadFile(p); err == nil {
					for _, m := range defmoduleLine.FindAllSubmatch(src, -1) {
						add(string(m[1]), app)
					}
				}
			}
			return nil
		})
	}
	builds, _ := filepath.Glob(filepath.Join(base, "_build", "*", "lib", "*", "ebin"))
	for _, ebin := range builds {
		app := filepath.Base(filepath.Dir(ebin))
		entries, _ := os.ReadDir(ebin)
		for _, e := range entries {
			if budget--; budget <= 0 {
				return
			}
			if m, ok := strings.CutSuffix(e.Name(), ".beam"); ok {
				if rest, ok := strings.CutPrefix(m, "Elixir."); ok {
					m = rest
				}
				add(m, app)
			}
		}
	}
}

// testPath reports whether a file is test code or a test fixture.
func testPath(p string) bool {
	for _, seg := range strings.Split(path.Dir(p), "/") {
		switch seg {
		case "test", "tests", "fixtures":
			return true
		}
	}
	return false
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// inside reports whether a cleaned relative path stays in the repository.
func inside(p string) bool { return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p) }

func under(file, dir string) bool { return dir == "." || strings.HasPrefix(file, dir+"/") }

// projectOf is the project a file belongs to: the nearest one above it.
func (r *resolver) projectOf(file string) *project {
	for _, p := range r.projects {
		if under(file, p.dir) {
			return p
		}
	}
	return &project{deps: map[string]*dependency{}, known: map[string]string{}}
}

// declared finds a dependency in the project or an enclosing one.
func (p *project) declared(app string) *dependency {
	for q := p; q != nil; q = q.parent {
		if d := q.deps[app]; d != nil {
			return d
		}
	}
	return nil
}

func (p *project) knows(app string) bool { _, ok := p.known[fold(app)]; return ok }

// Implements: REQ-BEAM-006, REQ-BEAM-007, REQ-BEAM-008, REQ-BEAM-009
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, used, _ := strings.Cut(imp.Name, ":")
	switch kind {
	case kindElixir:
		if used == "" {
			return r.elixirModule(file, imp.Module)
		}
		uses := strings.Split(used, ",")
		if full, ok := r.injectedAlias(uses, imp.Module); ok {
			return r.elixirModule(file, full)
		}
		t := r.elixirModule(file, imp.Module)
		if t.Unresolved && r.foreignUse(file, uses) {
			return lang.Target{} // most likely aliased by a package's __using__
		}
		return t
	case kindErlang:
		return r.erlangModule(file, imp.Module)
	case kindInclude:
		if t := r.include(file, imp.Module); t.Local != "" {
			return t
		}
		return lang.Target{} // generated, or on an include path set elsewhere
	case kindIncludeLib:
		return r.includeLib(file, imp.Module)
	case kindDep:
		return r.dependency(file, imp.Module)
	case kindApp:
		return r.application(file, imp.Module)
	}
	return lang.Target{}
}

// elixirModule resolves a module reference: to the project file defining it; to a
// package whose name a curated prefix table knows; to Elixir's own modules; to the
// package that defines it in deps/ or _build/; to the project file of its longest
// defined prefix (MyAppWeb.Router.Helpers is generated in MyAppWeb.Router); to the
// declared or locked package its prefix names; else, outside the project's own
// namespaces, to an unresolved package named after its first segment.
//
// Implements: REQ-BEAM-006, REQ-BEAM-007, REQ-BEAM-008
func (r *resolver) elixirModule(file, mod string) lang.Target {
	segments := strings.Split(mod, ".")
	// A test's stand-in for one of Elixir's own modules does not replace it.
	if f, ok := r.exMods[mod]; ok && !(elixirStd[segments[0]] && testPath(f)) {
		return lang.Target{Local: f}
	}
	p := r.projectOf(file)
	alias := aliasFor(mod)
	if alias != "" && p.knows(alias) {
		return r.hexTarget(p, p.known[fold(alias)])
	}
	if elixirStd[segments[0]] {
		return lang.Target{Ecosystem: ecoElixir, Package: segments[0]}
	}
	for k := len(segments); k > 0; k-- {
		if app, ok := r.depMods[strings.Join(segments[:k], ".")]; ok {
			return r.hexTarget(p, app)
		}
	}
	for k := len(segments) - 1; k > 0; k-- {
		if f, ok := r.exMods[strings.Join(segments[:k], ".")]; ok {
			return lang.Target{Local: f}
		}
	}
	for k := len(segments); k > 0; k-- {
		if app, ok := p.known[fold(strings.Join(segments[:k], ""))]; ok {
			return r.hexTarget(p, app)
		}
		if k == 2 {
			// A plugin package named by its namespace and last segment:
			// Ueberauth.Strategy.Github in ueberauth_github.
			for j := len(segments) - 1; j >= 2; j-- {
				if app, ok := p.known[fold(segments[0]+segments[j])]; ok {
					return r.hexTarget(p, app)
				}
			}
		}
	}
	if r.exRoots[segments[0]] {
		return lang.Target{} // a module of the project's own namespace it does not define
	}
	name := alias
	if name == "" {
		name = underscore(segments[0])
	}
	return lang.Target{Ecosystem: ecoHex, Package: name, Unresolved: true}
}

// injectedAlias expands a module reference through the aliases that the modules a
// file uses inject with their quote blocks (use MyApp.Schema aliasing Ecto.Multi,
// use MyAppWeb, :controller aliasing the route helpers), following their own uses.
//
// Implements: REQ-BEAM-006
func (r *resolver) injectedAlias(used []string, mod string) (string, bool) {
	first, rest, _ := strings.Cut(mod, ".")
	seen := map[string]bool{}
	for len(used) > 0 {
		u := used[0]
		used = used[1:]
		if seen[u] {
			continue
		}
		seen[u] = true
		in := r.injected[u]
		if in == nil {
			continue
		}
		if full, ok := in.aliases[first]; ok {
			if rest != "" {
				full += "." + rest
			}
			return full, true
		}
		used = append(used, in.uses...)
	}
	return "", false
}

// foreignUse reports whether a file uses a module of a package, whose __using__
// may alias names the plugin cannot see.
func (r *resolver) foreignUse(file string, uses []string) bool {
	_ = file
	seen := map[string]bool{}
	for len(uses) > 0 {
		u := uses[0]
		uses = uses[1:]
		if seen[u] || elixirStd[strings.SplitN(u, ".", 2)[0]] {
			continue
		}
		seen[u] = true
		if _, local := r.exMods[u]; !local {
			return true
		}
		if in := r.injected[u]; in != nil {
			uses = append(uses, in.uses...) // what a local module's quote uses in turn
		}
	}
	return false
}

// aliasFor is the package the longest prefix of mod in moduleAliases names.
func aliasFor(mod string) string {
	for m := mod; m != ""; {
		if app, ok := moduleAliases[m]; ok {
			return app
		}
		i := strings.LastIndex(m, ".")
		if i < 0 {
			break
		}
		m = m[:i]
	}
	return ""
}

// erlangModule resolves an Erlang module: to the project's <module>.erl; to the
// package that ships it in deps/ or _build/; to a declared or locked package of
// that name; to OTP; to a known package its prefix names (cowboy_req -> cowboy); else
// to an unresolved package named after the module.
//
// Implements: REQ-BEAM-006, REQ-BEAM-007, REQ-BEAM-008
func (r *resolver) erlangModule(file, mod string) lang.Target {
	if f, ok := r.erlMods[mod]; ok {
		return lang.Target{Local: f}
	}
	if rest, ok := strings.CutPrefix(mod, "Elixir."); ok {
		return r.elixirModule(file, rest)
	}
	p := r.projectOf(file)
	if strings.Contains(mod, "@") {
		return r.gleamModule(p, strings.ReplaceAll(mod, "@", "/"))
	}
	if app, ok := r.depMods[mod]; ok {
		return r.hexTarget(p, app)
	}
	if app, ok := p.known[fold(mod)]; ok && strings.EqualFold(app, mod) {
		return r.hexTarget(p, app)
	}
	if otpModules[mod] {
		return lang.Target{Ecosystem: ecoOTP, Package: mod}
	}
	if mod == "elixir" || strings.HasPrefix(mod, "elixir_") {
		return lang.Target{Ecosystem: ecoElixir, Package: "elixir"} // the compiler's own modules
	}
	for i := len(mod) - 1; i > 0; i-- {
		if mod[i] == '_' {
			if app, ok := p.known[fold(mod[:i])]; ok && strings.EqualFold(app, mod[:i]) {
				return r.hexTarget(p, app)
			}
		}
	}
	alias := ""
	for prefix := range erlangAliases {
		if strings.HasPrefix(mod, prefix) && len(prefix) > 0 && (alias == "" || len(prefix) > len(alias)) {
			alias = prefix
		}
	}
	if alias != "" && p.knows(erlangAliases[alias]) {
		return r.hexTarget(p, erlangAliases[alias])
	}
	if otpModule(mod) {
		return lang.Target{Ecosystem: ecoOTP, Package: mod}
	}
	if f, ok := r.appFiles[mod]; ok {
		return lang.Target{Local: f} // an application module named after its application
	}
	name := mod
	if alias != "" {
		name = erlangAliases[alias]
	}
	return lang.Target{Ecosystem: ecoHex, Package: name, Unresolved: true}
}

// gleamModule resolves an Erlang reference to a compiled Gleam module
// (gleam@list is gleam/list): to the project's .gleam file, else to the Hex
// package the Gleam plugin names it by.
//
// Implements: REQ-GLEAM-009
func (r *resolver) gleamModule(p *project, mod string) lang.Target {
	if f, ok := r.gleamMods[mod]; ok {
		return lang.Target{Local: f}
	}
	name, known := GleamPackage(mod, p.knows)
	if known {
		return r.hexTarget(p, p.known[fold(name)])
	}
	return lang.Target{Ecosystem: ecoHex, Package: name, Unresolved: true}
}

// appDirOf is the application directory a file belongs to.
func (r *resolver) appDirOf(file string) string {
	best := ""
	for _, d := range r.appDirs {
		if under(file, d) && (best == "" || depth(d) > depth(best)) {
			best = d
		}
	}
	if best != "" {
		return best
	}
	dir := path.Dir(file)
	for d := dir; d != "."; d = path.Dir(d) {
		switch path.Base(d) {
		case "src", "include", "test":
			return path.Dir(d)
		}
	}
	return dir
}

// include finds an -include file: beside the including file, in its application's
// include/ or src/, or at its application's root, or in a local application the
// path starts with.
//
// Implements: REQ-BEAM-004
func (r *resolver) include(file, name string) lang.Target {
	dir := path.Dir(file)
	app := r.appDirOf(file)
	candidates := []string{path.Join(dir, name), path.Join(app, "include", name), path.Join(app, "src", name), path.Join(app, name)}
	if first, rest, ok := strings.Cut(name, "/"); ok {
		if d, ok := r.appDirs[first]; ok {
			candidates = append(candidates, path.Join(d, rest))
		}
	}
	for _, c := range candidates {
		if inside(c) && r.files[c] {
			return lang.Target{Local: c}
		}
	}
	return lang.Target{}
}

// includeLib resolves -include_lib("app/include/x.hrl"): to a file of a local
// application, to an OTP application, to a package, or else as a plain -include.
//
// Implements: REQ-BEAM-004, REQ-BEAM-007
func (r *resolver) includeLib(file, name string) lang.Target {
	app, rest, _ := strings.Cut(name, "/")
	if d, ok := r.appDirs[app]; ok {
		if f := path.Join(d, rest); r.files[f] {
			return lang.Target{Local: f}
		}
		if r.dirs[d] || d == "." {
			return lang.Target{Local: d}
		}
	}
	if otpApps[app] {
		return lang.Target{Ecosystem: ecoOTP, Package: app}
	}
	if t := r.include(file, name); t.Local != "" {
		return t
	}
	p := r.projectOf(file)
	if a, ok := p.known[fold(app)]; ok {
		return r.hexTarget(p, a)
	}
	return lang.Target{Ecosystem: ecoHex, Package: app, Unresolved: true}
}

// dependency resolves a manifest's dependency: a path or umbrella dependency to the
// manifest of the application in the repository, anything else to its package.
//
// Implements: REQ-BEAM-009
func (r *resolver) dependency(file, app string) lang.Target {
	p := r.projectOf(file)
	d := p.declared(app)
	if d != nil && d.umbrella {
		if f, ok := r.appFiles[app]; ok {
			return lang.Target{Local: f}
		}
	}
	if d != nil && d.path != "" {
		dir := path.Join(p.dir, d.path)
		if inside(dir) {
			for _, m := range []string{"mix.exs", "rebar.config"} {
				if f := path.Join(dir, m); r.files[f] {
					return lang.Target{Local: f}
				}
			}
			if r.dirs[dir] {
				return lang.Target{Local: dir}
			}
		}
	}
	return r.hexTarget(p, app)
}

// application resolves an application a .app.src lists: one of the repository, one
// of Elixir's or OTP's, or a package.
//
// Implements: REQ-BEAM-007, REQ-BEAM-009
func (r *resolver) application(file, app string) lang.Target {
	if f, ok := r.appFiles[app]; ok {
		return lang.Target{Local: f}
	}
	if m, ok := elixirApps[app]; ok {
		return lang.Target{Ecosystem: ecoElixir, Package: m}
	}
	if otpApps[app] {
		return lang.Target{Ecosystem: ecoOTP, Package: app}
	}
	return r.hexTarget(r.projectOf(file), app)
}

// hexTarget is a package as the project's lock pins it or its manifests declare it.
// The lock pins a Hex package at its version (the requirement kept as requested)
// and a git one at its commit; without it, "== 1.2.3" and a bare version pin, a git
// ref that is a commit pins, and requirements, branches and tags float. A package
// of a private organization (organization: or repo: in mix.exs, its repository in
// mix.lock) names that repository as its Registry; a rebar3 project's package the
// repositories rebar3 asks (rebarRegistry).
//
// Implements: REQ-BEAM-010, REQ-BEAM-011, REQ-BEAM-013
func (r *resolver) hexTarget(p *project, app string) lang.Target {
	d := p.declared(app)
	if l := p.lock[app]; l != nil {
		t := lockTarget(l)
		if t.Origin == "" && t.Registry == "" {
			t.Registry = p.registry
		}
		if d != nil && t.Origin == "" && d.req != "" && d.req != l.version {
			if v, _ := hexPinned(d.req); v != l.version {
				t.Requested = d.req
			}
		}
		return t
	}
	if d == nil {
		return lang.Target{Ecosystem: ecoHex, Package: app, Unresolved: true}
	}
	t := lang.Target{Ecosystem: ecoHex, Package: d.hexName()}
	switch {
	case d.git != "":
		t.Origin, t.Version = d.git, d.ref
		t.Pinned = lang.Commit(d.ref)
		t.Floating = !t.Pinned
	case d.path != "":
		t.Origin = "path:" + d.path
	case d.umbrella:
		t.Origin = "in_umbrella"
	default:
		t.Version, t.Pinned = hexPinned(d.req)
		t.Floating = d.req == ""
		t.Registry = cmp.Or(d.repo, p.registry)
	}
	return t
}

func lockTarget(l *locked) lang.Target {
	if l.git != "" {
		return lang.Target{Ecosystem: ecoHex, Package: l.app, Version: l.ref, Origin: l.git, Pinned: lang.Commit(l.ref)}
	}
	pkg := l.pkg
	if pkg == "" {
		pkg = l.app
	}
	return lang.Target{Ecosystem: ecoHex, Package: pkg, Version: l.version, Pinned: true, Registry: l.repo}
}

// Dependencies implements lang.Transitive from mix.lock, whose every Hex package
// lists what it depends on; rebar.lock records only a depth, not the edges.
//
// Implements: REQ-BEAM-010
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoHex {
		return nil
	}
	var lock map[string]*locked
	var entry *locked
	for _, l := range r.locks {
		for _, e := range l {
			if e.git == "" && (e.pkg == t.Package || e.pkg == "" && e.app == t.Package) {
				if entry == nil || e.version == t.Version {
					lock, entry = l, e
				}
			}
		}
		if entry != nil && entry.version == t.Version {
			break
		}
	}
	if entry == nil {
		return nil
	}
	var out []lang.Target
	for _, d := range entry.deps {
		if l := lock[d.app]; l != nil {
			dt := lockTarget(l)
			if dt.Origin == "" && d.req != "" && d.req != l.version {
				dt.Requested = d.req
			}
			out = append(out, dt)
			continue
		}
		if d.optional {
			continue // not chosen: an optional dependency the lock did not resolve
		}
		pkg := d.pkg
		if pkg == "" {
			pkg = d.app
		}
		v, pinned := hexPinned(d.req)
		out = append(out, lang.Target{Ecosystem: ecoHex, Package: pkg, Version: v, Pinned: pinned, Registry: d.repo})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Package < out[j].Package })
	return out
}
