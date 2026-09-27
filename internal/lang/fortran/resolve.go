package fortran

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a directory with an fpm.toml: its manifest, what fpm recorded of
// the dependencies it fetched (build/cache.toml) and the modules each fetched
// dependency under build/dependencies/<name>/ defines.
type project struct {
	dir       string
	m         *manifest
	cache     map[string]*cached
	installed map[string]*installed // by dependency name
	modules   map[string]string     // module -> installed dependency name
}

// cached is an entry of build/cache.toml: the revision or version fpm fetched.
type cached struct {
	version, rev, git string
}

// installed is a dependency fpm fetched: its own manifest.
type installed struct {
	m *manifest
}

type resolver struct {
	files    map[string]bool
	dirs     map[string]bool
	modules  map[string][]string // module (lower case) -> defining files, sorted
	submods  map[string][]string // "ancestor:submodule" -> defining files
	projects map[string]*project
	order    []*project // shallowest first
	pkgs     cpp.Packages
}

// Implements: REQ-FORTRAN-004, REQ-FORTRAN-005, REQ-FORTRAN-006, REQ-FORTRAN-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{
		files: map[string]bool{}, dirs: map[string]bool{}, modules: map[string][]string{},
		submods: map[string][]string{}, projects: map[string]*project{}, pkgs: cpp.ReadPackages(all),
	}
	var sources []*scan.File
	for _, f := range all {
		if generated(f) {
			continue
		}
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		switch {
		case path.Base(f.Path) == "fpm.toml":
			if src, err := os.ReadFile(f.Abs); err == nil {
				p := &project{dir: path.Dir(f.Path), m: readManifest(src)}
				p.readBuild(filepath.Dir(f.Abs))
				r.projects[p.dir] = p
				r.order = append(r.order, p)
			}
		case source(f.Path) && (f.Lang == "" || f.Lang == "Fortran") && !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize:
			sources = append(sources, f)
		}
	}
	sort.Slice(r.order, func(i, j int) bool {
		di, dj := depth(r.order[i].dir), depth(r.order[j].dir)
		if di != dj {
			return di < dj
		}
		return r.order[i].dir < r.order[j].dir
	})
	for _, d := range declarations(sources) {
		for _, m := range d.modules {
			r.modules[m] = append(r.modules[m], d.file)
		}
		for _, s := range d.submods {
			r.submods[s] = append(r.submods[s], d.file)
		}
	}
	for _, list := range r.modules {
		sort.Strings(list)
	}
	for _, list := range r.submods {
		sort.Strings(list)
	}
	return r
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// declared is what one source file defines: modules and submodules, the latter
// as "ancestor:name".
type declared struct {
	file             string
	modules, submods []string
}

// declarations reads the module and submodule statements of every source, on
// all cores.
func declarations(files []*scan.File) []declared {
	out := make([]declared, len(files))
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f := files[i]
				out[i].file = f.Path
				if src, err := os.ReadFile(f.Abs); err == nil {
					out[i].modules, out[i].submods = definedModules(src, fixedExts[path.Ext(f.Path)])
				}
			}
		}()
	}
	for i := range files {
		next <- i
	}
	close(next)
	wg.Wait()
	return out
}

// definedModules is what a source defines: module names in lower case,
// submodules as "ancestor:name". Module and submodule statements are read from
// the lines that start with them (they are not continued in practice), which is
// far cheaper than reading every statement.
func definedModules(src []byte, fixed bool) (modules, submods []string) {
	if !mentionsModule(src) {
		return nil, nil
	}
	for _, line := range splitLines(src) {
		if fixed && line != "" && strings.IndexByte("Cc*!Dd", line[0]) >= 0 {
			continue
		}
		t := strings.TrimLeft(line, " \t")
		if len(t) < 7 || t[0]|0x20 != 'm' && t[0]|0x20 != 's' {
			continue
		}
		if i := strings.IndexByte(t, '!'); i >= 0 {
			t = t[:i]
		}
		ts := tokenize(t, 12)
		switch word(ts, 0) {
		case "module":
			if n := word(ts, 1); n != "" && eos(ts, 2) && n != "procedure" && validName(ts[1].text) {
				modules = append(modules, n)
			}
		case "submodule":
			anc, j := word(ts, 2), 3
			if !punct(ts, 1, "(") || anc == "" {
				continue
			}
			if punct(ts, j, ":") {
				j += 2
			}
			if punct(ts, j, ")") && word(ts, j+1) != "" && eos(ts, j+2) {
				submods = append(submods, anc+":"+word(ts, j+1))
			}
		}
	}
	return modules, submods
}

