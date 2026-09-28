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

// sysRef is a system some .asd file of the repository defines.
type sysRef struct {
	sys *system
	asd string // the .asd file
	dir string // its directory
}

// root is the directory the system's components and package-inferred files
// are relative to.
func (s *sysRef) root() string { return path.Join(s.dir, s.sys.base) }

// pins is what the qlfile, qlfile.lock and ocicl.csv of one directory fix.
type pins struct {
	dir       string
	ql        map[string]qlEntry
	lock      map[string]lockEntry
	lockDist  string                // the dist version of the lock's last dist (the one with priority)
	qlDist    string                // ql :all <version>, or dist <quicklisp> <version>
	ocicl     map[string]ociclEntry // by system
	ociclProj map[string]ociclEntry // by project
}

type resolver struct {
	files map[string]bool
	dirs  map[string]bool // directories holding files
	// systems are the repository's systems by name; fileSystems the systems
	// whose components include a file; asdDirs the systems defined in each
	// directory's .asd files.
	systems     map[string]*sysRef
	fileSystems map[string][]*sysRef
	asdDirs     map[string][]*sysRef
	all         []*sysRef
	// packages are the files defining each package of the repository (by
	// name and nickname), sorted.
	packages   map[string][]string
	nicknames  map[string]map[string]string // package -> its local nicknames
	stems      map[string][]string          // path without extension -> files
	registered map[string]string            // package -> system, asdf:register-system-packages
	pins       map[string]*pins
	pinDirs    []string // shallowest first
	declared   sync.Map // file -> *declaration
	// installed are the systems Qlot installed into .qlot/ and ocicl into
	// systems/, by name, and installedNames their names, sorted.
	installed      map[string]*installedSystem
	installedNames []string
}

// installedSystem is a system Qlot or ocicl installed: the systems it depends
// on and the pins of the directory it was installed for.
type installedSystem struct {
	deps []string
	pins *pins
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
		if s != "systems" || f.Abs == "" {
			continue
		}
		abs := strings.ReplaceAll(f.Abs, "\\", "/")
		if !strings.HasSuffix(abs, f.Path) {
			continue
		}
		dir := abs[:len(abs)-len(f.Path)] + strings.Join(segments[:i], "/")
		if besideOcicl(dir) {
			return true
		}
	}
	return false
}

var ociclDirs sync.Map // absolute directory -> bool

func besideOcicl(dir string) bool {
	if v, ok := ociclDirs.Load(dir); ok {
		return v.(bool)
	}
	_, err := os.Stat(path.Join(dir, "ocicl.csv"))
	ociclDirs.Store(dir, err == nil)
	return err == nil
}

// readable reports whether the resolver may read a file's content.
func readable(f *scan.File) bool {
	return !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize
}

