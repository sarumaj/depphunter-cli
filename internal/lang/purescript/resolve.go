package purescript

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/dhall"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// workspace is what decides the versions of a set of packages: a spago.yaml
// workspace (its package set, extra packages and spago.lock) or the package set a
// spago.dhall configuration uses.
type workspace struct {
	dir    string
	set    string // the package set's name, shown as the version of what it provides
	extra  map[string]*extraPackage
	lock   *spagoLock
	locals map[string]string // the workspace's own packages -> their manifest
	// installed is what spago installed into the workspace's .spago/, by name.
	installed map[string]*installedPackage
}

// installedPackage is a package spago installed: its version (or git ref) and
// the dependencies its own manifest lists.
type installedPackage struct {
	version string
	pinned  bool
	deps    []string
}

// project is one PureScript package of the repository: a spago.yaml package, a
// spago.dhall configuration or a bower.json.
type project struct {
	file, dir string
	kind      string // classYAML, classDhall or classBower
	name      string
	globs     []string // source globs, relative to the repository
	testGlobs []string
	deps      map[string]dependency
	ws        *workspace
}

type resolver struct {
	root     string
	files    map[string]bool
	dirs     map[string]bool
	projects []*project
	byFile   map[string]*project // by manifest
	wsByDir  map[string]*workspace
	owner    map[string]*project // a module file's project
	test     map[string]bool     // a module file read by a project's test globs
	modules  map[string][]string // module name -> files declaring it, sorted
	// evaluated memoizes Dhall files; nil while one is being evaluated (an
	// import cycle).
	evaluated map[string]*dhall.Value
	setWS     map[*dhall.Value]*workspace
	setFile   map[string]*workspace // a packages.dhall's workspace
	installed sync.Map              // dir -> *installedIndex
}

// Implements: REQ-PURESCRIPT-004, REQ-PURESCRIPT-005, REQ-PURESCRIPT-007
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, dirs: map[string]bool{}, byFile: map[string]*project{},
		wsByDir: map[string]*workspace{}, owner: map[string]*project{}, test: map[string]bool{},
		modules: map[string][]string{}, evaluated: map[string]*dhall.Value{}, setWS: map[*dhall.Value]*workspace{}, setFile: map[string]*workspace{}}
	var sources []*scan.File
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		if path.Ext(f.Path) == ".purs" && !installedPath(f.Path) && !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize {
			sources = append(sources, f)
		}
	}
	r.readYAML(all)
	r.readDhall(all)
	r.readBower(all)
	if root != "" {
		for _, ws := range r.workspaces() {
			ws.installed = readSpagoInstalled(filepath.Join(root, filepath.FromSlash(ws.dir), ".spago"))
		}
	}
	sort.Slice(r.projects, func(i, j int) bool {
		a, b := r.projects[i], r.projects[j]
		if a.dir != b.dir {
			return a.dir < b.dir
		}
		if a.kind != b.kind {
			return kindRank[a.kind] < kindRank[b.kind]
		}
		return a.file < b.file
	})
	for _, f := range sources {
		if src, err := os.ReadFile(f.Abs); err == nil {
			if m := moduleName(src); m != "" {
				r.modules[m] = append(r.modules[m], f.Path)
			}
		}
		r.own(f.Path)
	}
	return r
}

// kindRank orders projects of one directory: spago.yaml, then spago.dhall, then
// bower.json.
var kindRank = map[string]int{classYAML: 0, classDhall: 1, classBower: 2}

func (r *resolver) read(f *scan.File) []byte {
	if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
		return nil
	}
	src, _ := os.ReadFile(f.Abs)
	return src
}

