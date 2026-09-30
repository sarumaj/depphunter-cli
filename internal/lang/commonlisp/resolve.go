package commonlisp

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// sysReference is a system some .asd file of the repository defines.
type sysReference struct {
	system    *system
	asd       string // the .asd file
	directory string // its directory
}

// root is the directory the system's components and package-inferred files
// are relative to.
func (s *sysReference) root() string { return path.Join(s.directory, s.system.base) }

// pins is what the qlfile, qlfile.lock and ocicl.csv of one directory fix.
type pins struct {
	directory    string
	ql           map[string]qlEntry
	lock         map[string]lockEntry
	lockDist     string                // the dist version of the lock's last dist (the one with priority)
	qlDist       string                // ql :all <version>, or dist <quicklisp> <version>
	ocicl        map[string]ociclEntry // by system
	ociclProject map[string]ociclEntry // by project
}

type resolver struct {
	files       map[string]bool
	directories map[string]bool // directories holding files
	// systems are the repository's systems by name; fileSystems the systems
	// whose components include a file; asdDirectories the systems defined in each
	// directory's .asd files.
	systems        map[string]*sysReference
	fileSystems    map[string][]*sysReference
	asdDirectories map[string][]*sysReference
	all            []*sysReference
	// packages are the files defining each package of the repository (by
	// name and nickname), sorted.
	packages       map[string][]string
	nicknames      map[string]map[string]string // package -> its local nicknames
	stems          map[string][]string          // path without extension -> files
	registered     map[string]string            // package -> system, asdf:register-system-packages
	pins           map[string]*pins
	pinDirectories []string // shallowest first
	declared       sync.Map // file -> *declaration
	// installed are the systems Qlot installed into .qlot/ and ocicl into
	// systems/, by name, and installedNames their names, sorted.
	installed      map[string]*installedSystem
	installedNames []string
}

// installedSystem is a system Qlot or ocicl installed: the systems it depends
// on and the pins of the directory it was installed for.
type installedSystem struct {
	dependencies []string
	pins         *pins
}

// declaration is what a file's systems declare: the systems they depend on
// and the projects the governing qlfile, lock and ocicl.csv list.
type declaration struct {
	set   map[string]bool
	names []string // sorted
	pis   bool     // the file belongs to a package-inferred system
}

// ignored reports whether a path lies where Qlot and ocicl keep what they
// installed: .qlot/ anywhere, systems/ beside an ocicl.csv.
func ignored(f *scan.File) bool {
	if strings.HasPrefix(f.Path, ".qlot/") || strings.Contains(f.Path, "/.qlot/") {
		return true
	}
	segments := strings.Split(f.Path, "/")
	for i, s := range segments[:len(segments)-1] {
		if s != "systems" || f.AbsolutePath == "" {
			continue
		}
		absolute := strings.ReplaceAll(f.AbsolutePath, "\\", "/")
		if !strings.HasSuffix(absolute, f.Path) {
			continue
		}
		directory := absolute[:len(absolute)-len(f.Path)] + strings.Join(segments[:i], "/")
		if besideOcicl(directory) {
			return true
		}
	}
	return false
}

var ociclDirectories sync.Map // absolute directory -> bool

func besideOcicl(directory string) bool {
	if v, ok := ociclDirectories.Load(directory); ok {
		return v.(bool)
	}
	// Lstat: a marker committed as a symbolic link says nothing of its target.
	_, err := os.Lstat(path.Join(directory, "ocicl.csv"))
	ociclDirectories.Store(directory, err == nil)
	return err == nil
}

