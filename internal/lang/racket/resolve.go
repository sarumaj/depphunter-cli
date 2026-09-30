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

// racketPackage is a package of the repository: a directory whose info.rkt defines
// its collection, deps, build-deps or pkg-desc.
type racketPackage struct {
	directory    string // "." for the repository root
	name         string
	info         string // path of its info.rkt
	dependencies []dependency
}

type resolver struct {
	files map[string]bool
	// collections maps a collection name to the directories of the repository
	// that hold it (a collection may be spread over several packages).
	collections map[string][]string
	packages    map[string]*racketPackage // by name
	// directoryPackage maps every directory holding a file to its package (nearest
	// package directory above it), nil for none.
	directoryPackage map[string]*racketPackage
	all              []dependency // every package's dependencies, for files outside any package
}

// newResolver reads every info.rkt of the repository for its packages and
// collections, and the collects/ trees.
//
// Implements: REQ-RACKET-004, REQ-RACKET-005, REQ-RACKET-011
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, collections: map[string][]string{}, packages: map[string]*racketPackage{}, directoryPackage: map[string]*racketPackage{}}
	children := map[string]map[string]bool{} // dir -> child directories holding files
	var infos []*scan.File
	for _, f := range all {
		if strings.HasPrefix(f.Path, "compiled/") || strings.Contains(f.Path, "/compiled/") {
			continue
		}
		r.files[f.Path] = true
		segments := strings.Split(f.Path, "/")
		for i := 0; i+1 < len(segments); i++ {
			directory := "."
			if i > 0 {
				directory = strings.Join(segments[:i], "/")
			}
			if children[directory] == nil {
				children[directory] = map[string]bool{}
			}
			children[directory][segments[i]] = true
		}
		if path.Base(f.Path) == "info.rkt" && !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize {
			infos = append(infos, f)
		}
	}
	addColl := func(name, directory string) {
		if name == "" || strings.HasPrefix(name, ".") || name == "compiled" {
			return
		}
		for _, d := range r.collections[name] {
			if d == directory {
				return
			}
		}
		r.collections[name] = append(r.collections[name], directory)
	}
	var packageDirectories = map[string]*racketPackage{}
	for _, f := range infos {
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		in := readInfo(source)
		directory := path.Dir(f.Path)
		if !in.isPackage {
			continue
		}
		name := path.Base(directory)
		if directory == "." {
			name = filepath.Base(root)
			if in.defined && in.collection != "multi" && in.collection != "use-pkg-name" {
				name = in.collection
			}
		}
		p := &racketPackage{directory: directory, name: name, info: f.Path, dependencies: in.dependencies}
		packageDirectories[directory] = p
		if _, duplicate := r.packages[name]; !duplicate {
			r.packages[name] = p
		}
		r.all = append(r.all, in.dependencies...)
		switch {
		case in.collection == "multi":
			for c := range children[directory] {
				addColl(c, join(directory, c))
			}
		case in.defined && in.collection != "use-pkg-name":
			addColl(in.collection, directory)
		default:
			addColl(name, directory)
		}
	}
	// A collects/ directory (Racket's own main collection tree, or a
	// PLTCOLLECTS directory kept in a repository) holds collections.
	for directory, kids := range children {
		if path.Base(directory) == "collects" {
			for c := range kids {
				addColl(c, join(directory, c))
			}
		}
	}
	for _, directories := range r.collections {
		sort.Strings(directories)
	}
	for directory := range children {
		r.nearest(directory, packageDirectories)
	}
	for directory := range packageDirectories {
		r.nearest(directory, packageDirectories)
	}
	return r
}

func join(directory, name string) string {
	if directory == "." {
		return name
	}
	return directory + "/" + name
}

