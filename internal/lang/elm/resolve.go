package elm

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a directory with an elm.json.
type project struct {
	directory string
	m         *manifest
	roots     []string // source directories, relative to the repository root
	tests     string   // the elm-test directory, dir/tests
	// modules maps a module of an installed package to that package: from the
	// exposed-modules of each dependency's elm.json in ELM_HOME, when it is there.
	modules map[string]string
}

type resolver struct {
	lang.Layout
	projects    []*project          // sorted by directory
	byDirectory map[string]*project // by directory
	home        string              // ELM_HOME
	// installed memoizes the elm.json of installed packages, by "author/name@version"
	// (nil when not on disk).
	installed lang.Memo[string, *manifest] // name@version -> its installed elm.json, or nil
}

// elmHome is where the compiler keeps downloaded packages: ELM_HOME (a relative
// one against the repository root), else ~/.elm.
func elmHome(root string) string {
	if h, _ := lang.FromEnvironment(root, os.Getenv("ELM_HOME")); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".elm")
	}
	return ""
}

// Implements: REQ-ELM-004, REQ-ELM-005, REQ-ELM-007
func newResolver(root string, all []*scan.File, home string) *resolver {
	r := &resolver{Layout: lang.NewLayout(), byDirectory: map[string]*project{}, home: home}
	for _, f := range all {
		r.Add(f.Path)
	}
	for _, f := range all {
		if path.Base(f.Path) != "elm.json" || inElmStuff(f.Path) {
			continue
		}
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		m := readManifest(source)
		if m == nil {
			continue
		}
		directory := path.Dir(f.Path)
		if _, exact := parseVersion(m.elmVersion); !exact {
			// A package allows a range of compilers: the one elm-tooling
			// installs decides which ELM_HOME directory holds its packages.
			if v := toolingElm(root, directory); v != "" {
				m.elmVersion = v
			}
		}
		p := &project{directory: directory, m: m, tests: path.Join(directory, "tests"), modules: map[string]string{}}
		directories := m.sourceDirectories
		if m.kind == "package" {
			directories = []string{"src"}
		}
		for _, d := range directories {
			if d = path.Join(directory, strings.TrimSpace(d)); lang.Inside(d) {
				p.roots = append(p.roots, d)
			}
		}
		for _, name := range lang.SortedKeys(m.dependencies) {
			if in := r.installedManifest(name, r.installedVersion(m, m.dependencies[name])); in != nil {
				for _, module := range in.exposed {
					if _, ok := p.modules[module]; !ok {
						p.modules[module] = name
					}
				}
			}
		}
		r.projects = append(r.projects, p)
		r.byDirectory[directory] = p
	}
	sort.Slice(r.projects, func(i, j int) bool { return r.projects[i].directory < r.projects[j].directory })
	return r
}

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
		if lang.Within(file, p.tests) {
			n = len(p.tests)
		}
		for _, root := range p.roots {
			if lang.Within(file, root) && len(root) > n {
				n = len(root)
			}
		}
		if n >= 0 {
			found = append(found, scored{p, lang.Within(file, p.directory), n})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.ancestor != b.ancestor {
			return a.ancestor
		}
		if a.ancestor {
			return len(a.p.directory) > len(b.p.directory)
		}
		return a.n > b.n
	})
	if len(found) == 0 {
		p, ok := lang.Nearest(r.byDirectory, file)
		if !ok {
			return nil
		}
		found = append(found, scored{p: p})
	}
	out := make([]candidate, len(found))
	for i, f := range found {
		out[i] = candidate{p: f.p, roots: f.p.roots}
		if lang.Within(file, f.p.tests) {
			out[i].roots = append([]string{f.p.tests}, f.p.roots...)
		}
	}
	return out
}