// newResolver reads the repository's .asd files for its systems, the
// sources defining packages for the package index, the qlfile, lock and
// ocicl.csv files for pins, and the .asd files of what Qlot and ocicl
// installed beside them.
//
// Implements: REQ-COMMONLISP-005, REQ-COMMONLISP-006, REQ-COMMONLISP-011, REQ-COMMONLISP-012
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{
		files: map[string]bool{}, directories: map[string]bool{}, systems: map[string]*sysReference{},
		fileSystems: map[string][]*sysReference{}, asdDirectories: map[string][]*sysReference{},
		packages: map[string][]string{}, nicknames: map[string]map[string]string{}, stems: map[string][]string{}, registered: map[string]string{}, pins: map[string]*pins{},
		installed: map[string]*installedSystem{},
	}
	var asds, sources []*scan.File
	for _, f := range all {
		if ignored(f) {
			continue
		}
		r.files[f.Path] = true
		stem := strings.TrimSuffix(f.Path, path.Ext(f.Path))
		r.stems[stem] = append(r.stems[stem], f.Path)
		for d := path.Dir(f.Path); ; d = path.Dir(d) {
			if r.directories[d] {
				break
			}
			r.directories[d] = true
			if d == "." || d == "/" {
				break
			}
		}
		if !lang.Readable(f) {
			continue
		}
		switch base := path.Base(f.Path); {
		case strings.EqualFold(path.Ext(base), ".asd"):
			asds = append(asds, f)
		case base == "qlfile", base == "qlfile.lock", base == "ocicl.csv":
			r.readPins(f)
		case lispSource(f):
			sources = append(sources, f)
		}
	}
	for _, f := range asds {
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		in := read(source)
		directory := path.Dir(f.Path)
		for _, s := range in.systems {
			reference := &sysReference{system: s, asd: f.Path, directory: directory}
			r.all = append(r.all, reference)
			r.asdDirectories[directory] = append(r.asdDirectories[directory], reference)
			if _, duplicate := r.systems[s.name]; !duplicate {
				r.systems[s.name] = reference
			}
			for _, c := range s.files {
				p := path.Join(directory, c)
				r.fileSystems[p] = append(r.fileSystems[p], reference)
			}
		}
		for p, s := range in.registered {
			r.registered[p] = s
		}
		r.index(f.Path, in)
	}
	// Package definitions: only files that mention one are read.
	var mu sync.Mutex
	lang.ForEachFile(context.Background(), sources, func(f *scan.File, source []byte) *lang.FileResult {
		if !mentionsPackage(source) {
			return nil
		}
		in := read(source)
		mu.Lock()
		r.index(f.Path, in)
		mu.Unlock()
		return nil
	})
	for _, files := range r.packages {
		sort.Strings(files)
	}
	for d := range r.pins {
		r.pinDirectories = append(r.pinDirectories, d)
	}
	sort.Slice(r.pinDirectories, func(i, j int) bool {
		a, b := strings.Count(r.pinDirectories[i], "/"), strings.Count(r.pinDirectories[j], "/")
		if a != b {
			return a < b
		}
		return r.pinDirectories[i] < r.pinDirectories[j]
	})
	repository := lang.OpenRoot(root)
	for _, d := range r.pinDirectories {
		if root == "" {
			break
		}
		p, base := r.pins[d], filepath.Join(root, filepath.FromSlash(d))
		if len(p.ql) > 0 || len(p.lock) > 0 {
			r.readInstalled(repository, filepath.Join(base, ".qlot", "dists"), p)
		}
		if len(p.ocicl) > 0 {
			r.readInstalled(repository, filepath.Join(base, "systems"), p)
		}
	}
	for n := range r.installed {
		r.installedNames = append(r.installedNames, n)
	}
	sort.Strings(r.installedNames)
	return r
}

// maxInstalledFiles bounds the files looked at in one installed tree.
const maxInstalledFiles = 100_000

// readInstalled reads the .asd files under a tree Qlot or ocicl installed
// (.qlot/dists/<dist>/software/<release>/, systems/<release>/): each system's
// :depends-on and :defsystem-depends-on, not its weak dependencies or the
// implementation modules it requires. A system already read is kept.
//
// Implements: REQ-COMMONLISP-012
func (r *resolver) readInstalled(repository lang.Root, directory string, p *pins) {
	n := 0
	repository.WalkDir(directory, func(f string, e fs.DirEntry, err error) error {
		if n++; err != nil || n > maxInstalledFiles {
			return nil
		}
		if e.IsDir() || !strings.EqualFold(filepath.Ext(f), ".asd") {
			return nil
		}
		// Measured once open: a symbolic link's own size (e.Info) says
		// nothing of its target's.
		source, ok := repository.ReadBounded(f)
		if !ok {
			return nil
		}
		for _, s := range read(source).systems {
			if r.installed[s.name] != nil {
				continue
			}
			in := &installedSystem{pins: p}
			for _, d := range s.dependencies {
				if !d.require && d.option != "weakly-depends-on" {
					in.dependencies = append(in.dependencies, d.name)
				}
			}
			r.installed[s.name] = in
		}
		return nil
	})
}

