package racket

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// pkg is a package of the repository: a directory whose info.rkt defines
// its collection, deps, build-deps or pkg-desc.
type pkg struct {
	dir  string // "." for the repository root
	name string
	info string // path of its info.rkt
	deps []dep
}

type resolver struct {
	files map[string]bool
	// collections maps a collection name to the directories of the repository
	// that hold it (a collection may be spread over several packages).
	collections map[string][]string
	pkgs        map[string]*pkg // by name
	// dirPkg maps every directory holding a file to its package (nearest
	// package directory above it), nil for none.
	dirPkg map[string]*pkg
	all    []dep // every package's deps, for files outside any package
}

// newResolver reads every info.rkt of the repository for its packages and
// collections, and the collects/ trees.
//
// Implements: REQ-RACKET-004, REQ-RACKET-005, REQ-RACKET-011
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, collections: map[string][]string{}, pkgs: map[string]*pkg{}, dirPkg: map[string]*pkg{}}
	children := map[string]map[string]bool{} // dir -> child directories holding files
	var infos []*scan.File
	for _, f := range all {
		if strings.HasPrefix(f.Path, "compiled/") || strings.Contains(f.Path, "/compiled/") {
			continue
		}
		r.files[f.Path] = true
		segments := strings.Split(f.Path, "/")
		for i := 0; i+1 < len(segments); i++ {
			dir := "."
			if i > 0 {
				dir = strings.Join(segments[:i], "/")
			}
			if children[dir] == nil {
				children[dir] = map[string]bool{}
			}
			children[dir][segments[i]] = true
		}
		if path.Base(f.Path) == "info.rkt" && !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize {
			infos = append(infos, f)
		}
	}
	addColl := func(name, dir string) {
		if name == "" || strings.HasPrefix(name, ".") || name == "compiled" {
			return
		}
		for _, d := range r.collections[name] {
			if d == dir {
				return
			}
		}
		r.collections[name] = append(r.collections[name], dir)
	}
	var pkgDirs = map[string]*pkg{}
	for _, f := range infos {
		src, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		in := readInfo(src)
		dir := path.Dir(f.Path)
		if !in.pkg {
			continue
		}
		name := path.Base(dir)
		if dir == "." {
			name = filepath.Base(root)
			if in.defined && in.collection != "multi" && in.collection != "use-pkg-name" {
				name = in.collection
			}
		}
		p := &pkg{dir: dir, name: name, info: f.Path, deps: in.deps}
		pkgDirs[dir] = p
		if _, dup := r.pkgs[name]; !dup {
			r.pkgs[name] = p
		}
		r.all = append(r.all, in.deps...)
		switch {
		case in.collection == "multi":
			for c := range children[dir] {
				addColl(c, join(dir, c))
			}
		case in.defined && in.collection != "use-pkg-name":
			addColl(in.collection, dir)
		default:
			addColl(name, dir)
		}
	}
	// A collects/ directory (Racket's own main collection tree, or a
	// PLTCOLLECTS directory kept in a repository) holds collections.
	for dir, kids := range children {
		if path.Base(dir) == "collects" {
			for c := range kids {
				addColl(c, join(dir, c))
			}
		}
	}
	for _, dirs := range r.collections {
		sort.Strings(dirs)
	}
	for dir := range children {
		r.nearest(dir, pkgDirs)
	}
	for dir := range pkgDirs {
		r.nearest(dir, pkgDirs)
	}
	return r
}

func join(dir, name string) string {
	if dir == "." {
		return name
	}
	return dir + "/" + name
}

// nearest finds (and records) the package a directory belongs to.
func (r *resolver) nearest(dir string, pkgDirs map[string]*pkg) *pkg {
	if p, ok := r.dirPkg[dir]; ok {
		return p
	}
	p := pkgDirs[dir]
	if p == nil && dir != "." {
		p = r.nearest(path.Dir(dir), pkgDirs)
	}
	r.dirPkg[dir] = p
	return p
}