// mentionsModule reports whether src has the word "module" in any case, which
// every module and submodule statement has: most sources of a legacy library
// define none, and are then not read again.
func mentionsModule(src []byte) bool {
	for i := 0; i+6 <= len(src); i++ {
		if src[i]|0x20 == 'm' && src[i+1]|0x20 == 'o' && src[i+2]|0x20 == 'd' && src[i+3]|0x20 == 'u' && src[i+4]|0x20 == 'l' && src[i+5]|0x20 == 'e' {
			return true
		}
	}
	return false
}

// readBuild reads what fpm wrote into build/ beside the manifest: cache.toml and
// the dependencies it fetched into build/dependencies/<name>/, with the modules
// their sources define.
//
// Implements: REQ-FORTRAN-008
func (p *project) readBuild(absDir string) {
	p.cache, p.installed, p.modules = map[string]*cached{}, map[string]*installed{}, map[string]string{}
	build := filepath.Join(absDir, "build")
	if src, err := os.ReadFile(filepath.Join(build, "cache.toml")); err == nil {
		p.cache = readCache(src)
	}
	deps := filepath.Join(build, "dependencies")
	entries, err := os.ReadDir(deps)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(deps, e.Name())
		in := &installed{m: &manifest{deps: map[string]*dependency{}}}
		if src, err := os.ReadFile(filepath.Join(dir, "fpm.toml")); err == nil {
			in.m = readManifest(src)
		}
		p.installed[e.Name()] = in
		n := 0
		filepath.WalkDir(dir, func(q string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if q != dir && (d.Name() == "build" || strings.HasPrefix(d.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !source(q) || n >= 5000 {
				return nil
			}
			n++
			if st, err := d.Info(); err != nil || st.Size() > lang.MaxParseSize {
				return nil
			}
			if src, err := os.ReadFile(q); err == nil {
				mods, _ := definedModules(src, fixedExts[path.Ext(q)])
				for _, m := range mods {
					if _, ok := p.modules[m]; !ok {
						p.modules[m] = e.Name()
					}
				}
			}
			return nil
		})
	}
}

// readCache reads build/cache.toml: a table per dependency fpm resolved, with
// the version, the git repository and the revision it checked out.
func readCache(src []byte) map[string]*cached {
	out := map[string]*cached{}
	var raw map[string]any
	if _, err := toml.Decode(string(src), &raw); err != nil {
		return out
	}
	if deps, ok := raw["dependencies"].(map[string]any); ok {
		raw = deps
	}
	for name, v := range raw {
		t, ok := v.(map[string]any)
		if !ok {
			continue
		}
		str := func(k string) string {
			s, _ := t[k].(string)
			return strings.TrimSpace(s)
		}
		c := &cached{version: str("version"), rev: str("rev"), git: str("git")}
		if c.rev == "" {
			c.rev = str("revision")
		}
		out[name] = c
	}
	return out
}

// projectOf is the nearest project at or above dir.
func (r *resolver) projectOf(dir string) *project {
	for d := dir; ; d = path.Dir(d) {
		if p, ok := r.projects[d]; ok {
			return p
		}
		if d == "." || d == "/" || d == "" {
			return nil
		}
	}
}

// scope is the projects whose manifests speak for file: the nearest fpm.toml
// above it, or, for a file no fpm.toml governs, every one, shallowest first.
func (r *resolver) scope(file string) []*project {
	if p := r.projectOf(path.Dir(file)); p != nil {
		return []*project{p}
	}
	return r.order
}

// Implements: REQ-FORTRAN-004, REQ-FORTRAN-005, REQ-FORTRAN-006, REQ-FORTRAN-007
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindUse, kindIntrinsic, kindNonIntrinsic:
		return r.use(file, imp.Module, imp.Name)
	case kindSubmodule:
		anc, parent, ok := strings.Cut(imp.Module, ":")
		if ok {
			if t := r.local(file, r.submods[anc+":"+parent]); t.Local != "" {
				return t
			}
		}
		return r.use(file, anc, kindUse)
	case kindInclude, kindCpp, kindCppSys, kindFypp:
		return r.include(file, imp.Module, imp.Name)
	case kindDep, kindDevDep:
		if p := r.projects[path.Dir(file)]; p != nil {
			return r.dependency(p, imp.Module)
		}
	case kindDir, kindMain:
		if t := path.Join(path.Dir(file), imp.Module); r.files[t] || r.dirs[t] {
			return lang.Target{Local: t}
		}
	case kindExternal:
		return r.external(file, imp.Module)
	case kindLink:
		return r.pkgs.Library(file, imp.Module+".h")
	}
	return lang.Target{}
}

// local picks the defining file for file among files: the importing file itself
// is none; else the one sharing the longest directory prefix with it.
func (r *resolver) local(file string, files []string) lang.Target {
	best, bestLen := "", -1
	for _, f := range files {
		if f == file {
			continue
		}
		if n := commonDirs(f, file); n > bestLen {
			best, bestLen = f, n
		}
	}
	if best == "" {
		return lang.Target{}
	}
	return lang.Target{Local: best}
}

