package r

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// rpkg is a DESCRIPTION of the repository: an R package when it has a Package field,
// else a project declaring its dependencies.
type rpkg struct {
	directory, file string
	description     *description
}

type resolver struct {
	files        map[string]bool
	directories  map[string]bool
	descriptions []*rpkg          // deepest first
	named        map[string]*rpkg // R packages of the repository by name
	locks        []*lockfile      // deepest first
	// scopes are the directories whose files call each other's functions: every
	// package, and the projects outside packages (an .Rproj, renv.lock, _targets.R
	// or .here beside them), deepest first; "." is the last.
	scopes      []string
	definitions map[string]map[string][]string // scope -> function or class name -> files
}

// Implements: REQ-R-006, REQ-R-007, REQ-R-008, REQ-R-009, REQ-R-010
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, directories: map[string]bool{}, named: map[string]*rpkg{},
		definitions: map[string]map[string][]string{}}
	absolute := map[string]string{}
	var sources []*scan.File
	scopes := map[string]bool{".": true}
	// Where a lock may be: beside a DESCRIPTION or an .Rproj, where renv keeps its
	// activate.R, and the root - read from disk when it is not in the file list.
	lockDirectories := map[string]bool{".": true}
	for _, f := range all {
		r.files[f.Path] = true
		absolute[f.Path] = f.AbsolutePath
		for d := path.Dir(f.Path); d != "." && !r.directories[d]; d = path.Dir(d) {
			r.directories[d] = true
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
			lockDirectories[path.Dir(f.Path)] = true
		case path.Base(path.Dir(f.Path)) == "renv" || path.Base(path.Dir(f.Path)) == "packrat":
			lockDirectories[path.Dir(path.Dir(f.Path))] = true
		}
		if !(Plugin{}).Claims(f) || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		if base == "DESCRIPTION" {
			source, err := os.ReadFile(f.AbsolutePath)
			if err != nil {
				continue
			}
			if d := readDescription(source); d != nil {
				p := &rpkg{directory: path.Dir(f.Path), file: f.Path, description: d}
				r.descriptions = append(r.descriptions, p)
				if d.name != "" {
					scopes[p.directory] = true
					if q := r.named[d.name]; q == nil || lang.Depth(p.directory) < lang.Depth(q.directory) {
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
	sort.Slice(r.descriptions, func(i, j int) bool {
		return lang.DeepestFirst(r.descriptions[i].directory, r.descriptions[j].directory)
	})
	// renv.lock is committed by projects and packages alike; packrat keeps its lock in
	// packrat/. Either may be git-ignored, so what is on disk counts too.
	read := func(relative string) ([]byte, bool) {
		if a, ok := absolute[relative]; ok {
			data, err := os.ReadFile(a)
			return data, err == nil
		}
		if root == "" {
			return nil, false
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		return data, err == nil
	}
	for d := range lockDirectories {
		if source, ok := read(path.Join(d, "renv.lock")); ok {
			if l := readRenvLock(source, d); l != nil {
				r.locks = append(r.locks, l)
				scopes[d] = true
			}
		} else if source, ok := read(path.Join(d, "packrat", "packrat.lock")); ok {
			r.locks = append(r.locks, readPackratLock(source, d))
		}
	}
	sort.Slice(r.locks, func(i, j int) bool { return lang.DeepestFirst(r.locks[i].directory, r.locks[j].directory) })
	for s := range scopes {
		r.scopes = append(r.scopes, s)
	}
	sort.Slice(r.scopes, func(i, j int) bool { return lang.DeepestFirst(r.scopes[i], r.scopes[j]) })
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
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil || !lang.Parseable(f, source) {
			continue
		}
		extraction, _ := (Plugin{}).Extract(f, source)
		scope := r.scopeOf(f.Path)
		for _, s := range extraction.Symbols {
			if s.Kind != "function" && s.Kind != "class" && s.Kind != "generic" {
				continue
			}
			name, _, _ := strings.Cut(s.Name, "@")
			m := r.definitions[scope]
			if m == nil {
				m = map[string][]string{}
				r.definitions[scope] = m
			}
			if files := m[name]; len(files) == 0 || files[len(files)-1] != f.Path {
				m[name] = append(files, f.Path)
			}
		}
	}
}

func (r *resolver) scopeOf(file string) string {
	for _, s := range r.scopes {
		if lang.Within(file, s) {
			return s
		}
	}
	return "."
}

// descriptionOf is the nearest DESCRIPTION above a file; packageOf the nearest that is a package.
func (r *resolver) descriptionOf(file string) *rpkg {
	for _, p := range r.descriptions {
		if lang.Within(file, p.directory) {
			return p
		}
	}
	return nil
}

func (r *resolver) packageOf(file string) *rpkg {
	for _, p := range r.descriptions {
		if p.description.name != "" && lang.Within(file, p.directory) {
			return p
		}
	}
	return nil
}

func (r *resolver) lockOf(file string) *lockfile {
	for _, l := range r.locks {
		if lang.Within(file, l.directory) {
			return l
		}
	}
	return nil
}

// Implements: REQ-R-002, REQ-R-004, REQ-R-005, REQ-R-006, REQ-R-009, REQ-R-010
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, _, _ := strings.Cut(rawImport.Name, ":")
	directory := path.Dir(file)
	switch kind {
	case kindPackage, kindNS, kindRoxygen, kindBox, kindNSFile:
		return r.packageName(file, rawImport.Module, false)
	case kindDependency:
		return r.packageName(file, rawImport.Module, true)
	case kindCall:
		return r.call(file, rawImport.Module)
	case kindInclude:
		if f := path.Join(directory, rawImport.Module); r.files[f] {
			return lang.Target{Local: f}
		}
	case kindBoxLocal:
		module := rawImport.Module
		var bases []string
		if strings.HasPrefix(module, "./") || strings.HasPrefix(module, "../") {
			bases = []string{directory}
		} else {
			bases = slices.Collect(lang.DirectoryAndAncestors(directory)) // box.path: usually the project root
		}
		for _, b := range bases {
			p := path.Join(b, module)
			if !lang.Inside(p) {
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
		if path.IsAbs(rawImport.Module) || strings.HasPrefix(rawImport.Module, "~") || strings.Contains(rawImport.Module, "://") {
			return lang.Target{}
		}
		for b := range lang.DirectoryAndAncestors(directory) {
			if p := path.Join(b, rawImport.Module); lang.Inside(p) && r.files[p] {
				return lang.Target{Local: p}
			}
		}
	case kindSourceDirectory:
		for b := range lang.DirectoryAndAncestors(directory) {
			if p := path.Join(b, rawImport.Module); lang.Inside(p) && (r.directories[p] || r.files[p]) {
				return lang.Target{Local: p}
			}
		}
	}
	return lang.Target{}
}

// call resolves a function a file calls to the file of its scope (its package, or
// its project) that defines it. R has no per-file imports inside a package - every
// file of R/ sees every function of the package - so calls are what links them.
// A definition in the caller's own directory wins, then one in the package's R/;
// a name defined in several other places links nowhere rather than anywhere.
//
// Implements: REQ-R-010
func (r *resolver) call(file, name string) lang.Target {
	scope := r.scopeOf(file)
	candidates := r.definitions[scope][name]
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

// packageName resolves a package name: the importing package itself (its R/ directory; its
// own files are not an import of it), another package of the repository, R's base
// packages, a recommended package the project neither declares nor locks, and else
// CRAN or Bioconductor as renv.lock or packrat.lock pinned it, or as the nearest
// DESCRIPTION declares it. manifest says the importer is a DESCRIPTION, whose
// imports of a local package point at that package's DESCRIPTION.
//
// Implements: REQ-R-006, REQ-R-007, REQ-R-009
func (r *resolver) packageName(file, name string, manifest bool) lang.Target {
	if own := r.packageOf(file); own != nil && own.description.name == name {
		if lang.Within(file, path.Join(own.directory, "R")) || manifest {
			return lang.Target{}
		}
		return r.local(own, false)
	}
	if p := r.named[name]; p != nil {
		return r.local(p, manifest)
	}
	if basePackages[name] {
		return lang.Target{Ecosystem: ecosystemStd, Package: name}
	}
	var declared *dependency
	var declaredRemote *remote
	if d := r.descriptionOf(file); d != nil {
		if x, ok := d.description.dependencies[name]; ok {
			declared = &x
		}
		if x, ok := d.description.remotes[name]; ok {
			declaredRemote = &x
		}
	}
	var lockedPackage *locked
	if l := r.lockOf(file); l != nil {
		lockedPackage = l.packages[name]
	}
	if recommendedPackages[name] && declared == nil && lockedPackage == nil {
		return lang.Target{Ecosystem: ecosystemStd, Package: name}
	}
	ecosystem := ecosystemCRAN
	if biocPackages[name] || declaredRemote != nil && declaredRemote.bioc || lockedPackage != nil && lockedPackage.bioc {
		ecosystem = ecosystemBioc
	}
	if lockedPackage != nil {
		return lockedTarget(ecosystem, lockedPackage, declared)
	}
	t := lang.Target{Ecosystem: ecosystem, Package: name}
	switch {
	case declared == nil:
		t.Unresolved = true
	case declaredRemote != nil && declaredRemote.origin != "":
		t.Origin, t.Version = declaredRemote.origin, declaredRemote.reference
		t.Pinned = lang.Commit(declaredRemote.reference)
		t.Floating = !t.Pinned
	default:
		t.Version = declared.constraint
		t.Pinned = lang.Pinned(declared.constraint)
		t.Floating = declared.constraint == ""
	}
	return t
}

// lockedTarget is a package as a lock recorded it: pinned at its version, a
// repository package's DESCRIPTION requirement kept as requested; a GitHub package
// by its commit; a local one by nothing.
//
// Implements: REQ-R-008, REQ-R-009
func lockedTarget(ecosystem string, l *locked, declared *dependency) lang.Target {
	t := lang.Target{Ecosystem: ecosystem, Package: l.name, Version: l.version}
	switch {
	case strings.HasPrefix(l.origin, "path:"):
		t.Origin = l.origin
	case l.origin != "":
		t.Origin = l.origin
		if l.sha != "" {
			t.Version, t.Pinned = l.sha, lang.Commit(l.sha)
		} else {
			t.Version, t.Floating = l.reference, true
		}
	default:
		t.Pinned = l.version != ""
		if declared != nil && declared.constraint != "" && declared.constraint != l.version {
			t.Requested = declared.constraint
		}
	}
	return t
}

// local is a package of the repository: its R/ directory for code, its DESCRIPTION
// for a DESCRIPTION.
func (r *resolver) local(p *rpkg, manifest bool) lang.Target {
	if rDirectory := path.Join(p.directory, "R"); !manifest && r.directories[rDirectory] {
		return lang.Target{Local: rDirectory}
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
	if t.Ecosystem != ecosystemCRAN && t.Ecosystem != ecosystemBioc {
		return nil
	}
	for _, l := range r.locks {
		lockedPackage := l.packages[t.Package]
		if lockedPackage == nil || t.Version != lockedPackage.version && t.Version != lockedPackage.sha {
			continue
		}
		var out []lang.Target
		for _, name := range lockedPackage.requires {
			if basePackages[name] {
				continue
			}
			if dependency := l.packages[name]; dependency != nil {
				ecosystem := ecosystemCRAN
				if dependency.bioc || biocPackages[name] {
					ecosystem = ecosystemBioc
				}
				out = append(out, lockedTarget(ecosystem, dependency, nil))
				continue
			}
			if recommendedPackages[name] {
				continue // shipped with R, and not locked
			}
			ecosystem := ecosystemCRAN
			if biocPackages[name] {
				ecosystem = ecosystemBioc
			}
			out = append(out, lang.Target{Ecosystem: ecosystem, Package: name})
		}
		return out
	}
	return nil
}
