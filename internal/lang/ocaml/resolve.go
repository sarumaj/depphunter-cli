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
	directory string
	dune      string            // the dune file ("" for a directory without one)
	modules   map[string]string // module name (Sub.Module under qualified include_subdirs) -> file
	name      string            // a library's name
	main      string            // a library's main module: its name capitalized
	wrapped   bool
	// qualified is (include_subdirs qualified): a subdirectory's modules are
	// Sub.Module outside it.
	qualified bool
	libraries []string // the libraries it uses (libraries and pps)
	opens     []string // modules its flags open (-open M)
}

// manifests is what one directory's dune-project, opam files and locks say.
type manifests struct {
	directory    string
	own          map[string]string // package described here -> its file
	dependencies map[string]opam.Dependency
	pins         map[string]opam.Pin
	locked       map[string]string // package -> version, from *.opam.locked, opam.locked or dune.lock
}

type lockPackage struct {
	version      string
	dependencies []string
}

type resolver struct {
	compOf        map[string]*component // source file -> component
	libraries     map[string]*component // local library by name and public name
	libraryByMain map[string]*component // local library by main module
	files         map[string][]string   // module name -> every file of that name
	sets          []*manifests          // shallowest first
	setAt         map[string]*manifests
	own           map[string]string // every package the repository describes -> its file
	lock          map[string]lockPackage
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

// extensionRank orders the files of one module: the implementation (or what generates
// it) before the interface.
var extensionRank = map[string]int{".ml": 0, ".mly": 1, ".mll": 2, ".mli": 3}

// rank orders the files of one module; a file a rule preprocesses (t.cppo.ml) comes
// after the module's own.
func rank(p string) int {
	r := extensionRank[path.Ext(p)]
	if strings.Count(path.Base(p), ".") > 1 {
		r += 4
	}
	return r
}

// Implements: REQ-OCAML-004, REQ-OCAML-006, REQ-OCAML-007, REQ-OCAML-009
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{
		compOf: map[string]*component{}, libraries: map[string]*component{}, libraryByMain: map[string]*component{},
		files: map[string][]string{}, setAt: map[string]*manifests{}, own: map[string]string{},
		lock: map[string]lockPackage{}, exports: map[string]*exports{},
	}
	directoryModules := map[string]map[string]string{} // dir -> module -> file
	dunes := map[string]*duneFile{}
	duneAt := map[string]string{}
	for _, f := range all {
		if ignored(f.Path) || !lang.Readable(f) {
			continue
		}
		switch fileClass(f.Path) {
		case classSource:
			directory, m := path.Dir(f.Path), moduleName(f.Path)
			if directoryModules[directory] == nil {
				directoryModules[directory] = map[string]string{}
			}
			if old, ok := directoryModules[directory][m]; !ok || rank(f.Path) < rank(old) {
				directoryModules[directory][m] = f.Path
			}
		case classDune:
			if source, err := os.ReadFile(f.AbsolutePath); err == nil {
				dunes[path.Dir(f.Path)] = readDune(source)
				duneAt[path.Dir(f.Path)] = f.Path
			}
		case classProject, classOpam, classLock:
			r.readManifest(f)
		}
	}
	for _, modules := range directoryModules {
		for m, p := range modules {
			r.files[m] = append(r.files[m], p)
		}
	}
	for _, files := range r.files {
		sort.Strings(files)
	}
	r.components(directoryModules, dunes, duneAt)
	// A module's other files (its interface, a file a rule preprocesses) are in
	// its component.
	for _, f := range all {
		if _, ok := r.compOf[f.Path]; ok || fileClass(f.Path) != classSource {
			continue
		}
		if p, ok := directoryModules[path.Dir(f.Path)][moduleName(f.Path)]; ok {
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
	sort.Slice(r.sets, func(i, j int) bool { return lang.ShallowestFirst(r.sets[i].directory, r.sets[j].directory) })
	return r
}

func (r *resolver) set(directory string) *manifests {
	s := r.setAt[directory]
	if s == nil {
		s = &manifests{directory: directory, own: map[string]string{}, dependencies: map[string]opam.Dependency{}, pins: map[string]opam.Pin{}, locked: map[string]string{}}
		r.setAt[directory] = s
		r.sets = append(r.sets, s)
	}
	return s
}

func (r *resolver) readManifest(f *scan.File) {
	source, err := os.ReadFile(f.AbsolutePath)
	if err != nil {
		return
	}
	s := r.set(path.Dir(f.Path))
	addDependency := func(d opam.Dependency) {
		if _, ok := s.dependencies[d.Name]; !ok && d.Name != "" {
			s.dependencies[d.Name] = d
		}
	}
	addPin := func(p opam.Pin) {
		if _, ok := s.pins[p.Name]; !ok && p.Name != "" {
			s.pins[p.Name] = p
		}
	}
	switch fileClass(f.Path) {
	case classProject:
		p := readDuneProject(source)
		for _, projectPackage := range p.packages {
			if projectPackage.name != "" {
				s.own[projectPackage.name] = f.Path
				if _, ok := r.own[projectPackage.name]; !ok {
					r.own[projectPackage.name] = f.Path
				}
			}
			for _, d := range projectPackage.depends {
				addDependency(d)
			}
		}
		for _, p := range p.pins {
			addPin(p)
		}
	case classOpam:
		o := opam.Read(source)
		name := opamPackageName(f.Path, o)
		s.own[name] = f.Path
		r.own[name] = f.Path // an opam file is the package's own description
		for _, d := range append(o.Depends, o.Depopts...) {
			addDependency(d)
		}
		for _, p := range o.Pins {
			addPin(p)
		}
	case classLock:
		o := opam.Read(source)
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
	directory := filepath.Join(root, filepath.FromSlash(s.directory), "dune.lock")
	entries, err := os.ReadDir(directory)
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
		source, ok := lang.ReadCapped(filepath.Join(directory, n+".pkg"))
		if !ok {
			continue
		}
		var p lockPackage
		for _, x := range parseSexps(source) {
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
						p.dependencies = append(p.dependencies, a.atom)
					}
				}
			}
		}
		sort.Strings(p.dependencies)
		if p.version != "" {
			s.locked[n] = p.version
		}
		r.lock[n] = p
	}
}

