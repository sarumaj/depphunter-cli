package haskell

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// pkgInfo is a package of the repository: a .cabal file, or a package.yaml without one.
type pkgInfo struct {
	*cabalPkg
	project *projectInfo
}

// projectInfo is a directory building packages together - one with cabal.project,
// stack.yaml or cabal.project.freeze, or else a package's own directory - and what
// pins its dependencies there.
type projectInfo struct {
	dir     string
	pins    map[string]string
	plan    *buildPlan
	stack   *stackProject
	lock    *stackLock
	repos   map[string]srp
	extras  map[string]extraDep
	members []string // package directories cabal.project or stack.yaml name
}

type resolver struct {
	files    map[string]bool
	dirs     map[string]bool
	pkgs     []*pkgInfo // deepest first
	byName   map[string]*pkgInfo
	byDir    map[string]*pkgInfo
	projects []*projectInfo      // deepest first
	mods     map[string][]string // module name -> files declaring it
	boots    map[string][]string // module name -> .hs-boot files
}

// ignored are the directories cabal and stack build into.
func ignored(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "dist-newstyle" || seg == ".stack-work" {
			return true
		}
	}
	return false
}

// Implements: REQ-HASKELL-004, REQ-HASKELL-006, REQ-HASKELL-007, REQ-HASKELL-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, byName: map[string]*pkgInfo{},
		byDir: map[string]*pkgInfo{}, mods: map[string][]string{}, boots: map[string][]string{}}
	abs := map[string]string{}
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
	projDirs := map[string]bool{}
	var sources []*scan.File
	hpack := map[string]*scan.File{}
	for _, f := range all {
		if ignored(f.Path) {
			continue
		}
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		base := path.Base(f.Path)
		switch {
		case base == "cabal.project" || base == "stack.yaml" || base == "cabal.project.freeze":
			projDirs[path.Dir(f.Path)] = true
		case strings.HasSuffix(base, ".cabal") && !f.Binary && !f.TooLarge:
			src, err := os.ReadFile(f.Abs)
			if err != nil {
				continue
			}
			p := readCabal(src, f.Path)
			if p.name == "" {
				p.name = strings.TrimSuffix(base, ".cabal")
			}
			r.addPkg(p)
		case base == "package.yaml":
			hpack[path.Dir(f.Path)] = f
		case sourceExt(f.Path) != "" && !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize:
			sources = append(sources, f)
		}
	}
	for dir, f := range hpack {
		if r.byDir[dir] != nil {
			continue // stack and hpack commit the .cabal they generate: it is the same package
		}
		if src, err := os.ReadFile(f.Abs); err == nil {
			if p := readHpack(src, f.Path); p != nil && p.name != "" {
				r.addPkg(p)
			}
		}
	}
	sort.Slice(r.pkgs, func(i, j int) bool { return deeper(r.pkgs[i].dir, r.pkgs[j].dir) })
	for _, p := range r.pkgs {
		covered := false
		for d := range projDirs {
			if within(p.dir, d) || p.dir == d {
				covered = true
				break
			}
		}
		if !covered {
			projDirs[p.dir] = true
		}
	}
	for d := range projDirs {
		r.projects = append(r.projects, r.readProject(d, read))
	}
	sort.Slice(r.projects, func(i, j int) bool { return deeper(r.projects[i].dir, r.projects[j].dir) })
	for _, p := range r.pkgs {
		p.project = r.projectOf(p.file)
	}
	r.index(sources)
	return r
}

func (r *resolver) addPkg(p *cabalPkg) {
	info := &pkgInfo{cabalPkg: p}
	r.pkgs = append(r.pkgs, info)
	r.byDir[p.dir] = info
	if q := r.byName[p.name]; q == nil || depth(p.dir) < depth(q.dir) {
		r.byName[p.name] = info
	}
}

