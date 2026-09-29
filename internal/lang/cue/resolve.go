package cue

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
	gomodule "golang.org/x/mod/module"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// module is a directory with a cue.mod directory.
type module struct {
	root  string
	file  *moduleFile
	gomod *goMod // the go.mod at or above the module root
}

type goMod struct {
	directory, path string
	requires        map[string]string
}

type cueFile struct {
	path, packageName string
}

type resolver struct {
	root        string
	files       map[string]bool
	modules     map[string]*module
	order       []*module            // shallowest first
	packages    map[string][]cueFile // directory -> its CUE files with their package names
	directories sync.Map             // repository-relative directory -> exists on disk
	cache       string               // cue's cache directory, where fetched modules are extracted
}

// Implements: REQ-CUE-004, REQ-CUE-005, REQ-CUE-006, REQ-CUE-010
func newResolver(root string, all []*scan.File, cache string) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, modules: map[string]*module{}, packages: map[string][]cueFile{}, cache: cache}
	gomods := map[string]*goMod{}
	for _, f := range all {
		r.files[f.Path] = true
		segments := strings.Split(f.Path, "/")
		for i, s := range segments[:len(segments)-1] {
			if s == "cue.mod" {
				directory := "."
				if i > 0 {
					directory = strings.Join(segments[:i], "/")
				}
				if r.modules[directory] == nil {
					r.modules[directory] = &module{root: directory, file: &moduleFile{}}
				}
				break
			}
		}
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		switch {
		case path.Base(f.Path) == "go.mod":
			if goModFile := readGoMod(f); goModFile != nil {
				gomods[goModFile.directory] = goModFile
			}
		case path.Ext(f.Path) == ".cue" && !ignored(f.Path):
			directory := path.Dir(f.Path)
			r.packages[directory] = append(r.packages[directory], cueFile{f.Path, packageName(readFile(f.AbsolutePath, 64<<10))})
		}
	}
	for _, directory := range sortedKeys(r.modules) {
		m := r.modules[directory]
		r.order = append(r.order, m)
		if source := readFile(filepath.Join(root, filepath.FromSlash(directory), "cue.mod", "module.cue"), lang.MaxParseSize); source != nil {
			m.file = readModule(source)
		}
		for d := directory; ; d = path.Dir(d) {
			if goModFile := gomods[d]; goModFile != nil {
				m.gomod = goModFile
				break
			}
			if d == "." {
				break
			}
		}
	}
	sort.SliceStable(r.order, func(i, j int) bool {
		return depth(r.order[i].root) < depth(r.order[j].root)
	})
	return r
}

func depth(directory string) int {
	if directory == "." {
		return 0
	}
	return strings.Count(directory, "/") + 1
}

func readGoMod(f *scan.File) *goMod {
	data, err := os.ReadFile(f.AbsolutePath)
	if err != nil {
		return nil
	}
	parsed, err := modfile.ParseLax(f.Path, data, nil)
	if err != nil || parsed.Module == nil {
		return nil
	}
	goModFile := &goMod{directory: path.Dir(f.Path), path: parsed.Module.Mod.Path, requires: map[string]string{}}
	for _, require := range parsed.Require {
		goModFile.requires[require.Mod.Path] = require.Mod.Version
	}
	return goModFile
}

// packageName reads a CUE file's package clause.
func packageName(source []byte) string {
	for _, symbol := range extractSource(source).Symbols {
		if symbol.Kind == "package" {
			return symbol.Name
		}
	}
	return ""
}

// modulesOf lists the modules governing file: those whose directory holds
// it, nearest first; a file outside every module sees all of them,
// shallowest first.
func (r *resolver) modulesOf(file string) []*module {
	var out []*module
	for d := path.Dir(file); ; d = path.Dir(d) {
		if m := r.modules[d]; m != nil {
			out = append(out, m)
		}
		if d == "." {
			break
		}
	}
	if len(out) == 0 {
		return r.order
	}
	return out
}

func within(p, directory string) (string, bool) {
	if p == directory {
		return "", true
	}
	if directory == "." {
		return p, true
	}
	return strings.CutPrefix(p, directory+"/")
}

func join(directory, rest string) string {
	if rest == "" {
		return directory
	}
	return path.Join(directory, rest)
}

