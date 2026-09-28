package ocaml

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/opam"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// component is a dune library, executable or test - or, for sources no dune stanza
// builds, their directory. A module name means the component's own module first.
type component struct {
	dir     string
	dune    string            // the dune file ("" for a directory without one)
	modules map[string]string // module name (Sub.Module under qualified include_subdirs) -> file
	name    string            // a library's name
	main    string            // a library's main module: its name capitalized
	wrapped bool
	// qualified is (include_subdirs qualified): a subdirectory's modules are
	// Sub.Module outside it.
	qualified bool
	libs      []string // the libraries it uses (libraries and pps)
	opens     []string // modules its flags open (-open M)
}

// manifests is what one directory's dune-project, opam files and locks say.
type manifests struct {
	dir    string
	own    map[string]string // package described here -> its file
	deps   map[string]opam.Dep
	pins   map[string]opam.Pin
	locked map[string]string // package -> version, from *.opam.locked, opam.locked or dune.lock
}

type lockPkg struct {
	version string
	deps    []string
}

type resolver struct {
	compOf    map[string]*component // source file -> component
	libs      map[string]*component // local library by name and public name
	libByMain map[string]*component // local library by main module
	files     map[string][]string   // module name -> every file of that name
	sets      []*manifests          // shallowest first
	setAt     map[string]*manifests
	own       map[string]string // every package the repository describes -> its file
	lock      map[string]lockPkg
	// exports are the modules a file opened somewhere declares at its top level,
	// and the modules it includes: what `open Import` brings into scope.
	exports map[string]*exports
	// flat are the opam lock files that pinned something: they record no edges.
	flat []string
	lang.NoteList
}

// The notes a resolver keeps reach --explain only through lang.Noter.
var _ lang.Noter = (*resolver)(nil)

type exports struct {
	modules  map[string]bool
	includes []string
}

// moduleName is the module a source file defines: its base name capitalized, up to
// the first dot - a t.cppo.ml is what a rule preprocesses into t.ml.
func moduleName(p string) string {
	base, _, _ := strings.Cut(path.Base(p), ".")
	return capitalize(base)
}

// extRank orders the files of one module: the implementation (or what generates
// it) before the interface.
var extRank = map[string]int{".ml": 0, ".mly": 1, ".mll": 2, ".mli": 3}

// rank orders the files of one module; a file a rule preprocesses (t.cppo.ml) comes
// after the module's own.
func rank(p string) int {
	r := extRank[path.Ext(p)]
	if strings.Count(path.Base(p), ".") > 1 {
		r += 4
	}
	return r
}

func readable(f *scan.File) bool {
	return !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize
}