// Implements: REQ-RACKET-004, REQ-RACKET-005, REQ-RACKET-006, REQ-RACKET-007, REQ-RACKET-008
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, rest, _ := strings.Cut(imp.Name, "\x00")
	switch kind {
	case kindDep:
		version, checksum, _ := strings.Cut(rest, "\x00")
		d := dep{source: imp.Module, version: version, checksum: checksum}
		d.name, d.url, d.ref, d.local = parseSource(imp.Module)
		return r.depTarget(d, path.Dir(file))
	case kindRel:
		if p := r.probe(path.Join(path.Dir(file), imp.Module)); p != "" && p != file {
			return lang.Target{Local: p}
		}
	case kindUp:
		for dir := path.Dir(file); ; dir = path.Dir(dir) {
			if p := r.probe(path.Join(dir, imp.Module)); p != "" && p != file {
				return lang.Target{Local: p}
			}
			if dir == "." || dir == "/" {
				break
			}
		}
	case kindPlanet:
		name, ver, _ := strings.Cut(imp.Module, ":")
		t := lang.Target{Ecosystem: ecoRaco, Package: "planet/" + name, Version: ver, Floating: true}
		return t
	case kindColl:
		return r.collection(file, imp.Module, false)
	case kindLang:
		return r.collection(file, imp.Module, true)
	}
	return lang.Target{}
}

// probe returns a file of the repository at p, or at p with .rkt for a .ss
// path (Racket reads x.rkt for a require of x.ss when it exists).
func (r *resolver) probe(p string) string {
	if strings.HasPrefix(p, "../") || p == ".." {
		return ""
	}
	if r.files[p] {
		return p
	}
	if base, ok := strings.CutSuffix(p, ".ss"); ok && r.files[base+".rkt"] {
		return base + ".rkt"
	}
	if path.Ext(p) == "" && r.files[p+".rkt"] {
		return p + ".rkt" // a relative path without a suffix gets .rkt
	}
	return ""
}

// collection resolves a collection-based module path; a #lang's path
// (isLang) names the module whose lang/reader.rkt reads the file, else the
// module itself (its reader submodule).
func (r *resolver) collection(file, mod string, isLang bool) lang.Target {
	segments := strings.Split(mod, "/")
	releases := []string{"main.rkt"}
	if len(segments) > 1 {
		releases[0] = strings.Join(segments[1:], "/") + ".rkt"
	}
	if isLang {
		releases = append([]string{join(strings.Join(segments[1:], "/"), "lang/reader.rkt")}, releases...)
		if len(segments) == 1 {
			releases[0] = "lang/reader.rkt"
		}
	}
	roots := r.collections[segments[0]]
	if len(roots) > 0 {
		best := ""
		for _, rel := range releases {
			for _, root := range roots {
				if p := r.probe(join(root, rel)); p != "" && (best == "" || shared(p, file) > shared(best, file)) {
					best = p
				}
			}
			if best != "" {
				break
			}
		}
		if best != "" {
			if best == file {
				return lang.Target{}
			}
			return lang.Target{Local: best}
		}
	}
	if baseModule(segments) {
		return lang.Target{Ecosystem: ecoStd, Package: segments[0]}
	}
	declared := r.declared(file)
	if k := knownPackage(segments); k != "" {
		if k == "base" {
			return lang.Target{Ecosystem: ecoStd, Package: segments[0]}
		}
		if d, ok := match(declared, k); ok {
			return r.packageTarget(d)
		}
		if r.pkgs[k] != nil || len(roots) > 0 {
			return lang.Target{}
		}
		if d, ok := startsLike(declared, segments[0]); ok {
			return r.packageTarget(d)
		}
		return lang.Target{Ecosystem: ecoRaco, Package: k, Unresolved: true}
	}
	if d, ok := byCollection(declared, segments); ok {
		return r.packageTarget(d)
	}
	if d, ok := startsLike(declared, segments[0]); ok {
		return r.packageTarget(d)
	}
	if baseTops[segments[0]] {
		return lang.Target{Ecosystem: ecoStd, Package: segments[0]}
	}
	if len(roots) > 0 || r.pkgs[segments[0]] != nil {
		return lang.Target{} // the repository's own collection, file missing
	}
	return lang.Target{Ecosystem: ecoRaco, Package: segments[0], Unresolved: true}
}