// Dependencies is what a Quicklisp project Qlot or ocicl installed depends
// on: the projects of the systems its primary system (named like the project,
// or like a git source's repository) depends on, pinned as the pins it was
// installed for say.
//
// Implements: REQ-COMMONLISP-012
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemQuicklisp {
		return nil
	}
	name := path.Base(t.Package)
	for _, n := range r.installedNames {
		if r.installed[name] != nil {
			break
		}
		if !strings.Contains(n, "/") && projectOf(n) == t.Package {
			name = n
		}
	}
	s := r.installed[name]
	if s == nil {
		return nil
	}
	out := []lang.Target{}
	seen := map[string]bool{t.Package: true, projectOf(name): true} // its own secondary systems
	for _, d := range s.dependencies {
		dependencyTarget := r.systemTarget("", d, "", s.pins)
		if dependencyTarget.Ecosystem == ecosystemQuicklisp && !seen[dependencyTarget.Package] {
			seen[dependencyTarget.Package] = true
			out = append(out, dependencyTarget)
		}
	}
	return out
}

// Installed says a project's dependencies come from what Qlot or ocicl
// installed.
func (r *resolver) Installed(t lang.Target) bool { return t.Ecosystem == ecosystemQuicklisp }

// index records the packages a file defines.
func (r *resolver) index(file string, in *info) {
	for _, p := range in.packages {
		if p.nicks != nil && r.nicknames[p.names[0]] == nil {
			r.nicknames[p.names[0]] = p.nicks
		}
		for _, n := range p.names {
			if !slices.Contains(r.packages[n], file) {
				r.packages[n] = append(r.packages[n], file)
			}
		}
	}
}

// mentionsPackage reports whether a source may define a package.
func mentionsPackage(source []byte) bool {
	for _, w := range [][]byte{[]byte("defpackage"), []byte("define-package"), []byte("DEFPACKAGE"), []byte("DEFINE-PACKAGE")} {
		if bytes.Contains(source, w) {
			return true
		}
	}
	return bytes.Contains(bytes.ToLower(source), []byte("defpackage"))
}

// readPins reads a qlfile, qlfile.lock or ocicl.csv into its directory's
// pins.
func (r *resolver) readPins(f *scan.File) {
	source, err := os.ReadFile(f.AbsolutePath)
	if err != nil {
		return
	}
	directory := path.Dir(f.Path)
	p := r.pins[directory]
	if p == nil {
		p = &pins{directory: directory, ql: map[string]qlEntry{}, lock: map[string]lockEntry{}, ocicl: map[string]ociclEntry{}, ociclProject: map[string]ociclEntry{}}
		r.pins[directory] = p
	}
	switch path.Base(f.Path) {
	case "qlfile":
		q := readQlfile(source)
		for _, e := range q.entries {
			if _, duplicate := p.ql[e.name]; !duplicate {
				p.ql[e.name] = e
			}
		}
		p.qlDist = q.dist
	case "qlfile.lock":
		l := readLock(source)
		for _, e := range l.entries {
			p.lock[e.name] = e
		}
		if n := len(l.dists); n > 0 {
			p.lockDist = l.dists[n-1].version
		}
	case "ocicl.csv":
		for _, e := range readOcicl(source) {
			p.ocicl[e.system] = e
			if _, duplicate := p.ociclProject[e.project]; !duplicate {
				p.ociclProject[e.project] = e
			}
		}
	}
}

