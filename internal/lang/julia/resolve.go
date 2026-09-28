package julia

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/juliapkg"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	files     map[string]bool
	dirs      map[string]bool // directories holding files
	projects  []*project      // shallowest first
	byDir     map[string]*project
	byName    map[string][]*project
	byUUID    map[string][]*project
	byFile    map[string]*project
	manifests map[string]*manifest // by file, those claimed
	all       []*manifest          // every manifest read, by path
	// modules maps a module's absolute path (Shop.Internal) to the files defining
	// it; ctx is the module path a file's top level is in, from the include graph.
	modules map[string][]string
	ctx     map[string]string
}

// readable reports whether a file is small enough to read here.
func readable(f *scan.File) bool {
	return !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize
}

// newResolver reads every project, manifest and source: sources for the modules
// they define and the files they include, which place each file in a module.
//
// Implements: REQ-JULIA-004, REQ-JULIA-005, REQ-JULIA-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, byDir: map[string]*project{},
		byName: map[string][]*project{}, byUUID: map[string][]*project{}, byFile: map[string]*project{},
		manifests: map[string]*manifest{}, modules: map[string][]string{}, ctx: map[string]string{}}
	var sources []*scan.File
	dirManifest := map[string]*manifest{}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); ; d = path.Dir(d) {
			if r.dirs[d] {
				break
			}
			r.dirs[d] = true
			if d == "." {
				break
			}
		}
		if !readable(f) {
			continue
		}
		switch fileClass(f.Path) {
		case classSource:
			sources = append(sources, f)
		case classProject:
			src, err := os.ReadFile(f.Abs)
			if err != nil {
				continue
			}
			p := readProject(src)
			if p == nil {
				continue
			}
			p.file, p.dir = f.Path, path.Dir(f.Path)
			// JuliaProject.toml takes precedence over Project.toml, as in Pkg.
			if have := r.byDir[p.dir]; have != nil && path.Base(have.file) == "JuliaProject.toml" {
				continue
			}
			r.byDir[p.dir] = p
		}
	}
	// Manifests are often not committed; they are looked for on disk beside every
	// project too.
	for dir := range r.byDir {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			continue
		}
		best := ""
		for _, e := range entries {
			if !e.IsDir() && manifestName(e.Name()) && manifestRank(e.Name()) > manifestRank(best) {
				best = e.Name()
			}
		}
		if best == "" {
			continue
		}
		file := path.Join(dir, best)
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil || len(src) > 8*lang.MaxParseSize {
			continue
		}
		m := readManifest(src)
		m.file, m.dir = file, dir
		dirManifest[dir] = m
	}
	for _, f := range all {
		if fileClass(f.Path) != classManifest || !readable(f) {
			continue
		}
		if m := dirManifest[path.Dir(f.Path)]; m != nil && m.file == f.Path {
			r.manifests[f.Path] = m
			continue
		}
		src, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		m := readManifest(src)
		m.file, m.dir = f.Path, path.Dir(f.Path)
		r.manifests[f.Path] = m
		if dirManifest[m.dir] == nil {
			dirManifest[m.dir] = m
		}
	}
	for _, m := range dirManifest {
		r.all = append(r.all, m)
	}
	sort.Slice(r.all, func(i, j int) bool { return r.all[i].file < r.all[j].file })
	for _, p := range r.byDir {
		r.projects = append(r.projects, p)
		// The manifest resolving a project is its own, else the nearest above it (a
		// workspace's, or the repository's environment).
		for d := p.dir; ; d = path.Dir(d) {
			if m := dirManifest[d]; m != nil {
				p.manifest = m
				break
			}
			if d == "." {
				break
			}
		}
	}
	sort.Slice(r.projects, func(i, j int) bool {
		di, dj := depth(r.projects[i].dir), depth(r.projects[j].dir)
		if di != dj {
			return di < dj
		}
		return r.projects[i].dir < r.projects[j].dir
	})
	for _, p := range r.projects {
		r.byFile[p.file] = p
		if p.name != "" {
			r.byName[p.name] = append(r.byName[p.name], p)
		}
		if p.uuid != "" {
			r.byUUID[p.uuid] = append(r.byUUID[p.uuid], p)
		}
	}
	r.readSources(sources)
	return r
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// manifestRank orders the manifests of one directory as Pkg prefers them: a
// versioned one (the newest version) over a plain one, JuliaManifest over Manifest.
func manifestRank(name string) int {
	if name == "" {
		return 0
	}
	rank := 1
	if strings.HasPrefix(name, "Julia") {
		rank++
	}
	stem := strings.TrimSuffix(strings.TrimPrefix(name, "Julia"), ".toml")
	if v, ok := strings.CutPrefix(stem, "Manifest-v"); ok {
		if pv, ok := juliapkg.ParseVersion(v); ok {
			rank += 10 + pv.At(0)*100000 + pv.At(1)*100
		}
	}
	return rank
}