// isDirectory reports whether a repository directory exists on disk (the trees
// under cue.mod are not scanned).
func (r *resolver) isDirectory(p string) bool {
	if v, ok := r.directories.Load(p); ok {
		return v.(bool)
	}
	fileInfo, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(p)))
	ok := err == nil && fileInfo.IsDir()
	r.directories.Store(p, ok)
	return ok
}

// splitImport splits an import path into the path without major version
// suffixes and the package name (the :qualifier, else the last element).
func splitImport(spec string) (string, string) {
	p, q := spec, ""
	if i := strings.LastIndexByte(spec, ':'); i > strings.LastIndexByte(spec, '/') {
		p, q = spec[:i], spec[i+1:]
	}
	p = stripMajor(p)
	if q == "" {
		q = path.Base(p)
	}
	return p, q
}

// local lists the files of a module-local package: the files of the import's
// directory with that package name, in the governing module whose path the
// import starts with, else in any module of the repository it starts with
// (the longest). ok is false for any other import.
func (r *resolver) local(file, spec string) ([]string, bool) {
	p, packageName := splitImport(spec)
	var found *module
	rest := ""
	for _, m := range r.modulesOf(file) {
		if x, ok := within(p, m.file.path); ok && m.file.path != "" {
			found, rest = m, x
			break
		}
	}
	if found == nil {
		for _, m := range r.order {
			if x, ok := within(p, m.file.path); ok && m.file.path != "" && (found == nil || len(m.file.path) > len(found.file.path)) {
				found, rest = m, x
			}
		}
	}
	if found == nil {
		return nil, false
	}
	var out []string
	for _, f := range r.packages[join(found.root, rest)] {
		if f.packageName == packageName && len(out) < maxImport {
			out = append(out, f.path)
		}
	}
	return out, true
}

// Expand makes one import per file of a module-local package.
//
// Implements: REQ-CUE-004
func (r *resolver) Expand(file string, rawImport lang.RawImport) ([]lang.Import, bool) {
	if rawImport.Name != kindImport {
		return nil, false
	}
	files, _ := r.local(file, rawImport.Module)
	if len(files) == 0 {
		return nil, false
	}
	if len(files) == 1 {
		return []lang.Import{{Spec: rawImport.Spec, Line: rawImport.Line, Target: r.Resolve(file, rawImport)}}, true
	}
	out := make([]lang.Import, 0, len(files))
	for _, f := range files {
		out = append(out, lang.Import{Spec: rawImport.Spec + " (" + path.Base(f) + ")", Line: rawImport.Line, Target: lang.Target{Local: f}})
	}
	return out, true
}

// Implements: REQ-CUE-004, REQ-CUE-005, REQ-CUE-006, REQ-CUE-007
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	if rawImport.Name == kindDependency {
		if m := r.modules[path.Dir(path.Dir(file))]; m != nil {
			for _, d := range m.file.dependencies {
				if d.key == rawImport.Module {
					return dependencyTarget(d)
				}
			}
		}
		return lang.Target{}
	}
	files, own := r.local(file, rawImport.Module)
	if len(files) > 0 {
		return lang.Target{Local: files[0]}
	}
	p, _ := splitImport(rawImport.Module)
	if std[p] {
		return lang.Target{Ecosystem: ecosystemStd, Package: p} // builtin packages come first
	}
	for _, m := range r.modulesOf(file) {
		for _, subdirectory := range []string{"gen", "usr", "pkg"} {
			if !r.isDirectory(path.Join(m.root, "cue.mod", subdirectory, p)) {
				continue
			}
			if subdirectory == "pkg" {
				return r.vendored(m, p)
			}
			return r.generated(m, p)
		}
		var best *moduleDependency
		for _, d := range m.file.dependencies {
			if _, ok := within(p, d.path); ok && (best == nil || len(d.path) > len(best.path)) {
				best = d
			}
		}
		if best != nil {
			return dependencyTarget(best)
		}
	}
	if own {
		return lang.Target{} // the module's own package, not there
	}
	first, _, _ := strings.Cut(p, "/")
	if !strings.Contains(first, ".") {
		return lang.Target{Ecosystem: ecosystemCUE, Package: p, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecosystemCUE, Package: guess(p), Unresolved: true}
}

// dependencyTarget is a module.cue dependency, pinned by its exact version (the
// modules system selects versions deterministically, as Go's does).
//
// Implements: REQ-CUE-005
func dependencyTarget(d *moduleDependency) lang.Target {
	t := lang.Target{Ecosystem: ecosystemCUE, Package: d.path, Version: d.v, Pinned: d.v != ""}
	if d.v == "" {
		t.Floating = true
	}
	return t
}

