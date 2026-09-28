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
	dir, path string
	requires  map[string]string
}

type cueFile struct {
	path, pkg string
}

type resolver struct {
	root    string
	files   map[string]bool
	modules map[string]*module
	order   []*module            // shallowest first
	pkgs    map[string][]cueFile // directory -> its CUE files with their package names
	dirs    sync.Map             // repository-relative directory -> exists on disk
	cache   string               // cue's cache directory, where fetched modules are extracted
}

// Implements: REQ-CUE-004, REQ-CUE-005, REQ-CUE-006, REQ-CUE-010
func newResolver(root string, all []*scan.File, cache string) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, modules: map[string]*module{}, pkgs: map[string][]cueFile{}, cache: cache}
	gomods := map[string]*goMod{}
	for _, f := range all {
		r.files[f.Path] = true
		segments := strings.Split(f.Path, "/")
		for i, s := range segments[:len(segments)-1] {
			if s == "cue.mod" {
				dir := "."
				if i > 0 {
					dir = strings.Join(segments[:i], "/")
				}
				if r.modules[dir] == nil {
					r.modules[dir] = &module{root: dir, file: &moduleFile{}}
				}
				break
			}
		}
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		switch {
		case path.Base(f.Path) == "go.mod":
			if gm := readGoMod(f); gm != nil {
				gomods[gm.dir] = gm
			}
		case path.Ext(f.Path) == ".cue" && !ignored(f.Path):
			dir := path.Dir(f.Path)
			r.pkgs[dir] = append(r.pkgs[dir], cueFile{f.Path, packageName(readFile(f.Abs, 64<<10))})
		}
	}
	for _, dir := range sortedKeys(r.modules) {
		m := r.modules[dir]
		r.order = append(r.order, m)
		if src := readFile(filepath.Join(root, filepath.FromSlash(dir), "cue.mod", "module.cue"), lang.MaxParseSize); src != nil {
			m.file = readModule(src)
		}
		for d := dir; ; d = path.Dir(d) {
			if gm := gomods[d]; gm != nil {
				m.gomod = gm
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

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

func readGoMod(f *scan.File) *goMod {
	data, err := os.ReadFile(f.Abs)
	if err != nil {
		return nil
	}
	mf, err := modfile.ParseLax(f.Path, data, nil)
	if err != nil || mf.Module == nil {
		return nil
	}
	gm := &goMod{dir: path.Dir(f.Path), path: mf.Module.Mod.Path, requires: map[string]string{}}
	for _, req := range mf.Require {
		gm.requires[req.Mod.Path] = req.Mod.Version
	}
	return gm
}

// packageName reads a CUE file's package clause.
func packageName(src []byte) string {
	for _, sym := range extractSource(src).Symbols {
		if sym.Kind == "package" {
			return sym.Name
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

func within(p, dir string) (string, bool) {
	if p == dir {
		return "", true
	}
	if dir == "." {
		return p, true
	}
	return strings.CutPrefix(p, dir+"/")
}

func join(dir, rest string) string {
	if rest == "" {
		return dir
	}
	return path.Join(dir, rest)
}

// isDir reports whether a repository directory exists on disk (the trees
// under cue.mod are not scanned).
func (r *resolver) isDir(p string) bool {
	if v, ok := r.dirs.Load(p); ok {
		return v.(bool)
	}
	st, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(p)))
	ok := err == nil && st.IsDir()
	r.dirs.Store(p, ok)
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
	p, pkg := splitImport(spec)
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
	for _, f := range r.pkgs[join(found.root, rest)] {
		if f.pkg == pkg && len(out) < maxImport {
			out = append(out, f.path)
		}
	}
	return out, true
}

// Expand makes one import per file of a module-local package.
//
// Implements: REQ-CUE-004
func (r *resolver) Expand(file string, imp lang.RawImport) ([]lang.Import, bool) {
	if imp.Name != kindImport {
		return nil, false
	}
	files, _ := r.local(file, imp.Module)
	if len(files) == 0 {
		return nil, false
	}
	if len(files) == 1 {
		return []lang.Import{{Spec: imp.Spec, Line: imp.Line, Target: r.Resolve(file, imp)}}, true
	}
	out := make([]lang.Import, 0, len(files))
	for _, f := range files {
		out = append(out, lang.Import{Spec: imp.Spec + " (" + path.Base(f) + ")", Line: imp.Line, Target: lang.Target{Local: f}})
	}
	return out, true
}

// Implements: REQ-CUE-004, REQ-CUE-005, REQ-CUE-006, REQ-CUE-007
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	if imp.Name == kindDep {
		if m := r.modules[path.Dir(path.Dir(file))]; m != nil {
			for _, d := range m.file.deps {
				if d.key == imp.Module {
					return depTarget(d)
				}
			}
		}
		return lang.Target{}
	}
	files, own := r.local(file, imp.Module)
	if len(files) > 0 {
		return lang.Target{Local: files[0]}
	}
	p, _ := splitImport(imp.Module)
	if std[p] {
		return lang.Target{Ecosystem: ecoStd, Package: p} // builtin packages come first
	}
	for _, m := range r.modulesOf(file) {
		for _, sub := range []string{"gen", "usr", "pkg"} {
			if !r.isDir(path.Join(m.root, "cue.mod", sub, p)) {
				continue
			}
			if sub == "pkg" {
				return r.vendored(m, p)
			}
			return r.generated(m, p)
		}
		var best *modDep
		for _, d := range m.file.deps {
			if _, ok := within(p, d.path); ok && (best == nil || len(d.path) > len(best.path)) {
				best = d
			}
		}
		if best != nil {
			return depTarget(best)
		}
	}
	if own {
		return lang.Target{} // the module's own package, not there
	}
	first, _, _ := strings.Cut(p, "/")
	if !strings.Contains(first, ".") {
		return lang.Target{Ecosystem: ecoCUE, Package: p, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecoCUE, Package: guess(p), Unresolved: true}
}

// depTarget is a module.cue dependency, pinned by its exact version (the
// modules system selects versions deterministically, as Go's does).
//
// Implements: REQ-CUE-005
func depTarget(d *modDep) lang.Target {
	t := lang.Target{Ecosystem: ecoCUE, Package: d.path, Version: d.v, Pinned: d.v != ""}
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
	if t.Ecosystem != ecoCUE || !t.Pinned || r.cache == "" {
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
	src := readFile(filepath.Join(r.cache, "mod", "extract", filepath.FromSlash(p)+"@"+v, "cue.mod", "module.cue"), lang.MaxParseSize)
	if src == nil {
		return nil
	}
	var out []lang.Target
	for _, d := range readModule(src).deps {
		out = append(out, r.selected(d))
	}
	return out
}

// selected is a dependency at the version the repository's modules select for it.
func (r *resolver) selected(d *modDep) lang.Target {
	for _, m := range r.order {
		for _, own := range m.file.deps {
			if own.path == d.path {
				return depTarget(own)
			}
		}
	}
	return depTarget(d)
}

// Installed says a module's dependencies come from cue's module cache.
func (r *resolver) Installed(t lang.Target) bool { return t.Ecosystem == ecoCUE }

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
		if src := readFile(filepath.Join(r.root, filepath.FromSlash(path.Join(base, d)), "cue.mod", "module.cue"), 64<<10); src != nil {
			if mp := readModule(src).path; mp != "" {
				return lang.Target{Ecosystem: ecoCUE, Package: mp}
			}
		}
	}
	return lang.Target{Ecosystem: ecoCUE, Package: guess(p)}
}

// generated is a package `cue get go` wrote into cue.mod/gen (or its
// hand-written companion in cue.mod/usr): the Go module's own package
// directory, a module go.mod requires, the Go standard library, else a CUE
// package named by its path.
//
// Implements: REQ-CUE-006
func (r *resolver) generated(m *module, p string) lang.Target {
	if gm := m.gomod; gm != nil {
		if rest, ok := within(p, gm.path); ok {
			return lang.Target{Local: join(gm.dir, rest)}
		}
		best := ""
		for mp := range gm.requires {
			if _, ok := within(p, mp); ok && len(mp) > len(best) {
				best = mp
			}
		}
		if best != "" {
			v := gm.requires[best]
			return lang.Target{Ecosystem: ecoGo, Package: best, Version: v, Pinned: lang.Pinned(v)}
		}
	}
	if first, _, _ := strings.Cut(p, "/"); !strings.Contains(first, ".") {
		return lang.Target{Ecosystem: ecoGoStd, Package: p}
	}
	return lang.Target{Ecosystem: ecoCUE, Package: guess(p)}
}