// readSources places every source file in a module: a file included from inside
// module Shop has Shop as its top level, so the modules it defines are Shop.X.
func (r *resolver) readSources(files []*scan.File) {
	type parsed struct {
		file string
		src  *source
	}
	var list []parsed
	for _, f := range files {
		src, err := os.ReadFile(f.Abs)
		if err != nil || !lang.Parseable(f, src) {
			continue
		}
		list = append(list, parsed{f.Path, readSource(src)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].file < list[j].file })
	type parent struct{ file, module string }
	includedBy := map[string]parent{}
	for _, p := range list {
		for _, inc := range p.src.includes {
			child := path.Join(path.Dir(p.file), inc.path)
			if _, ok := includedBy[child]; !ok && r.files[child] && child != p.file {
				includedBy[child] = parent{p.file, inc.module}
			}
		}
	}
	var ctxOf func(file string, seen map[string]bool) string
	ctxOf = func(file string, seen map[string]bool) string {
		if c, ok := r.ctx[file]; ok {
			return c
		}
		c := ""
		if par, ok := includedBy[file]; ok && !seen[file] {
			seen[file] = true
			c = joinModule(ctxOf(par.file, seen), par.module)
		}
		r.ctx[file] = c
		return c
	}
	for _, p := range list {
		c := ctxOf(p.file, map[string]bool{})
		for _, m := range p.src.modules {
			key := joinModule(c, m)
			r.modules[key] = append(r.modules[key], p.file)
		}
	}
}

func joinModule(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "." + b
}

func splitModule(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ".")
}

// moduleFile is the file defining a module, the one nearest to from when several do.
func (r *resolver) moduleFile(key, from string) string {
	best, bestLen := "", -1
	for _, f := range r.modules[key] {
		if n := commonDir(f, from); n > bestLen {
			best, bestLen = f, n
		}
	}
	return best
}

// commonDir counts the leading directories two paths share.
func commonDir(a, b string) int {
	as, bs := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// local targets a file: none for the importer itself.
func local(p, file string) lang.Target {
	if p == "" || p == file {
		return lang.Target{}
	}
	return lang.Target{Local: p}
}

func std(name string) lang.Target { return lang.Target{Ecosystem: ecoStd, Package: name} }

// Resolve maps an import to its target.
//
// Implements: REQ-JULIA-004, REQ-JULIA-005, REQ-JULIA-006, REQ-JULIA-008, REQ-JULIA-010
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, arg, _ := strings.Cut(imp.Name, "\n")
	switch kind {
	case kindInclude:
		if imp.Module == "" {
			return lang.Target{}
		}
		p := path.Join(path.Dir(file), imp.Module)
		if !r.files[p] {
			return lang.Target{}
		}
		return local(p, file)
	case kindUsing:
		return r.module(file, arg, imp.Module)
	case kindDep:
		p := r.byFile[file]
		if p == nil {
			return lang.Target{}
		}
		return r.declared(p, imp.Module, p.deps[imp.Module], []string{imp.Module}, file)
	case kindMember:
		d := path.Join(path.Dir(file), imp.Module)
		if p := r.byDir[d]; p != nil {
			return local(p.file, file)
		}
		if r.dirs[d] {
			return lang.Target{Local: d}
		}
		return lang.Target{}
	case kindExt:
		dir := path.Dir(file)
		for _, c := range []string{"ext/" + imp.Module + ".jl", "ext/" + imp.Module + "/" + imp.Module + ".jl"} {
			if p := path.Join(dir, c); r.files[p] {
				return lang.Target{Local: p}
			}
		}
		return lang.Target{}
	case kindManifest:
		m := r.manifests[file]
		if e := m.find(imp.Module, arg); e != nil {
			return r.entry(m, e, []string{e.name}, file)
		}
	}
	return lang.Target{}
}