// readYAML reads every spago.yaml: workspaces (with their spago.lock, read from
// disk) and packages, each package in the nearest workspace above it.
func (r *resolver) readYAML(all []*scan.File) {
	type found struct {
		f *scan.File
		m *spagoYAML
	}
	var pkgs []found
	for _, f := range all {
		if path.Base(f.Path) != "spago.yaml" || installedPath(f.Path) {
			continue
		}
		m := readSpagoYAML(r.read(f))
		if m == nil {
			continue
		}
		dir := path.Dir(f.Path)
		if m.workspace {
			ws := &workspace{dir: dir, set: m.set, extra: map[string]*extraPackage{}, locals: map[string]string{}}
			for _, e := range m.extra {
				e.dir = dir
				ws.extra[e.name] = e
			}
			if src, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(dir), "spago.lock")); err == nil {
				ws.lock = readLock(src)
				if ws.lock != nil && ws.set == "" {
					ws.set = ws.lock.set
				}
			}
			r.wsByDir[dir] = ws
		}
		if m.isPackage {
			pkgs = append(pkgs, found{f, m})
		}
	}
	for _, p := range pkgs {
		dir := path.Dir(p.f.Path)
		ws := r.nearestWorkspace(dir)
		if ws == nil {
			ws = &workspace{dir: dir, extra: map[string]*extraPackage{}, locals: map[string]string{}}
		}
		pr := &project{file: p.f.Path, dir: dir, kind: classYAML, name: p.m.name, deps: map[string]dependency{}, ws: ws,
			globs: []string{path.Join(dir, "src/**/*.purs")}, testGlobs: []string{path.Join(dir, "test/**/*.purs")}}
		for _, d := range p.m.deps {
			if _, ok := pr.deps[d.name]; !ok || !d.test {
				pr.deps[d.name] = d
			}
		}
		if pr.name != "" {
			ws.locals[pr.name] = p.f.Path
		}
		r.projects = append(r.projects, pr)
		r.byFile[p.f.Path] = pr
	}
	// A lock's local packages are the workspace's too (their spago.yaml may be
	// missing from the scan).
	for _, ws := range r.wsByDir {
		if ws.lock == nil {
			continue
		}
		for name, p := range ws.lock.locals {
			if _, ok := ws.locals[name]; !ok {
				if d := path.Join(ws.dir, p); inside(d) && (r.files[path.Join(d, "spago.yaml")]) {
					ws.locals[name] = path.Join(d, "spago.yaml")
				}
			}
		}
	}
}

// nearestWorkspace is the spago.yaml workspace in dir or the nearest above it.
func (r *resolver) nearestWorkspace(dir string) *workspace {
	for d := dir; ; d = path.Dir(d) {
		if ws := r.wsByDir[d]; ws != nil {
			return ws
		}
		if d == "." || d == "/" {
			return nil
		}
	}
}

// readDhall evaluates every spago.dhall configuration: a Dhall file with
// dependencies or sources, its imports of other local files followed.
func (r *resolver) readDhall(all []*scan.File) {
	byPath := map[string]*scan.File{}
	for _, f := range all {
		if path.Ext(f.Path) == ".dhall" && !installedPath(f.Path) {
			byPath[f.Path] = f
		}
	}
	for _, p := range sortedKeys(byPath) {
		if path.Base(p) == "packages.dhall" {
			if v := r.loadDhall(p); v != nil {
				r.setFile[p] = r.dhallWorkspace(v, path.Dir(p))
			}
			continue
		}
		src := r.read(byPath[p])
		if !strings.Contains(string(src), "dependencies") && !strings.Contains(string(src), "sources") {
			continue
		}
		v := r.loadDhall(p)
		if v == nil || !dhallConfig(v) {
			continue
		}
		dir := path.Dir(p)
		pr := &project{file: p, dir: dir, kind: classDhall, deps: map[string]dependency{}}
		if n := v.Field("name"); n.Kind == dhall.KindText {
			pr.name = n.Text
		}
		for _, t := range v.Field("dependencies").Texts() {
			if _, ok := pr.deps[t.Text]; !ok {
				pr.deps[t.Text] = dependency{name: t.Text, line: t.Line}
			}
		}
		for _, t := range v.Field("sources").Texts() {
			pr.globs = append(pr.globs, r.globBase(dir, t.Text))
		}
		pr.ws = r.dhallWorkspace(v.Field("packages"), dir)
		r.projects = append(r.projects, pr)
		r.byFile[p] = pr
	}
}