// governing lists the pins that apply to a file: those of the nearest
// directory at or above it that has any, else all of the repository's,
// shallowest first.
func (r *resolver) governing(file string) []*pins {
	if p, ok := lang.Nearest(r.pins, file); ok {
		return []*pins{p}
	}
	out := make([]*pins, 0, len(r.pinDirectories))
	for _, d := range r.pinDirectories {
		out = append(out, r.pins[d])
	}
	return out
}

// Implements: REQ-COMMONLISP-004, REQ-COMMONLISP-005, REQ-COMMONLISP-006, REQ-COMMONLISP-007, REQ-COMMONLISP-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, version, _ := strings.Cut(rawImport.Name, "\x00")
	switch kind {
	case kindComponent:
		p := path.Join(path.Dir(file), rawImport.Module)
		if r.files[p] && p != file {
			return lang.Target{Local: p}
		}
		// A component class of the system's own (a static file with a type
		// of its own, say) has a file type the .asd sets in code: the one
		// file of that name is it.
		if c := r.stems[strings.TrimSuffix(p, path.Ext(p))]; len(c) == 1 && c[0] != file {
			return lang.Target{Local: c[0]}
		}
	case kindLoad:
		for _, p := range []string{path.Join(path.Dir(file), rawImport.Module), path.Clean(rawImport.Module)} {
			if strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
				continue
			}
			for _, c := range []string{p, p + ".lisp"} {
				if r.files[c] && c != file {
					return lang.Target{Local: c}
				}
			}
		}
	case kindSystem:
		return r.systemTarget(file, rawImport.Module, version, nil)
	case kindRequire:
		// require loads an implementation's module (SBCL's contribs, ABCL's
		// and CMUCL's gray-streams) or, through ASDF's hook, a system.
		if s, ok := stdSystem(rawImport.Module); ok {
			return lang.Target{Ecosystem: ecosystemStd, Package: s}
		}
		if s, ok := implementationModules[rawImport.Module]; ok {
			return lang.Target{Ecosystem: ecosystemStd, Package: s}
		}
		if strings.Contains(rawImport.Module, ".") && !r.declarationOf(file).set[rawImport.Module] {
			return lang.Target{} // a file: (require "streamc.fasl")
		}
		return r.systemTarget(file, rawImport.Module, "", nil)
	case kindQlfile, kindLock, kindOcicl:
		p := r.pins[path.Dir(file)]
		if p == nil {
			break
		}
		if kind == kindQlfile {
			if e, ok := p.ql[rawImport.Module]; ok && e.source == "local" {
				return r.localSource(e, p.directory)
			}
		}
		return r.systemTarget(file, rawImport.Module, "", p)
	case kindReference:
		// A local nickname the file's package defines elsewhere.
		m := rawImport.Module
		if t, ok := r.nicknames[version][m]; ok && len(r.packages[m]) == 0 {
			m = t
		}
		return r.packageTarget(file, m, kind)
	case kindPackage, kindInPackage:
		return r.packageTarget(file, rawImport.Module, kind)
	}
	return lang.Target{}
}

// localSystem resolves a system of the repository: the .asd defining it,
// or for a package-inferred system's foo/bar/baz the file bar/baz.lisp under
// foo's root. ok is false for a system the repository does not define.
func (r *resolver) localSystem(file, system string) (lang.Target, bool) {
	if reference := r.systems[system]; reference != nil {
		if reference.asd == file {
			return lang.Target{}, true
		}
		return lang.Target{Local: reference.asd}, true
	}
	primary, rest, slash := strings.Cut(system, "/")
	reference := r.systems[primary]
	if !slash || reference == nil {
		return lang.Target{}, false
	}
	if reference.system.pis && rest != "" {
		for _, extension := range []string{".lisp", ".lsp", ".cl"} {
			if p := path.Join(reference.root(), rest) + extension; r.files[p] {
				if p == file {
					return lang.Target{}, true
				}
				return lang.Target{Local: p}, true
			}
		}
	}
	if reference.asd == file {
		return lang.Target{}, true
	}
	return lang.Target{Local: reference.asd}, true
}