// Implements: REQ-OCAML-004, REQ-OCAML-006, REQ-OCAML-007, REQ-OCAML-009
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{
		compOf: map[string]*component{}, libs: map[string]*component{}, libByMain: map[string]*component{},
		files: map[string][]string{}, setAt: map[string]*manifests{}, own: map[string]string{},
		lock: map[string]lockPkg{}, exports: map[string]*exports{},
	}
	dirModules := map[string]map[string]string{} // dir -> module -> file
	dunes := map[string]*duneFile{}
	duneAt := map[string]string{}
	for _, f := range all {
		if ignored(f.Path) || !readable(f) {
			continue
		}
		switch fileClass(f.Path) {
		case classSource:
			dir, m := path.Dir(f.Path), moduleName(f.Path)
			if dirModules[dir] == nil {
				dirModules[dir] = map[string]string{}
			}
			if old, ok := dirModules[dir][m]; !ok || rank(f.Path) < rank(old) {
				dirModules[dir][m] = f.Path
			}
		case classDune:
			if src, err := os.ReadFile(f.Abs); err == nil {
				dunes[path.Dir(f.Path)] = readDune(src)
				duneAt[path.Dir(f.Path)] = f.Path
			}
		case classProject, classOpam, classLock:
			r.readManifest(f)
		}
	}
	for _, mods := range dirModules {
		for m, p := range mods {
			r.files[m] = append(r.files[m], p)
		}
	}
	for _, ps := range r.files {
		sort.Strings(ps)
	}
	r.components(dirModules, dunes, duneAt)
	// A module's other files (its interface, a file a rule preprocesses) are in
	// its component.
	for _, f := range all {
		if _, ok := r.compOf[f.Path]; ok || fileClass(f.Path) != classSource {
			continue
		}
		if p, ok := dirModules[path.Dir(f.Path)][moduleName(f.Path)]; ok {
			if c := r.compOf[p]; c != nil {
				r.compOf[f.Path] = c
			}
		}
	}
	r.readExports(all)
	for _, s := range r.sets {
		r.readDuneLock(root, s)
	}
	// Implements: REQ-OCAML-009, REQ-TRC-017
	for _, f := range r.flat {
		if len(r.lock) == 0 {
			r.Note(f, trace.NoteFlat, "opam's lock pins versions but records no edges, and no dune.lock "+
				"directory is on disk: offline, --resolve-depth adds nothing past the packages it pins "+
				"(--online asks opam-repository)")
		}
	}
	sort.Slice(r.sets, func(i, j int) bool {
		di, dj := depth(r.sets[i].dir), depth(r.sets[j].dir)
		if di != dj {
			return di < dj
		}
		return r.sets[i].dir < r.sets[j].dir
	})
	return r
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

func (r *resolver) set(dir string) *manifests {
	s := r.setAt[dir]
	if s == nil {
		s = &manifests{dir: dir, own: map[string]string{}, deps: map[string]opam.Dep{}, pins: map[string]opam.Pin{}, locked: map[string]string{}}
		r.setAt[dir] = s
		r.sets = append(r.sets, s)
	}
	return s
}

func (r *resolver) readManifest(f *scan.File) {
	src, err := os.ReadFile(f.Abs)
	if err != nil {
		return
	}
	s := r.set(path.Dir(f.Path))
	addDep := func(d opam.Dep) {
		if _, ok := s.deps[d.Name]; !ok && d.Name != "" {
			s.deps[d.Name] = d
		}
	}
	addPin := func(p opam.Pin) {
		if _, ok := s.pins[p.Name]; !ok && p.Name != "" {
			s.pins[p.Name] = p
		}
	}
	switch fileClass(f.Path) {
	case classProject:
		p := readDuneProject(src)
		for _, pkg := range p.packages {
			if pkg.name != "" {
				s.own[pkg.name] = f.Path
				if _, ok := r.own[pkg.name]; !ok {
					r.own[pkg.name] = f.Path
				}
			}
			for _, d := range pkg.depends {
				addDep(d)
			}
		}
		for _, p := range p.pins {
			addPin(p)
		}
	case classOpam:
		o := opam.Read(src)
		name := opamPackageName(f.Path, o)
		s.own[name] = f.Path
		r.own[name] = f.Path // an opam file is the package's own description
		for _, d := range append(o.Depends, o.Depopts...) {
			addDep(d)
		}
		for _, p := range o.Pins {
			addPin(p)
		}
	case classLock:
		o := opam.Read(src)
		pinned := false
		for _, d := range o.Depends {
			if d.Exact != "" {
				s.locked[d.Name] = d.Exact
				pinned = true
			}
		}
		if pinned {
			r.flat = append(r.flat, f.Path)
		}
		for _, p := range o.Pins {
			addPin(p)
		}
	}
}

