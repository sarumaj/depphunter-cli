package r

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// rpkg is a DESCRIPTION of the repository: an R package when it has a Package field,
// else a project declaring its dependencies.
type rpkg struct {
	dir, file string
	desc      *description
}

type resolver struct {
	files        map[string]bool
	dirs         map[string]bool
	descriptions []*rpkg          // deepest first
	named        map[string]*rpkg // R packages of the repository by name
	locks        []*lockfile      // deepest first
	// scopes are the directories whose files call each other's functions: every
	// package, and the projects outside packages (an .Rproj, renv.lock, _targets.R
	// or .here beside them), deepest first; "." is the last.
	scopes []string
	defs   map[string]map[string][]string // scope -> function or class name -> files
}

// Implements: REQ-R-006, REQ-R-007, REQ-R-008, REQ-R-009, REQ-R-010
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, named: map[string]*rpkg{},
		defs: map[string]map[string][]string{}}
	abs := map[string]string{}
	var sources []*scan.File
	scopes := map[string]bool{".": true}
	// Where a lock may be: beside a DESCRIPTION or an .Rproj, where renv keeps its
	// activate.R, and the root - read from disk when it is not in the file list.
	lockDirs := map[string]bool{".": true}
	for _, f := range all {
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		base := path.Base(f.Path)
		switch {
		case strings.HasSuffix(base, ".Rproj") || base == "renv.lock" || base == "_targets.R" || base == ".here":
			scopes[path.Dir(f.Path)] = true
		case base == "packrat.lock" && path.Base(path.Dir(f.Path)) == "packrat":
			scopes[path.Dir(path.Dir(f.Path))] = true
		}
		switch {
		case base == "DESCRIPTION" || base == "renv.lock" || strings.HasSuffix(base, ".Rproj"):
			lockDirs[path.Dir(f.Path)] = true
		case path.Base(path.Dir(f.Path)) == "renv" || path.Base(path.Dir(f.Path)) == "packrat":
			lockDirs[path.Dir(path.Dir(f.Path))] = true
		}
		if !(Plugin{}).Claims(f) || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		if base == "DESCRIPTION" {
			src, err := os.ReadFile(f.Abs)
			if err != nil {
				continue
			}
			if d := readDescription(src); d != nil {
				p := &rpkg{dir: path.Dir(f.Path), file: f.Path, desc: d}
				r.descriptions = append(r.descriptions, p)
				if d.name != "" {
					scopes[p.dir] = true
					if q := r.named[d.name]; q == nil || depth(p.dir) < depth(q.dir) {
						r.named[d.name] = p
					}
				}
			}
			continue
		}
		if base != "NAMESPACE" {
			sources = append(sources, f)
		}
	}
	sort.Slice(r.descriptions, func(i, j int) bool { return deeper(r.descriptions[i].dir, r.descriptions[j].dir) })
	// renv.lock is committed by projects and packages alike; packrat keeps its lock in
	// packrat/. Either may be git-ignored, so what is on disk counts too.
	read := func(rel string) ([]byte, bool) {
		if a, ok := abs[rel]; ok {
			data, err := os.ReadFile(a)
			return data, err == nil
		}
		if root == "" {
			return nil, false
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return data, err == nil
	}
	for d := range lockDirs {
		if src, ok := read(path.Join(d, "renv.lock")); ok {
			if l := readRenvLock(src, d); l != nil {
				r.locks = append(r.locks, l)
				scopes[d] = true
			}
		} else if src, ok := read(path.Join(d, "packrat", "packrat.lock")); ok {
			r.locks = append(r.locks, readPackratLock(src, d))
		}
	}
	sort.Slice(r.locks, func(i, j int) bool { return deeper(r.locks[i].dir, r.locks[j].dir) })
	for s := range scopes {
		r.scopes = append(r.scopes, s)
	}
	sort.Slice(r.scopes, func(i, j int) bool { return deeper(r.scopes[i], r.scopes[j]) })
	r.index(sources)
	return r
}

