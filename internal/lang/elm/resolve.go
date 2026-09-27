package elm

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a directory with an elm.json.
type project struct {
	dir   string
	m     *manifest
	roots []string // source directories, relative to the repository root
	tests string   // the elm-test directory, dir/tests
	// modules maps a module of an installed package to that package: from the
	// exposed-modules of each dependency's elm.json in ELM_HOME, when it is there.
	modules map[string]string
}

type resolver struct {
	files    map[string]bool
	dirs     map[string]bool
	projects []*project          // sorted by directory
	byDir    map[string]*project // by directory
	home     string              // ELM_HOME
	// installed memoizes the elm.json of installed packages, by "author/name@version"
	// (nil when not on disk).
	installed sync.Map
}

// elmHome is where the compiler keeps downloaded packages: ELM_HOME, else ~/.elm.
func elmHome() string {
	if h := os.Getenv("ELM_HOME"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".elm")
	}
	return ""
}

// Implements: REQ-ELM-004, REQ-ELM-005, REQ-ELM-007
func newResolver(root string, all []*scan.File, home string) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, byDir: map[string]*project{}, home: home}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
	for _, f := range all {
		if path.Base(f.Path) != "elm.json" || inElmStuff(f.Path) {
			continue
		}
		src, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		m := readManifest(src)
		if m == nil {
			continue
		}
		dir := path.Dir(f.Path)
		p := &project{dir: dir, m: m, tests: path.Join(dir, "tests"), modules: map[string]string{}}
		dirs := m.srcDirs
		if m.kind == "package" {
			dirs = []string{"src"}
		}
		for _, d := range dirs {
			if d = path.Join(dir, strings.TrimSpace(d)); inside(d) {
				p.roots = append(p.roots, d)
			}
		}
		for _, name := range sortedKeys(m.deps) {
			if in := r.installedManifest(name, r.installedVersion(m, m.deps[name])); in != nil {
				for _, mod := range in.exposed {
					if _, ok := p.modules[mod]; !ok {
						p.modules[mod] = name
					}
				}
			}
		}
		r.projects = append(r.projects, p)
		r.byDir[dir] = p
	}
	sort.Slice(r.projects, func(i, j int) bool { return r.projects[i].dir < r.projects[j].dir })
	return r
}

// inside reports whether a cleaned relative path stays in the repository.
func inside(p string) bool { return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p) }

// under reports whether file is inside dir ("." holds everything).
func under(file, dir string) bool { return dir == "." || strings.HasPrefix(file, dir+"/") }

// candidate is a project that lists a module file in its source directories, with
// the roots its imports are looked up under.
type candidate struct {
	p     *project
	roots []string
}

// candidates are the projects a module file belongs to, best first. Several
// elm.json files may list one directory (an examples/ application with
// "../src"): the projects whose own directory holds the file come first, the
// deepest first, then the others by the longest source directory holding the
// file. A file no project lists belongs to the nearest elm.json above it. A file
// under a project's tests/ looks there first (elm-test).
func (r *resolver) candidates(file string) []candidate {
	type scored struct {
		p        *project
		ancestor bool
		n        int
	}
	var found []scored
	for _, p := range r.projects {
		n := -1
		if under(file, p.tests) {
			n = len(p.tests)
		}
		for _, root := range p.roots {
			if under(file, root) && len(root) > n {
				n = len(root)
			}
		}
		if n >= 0 {
			found = append(found, scored{p, under(file, p.dir), n})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.ancestor != b.ancestor {
			return a.ancestor
		}
		if a.ancestor {
			return len(a.p.dir) > len(b.p.dir)
		}
		return a.n > b.n
	})
	if len(found) == 0 {
		for d := path.Dir(file); ; d = path.Dir(d) {
			if p := r.byDir[d]; p != nil {
				found = append(found, scored{p: p})
				break
			}
			if d == "." || d == "/" {
				return nil
			}
		}
	}
	out := make([]candidate, len(found))
	for i, f := range found {
		out[i] = candidate{p: f.p, roots: f.p.roots}
		if under(file, f.p.tests) {
			out[i].roots = append([]string{f.p.tests}, f.p.roots...)
		}
	}
	return out
}

// Implements: REQ-ELM-004, REQ-ELM-005, REQ-ELM-006
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindModule:
		return r.module(file, imp.Module)
	case kindDep:
		if p := r.byDir[path.Dir(file)]; p != nil && p.m.deps[imp.Module] != nil {
			return target(p, imp.Module)
		}
	case kindSrcDir:
		if p := r.byDir[path.Dir(file)]; p != nil {
			if d := path.Join(p.dir, imp.Module); inside(d) && (r.dirs[d] || d == ".") {
				return lang.Target{Local: d}
			}
		}
	}
	return lang.Target{}
}