// Dependencies is what a module dependency depends on, from its own module.cue
// in cue's module cache (mod/extract/<module>@<version>/cue.mod/module.cue, the
// path escaped as Go's module cache escapes it). A module the repository's
// module.cue lists too takes the version selected there: like Go's, a module
// file lists every module of the build at the version the build uses.
//
// Implements: REQ-CUE-011
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemCUE || !t.Pinned || r.cache == "" {
		return nil
	}
	p, err := gomodule.EscapePath(t.Package)
	if err != nil {
		return nil
	}
	v, err := gomodule.EscapeVersion(t.Version)
	if err != nil {
		return nil
	}
	source := readFile(filepath.Join(r.cache, "mod", "extract", filepath.FromSlash(p)+"@"+v, "cue.mod", "module.cue"), lang.MaxParseSize)
	if source == nil {
		return nil
	}
	var out []lang.Target
	for _, d := range readModule(source).dependencies {
		out = append(out, r.selected(d))
	}
	return out
}

// selected is a dependency at the version the repository's modules select for it.
func (r *resolver) selected(d *moduleDependency) lang.Target {
	for _, m := range r.order {
		for _, own := range m.file.dependencies {
			if own.path == d.path {
				return dependencyTarget(own)
			}
		}
	}
	return dependencyTarget(d)
}

// Installed says a module's dependencies come from cue's module cache.
func (r *resolver) Installed(t lang.Target) bool { return t.Ecosystem == ecosystemCUE }

var forges = map[string]bool{"github.com": true, "gitlab.com": true, "bitbucket.org": true, "codeberg.org": true, "git.sr.ht": true, "cue.dev": true}

// guess names a module by its import path: host/owner/repo on a forge (and
// cue.dev/x/<name> on CUE's registry), else host/first element.
func guess(p string) string {
	segments := strings.Split(p, "/")
	n := 2
	if forges[segments[0]] {
		n = 3
	}
	return strings.Join(segments[:min(n, len(segments))], "/")
}

// vendored is a package copied the old way into cue.mod/pkg: named by the
// module path of a cue.mod/module.cue inside it, else by its import path.
//
// Implements: REQ-CUE-006
func (r *resolver) vendored(m *module, p string) lang.Target {
	base := path.Join(m.root, "cue.mod", "pkg")
	for d := p; d != "." && d != ""; d = path.Dir(d) {
		if source := readFile(filepath.Join(r.root, filepath.FromSlash(path.Join(base, d)), "cue.mod", "module.cue"), 64<<10); source != nil {
			if modulePath := readModule(source).path; modulePath != "" {
				return lang.Target{Ecosystem: ecosystemCUE, Package: modulePath}
			}
		}
	}
	return lang.Target{Ecosystem: ecosystemCUE, Package: guess(p)}
}

// generated is a package `cue get go` wrote into cue.mod/gen (or its
// hand-written companion in cue.mod/usr): the Go module's own package
// directory, a module go.mod requires, the Go standard library, else a CUE
// package named by its path.
//
// Implements: REQ-CUE-006
func (r *resolver) generated(m *module, p string) lang.Target {
	if goModFile := m.gomod; goModFile != nil {
		if rest, ok := within(p, goModFile.path); ok {
			return lang.Target{Local: join(goModFile.directory, rest)}
		}
		best := ""
		for modulePath := range goModFile.requires {
			if _, ok := within(p, modulePath); ok && len(modulePath) > len(best) {
				best = modulePath
			}
		}
		if best != "" {
			v := goModFile.requires[best]
			return lang.Target{Ecosystem: ecosystemGo, Package: best, Version: v, Pinned: lang.Pinned(v)}
		}
	}
	if first, _, _ := strings.Cut(p, "/"); !strings.Contains(first, ".") {
		return lang.Target{Ecosystem: ecosystemGoStd, Package: p}
	}
	return lang.Target{Ecosystem: ecosystemCUE, Package: guess(p)}
}

// ModuleDependencies reads the deps of a module.cue - a module's own, as a CUE
// registry serves it beside the module's archive - as the targets the resolver
// makes of them: each module path without its major version, pinned by the
// version it names.
//
// Implements: REQ-CUE-012
func ModuleDependencies(source []byte) []lang.Target {
	var out []lang.Target
	for _, d := range readModule(source).dependencies {
		out = append(out, dependencyTarget(d))
	}
	return out
}