// openedNames adds the modules source opens or includes (open M, open! M, include M)
// to set. It is a byte scan, not the lexer: a name in a comment costs one more
// file read for readExports, nothing more.
func openedNames(source []byte, set map[string]bool) {
	for _, keyword := range []string{"open", "include"} {
		for i := 0; ; {
			k := bytes.Index(source[i:], []byte(keyword))
			if k < 0 {
				break
			}
			at := i + k
			i = at + len(keyword)
			if at > 0 && isIdentifier(source[at-1]) {
				continue
			}
			j := i
			if j < len(source) && source[j] == '!' {
				j++
			}
			start := j
			for j < len(source) && (source[j] == ' ' || source[j] == '\t' || source[j] == '\n' || source[j] == '\r') {
				j++
			}
			if j == start || j >= len(source) || source[j] < 'A' || source[j] > 'Z' {
				continue
			}
			e := j
			for e < len(source) && isIdentifier(source[e]) {
				e++
			}
			set[string(source[j:e])] = true
		}
	}
}

// readExports reads what the files of modules that some source opens (or
// includes) declare: projects commonly open a module of their own - Import,
// Std, Prelude - that aliases or defines the modules the rest use.
func (r *resolver) readExports(all []*scan.File) {
	type source struct {
		f *scan.File
		b []byte
	}
	var sources []source
	opened := map[string]bool{}
	for _, f := range all {
		if fileClass(f.Path) != classSource || ignored(f.Path) {
			continue
		}
		b, ok := lang.ReadScanned(f)
		if !ok {
			continue
		}
		sources = append(sources, source{f, b})
		openedNames(b, opened)
	}
	for _, s := range sources {
		if !opened[moduleName(s.f.Path)] || path.Ext(s.f.Path) == ".mli" && r.hasImplementation(s.f.Path) {
			continue
		}
		extraction := readSource(s.b, path.Ext(s.f.Path))
		e := &exports{modules: map[string]bool{}}
		for _, symbol := range extraction.Symbols {
			if (symbol.Kind == "module" || symbol.Kind == "module type") && !strings.Contains(symbol.Name, ".") {
				e.modules[strings.SplitN(symbol.Name, "@", 2)[0]] = true
			}
		}
		for _, rawImport := range extraction.Imports {
			if strings.HasPrefix(rawImport.Name, kindInclude+"\n") {
				first, _, _ := strings.Cut(rawImport.Module, ".")
				e.includes = append(e.includes, first)
			}
		}
		r.exports[s.f.Path] = e
	}
}

