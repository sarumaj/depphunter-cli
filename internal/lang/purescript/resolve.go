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
	directory string
	set       string // the package set's name, shown as the version of what it provides
	extra     map[string]*extraPackage
	lock      *spagoLock
	locals    map[string]string // the workspace's own packages -> their manifest
	// installed is what spago installed into the workspace's .spago/, by name.
	installed map[string]*installedPackage
}

// installedPackage is a package spago installed: its version (or git reference) and
// the dependencies its own manifest lists.
type installedPackage struct {
	version      string
	pinned       bool
	dependencies []string
}

// project is one PureScript package of the repository: a spago.yaml package, a
// spago.dhall configuration or a bower.json.
type project struct {
	file, directory string
	kind            string // classYAML, classDhall or classBower
	name            string
	globs           []string // source globs, relative to the repository
	testGlobs       []string
	dependencies    map[string]dependency
	workspace       *workspace
}

type resolver struct {
	root          string
	files         map[string]bool
	directories   map[string]bool
	projects      []*project
	byFile        map[string]*project // by manifest
	wsByDirectory map[string]*workspace
	owner         map[string]*project // a module file's project
	test          map[string]bool     // a module file read by a project's test globs
	modules       map[string][]string // module name -> files declaring it, sorted
	// evaluated memoizes Dhall files; nil while one is being evaluated (an
	// import cycle).
	evaluated map[string]*dhall.Value
	setWS     map[*dhall.Value]*workspace
	setFile   map[string]*workspace // a packages.dhall's workspace
	installed sync.Map              // dir -> *installedIndex
}

// Implements: REQ-PURESCRIPT-004, REQ-PURESCRIPT-005, REQ-PURESCRIPT-007
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, directories: map[string]bool{}, byFile: map[string]*project{},
		wsByDirectory: map[string]*workspace{}, owner: map[string]*project{}, test: map[string]bool{},
		modules: map[string][]string{}, evaluated: map[string]*dhall.Value{}, setWS: map[*dhall.Value]*workspace{}, setFile: map[string]*workspace{}}
	var sources []*scan.File
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.directories[d]; d = path.Dir(d) {
			r.directories[d] = true
		}
		if path.Ext(f.Path) == ".purs" && !installedPath(f.Path) && !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize {
			sources = append(sources, f)
		}
	}
	r.readYAML(all)
	r.readDhall(all)
	r.readBower(all)
	if root != "" {
		for _, workspace := range r.workspaces() {
			workspace.installed = readSpagoInstalled(filepath.Join(root, filepath.FromSlash(workspace.directory), ".spago"))
		}
	}
	sort.Slice(r.projects, func(i, j int) bool {
		a, b := r.projects[i], r.projects[j]
		if a.directory != b.directory {
			return a.directory < b.directory
		}
		if a.kind != b.kind {
			return kindRank[a.kind] < kindRank[b.kind]
		}
		return a.file < b.file
	})
	for _, f := range sources {
		if source, err := os.ReadFile(f.AbsolutePath); err == nil {
			if m := moduleName(source); m != "" {
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
	source, _ := os.ReadFile(f.AbsolutePath)
	return source
}

// readYAML reads every spago.yaml: workspaces (with their spago.lock, read from
// disk) and packages, each package in the nearest workspace above it.
func (r *resolver) readYAML(all []*scan.File) {
	type found struct {
		f *scan.File
		m *spagoYAML
	}
	var packages []found
	for _, f := range all {
		if path.Base(f.Path) != "spago.yaml" || installedPath(f.Path) {
			continue
		}
		m := readSpagoYAML(r.read(f))
		if m == nil {
			continue
		}
		directory := path.Dir(f.Path)
		if m.workspace {
			activeWorkspace := &workspace{directory: directory, set: m.set, extra: map[string]*extraPackage{}, locals: map[string]string{}}
			for _, e := range m.extra {
				e.directory = directory
				activeWorkspace.extra[e.name] = e
			}
			if source, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(directory), "spago.lock")); err == nil {
				activeWorkspace.lock = readLock(source)
				if activeWorkspace.lock != nil && activeWorkspace.set == "" {
					activeWorkspace.set = activeWorkspace.lock.set
				}
			}
			r.wsByDirectory[directory] = activeWorkspace
		}
		if m.isPackage {
			packages = append(packages, found{f, m})
		}
	}
	for _, p := range packages {
		directory := path.Dir(p.f.Path)
		activeWorkspace := r.nearestWorkspace(directory)
		if activeWorkspace == nil {
			activeWorkspace = &workspace{directory: directory, extra: map[string]*extraPackage{}, locals: map[string]string{}}
		}
		newProject := &project{file: p.f.Path, directory: directory, kind: classYAML, name: p.m.name, dependencies: map[string]dependency{}, workspace: activeWorkspace,
			globs: []string{path.Join(directory, "src/**/*.purs")}, testGlobs: []string{path.Join(directory, "test/**/*.purs")}}
		for _, d := range p.m.dependencies {
			if _, ok := newProject.dependencies[d.name]; !ok || !d.test {
				newProject.dependencies[d.name] = d
			}
		}
		if newProject.name != "" {
			activeWorkspace.locals[newProject.name] = p.f.Path
		}
		r.projects = append(r.projects, newProject)
		r.byFile[p.f.Path] = newProject
	}
	// A lock's local packages are the workspace's too (their spago.yaml may be
	// missing from the scan).
	for _, workspace := range r.wsByDirectory {
		if workspace.lock == nil {
			continue
		}
		for name, p := range workspace.lock.locals {
			if _, ok := workspace.locals[name]; !ok {
				if d := path.Join(workspace.directory, p); lang.Inside(d) && (r.files[path.Join(d, "spago.yaml")]) {
					workspace.locals[name] = path.Join(d, "spago.yaml")
				}
			}
		}
	}
}