// nearest finds (and records) the package a directory belongs to.
func (r *resolver) nearest(directory string, packageDirectories map[string]*racketPackage) *racketPackage {
	if p, ok := r.directoryPackage[directory]; ok {
		return p
	}
	p := packageDirectories[directory]
	if p == nil && directory != "." {
		p = r.nearest(path.Dir(directory), packageDirectories)
	}
	r.directoryPackage[directory] = p
	return p
}

// Implements: REQ-RACKET-004, REQ-RACKET-005, REQ-RACKET-006, REQ-RACKET-007, REQ-RACKET-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, rest, _ := strings.Cut(rawImport.Name, "\x00")
	switch kind {
	case kindDependency:
		version, checksum, _ := strings.Cut(rest, "\x00")
		d := dependency{source: rawImport.Module, version: version, checksum: checksum}
		d.name, d.url, d.reference, d.local = parseSource(rawImport.Module)
		return r.dependencyTarget(d, path.Dir(file))
	case kindRelative:
		if p := r.probe(path.Join(path.Dir(file), rawImport.Module)); p != "" && p != file {
			return lang.Target{Local: p}
		}
	case kindUp:
		for directory := range lang.Ancestors(file) {
			if p := r.probe(path.Join(directory, rawImport.Module)); p != "" && p != file {
				return lang.Target{Local: p}
			}
		}
	case kindPlanet:
		name, version, _ := strings.Cut(rawImport.Module, ":")
		t := lang.Target{Ecosystem: ecosystemRaco, Package: "planet/" + name, Version: version, Floating: true}
		return t
	case kindColl:
		return r.collection(file, rawImport.Module, false)
	case kindLanguage:
		return r.collection(file, rawImport.Module, true)
	}
	return lang.Target{}
}