// hasImplementation reports whether an interface has its implementation beside it.
func (r *resolver) hasImplementation(mli string) bool {
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
	for _, include := range e.includes {
		if c != nil {
			if p, ok := c.modules[include]; ok && p != file {
				if d := r.exported(p, name, depth+1); d != "" {
					return d
				}
				continue
			}
		}
		// include Stdune: a library of the repository.
		if l := r.libraryByMain[include]; l != nil && l.wrapped {
			if p, ok := l.modules[name]; ok {
				return p
			}
		}
		if c != nil {
			if p := r.localFile(c, include); p != "" && p != file {
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
func (r *resolver) components(directoryModules map[string]map[string]string, dunes map[string]*duneFile, duneAt map[string]string) {
	// The stanzas of directory build the modules of directory, and of the directories below it
	// when it has (include_subdirs).
	owner := func(directory string) (string, bool) {
		for d := range lang.DirectoryAndAncestors(directory) {
			if df := dunes[d]; df != nil && (len(df.stanzas) > 0 || df.includeSubdirectories != "") {
				return d, d == directory || df.includeSubdirectories != ""
			}
		}
		return "", false
	}
	pools := map[string]map[string]string{}
	var directories []string
	for directory := range directoryModules {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	for _, directory := range directories {
		ownerDirectory, ok := owner(directory)
		if !ok || len(dunes[ownerDirectory].stanzas) == 0 {
			c := &component{directory: directory, modules: directoryModules[directory]}
			for _, p := range directoryModules[directory] {
				r.compOf[p] = c
			}
			continue
		}
		if pools[ownerDirectory] == nil {
			pools[ownerDirectory] = map[string]string{}
		}
		prefix := ""
		if dunes[ownerDirectory].includeSubdirectories == "qualified" && directory != ownerDirectory {
			prefix = qualifier(ownerDirectory, directory)
			pools[ownerDirectory][prefix] = directory
		}
		for m, p := range directoryModules[directory] {
			if prefix != "" {
				m = prefix + "." + m
			}
			if _, ok := pools[ownerDirectory][m]; !ok {
				pools[ownerDirectory][m] = p
			}
		}
	}
	var owners []string
	for ownerDirectory := range dunes {
		owners = append(owners, ownerDirectory)
	}
	sort.Strings(owners)
	for _, ownerDirectory := range owners {
		df := dunes[ownerDirectory]
		pool := pools[ownerDirectory]
		var comps []*component
		claimed := map[string]bool{}
		for _, stanza := range df.stanzas {
			c := &component{directory: ownerDirectory, dune: duneAt[ownerDirectory], modules: map[string]string{}, wrapped: stanza.wrapped, opens: stanza.opens,
				qualified: df.includeSubdirectories == "qualified"}
			for _, l := range append(append([]*sexp{}, stanza.libraries...), stanza.pps...) {
				c.libraries = append(c.libraries, l.atom)
			}
			if stanza.kind == "library" && len(stanza.names) > 0 {
				c.name = stanza.names[0]
				c.main = capitalize(c.name)
				r.libraries[c.name] = c
				for _, p := range stanza.public {
					r.libraries[p] = c
				}
				if _, ok := r.libraryByMain[c.main]; !ok {
					r.libraryByMain[c.main] = c
				}
			}
			if stanza.modules != nil {
				for _, m := range stanza.modules {
					if p, ok := pool[m]; ok {
						c.modules[m] = p
						claimed[m] = true
					}
				}
			}
			comps = append(comps, c)
		}
		for i, stanza := range df.stanzas {
			if stanza.modules != nil {
				continue
			}
			except := map[string]bool{}
			for _, m := range stanza.except {
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
				directory := path.Dir(p)
				if loose[directory] == nil {
					loose[directory] = &component{directory: directory, modules: directoryModules[directory]}
				}
				r.compOf[p] = loose[directory]
			}
		}
	}
}

// Resolve maps a module path, a library, or a manifest's package to its target.
//
// Implements: REQ-OCAML-004, REQ-OCAML-005, REQ-OCAML-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, rest, _ := strings.Cut(rawImport.Name, "\n")
	switch kind {
	case kindLibrary, kindRequire:
		return r.library(file, rawImport.Module)
	case kindDependency:
		return r.manifestDependency(file, rawImport.Module, rest)
	case kindPin:
		if p, ok := r.own[rawImport.Module]; ok {
			return lang.Target{Local: p}
		}
		return pinTarget(opam.Pin{Name: rawImport.Module, URL: rest})
	}
	var opens []string
	if rest != "" {
		opens = strings.Split(rest, ",")
	}
	return r.module(file, strings.Split(rawImport.Module, "."), opens)
}

// library resolves a findlib library: the repository's own, the compiler's, or an
// opam package's.
func (r *resolver) library(file, library string) lang.Target {
	if c := r.libraries[library]; c != nil {
		return lang.Target{Local: c.dune}
	}
	if s := stdLibrary(library); s != "" {
		return lang.Target{Ecosystem: ecosystemStd, Package: s}
	}
	packageName := libraryPackage(library)
	if packageName == "" {
		return lang.Target{Ecosystem: ecosystemStd, Package: library}
	}
	return r.packageName(file, packageName)
}

// governing are the manifests of the file's directory and its ancestors, nearest
// first; a file with none above it is governed by all of them.
func (r *resolver) governing(file string) []*manifests {
	out := lang.Chain(r.setAt, file)
	if len(out) == 0 {
		return r.sets
	}
	return out
}

func (s *manifests) knows(packageName string) bool {
	_, d := s.dependencies[packageName]
	_, p := s.pins[packageName]
	_, l := s.locked[packageName]
	return d || p || l
}

// packageName is an opam package as the manifests governing file have it: the
// repository's own package, locked, pinned to a source, constrained, or
// undeclared.
func (r *resolver) packageName(file, packageName string) lang.Target {
	if p, ok := r.own[packageName]; ok {
		return lang.Target{Local: p}
	}
	for _, s := range r.governing(file) {
		if s.knows(packageName) {
			return s.target(packageName, nil)
		}
	}
	for _, s := range r.sets {
		if s.knows(packageName) {
			return s.target(packageName, nil)
		}
	}
	return lang.Target{Ecosystem: ecosystemOpam, Package: packageName, Unresolved: true}
}

// target is how s pins pkg; line is the requirement of the manifest line being
// resolved, else the directory's first declaration of pkg.
//
// Implements: REQ-OCAML-008
func (s *manifests) target(packageName string, line *opam.Dependency) lang.Target {
	d, declared := s.dependencies[packageName]
	if line != nil {
		d, declared = *line, true
	}
	if v, ok := s.locked[packageName]; ok {
		t := lang.Target{Ecosystem: ecosystemOpam, Package: packageName, Version: v, Pinned: true}
		if declared && d.Constraint != "" && d.Exact != v {
			t.Requested = d.Constraint
		}
		return t
	}
	if p, ok := s.pins[packageName]; ok {
		return pinTarget(p)
	}
	return dependencyTarget(packageName, d)
}

func dependencyTarget(packageName string, d opam.Dependency) lang.Target {
	t := lang.Target{Ecosystem: ecosystemOpam, Package: packageName}
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
	u, reference := opam.SplitReference(p.URL)
	t := lang.Target{Ecosystem: ecosystemOpam, Package: p.Name, Origin: strings.TrimPrefix(u, "git+")}
	if lang.Commit(reference) {
		t.Version, t.Pinned = reference, true
	} else {
		t.Version, t.Floating = reference, true
	}
	return t
}

// manifestDependency resolves a dependency a manifest line declares, with the line's own
// constraint; a lock's line is its pin.
func (r *resolver) manifestDependency(file, packageName, constraint string) lang.Target {
	if p, ok := r.own[packageName]; ok {
		if p == file {
			return lang.Target{}
		}
		return lang.Target{Local: p}
	}
	d := opam.Dependency{Name: packageName, Constraint: constraint}
	if v, ok := strings.CutPrefix(constraint, "= "); ok && !strings.ContainsAny(v, " &|") && v != "version" {
		d.Exact = v
	}
	if fileClass(file) == classLock {
		return dependencyTarget(packageName, d)
	}
	s := r.setAt[path.Dir(file)]
	if s == nil {
		return dependencyTarget(packageName, d)
	}
	return s.target(packageName, &d)
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
		c = &component{directory: path.Dir(file)}
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
	if directory := path.Dir(file); c.qualified && directory != c.directory {
		if p, ok := c.modules[qualifier(c.directory, directory)+"."+first]; ok && p != file {
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
		if l := r.libraryByMain[o]; l != nil && l.wrapped {
			if p, ok := l.modules[first]; ok {
				return local(p)
			}
		}
		if stdModules[first] == "stdlib" {
			if packageName := r.modulePackage(c, o); stdlibReplacements[packageName] {
				return r.packageName(file, packageName)
			}
		}
	}
	// 3. Libraries of the repository the component uses.
	for _, name := range c.libraries {
		if l := r.libraries[name]; l != nil {
			if t, ok := r.inLibrary(l, segments); ok {
				return t
			}
		}
	}
	// 4. The standard library.
	if library := stdModules[first]; library != "" {
		return lang.Target{Ecosystem: ecosystemStd, Package: library}
	}
	if compilerLibraries[first] {
		for _, l := range c.libraries {
			if stdLibrary(l) == "compiler-libs" {
				return lang.Target{Ecosystem: ecosystemStd, Package: "compiler-libs"}
			}
		}
		for _, l := range c.libraries {
			if libraryPackage(l) == "ppxlib" {
				return r.packageName(file, "ppxlib")
			}
		}
	}
	// 5. Packages of the libraries the component uses.
	if library := bestLibrary(c.libraries, first); library != "" {
		return r.packageName(file, libraryPackage(library))
	}
	// 6. A library or module of the repository the component does not declare.
	if l := r.libraryByMain[first]; l != nil {
		if t, ok := r.inLibrary(l, segments); ok {
			return t
		}
	}
	if files := r.files[first]; len(files) == 1 && files[0] != file {
		return local(files[0])
	}
	// 7. The one package whose module the file opens: open Cmdliner, then Term.
	opened := ""
	for _, o := range all {
		first, _, _ := strings.Cut(o, ".")
		if r.localFile(c, first) != "" || r.libraryByMain[first] != nil || stdModules[first] != "" {
			continue
		}
		if packageName := r.modulePackage(c, first); packageName != "" && packageName != opened {
			if opened != "" {
				opened = "-"
				break
			}
			opened = packageName
		}
	}
	if opened != "" && opened != "-" {
		return r.packageName(file, opened)
	}
	// 8. Packages the manifests declare, then well-known modules.
	var declared []string
	for _, s := range r.governing(file) {
		for p := range s.dependencies {
			declared = append(declared, p)
		}
		for p := range s.locked {
			declared = append(declared, p)
		}
	}
	sort.Strings(declared)
	if p := bestLibrary(declared, first); p != "" {
		return r.packageName(file, libraryPackage(p))
	}
	if p, ok := modulePackages[first]; ok {
		return r.packageName(file, p)
	}
	return lang.Target{}
}

// qualifier is the module path of a subdirectory under (include_subdirs
// qualified): src/rpc/client under src is Rpc.Client.
func qualifier(top, directory string) string {
	relative := strings.TrimPrefix(directory, top+"/")
	if top == "." {
		relative = directory
	}
	var segments []string
	for _, s := range strings.Split(relative, "/") {
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
	for _, name := range c.libraries {
		if l := r.libraries[name]; l != nil && (!l.wrapped || l.main == m) {
			if p, ok := l.modules[m]; ok {
				return p
			}
		}
	}
	if l := r.libraryByMain[m]; l != nil {
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

// bestLibrary is the library (or package) of libraries that best provides module.
func bestLibrary(libraries []string, module string) string {
	best, score := "", 0
	for _, l := range libraries {
		if s := provides(l, module); s > score || s == score && s > 0 && len(libraryPackage(l)) > len(libraryPackage(best)) {
			best, score = l, s
		}
	}
	return best
}

// modulePackage is the opam package whose module an open names, "" if none.
func (r *resolver) modulePackage(c *component, module string) string {
	if library := bestLibrary(c.libraries, module); library != "" {
		return libraryPackage(library)
	}
	return modulePackages[module]
}

// Dependencies answers --resolve-depth from dune's lock directory: a locked
// package's dependencies at their locked versions. opam's own locks are flat and
// answer nothing.
//
// Implements: REQ-OCAML-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemOpam {
		return nil
	}
	p, ok := r.lock[t.Package]
	if !ok || t.Version != "" && t.Version != p.version {
		return nil
	}
	var out []lang.Target
	for _, d := range p.dependencies {
		if opam.Compiler(d) {
			continue
		}
		if _, own := r.own[d]; own {
			continue
		}
		dependencyTarget := lang.Target{Ecosystem: ecosystemOpam, Package: d, Version: r.lock[d].version}
		dependencyTarget.Pinned = dependencyTarget.Version != ""
		out = append(out, dependencyTarget)
	}
	return out
}
