package haxe

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// decl is a library a manifest declares, with the version it asks for ("" for
// any).
type decl struct {
	name, version string
}

// manifest is what one build file (.hxml, haxelib.json, project file) says.
type manifest struct {
	file  string
	libs  []decl
	cps   []string // class paths, relative to the repository
	owner string   // a haxelib.json's own library name
}

type resolver struct {
	files     map[string]bool
	dirs      map[string]bool
	mods      map[string][]string // module (a.b.C) -> files declaring it
	pkgs      map[string][]string // package (a.b) -> its modules' files
	firsts    map[string]bool     // first segments of the repository's packages
	manifests map[string]*manifest
	byDir     map[string][]*manifest // directory -> manifests in it
	all       []*manifest            // every manifest, shallowest first
	own       map[string]string      // library the repository is (haxelib.json name) -> its class path
	lix       map[string]*lixScope   // directory with haxe_libraries/ -> its pins
	lixFile   map[string]*lixLib     // haxe_libraries/<name>.hxml -> its pin
	installed map[string]*installed  // lower-case library name -> as installed
	limeFiles map[string]bool        // project files that are Lime's
	probed    sync.Map               // absolute path -> bool: exists
}

// Implements: REQ-HAXE-004, REQ-HAXE-005, REQ-HAXE-006, REQ-HAXE-007, REQ-HAXE-008
func newResolver(root string, all []*scan.File, getenv func(string) string) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{".": true}, mods: map[string][]string{},
		pkgs: map[string][]string{}, firsts: map[string]bool{}, manifests: map[string]*manifest{},
		byDir: map[string][]*manifest{}, own: map[string]string{}, lix: map[string]*lixScope{},
		lixFile: map[string]*lixLib{}, installed: map[string]*installed{}, limeFiles: map[string]bool{}}
	var sources, projects []*scan.File
	abs := map[string]string{}
	for _, f := range all {
		if haxelibDir(f.Path) {
			continue
		}
		r.files[f.Path] = true
		if !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize {
			abs[f.Path] = f.Abs
		}
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		switch class(f.Path) {
		case classSource:
			sources = append(sources, f)
		case classHXML:
			if data, err := os.ReadFile(f.Abs); err == nil {
				h := readHXML(data)
				m := &manifest{file: f.Path}
				for _, l := range h.libs {
					m.libs = append(m.libs, decl{l.name, l.version})
				}
				for _, cp := range h.cps {
					m.cps = append(m.cps, path.Join(path.Dir(f.Path), cp))
				}
				r.manifests[f.Path] = m
			}
		case classHaxelib:
			if data, err := os.ReadFile(f.Abs); err == nil {
				if h, deps, ok := readHaxelib(data); ok {
					m := &manifest{file: f.Path, owner: h.Name}
					for _, d := range deps {
						m.libs = append(m.libs, decl{d.name, d.version})
					}
					m.cps = []string{path.Join(path.Dir(f.Path), h.ClassPath)}
					r.manifests[f.Path] = m
				}
			}
		case classProject:
			projects = append(projects, f)
		}
	}
	for _, f := range projects {
		m := &manifest{file: f.Path}
		r.readProjectFile(m, f.Path, abs, map[string]bool{})
		if len(m.libs)+len(m.cps) > 0 || r.limeFiles[f.Path] {
			r.manifests[f.Path] = m
		}
	}
	for _, m := range r.manifests {
		r.all = append(r.all, m)
	}
	sort.Slice(r.all, func(i, j int) bool {
		di, dj := strings.Count(r.all[i].file, "/"), strings.Count(r.all[j].file, "/")
		if di != dj {
			return di < dj
		}
		return r.all[i].file < r.all[j].file
	})
	for _, m := range r.all {
		r.byDir[path.Dir(m.file)] = append(r.byDir[path.Dir(m.file)], m)
		if m.owner != "" {
			if _, ok := r.own[strings.ToLower(m.owner)]; !ok {
				dir := path.Dir(m.file)
				if len(m.cps) > 0 && r.dirs[m.cps[0]] {
					dir = m.cps[0]
				}
				r.own[strings.ToLower(m.owner)] = dir
			}
		}
	}
	r.index(sources)
	r.readLix(root, all, getenv)
	r.readInstalled(root, getenv)
	return r
}