// nearestWorkspace is the spago.yaml workspace in directory or the nearest above it.
func (r *resolver) nearestWorkspace(directory string) *workspace {
	for d := directory; ; d = path.Dir(d) {
		if workspace := r.wsByDirectory[d]; workspace != nil {
			return workspace
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
	for _, p := range lang.SortedKeys(byPath) {
		if path.Base(p) == "packages.dhall" {
			if v := r.loadDhall(p); v != nil {
				r.setFile[p] = r.dhallWorkspace(v, path.Dir(p))
			}
			continue
		}
		source := r.read(byPath[p])
		if !strings.Contains(string(source), "dependencies") && !strings.Contains(string(source), "sources") {
			continue
		}
		v := r.loadDhall(p)
		if v == nil || !dhallConfig(v) {
			continue
		}
		directory := path.Dir(p)
		newProject := &project{file: p, directory: directory, kind: classDhall, dependencies: map[string]dependency{}}
		if n := v.Field("name"); n.Kind == dhall.KindText {
			newProject.name = n.Text
		}
		for _, t := range v.Field("dependencies").Texts() {
			if _, ok := newProject.dependencies[t.Text]; !ok {
				newProject.dependencies[t.Text] = dependency{name: t.Text, line: t.Line}
			}
		}
		for _, t := range v.Field("sources").Texts() {
			newProject.globs = append(newProject.globs, r.globBase(directory, t.Text))
		}
		newProject.workspace = r.dhallWorkspace(v.Field("packages"), directory)
		r.projects = append(r.projects, newProject)
		r.byFile[p] = newProject
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
	source, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(file)))
	if err != nil || len(source) > lang.MaxParseSize {
		return nil
	}
	r.evaluated[file] = nil
	v := dhall.Eval(source, path.Dir(file), r.loadDhall)
	r.evaluated[file] = v
	return v
}

// dhallWorkspace is the workspace of a package set value: its name and the
// packages it adds or overrides. One set evaluated once is one workspace.
func (r *resolver) dhallWorkspace(set *dhall.Value, directory string) *workspace {
	if workspace := r.setWS[set]; workspace != nil && set != dhall.Unknown {
		return workspace
	}
	activeWorkspace := &workspace{directory: directory, set: setName(set), extra: map[string]*extraPackage{}, locals: map[string]string{}}
	setDirectory := directory
	for f, v := range r.evaluated {
		if v == set && path.Base(f) == "packages.dhall" {
			setDirectory = path.Dir(f)
		}
	}
	for _, e := range dhallExtras(set, setDirectory) {
		activeWorkspace.extra[e.name] = e
	}
	if set != dhall.Unknown {
		r.setWS[set] = activeWorkspace
	}
	return activeWorkspace
}

// globBase makes a spago.dhall source glob relative to the repository. Legacy
// spago read globs relative to where it ran, which is the configuration's
// directory for most projects and the repository root for configurations that
// extend another (examples/basic/spago.dhall listing "examples/basic/**"): the
// first of the configuration's directory and its ancestors under which the
// glob's literal part exists wins.
func (r *resolver) globBase(directory, glob string) string {
	literal := glob
	if i := strings.IndexAny(glob, "*?["); i >= 0 {
		literal = path.Dir(glob[:i+1])
	}
	for d := directory; ; d = path.Dir(d) {
		if p := path.Join(d, literal); lang.Inside(p) && (r.directories[p] || r.files[p] || p == ".") {
			return path.Join(d, glob)
		}
		if d == "." || d == "/" {
			break
		}
	}
	return path.Join(directory, glob)
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
		directory := path.Dir(f.Path)
		newProject := &project{file: f.Path, directory: directory, kind: classBower, name: m.name, dependencies: map[string]dependency{},
			globs: []string{path.Join(directory, "src/**/*.purs")}, testGlobs: []string{path.Join(directory, "test/**/*.purs")},
			workspace: &workspace{directory: directory, extra: map[string]*extraPackage{}, locals: map[string]string{}}}
		for _, d := range m.dependencies {
			if _, ok := newProject.dependencies[d.name]; !ok || !d.test {
				newProject.dependencies[d.name] = d
			}
		}
		r.projects = append(r.projects, newProject)
		r.byFile[f.Path] = newProject
	}
}