// loadDhall evaluates a Dhall file of the repository, following its imports of
// other local files (a cycle, or a file that is not there, stays an import).
func (r *resolver) loadDhall(file string) *dhall.Value {
	if v, ok := r.evaluated[file]; ok {
		return v // nil while in progress: a cycle
	}
	if !r.files[file] || len(r.evaluated) > 1000 {
		return nil
	}
	src, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(file)))
	if err != nil || len(src) > lang.MaxParseSize {
		return nil
	}
	r.evaluated[file] = nil
	v := dhall.Eval(src, path.Dir(file), r.loadDhall)
	r.evaluated[file] = v
	return v
}

// dhallWorkspace is the workspace of a package set value: its name and the
// packages it adds or overrides. One set evaluated once is one workspace.
func (r *resolver) dhallWorkspace(set *dhall.Value, dir string) *workspace {
	if ws := r.setWS[set]; ws != nil && set != dhall.Unknown {
		return ws
	}
	ws := &workspace{dir: dir, set: setName(set), extra: map[string]*extraPackage{}, locals: map[string]string{}}
	setDir := dir
	for f, v := range r.evaluated {
		if v == set && path.Base(f) == "packages.dhall" {
			setDir = path.Dir(f)
		}
	}
	for _, e := range dhallExtras(set, setDir) {
		ws.extra[e.name] = e
	}
	if set != dhall.Unknown {
		r.setWS[set] = ws
	}
	return ws
}

// globBase makes a spago.dhall source glob relative to the repository. Legacy
// spago read globs relative to where it ran, which is the configuration's
// directory for most projects and the repository root for configurations that
// extend another (examples/basic/spago.dhall listing "examples/basic/**"): the
// first of the configuration's directory and its ancestors under which the
// glob's literal part exists wins.
func (r *resolver) globBase(dir, glob string) string {
	lit := glob
	if i := strings.IndexAny(glob, "*?["); i >= 0 {
		lit = path.Dir(glob[:i+1])
	}
	for d := dir; ; d = path.Dir(d) {
		if p := path.Join(d, lit); inside(p) && (r.dirs[p] || r.files[p] || p == ".") {
			return path.Join(d, glob)
		}
		if d == "." || d == "/" {
			break
		}
	}
	return path.Join(dir, glob)
}

// readBower reads bower.json files that name PureScript packages; each is a
// project reading src/ and test/.
func (r *resolver) readBower(all []*scan.File) {
	for _, f := range all {
		if path.Base(f.Path) != "bower.json" || installedPath(f.Path) {
			continue
		}
		m := readBower(r.read(f))
		if m == nil {
			continue
		}
		dir := path.Dir(f.Path)
		pr := &project{file: f.Path, dir: dir, kind: classBower, name: m.name, deps: map[string]dependency{},
			globs: []string{path.Join(dir, "src/**/*.purs")}, testGlobs: []string{path.Join(dir, "test/**/*.purs")},
			ws: &workspace{dir: dir, extra: map[string]*extraPackage{}, locals: map[string]string{}}}
		for _, d := range m.deps {
			if _, ok := pr.deps[d.name]; !ok || !d.test {
				pr.deps[d.name] = d
			}
		}
		r.projects = append(r.projects, pr)
		r.byFile[f.Path] = pr
	}
}

// own finds the project a module file belongs to: of the projects whose source
// globs match it, the one whose directory holds it (the deepest), else the first;
// a file no glob matches belongs to the nearest project above it.
func (r *resolver) own(file string) {
	var best *project
	bestTest, bestAnc := false, false
	for _, p := range r.projects {
		m, t := matchAny(p.globs, file), false
		if !m {
			if m = matchAny(p.testGlobs, file); m {
				t = true
			}
		}
		if !m {
			continue
		}
		anc := under(file, p.dir)
		if best == nil || anc && !bestAnc || anc && bestAnc && len(p.dir) > len(best.dir) {
			best, bestTest, bestAnc = p, t, anc
		}
	}
	if best == nil {
		for d := path.Dir(file); best == nil; d = path.Dir(d) {
			for _, p := range r.projects {
				if p.dir == d {
					best = p
					bestTest = strings.HasPrefix(file, path.Join(d, "test")+"/")
					break
				}
			}
			if d == "." || d == "/" {
				break
			}
		}
	}
	if best != nil {
		r.owner[file] = best
		r.test[file] = bestTest
	}
}