// readProject reads what pins a project's packages: cabal.project and the local
// project files it imports, with their packages, constraints and
// source-repository-package stanzas, cabal.project.freeze, the
// build plan cabal wrote to dist-newstyle/cache/plan.json, stack.yaml and
// stack.yaml.lock. The freeze file, the plan and the lock are often git-ignored, so
// they are read from disk too.
//
// Implements: REQ-HASKELL-007, REQ-HASKELL-008
func (r *resolver) readProject(dir string, read func(string) ([]byte, bool)) *projectInfo {
	p := &projectInfo{dir: dir, pins: map[string]string{}, repos: map[string]srp{}, extras: map[string]extraDep{}}
	pin := func(name, c string) {
		if _, ok := p.pins[name]; !ok && lang.Pinned(c) {
			p.pins[name] = c
		}
	}
	if src, ok := read(path.Join(dir, "cabal.project.freeze")); ok {
		for name, c := range readCabalProject(src).constraints {
			pin(name, c)
		}
	}
	for i, cp := range projectFiles(path.Join(dir, "cabal.project"), read) {
		for name, c := range cp.constraints {
			pin(name, c)
		}
		for _, s := range cp.repos {
			if _, dup := p.repos[s.name]; !dup || i == 0 {
				p.repos[s.name] = s
			}
		}
		for _, m := range cp.members {
			p.members = append(p.members, r.memberDirs(dir, m.text)...)
		}
	}
	if src, ok := read(path.Join(dir, "dist-newstyle", "cache", "plan.json")); ok {
		p.plan = readPlan(src)
	}
	if src, ok := read(path.Join(dir, "stack.yaml")); ok {
		p.stack = readStack(src)
		if p.stack != nil {
			for _, e := range p.stack.extras {
				if _, dup := p.extras[e.name]; !dup {
					p.extras[e.name] = e
				}
			}
			for _, m := range p.stack.members {
				p.members = append(p.members, path.Join(dir, m.text))
			}
		}
	}
	if src, ok := read(path.Join(dir, "stack.yaml.lock")); ok {
		p.lock = readStackLock(src)
	}
	if p.lock != nil {
		// A repository extra-dep is named after its repository until the lock
		// says which package it holds.
		for _, l := range p.lock.pkgs {
			for name, e := range p.extras {
				if l.origin != "" && e.origin == l.origin && name != l.name {
					e.name, e.version = l.name, l.version
					p.extras[l.name] = e
					p.extras[name] = e
				}
			}
		}
	}
	return p
}

// sourceExt is the extension of a Haskell source file the plugin reads, "" for any
// other file.
func sourceExt(p string) string {
	base := path.Base(p)
	if strings.HasSuffix(base, ".hs-boot") {
		return ".hs-boot"
	}
	switch ext := path.Ext(base); ext {
	case ".hs", ".lhs", ".hsc":
		return ext
	}
	return ""
}

// index learns which file declares each module from the files' module headers
// (extraction results are not shared with resolvers; the lexer is fast enough to read
// every file twice). A file without a header is a Main module and is not indexed.
//
// Implements: REQ-HASKELL-006
func (r *resolver) index(sources []*scan.File) {
	sort.Slice(sources, func(i, j int) bool { return sources[i].Path < sources[j].Path })
	for _, f := range sources {
		src, err := os.ReadFile(f.Abs)
		if err != nil || !lang.Parseable(f, src) {
			continue
		}
		mod := moduleName(src, sourceExt(f.Path) == ".lhs")
		if mod == "" || mod == "Main" {
			continue
		}
		if sourceExt(f.Path) == ".hs-boot" {
			r.boots[mod] = append(r.boots[mod], f.Path)
		} else {
			r.mods[mod] = append(r.mods[mod], f.Path)
		}
	}
}