// own finds the project a module file belongs to: of the projects whose source
// globs match it, the one whose directory holds it (the deepest), else the first;
// a file no glob matches belongs to the nearest project above it.
func (r *resolver) own(file string) {
	var best *project
	bestTest, bestAncestor := false, false
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
		ancestor := lang.Within(file, p.directory)
		if best == nil || ancestor && !bestAncestor || ancestor && bestAncestor && len(p.directory) > len(best.directory) {
			best, bestTest, bestAncestor = p, t, ancestor
		}
	}
	if best == nil {
		for d := path.Dir(file); best == nil; d = path.Dir(d) {
			for _, p := range r.projects {
				if p.directory == d {
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

// moduleName reads the name a module file declares in its header, or "".
func moduleName(source []byte) string {
	tokens := lex(source[:min(len(source), 64<<10)])
	if len(tokens) >= 2 && tokens[0].kind == tLower && tokens[0].text == "module" && tokens[1].kind == tUpper {
		return tokens[1].text
	}
	return ""
}

// Implements: REQ-PURESCRIPT-004, REQ-PURESCRIPT-005, REQ-PURESCRIPT-006
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindModule:
		return r.module(file, rawImport.Module)
	case kindFFI:
		// The JavaScript companion of a module with foreign imports.
		if js := strings.TrimSuffix(file, ".purs") + ".js"; r.files[js] {
			return lang.Target{Local: js}
		}
	case kindDependency:
		if p := r.byFile[file]; p != nil {
			return r.target(p, rawImport.Module)
		}
	case kindExtra:
		if p := r.byFile[file]; p != nil {
			return r.fromWorkspace(p.workspace, rawImport.Module, "")
		}
		if workspace := r.wsByDirectory[path.Dir(file)]; workspace != nil {
			return r.fromWorkspace(workspace, rawImport.Module, "")
		}
		if workspace := r.setFile[file]; workspace != nil {
			return r.fromWorkspace(workspace, rawImport.Module, "")
		}
	case kindLock:
		if workspace := r.wsByDirectory[path.Dir(file)]; workspace != nil {
			return r.fromWorkspace(workspace, rawImport.Module, "")
		}
		if source, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(file))); err == nil {
			if l := readLock(source); l != nil {
				return r.fromWorkspace(&workspace{directory: path.Dir(file), lock: l}, rawImport.Module, "")
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
func (r *resolver) module(file, module string) lang.Target {
	if primModules[module] {
		return lang.Target{Ecosystem: ecosystemStd, Package: prim}
	}
	p := r.owner[file]
	candidates := r.modules[module]
	if p != nil {
		for _, c := range candidates {
			if o := r.owner[c]; o == p && (!r.test[c] || r.test[file]) {
				return lang.Target{Local: c}
			}
		}
		for _, c := range candidates {
			if o := r.owner[c]; o != nil && o.workspace == p.workspace {
				return lang.Target{Local: c}
			}
		}
		if name := r.installedIndex(p.directory).modules[module]; name != "" {
			return r.target(p, name)
		}
		declared := func(name string) bool { return r.declares(p, name) }
		name, n := spelled(module, p, declared)
		if known, k := knownPackage(module); known != "" && declared(known) && k >= n {
			name = known
		}
		if name != "" {
			t := r.target(p, name)
			if t.Local != "" {
				// A local package: the module's file in it.
				directory := t.Local
				if !r.directories[directory] {
					directory = path.Dir(directory)
				}
				for _, c := range candidates {
					if lang.Within(c, directory) {
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
	if known, _ := knownPackage(module); known != "" {
		if p != nil {
			return r.target(p, known)
		}
		return lang.Target{Ecosystem: ecosystemPureScript, Package: known, Unresolved: true}
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
	if _, ok := p.dependencies[name]; ok {
		return true
	}
	if _, ok := p.workspace.extra[name]; ok {
		return true
	}
	if p.workspace.lock != nil {
		if _, ok := p.workspace.lock.packages[name]; ok {
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
func spelled(module string, p *project, declared func(string) bool) (string, int) {
	segments := strings.Split(module, ".")
	names := map[string]string{} // folded -> name
	add := func(name string) {
		if f := fold(name); f != "" && declared(name) {
			if old, ok := names[f]; !ok || name < old {
				names[f] = name
			}
		}
	}
	for n := range p.dependencies {
		add(n)
	}
	for n := range p.workspace.extra {
		add(n)
	}
	if p.workspace.lock != nil {
		for n := range p.workspace.lock.packages {
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
	d, declared := p.dependencies[name]
	if t := r.fromWorkspace(p.workspace, name, d.versionRange); t != (lang.Target{}) {
		return t
	}
	if !declared {
		return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Unresolved: true}
	}
	if d.origin != "" {
		// bower installing from git: a commit pins, a tag or range does not.
		return lang.Target{Ecosystem: ecosystemPureScript, Package: gitName(d.origin, ""), Version: d.versionRange, Origin: d.origin,
			Pinned: lang.Commit(d.versionRange), Floating: !lang.Commit(d.versionRange)}
	}
	if d.versionRange != "" {
		v := strings.TrimPrefix(d.versionRange, "v")
		if lang.PinnedSemver(v) {
			return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Version: v, Pinned: true}
		}
		return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Version: d.versionRange, Floating: true}
	}
	if p.workspace.set != "" {
		// A package set decides the version, but which one is not known offline.
		return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Version: p.workspace.set}
	}
	return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Floating: true}
}

// fromWorkspace is a package as a workspace decides it: one of the workspace's
// own packages (its manifest), the lock's entry, or an extra package; the empty
// Target when the workspace says nothing about it.
//
// Implements: REQ-PURESCRIPT-006
func (r *resolver) fromWorkspace(activeWorkspace *workspace, name, requested string) lang.Target {
	if activeWorkspace == nil {
		return lang.Target{}
	}
	if m, ok := activeWorkspace.locals[name]; ok {
		return lang.Target{Local: m}
	}
	if activeWorkspace.lock != nil {
		if l := activeWorkspace.lock.packages[name]; l != nil {
			switch {
			case l.typeName == "git" || l.url != "":
				return lang.Target{Ecosystem: ecosystemPureScript, Package: gitName(l.url, l.subdirectory), Version: l.rev,
					Pinned: lang.Commit(l.rev), Origin: l.url}
			case l.typeName == "local" || l.path != "":
				return r.localDirectory(activeWorkspace.directory, l.path)
			case l.version != "":
				t := lang.Target{Ecosystem: ecosystemPureScript, Package: name, Version: l.version, Pinned: true}
				if requested != "" && requested != l.version {
					t.Requested = requested
				}
				return t
			}
		}
	}
	if e := activeWorkspace.extra[name]; e != nil {
		switch {
		case e.path != "":
			return r.localDirectory(e.directory, e.path)
		case e.git != "":
			reference := e.reference
			if reference == "" {
				reference = e.version // packages.dhall: version is the git reference
			}
			return lang.Target{Ecosystem: ecosystemPureScript, Package: gitName(e.git, e.subdirectory), Version: reference,
				Pinned: lang.Commit(reference), Floating: reference == "", Origin: e.git}
		case e.version != "":
			v := strings.TrimPrefix(e.version, "v")
			if lang.PinnedSemver(v) && !strings.HasPrefix(e.version, "v") {
				return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Version: v, Pinned: true}
			}
			// A packages.dhall override of the version alone: a tag of the
			// package's repository, or a commit.
			return lang.Target{Ecosystem: ecosystemPureScript, Package: name, Version: e.version, Pinned: lang.Commit(e.version)}
		}
	}
	return lang.Target{}
}

// localDirectory is a local package's directory (its spago.yaml or spago.dhall when
// it has one), or nothing when it is outside the repository or not there.
func (r *resolver) localDirectory(base, relative string) lang.Target {
	d := path.Join(base, relative)
	if !lang.Inside(d) {
		return lang.Target{}
	}
	for _, m := range []string{"spago.yaml", "spago.dhall"} {
		if f := path.Join(d, m); r.files[f] {
			return lang.Target{Local: f}
		}
	}
	if r.directories[d] {
		return lang.Target{Local: d}
	}
	return lang.Target{}
}

// installedIndex maps the modules of the packages spago (or bower) installed
// for a project to their packages.
type installedIndex struct {
	once    sync.Once
	modules map[string]string // module -> package
}

// maxInstalledFiles bounds how many installed files are listed per directory.
const maxInstalledFiles = 100_000

// installedIndex is what is installed under the nearest directory at or above
// directory that has a .spago/ or bower_components/: spago 0.93 and later keep
// registry packages in .spago/p/<name>-<version>/ and git packages in
// .spago/p/<name>/<ref>/, spago 0.20 in .spago/<name>/<version>/, bower in
// bower_components/purescript-<name>/. A module is a file under the package's
// src/, named by its path (Data/Maybe.purs is Data.Maybe).
//
// Implements: REQ-PURESCRIPT-007
func (r *resolver) installedIndex(directory string) *installedIndex {
	for d := directory; ; d = path.Dir(d) {
		absolute := filepath.Join(r.root, filepath.FromSlash(d))
		spago, bower := filepath.Join(absolute, ".spago"), filepath.Join(absolute, "bower_components")
		if lang.IsDirectory(spago) || lang.IsDirectory(bower) {
			v, _ := r.installed.LoadOrStore(d, &installedIndex{})
			index := v.(*installedIndex)
			index.once.Do(func() { index.modules = readInstalled(spago, bower) })
			return index
		}
		if d == "." || d == "/" {
			return &installedIndex{}
		}
	}
}

func readInstalled(spago, bower string) map[string]string {
	modules := map[string]string{}
	count := 0
	addSource := func(packageName, source string) {
		filepath.WalkDir(source, func(p string, d os.DirEntry, err error) error {
			if err != nil || count > maxInstalledFiles {
				return filepath.SkipDir
			}
			if d.IsDir() || filepath.Ext(p) != ".purs" {
				return nil
			}
			count++
			relative, _ := filepath.Rel(source, p)
			module := strings.ReplaceAll(strings.TrimSuffix(filepath.ToSlash(relative), ".purs"), "/", ".")
			if _, ok := modules[module]; !ok {
				modules[module] = packageName
			}
			return nil
		})
	}
	entries, _ := os.ReadDir(filepath.Join(spago, "p"))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		directory := filepath.Join(spago, "p", e.Name())
		if lang.IsDirectory(filepath.Join(directory, "src")) {
			if name, _, ok := splitNameVersion(e.Name()); ok {
				addSource(name, filepath.Join(directory, "src"))
			}
			continue
		}
		references, _ := os.ReadDir(directory)
		for _, reference := range references {
			if source := filepath.Join(directory, reference.Name(), "src"); reference.IsDir() && lang.IsDirectory(source) {
				addSource(e.Name(), source)
				break
			}
		}
	}
	entries, _ = os.ReadDir(spago)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "p" {
			continue
		}
		versions, _ := os.ReadDir(filepath.Join(spago, e.Name()))
		for _, v := range versions {
			if source := filepath.Join(spago, e.Name(), v.Name(), "src"); v.IsDir() && lang.IsDirectory(source) {
				addSource(e.Name(), source)
				break
			}
		}
	}
	entries, _ = os.ReadDir(bower)
	for _, e := range entries {
		if name, ok := strings.CutPrefix(e.Name(), "purescript-"); ok && e.IsDir() {
			addSource(name, filepath.Join(bower, e.Name(), "src"))
		}
	}
	return modules
}

// readSpagoInstalled reads the manifests of the packages spago installed into a
// .spago/ directory: spago 0.93's p/<name>-<version>/ (a registry version, which
// pins) and p/<name>/<ref>/ (a git ref, pinned when a commit), spago 0.20's
// <name>/<version>/. A package without a manifest it can read is left out.
//
// Implements: REQ-PURESCRIPT-008
func readSpagoInstalled(spago string) map[string]*installedPackage {
	out := map[string]*installedPackage{}
	add := func(name, version string, pinned bool, directory string) bool {
		if out[name] != nil {
			return true
		}
		dependencies, ok := manifestDependencies(directory)
		if ok {
			out[name] = &installedPackage{version: version, pinned: pinned, dependencies: dependencies}
		}
		return ok
	}
	entries, _ := os.ReadDir(filepath.Join(spago, "p"))
	for _, e := range entries {
		directory := filepath.Join(spago, "p", e.Name())
		if !e.IsDir() {
			continue
		}
		if name, version, ok := splitNameVersion(e.Name()); ok && add(name, version, true, directory) {
			continue
		}
		references, _ := os.ReadDir(directory)
		for _, reference := range references {
			if reference.IsDir() && add(e.Name(), reference.Name(), lang.Commit(reference.Name()), filepath.Join(directory, reference.Name())) {
				break
			}
		}
	}
	entries, _ = os.ReadDir(spago)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "p" {
			continue
		}
		versions, _ := os.ReadDir(filepath.Join(spago, e.Name()))
		for _, v := range versions {
			if v.IsDir() && add(e.Name(), v.Name(), lang.Commit(v.Name()), filepath.Join(spago, e.Name(), v.Name())) {
				break
			}
		}
	}
	return out
}

// manifestDependencies are the dependencies an installed package's manifest lists (not
// its test dependencies): spago.yaml's, else spago.dhall's, else the registry's
// purs.json's. ok is false when it has none of them readable.
func manifestDependencies(directory string) (dependencies []string, ok bool) {
	read := func(name string) []byte {
		source, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || len(source) > lang.MaxParseSize {
			return nil
		}
		return source
	}
	if m := readSpagoYAML(read("spago.yaml")); m != nil && m.isPackage {
		for _, d := range m.dependencies {
			if !d.test {
				dependencies = append(dependencies, d.name)
			}
		}
		return dependencies, true
	}
	if source := read("spago.dhall"); source != nil {
		if v := dhall.Eval(source, ".", nil); dhallConfig(v) {
			for _, t := range v.Field("dependencies").Texts() {
				dependencies = append(dependencies, t.Text)
			}
			return dependencies, true
		}
	}
	if m := yamlGet(yamlDoc(read("purs.json")), "dependencies"); m != nil && m.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(m.Content); i += 2 {
			dependencies = append(dependencies, m.Content[i].Value)
		}
		return dependencies, true
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
	dependencies, _ := r.dependencies(t)
	return dependencies
}

// Installed reports whether a package's dependencies come from what spago
// installed.
func (r *resolver) Installed(t lang.Target) bool {
	_, installed := r.dependencies(t)
	return installed
}

// dependencies answers for Dependencies and says whether the answer came from
// what spago installed.
func (r *resolver) dependencies(t lang.Target) (dependencies []lang.Target, installed bool) {
	if t.Ecosystem != ecosystemPureScript || t.Package == "" {
		return nil, false
	}
	for _, workspace := range r.workspaces() {
		if workspace.lock != nil {
			for _, name := range lang.SortedKeys(workspace.lock.packages) {
				l := workspace.lock.packages[name]
				if name == t.Package && l.url == "" || l.url != "" && gitName(l.url, l.subdirectory) == t.Package {
					return r.dependencyTargets(workspace, l.dependencies), false
				}
			}
		}
		for _, name := range lang.SortedKeys(workspace.extra) {
			e := workspace.extra[name]
			if e.hasDependencies && (name == t.Package && e.git == "" || e.git != "" && gitName(e.git, e.subdirectory) == t.Package) {
				return r.dependencyTargets(workspace, e.dependencies), false
			}
		}
	}
	for _, workspace := range r.workspaces() {
		if p := workspace.installed[t.Package]; p != nil {
			return r.dependencyTargets(workspace, p.dependencies), true
		}
	}
	return nil, false
}

func (r *resolver) dependencyTargets(activeWorkspace *workspace, names []string) []lang.Target {
	var out []lang.Target
	for _, n := range names {
		if t := r.fromWorkspace(activeWorkspace, n, ""); t.Package != "" {
			out = append(out, t)
		} else if p := activeWorkspace.installed[n]; p != nil && t.Local == "" {
			out = append(out, lang.Target{Ecosystem: ecosystemPureScript, Package: n, Version: p.version, Pinned: p.pinned})
		} else if t.Local == "" {
			out = append(out, lang.Target{Ecosystem: ecosystemPureScript, Package: n, Version: activeWorkspace.set, Floating: activeWorkspace.set == ""})
		}
	}
	return out
}

// workspaces are every workspace, spago.yaml ones first, in a stable order.
func (r *resolver) workspaces() []*workspace {
	var out []*workspace
	for _, d := range lang.SortedKeys(r.wsByDirectory) {
		out = append(out, r.wsByDirectory[d])
	}
	seen := map[*workspace]bool{}
	for _, p := range r.projects {
		if p.workspace != nil && !seen[p.workspace] && r.wsByDirectory[p.workspace.directory] != p.workspace {
			seen[p.workspace] = true
			out = append(out, p.workspace)
		}
	}
	return out
}