func commonDirs(a, b string) int {
	as, bs := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// use resolves a module: a project file that defines it, the compiler's
// intrinsic modules, a C library's Fortran module, a dependency fpm fetched that
// defines it, a declared dependency its name spells, a module [build]
// external-modules lists, the curated table, and else an unresolved module.
//
// Implements: REQ-FORTRAN-004, REQ-FORTRAN-007
func (r *resolver) use(file, module, kind string) lang.Target {
	if kind == kindIntrinsic {
		return lang.Target{Ecosystem: ecoStd, Package: stdPackage(module)}
	}
	if files := r.modules[module]; len(files) > 0 {
		if t := r.local(file, files); t.Local != "" {
			return t
		}
		return lang.Target{} // the module the file itself defines
	}
	if kind != kindNonIntrinsic {
		if p, ok := intrinsic[module]; ok {
			return lang.Target{Ecosystem: ecoStd, Package: p}
		}
	}
	if h, ok := libraries[module]; ok {
		return r.pkgs.Library(file, h, module)
	}
	scope := r.scope(file)
	for _, p := range scope {
		if name, ok := p.modules[module]; ok {
			return r.dependency(p, name)
		}
	}
	for _, dev := range []bool{false, true} {
		for _, p := range scope {
			for _, name := range sortedKeys(p.m.deps) {
				if d := p.m.deps[name]; d.dev == dev && d.path == "" && spells(module, name) {
					return r.dependency(p, name)
				}
			}
		}
	}
	for _, p := range scope {
		for _, e := range p.m.externalModules {
			if e == module {
				return lang.Target{Ecosystem: ecoExternal, Package: module}
			}
		}
	}
	if pkg := knownPackage(module); pkg != "" {
		for _, p := range scope {
			if p.m.name != "" && fold(p.m.name) == fold(pkg) {
				return lang.Target{} // the project's own module, missing
			}
			for _, name := range sortedKeys(p.m.deps) {
				if fold(name) == fold(pkg) {
					return r.dependency(p, name)
				}
			}
		}
		return lang.Target{Ecosystem: ecoFpm, Package: pkg, Unresolved: true}
	}
	for _, p := range scope {
		if p.m.name != "" && spells(module, p.m.name) {
			return lang.Target{} // the project's own module, missing
		}
	}
	return lang.Target{Ecosystem: ecoExternal, Package: module, Unresolved: true}
}

// external is a module [build] external-modules lists: a C library's module, or
// a module of the system's that is none of the project's.
func (r *resolver) external(file, module string) lang.Target {
	if h, ok := libraries[module]; ok {
		return r.pkgs.Library(file, h, module)
	}
	if p, ok := intrinsic[module]; ok {
		return lang.Target{Ecosystem: ecoStd, Package: p}
	}
	return lang.Target{Ecosystem: ecoExternal, Package: module}
}

// include resolves an included file: next to the including file, then in the
// include directories of the fpm projects over it, then in an include/
// directory or the directory itself of each ancestor. A file of a library's
// installation (mpif.h) and a system header (#include <x>) not in the project
// are the library's; anything else is dropped.
//
// Implements: REQ-FORTRAN-006
func (r *resolver) include(file, spec, kind string) lang.Target {
	dir := path.Dir(file)
	if !path.IsAbs(spec) {
		if kind != kindCppSys {
			if t := path.Join(dir, spec); r.files[t] {
				return lang.Target{Local: t}
			}
		}
		for _, p := range r.scope(file) {
			for _, inc := range p.m.includeDirs {
				if t := path.Join(p.dir, inc, spec); r.files[t] {
					return lang.Target{Local: t}
				}
			}
		}
		for d := dir; ; d = path.Dir(d) {
			for _, t := range []string{path.Join(d, "include", spec), path.Join(d, spec)} {
				if t != file && r.files[t] && !strings.HasPrefix(t, "../") {
					return lang.Target{Local: t}
				}
			}
			if d == "." {
				break
			}
		}
	}
	base := strings.ToLower(path.Base(spec))
	if h, ok := includeLibraries[base]; ok {
		if h == "" {
			return lang.Target{Ecosystem: ecoStd, Package: "openmp"}
		}
		return r.pkgs.Library(file, h)
	}
	if kind == kindCppSys {
		return r.pkgs.Library(file, spec)
	}
	return lang.Target{}
}

// dependency is the dependency name as project p declares it, pinned by fpm's
// rules: a git rev pins; a tag names one release but can be moved, so it is
// shown, neither pinned nor floating; a branch or no ref floats; a registry
// package's v pins, none floats. A path dependency inside the repository is its
// directory. What build/cache.toml says fpm fetched is shown as the version.
//
// Implements: REQ-FORTRAN-005
func (r *resolver) dependency(p *project, name string) lang.Target {
	d := p.m.deps[name]
	if d == nil {
		// fetched as a dependency of a dependency
		t := lang.Target{Ecosystem: ecoFpm, Package: name}
		if c := p.cache[name]; c != nil {
			r.cachedVersion(&t, c, nil)
		}
		if !public(p.cacheGit(name)) {
			t.Origin = p.cacheGit(name)
		}
		return t
	}
	if d.path != "" {
		dir := path.Join(p.dir, d.path)
		switch {
		case r.files[path.Join(dir, "fpm.toml")]:
			return lang.Target{Local: path.Join(dir, "fpm.toml")}
		case r.dirs[dir]:
			return lang.Target{Local: dir}
		}
		return lang.Target{}
	}
	if d.metaSet {
		if name == "openmp" {
			return lang.Target{Ecosystem: ecoStd, Package: "openmp"}
		}
		if h, ok := metaLibraries[name]; ok {
			return r.pkgs.Library(path.Join(p.dir, "fpm.toml"), h, name)
		}
	}
	t := lang.Target{Ecosystem: ecoFpm, Package: name}
	pinRule(&t, d)
	if d.git != "" && !public(d.git) {
		t.Origin = d.git
	}
	if c := p.cache[name]; c != nil {
		r.cachedVersion(&t, c, d)
	}
	return t
}

func (p *project) cacheGit(name string) string {
	if c := p.cache[name]; c != nil {
		return c.git
	}
	return ""
}

// cachedVersion shows what fpm fetched: a floating or unversioned dependency's
// revision or version, with the requirement kept as Requested.
func (r *resolver) cachedVersion(t *lang.Target, c *cached, d *dependency) {
	if t.Pinned {
		return
	}
	v := c.rev
	if v == "" {
		v = c.version
	}
	if v == "" || v == t.Version {
		return
	}
	if d != nil && t.Version != "" {
		t.Requested = t.Version
	}
	t.Version = v
}

// pinRule is fpm's pinning: there is no lock file.
//
// Implements: REQ-FORTRAN-005
func pinRule(t *lang.Target, d *dependency) {
	switch {
	case d.rev != "":
		t.Version, t.Pinned = d.rev, true
	case d.tag != "":
		t.Version = d.tag
	case d.branch != "":
		t.Version, t.Floating = d.branch, true
	case d.namespace != "" && d.v != "":
		t.Version, t.Pinned = d.v, true
	case d.metaSet && d.meta != "*" && d.meta != "":
		t.Version = d.meta
		t.Pinned = lang.Pinned(d.meta)
		t.Floating = !t.Pinned
	default:
		t.Version, t.Floating = d.meta, true
	}
}

// public reports whether a repository is on a public forge (or unknown), as
// opposed to a git server of the organization's own, which is recorded as the
// package's origin (and keeps it from being named to an index).
func public(url string) bool {
	if url == "" {
		return true
	}
	host, _, _ := strings.Cut(lang.RepoName(url), "/")
	switch host {
	case "github.com", "gitlab.com", "bitbucket.org", "codeberg.org", "git.sr.ht", "sr.ht":
		return true
	}
	return false
}

// Dependencies lists what a dependency fpm fetched into build/dependencies/
// depends on, from the fpm.toml it ships (not its dev-dependencies), versioned
// as the fetching project's cache and manifest say.
//
// Implements: REQ-FORTRAN-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoFpm {
		return nil
	}
	for _, p := range r.order {
		in, ok := p.installed[t.Package]
		if !ok {
			continue
		}
		var out []lang.Target
		for _, name := range sortedKeys(in.m.deps) {
			d := in.m.deps[name]
			if d.dev || d.path != "" {
				continue
			}
			if p.m.deps[name] != nil || p.cache[name] != nil {
				if dt := r.dependency(p, name); dt.Ecosystem != "" {
					out = append(out, dt)
				}
				continue
			}
			if d.metaSet {
				if dt := r.dependency(&project{m: in.m, cache: map[string]*cached{}}, name); dt.Ecosystem != "" {
					out = append(out, dt)
				}
				continue
			}
			dt := lang.Target{Ecosystem: ecoFpm, Package: name}
			pinRule(&dt, d)
			if d.git != "" && !public(d.git) {
				dt.Origin = d.git
			}
			out = append(out, dt)
		}
		return out
	}
	return nil
}

// Installed reports whether a package's dependencies come from what fpm fetched
// into build/dependencies/.
func (r *resolver) Installed(t lang.Target) bool {
	if t.Ecosystem != ecoFpm {
		return false
	}
	for _, p := range r.order {
		if _, ok := p.installed[t.Package]; ok {
			return true
		}
	}
	return false
}