// systemTarget resolves an ASDF system: one of the repository's, one the
// implementation provides, else the Quicklisp project releasing it, pinned
// as the governing qlfile, lock or ocicl.csv say (from, when set, instead).
//
// Implements: REQ-COMMONLISP-005, REQ-COMMONLISP-006
func (r *resolver) systemTarget(file, system, version string, from *pins) lang.Target {
	if t, ok := r.localSystem(file, system); ok {
		return t
	}
	if s, ok := stdSystem(system); ok {
		return lang.Target{Ecosystem: ecosystemStd, Package: s}
	}
	project := projectOf(system)
	sets := []*pins{from}
	if from == nil {
		sets = r.governing(file)
	}
	for _, p := range sets {
		if t, ok := r.pinned(p, system, project); ok {
			return t
		}
	}
	return lang.Target{Ecosystem: ecosystemQuicklisp, Package: project, Version: version, Floating: true}
}

// pinned is the target a directory's pins give a system, if they list it or
// fix a dist version.
//
// Implements: REQ-COMMONLISP-006
func (r *resolver) pinned(p *pins, system, project string) (lang.Target, bool) {
	if e, ok := p.ocicl[system]; ok {
		return ociclTarget(e), true
	}
	if e, ok := p.ociclProject[project]; ok {
		return ociclTarget(e), true
	}
	q, hasQ := p.ql[project]
	if !hasQ {
		q, hasQ = p.ql[system]
	}
	e, hasL := p.lock[project]
	if !hasL {
		e, hasL = p.lock[system]
	}
	switch {
	case hasL:
		var asked *qlEntry
		if hasQ {
			asked = &q
		}
		return lockTarget(e, asked, project), true
	case hasQ && q.source == "local":
		return r.localSource(q, p.directory), true
	case hasQ:
		return qlTarget(q, project), true
	case p.lockDist != "":
		return lang.Target{Ecosystem: ecosystemQuicklisp, Package: project, Version: p.lockDist, Pinned: true}, true
	case p.qlDist != "":
		return lang.Target{Ecosystem: ecosystemQuicklisp, Package: project, Version: p.qlDist, Pinned: true}, true
	}
	return lang.Target{}, false
}

func ociclTarget(e ociclEntry) lang.Target {
	v := e.version()
	if v == "" {
		v = e.digest
	}
	return lang.Target{Ecosystem: ecosystemQuicklisp, Package: e.project, Version: v, Pinned: e.digest != ""}
}

// gitName names a git source by its repository URL.
func gitName(url, fallback string) string {
	if url == "" {
		return fallback
	}
	return lang.RepositoryName(url)
}

// lockTarget is what a qlfile.lock records for a project: the dist version
// of a Quicklisp (or other dist's) project, the commit of a git one. asked
// is the qlfile's entry, if any, for Requested.
func lockTarget(e lockEntry, asked *qlEntry, project string) lang.Target {
	requested := e.branch
	if asked != nil {
		switch {
		case asked.version != "":
			requested = asked.version
		case asked.tag != "":
			requested = asked.tag
		case asked.branch != "":
			requested = asked.branch
		case asked.reference != "":
			requested = asked.reference
		}
	}
	requested = strings.TrimPrefix(requested, ":")
	switch {
	case strings.Contains(e.class, "source-ql"), strings.Contains(e.class, "ultralisp"):
		t := lang.Target{Ecosystem: ecosystemQuicklisp, Package: project, Pinned: true}
		switch {
		case e.commit != "": // ql <project> :upstream
			t.Version = e.commit
		default:
			v := e.version
			for _, prerelease := range []string{"ql-dist-", "ql-upstream-", "ultralisp-", "ql-"} {
				if s, ok := strings.CutPrefix(v, prerelease); ok {
					v = s
					break
				}
			}
			t.Version = v
			t.Pinned = v != ""
		}
		if requested != t.Version {
			t.Requested = requested
		}
		return t
	case e.commit != "":
		t := lang.Target{Ecosystem: ecosystemQuicklisp, Package: gitName(e.url, project), Version: e.commit, Pinned: true}
		if requested != e.commit {
			t.Requested = requested
		}
		if e.url != "" && !lang.PublicForge(e.url) {
			t.Origin = e.url
		}
		return t
	}
	t := lang.Target{Ecosystem: ecosystemQuicklisp, Package: project, Version: e.version, Pinned: e.version != ""}
	if e.url != "" && !lang.PublicForge(e.url) {
		t.Origin = e.url
	}
	return t
}

