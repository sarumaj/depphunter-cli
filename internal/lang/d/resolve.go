package d

import (
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a dub package of the repository: a directory with dub.json or
// dub.sdl, or an inline sub-package of one. Its root is the package whose
// dub.selections.json governs it: itself, or the package listing it among its
// subPackages.
type project struct {
	dir      string
	recipe   *recipe
	root     *project
	inline   bool
	subs     map[string]*project // the root's sub-packages by name
	sel      map[string]*selection
	imports  []string // directories modules are found in
	strings  []string // string import directories
	manifest string   // the recipe file
	// reach is the package, its root and the repository's packages it depends
	// on (sub-packages, path dependencies), transitively, nearest first.
	reach []*project
}

type resolver struct {
	files    map[string]bool
	dirs     map[string]bool
	projects map[string]*project // by directory: the package whose recipe is there
	single   map[string]*project // by file: a single-file package
	inlines  []*project
	// installed maps a module to the dub package installed on this machine that
	// provides it, for the packages the repository declares or selects.
	installed map[string]string
	recipes   map[string]*recipe // installed package -> its recipe
}

// Implements: REQ-DLANG-004, REQ-DLANG-005, REQ-DLANG-006, REQ-DLANG-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, projects: map[string]*project{},
		single: map[string]*project{}, installed: map[string]string{}, recipes: map[string]*recipe{}}
	abs := map[string]string{}
	for _, f := range all {
		if dubDir(f.Path) {
			continue
		}
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
	r.dirs["."] = true
	read := func(rel string) ([]byte, bool) {
		if a, ok := abs[rel]; ok {
			data, err := os.ReadFile(a)
			return data, err == nil
		}
		if root == "" {
			return nil, false
		}
		// dub.selections.json is often ignored by git in libraries.
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return data, err == nil
	}
	var manifests []string
	for rel := range abs {
		if b := path.Base(rel); b == "dub.json" || b == "dub.sdl" {
			manifests = append(manifests, rel)
		}
	}
	sort.Strings(manifests) // dub.json before dub.sdl: dub reads dub.json first
	for _, rel := range manifests {
		dir := path.Dir(rel)
		if r.projects[dir] != nil {
			continue
		}
		data, ok := read(rel)
		if !ok {
			continue
		}
		rec := readJSONRecipe(data)
		if path.Base(rel) == "dub.sdl" {
			rec = readSDLRecipe(data)
		}
		p := &project{dir: dir, recipe: rec, manifest: rel, subs: map[string]*project{}}
		p.root = p
		r.projects[dir] = p
	}
	// Sub-packages: a directory the root lists, or an inline recipe.
	for _, dir := range sortedKeys(r.projects) {
		p := r.projects[dir]
		for _, s := range p.recipe.subs {
			var sp *project
			if s.inline != nil {
				sp = &project{dir: p.dir, recipe: s.inline, inline: true, manifest: p.manifest, subs: p.subs}
				r.inlines = append(r.inlines, sp)
			} else if sp = r.projects[path.Join(p.dir, s.path)]; sp == nil || sp == p {
				continue
			}
			sp.root = p
			sp.subs = p.subs
			if sp.recipe.name != "" {
				p.subs[sp.recipe.name] = sp
			}
		}
	}
	for _, rel := range sortedKeys(abs) {
		if ext := path.Ext(rel); ext == ".d" || ext == ".di" {
			if rec := readSingle(abs[rel]); rec != nil {
				p := &project{dir: path.Dir(rel), recipe: rec, manifest: rel, subs: map[string]*project{}}
				p.root = p
				r.single[rel] = p
			}
		}
	}
	all2 := append(sortedProjects(r.projects), r.inlines...)
	for _, rel := range sortedKeys(r.single) {
		all2 = append(all2, r.single[rel])
	}
	for _, p := range all2 {
		p.imports, p.strings = r.paths(p)
	}
	for _, p := range all2 {
		if p.root == p {
			if data, ok := read(path.Join(p.dir, "dub.selections.json")); ok {
				p.sel = readSelections(data)
			}
		}
	}
	for _, p := range all2 {
		if p.sel == nil {
			p.sel = p.root.sel
		}
	}
	for _, p := range all2 {
		p.reach = r.reachOf(p)
	}
	r.readInstalled(root, all2)
	return r
}