// index learns which file defines each function, class and generic, per scope. It
// reads the sources a second time (extraction results are not shared with
// resolvers); the lexer is fast enough for that to be cheap.
//
// Implements: REQ-R-010
func (r *resolver) index(sources []*scan.File) {
	sort.Slice(sources, func(i, j int) bool { return sources[i].Path < sources[j].Path })
	for _, f := range sources {
		src, err := os.ReadFile(f.Abs)
		if err != nil || !lang.Parseable(f, src) {
			continue
		}
		ex, _ := (Plugin{}).Extract(f, src)
		scope := r.scopeOf(f.Path)
		for _, s := range ex.Symbols {
			if s.Kind != "function" && s.Kind != "class" && s.Kind != "generic" {
				continue
			}
			name, _, _ := strings.Cut(s.Name, "@")
			m := r.defs[scope]
			if m == nil {
				m = map[string][]string{}
				r.defs[scope] = m
			}
			if fs := m[name]; len(fs) == 0 || fs[len(fs)-1] != f.Path {
				m[name] = append(fs, f.Path)
			}
		}
	}
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// deeper orders directories deepest first, then by name.
func deeper(a, b string) bool {
	if da, db := depth(a), depth(b); da != db {
		return da > db
	}
	return a < b
}

func within(file, dir string) bool { return dir == "." || strings.HasPrefix(file, dir+"/") }

func (r *resolver) scopeOf(file string) string {
	for _, s := range r.scopes {
		if within(file, s) {
			return s
		}
	}
	return "."
}

// descOf is the nearest DESCRIPTION above a file; pkgOf the nearest that is a package.
func (r *resolver) descOf(file string) *rpkg {
	for _, p := range r.descriptions {
		if within(file, p.dir) {
			return p
		}
	}
	return nil
}

func (r *resolver) pkgOf(file string) *rpkg {
	for _, p := range r.descriptions {
		if p.desc.name != "" && within(file, p.dir) {
			return p
		}
	}
	return nil
}

func (r *resolver) lockOf(file string) *lockfile {
	for _, l := range r.locks {
		if within(file, l.dir) {
			return l
		}
	}
	return nil
}

// Implements: REQ-R-002, REQ-R-004, REQ-R-005, REQ-R-006, REQ-R-009, REQ-R-010
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, _, _ := strings.Cut(imp.Name, ":")
	dir := path.Dir(file)
	switch kind {
	case kindPkg, kindNS, kindRoxygen, kindBox, kindNSFile:
		return r.pkg(file, imp.Module, false)
	case kindDep:
		return r.pkg(file, imp.Module, true)
	case kindCall:
		return r.call(file, imp.Module)
	case kindInclude:
		if f := path.Join(dir, imp.Module); r.files[f] {
			return lang.Target{Local: f}
		}
	case kindBoxLocal:
		mod := imp.Module
		var bases []string
		if strings.HasPrefix(mod, "./") || strings.HasPrefix(mod, "../") {
			bases = []string{dir}
		} else {
			bases = ancestors(dir) // box.path: usually the project root
		}
		for _, b := range bases {
			p := path.Join(b, mod)
			if !inside(p) {
				continue
			}
			for _, candidate := range []string{p + ".R", p + ".r", p + "/__init__.R", p + "/__init__.r"} {
				if r.files[candidate] {
					return lang.Target{Local: candidate}
				}
			}
		}
	case kindSource, kindChild:
		// A path is relative to the working directory, which is the project root far
		// more often than the file's own directory; both are tried, nearest first.
		if path.IsAbs(imp.Module) || strings.HasPrefix(imp.Module, "~") || strings.Contains(imp.Module, "://") {
			return lang.Target{}
		}
		for _, b := range ancestors(dir) {
			if p := path.Join(b, imp.Module); inside(p) && r.files[p] {
				return lang.Target{Local: p}
			}
		}
	case kindSrcDir:
		for _, b := range ancestors(dir) {
			if p := path.Join(b, imp.Module); inside(p) && (r.dirs[p] || r.files[p]) {
				return lang.Target{Local: p}
			}
		}
	}
	return lang.Target{}
}

// ancestors are a directory and those above it, nearest first.
func ancestors(dir string) []string {
	out := []string{dir}
	for dir != "." {
		dir = path.Dir(dir)
		out = append(out, dir)
	}
	return out
}

func inside(p string) bool { return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p) }

// call resolves a function a file calls to the file of its scope (its package, or
// its project) that defines it. R has no per-file imports inside a package - every
// file of R/ sees every function of the package - so calls are what links them.
// A definition in the caller's own directory wins, then one in the package's R/;
// a name defined in several other places links nowhere rather than anywhere.
//
// Implements: REQ-R-010
func (r *resolver) call(file, name string) lang.Target {
	scope := r.scopeOf(file)
	candidates := r.defs[scope][name]
	if len(candidates) == 0 {
		return lang.Target{}
	}
	for _, c := range candidates {
		if c == file {
			return lang.Target{} // defined here too (a method, a redefinition)
		}
	}
	pick := func(keep func(string) bool) []string {
		var out []string
		for _, c := range candidates {
			if keep(c) {
				out = append(out, c)
			}
		}
		return out
	}
	for _, keep := range []func(string) bool{
		func(c string) bool { return path.Dir(c) == path.Dir(file) },
		func(c string) bool { return path.Dir(c) == path.Join(scope, "R") },
		func(string) bool { return true },
	} {
		switch got := pick(keep); len(got) {
		case 0:
			continue
		case 1:
			return lang.Target{Local: got[0]}
		default:
			return lang.Target{}
		}
	}
	return lang.Target{}
}