// qlTarget is what a qlfile entry alone (no lock) says: a dist version
// pins, :latest floats; a git commit pins, a tag is shown, a branch or
// nothing floats; an http tarball with its md5 pins.
func qlTarget(e qlEntry, project string) lang.Target {
	switch e.source {
	case "git", "github":
		t := lang.Target{Ecosystem: ecosystemQuicklisp, Package: gitName(e.url, project)}
		switch {
		case lang.Commit(e.reference):
			t.Version, t.Pinned = e.reference, true
		case e.tag != "":
			t.Version = e.tag
		case e.branch != "":
			t.Version, t.Floating = e.branch, true
		case e.reference != "":
			t.Version, t.Floating = e.reference, true
		default:
			t.Floating = true
		}
		if !lang.PublicForge(e.url) {
			t.Origin = e.url
		}
		return t
	case "http":
		t := lang.Target{Ecosystem: ecosystemQuicklisp, Package: project, Origin: e.url}
		if e.md5 != "" {
			t.Version, t.Pinned = e.md5, true
		} else {
			t.Floating = true
		}
		return t
	}
	if dated(e.version) {
		return lang.Target{Ecosystem: ecosystemQuicklisp, Package: project, Version: e.version, Pinned: true}
	}
	return lang.Target{Ecosystem: ecosystemQuicklisp, Package: project, Floating: true}
}

// localSource is a qlfile's local <name> <directory>: the directory when
// it is in the repository, else a package from that path.
func (r *resolver) localSource(e qlEntry, directory string) lang.Target {
	if !path.IsAbs(e.url) && !strings.HasPrefix(e.url, "~") {
		if p := path.Join(directory, e.url); r.directories[p] && !strings.HasPrefix(p, "../") && p != "." {
			return lang.Target{Local: p}
		}
	}
	return lang.Target{Ecosystem: ecosystemQuicklisp, Package: e.name, Floating: true, Origin: e.url}
}

// declaration is memoized per file: the systems that include it (or the
// nearest .asd files' systems, else all) and what they depend on.
func (r *resolver) declarationOf(file string) *declaration {
	if d, ok := r.declared.Load(file); ok {
		return d.(*declaration)
	}
	d := &declaration{set: map[string]bool{}}
	references := r.fileSystems[file]
	if strings.EqualFold(path.Ext(file), ".asd") {
		references = r.asdDirectories[path.Dir(file)]
	}
	if len(references) == 0 {
		for _, reference := range r.all {
			if reference.system.pis && lang.Within(file, reference.root()) {
				references = append(references, reference)
			}
		}
	}
	for _, reference := range references {
		d.pis = d.pis || reference.system.pis
	}
	if len(references) == 0 {
		for directory := range lang.Ancestors(file) {
			if references = r.asdDirectories[directory]; len(references) > 0 {
				break
			}
		}
	}
	if len(references) == 0 {
		references = r.all
	}
	// What a system of the repository depends on is loaded with the
	// systems depending on it: the closure through local systems counts.
	// references is a copy: appending to r.all or r.asdDirs' slices would write
	// into memory other files' resolutions read.
	references = slices.Clone(references)
	seen := map[*sysReference]bool{}
	for len(references) > 0 && len(seen) < 4096 {
		reference := references[0]
		references = references[1:]
		if seen[reference] {
			continue
		}
		seen[reference] = true
		for _, dependency := range reference.system.dependencies {
			d.set[dependency.name] = true
			primary, _, _ := strings.Cut(dependency.name, "/")
			for _, n := range []string{dependency.name, primary} {
				if l := r.systems[n]; l != nil && !seen[l] {
					references = append(references, l)
				}
			}
		}
	}
	for _, p := range r.governing(file) {
		for n := range p.ql {
			d.set[n] = true
		}
		for n := range p.lock {
			d.set[n] = true
		}
		for n, e := range p.ocicl {
			d.set[n] = true
			d.set[e.project] = true
		}
	}
	for n := range d.set {
		d.names = append(d.names, n)
	}
	sort.Strings(d.names)
	v, _ := r.declared.LoadOrStore(file, d)
	return v.(*declaration)
}