// ancestors are the projects of the file's directory and those above it, nearest
// first.
func (r *resolver) ancestors(file string) []*project {
	var out []*project
	for d := path.Dir(file); ; d = path.Dir(d) {
		if p := r.byDir[d]; p != nil {
			out = append(out, p)
		}
		if d == "." || d == "/" {
			break
		}
	}
	return out
}

// governing are the projects whose dependencies a file may use: its own and those
// above it, nearest first; a file outside every project may use any.
func (r *resolver) governing(file string) []*project {
	if a := r.ancestors(file); len(a) > 0 {
		return a
	}
	return r.projects
}

// module resolves a module path: relative ones through the module tree, absolute
// ones to the package, a declared dependency or the standard library.
func (r *resolver) module(file, within, spec string) lang.Target {
	dots := len(spec) - len(strings.TrimLeft(spec, "."))
	segments := splitModule(spec[dots:])
	if dots > 0 {
		return r.relative(file, within, dots, segments)
	}
	if len(segments) == 0 {
		return lang.Target{}
	}
	first := segments[0]
	switch first {
	case "Base", "Core":
		return std(first)
	case "Main":
		// The session's module: Main.TestUtilities is a module some script
		// defined at its top level.
		for n := len(segments); n >= 2; n-- {
			if f := r.moduleFile(strings.Join(segments[1:n], "."), file); f != "" {
				return local(f, file)
			}
		}
		return lang.Target{}
	}
	for _, p := range r.ancestors(file) {
		if p.name == first {
			return r.local(p, segments, file)
		}
	}
	gov := r.governing(file)
	for _, p := range gov {
		if uuid, ok := p.deps[first]; ok {
			return r.declared(p, first, uuid, segments, file)
		}
	}
	if juliapkg.Stdlib(first) {
		return std(first)
	}
	for _, p := range gov {
		if e := p.manifest.find(first, ""); e != nil {
			return r.entry(p.manifest, e, segments, file)
		}
	}
	if ps := r.byName[first]; len(ps) > 0 {
		best := ps[0]
		for _, p := range ps[1:] {
			if commonDir(p.file, file) > commonDir(best.file, file) {
				best = p
			}
		}
		return r.local(best, segments, file)
	}
	// A module defined at the top of a script's session (include("x.jl") then
	// using X without the dot): the file defining it.
	if f := r.moduleFile(first, file); f != "" && r.ctx[file] == "" {
		return local(f, file)
	}
	for _, p := range r.projects {
		if uuid, ok := p.deps[first]; ok {
			return r.declared(p, first, uuid, segments, file)
		}
	}
	return lang.Target{Ecosystem: ecoJulia, Package: first, Unresolved: true, Floating: true}
}

// relative resolves using .Sub / ..Parent.X: one dot is the module the statement
// is in, each further dot its parent.
func (r *resolver) relative(file, within string, dots int, segments []string) lang.Target {
	base := splitModule(joinModule(r.ctx[file], within))
	// A package's top module is its own parent, and Main is Main's: going up
	// stops there (Documenter.HTMLWriter's using ...DOM is Documenter.DOM).
	n := len(base) - (dots - 1)
	if n < 1 {
		n = min(1, len(base))
	}
	base = base[:n]
	try := func(base, segments []string) (lang.Target, bool) {
		for n := len(segments); n >= 0; n-- {
			key := strings.Join(append(append([]string{}, base...), segments[:n]...), ".")
			if key == "" {
				continue
			}
			if f := r.moduleFile(key, file); f != "" {
				return local(f, file), true
			}
			if n == 0 {
				break
			}
		}
		return lang.Target{}, false
	}
	if t, ok := try(base, segments); ok && len(segments) > 0 {
		return t
	}
	// A module's own name is bound inside it: using ..Shop from Shop.Sub is Shop.
	if len(base) > 0 && len(segments) > 0 && base[len(base)-1] == segments[0] {
		if t, ok := try(base, segments[1:]); ok {
			return t
		}
	}
	if len(segments) == 0 {
		t, _ := try(base, nil)
		return t
	}
	return lang.Target{}
}