// readDuneLock reads dune package management's lock directory beside a
// dune-project: one <package>.pkg per locked package with its version and
// dependencies. It is read from disk, as lock directories may be ignored by git.
func (r *resolver) readDuneLock(root string, s *manifests) {
	dir := filepath.Join(root, filepath.FromSlash(s.dir), "dune.lock")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	names := map[string]bool{}
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e.Name(), ".pkg"); ok && !e.IsDir() {
			names[n] = true
		}
	}
	for n := range names {
		src, err := os.ReadFile(filepath.Join(dir, n+".pkg"))
		if err != nil || len(src) > lang.MaxParseSize {
			continue
		}
		var p lockPkg
		for _, x := range parseSexps(src) {
			switch x.head() {
			case "version":
				p.version = x.value("version")
				if a := x.atoms(); len(a) > 0 {
					p.version = a[0].atom
				}
			case "depends":
				seen := map[string]bool{}
				for _, a := range flatten(x.list[1:], nil, 0) {
					if names[a.atom] && a.atom != n && !seen[a.atom] {
						seen[a.atom] = true
						p.deps = append(p.deps, a.atom)
					}
				}
			}
		}
		sort.Strings(p.deps)
		if p.version != "" {
			s.locked[n] = p.version
		}
		r.lock[n] = p
	}
}

// openedNames adds the modules src opens or includes (open M, open! M, include M)
// to set. It is a byte scan, not the lexer: a name in a comment costs one more
// file read for readExports, nothing more.
func openedNames(src []byte, set map[string]bool) {
	for _, kw := range []string{"open", "include"} {
		for i := 0; ; {
			k := bytes.Index(src[i:], []byte(kw))
			if k < 0 {
				break
			}
			at := i + k
			i = at + len(kw)
			if at > 0 && isIdent(src[at-1]) {
				continue
			}
			j := i
			if j < len(src) && src[j] == '!' {
				j++
			}
			start := j
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
				j++
			}
			if j == start || j >= len(src) || src[j] < 'A' || src[j] > 'Z' {
				continue
			}
			e := j
			for e < len(src) && isIdent(src[e]) {
				e++
			}
			set[string(src[j:e])] = true
		}
	}
}

// readExports reads what the files of modules that some source opens (or
// includes) declare: projects commonly open a module of their own - Import,
// Std, Prelude - that aliases or defines the modules the rest use.
func (r *resolver) readExports(all []*scan.File) {
	type src struct {
		f *scan.File
		b []byte
	}
	var sources []src
	opened := map[string]bool{}
	for _, f := range all {
		if fileClass(f.Path) != classSource || ignored(f.Path) || !readable(f) {
			continue
		}
		b, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		sources = append(sources, src{f, b})
		openedNames(b, opened)
	}
	for _, s := range sources {
		if !opened[moduleName(s.f.Path)] || path.Ext(s.f.Path) == ".mli" && r.hasImpl(s.f.Path) {
			continue
		}
		ex := readSource(s.b, path.Ext(s.f.Path))
		e := &exports{modules: map[string]bool{}}
		for _, sym := range ex.Symbols {
			if (sym.Kind == "module" || sym.Kind == "module type") && !strings.Contains(sym.Name, ".") {
				e.modules[strings.SplitN(sym.Name, "@", 2)[0]] = true
			}
		}
		for _, im := range ex.Imports {
			if strings.HasPrefix(im.Name, kindInclude+"\n") {
				first, _, _ := strings.Cut(im.Module, ".")
				e.includes = append(e.includes, first)
			}
		}
		r.exports[s.f.Path] = e
	}
}

// hasImpl reports whether an interface has its implementation beside it.
func (r *resolver) hasImpl(mli string) bool {
	c := r.compOf[mli]
	return c != nil && c.modules[moduleName(mli)] != mli
}

// exported is the file that declares module name for a file that opens file,
// following the local modules file includes; "" if none.
func (r *resolver) exported(file, name string, depth int) string {
	e := r.exports[file]
	if e == nil || depth > 4 {
		return ""
	}
	if e.modules[name] {
		return file
	}
	c := r.compOf[file]
	for _, inc := range e.includes {
		if c != nil {
			if p, ok := c.modules[inc]; ok && p != file {
				if d := r.exported(p, name, depth+1); d != "" {
					return d
				}
				continue
			}
		}
		// include Stdune: a library of the repository.
		if l := r.libByMain[inc]; l != nil && l.wrapped {
			if p, ok := l.modules[name]; ok {
				return p
			}
		}
		if c != nil {
			if p := r.localFile(c, inc); p != "" && p != file {
				if d := r.exported(p, name, depth+1); d != "" {
					return d
				}
			}
		}
	}
	return ""
}