// fold is a name without case, a cl- prefix or -cl suffix and punctuation:
// cl-json and json fold alike, and tree-sitter-cl and tree-sitter.
func fold(s string) string {
	s = strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(s), "cl-"), "-cl")
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '.' || r == '_' || r == '/' {
			return -1
		}
		return r
	}, s)
}

// match finds the declared system a package belongs to: the one
// register-system-packages or the curated table names, the package's own
// name (or its primary system's, for foo/bar), the declared name folding
// alike (json and cl-json), or the declared system the package's dotted or
// slashed prefix names (lack.request is lack's).
func (r *resolver) match(packageName string, d *declaration) (string, bool) {
	var candidates []string
	if s, ok := r.registered[packageName]; ok {
		candidates = append(candidates, s)
	}
	if s, ok := packageSystem(packageName); ok {
		candidates = append(candidates, s)
	}
	primary, _, _ := strings.Cut(packageName, "/")
	candidates = append(candidates, packageName, primary)
	for _, c := range candidates {
		if d.set[c] {
			return c, true
		}
		for _, n := range d.names {
			if projectOf(n) == c && !strings.Contains(n, "/") {
				return n, true
			}
		}
	}
	f := fold(packageName)
	for _, n := range d.names {
		if fold(n) == f {
			return n, true
		}
	}
	best := ""
	for _, n := range d.names {
		if (strings.HasPrefix(packageName, n+".") || strings.HasPrefix(packageName, n+"/")) && len(n) > len(best) {
			best = n
		}
	}
	return best, best != ""
}

// packageTarget resolves a package a file names (defpackage options,
// in-package, a qualified symbol): the file of the repository defining it,
// the implementation's, a package-inferred system's file, the declared
// system it belongs to; else, for defpackage options, the system a curated
// table or (in a package-inferred system, where a package is its system)
// the package's name gives, unresolved unless the package-inferred system
// declares it by importing it. in-package and qualified symbols of packages
// nothing declares are dropped.
//
// Implements: REQ-COMMONLISP-004, REQ-COMMONLISP-007, REQ-COMMONLISP-008
func (r *resolver) packageTarget(file, packageName, kind string) lang.Target {
	if files := r.packages[packageName]; len(files) > 0 {
		if slices.Contains(files, file) {
			return lang.Target{}
		}
		best := files[0]
		for _, f := range files[1:] {
			if shared(f, file) > shared(best, file) {
				best = f
			}
		}
		return lang.Target{Local: best}
	}
	if s, ok := stdPackage(packageName); ok {
		return lang.Target{Ecosystem: ecosystemStd, Package: s}
	}
	primary, _, _ := strings.Cut(packageName, "/")
	if r.systems[packageName] != nil || r.systems[primary] != nil {
		t, _ := r.localSystem(file, packageName)
		return t
	}
	d := r.declarationOf(file)
	if system, ok := r.match(packageName, d); ok {
		return r.systemTarget(file, system, "", nil)
	}
	if kind != kindPackage {
		return lang.Target{}
	}
	system, known := packageSystem(packageName)
	if s, ok := r.registered[packageName]; ok {
		system, known = s, true
	}
	if !known {
		system = packageName
	}
	if d.pis {
		return r.systemTarget(file, system, "", nil)
	}
	return lang.Target{Ecosystem: ecosystemQuicklisp, Package: projectOf(system), Unresolved: true}
}

// shared is the length of the common directory prefix of two paths.
func shared(a, b string) int {
	n := 0
	for i := 0; i < len(a) && i < len(b) && a[i] == b[i]; i++ {
		if a[i] == '/' {
			n = i
		}
	}
	return n
}