// packageTarget is the target of a declared package a module path belongs
// to; one of the repository's own packages is not a dependency.
func (r *resolver) packageTarget(d dep) lang.Target {
	if r.pkgs[d.name] != nil && d.url == "" {
		return lang.Target{}
	}
	return r.depTarget(d, "")
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

// declared lists the deps and build-deps of the package a file belongs to;
// a file outside every package sees all of the repository's.
func (r *resolver) declared(file string) []dep {
	if p := r.dirPkg[path.Dir(file)]; p != nil {
		return p.deps
	}
	return r.all
}

// norm is a package name without the -lib, -doc or -test the main
// distribution's split packages add to their collection's name.
func norm(name string) string {
	for _, suf := range []string{"-lib", "-doc", "-test"} {
		if s, ok := strings.CutSuffix(name, suf); ok {
			return s
		}
	}
	return name
}

// match finds the declared package that is k, or k's umbrella package
// (typed-racket for typed-racket-lib).
func match(declared []dep, k string) (dep, bool) {
	for _, d := range declared {
		if d.name == k {
			return d, true
		}
	}
	for _, d := range declared {
		if norm(d.name) == norm(k) && !strings.HasSuffix(d.name, "-doc") && !strings.HasSuffix(d.name, "-test") {
			return d, true
		}
	}
	return dep{}, false
}

// byCollection finds the declared package a module path's collection names:
// its first segments joined with - (typed/racket is typed-racket), with or
// without -lib; the longest wins.
func byCollection(declared []dep, segments []string) (dep, bool) {
	for k := min(len(segments), 3); k > 0; k-- {
		c := strings.Join(segments[:k], "-")
		if d, ok := match(declared, c); ok {
			return d, true
		}
	}
	return dep{}, false
}

// startsLike finds the one declared package whose name starts with the
// collection's and a dash (srfi-lite-lib has srfi/1), not a -doc or -test.
func startsLike(declared []dep, coll string) (dep, bool) {
	var found []dep
	for _, d := range declared {
		if strings.HasPrefix(d.name, coll+"-") && !strings.HasSuffix(d.name, "-doc") && !strings.HasSuffix(d.name, "-test") {
			if len(found) == 0 || found[0].name != d.name {
				found = append(found, d)
			}
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return dep{}, false
}

// depTarget is the target of a deps entry. from is the directory of the
// info.rkt that lists it ("" when a require reached it).
//
// Implements: REQ-RACKET-006
func (r *resolver) depTarget(d dep, from string) lang.Target {
	if d.local != "" {
		if from != "" && !path.IsAbs(d.local) {
			dir := path.Join(from, d.local)
			if r.files[join(dir, "info.rkt")] {
				return lang.Target{Local: join(dir, "info.rkt")}
			}
		}
		return lang.Target{Ecosystem: ecoRaco, Package: d.name, Floating: true, Origin: d.local}
	}
	if p := r.pkgs[d.name]; p != nil && from != "" {
		return lang.Target{Local: p.info}
	}
	if d.url == "" && (d.name == "base" || d.name == "racket") {
		return lang.Target{Ecosystem: ecoStd, Package: d.name}
	}
	t := lang.Target{Ecosystem: ecoRaco, Package: d.name}
	switch {
	case d.checksum != "":
		t.Version, t.Requested, t.Pinned = d.checksum, d.version, true
	case d.url != "" && lang.Commit(d.ref):
		t.Version, t.Requested, t.Pinned = d.ref, d.version, true
	case d.url != "" && d.ref != "" && lang.Pinned(d.ref):
		t.Version, t.Requested = d.ref, d.version // a version tag: neither pinned nor floating
	case d.url != "" && d.ref != "":
		t.Version, t.Requested, t.Floating = d.ref, d.version, true
	default:
		t.Version, t.Floating = d.version, true // #:version is a minimum
	}
	if d.url != "" && !public(d.url) {
		t.Origin = d.url
	}
	return t
}

// public reports whether a git URL is on a public forge, whose packages are
// named, not origins.
func public(url string) bool {
	host, _, _ := strings.Cut(lang.RepoName(url), "/")
	switch host {
	case "github.com", "gitlab.com", "bitbucket.org", "codeberg.org", "git.sr.ht", "sr.ht":
		return true
	}
	return false
}