// matchAny reports whether a path matches one of the globs (`**` is any number
// of directories).
func matchAny(globs []string, p string) bool {
	for _, g := range globs {
		if globMatch(g, p) {
			return true
		}
	}
	return false
}

func globMatch(glob, p string) bool {
	return matchSegments(strings.Split(glob, "/"), strings.Split(p, "/"), 0)
}

func matchSegments(g, p []string, depth int) bool {
	for len(g) > 0 {
		if g[0] == "**" {
			if depth > 8 {
				return false
			}
			for k := 0; k <= len(p); k++ {
				if matchSegments(g[1:], p[k:], depth+1) {
					return true
				}
			}
			return false
		}
		if len(p) == 0 {
			return false
		}
		if ok, _ := path.Match(g[0], p[0]); !ok {
			return false
		}
		g, p = g[1:], p[1:]
	}
	return len(p) == 0
}

func inside(p string) bool { return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p) }

func under(file, dir string) bool { return dir == "." || strings.HasPrefix(file, dir+"/") }

// moduleName reads the name a module file declares in its header, or "".
func moduleName(src []byte) string {
	tokens := lex(src[:min(len(src), 64<<10)])
	if len(tokens) >= 2 && tokens[0].kind == tLower && tokens[0].text == "module" && tokens[1].kind == tUpper {
		return tokens[1].text
	}
	return ""
}

// Implements: REQ-PURESCRIPT-004, REQ-PURESCRIPT-005, REQ-PURESCRIPT-006
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindModule:
		return r.module(file, imp.Module)
	case kindFFI:
		// The JavaScript companion of a module with foreign imports.
		if js := strings.TrimSuffix(file, ".purs") + ".js"; r.files[js] {
			return lang.Target{Local: js}
		}
	case kindDep:
		if p := r.byFile[file]; p != nil {
			return r.target(p, imp.Module)
		}
	case kindExtra:
		if p := r.byFile[file]; p != nil {
			return r.fromWorkspace(p.ws, imp.Module, "")
		}
		if ws := r.wsByDir[path.Dir(file)]; ws != nil {
			return r.fromWorkspace(ws, imp.Module, "")
		}
		if ws := r.setFile[file]; ws != nil {
			return r.fromWorkspace(ws, imp.Module, "")
		}
	case kindLock:
		if ws := r.wsByDir[path.Dir(file)]; ws != nil {
			return r.fromWorkspace(ws, imp.Module, "")
		}
		if src, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(file))); err == nil {
			if l := readLock(src); l != nil {
				return r.fromWorkspace(&workspace{dir: path.Dir(file), lock: l}, imp.Module, "")
			}
		}
	}
	return lang.Target{}
}

// module resolves an import, in order: a Prim module to the compiler's
// built-ins; a module a source file of the importing project declares; one a
// package of its workspace declares; the package spago installed in .spago/ (or
// bower in bower_components/) that provides it; the listed package that the
// curated table or the package's own name spells (the longer match wins, the
// table on a tie); another project's file declaring it; the unresolved package
// the table names; else it is dropped.
//
// Implements: REQ-PURESCRIPT-004, REQ-PURESCRIPT-007, REQ-PURESCRIPT-011
func (r *resolver) module(file, mod string) lang.Target {
	if primModules[mod] {
		return lang.Target{Ecosystem: ecoStd, Package: prim}
	}
	p := r.owner[file]
	candidates := r.modules[mod]
	if p != nil {
		for _, c := range candidates {
			if o := r.owner[c]; o == p && (!r.test[c] || r.test[file]) {
				return lang.Target{Local: c}
			}
		}
		for _, c := range candidates {
			if o := r.owner[c]; o != nil && o.ws == p.ws {
				return lang.Target{Local: c}
			}
		}
		if name := r.installedIndex(p.dir).mods[mod]; name != "" {
			return r.target(p, name)
		}
		declared := func(name string) bool { return r.declares(p, name) }
		name, n := spelled(mod, p, declared)
		if known, k := knownPackage(mod); known != "" && declared(known) && k >= n {
			name = known
		}
		if name != "" {
			t := r.target(p, name)
			if t.Local != "" {
				// A local package: the module's file in it.
				dir := t.Local
				if !r.dirs[dir] {
					dir = path.Dir(dir)
				}
				for _, c := range candidates {
					if under(c, dir) {
						return lang.Target{Local: c}
					}
				}
			}
			return t
		}
	}
	if len(candidates) > 0 {
		return lang.Target{Local: nearest(file, candidates)}
	}
	if known, _ := knownPackage(mod); known != "" {
		if p != nil {
			return r.target(p, known)
		}
		return lang.Target{Ecosystem: ecoPureScript, Package: known, Unresolved: true}
	}
	return lang.Target{}
}