// readProjectFile reads a Lime project file into m, and the XML files it
// includes (<include path="other.xml"/>), which Lime reads as part of it.
func (r *resolver) readProjectFile(m *manifest, rel string, abs map[string]string, seen map[string]bool) {
	if seen[rel] || len(seen) >= 16 || abs[rel] == "" {
		return
	}
	seen[rel] = true
	data, err := os.ReadFile(abs[rel])
	if err != nil || !limeProject(data) {
		return
	}
	r.limeFiles[rel] = true
	p := readProject(data)
	for _, d := range p.libs {
		m.libs = append(m.libs, decl{d.name, d.version})
	}
	for _, s := range p.sources {
		m.cps = append(m.cps, path.Join(path.Dir(rel), s))
	}
	for _, im := range p.imps {
		if im.Name == kindFile && strings.HasSuffix(im.Module, ".xml") {
			r.readProjectFile(m, path.Join(path.Dir(rel), im.Module), abs, seen)
		}
	}
}

// index reads the package every module declares, concurrently: the module a.b.C
// is the file C.hx declaring package a.b, wherever its class path is.
func (r *resolver) index(sources []*scan.File) {
	pkgs := make([]string, len(sources))
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				pkgs[i] = packageOf(sources[i].Abs)
			}
		}()
	}
	for i := range sources {
		next <- i
	}
	close(next)
	wg.Wait()
	for i, f := range sources {
		name := strings.TrimSuffix(path.Base(f.Path), ".hx")
		if !upper(name) {
			continue // import.hx and other files that are no module
		}
		mod := name
		if pkgs[i] != "" {
			mod = pkgs[i] + "." + name
			first, _, _ := strings.Cut(pkgs[i], ".")
			r.firsts[first] = true
		}
		r.mods[mod] = append(r.mods[mod], f.Path)
		r.pkgs[pkgs[i]] = append(r.pkgs[pkgs[i]], f.Path)
	}
	for _, m := range []map[string][]string{r.mods, r.pkgs} {
		for _, list := range m {
			sort.Strings(list)
		}
	}
}

// packageOf reads the package a module declares from the head of its file.
func packageOf(abs string) string {
	f, err := os.Open(abs)
	if err != nil {
		return ""
	}
	defer f.Close()
	head := make([]byte, 64<<10)
	n, _ := io.ReadFull(f, head)
	return readPackage(head[:n])
}

// Resolve maps an import to a file of the repository, the standard library or
// a haxelib library.
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	dir := path.Dir(file)
	switch imp.Name {
	case kindImport, kindUsing, kindRef:
		return r.module(file, imp.Module, imp.Name == kindRef)
	case kindLib:
		name, version, _ := strings.Cut(imp.Module, ":")
		return r.libTarget(file, name, &decl{name, version})
	case kindLix:
		if l := r.lixFile[file]; l != nil {
			return l.target()
		}
		return lang.Target{}
	case kindCP:
		if d := path.Join(dir, imp.Module); inside(d) && (r.dirs[d] || d == ".") {
			return lang.Target{Local: d}
		}
	case kindHXML, kindFile:
		if f := path.Join(dir, imp.Module); inside(f) && (r.files[f] || r.dirs[f]) {
			return lang.Target{Local: f}
		}
	case kindMain:
		return r.main(file, imp.Module)
	}
	return lang.Target{}
}

func inside(p string) bool {
	return p != ".." && !strings.HasPrefix(p, "../") && !strings.HasPrefix(p, "/")
}

// main resolves a build's main class under the class paths it names, else
// wherever the module is.
//
// Implements: REQ-HAXE-005
func (r *resolver) main(file, mod string) lang.Target {
	rel := strings.ReplaceAll(mod, ".", "/") + ".hx"
	var cps []string
	if m := r.manifests[file]; m != nil {
		cps = m.cps
	}
	for _, cp := range append(cps, path.Dir(file), path.Join(path.Dir(file), "src")) {
		if f := path.Join(cp, rel); r.files[f] {
			return lang.Target{Local: f}
		}
	}
	if f := r.local(file, strings.Split(mod, ".")); f != "" {
		return lang.Target{Local: f}
	}
	return lang.Target{}
}