func sortedProjects(m map[string]*project) []*project {
	var out []*project
	for _, k := range sortedKeys(m) {
		out = append(out, m[k])
	}
	return out
}

// readSingle reads the recipe a module embeds as a single-file package, from
// the head of the file only.
func readSingle(abs string) *recipe {
	f, err := os.Open(abs)
	if err != nil {
		return nil
	}
	defer f.Close()
	head := make([]byte, 16<<10)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if !bytes.Contains(head[:min(n, 512)], []byte("dub.")) {
		return nil
	}
	rec, _ := singleFile(head)
	return rec
}

// dubDir reports whether a path lies in dub's .dub/ directory (build output
// and packages fetched with --cache=local).
func dubDir(p string) bool {
	return strings.HasPrefix(p, ".dub/") || strings.Contains(p, "/.dub/")
}

// paths are a package's import directories - its importPaths and sourcePaths,
// by default source/ or src/ where they exist - and its string import
// directories, by default views/.
func (r *resolver) paths(p *project) (imports, strs []string) {
	add := func(list []string, p string) []string {
		p = path.Clean(p)
		if strings.HasPrefix(p, "../") || p == ".." || strings.HasPrefix(p, "/") {
			return list
		}
		for _, q := range list {
			if q == p {
				return list
			}
		}
		return append(list, p)
	}
	rec := p.recipe
	for _, set := range []struct {
		list []string
		ok   bool
	}{{rec.importPaths, rec.importsSet}, {rec.sourcePaths, rec.sourcesSet}} {
		if set.ok {
			for _, q := range set.list {
				imports = add(imports, path.Join(p.dir, q))
			}
			continue
		}
		for _, def := range []string{"source", "src"} {
			if d := path.Join(p.dir, def); r.dirs[d] {
				imports = add(imports, d)
			}
		}
	}
	if rec.stringSet {
		for _, q := range rec.stringPaths {
			strs = add(strs, path.Join(p.dir, q))
		}
	} else if d := path.Join(p.dir, "views"); r.dirs[d] {
		strs = add(strs, d)
	}
	return imports, strs
}

// projectOf is the package governing a file: the nearest recipe at or above
// its directory, or an inline sub-package of it whose directories hold the file.
func (r *resolver) projectOf(file string) *project {
	if p := r.single[file]; p != nil {
		return p
	}
	dir := path.Dir(file)
	for {
		if p := r.projects[dir]; p != nil {
			for _, in := range r.inlines {
				if in.root == p && under(file, in.imports) {
					return in
				}
			}
			return p
		}
		if dir == "." || dir == "/" || dir == "" {
			return nil
		}
		dir = path.Dir(dir)
	}
}

func under(file string, dirs []string) bool {
	for _, d := range dirs {
		if d == "." || strings.HasPrefix(file, d+"/") {
			return true
		}
	}
	return false
}

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindImport:
		return r.module(file, imp.Module)
	case kindString:
		return r.stringImport(file, imp.Module)
	case kindDep:
		if p := r.manifestProject(file, imp.Module); p != nil {
			return r.dubTarget(p, imp.Module)
		}
		return lang.Target{Ecosystem: ecoDub, Package: base(imp.Module)}
	case kindSelected:
		if p := r.projects[path.Dir(file)]; p != nil {
			return r.dubTarget(p, imp.Module)
		}
		return lang.Target{Ecosystem: ecoDub, Package: base(imp.Module)}
	case kindSubPath:
		if d := path.Join(path.Dir(file), imp.Module); r.dirs[d] && !strings.HasPrefix(d, "../") {
			return lang.Target{Local: d}
		}
	}
	return lang.Target{}
}

// manifestProject is the package that declares a dependency in a recipe file:
// the recipe's own package or the inline sub-package declaring it.
func (r *resolver) manifestProject(file, name string) *project {
	if p := r.single[file]; p != nil {
		return p
	}
	p := r.projects[path.Dir(file)]
	if p == nil || p.manifest != file {
		return p
	}
	if p.recipe.dependencies[name] != nil {
		return p
	}
	for _, in := range r.inlines {
		if in.root == p && in.recipe.dependencies[name] != nil {
			return in
		}
	}
	return p
}