// components makes dune's libraries, executables and tests components, and the
// directories no stanza builds components of their own.
func (r *resolver) components(dirModules map[string]map[string]string, dunes map[string]*duneFile, duneAt map[string]string) {
	// The stanzas of dir build the modules of dir, and of the directories below it
	// when it has (include_subdirs).
	owner := func(dir string) (string, bool) {
		for d := dir; ; d = path.Dir(d) {
			if df := dunes[d]; df != nil && (len(df.stanzas) > 0 || df.includeSubdirs != "") {
				return d, d == dir || df.includeSubdirs != ""
			}
			if d == "." || d == "/" {
				return "", false
			}
		}
	}
	pools := map[string]map[string]string{}
	var dirs []string
	for dir := range dirModules {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		od, ok := owner(dir)
		if !ok || len(dunes[od].stanzas) == 0 {
			c := &component{dir: dir, modules: dirModules[dir]}
			for _, p := range dirModules[dir] {
				r.compOf[p] = c
			}
			continue
		}
		if pools[od] == nil {
			pools[od] = map[string]string{}
		}
		prefix := ""
		if dunes[od].includeSubdirs == "qualified" && dir != od {
			prefix = qualifier(od, dir)
			pools[od][prefix] = dir
		}
		for m, p := range dirModules[dir] {
			if prefix != "" {
				m = prefix + "." + m
			}
			if _, ok := pools[od][m]; !ok {
				pools[od][m] = p
			}
		}
	}
	var owners []string
	for od := range dunes {
		owners = append(owners, od)
	}
	sort.Strings(owners)
	for _, od := range owners {
		df := dunes[od]
		pool := pools[od]
		var comps []*component
		claimed := map[string]bool{}
		for _, st := range df.stanzas {
			c := &component{dir: od, dune: duneAt[od], modules: map[string]string{}, wrapped: st.wrapped, opens: st.opens,
				qualified: df.includeSubdirs == "qualified"}
			for _, l := range append(append([]*sexp{}, st.libs...), st.pps...) {
				c.libs = append(c.libs, l.atom)
			}
			if st.kind == "library" && len(st.names) > 0 {
				c.name = st.names[0]
				c.main = capitalize(c.name)
				r.libs[c.name] = c
				for _, p := range st.public {
					r.libs[p] = c
				}
				if _, ok := r.libByMain[c.main]; !ok {
					r.libByMain[c.main] = c
				}
			}
			if st.modules != nil {
				for _, m := range st.modules {
					if p, ok := pool[m]; ok {
						c.modules[m] = p
						claimed[m] = true
					}
				}
			}
			comps = append(comps, c)
		}
		for i, st := range df.stanzas {
			if st.modules != nil {
				continue
			}
			except := map[string]bool{}
			for _, m := range st.except {
				except[m] = true
			}
			for m, p := range pool {
				if !claimed[m] && !except[m] {
					comps[i].modules[m] = p
				}
			}
		}
		for i := len(comps) - 1; i >= 0; i-- { // the first stanza wins a shared module
			for _, p := range comps[i].modules {
				if strings.Contains(path.Base(p), ".") {
					r.compOf[p] = comps[i]
				}
			}
		}
		// Modules no stanza takes (another stanza's modules field left them out)
		// make their directory's component.
		loose := map[string]*component{}
		for _, p := range pool {
			if _, ok := r.compOf[p]; !ok && strings.Contains(path.Base(p), ".") {
				dir := path.Dir(p)
				if loose[dir] == nil {
					loose[dir] = &component{dir: dir, modules: dirModules[dir]}
				}
				r.compOf[p] = loose[dir]
			}
		}
	}
}