// moduleName is the name a module header declares, "" without one.
func moduleName(src []byte, literate bool) string {
	if literate {
		src = unlit(src)
	}
	tokens := lex(src)
	for i, t := range tokens {
		switch {
		case t.k == tPragma:
			continue
		case t.k == tVar && t.s == "module" && i+1 < len(tokens) && tokens[i+1].k == tCon:
			return tokens[i+1].s
		}
		return ""
	}
	return ""
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

func deeper(a, b string) bool {
	if da, db := depth(a), depth(b); da != db {
		return da > db
	}
	return a < b
}

func within(file, dir string) bool { return dir == "." || strings.HasPrefix(file, dir+"/") }

func (r *resolver) pkgOf(file string) *pkgInfo {
	for _, p := range r.pkgs {
		if within(file, p.dir) {
			return p
		}
	}
	return nil
}

func (r *resolver) projectOf(file string) *projectInfo {
	for _, p := range r.projects {
		if within(file, p.dir) {
			return p
		}
	}
	return nil
}

// components are those of a package whose source directories hold file; all of them
// for a file in none (Setup.hs) and for the package's manifest.
func (p *pkgInfo) components(file string) []*component {
	var out []*component
	best := -1
	for _, c := range p.comps {
		n := -1
		for _, d := range c.dirs {
			if dir := path.Join(p.dir, d); within(file, dir) {
				n = max(n, depth(dir))
			}
		}
		switch {
		case n < 0:
		case n > best:
			best, out = n, []*component{c}
		case n == best:
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return p.comps
	}
	return out
}

// declared are the packages a file's components depend on, by name, with the first
// constraint written for each.
func (r *resolver) declared(file string, own *pkgInfo, manifest bool) map[string]dep {
	out := map[string]dep{}
	if own == nil {
		return out
	}
	comps := own.comps
	if setup(file, own) {
		// Setup.hs is built against its custom-setup's setup-depends, or else
		// against Cabal and base.
		comps = []*component{own.setup}
		if own.setup == nil {
			return map[string]dep{"Cabal": {name: "Cabal"}, "base": {name: "base"}}
		}
	} else if !manifest {
		comps = own.components(file)
	} else if own.setup != nil {
		comps = append(append([]*component(nil), comps...), own.setup)
	}
	for _, c := range comps {
		for _, d := range c.deps {
			if prev, ok := out[d.name]; !ok || prev.constraint == "" && d.constraint != "" {
				out[d.name] = d
			}
		}
	}
	return out
}

// setup reports whether file is a package's Setup script.
func setup(file string, own *pkgInfo) bool {
	base := path.Base(file)
	return path.Dir(file) == own.dir && (base == "Setup.hs" || base == "Setup.lhs")
}

// Implements: REQ-HASKELL-002, REQ-HASKELL-005, REQ-HASKELL-006, REQ-HASKELL-007, REQ-HASKELL-009
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch {
	case imp.Name == kindImport || imp.Name == kindSource || strings.HasPrefix(imp.Name, kindPkg):
		return r.module(file, imp)
	case imp.Name == kindDep:
		own := r.byDir[path.Dir(file)]
		if own == nil {
			own = r.pkgOf(file)
		}
		if own != nil && imp.Module == own.name {
			return lang.Target{} // a component using the package's own library
		}
		if lp := r.byName[imp.Module]; lp != nil {
			return lang.Target{Local: lp.file}
		}
		return r.pkgTarget(file, own, imp.Module, true)
	case imp.Name == kindMember:
		return r.member(file, imp.Module)
	case imp.Name == kindInclude:
		return r.include(file, imp.Module)
	case imp.Name == kindRepo:
		if p := r.projectOf(file); p != nil {
			if s, ok := p.repos[imp.Module]; ok {
				return originTarget(imp.Module, s.location, s.tag)
			}
		}
	case imp.Name == kindExtra:
		p := r.projectOf(file)
		if p == nil {
			return lang.Target{}
		}
		e, ok := p.extras[imp.Module]
		if !ok {
			return lang.Target{}
		}
		if dir, ok := strings.CutPrefix(e.origin, "path:"); ok {
			if lp := r.byDir[path.Join(p.dir, dir)]; lp != nil {
				return lang.Target{Local: lp.file}
			}
			return lang.Target{Ecosystem: ecoHackage, Package: e.name, Origin: e.origin}
		}
		if l, ok := p.lock.get(e.name); ok && e.origin != "" && l.origin != "" {
			e = l
		}
		if e.origin != "" {
			return originTarget(e.name, e.origin, e.version)
		}
		return lang.Target{Ecosystem: ecoHackage, Package: e.name, Version: e.version, Pinned: lang.Pinned(e.version)}
	}
	return lang.Target{}
}

func (l *stackLock) get(name string) (extraDep, bool) {
	if l == nil {
		return extraDep{}, false
	}
	e, ok := l.pkgs[name]
	return e, ok
}

// originTarget is a package built from a repository at a tag or commit: pinned by a
// commit only.
func originTarget(name, location, ref string) lang.Target {
	t := lang.Target{Ecosystem: ecoHackage, Package: name, Origin: location, Version: ref}
	t.Pinned = lang.Commit(ref)
	t.Floating = !t.Pinned
	return t
}

// member resolves an entry of cabal.project's or stack.yaml's packages: a package
// directory or .cabal file to its package description. Globs are expanded by
// Expand.
func (r *resolver) member(file, entry string) lang.Target {
	if strings.ContainsAny(entry, "*?[{") || strings.Contains(entry, "://") {
		return lang.Target{}
	}
	p := path.Join(path.Dir(file), strings.TrimSuffix(entry, "/"))
	if strings.HasSuffix(p, ".cabal") && r.files[p] {
		return lang.Target{Local: p}
	}
	if lp := r.byDir[p]; lp != nil {
		return lang.Target{Local: lp.file}
	}
	if r.dirs[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// module resolves an imported module: to the file of the project that declares it
// (the importer's own package first, then the packages of its project and those it
// depends on), and else to the package providing it (external).
//
// Implements: REQ-HASKELL-006, REQ-HASKELL-009
func (r *resolver) module(file string, imp lang.RawImport) lang.Target {
	mod := imp.Module
	own := r.pkgOf(file)
	boot := imp.Name == kindSource
	if pkg, ok := strings.CutPrefix(imp.Name, kindPkg); ok {
		// PackageImports names the package: only its modules are candidates.
		var in *pkgInfo
		switch {
		case pkg == "this" || own != nil && pkg == own.name:
			in = own
		case r.byName[pkg] != nil:
			in = r.byName[pkg]
		default:
			return r.pkgTarget(file, own, pkg, false)
		}
		if in == nil {
			return lang.Target{}
		}
		if f := r.localIn(file, mod, boot, func(p *pkgInfo) bool { return p == in }); f != "" {
			return local(file, f)
		}
		if f := r.probe(file, in, mod, boot); f != "" {
			return local(file, f)
		}
		if in == own {
			return lang.Target{}
		}
		return lang.Target{Local: in.file}
	}
	if f := r.local(file, own, mod, boot); f != "" {
		return local(file, f)
	}
	if mod == "Main" {
		return lang.Target{} // a program, not a library module
	}
	for _, prefix := range []string{"Paths_", "PackageInfo_", "Build_"} {
		// Modules cabal (or a package's Setup.hs) generates from the package
		// description.
		if name, ok := strings.CutPrefix(mod, prefix); ok {
			if lp := r.byName[strings.ReplaceAll(name, "_", "-")]; lp != nil {
				return lang.Target{Local: lp.file}
			}
			return lang.Target{}
		}
	}
	return r.pkgTarget(file, own, r.packageOf(file, own, mod), false)
}

func local(file, f string) lang.Target {
	if f == file {
		return lang.Target{}
	}
	return lang.Target{Local: f}
}

// local finds the file declaring mod: in the importer's own package (its component's
// source directories first, then a generated module's source - .y, .x, .chs - probed
// there), else in a package of the same project or one the importer depends on. A
// file of no package takes the nearest declaration anywhere.
func (r *resolver) local(file string, own *pkgInfo, mod string, boot bool) string {
	if own == nil {
		return r.localIn(file, mod, boot, nil)
	}
	if f := r.localIn(file, mod, boot, func(p *pkgInfo) bool { return p == own }); f != "" {
		return f
	}
	if f := r.probe(file, own, mod, boot); f != "" {
		return f
	}
	declared := r.declared(file, own, false)
	return r.localIn(file, mod, boot, func(p *pkgInfo) bool {
		if p == nil {
			return false
		}
		_, dep := declared[p.name]
		return dep || p.project != nil && p.project == own.project || own.project != nil && member(own.project, p)
	})
}

func member(proj *projectInfo, p *pkgInfo) bool {
	for _, m := range proj.members {
		if m == p.dir || m == p.file {
			return true
		}
	}
	return false
}

// localIn picks among the files declaring mod those whose package keep accepts (nil
// accepts all): one in the importer's component directories, else the nearest.
func (r *resolver) localIn(file, mod string, boot bool, keep func(*pkgInfo) bool) string {
	candidates := r.mods[mod]
	if boot {
		candidates = append(append([]string(nil), r.boots[mod]...), candidates...)
	}
	var ok []string
	for _, c := range candidates {
		if keep == nil || keep(r.pkgOf(c)) {
			ok = append(ok, c)
		}
	}
	if len(ok) == 0 {
		return ""
	}
	if own := r.pkgOf(file); own != nil {
		for _, comp := range own.components(file) {
			for _, d := range comp.dirs {
				for _, c := range ok {
					if within(c, path.Join(own.dir, d)) {
						return c
					}
				}
			}
		}
	}
	best, bestLen := ok[0], -1
	for _, c := range ok {
		if n := common(file, c); n > bestLen {
			best, bestLen = c, n
		}
	}
	return best
}

// common is the number of leading directories two paths share.
func common(a, b string) int {
	as, bs := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// probe looks for the source of a module under the importer's component directories
// by its path, for the modules no header declares: those generated from Alex, Happy,
// c2hs or hsc2hs sources.
func (r *resolver) probe(file string, own *pkgInfo, mod string, boot bool) string {
	rel := strings.ReplaceAll(mod, ".", "/")
	exts := []string{".hs", ".lhs", ".hsc", ".y", ".ly", ".x", ".chs", ".hsig"}
	if boot {
		exts = append([]string{".hs-boot"}, exts...)
	}
	for _, c := range own.components(file) {
		for _, d := range c.dirs {
			for _, ext := range exts {
				if p := path.Join(own.dir, d, rel+ext); r.files[p] {
					return p
				}
			}
		}
	}
	return ""
}

// packageOf names the package providing a module no project file declares, in this
// order: the curated table's entry for exactly that module when the package is
// GHC's own or declared; the longest table entry among declared packages; a declared
// package named by a run of the module's segments (Network.HTTP.Client:
// http-client); the longest table entry at all; a package the project's plan, freeze
// file or lock names by such a run; and else the module's first segment that does
// not name a subject, in lower case.
//
// Implements: REQ-HASKELL-009
func (r *resolver) packageOf(file string, own *pkgInfo, mod string) string {
	declared := r.declared(file, own, false)
	isDeclared := func(p string) bool { _, ok := declared[p]; return ok && !stdPkgs[p] }
	for _, p := range moduleTable[mod] {
		if stdPkgs[p] || isDeclared(p) {
			return p
		}
	}
	if _, ps := tableMatch(mod, isDeclared); len(ps) > 0 {
		return ps[0]
	}
	names := map[string]string{}
	for name := range declared {
		if !stdPkgs[name] {
			names[fold(name)] = name
		}
	}
	if p := runMatch(mod, names); p != "" {
		return p
	}
	proj := r.projectOf(file)
	if _, ps := tableMatch(mod, nil); len(ps) > 0 {
		for _, p := range ps {
			if proj.pinned(p) {
				return p
			}
		}
		return ps[0]
	}
	// A declared package named after its first word (hermes-json for Data.Hermes).
	firsts := map[string]string{}
	for name := range declared {
		first, _, _ := strings.Cut(name, "-")
		if !stdPkgs[name] && first != name {
			if _, dup := firsts[fold(first)]; dup {
				firsts[fold(first)] = ""
			} else {
				firsts[fold(first)] = name
			}
		}
	}
	if p := runMatch(mod, firsts); p != "" {
		return p
	}
	if proj != nil {
		known := map[string]string{}
		for _, name := range proj.names() {
			known[fold(name)] = name
		}
		if p := runMatch(mod, known); p != "" {
			return p
		}
	}
	return guessName(mod)
}

// pinned reports whether the project's plan, freeze file, constraints, lock or
// extra-deps name a package.
func (p *projectInfo) pinned(name string) bool {
	if p == nil {
		return false
	}
	_, a := p.pins[name]
	_, b := p.extras[name]
	_, c := p.lock.get(name)
	_, d := p.repos[name]
	e := p.plan != nil && p.plan.pkgs[name] != nil
	return a || b || c || d || e
}

func (p *projectInfo) names() []string {
	var out []string
	for n := range p.pins {
		out = append(out, n)
	}
	for n := range p.extras {
		out = append(out, n)
	}
	if p.lock != nil {
		for n := range p.lock.pkgs {
			out = append(out, n)
		}
	}
	if p.plan != nil {
		for n := range p.plan.pkgs {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// pkgTarget is a package as the importer's project pins it: GHC's own packages to
// haskell-std; a package of the repository to its description; and else a Hackage
// package at the version the build plan, the freeze file or an exact constraint of
// cabal.project, a source-repository-package, stack.yaml.lock or stack.yaml's
// extra-deps fix (pinned, the declared range kept as requested), else as build-depends
// declares it: an exact ==x pins, a range does not, no version floats unless a stack
// snapshot fixes it (then its version is the snapshot's, unknown offline, and shown
// as the snapshot's name). A package nothing declares or pins is unresolved.
//
// Implements: REQ-HASKELL-007, REQ-HASKELL-008, REQ-HASKELL-009, REQ-HASKELL-010
func (r *resolver) pkgTarget(file string, own *pkgInfo, name string, manifest bool) lang.Target {
	if stdPkgs[name] {
		return lang.Target{Ecosystem: ecoStd, Package: name}
	}
	if lp := r.byName[name]; lp != nil {
		return lang.Target{Local: lp.file}
	}
	d, declared := r.declared(file, own, manifest)[name]
	if !declared && !manifest && own != nil {
		// Declared by another component of the package: the flag or stanza that
		// guards it is not evaluated, so it counts.
		d, declared = r.declared(file, own, true)[name]
	}
	proj := r.projectOf(file)
	if own != nil && own.project != nil {
		proj = own.project
	}
	t := lang.Target{Ecosystem: ecoHackage, Package: name}
	exact := func(v string) lang.Target {
		t.Version, t.Pinned = v, true
		if declared && d.constraint != "" && d.constraint != v {
			t.Requested = d.constraint
		}
		return t
	}
	if proj != nil {
		if proj.plan != nil {
			if pk := proj.plan.pkgs[name]; pk != nil && !pk.local {
				if pk.origin != "" {
					return originTarget(name, pk.origin, pk.tag)
				}
				return exact(pk.version)
			}
		}
		if v, ok := proj.pins[name]; ok {
			return exact(v)
		}
		if s, ok := proj.repos[name]; ok {
			return originTarget(name, s.location, s.tag)
		}
		if l, ok := proj.lock.get(name); ok {
			if l.origin != "" {
				return originTarget(name, l.origin, l.version)
			}
			return exact(l.version)
		}
		if e, ok := proj.extras[name]; ok {
			switch {
			case strings.HasPrefix(e.origin, "path:"):
				t.Origin = e.origin
				return t
			case e.origin != "":
				return originTarget(name, e.origin, e.version)
			default:
				return exact(e.version)
			}
		}
	}
	if !declared {
		t.Unresolved = true
		return t
	}
	t.Version, t.Pinned = d.constraint, lang.Pinned(d.constraint)
	if d.constraint == "" {
		if proj != nil && proj.stack != nil && proj.stack.snapshot != "" {
			t.Version = proj.stack.snapshot
		} else {
			t.Floating = true
		}
	}
	return t
}

// Dependencies answers --resolve-depth from cabal's build plan: what plan.json says a
// package depends on, each at the version the plan chose. GHC's own packages and the
// project's are left out. stack.yaml.lock and cabal.project.freeze list versions
// only, not edges.
//
// Implements: REQ-HASKELL-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoHackage {
		return nil
	}
	for _, p := range r.projects {
		if p.plan == nil {
			continue
		}
		pk := p.plan.pkgs[t.Package]
		if pk == nil || t.Version != pk.version && t.Version != pk.tag {
			continue
		}
		var out []lang.Target
		for _, name := range pk.depends {
			dep := p.plan.pkgs[name]
			if stdPkgs[name] || dep == nil || dep.local {
				continue
			}
			if dep.origin != "" {
				out = append(out, originTarget(name, dep.origin, dep.tag))
				continue
			}
			out = append(out, lang.Target{Ecosystem: ecoHackage, Package: name, Version: dep.version, Pinned: true})
		}
		return out
	}
	return nil
}