// probe returns a file of the repository at p, or at p with .rkt for a .ss
// path (Racket reads x.rkt for a require of x.ss when it exists).
func (r *resolver) probe(p string) string {
	if lang.ClimbsOut(p) {
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
func (r *resolver) collection(file, module string, isLanguage bool) lang.Target {
	segments := strings.Split(module, "/")
	releases := []string{"main.rkt"}
	if len(segments) > 1 {
		releases[0] = strings.Join(segments[1:], "/") + ".rkt"
	}
	if isLanguage {
		releases = append([]string{join(strings.Join(segments[1:], "/"), "lang/reader.rkt")}, releases...)
		if len(segments) == 1 {
			releases[0] = "lang/reader.rkt"
		}
	}
	roots := r.collections[segments[0]]
	if len(roots) > 0 {
		best := ""
		for _, relative := range releases {
			for _, root := range roots {
				if p := r.probe(join(root, relative)); p != "" && (best == "" || shared(p, file) > shared(best, file)) {
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
		return lang.Target{Ecosystem: ecosystemStd, Package: segments[0]}
	}
	declared := r.declared(file)
	if k := knownPackage(segments); k != "" {
		if k == "base" {
			return lang.Target{Ecosystem: ecosystemStd, Package: segments[0]}
		}
		if d, ok := match(declared, k); ok {
			return r.packageTarget(d)
		}
		if r.packages[k] != nil || len(roots) > 0 {
			return lang.Target{}
		}
		if d, ok := startsLike(declared, segments[0]); ok {
			return r.packageTarget(d)
		}
		return lang.Target{Ecosystem: ecosystemRaco, Package: k, Unresolved: true}
	}
	if d, ok := byCollection(declared, segments); ok {
		return r.packageTarget(d)
	}
	if d, ok := startsLike(declared, segments[0]); ok {
		return r.packageTarget(d)
	}
	if baseTops[segments[0]] {
		return lang.Target{Ecosystem: ecosystemStd, Package: segments[0]}
	}
	if len(roots) > 0 || r.packages[segments[0]] != nil {
		return lang.Target{} // the repository's own collection, file missing
	}
	return lang.Target{Ecosystem: ecosystemRaco, Package: segments[0], Unresolved: true}
}

// packageTarget is the target of a declared package a module path belongs
// to; one of the repository's own packages is not a dependency.
func (r *resolver) packageTarget(d dependency) lang.Target {
	if r.packages[d.name] != nil && d.url == "" {
		return lang.Target{}
	}
	return r.dependencyTarget(d, "")
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
func (r *resolver) declared(file string) []dependency {
	if p := r.directoryPackage[path.Dir(file)]; p != nil {
		return p.dependencies
	}
	return r.all
}

// normalize is a package name without the -lib, -doc or -test the main
// distribution's split packages add to their collection's name.
func normalize(name string) string {
	for _, suffix := range []string{"-lib", "-doc", "-test"} {
		if s, ok := strings.CutSuffix(name, suffix); ok {
			return s
		}
	}
	return name
}

// match finds the declared package that is k, or k's umbrella package
// (typed-racket for typed-racket-lib).
func match(declared []dependency, k string) (dependency, bool) {
	for _, d := range declared {
		if d.name == k {
			return d, true
		}
	}
	for _, d := range declared {
		if normalize(d.name) == normalize(k) && !strings.HasSuffix(d.name, "-doc") && !strings.HasSuffix(d.name, "-test") {
			return d, true
		}
	}
	return dependency{}, false
}

// byCollection finds the declared package a module path's collection names:
// its first segments joined with - (typed/racket is typed-racket), with or
// without -lib; the longest wins.
func byCollection(declared []dependency, segments []string) (dependency, bool) {
	for k := min(len(segments), 3); k > 0; k-- {
		c := strings.Join(segments[:k], "-")
		if d, ok := match(declared, c); ok {
			return d, true
		}
	}
	return dependency{}, false
}

// startsLike finds the one declared package whose name starts with the
// collection's and a dash (srfi-lite-lib has srfi/1), not a -doc or -test.
func startsLike(declared []dependency, collection string) (dependency, bool) {
	var found []dependency
	for _, d := range declared {
		if strings.HasPrefix(d.name, collection+"-") && !strings.HasSuffix(d.name, "-doc") && !strings.HasSuffix(d.name, "-test") {
			if len(found) == 0 || found[0].name != d.name {
				found = append(found, d)
			}
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return dependency{}, false
}

// dependencyTarget is the target of a deps entry. from is the directory of the
// info.rkt that lists it ("" when a require reached it).
//
// Implements: REQ-RACKET-006
func (r *resolver) dependencyTarget(d dependency, from string) lang.Target {
	if d.local != "" {
		if from != "" && !path.IsAbs(d.local) {
			directory := path.Join(from, d.local)
			if r.files[join(directory, "info.rkt")] {
				return lang.Target{Local: join(directory, "info.rkt")}
			}
		}
		return lang.Target{Ecosystem: ecosystemRaco, Package: d.name, Floating: true, Origin: d.local}
	}
	if p := r.packages[d.name]; p != nil && from != "" {
		return lang.Target{Local: p.info}
	}
	if d.url == "" && (d.name == "base" || d.name == "racket") {
		return lang.Target{Ecosystem: ecosystemStd, Package: d.name}
	}
	t := lang.Target{Ecosystem: ecosystemRaco, Package: d.name}
	switch {
	case d.checksum != "":
		t.Version, t.Requested, t.Pinned = d.checksum, d.version, true
	case d.url != "" && lang.Commit(d.reference):
		t.Version, t.Requested, t.Pinned = d.reference, d.version, true
	case d.url != "" && d.reference != "" && lang.Pinned(d.reference):
		t.Version, t.Requested = d.reference, d.version // a version tag: neither pinned nor floating
	case d.url != "" && d.reference != "":
		t.Version, t.Requested, t.Floating = d.reference, d.version, true
	default:
		t.Version, t.Floating = d.version, true // #:version is a minimum
	}
	if d.url != "" && !lang.PublicForge(d.url) {
		t.Origin = d.url
	}
	return t
}