// Resolve maps a module path, a library, or a manifest's package to its target.
//
// Implements: REQ-OCAML-004, REQ-OCAML-005, REQ-OCAML-008
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, rest, _ := strings.Cut(imp.Name, "\n")
	switch kind {
	case kindLib, kindRequire:
		return r.library(file, imp.Module)
	case kindDep:
		return r.manifestDep(file, imp.Module, rest)
	case kindPin:
		if p, ok := r.own[imp.Module]; ok {
			return lang.Target{Local: p}
		}
		return pinTarget(opam.Pin{Name: imp.Module, URL: rest})
	}
	var opens []string
	if rest != "" {
		opens = strings.Split(rest, ",")
	}
	return r.module(file, strings.Split(imp.Module, "."), opens)
}

// library resolves a findlib library: the repository's own, the compiler's, or an
// opam package's.
func (r *resolver) library(file, lib string) lang.Target {
	if c := r.libs[lib]; c != nil {
		return lang.Target{Local: c.dune}
	}
	if s := stdLibrary(lib); s != "" {
		return lang.Target{Ecosystem: ecoStd, Package: s}
	}
	pkg := libraryPackage(lib)
	if pkg == "" {
		return lang.Target{Ecosystem: ecoStd, Package: lib}
	}
	return r.pkg(file, pkg)
}

// governing are the manifests of the file's directory and its ancestors, nearest
// first; a file with none above it is governed by all of them.
func (r *resolver) governing(file string) []*manifests {
	var out []*manifests
	for d := path.Dir(file); ; d = path.Dir(d) {
		if s := r.setAt[d]; s != nil {
			out = append(out, s)
		}
		if d == "." || d == "/" {
			break
		}
	}
	if len(out) == 0 {
		return r.sets
	}
	return out
}

func (s *manifests) knows(pkg string) bool {
	_, d := s.deps[pkg]
	_, p := s.pins[pkg]
	_, l := s.locked[pkg]
	return d || p || l
}

// pkg is an opam package as the manifests governing file have it: the
// repository's own package, locked, pinned to a source, constrained, or
// undeclared.
func (r *resolver) pkg(file, pkg string) lang.Target {
	if p, ok := r.own[pkg]; ok {
		return lang.Target{Local: p}
	}
	for _, s := range r.governing(file) {
		if s.knows(pkg) {
			return s.target(pkg, nil)
		}
	}
	for _, s := range r.sets {
		if s.knows(pkg) {
			return s.target(pkg, nil)
		}
	}
	return lang.Target{Ecosystem: ecoOpam, Package: pkg, Unresolved: true}
}

// target is how s pins pkg; line is the requirement of the manifest line being
// resolved, else the directory's first declaration of pkg.
//
// Implements: REQ-OCAML-008
func (s *manifests) target(pkg string, line *opam.Dep) lang.Target {
	d, declared := s.deps[pkg]
	if line != nil {
		d, declared = *line, true
	}
	if v, ok := s.locked[pkg]; ok {
		t := lang.Target{Ecosystem: ecoOpam, Package: pkg, Version: v, Pinned: true}
		if declared && d.Constraint != "" && d.Exact != v {
			t.Requested = d.Constraint
		}
		return t
	}
	if p, ok := s.pins[pkg]; ok {
		return pinTarget(p)
	}
	return depTarget(pkg, d)
}

func depTarget(pkg string, d opam.Dep) lang.Target {
	t := lang.Target{Ecosystem: ecoOpam, Package: pkg}
	switch {
	case d.Exact != "":
		t.Version, t.Pinned = d.Exact, true
	case d.Constraint != "":
		t.Version, t.Floating = d.Constraint, true
	default:
		t.Floating = true
	}
	return t
}

// pinTarget is a package pinned to a source: a commit pins it, a branch or tag (or
// nothing) leaves it moving.
func pinTarget(p opam.Pin) lang.Target {
	u, ref := opam.SplitRef(p.URL)
	t := lang.Target{Ecosystem: ecoOpam, Package: p.Name, Origin: strings.TrimPrefix(u, "git+")}
	if lang.Commit(ref) {
		t.Version, t.Pinned = ref, true
	} else {
		t.Version, t.Floating = ref, true
	}
	return t
}