// module resolves an import, in order: to A/B.elm under the source directories
// of the projects listing the file, best first (tests/ first for a file under
// tests/); a kernel module (Elm.Kernel.List) to its .js file there, else to
// elm/core or the declared package it spells (Elm.Kernel.VirtualDom is
// elm/virtual-dom); to the dependency whose installed elm.json
// exposes it; an elm/core module to elm/core; to the declared package the
// curated table or the package's own name spells (the longer match wins); to a
// declared package whose name starts like the module (Iso8601 is
// rtfeldman/elm-iso8601-date-strings); to the unresolved package the table
// names; else it is dropped: an Elm module name does not say which author
// published it.
//
// Implements: REQ-ELM-004, REQ-ELM-007, REQ-ELM-011
func (r *resolver) module(file, mod string) lang.Target {
	candidates := r.candidates(file)
	rel := strings.ReplaceAll(mod, ".", "/")
	kernel, isKernel := strings.CutPrefix(mod, "Elm.Kernel.")
	ext := ".elm"
	if isKernel {
		ext = ".js" // the kernel code of elm/core, elm/html's elm/virtual-dom, ...
	}
	for _, c := range candidates {
		for _, root := range c.roots {
			if f := path.Join(root, rel+ext); r.files[f] {
				return lang.Target{Local: f}
			}
		}
	}
	var p *project
	if len(candidates) > 0 {
		p = candidates[0].p
	}
	if isKernel {
		// Another kernel package's JavaScript: the declared package it names.
		if p != nil && coreKernel[kernel] {
			return target(p, core)
		}
		if p != nil {
			if name, _ := spelled(p, kernel); name != "" {
				return target(p, name)
			}
		}
		return lang.Target{}
	}
	if p == nil {
		if coreModules[mod] {
			return lang.Target{Ecosystem: ecoElm, Package: core, Unresolved: true}
		}
		if name, ok := knownPackage(mod, func(string) bool { return false }); ok {
			return lang.Target{Ecosystem: ecoElm, Package: name, Unresolved: true}
		}
		return lang.Target{}
	}
	if name, ok := p.modules[mod]; ok {
		return target(p, name)
	}
	if coreModules[mod] {
		return target(p, core)
	}
	declared := func(name string) bool { return p.m.deps[name] != nil }
	name, n := spelled(p, mod)
	if known, ok := knownPackage(mod, declared); ok && declared(known) && segments(known, mod) >= n {
		return target(p, known)
	}
	if name != "" {
		return target(p, name)
	}
	if name = startsLike(p, mod); name != "" {
		return target(p, name)
	}
	if p.m.kind == "package" && p.m.name != "" {
		for _, e := range p.m.exposed {
			if e == mod {
				return lang.Target{} // the package's own module, missing from src/
			}
		}
	}
	if known, ok := knownPackage(mod, declared); ok {
		return target(p, known)
	}
	return lang.Target{}
}

// segments is how many leading segments of mod the table entry for pkg covers.
func segments(pkg, mod string) int {
	for m := mod; m != ""; m = parent(m) {
		if knownModules[m] == pkg {
			return strings.Count(m, ".") + 1
		}
	}
	return 0
}

// spelled is the declared package whose name, without an elm- prefix or -elm
// suffix and folded, equals the module's leading segments folded - the longest
// such run: List.Extra is elm-community/list-extra, Json.Decode.Pipeline
// NoRedInk/elm-json-decode-pipeline. It returns the package and how many
// segments matched.
func spelled(p *project, mod string) (string, int) {
	segs := strings.Split(mod, ".")
	best, n := "", 0
	for _, name := range sortedKeys(p.m.deps) {
		f := foldPackage(name)
		if f == "" {
			continue
		}
		for k := len(segs); k > n; k-- {
			if fold(strings.Join(segs[:k], "")) == f {
				best, n = name, k
				break
			}
		}
	}
	return best, n
}

// startsLike is the declared package whose folded name starts with the module's
// first segment folded, when that is at least four characters: Iso8601 is
// rtfeldman/elm-iso8601-date-strings.
func startsLike(p *project, mod string) string {
	first := mod
	if i := strings.IndexByte(mod, '.'); i >= 0 {
		first = mod[:i]
	}
	first = fold(first)
	if len(first) < 4 {
		return ""
	}
	for _, name := range sortedKeys(p.m.deps) {
		if strings.HasPrefix(foldPackage(name), first) {
			return name
		}
	}
	return ""
}

func foldPackage(name string) string {
	_, base, _ := strings.Cut(name, "/")
	base = strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(base), "elm-"), "-elm")
	return fold(base)
}

// fold keeps letters and digits, in lower case.
func fold(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c + 'a' - 'A')
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteByte(c)
		}
	}
	return b.String()
}

// target is a package as the project's elm.json lists it. An application lists
// exact versions, which pin; a package lists ranges ("1.0.0 <= v < 2.0.0"),
// which float as written. A package the project does not list is unresolved.
//
// Implements: REQ-ELM-005, REQ-ELM-006
func target(p *project, name string) lang.Target {
	d := p.m.deps[name]
	if d == nil {
		return lang.Target{Ecosystem: ecoElm, Package: name, Unresolved: true}
	}
	if _, ok := parseVersion(d.version); ok {
		return lang.Target{Ecosystem: ecoElm, Package: name, Version: d.version, Pinned: true}
	}
	return lang.Target{Ecosystem: ecoElm, Package: name, Version: d.version, Floating: true}
}