// nearest is the candidate file sharing the longest directory prefix with file.
func nearest(file string, candidates []string) string {
	best, n := candidates[0], -1
	for _, c := range candidates {
		k := 0
		for k < len(c) && k < len(file) && c[k] == file[k] {
			k++
		}
		if k > n {
			best, n = c, k
		}
	}
	return best
}

// declares reports whether a package is listed for the project: a dependency, a
// workspace's extra package, or one its lock records.
func (r *resolver) declares(p *project, name string) bool {
	if _, ok := p.deps[name]; ok {
		return true
	}
	if _, ok := p.ws.extra[name]; ok {
		return true
	}
	if p.ws.lock != nil {
		if _, ok := p.ws.lock.packages[name]; ok {
			return true
		}
	}
	return false
}

// spelled is the listed package whose name, folded, equals a run of the module's
// segments folded that starts at the first segment, or at the second after a
// namespace (Data, Control, Effect, Test, Type): Node.FS is node-fs, Web.HTML
// web-html, Data.Maybe maybe, Effect.Aff aff. The run ending furthest wins; it
// returns the package and where the run ends.
func spelled(mod string, p *project, declared func(string) bool) (string, int) {
	segments := strings.Split(mod, ".")
	names := map[string]string{} // folded -> name
	add := func(name string) {
		if f := fold(name); f != "" && declared(name) {
			if old, ok := names[f]; !ok || name < old {
				names[f] = name
			}
		}
	}
	for n := range p.deps {
		add(n)
	}
	for n := range p.ws.extra {
		add(n)
	}
	if p.ws.lock != nil {
		for n := range p.ws.lock.packages {
			add(n)
		}
	}
	best, end := "", 0
	for start := 0; start <= 1 && start < len(segments); start++ {
		if start == 1 && !namespaces[segments[0]] {
			break
		}
		for k := len(segments); k > start && k > end; k-- {
			if name, ok := names[fold(strings.Join(segments[start:k], ""))]; ok {
				best, end = name, k
				break
			}
		}
	}
	return best, end
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

// target is a package as the project's workspace and manifest describe it.
//
// Implements: REQ-PURESCRIPT-005, REQ-PURESCRIPT-006
func (r *resolver) target(p *project, name string) lang.Target {
	d, declared := p.deps[name]
	if t := r.fromWorkspace(p.ws, name, d.rng); t != (lang.Target{}) {
		return t
	}
	if !declared {
		return lang.Target{Ecosystem: ecoPureScript, Package: name, Unresolved: true}
	}
	if d.origin != "" {
		// bower installing from git: a commit pins, a tag or range does not.
		return lang.Target{Ecosystem: ecoPureScript, Package: gitName(d.origin, ""), Version: d.rng, Origin: d.origin,
			Pinned: lang.Commit(d.rng), Floating: !lang.Commit(d.rng)}
	}
	if d.rng != "" {
		v := strings.TrimPrefix(d.rng, "v")
		if lang.PinnedSemver(v) {
			return lang.Target{Ecosystem: ecoPureScript, Package: name, Version: v, Pinned: true}
		}
		return lang.Target{Ecosystem: ecoPureScript, Package: name, Version: d.rng, Floating: true}
	}
	if p.ws.set != "" {
		// A package set decides the version, but which one is not known offline.
		return lang.Target{Ecosystem: ecoPureScript, Package: name, Version: p.ws.set}
	}
	return lang.Target{Ecosystem: ecoPureScript, Package: name, Floating: true}
}

// fromWorkspace is a package as a workspace decides it: one of the workspace's
// own packages (its manifest), the lock's entry, or an extra package; the empty
// Target when the workspace says nothing about it.
//
// Implements: REQ-PURESCRIPT-006
func (r *resolver) fromWorkspace(ws *workspace, name, requested string) lang.Target {
	if ws == nil {
		return lang.Target{}
	}
	if m, ok := ws.locals[name]; ok {
		return lang.Target{Local: m}
	}
	if ws.lock != nil {
		if l := ws.lock.packages[name]; l != nil {
			switch {
			case l.typ == "git" || l.url != "":
				return lang.Target{Ecosystem: ecoPureScript, Package: gitName(l.url, l.subdir), Version: l.rev,
					Pinned: lang.Commit(l.rev), Origin: l.url}
			case l.typ == "local" || l.path != "":
				return r.localDir(ws.dir, l.path)
			case l.version != "":
				t := lang.Target{Ecosystem: ecoPureScript, Package: name, Version: l.version, Pinned: true}
				if requested != "" && requested != l.version {
					t.Requested = requested
				}
				return t
			}
		}
	}
	if e := ws.extra[name]; e != nil {
		switch {
		case e.path != "":
			return r.localDir(e.dir, e.path)
		case e.git != "":
			ref := e.ref
			if ref == "" {
				ref = e.version // packages.dhall: version is the git ref
			}
			return lang.Target{Ecosystem: ecoPureScript, Package: gitName(e.git, e.subdir), Version: ref,
				Pinned: lang.Commit(ref), Floating: ref == "", Origin: e.git}
		case e.version != "":
			v := strings.TrimPrefix(e.version, "v")
			if lang.PinnedSemver(v) && !strings.HasPrefix(e.version, "v") {
				return lang.Target{Ecosystem: ecoPureScript, Package: name, Version: v, Pinned: true}
			}
			// A packages.dhall override of the version alone: a tag of the
			// package's repository, or a commit.
			return lang.Target{Ecosystem: ecoPureScript, Package: name, Version: e.version, Pinned: lang.Commit(e.version)}
		}
	}
	return lang.Target{}
}

// localDir is a local package's directory (its spago.yaml or spago.dhall when
// it has one), or nothing when it is outside the repository or not there.
func (r *resolver) localDir(base, rel string) lang.Target {
	d := path.Join(base, rel)
	if !inside(d) {
		return lang.Target{}
	}
	for _, m := range []string{"spago.yaml", "spago.dhall"} {
		if f := path.Join(d, m); r.files[f] {
			return lang.Target{Local: f}
		}
	}
	if r.dirs[d] {
		return lang.Target{Local: d}
	}
	return lang.Target{}
}

// installedIndex maps the modules of the packages spago (or bower) installed
// for a project to their packages.
type installedIndex struct {
	once sync.Once
	mods map[string]string // module -> package
}

// maxInstalledFiles bounds how many installed files are listed per directory.
const maxInstalledFiles = 100_000

// installedIndex is what is installed under the nearest directory at or above
// dir that has a .spago/ or bower_components/: spago 0.93 and later keep
// registry packages in .spago/p/<name>-<version>/ and git packages in
// .spago/p/<name>/<ref>/, spago 0.20 in .spago/<name>/<version>/, bower in
// bower_components/purescript-<name>/. A module is a file under the package's
// src/, named by its path (Data/Maybe.purs is Data.Maybe).
//
// Implements: REQ-PURESCRIPT-007
func (r *resolver) installedIndex(dir string) *installedIndex {
	for d := dir; ; d = path.Dir(d) {
		abs := filepath.Join(r.root, filepath.FromSlash(d))
		spago, bower := filepath.Join(abs, ".spago"), filepath.Join(abs, "bower_components")
		if isDir(spago) || isDir(bower) {
			v, _ := r.installed.LoadOrStore(d, &installedIndex{})
			idx := v.(*installedIndex)
			idx.once.Do(func() { idx.mods = readInstalled(spago, bower) })
			return idx
		}
		if d == "." || d == "/" {
			return &installedIndex{}
		}
	}
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func readInstalled(spago, bower string) map[string]string {
	mods := map[string]string{}
	count := 0
	addSrc := func(pkg, src string) {
		filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
			if err != nil || count > maxInstalledFiles {
				return filepath.SkipDir
			}
			if d.IsDir() || filepath.Ext(p) != ".purs" {
				return nil
			}
			count++
			rel, _ := filepath.Rel(src, p)
			mod := strings.ReplaceAll(strings.TrimSuffix(filepath.ToSlash(rel), ".purs"), "/", ".")
			if _, ok := mods[mod]; !ok {
				mods[mod] = pkg
			}
			return nil
		})
	}
	entries, _ := os.ReadDir(filepath.Join(spago, "p"))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(spago, "p", e.Name())
		if isDir(filepath.Join(dir, "src")) {
			if name, _, ok := splitNameVersion(e.Name()); ok {
				addSrc(name, filepath.Join(dir, "src"))
			}
			continue
		}
		refs, _ := os.ReadDir(dir)
		for _, ref := range refs {
			if src := filepath.Join(dir, ref.Name(), "src"); ref.IsDir() && isDir(src) {
				addSrc(e.Name(), src)
				break
			}
		}
	}
	entries, _ = os.ReadDir(spago)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "p" {
			continue
		}
		vers, _ := os.ReadDir(filepath.Join(spago, e.Name()))
		for _, v := range vers {
			if src := filepath.Join(spago, e.Name(), v.Name(), "src"); v.IsDir() && isDir(src) {
				addSrc(e.Name(), src)
				break
			}
		}
	}
	entries, _ = os.ReadDir(bower)
	for _, e := range entries {
		if name, ok := strings.CutPrefix(e.Name(), "purescript-"); ok && e.IsDir() {
			addSrc(name, filepath.Join(bower, e.Name(), "src"))
		}
	}
	return mods
}