// local is the repository's file of the module an import names (the longest
// leading module path: a.b.C.D is the type D of a.b.C), the one nearest the
// importing file when several class paths have it.
func (r *resolver) local(file string, segs []string) string {
	for n := len(segs); n >= 1; n-- {
		if !upper(segs[n-1]) {
			continue
		}
		if list := r.mods[strings.Join(segs[:n], ".")]; len(list) > 0 {
			return nearest(file, list)
		}
	}
	return ""
}

// nearest is the file sharing the longest directory prefix with from, the
// first in path order on a tie; from itself only when nothing else is there.
func nearest(from string, list []string) string {
	best, bestN := "", -1
	for _, f := range list {
		if f == from {
			continue
		}
		if n := common(from, f); n > bestN {
			best, bestN = f, n
		}
	}
	if best == "" && len(list) > 0 {
		return list[0]
	}
	return best
}

func common(a, b string) int {
	as, bs := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// module resolves an import, a using or a qualified name.
//
// Implements: REQ-HAXE-004, REQ-HAXE-007
func (r *resolver) module(file, mod string, ref bool) lang.Target {
	wild := strings.HasSuffix(mod, ".*")
	segs := strings.Split(strings.TrimSuffix(mod, ".*"), ".")
	if segs[0] == "std" && len(segs) > 1 {
		segs = segs[1:] // std.Any, std.format.Data: from the root package
	}
	if f := r.local(file, segs); f != "" {
		if f == file {
			return lang.Target{}
		}
		return lang.Target{Local: f}
	}
	if wild {
		if list := r.pkgs[strings.Join(segs, ".")]; len(list) > 0 && segs[0] != "" {
			return lang.Target{Local: path.Dir(nearest(file, list))}
		}
	}
	if name := r.installedFor(file, segs, wild); name != "" {
		return r.library(file, name)
	}
	lib, k, std := table(segs)
	decls := r.declared(file)
	spelled, ks := spell(decls, segs)
	if spelled != "" && ks > k {
		return r.library(file, spelled)
	}
	if std {
		return lang.Target{Ecosystem: ecoStd, Package: stdName(segs)}
	}
	if lib != "" {
		for _, d := range decls {
			if strings.EqualFold(d.name, lib) {
				return r.library(file, d.name)
			}
		}
		if r.known(file, lib) {
			return r.library(file, lib)
		}
		if ref || r.firsts[segs[0]] {
			return lang.Target{}
		}
		return lang.Target{Ecosystem: ecoHaxelib, Package: lib, Unresolved: true}
	}
	if spelled != "" {
		return r.library(file, spelled)
	}
	if ref || len(segs) == 1 || r.firsts[segs[0]] || !lower(segs[0]) {
		return lang.Target{} // the repository's own module, missing; or no package at all
	}
	return lang.Target{Ecosystem: ecoHaxelib, Package: segs[0], Unresolved: true}
}

// library is the haxelib library a module belongs to; nothing when that is
// the repository's own library, whose file is missing.
func (r *resolver) library(file, name string) lang.Target {
	if _, ok := r.own[strings.ToLower(name)]; ok {
		return lang.Target{}
	}
	return r.libTarget(file, name, nil)
}

// spell is the declared library the leading package segments of a module
// spell (tink.core is tink_core, thx.promise thx.promise, hxnodejs hxnodejs)
// and how many segments it took, longest first.
func spell(decls []decl, segs []string) (string, int) {
	pk := 0
	for pk < len(segs) && lower(segs[pk]) {
		pk++
	}
	for n := pk; n >= 1; n-- {
		for _, sep := range []string{"_", "-", ".", ""} {
			want := strings.ToLower(strings.Join(segs[:n], sep))
			for _, d := range decls {
				if strings.ToLower(d.name) == want {
					return d.name, n
				}
			}
		}
	}
	return "", 0
}

// scope lists the manifests governing a file: those in its directory and its
// ancestors, nearest first; every manifest when there are none.
func (r *resolver) scope(file string) []*manifest {
	var out []*manifest
	for d := path.Dir(file); ; d = path.Dir(d) {
		out = append(out, r.byDir[d]...)
		if d == "." || d == "/" {
			break
		}
	}
	if len(out) == 0 {
		return r.all
	}
	return out
}

// declared lists the libraries a file's manifests declare, nearest first, and
// the libraries lix pins for it.
func (r *resolver) declared(file string) []decl {
	var out []decl
	seen := map[string]bool{}
	for _, m := range r.scope(file) {
		for _, d := range m.libs {
			if k := strings.ToLower(d.name); !seen[k] {
				seen[k] = true
				out = append(out, d)
			}
		}
	}
	if s := r.lixScope(file); s != nil {
		for _, k := range s.names {
			if !seen[k] {
				seen[k] = true
				out = append(out, decl{name: s.libs[k].name})
			}
		}
	}
	return out
}

// known reports whether a library is pinned by lix or installed for a file.
func (r *resolver) known(file, name string) bool {
	k := strings.ToLower(name)
	if s := r.lixScope(file); s != nil && s.libs[k] != nil {
		return true
	}
	return r.installed[k] != nil
}

// libTarget is a haxelib library as the file's manifests and lix pin it. own
// is the declaration of the manifest being resolved, which comes first.
//
// Implements: REQ-HAXE-006
func (r *resolver) libTarget(file, name string, own *decl) lang.Target {
	k := strings.ToLower(name)
	if dir, ok := r.own[k]; ok {
		return lang.Target{Local: dir} // the repository's own library
	}
	d := own
	if d == nil || d.version == "" {
	search:
		for _, m := range r.scope(file) {
			for _, x := range m.libs {
				if strings.ToLower(x.name) == k && (d == nil || x.version != "") {
					d = &x
					if x.version != "" {
						break search // the nearest manifest asking for a version
					}
				}
			}
		}
	}
	if own != nil {
		name = own.name
	} else if d != nil {
		name = d.name
	}
	if s := r.lixScope(file); s != nil && s.libs[k] != nil {
		t := s.libs[k].target()
		if d != nil && d.version != "" && d.version != t.Version && t.Local == "" {
			t.Requested = d.version
		}
		return t
	}
	t := lang.Target{Ecosystem: ecoHaxelib, Package: name}
	if d != nil && d.version != "" {
		pinRule(&t, d.version)
		return t
	}
	in := r.installed[k]
	switch {
	case in != nil:
		t.Version, t.Floating = in.version, true
		if d == nil {
			t.Package = in.name
		}
	case d != nil:
		t.Floating = true
	default:
		t.Unresolved = true
	}
	return t
}

// pinRule reads a haxelib version: an exact version pins, git:<url>#<commit>
// pins, a tag (#v1.2.3) is shown, neither pinned nor floating, and a branch,
// a git URL alone or no version floats. A git server other than the public
// forges is the library's origin.
//
// Implements: REQ-HAXE-006
func pinRule(t *lang.Target, v string) {
	v = strings.TrimSpace(v)
	if vcs, rest, ok := strings.Cut(v, ":"); ok && (vcs == "git" || vcs == "hg") {
		url, ref, _ := strings.Cut(rest, "#")
		if !public(url) {
			t.Origin = url
		}
		switch {
		case lang.Commit(ref):
			t.Version, t.Pinned = ref, true
		case tagLike(ref):
			t.Version = ref
		default:
			t.Version, t.Floating = ref, true
		}
		return
	}
	if lang.Pinned(v) {
		t.Version, t.Pinned = v, true
		return
	}
	t.Version, t.Floating = v, true
}

func tagLike(ref string) bool {
	s := strings.TrimPrefix(ref, "v")
	return s != "" && s[0] >= '0' && s[0] <= '9'
}

// public reports whether a repository is on a public forge (or unknown), as
// opposed to a git server of the organization's own, which is recorded as the
// library's origin.
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

// Expand turns `import a.b.*` of a package of the repository into one import
// per module of the package, in path order and capped.
//
// Implements: REQ-HAXE-004
func (r *resolver) Expand(file string, imp lang.RawImport) ([]lang.Import, bool) {
	if (imp.Name != kindImport && imp.Name != kindUsing) || !strings.HasSuffix(imp.Module, ".*") {
		return nil, false
	}
	pkg := strings.TrimSuffix(imp.Module, ".*")
	segs := strings.Split(pkg, ".")
	if upper(segs[len(segs)-1]) || r.local(file, segs) != "" {
		return nil, false // a module's fields: import a.b.C.*
	}
	list := r.pkgs[pkg]
	// One file per module: several class paths may declare the same one.
	byMod := map[string][]string{}
	var names []string
	for _, f := range list {
		n := path.Base(f)
		if byMod[n] == nil {
			names = append(names, n)
		}
		byMod[n] = append(byMod[n], f)
	}
	sort.Strings(names)
	var out []lang.Import
	for _, n := range names {
		f := nearest(file, byMod[n])
		if f == file {
			continue
		}
		if len(out) == maxExpand {
			break
		}
		out = append(out, lang.Import{Spec: imp.Spec + " (" + f + ")", Line: imp.Line, Target: lang.Target{Local: f}})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

const maxExpand = 100

// Dependencies lists what a library depends on: the -lib lines of its lix pin,
// else the haxelib.json of the version installed.
//
// Implements: REQ-HAXE-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoHaxelib {
		return nil
	}
	k := strings.ToLower(t.Package)
	for _, dir := range sortedKeys(r.lix) {
		s := r.lix[dir]
		l := s.libs[k]
		if l == nil {
			continue
		}
		var out []lang.Target
		for _, dep := range l.deps {
			if dl := s.libs[strings.ToLower(dep)]; dl != nil {
				out = append(out, dl.target())
			} else {
				out = append(out, lang.Target{Ecosystem: ecoHaxelib, Package: dep, Floating: true})
			}
		}
		return out
	}
	in := r.installed[k]
	if in == nil {
		return nil
	}
	var out []lang.Target
	for _, d := range in.deps {
		dt := lang.Target{Ecosystem: ecoHaxelib, Package: d.name}
		if d.version != "" {
			pinRule(&dt, d.version)
		} else if x := r.installed[strings.ToLower(d.name)]; x != nil {
			dt.Version, dt.Floating = x.version, true
		} else {
			dt.Floating = true
		}
		out = append(out, dt)
	}
	return out
}

// Installed reports whether a library's dependencies come from what haxelib
// installed on this machine rather than from lix's pins.
func (r *resolver) Installed(t lang.Target) bool {
	if t.Ecosystem != ecoHaxelib {
		return false
	}
	k := strings.ToLower(t.Package)
	for _, s := range r.lix {
		if s.libs[k] != nil {
			return false
		}
	}
	return r.installed[k] != nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// exists reports whether an absolute path exists, remembered.
func (r *resolver) exists(p string) bool {
	if v, ok := r.probed.Load(p); ok {
		return v.(bool)
	}
	_, err := os.Stat(p)
	r.probed.Store(p, err == nil)
	return err == nil
}

// installedFor is the library installed for a file (lix's cache, a haxelib
// repository) whose class path has the module, "" when none has.
func (r *resolver) installedFor(file string, segs []string, wild bool) string {
	try := func(cps []string) bool {
		for _, cp := range cps {
			if wild && r.exists(filepath.Join(cp, filepath.Join(segs...))) {
				return true
			}
			for n := len(segs); n >= 1; n-- {
				if upper(segs[n-1]) && r.exists(filepath.Join(cp, filepath.Join(segs[:n]...)+".hx")) {
					return true
				}
			}
		}
		return false
	}
	if len(segs) > maxSegs {
		return ""
	}
	seen := map[string]bool{}
	s := r.lixScope(file)
	for _, d := range r.declared(file) {
		k := strings.ToLower(d.name)
		seen[k] = true
		if s != nil && s.libs[k] != nil {
			if try(s.libs[k].abs) {
				return s.libs[k].name
			}
		} else if in := r.installed[k]; in != nil && try(in.cps) {
			return in.name
		}
	}
	for _, k := range sortedKeys(r.installed) {
		if in := r.installed[k]; !seen[k] && in.local && try(in.cps) {
			return in.name
		}
	}
	return ""
}