// pkg resolves a package name: the importing package itself (its R/ directory; its
// own files are not an import of it), another package of the repository, R's base
// packages, a recommended package the project neither declares nor locks, and else
// CRAN or Bioconductor as renv.lock or packrat.lock pinned it, or as the nearest
// DESCRIPTION declares it. manifest says the importer is a DESCRIPTION, whose
// imports of a local package point at that package's DESCRIPTION.
//
// Implements: REQ-R-006, REQ-R-007, REQ-R-009
func (r *resolver) pkg(file, name string, manifest bool) lang.Target {
	if own := r.pkgOf(file); own != nil && own.desc.name == name {
		if within(file, path.Join(own.dir, "R")) || manifest {
			return lang.Target{}
		}
		return r.local(own, false)
	}
	if p := r.named[name]; p != nil {
		return r.local(p, manifest)
	}
	if basePkgs[name] {
		return lang.Target{Ecosystem: ecoStd, Package: name}
	}
	var dep *dependency
	var rm *remote
	if d := r.descOf(file); d != nil {
		if x, ok := d.desc.deps[name]; ok {
			dep = &x
		}
		if x, ok := d.desc.remotes[name]; ok {
			rm = &x
		}
	}
	var lk *locked
	if l := r.lockOf(file); l != nil {
		lk = l.pkgs[name]
	}
	if recommendedPkgs[name] && dep == nil && lk == nil {
		return lang.Target{Ecosystem: ecoStd, Package: name}
	}
	eco := ecoCRAN
	if biocPkgs[name] || rm != nil && rm.bioc || lk != nil && lk.bioc {
		eco = ecoBioc
	}
	if lk != nil {
		return lockedTarget(eco, lk, dep)
	}
	t := lang.Target{Ecosystem: eco, Package: name}
	switch {
	case dep == nil:
		t.Unresolved = true
	case rm != nil && rm.origin != "":
		t.Origin, t.Version = rm.origin, rm.ref
		t.Pinned = lang.Commit(rm.ref)
		t.Floating = !t.Pinned
	default:
		t.Version = dep.constraint
		t.Pinned = lang.Pinned(dep.constraint)
		t.Floating = dep.constraint == ""
	}
	return t
}

// lockedTarget is a package as a lock recorded it: pinned at its version, a
// repository package's DESCRIPTION requirement kept as requested; a GitHub package
// by its commit; a local one by nothing.
//
// Implements: REQ-R-008, REQ-R-009
func lockedTarget(eco string, l *locked, dep *dependency) lang.Target {
	t := lang.Target{Ecosystem: eco, Package: l.name, Version: l.version}
	switch {
	case strings.HasPrefix(l.origin, "path:"):
		t.Origin = l.origin
	case l.origin != "":
		t.Origin = l.origin
		if l.sha != "" {
			t.Version, t.Pinned = l.sha, lang.Commit(l.sha)
		} else {
			t.Version, t.Floating = l.ref, true
		}
	default:
		t.Pinned = l.version != ""
		if dep != nil && dep.constraint != "" && dep.constraint != l.version {
			t.Requested = dep.constraint
		}
	}
	return t
}

// local is a package of the repository: its R/ directory for code, its DESCRIPTION
// for a DESCRIPTION.
func (r *resolver) local(p *rpkg, manifest bool) lang.Target {
	if rDir := path.Join(p.dir, "R"); !manifest && r.dirs[rDir] {
		return lang.Target{Local: rDir}
	}
	return lang.Target{Local: p.file}
}

// Dependencies answers --resolve-depth from the locks: what renv.lock (Requirements,
// or renv 1.1's Depends/Imports/LinkingTo) or packrat.lock (Requires) records for
// the package at the version it was locked, each as the same lock pinned it. Base
// packages are left out: they are R.
//
// Implements: REQ-R-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoCRAN && t.Ecosystem != ecoBioc {
		return nil
	}
	for _, l := range r.locks {
		lk := l.pkgs[t.Package]
		if lk == nil || t.Version != lk.version && t.Version != lk.sha {
			continue
		}
		var out []lang.Target
		for _, name := range lk.requires {
			if basePkgs[name] {
				continue
			}
			if dep := l.pkgs[name]; dep != nil {
				eco := ecoCRAN
				if dep.bioc || biocPkgs[name] {
					eco = ecoBioc
				}
				out = append(out, lockedTarget(eco, dep, nil))
				continue
			}
			if recommendedPkgs[name] {
				continue // shipped with R, and not locked
			}
			eco := ecoCRAN
			if biocPkgs[name] {
				eco = ecoBioc
			}
			out = append(out, lang.Target{Ecosystem: eco, Package: name})
		}
		return out
	}
	return nil
}
