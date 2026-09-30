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
	files       map[string]bool
	directories map[string]bool // directories holding files
	projects    []*project      // shallowest first
	byDirectory map[string]*project
	byName      map[string][]*project
	byUUID      map[string][]*project
	byFile      map[string]*project
	manifests   map[string]*manifest // by file, those claimed
	all         []*manifest          // every manifest read, by path
	// modules maps a module's absolute path (Shop.Internal) to the files defining
	// it; ctx is the module path a file's top level is in, from the include graph.
	modules map[string][]string
	ctx     map[string]string
}

// newResolver reads every project, manifest and source: sources for the modules
// they define and the files they include, which place each file in a module.
//
// Implements: REQ-JULIA-004, REQ-JULIA-005, REQ-JULIA-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, directories: map[string]bool{}, byDirectory: map[string]*project{},
		byName: map[string][]*project{}, byUUID: map[string][]*project{}, byFile: map[string]*project{},
		manifests: map[string]*manifest{}, modules: map[string][]string{}, ctx: map[string]string{}}
	var sources []*scan.File
	directoryManifest := map[string]*manifest{}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); ; d = path.Dir(d) {
			if r.directories[d] {
				break
			}
			r.directories[d] = true
			if d == "." {
				break
			}
		}
		if !lang.Readable(f) {
			continue
		}
		switch fileClass(f.Path) {
		case classSource:
			sources = append(sources, f)
		case classProject:
			source, err := os.ReadFile(f.AbsolutePath)
			if err != nil {
				continue
			}
			p := readProject(source)
			if p == nil {
				continue
			}
			p.file, p.directory = f.Path, path.Dir(f.Path)
			// JuliaProject.toml takes precedence over Project.toml, as in Pkg.
			if have := r.byDirectory[p.directory]; have != nil && path.Base(have.file) == "JuliaProject.toml" {
				continue
			}
			r.byDirectory[p.directory] = p
		}
	}
	// Manifests are often not committed; they are looked for on disk beside every
	// project too.
	for directory := range r.byDirectory {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
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
		file := path.Join(directory, best)
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil || len(source) > 8*lang.MaxParseSize {
			continue
		}
		m := readManifest(source)
		m.file, m.directory = file, directory
		directoryManifest[directory] = m
	}
	for _, f := range all {
		if fileClass(f.Path) != classManifest || !lang.Readable(f) {
			continue
		}
		if m := directoryManifest[path.Dir(f.Path)]; m != nil && m.file == f.Path {
			r.manifests[f.Path] = m
			continue
		}
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		m := readManifest(source)
		m.file, m.directory = f.Path, path.Dir(f.Path)
		r.manifests[f.Path] = m
		if directoryManifest[m.directory] == nil {
			directoryManifest[m.directory] = m
		}
	}
	for _, m := range directoryManifest {
		r.all = append(r.all, m)
	}
	sort.Slice(r.all, func(i, j int) bool { return r.all[i].file < r.all[j].file })
	for _, p := range r.byDirectory {
		r.projects = append(r.projects, p)
		// The manifest resolving a project is its own, else the nearest above it (a
		// workspace's, or the repository's environment).
		if m, ok := lang.NearestAtOrAbove(directoryManifest, p.directory); ok {
			p.manifest = m
		}
	}
	sort.Slice(r.projects, func(i, j int) bool {
		depthI, depthJ := lang.Depth(r.projects[i].directory), lang.Depth(r.projects[j].directory)
		if depthI != depthJ {
			return depthI < depthJ
		}
		return r.projects[i].directory < r.projects[j].directory
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
		file   string
		source *source
	}
	var list []parsed
	for _, f := range files {
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil || !lang.Parseable(f, source) {
			continue
		}
		list = append(list, parsed{f.Path, readSource(source)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].file < list[j].file })
	type parent struct{ file, module string }
	includedBy := map[string]parent{}
	for _, p := range list {
		for _, include := range p.source.includes {
			child := path.Join(path.Dir(p.file), include.path)
			if _, ok := includedBy[child]; !ok && r.files[child] && child != p.file {
				includedBy[child] = parent{p.file, include.module}
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
		for _, m := range p.source.modules {
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
	best, bestLength := "", -1
	for _, f := range r.modules[key] {
		if n := lang.CommonDirectories(f, from); n > bestLength {
			best, bestLength = f, n
		}
	}
	return best
}

// local targets a file: none for the importer itself.
func local(p, file string) lang.Target {
	if p == "" || p == file {
		return lang.Target{}
	}
	return lang.Target{Local: p}
}

func std(name string) lang.Target { return lang.Target{Ecosystem: ecosystemStd, Package: name} }

// Resolve maps an import to its target.
//
// Implements: REQ-JULIA-004, REQ-JULIA-005, REQ-JULIA-006, REQ-JULIA-008, REQ-JULIA-010
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, argument, _ := strings.Cut(rawImport.Name, "\n")
	switch kind {
	case kindInclude:
		if rawImport.Module == "" {
			return lang.Target{}
		}
		p := path.Join(path.Dir(file), rawImport.Module)
		if !r.files[p] {
			return lang.Target{}
		}
		return local(p, file)
	case kindUsing:
		return r.module(file, argument, rawImport.Module)
	case kindDependency:
		p := r.byFile[file]
		if p == nil {
			return lang.Target{}
		}
		return r.declared(p, rawImport.Module, p.dependencies[rawImport.Module], []string{rawImport.Module}, file)
	case kindMember:
		d := path.Join(path.Dir(file), rawImport.Module)
		if p := r.byDirectory[d]; p != nil {
			return local(p.file, file)
		}
		if r.directories[d] {
			return lang.Target{Local: d}
		}
		return lang.Target{}
	case kindExtension:
		directory := path.Dir(file)
		for _, c := range []string{"ext/" + rawImport.Module + ".jl", "ext/" + rawImport.Module + "/" + rawImport.Module + ".jl"} {
			if p := path.Join(directory, c); r.files[p] {
				return lang.Target{Local: p}
			}
		}
		return lang.Target{}
	case kindManifest:
		m := r.manifests[file]
		if e := m.find(rawImport.Module, argument); e != nil {
			return r.entry(m, e, []string{e.name}, file)
		}
	}
	return lang.Target{}
}

// ancestors are the projects of the file's directory and those above it, nearest
// first.
func (r *resolver) ancestors(file string) []*project {
	return lang.Chain(r.byDirectory, file)
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
	governing := r.governing(file)
	for _, p := range governing {
		if uuid, ok := p.dependencies[first]; ok {
			return r.declared(p, first, uuid, segments, file)
		}
	}
	if juliapkg.Stdlib(first) {
		return std(first)
	}
	for _, p := range governing {
		if e := p.manifest.find(first, ""); e != nil {
			return r.entry(p.manifest, e, segments, file)
		}
	}
	if projects := r.byName[first]; len(projects) > 0 {
		best := projects[0]
		for _, p := range projects[1:] {
			if lang.CommonDirectories(p.file, file) > lang.CommonDirectories(best.file, file) {
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
		if uuid, ok := p.dependencies[first]; ok {
			return r.declared(p, first, uuid, segments, file)
		}
	}
	return lang.Target{Ecosystem: ecosystemJulia, Package: first, Unresolved: true, Floating: true}
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
	entry := path.Join(p.directory, "src", name+".jl")
	for n := len(segments); n >= 2; n-- {
		if f := r.moduleFile(strings.Join(segments[:n], "."), entry); f != "" && strings.HasPrefix(f, prefixOf(p.directory)) {
			return local(f, file)
		}
	}
	if r.files[entry] {
		return local(entry, file)
	}
	if f := r.moduleFile(name, entry); f != "" && strings.HasPrefix(f, prefixOf(p.directory)) {
		return local(f, file)
	}
	if p.directory != "." && r.directories[p.directory] {
		return lang.Target{Local: p.directory}
	}
	return local(p.file, file)
}

func prefixOf(directory string) string {
	if directory == "." {
		return ""
	}
	return directory + "/"
}

// declared resolves a dependency a project declares: a [sources] path or URL, the
// manifest's entry, a package of the repository with that UUID, the standard
// library, else the registry package floating on its [compat] entry.
func (r *resolver) declared(p *project, name, uuid string, segments []string, file string) lang.Target {
	if s, ok := p.sources[name]; ok {
		if s.path != "" {
			d := path.Join(p.directory, s.path)
			if localProject := r.byDirectory[d]; localProject != nil {
				return r.local(localProject, segments, file)
			}
			if r.directories[d] {
				return lang.Target{Local: d}
			}
		}
		if s.url != "" {
			t := lang.Target{Ecosystem: ecosystemJulia, Package: name, Version: s.rev, Origin: s.url, Pinned: lang.Commit(s.rev)}
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
	return compatTarget(name, uuid, p.compat[name])
}

// compatTarget is a registry package as a [compat] entry asks for it: "=1.2.3" pins,
// anything else ("1.2" is ^1.2) is kept as written and floats, none floats. Its
// UUID is what the registries know it by.
//
// Implements: REQ-JULIA-008
func compatTarget(name, uuid, compat string) lang.Target {
	t := lang.Target{Ecosystem: ecosystemJulia, Package: name, Registry: uuid}
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
		d := path.Join(m.directory, e.path)
		if localProject := r.byDirectory[d]; localProject != nil {
			return r.local(localProject, segments, file)
		}
		if r.directories[d] {
			return lang.Target{Local: d}
		}
		return lang.Target{Ecosystem: ecosystemJulia, Package: e.name, Version: e.version, Origin: "path:" + e.path}
	}
	if juliapkg.Stdlib(e.name) && e.tree == "" {
		return std(e.name)
	}
	v := e.version
	if v == "" {
		v = e.tree // a package added by URL without a version: its tree hash
	}
	t := lang.Target{Ecosystem: ecosystemJulia, Package: e.name, Version: v, Pinned: v != "", Origin: e.repositoryURL, Registry: e.uuid}
	t.Floating = v == ""
	return t
}

// Dependencies answers --resolve-depth from the manifests: the dependencies of the entry
// that pinned the package.
//
// Implements: REQ-JULIA-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemJulia {
		return nil
	}
	for _, m := range r.all {
		for _, e := range m.byName[t.Package] {
			if t.Version != "" && e.version != t.Version && e.tree != t.Version {
				continue
			}
			var out []lang.Target
			for _, d := range e.dependencies {
				de := m.find(d, "")
				switch {
				case de == nil && juliapkg.Stdlib(d):
					out = append(out, std(d))
				case de == nil:
					out = append(out, lang.Target{Ecosystem: ecosystemJulia, Package: d, Floating: true})
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