// manifestDep resolves a dependency a manifest line declares, with the line's own
// constraint; a lock's line is its pin.
func (r *resolver) manifestDep(file, pkg, constraint string) lang.Target {
	if p, ok := r.own[pkg]; ok {
		if p == file {
			return lang.Target{}
		}
		return lang.Target{Local: p}
	}
	d := opam.Dep{Name: pkg, Constraint: constraint}
	if v, ok := strings.CutPrefix(constraint, "= "); ok && !strings.ContainsAny(v, " &|") && v != "version" {
		d.Exact = v
	}
	if fileClass(file) == classLock {
		return depTarget(pkg, d)
	}
	s := r.setAt[path.Dir(file)]
	if s == nil {
		return depTarget(pkg, d)
	}
	return s.target(pkg, &d)
}

// module resolves a module path (at most two segments) a source file names; a
// path nothing in reach explains is dropped.
//
// Implements: REQ-OCAML-004, REQ-OCAML-005, REQ-OCAML-008, REQ-OCAML-010
func (r *resolver) module(file string, segments []string, opens []string) lang.Target {
	first := segments[0]
	if first == moduleName(file) {
		return lang.Target{}
	}
	c := r.compOf[file]
	if c == nil {
		c = &component{dir: path.Dir(file)}
	}
	// 1. The component's own modules.
	if len(segments) > 1 {
		if p, ok := c.modules[first+"."+segments[1]]; ok {
			return local(p)
		}
	}
	if p, ok := c.modules[first]; ok && p != file {
		return local(p)
	}
	if dir := path.Dir(file); c.qualified && dir != c.dir {
		if p, ok := c.modules[qualifier(c.dir, dir)+"."+first]; ok && p != file {
			return local(p)
		}
	}
	// 2. What the file's and the component's opens bring into scope.
	all := append(append([]string{}, c.opens...), opens...)
	for i := len(all) - 1; i >= 0; i-- {
		o, _, _ := strings.Cut(all[i], ".")
		if p := r.localFile(c, o); p != "" && p != file {
			if d := r.exported(p, first, 0); d != "" {
				return local(d)
			}
		}
		if l := r.libByMain[o]; l != nil && l.wrapped {
			if p, ok := l.modules[first]; ok {
				return local(p)
			}
		}
		if stdModules[first] == "stdlib" {
			if pkg := r.modulePackage(c, o); stdlibReplacements[pkg] {
				return r.pkg(file, pkg)
			}
		}
	}
	// 3. Libraries of the repository the component uses.
	for _, name := range c.libs {
		if l := r.libs[name]; l != nil {
			if t, ok := r.inLibrary(l, segments); ok {
				return t
			}
		}
	}
	// 4. The standard library.
	if lib := stdModules[first]; lib != "" {
		return lang.Target{Ecosystem: ecoStd, Package: lib}
	}
	if compilerLibs[first] {
		for _, l := range c.libs {
			if stdLibrary(l) == "compiler-libs" {
				return lang.Target{Ecosystem: ecoStd, Package: "compiler-libs"}
			}
		}
		for _, l := range c.libs {
			if libraryPackage(l) == "ppxlib" {
				return r.pkg(file, "ppxlib")
			}
		}
	}
	// 5. Packages of the libraries the component uses.
	if lib := bestLibrary(c.libs, first); lib != "" {
		return r.pkg(file, libraryPackage(lib))
	}
	// 6. A library or module of the repository the component does not declare.
	if l := r.libByMain[first]; l != nil {
		if t, ok := r.inLibrary(l, segments); ok {
			return t
		}
	}
	if ps := r.files[first]; len(ps) == 1 && ps[0] != file {
		return local(ps[0])
	}
	// 7. The one package whose module the file opens: open Cmdliner, then Term.
	opened := ""
	for _, o := range all {
		first, _, _ := strings.Cut(o, ".")
		if r.localFile(c, first) != "" || r.libByMain[first] != nil || stdModules[first] != "" {
			continue
		}
		if pkg := r.modulePackage(c, first); pkg != "" && pkg != opened {
			if opened != "" {
				opened = "-"
				break
			}
			opened = pkg
		}
	}
	if opened != "" && opened != "-" {
		return r.pkg(file, opened)
	}
	// 8. Packages the manifests declare, then well-known modules.
	var declared []string
	for _, s := range r.governing(file) {
		for p := range s.deps {
			declared = append(declared, p)
		}
		for p := range s.locked {
			declared = append(declared, p)
		}
	}
	sort.Strings(declared)
	if p := bestLibrary(declared, first); p != "" {
		return r.pkg(file, libraryPackage(p))
	}
	if p, ok := modulePackages[first]; ok {
		return r.pkg(file, p)
	}
	return lang.Target{}
}