// base is a package name without its sub-package: vibe-d for vibe-d:http.
func base(name string) string {
	b, _, _ := strings.Cut(name, ":")
	return b
}

// probe finds the file of a module under dir: a/b.d, a/b.di, a/b/package.d or
// a/b/package.di.
func (r *resolver) probe(dir, module string) string {
	rel := strings.ReplaceAll(module, ".", "/")
	p := path.Join(dir, rel)
	if strings.HasPrefix(p, "../") {
		return ""
	}
	for _, f := range []string{p + ".d", p + ".di", p + "/package.d", p + "/package.di"} {
		if r.files[f] {
			return f
		}
	}
	return ""
}

// reachOf lists p, its root and the repository's packages p depends on,
// transitively and bounded, nearest first.
func (r *resolver) reachOf(p *project) []*project {
	var out []*project
	visited := map[*project]bool{}
	queue := []*project{p, p.root}
	for len(queue) > 0 && len(out) < 64 {
		q := queue[0]
		queue = queue[1:]
		if visited[q] {
			continue
		}
		visited[q] = true
		out = append(out, q)
		for _, dep := range q.recipe.deps {
			if lp := r.localProject(q, dep); lp != nil {
				queue = append(queue, lp)
			}
		}
	}
	return out
}

// roots are the directories a file's imports are looked up in: the import
// directories of its package and of the repository's packages it depends on,
// then the file's own directory and its ancestors, which is where a module's
// name starts when no recipe says (std/algorithm.d imports std.range from the
// parent of std/).
func (r *resolver) roots(file string, p *project) []string {
	var out []string
	seen := map[string]bool{}
	add := func(d string) {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	if p != nil {
		for _, q := range p.reach {
			for _, d := range q.imports {
				add(d)
			}
		}
	}
	for d := path.Dir(file); ; d = path.Dir(d) {
		add(d)
		if d == "." || d == "/" {
			break
		}
	}
	if p == nil {
		for _, d := range []string{"source", "src", "import"} {
			if r.dirs[d] {
				add(d)
			}
		}
	}
	return out
}

// localProject is the repository's package a dependency names: a sub-package of
// the same root, or a path dependency whose directory has a recipe.
func (r *resolver) localProject(p *project, dep *dependency) *project {
	name := dep.name
	b, sub, isSub := strings.Cut(name, ":")
	if isSub && (b == "" || b == p.root.recipe.name) {
		return p.subs[sub]
	}
	if dep.path != "" {
		if lp := r.projects[path.Join(p.dir, dep.path)]; lp != nil {
			if isSub {
				return lp.subs[sub]
			}
			return lp
		}
	}
	if s := p.sel[b]; s != nil && s.path != "" {
		if lp := r.projects[path.Join(p.root.dir, s.path)]; lp != nil {
			if isSub {
				return lp.subs[sub]
			}
			return lp
		}
	}
	return nil
}

// module resolves `import a.b.c` from file.
//
// Implements: REQ-DLANG-004, REQ-DLANG-007
func (r *resolver) module(file, module string) lang.Target {
	p := r.projectOf(file)
	for _, d := range r.roots(file, p) {
		if f := r.probe(d, module); f == file {
			return lang.Target{} // a module importing itself (in a unittest)
		} else if f != "" {
			return lang.Target{Local: f}
		}
	}
	if std := stdPackage(module); std != "" {
		return lang.Target{Ecosystem: ecoStd, Package: std}
	}
	if name, ok := r.installed[module]; ok && p != nil {
		return r.dubTarget(p, name)
	}
	if name, ok := r.installed[module]; ok {
		return lang.Target{Ecosystem: ecoDub, Package: name}
	}
	segs := strings.Split(module, ".")
	var declared []string
	if p != nil {
		declared = r.declared(p)
	}
	// A declared package spelled by the module's leading segments (mir.random is
	// mir-random), or the one a curated table names.
	spelled, k := "", 0
	for n := len(segs); n >= 1 && spelled == ""; n-- {
		want := fold(strings.Join(segs[:n], "_"))
		for _, name := range declared {
			for _, s := range spellings(base(name)) {
				if s == want {
					spelled, k = base(name), n
					break
				}
			}
			if spelled != "" {
				break
			}
		}
	}
	cands, tk := table(module)
	if cands != nil && tk >= k {
		for _, c := range cands {
			for _, name := range declared {
				if base(name) == c {
					return r.dubTarget(p, c)
				}
			}
		}
		if spelled == "" {
			if p != nil && r.own(p, cands[0]) {
				// The package's own prefix, but not its file: mir-algorithm's
				// mir.exception is mir-core's.
				if name := startsLike(declared, segs[0]); name != "" {
					return r.dubTarget(p, name)
				}
				return lang.Target{}
			}
			return lang.Target{Ecosystem: ecoDub, Package: cands[0], Unresolved: true}
		}
	}
	if spelled != "" {
		return r.dubTarget(p, spelled)
	}
	if name := startsLike(declared, segs[0]); name != "" {
		return r.dubTarget(p, name)
	}
	if p != nil && r.own(p, segs[0]) {
		return lang.Target{} // the package's own module, missing
	}
	return lang.Target{Ecosystem: ecoDub, Package: segs[0], Unresolved: true}
}

// startsLike is the one declared package whose name starts with a module's
// first segment and a dash (mir.exception in a package depending on mir-core
// only), "" when none or several do.
func startsLike(declared []string, first string) string {
	found := ""
	for _, name := range declared {
		if strings.HasPrefix(fold(base(name)), fold(first)+"_") {
			if found != "" && found != base(name) {
				return ""
			}
			found = base(name)
		}
	}
	return found
}

// own reports whether name spells the package itself or its root.
func (r *resolver) own(p *project, name string) bool {
	for _, q := range []*project{p, p.root} {
		if q.recipe.name == "" {
			continue
		}
		for _, s := range spellings(q.recipe.name) {
			if s == fold(name) {
				return true
			}
		}
	}
	return false
}

// declared lists the packages a file's package can import from: what it, its
// root and the repository's packages it reaches depend on, and what
// dub.selections.json selected, sorted.
func (r *resolver) declared(p *project) []string {
	set := map[string]bool{}
	for _, q := range p.reach {
		for _, d := range q.recipe.deps {
			if b := base(d.name); b != "" {
				set[b] = true
			}
		}
	}
	for name := range p.sel {
		set[name] = true
	}
	for _, q := range p.reach {
		delete(set, q.recipe.name) // a sub-package depending on its package
	}
	return sortedKeys(set)
}

// dependency is how package p, or a package it reaches, declares name (and
// which package does), nil when none does.
func (r *resolver) dependency(p *project, name string) (*dependency, *project) {
	b := base(name)
	for _, q := range p.reach {
		if d := q.recipe.dependencies[name]; d != nil {
			return d, q
		}
		for _, d := range q.recipe.deps {
			if base(d.name) == b {
				return d, q
			}
		}
	}
	return nil, nil
}

// dubTarget is the dub package name as package p depends on it. A sub-package
// is its base package's node (vibe-d:http is vibe-d, as the registry publishes
// it); a sub-package of p's own root is its directory, or dropped when it is
// written inline.
//
// Implements: REQ-DLANG-006
func (r *resolver) dubTarget(p *project, name string) lang.Target {
	b, sub, isSub := strings.Cut(name, ":")
	if isSub && (b == "" || b == p.root.recipe.name) {
		if sp := p.subs[sub]; sp != nil && !sp.inline {
			return lang.Target{Local: sp.dir}
		}
		return lang.Target{}
	}
	if !isSub && p != p.root && b == p.root.recipe.name {
		return lang.Target{Local: p.root.dir} // a sub-package depending on its package
	}
	d, dp := r.dependency(p, name)
	if d != nil && d.path != "" {
		if dir := path.Join(dp.dir, d.path); r.dirs[dir] && !strings.HasPrefix(dir, "../") {
			return lang.Target{Local: dir}
		}
		return lang.Target{}
	}
	t := lang.Target{Ecosystem: ecoDub, Package: b}
	if s := p.sel[b]; s != nil {
		switch {
		case s.path != "":
			if dir := path.Join(p.root.dir, s.path); r.dirs[dir] && !strings.HasPrefix(dir, "../") {
				return lang.Target{Local: dir}
			}
			return lang.Target{}
		case s.repo != "":
			t.Origin = repoURL(s.repo)
			pinRule(&t, s.version, true)
			t.Floating = false
		case strings.HasPrefix(s.version, "~"):
			t.Version, t.Floating = s.version, true // a branch: ~master
		default:
			t.Version, t.Pinned = s.version, s.version != ""
		}
		if d != nil && d.version != "" && d.version != t.Version {
			t.Requested = d.version
		}
		return t
	}
	if d != nil {
		if d.repo != "" {
			t.Origin = repoURL(d.repo)
		}
		pinRule(&t, d.version, d.repo != "")
		return t
	}
	if _, ok := r.recipes[b]; !ok {
		t.Unresolved = true
	}
	return t
}

// repoURL is a dub repository reference as a URL: git+https://x -> https://x.
func repoURL(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "git+") }