// local resolves a module path of a package in the repository: a submodule to the
// file defining it, else the package's entry file src/Name.jl, else its directory.
func (r *resolver) local(p *project, segments []string, file string) lang.Target {
	name := p.name
	if name == "" && len(segments) > 0 {
		name = segments[0]
	}
	entry := path.Join(p.dir, "src", name+".jl")
	for n := len(segments); n >= 2; n-- {
		if f := r.moduleFile(strings.Join(segments[:n], "."), entry); f != "" && strings.HasPrefix(f, prefixOf(p.dir)) {
			return local(f, file)
		}
	}
	if r.files[entry] {
		return local(entry, file)
	}
	if f := r.moduleFile(name, entry); f != "" && strings.HasPrefix(f, prefixOf(p.dir)) {
		return local(f, file)
	}
	if p.dir != "." && r.dirs[p.dir] {
		return lang.Target{Local: p.dir}
	}
	return local(p.file, file)
}

func prefixOf(dir string) string {
	if dir == "." {
		return ""
	}
	return dir + "/"
}

// declared resolves a dependency a project declares: a [sources] path or URL, the
// manifest's entry, a package of the repository with that UUID, the standard
// library, else the registry package floating on its [compat] entry.
func (r *resolver) declared(p *project, name, uuid string, segments []string, file string) lang.Target {
	if s, ok := p.sources[name]; ok {
		if s.path != "" {
			d := path.Join(p.dir, s.path)
			if lp := r.byDir[d]; lp != nil {
				return r.local(lp, segments, file)
			}
			if r.dirs[d] {
				return lang.Target{Local: d}
			}
		}
		if s.url != "" {
			t := lang.Target{Ecosystem: ecoJulia, Package: name, Version: s.rev, Origin: s.url, Pinned: lang.Commit(s.rev)}
			t.Floating = s.rev == ""
			return t
		}
	}
	if e := p.manifest.find(name, uuid); e != nil {
		t := r.entry(p.manifest, e, segments, file)
		if c := p.compat[name]; c != "" && t.Pinned && c != t.Version && c != "="+t.Version {
			t.Requested = c
		}
		return t
	}
	if uuid != "" {
		if lps := r.byUUID[uuid]; len(lps) > 0 {
			return r.local(lps[0], segments, file)
		}
	}
	if juliapkg.Stdlib(name) {
		return std(name)
	}
	return compatTarget(name, p.compat[name])
}

// compatTarget is a registry package as a [compat] entry asks for it: "=1.2.3" pins,
// anything else ("1.2" is ^1.2) is kept as written and floats, none floats.
//
// Implements: REQ-JULIA-008
func compatTarget(name, compat string) lang.Target {
	t := lang.Target{Ecosystem: ecoJulia, Package: name}
	switch v, ok := juliapkg.ExactCompat(compat); {
	case ok:
		t.Version, t.Pinned = v, true
	case compat != "":
		t.Version = compat
	default:
		t.Floating = true
	}
	return t
}

// entry is what a manifest says a package is: a developed directory, a standard
// library (no git-tree-sha1: shipped with Julia, not installed from a registry), or
// a package pinned to its version.
func (r *resolver) entry(m *manifest, e *entry, segments []string, file string) lang.Target {
	if e.path != "" {
		d := path.Join(m.dir, e.path)
		if lp := r.byDir[d]; lp != nil {
			return r.local(lp, segments, file)
		}
		if r.dirs[d] {
			return lang.Target{Local: d}
		}
		return lang.Target{Ecosystem: ecoJulia, Package: e.name, Version: e.version, Origin: "path:" + e.path}
	}
	if juliapkg.Stdlib(e.name) && e.tree == "" {
		return std(e.name)
	}
	v := e.version
	if v == "" {
		v = e.tree // a package added by URL without a version: its tree hash
	}
	t := lang.Target{Ecosystem: ecoJulia, Package: e.name, Version: v, Pinned: v != "", Origin: e.repoURL}
	t.Floating = v == ""
	return t
}

// Dependencies answers --resolve-depth from the manifests: the deps of the entry
// that pinned the package.
//
// Implements: REQ-JULIA-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoJulia {
		return nil
	}
	for _, m := range r.all {
		for _, e := range m.byName[t.Package] {
			if t.Version != "" && e.version != t.Version && e.tree != t.Version {
				continue
			}
			var out []lang.Target
			for _, d := range e.deps {
				de := m.find(d, "")
				switch {
				case de == nil && juliapkg.Stdlib(d):
					out = append(out, std(d))
				case de == nil:
					out = append(out, lang.Target{Ecosystem: ecoJulia, Package: d, Floating: true})
				case de.path != "":
					// a developed package is the repository's own, not a dependency
				default:
					out = append(out, r.entry(m, de, nil, ""))
				}
			}
			return out
		}
	}
	return nil
}