// newResolver reads the repository's .asd files for its systems, the
// sources defining packages for the package index, the qlfile, lock and
// ocicl.csv files for pins, and the .asd files of what Qlot and ocicl
// installed beside them.
//
// Implements: REQ-COMMONLISP-005, REQ-COMMONLISP-006, REQ-COMMONLISP-011, REQ-COMMONLISP-012
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{
		files: map[string]bool{}, dirs: map[string]bool{}, systems: map[string]*sysRef{},
		fileSystems: map[string][]*sysRef{}, asdDirs: map[string][]*sysRef{},
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
			if r.dirs[d] {
				break
			}
			r.dirs[d] = true
			if d == "." || d == "/" {
				break
			}
		}
		if !readable(f) {
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
		src, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		in := read(src)
		dir := path.Dir(f.Path)
		for _, s := range in.systems {
			ref := &sysRef{sys: s, asd: f.Path, dir: dir}
			r.all = append(r.all, ref)
			r.asdDirs[dir] = append(r.asdDirs[dir], ref)
			if _, dup := r.systems[s.name]; !dup {
				r.systems[s.name] = ref
			}
			for _, c := range s.files {
				p := path.Join(dir, c)
				r.fileSystems[p] = append(r.fileSystems[p], ref)
			}
		}
		for p, s := range in.registered {
			r.registered[p] = s
		}
		r.index(f.Path, in)
	}
	// Package definitions: only files that mention one are read.
	var mu sync.Mutex
	lang.ForEachFile(context.Background(), sources, func(f *scan.File, src []byte) *lang.FileResult {
		if !mentionsPackage(src) {
			return nil
		}
		in := read(src)
		mu.Lock()
		r.index(f.Path, in)
		mu.Unlock()
		return nil
	})
	for _, files := range r.packages {
		sort.Strings(files)
	}
	for d := range r.pins {
		r.pinDirs = append(r.pinDirs, d)
	}
	sort.Slice(r.pinDirs, func(i, j int) bool {
		a, b := strings.Count(r.pinDirs[i], "/"), strings.Count(r.pinDirs[j], "/")
		if a != b {
			return a < b
		}
		return r.pinDirs[i] < r.pinDirs[j]
	})
	for _, d := range r.pinDirs {
		if root == "" {
			break
		}
		p, base := r.pins[d], filepath.Join(root, filepath.FromSlash(d))
		if len(p.ql) > 0 || len(p.lock) > 0 {
			r.readInstalled(filepath.Join(base, ".qlot", "dists"), p)
		}
		if len(p.ocicl) > 0 {
			r.readInstalled(filepath.Join(base, "systems"), p)
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
func (r *resolver) readInstalled(dir string, p *pins) {
	n := 0
	filepath.WalkDir(dir, func(f string, e fs.DirEntry, err error) error {
		if n++; err != nil || n > maxInstalledFiles {
			return nil
		}
		if e.IsDir() || !strings.EqualFold(filepath.Ext(f), ".asd") {
			return nil
		}
		if info, err := e.Info(); err != nil || info.Size() > lang.MaxParseSize {
			return nil
		}
		src, err := os.ReadFile(f)
		if err != nil {
			return nil
		}
		for _, s := range read(src).systems {
			if r.installed[s.name] != nil {
				continue
			}
			in := &installedSystem{pins: p}
			for _, d := range s.deps {
				if !d.require && d.option != "weakly-depends-on" {
					in.deps = append(in.deps, d.name)
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
	if t.Ecosystem != ecoQuicklisp {
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
	for _, d := range s.deps {
		dt := r.systemTarget("", d, "", s.pins)
		if dt.Ecosystem == ecoQuicklisp && !seen[dt.Package] {
			seen[dt.Package] = true
			out = append(out, dt)
		}
	}
	return out
}

// Installed says a project's dependencies come from what Qlot or ocicl
// installed.
func (r *resolver) Installed(t lang.Target) bool { return t.Ecosystem == ecoQuicklisp }

// index records the packages a file defines.
func (r *resolver) index(file string, in *info) {
	for _, p := range in.packages {
		if p.nicks != nil && r.nicknames[p.names[0]] == nil {
			r.nicknames[p.names[0]] = p.nicks
		}
		for _, n := range p.names {
			if !contains(r.packages[n], file) {
				r.packages[n] = append(r.packages[n], file)
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// mentionsPackage reports whether a source may define a package.
func mentionsPackage(src []byte) bool {
	for _, w := range [][]byte{[]byte("defpackage"), []byte("define-package"), []byte("DEFPACKAGE"), []byte("DEFINE-PACKAGE")} {
		if bytes.Contains(src, w) {
			return true
		}
	}
	return bytes.Contains(bytes.ToLower(src), []byte("defpackage"))
}

// readPins reads a qlfile, qlfile.lock or ocicl.csv into its directory's
// pins.
func (r *resolver) readPins(f *scan.File) {
	src, err := os.ReadFile(f.Abs)
	if err != nil {
		return
	}
	dir := path.Dir(f.Path)
	p := r.pins[dir]
	if p == nil {
		p = &pins{dir: dir, ql: map[string]qlEntry{}, lock: map[string]lockEntry{}, ocicl: map[string]ociclEntry{}, ociclProj: map[string]ociclEntry{}}
		r.pins[dir] = p
	}
	switch path.Base(f.Path) {
	case "qlfile":
		q := readQlfile(src)
		for _, e := range q.entries {
			if _, dup := p.ql[e.name]; !dup {
				p.ql[e.name] = e
			}
		}
		p.qlDist = q.dist
	case "qlfile.lock":
		l := readLock(src)
		for _, e := range l.entries {
			p.lock[e.name] = e
		}
		if n := len(l.dists); n > 0 {
			p.lockDist = l.dists[n-1].version
		}
	case "ocicl.csv":
		for _, e := range readOcicl(src) {
			p.ocicl[e.system] = e
			if _, dup := p.ociclProj[e.project]; !dup {
				p.ociclProj[e.project] = e
			}
		}
	}
}

// governing lists the pins that apply to a file: those of the nearest
// directory at or above it that has any, else all of the repository's,
// shallowest first.
func (r *resolver) governing(file string) []*pins {
	for d := path.Dir(file); ; d = path.Dir(d) {
		if p := r.pins[d]; p != nil {
			return []*pins{p}
		}
		if d == "." || d == "/" {
			break
		}
	}
	out := make([]*pins, 0, len(r.pinDirs))
	for _, d := range r.pinDirs {
		out = append(out, r.pins[d])
	}
	return out
}

// Implements: REQ-COMMONLISP-004, REQ-COMMONLISP-005, REQ-COMMONLISP-006, REQ-COMMONLISP-007, REQ-COMMONLISP-008
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, version, _ := strings.Cut(imp.Name, "\x00")
	switch kind {
	case kindComponent:
		p := path.Join(path.Dir(file), imp.Module)
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
		for _, p := range []string{path.Join(path.Dir(file), imp.Module), path.Clean(imp.Module)} {
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
		return r.systemTarget(file, imp.Module, version, nil)
	case kindRequire:
		// require loads an implementation's module (SBCL's contribs, ABCL's
		// and CMUCL's gray-streams) or, through ASDF's hook, a system.
		if s, ok := stdSystem(imp.Module); ok {
			return lang.Target{Ecosystem: ecoStd, Package: s}
		}
		if s, ok := implementationModules[imp.Module]; ok {
			return lang.Target{Ecosystem: ecoStd, Package: s}
		}
		if strings.Contains(imp.Module, ".") && !r.declarationOf(file).set[imp.Module] {
			return lang.Target{} // a file: (require "streamc.fasl")
		}
		return r.systemTarget(file, imp.Module, "", nil)
	case kindQlfile, kindLock, kindOcicl:
		p := r.pins[path.Dir(file)]
		if p == nil {
			break
		}
		if kind == kindQlfile {
			if e, ok := p.ql[imp.Module]; ok && e.source == "local" {
				return r.localSource(e, p.dir)
			}
		}
		return r.systemTarget(file, imp.Module, "", p)
	case kindRef:
		// A local nickname the file's package defines elsewhere.
		m := imp.Module
		if t, ok := r.nicknames[version][m]; ok && len(r.packages[m]) == 0 {
			m = t
		}
		return r.packageTarget(file, m, kind)
	case kindPackage, kindInPackage:
		return r.packageTarget(file, imp.Module, kind)
	}
	return lang.Target{}
}

// localSystem resolves a system of the repository: the .asd defining it,
// or for a package-inferred system's foo/bar/baz the file bar/baz.lisp under
// foo's root. ok is false for a system the repository does not define.
func (r *resolver) localSystem(file, sys string) (lang.Target, bool) {
	if ref := r.systems[sys]; ref != nil {
		if ref.asd == file {
			return lang.Target{}, true
		}
		return lang.Target{Local: ref.asd}, true
	}
	primary, rest, slash := strings.Cut(sys, "/")
	ref := r.systems[primary]
	if !slash || ref == nil {
		return lang.Target{}, false
	}
	if ref.sys.pis && rest != "" {
		for _, ext := range []string{".lisp", ".lsp", ".cl"} {
			if p := path.Join(ref.root(), rest) + ext; r.files[p] {
				if p == file {
					return lang.Target{}, true
				}
				return lang.Target{Local: p}, true
			}
		}
	}
	if ref.asd == file {
		return lang.Target{}, true
	}
	return lang.Target{Local: ref.asd}, true
}

// systemTarget resolves an ASDF system: one of the repository's, one the
// implementation provides, else the Quicklisp project releasing it, pinned
// as the governing qlfile, lock or ocicl.csv say (from, when set, instead).
//
// Implements: REQ-COMMONLISP-005, REQ-COMMONLISP-006
func (r *resolver) systemTarget(file, sys, version string, from *pins) lang.Target {
	if t, ok := r.localSystem(file, sys); ok {
		return t
	}
	if s, ok := stdSystem(sys); ok {
		return lang.Target{Ecosystem: ecoStd, Package: s}
	}
	project := projectOf(sys)
	sets := []*pins{from}
	if from == nil {
		sets = r.governing(file)
	}
	for _, p := range sets {
		if t, ok := r.pinned(p, sys, project); ok {
			return t
		}
	}
	return lang.Target{Ecosystem: ecoQuicklisp, Package: project, Version: version, Floating: true}
}

// pinned is the target a directory's pins give a system, if they list it or
// fix a dist version.
//
// Implements: REQ-COMMONLISP-006
func (r *resolver) pinned(p *pins, sys, project string) (lang.Target, bool) {
	if e, ok := p.ocicl[sys]; ok {
		return ociclTarget(e), true
	}
	if e, ok := p.ociclProj[project]; ok {
		return ociclTarget(e), true
	}
	q, hasQ := p.ql[project]
	if !hasQ {
		q, hasQ = p.ql[sys]
	}
	e, hasL := p.lock[project]
	if !hasL {
		e, hasL = p.lock[sys]
	}
	switch {
	case hasL:
		var asked *qlEntry
		if hasQ {
			asked = &q
		}
		return lockTarget(e, asked, project), true
	case hasQ && q.source == "local":
		return r.localSource(q, p.dir), true
	case hasQ:
		return qlTarget(q, project), true
	case p.lockDist != "":
		return lang.Target{Ecosystem: ecoQuicklisp, Package: project, Version: p.lockDist, Pinned: true}, true
	case p.qlDist != "":
		return lang.Target{Ecosystem: ecoQuicklisp, Package: project, Version: p.qlDist, Pinned: true}, true
	}
	return lang.Target{}, false
}

func ociclTarget(e ociclEntry) lang.Target {
	v := e.version()
	if v == "" {
		v = e.digest
	}
	return lang.Target{Ecosystem: ecoQuicklisp, Package: e.project, Version: v, Pinned: e.digest != ""}
}

// gitName names a git source by its repository URL.
func gitName(url, fallback string) string {
	if url == "" {
		return fallback
	}
	return lang.RepoName(url)
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
		case asked.ref != "":
			requested = asked.ref
		}
	}
	requested = strings.TrimPrefix(requested, ":")
	switch {
	case strings.Contains(e.class, "source-ql"), strings.Contains(e.class, "ultralisp"):
		t := lang.Target{Ecosystem: ecoQuicklisp, Package: project, Pinned: true}
		switch {
		case e.commit != "": // ql <project> :upstream
			t.Version = e.commit
		default:
			v := e.version
			for _, pre := range []string{"ql-dist-", "ql-upstream-", "ultralisp-", "ql-"} {
				if s, ok := strings.CutPrefix(v, pre); ok {
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
		t := lang.Target{Ecosystem: ecoQuicklisp, Package: gitName(e.url, project), Version: e.commit, Pinned: true}
		if requested != e.commit {
			t.Requested = requested
		}
		if e.url != "" && !public(e.url) {
			t.Origin = e.url
		}
		return t
	}
	t := lang.Target{Ecosystem: ecoQuicklisp, Package: project, Version: e.version, Pinned: e.version != ""}
	if e.url != "" && !public(e.url) {
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
		t := lang.Target{Ecosystem: ecoQuicklisp, Package: gitName(e.url, project)}
		switch {
		case lang.Commit(e.ref):
			t.Version, t.Pinned = e.ref, true
		case e.tag != "":
			t.Version = e.tag
		case e.branch != "":
			t.Version, t.Floating = e.branch, true
		case e.ref != "":
			t.Version, t.Floating = e.ref, true
		default:
			t.Floating = true
		}
		if !public(e.url) {
			t.Origin = e.url
		}
		return t
	case "http":
		t := lang.Target{Ecosystem: ecoQuicklisp, Package: project, Origin: e.url}
		if e.md5 != "" {
			t.Version, t.Pinned = e.md5, true
		} else {
			t.Floating = true
		}
		return t
	}
	if dated(e.version) {
		return lang.Target{Ecosystem: ecoQuicklisp, Package: project, Version: e.version, Pinned: true}
	}
	return lang.Target{Ecosystem: ecoQuicklisp, Package: project, Floating: true}
}

// localSource is a qlfile's local <name> <directory>: the directory when
// it is in the repository, else a package from that path.
func (r *resolver) localSource(e qlEntry, dir string) lang.Target {
	if !path.IsAbs(e.url) && !strings.HasPrefix(e.url, "~") {
		if p := path.Join(dir, e.url); r.dirs[p] && !strings.HasPrefix(p, "../") && p != "." {
			return lang.Target{Local: p}
		}
	}
	return lang.Target{Ecosystem: ecoQuicklisp, Package: e.name, Floating: true, Origin: e.url}
}

// public reports whether a git URL is on a public forge, whose projects are
// named, not origins.
func public(url string) bool {
	host, _, _ := strings.Cut(lang.RepoName(url), "/")
	switch host {
	case "github.com", "gitlab.com", "bitbucket.org", "codeberg.org", "git.sr.ht", "sr.ht":
		return true
	}
	return false
}

// declaration is memoized per file: the systems that include it (or the
// nearest .asd files' systems, else all) and what they depend on.
func (r *resolver) declarationOf(file string) *declaration {
	if d, ok := r.declared.Load(file); ok {
		return d.(*declaration)
	}
	d := &declaration{set: map[string]bool{}}
	refs := r.fileSystems[file]
	if strings.EqualFold(path.Ext(file), ".asd") {
		refs = r.asdDirs[path.Dir(file)]
	}
	if len(refs) == 0 {
		for _, ref := range r.all {
			if ref.sys.pis && under(file, ref.root()) {
				refs = append(refs, ref)
			}
		}
	}
	for _, ref := range refs {
		d.pis = d.pis || ref.sys.pis
	}
	if len(refs) == 0 {
		for dir := path.Dir(file); ; dir = path.Dir(dir) {
			if refs = r.asdDirs[dir]; len(refs) > 0 || dir == "." || dir == "/" {
				break
			}
		}
	}
	if len(refs) == 0 {
		refs = r.all
	}
	// What a system of the repository depends on is loaded with the
	// systems depending on it: the closure through local systems counts.
	// refs is a copy: appending to r.all or r.asdDirs' slices would write
	// into memory other files' resolutions read.
	refs = slices.Clone(refs)
	seen := map[*sysRef]bool{}
	for len(refs) > 0 && len(seen) < 4096 {
		ref := refs[0]
		refs = refs[1:]
		if seen[ref] {
			continue
		}
		seen[ref] = true
		for _, dep := range ref.sys.deps {
			d.set[dep.name] = true
			primary, _, _ := strings.Cut(dep.name, "/")
			for _, n := range []string{dep.name, primary} {
				if l := r.systems[n]; l != nil && !seen[l] {
					refs = append(refs, l)
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

func under(file, dir string) bool {
	return dir == "." || strings.HasPrefix(file, dir+"/")
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
func (r *resolver) match(pkg string, d *declaration) (string, bool) {
	var candidates []string
	if s, ok := r.registered[pkg]; ok {
		candidates = append(candidates, s)
	}
	if s, ok := packageSystem(pkg); ok {
		candidates = append(candidates, s)
	}
	primary, _, _ := strings.Cut(pkg, "/")
	candidates = append(candidates, pkg, primary)
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
	f := fold(pkg)
	for _, n := range d.names {
		if fold(n) == f {
			return n, true
		}
	}
	best := ""
	for _, n := range d.names {
		if (strings.HasPrefix(pkg, n+".") || strings.HasPrefix(pkg, n+"/")) && len(n) > len(best) {
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
func (r *resolver) packageTarget(file, pkg, kind string) lang.Target {
	if files := r.packages[pkg]; len(files) > 0 {
		if contains(files, file) {
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
	if s, ok := stdPackage(pkg); ok {
		return lang.Target{Ecosystem: ecoStd, Package: s}
	}
	primary, _, _ := strings.Cut(pkg, "/")
	if r.systems[pkg] != nil || r.systems[primary] != nil {
		t, _ := r.localSystem(file, pkg)
		return t
	}
	d := r.declarationOf(file)
	if sys, ok := r.match(pkg, d); ok {
		return r.systemTarget(file, sys, "", nil)
	}
	if kind != kindPackage {
		return lang.Target{}
	}
	sys, known := packageSystem(pkg)
	if s, ok := r.registered[pkg]; ok {
		sys, known = s, true
	}
	if !known {
		sys = pkg
	}
	if d.pis {
		return r.systemTarget(file, sys, "", nil)
	}
	return lang.Target{Ecosystem: ecoQuicklisp, Package: projectOf(sys), Unresolved: true}
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