// readSpagoInstalled reads the manifests of the packages spago installed into a
// .spago/ directory: spago 0.93's p/<name>-<version>/ (a registry version, which
// pins) and p/<name>/<ref>/ (a git ref, pinned when a commit), spago 0.20's
// <name>/<version>/. A package without a manifest it can read is left out.
//
// Implements: REQ-PURESCRIPT-008
func readSpagoInstalled(spago string) map[string]*installedPackage {
	out := map[string]*installedPackage{}
	add := func(name, version string, pinned bool, dir string) bool {
		if out[name] != nil {
			return true
		}
		deps, ok := manifestDeps(dir)
		if ok {
			out[name] = &installedPackage{version: version, pinned: pinned, deps: deps}
		}
		return ok
	}
	entries, _ := os.ReadDir(filepath.Join(spago, "p"))
	for _, e := range entries {
		dir := filepath.Join(spago, "p", e.Name())
		if !e.IsDir() {
			continue
		}
		if name, version, ok := splitNameVersion(e.Name()); ok && add(name, version, true, dir) {
			continue
		}
		refs, _ := os.ReadDir(dir)
		for _, ref := range refs {
			if ref.IsDir() && add(e.Name(), ref.Name(), lang.Commit(ref.Name()), filepath.Join(dir, ref.Name())) {
				break
			}
		}
	}
	entries, _ = os.ReadDir(spago)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "p" {
			continue
		}
		vers, _ := os.ReadDir(filepath.Join(spago, e.Name()))
		for _, v := range vers {
			if v.IsDir() && add(e.Name(), v.Name(), lang.Commit(v.Name()), filepath.Join(spago, e.Name(), v.Name())) {
				break
			}
		}
	}
	return out
}