// stringImport resolves import("file") under the package's string import
// directories (views/ by default), else beside the file.
//
// Implements: REQ-DLANG-004
func (r *resolver) stringImport(file, name string) lang.Target {
	if strings.HasPrefix(name, "/") {
		return lang.Target{}
	}
	var dirs []string
	if p := r.projectOf(file); p != nil {
		dirs = append(dirs, p.strings...)
	} else {
		for d := path.Dir(file); ; d = path.Dir(d) {
			if v := path.Join(d, "views"); r.dirs[v] {
				dirs = append(dirs, v)
			}
			if d == "." || d == "/" {
				break
			}
		}
	}
	dirs = append(dirs, path.Dir(file))
	for _, d := range dirs {
		if f := path.Join(d, name); r.files[f] && !strings.HasPrefix(f, "../") {
			return lang.Target{Local: f}
		}
	}
	return lang.Target{}
}

// Dependencies lists what an installed dub package depends on, from the recipe
// dub fetched with it: dub.selections.json is flat, so without the package on
// disk nothing is known offline. Each is pinned as the repository's selections
// pin it.
//
// Implements: REQ-DLANG-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	rec := r.recipes[t.Package]
	if t.Ecosystem != ecoDub || rec == nil {
		return nil
	}
	var out []lang.Target
	seen := map[string]bool{t.Package: true}
	for _, d := range allDeps(rec) {
		b := base(d.name)
		if b == "" || seen[b] || d.optional || d.path != "" {
			continue
		}
		seen[b] = true
		dt := lang.Target{Ecosystem: ecoDub, Package: b}
		if s := r.selected(b); s != nil && s.version != "" && s.path == "" {
			dt.Version, dt.Pinned = s.version, s.repo == "" && !strings.HasPrefix(s.version, "~") || lang.Commit(s.version)
			dt.Floating = !dt.Pinned
			if s.repo != "" {
				dt.Origin = repoURL(s.repo)
			}
		} else {
			if d.repo != "" {
				dt.Origin = repoURL(d.repo)
			}
			pinRule(&dt, d.version, d.repo != "")
		}
		out = append(out, dt)
	}
	return out
}

// Installed reports whether a package's dependencies come from what dub
// fetched onto this machine.
func (r *resolver) Installed(t lang.Target) bool {
	return t.Ecosystem == ecoDub && r.recipes[t.Package] != nil
}

// selected is the first selection of name among the repository's packages.
func (r *resolver) selected(name string) *selection {
	for _, p := range sortedProjects(r.projects) {
		if s := p.sel[name]; s != nil {
			return s
		}
	}
	return nil
}

// allDeps are a recipe's dependencies and its inline sub-packages'.
func allDeps(rec *recipe) []*dependency {
	out := append([]*dependency(nil), rec.deps...)
	for _, s := range rec.subs {
		if s.inline != nil {
			out = append(out, allDeps(s.inline)...)
		}
	}
	return out
}