// Implements: REQ-ELM-004, REQ-ELM-005, REQ-ELM-006
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindModule:
		return r.module(file, rawImport.Module)
	case kindDependency:
		if p := r.byDirectory[path.Dir(file)]; p != nil && p.m.dependencies[rawImport.Module] != nil {
			return target(p, rawImport.Module)
		}
	case kindSourceDirectory:
		if p := r.byDirectory[path.Dir(file)]; p != nil {
			if d := path.Join(p.directory, rawImport.Module); lang.Inside(d) && (r.Directories[d] || d == ".") {
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
func (r *resolver) module(file, module string) lang.Target {
	candidates := r.candidates(file)
	relative := strings.ReplaceAll(module, ".", "/")
	kernel, isKernel := strings.CutPrefix(module, "Elm.Kernel.")
	extension := ".elm"
	if isKernel {
		extension = ".js" // the kernel code of elm/core, elm/html's elm/virtual-dom, ...
	}
	for _, c := range candidates {
		for _, root := range c.roots {
			if f := path.Join(root, relative+extension); r.Files[f] {
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
		if coreModules[module] {
			return lang.Target{Ecosystem: ecosystemElm, Package: core, Unresolved: true}
		}
		if name, ok := knownPackage(module, func(string) bool { return false }); ok {
			return lang.Target{Ecosystem: ecosystemElm, Package: name, Unresolved: true}
		}
		return lang.Target{}
	}
	if name, ok := p.modules[module]; ok {
		return target(p, name)
	}
	if coreModules[module] {
		return target(p, core)
	}
	declared := func(name string) bool { return p.m.dependencies[name] != nil }
	name, n := spelled(p, module)
	if known, ok := knownPackage(module, declared); ok && declared(known) && segments(known, module) >= n {
		return target(p, known)
	}
	if name != "" {
		return target(p, name)
	}
	if name = startsLike(p, module); name != "" {
		return target(p, name)
	}
	if p.m.kind == "package" && p.m.name != "" {
		for _, e := range p.m.exposed {
			if e == module {
				return lang.Target{} // the package's own module, missing from src/
			}
		}
	}
	if known, ok := knownPackage(module, declared); ok {
		return target(p, known)
	}
	return lang.Target{}
}

// segments is how many leading segments of module the table entry for packageName covers.
func segments(packageName, module string) int {
	for m := module; m != ""; m = parent(m) {
		if knownModules[m] == packageName {
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
func spelled(p *project, module string) (string, int) {
	segments := strings.Split(module, ".")
	best, n := "", 0
	for _, name := range lang.SortedKeys(p.m.dependencies) {
		f := foldPackage(name)
		if f == "" {
			continue
		}
		for k := len(segments); k > n; k-- {
			if fold(strings.Join(segments[:k], "")) == f {
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
func startsLike(p *project, module string) string {
	first := module
	if i := strings.IndexByte(module, '.'); i >= 0 {
		first = module[:i]
	}
	first = fold(first)
	if len(first) < 4 {
		return ""
	}
	for _, name := range lang.SortedKeys(p.m.dependencies) {
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
	d := p.m.dependencies[name]
	if d == nil {
		return lang.Target{Ecosystem: ecosystemElm, Package: name, Unresolved: true}
	}
	if _, ok := parseVersion(d.version); ok {
		return lang.Target{Ecosystem: ecosystemElm, Package: name, Version: d.version, Pinned: true}
	}
	return lang.Target{Ecosystem: ecosystemElm, Package: name, Version: d.version, Floating: true}
}

// installedVersion is the version of a dependency to look for in ELM_HOME: an
// exact version as written, else the newest installed one a range admits.
func (r *resolver) installedVersion(m *manifest, d *dependency) string {
	if _, ok := parseVersion(d.version); ok {
		return d.version
	}
	low, high, ok := parseRange(d.version)
	if !ok || r.home == "" {
		return ""
	}
	best, bestV := "", version{}
	for _, elmVersion := range elmVersions(m) {
		entries, _ := os.ReadDir(filepath.Join(r.home, elmVersion, "packages", filepath.FromSlash(d.name)))
		for _, e := range entries {
			v, ok := parseVersion(e.Name())
			if ok && e.IsDir() && low.admits(v, high) && (best == "" || bestV.less(v)) {
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
		if _, ok := parseVersion(m.elmVersion); ok {
			out = append([]string{m.elmVersion}, slices.DeleteFunc(out, func(v string) bool { return v == m.elmVersion })...)
		}
	}
	return out
}

// installedManifest is the elm.json of a package version the compiler downloaded
// into ELM_HOME, or nil.
func (r *resolver) installedManifest(name, version string) *manifest {
	if r.home == "" || version == "" || !validName(name) {
		return nil
	}
	return r.installed.Get(name+"@"+version, func(string) *manifest {
		for _, elmVersion := range elmVersions(nil) {
			source, err := os.ReadFile(filepath.Join(r.home, elmVersion, "packages", filepath.FromSlash(name), version, "elm.json"))
			if err == nil {
				if found := readManifest(source); found != nil {
					return found
				}
			}
		}
		return nil
	})
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
	for _, name := range lang.SortedKeys(m.dependencies) {
		d := m.dependencies[name]
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
	if t.Ecosystem != ecosystemElm || t.Package == "" {
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
func (r *resolver) pinned(name, versionRange string) lang.Target {
	for _, p := range r.projects {
		if d := p.m.dependencies[name]; d != nil && p.m.kind == "application" {
			if _, ok := parseVersion(d.version); ok {
				return lang.Target{Ecosystem: ecosystemElm, Package: name, Version: d.version, Pinned: true}
			}
		}
	}
	return lang.Target{Ecosystem: ecosystemElm, Package: name, Version: versionRange, Floating: true}
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
func parseRange(s string) (low, high bound, ok bool) {
	f := strings.Fields(s)
	if len(f) != 5 || f[2] != "v" || f[1] != "<" && f[1] != "<=" || f[3] != "<" && f[3] != "<=" {
		return low, high, false
	}
	a, ok1 := parseVersion(f[0])
	b, ok2 := parseVersion(f[4])
	if !ok1 || !ok2 {
		return low, high, false
	}
	return bound{a, f[1] == "<"}, bound{b, f[3] == "<"}, true
}

// admits reports whether v lies between the lower bound lo and the upper bound hi.
func (low bound) admits(v version, high bound) bool {
	if v.less(low.v) || low.strict && v == low.v {
		return false
	}
	return v.less(high.v) || !high.strict && v == high.v
}