// qualifier is the module path of a subdirectory under (include_subdirs
// qualified): src/rpc/client under src is Rpc.Client.
func qualifier(top, dir string) string {
	rel := strings.TrimPrefix(dir, top+"/")
	if top == "." {
		rel = dir
	}
	var segments []string
	for _, s := range strings.Split(rel, "/") {
		segments = append(segments, capitalize(s))
	}
	return strings.Join(segments, ".")
}

func local(p string) lang.Target { return lang.Target{Local: p} }

// localFile is the file of a module of the repository a component can name: its
// own, a module of an unwrapped library it uses, a library's main module.
func (r *resolver) localFile(c *component, m string) string {
	if p, ok := c.modules[m]; ok {
		return p
	}
	for _, name := range c.libs {
		if l := r.libs[name]; l != nil && (!l.wrapped || l.main == m) {
			if p, ok := l.modules[m]; ok {
				return p
			}
		}
	}
	if l := r.libByMain[m]; l != nil {
		return l.modules[m]
	}
	return ""
}

// inLibrary resolves a path through a library of the repository: Lib.Module to the
// module's file (wrapped), Lib to its main module's file or its dune file, and a
// module of an unwrapped library to its file.
func (r *resolver) inLibrary(l *component, segments []string) (lang.Target, bool) {
	if segments[0] == l.main {
		if len(segments) > 1 && l.wrapped {
			if p, ok := l.modules[segments[1]]; ok {
				return local(p), true
			}
		}
		if p, ok := l.modules[l.main]; ok {
			return local(p), true
		}
		return lang.Target{Local: l.dune}, true
	}
	if !l.wrapped {
		if p, ok := l.modules[segments[0]]; ok {
			return local(p), true
		}
	}
	return lang.Target{}, false
}

// bestLibrary is the library (or package) of libs that best provides module.
func bestLibrary(libs []string, module string) string {
	best, score := "", 0
	for _, l := range libs {
		if s := provides(l, module); s > score || s == score && s > 0 && len(libraryPackage(l)) > len(libraryPackage(best)) {
			best, score = l, s
		}
	}
	return best
}

// modulePackage is the opam package whose module an open names, "" if none.
func (r *resolver) modulePackage(c *component, module string) string {
	if lib := bestLibrary(c.libs, module); lib != "" {
		return libraryPackage(lib)
	}
	return modulePackages[module]
}

// Dependencies answers --resolve-depth from dune's lock directory: a locked
// package's dependencies at their locked versions. opam's own locks are flat and
// answer nothing.
//
// Implements: REQ-OCAML-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoOpam {
		return nil
	}
	p, ok := r.lock[t.Package]
	if !ok || t.Version != "" && t.Version != p.version {
		return nil
	}
	var out []lang.Target
	for _, d := range p.deps {
		if opam.Compiler(d) {
			continue
		}
		if _, own := r.own[d]; own {
			continue
		}
		dt := lang.Target{Ecosystem: ecoOpam, Package: d, Version: r.lock[d].version}
		dt.Pinned = dt.Version != ""
		out = append(out, dt)
	}
	return out
}