// manifestDeps are the dependencies an installed package's manifest lists (not
// its test dependencies): spago.yaml's, else spago.dhall's, else the registry's
// purs.json's. ok is false when it has none of them readable.
func manifestDeps(dir string) (deps []string, ok bool) {
	read := func(name string) []byte {
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || len(src) > lang.MaxParseSize {
			return nil
		}
		return src
	}
	if m := readSpagoYAML(read("spago.yaml")); m != nil && m.isPackage {
		for _, d := range m.deps {
			if !d.test {
				deps = append(deps, d.name)
			}
		}
		return deps, true
	}
	if src := read("spago.dhall"); src != nil {
		if v := dhall.Eval(src, ".", nil); dhallConfig(v) {
			for _, t := range v.Field("dependencies").Texts() {
				deps = append(deps, t.Text)
			}
			return deps, true
		}
	}
	if m := yamlGet(yamlDoc(read("purs.json")), "dependencies"); m != nil && m.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(m.Content); i += 2 {
			deps = append(deps, m.Content[i].Value)
		}
		return deps, true
	}
	return nil, false
}

// splitNameVersion splits spago's name-version directory (web-html-4.1.0).
func splitNameVersion(s string) (name, version string, ok bool) {
	for i := len(s) - 1; i > 0; i-- {
		if s[i] == '-' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

// Dependencies implements lang.Transitive from what the repository records: a
// spago.lock entry's dependencies, pinned by that lock, else the dependencies an
// extra package (spago.yaml extraPackages, a packages.dhall override) lists, else
// those the manifest of the package spago installed lists.
//
// Implements: REQ-PURESCRIPT-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	deps, _ := r.dependencies(t)
	return deps
}

// Installed reports whether a package's dependencies come from what spago
// installed.
func (r *resolver) Installed(t lang.Target) bool {
	_, installed := r.dependencies(t)
	return installed
}

// dependencies answers for Dependencies and says whether the answer came from
// what spago installed.
func (r *resolver) dependencies(t lang.Target) (deps []lang.Target, installed bool) {
	if t.Ecosystem != ecoPureScript || t.Package == "" {
		return nil, false
	}
	for _, ws := range r.workspaces() {
		if ws.lock != nil {
			for _, name := range sortedKeys(ws.lock.packages) {
				l := ws.lock.packages[name]
				if name == t.Package && l.url == "" || l.url != "" && gitName(l.url, l.subdir) == t.Package {
					return r.depTargets(ws, l.dependencies), false
				}
			}
		}
		for _, name := range sortedKeys(ws.extra) {
			e := ws.extra[name]
			if e.hasDeps && (name == t.Package && e.git == "" || e.git != "" && gitName(e.git, e.subdir) == t.Package) {
				return r.depTargets(ws, e.dependencies), false
			}
		}
	}
	for _, ws := range r.workspaces() {
		if p := ws.installed[t.Package]; p != nil {
			return r.depTargets(ws, p.deps), true
		}
	}
	return nil, false
}

func (r *resolver) depTargets(ws *workspace, names []string) []lang.Target {
	var out []lang.Target
	for _, n := range names {
		if t := r.fromWorkspace(ws, n, ""); t.Package != "" {
			out = append(out, t)
		} else if p := ws.installed[n]; p != nil && t.Local == "" {
			out = append(out, lang.Target{Ecosystem: ecoPureScript, Package: n, Version: p.version, Pinned: p.pinned})
		} else if t.Local == "" {
			out = append(out, lang.Target{Ecosystem: ecoPureScript, Package: n, Version: ws.set, Floating: ws.set == ""})
		}
	}
	return out
}

// workspaces are every workspace, spago.yaml ones first, in a stable order.
func (r *resolver) workspaces() []*workspace {
	var out []*workspace
	for _, d := range sortedKeys(r.wsByDir) {
		out = append(out, r.wsByDir[d])
	}
	seen := map[*workspace]bool{}
	for _, p := range r.projects {
		if p.ws != nil && !seen[p.ws] && r.wsByDir[p.ws.dir] != p.ws {
			seen[p.ws] = true
			out = append(out, p.ws)
		}
	}
	return out
}