// installedVersion is the version of a dependency to look for in ELM_HOME: an
// exact version as written, else the newest installed one a range admits.
func (r *resolver) installedVersion(m *manifest, d *dependency) string {
	if _, ok := parseVersion(d.version); ok {
		return d.version
	}
	lo, hi, ok := parseRange(d.version)
	if !ok || r.home == "" {
		return ""
	}
	best, bestV := "", version{}
	for _, ev := range elmVersions(m) {
		entries, _ := os.ReadDir(filepath.Join(r.home, ev, "packages", filepath.FromSlash(d.name)))
		for _, e := range entries {
			v, ok := parseVersion(e.Name())
			if ok && e.IsDir() && lo.admits(v, hi) && (best == "" || bestV.less(v)) {
				best, bestV = e.Name(), v
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

// elmVersions are the compiler versions whose package directories are searched:
// the project's own when exact, then 0.19.1 and 0.19.0.
func elmVersions(m *manifest) []string {
	out := []string{"0.19.1", "0.19.0"}
	if m != nil {
		if _, ok := parseVersion(m.elmVersion); ok && m.elmVersion != out[0] && m.elmVersion != out[1] {
			out = append([]string{m.elmVersion}, out...)
		}
	}
	return out
}

// installedManifest is the elm.json of a package version the compiler downloaded
// into ELM_HOME, or nil.
func (r *resolver) installedManifest(name, ver string) *manifest {
	if r.home == "" || ver == "" || !validName(name) {
		return nil
	}
	key := name + "@" + ver
	if m, ok := r.installed.Load(key); ok {
		return m.(*manifest)
	}
	var found *manifest
	for _, ev := range elmVersions(nil) {
		src, err := os.ReadFile(filepath.Join(r.home, ev, "packages", filepath.FromSlash(name), ver, "elm.json"))
		if err == nil {
			if found = readManifest(src); found != nil {
				break
			}
		}
	}
	r.installed.Store(key, found)
	return found
}

// validName reports whether a package name is author/name without path tricks.
func validName(name string) bool {
	a, b, ok := strings.Cut(name, "/")
	return ok && a != "" && b != "" && !strings.Contains(b, "/") && !strings.Contains(name, "..")
}

// Dependencies implements lang.Transitive from what the compiler installed: the
// dependencies of the package's elm.json in ELM_HOME. An application of the
// repository that lists a dependency (all of them, directly or indirectly) pins
// it at its version; otherwise the package's range floats.
//
// Implements: REQ-ELM-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	m := r.installedFor(t)
	if m == nil {
		return nil
	}
	var out []lang.Target
	for _, name := range sortedKeys(m.deps) {
		d := m.deps[name]
		if d.test {
			continue
		}
		out = append(out, r.pinned(name, d.version))
	}
	return out
}

// Installed reports that Dependencies answered from ELM_HOME.
func (r *resolver) Installed(t lang.Target) bool { return r.installedFor(t) != nil }

func (r *resolver) installedFor(t lang.Target) *manifest {
	if t.Ecosystem != ecoElm || t.Package == "" {
		return nil
	}
	v := t.Version
	if _, ok := parseVersion(v); !ok {
		v = r.installedVersion(nil, &dependency{name: t.Package, version: v})
	}
	return r.installedManifest(t.Package, v)
}

// pinned is a dependency of an installed package: at the version an application
// of the repository lists, else its range.
func (r *resolver) pinned(name, rng string) lang.Target {
	for _, p := range r.projects {
		if d := p.m.deps[name]; d != nil && p.m.kind == "application" {
			if _, ok := parseVersion(d.version); ok {
				return lang.Target{Ecosystem: ecoElm, Package: name, Version: d.version, Pinned: true}
			}
		}
	}
	return lang.Target{Ecosystem: ecoElm, Package: name, Version: rng, Floating: true}
}

// version is an Elm package version: always major.minor.patch.
type version [3]int

func parseVersion(s string) (version, bool) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	var v version
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || p[0] == '+' {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func (v version) less(w version) bool {
	for i := range v {
		if v[i] != w[i] {
			return v[i] < w[i]
		}
	}
	return false
}

// bound is one side of a range; strict for `<`.
type bound struct {
	v      version
	strict bool
}

// parseRange reads an Elm constraint: "1.0.0 <= v < 2.0.0" (either side may be <
// or <=).
func parseRange(s string) (lo, hi bound, ok bool) {
	f := strings.Fields(s)
	if len(f) != 5 || f[2] != "v" || f[1] != "<" && f[1] != "<=" || f[3] != "<" && f[3] != "<=" {
		return lo, hi, false
	}
	a, ok1 := parseVersion(f[0])
	b, ok2 := parseVersion(f[4])
	if !ok1 || !ok2 {
		return lo, hi, false
	}
	return bound{a, f[1] == "<"}, bound{b, f[3] == "<"}, true
}

// admits reports whether v lies between the lower bound lo and the upper bound hi.
func (lo bound) admits(v version, hi bound) bool {
	if v.less(lo.v) || lo.strict && v == lo.v {
		return false
	}
	return v.less(hi.v) || !hi.strict && v == hi.v
}
