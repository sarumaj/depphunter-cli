package d

import (
	"bytes"
	"io"
	"os"
	"path"
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
	directory  string
	recipe     *recipe
	root       *project
	inline     bool
	subs       map[string]*project // the root's sub-packages by name
	selections map[string]*selection
	imports    []string // directories modules are found in
	strings    []string // string import directories
	manifest   string   // the recipe file
	// reach is the package, its root and the repository's packages it depends
	// on (sub-packages, path dependencies), transitively, nearest first.
	reach []*project
}

type resolver struct {
	lang.Layout
	projects map[string]*project // by directory: the package whose recipe is there
	single   map[string]*project // by file: a single-file package
	inlines  []*project
	// installed maps a module to the dub package installed on this machine that
	// provides it, for the packages the repository declares or selects.
	installed map[string]string
	recipes   map[string]*recipe // installed package -> its recipe
}

// Implements: REQ-DLANG-004, REQ-DLANG-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{Layout: lang.NewLayout(), projects: map[string]*project{},
		single: map[string]*project{}, installed: map[string]string{}, recipes: map[string]*recipe{}}
	repository := lang.NewSource(root)
	r.indexFiles(repository, all)
	r.readManifests(repository)
	r.linkSubPackages()
	r.readSingleFiles(repository)
	projects := append(sortedProjects(r.projects), r.inlines...)
	for _, relative := range sortedKeys(r.single) {
		projects = append(projects, r.single[relative])
	}
	for _, p := range projects {
		p.imports, p.strings = r.paths(p)
	}
	readLocks(repository, projects)
	for _, p := range projects {
		p.reach = r.reachOf(p)
	}
	r.readInstalled(root, projects)
	return r
}

func (r *resolver) indexFiles(repository *lang.Source, all []*scan.File) {
	for _, f := range all {
		if dubDirectory(f.Path) {
			continue
		}
		r.Add(f.Path)
		repository.Add(f)
	}
	r.Directories["."] = true
}

var recipeReaders = map[string]func([]byte) *recipe{"dub.json": readJSONRecipe, "dub.sdl": readSDLRecipe}

// Implements: REQ-DLANG-005
func (r *resolver) readManifests(repository *lang.Source) {
	var manifests []string
	for relative := range r.Files {
		if recipeReaders[path.Base(relative)] != nil {
			manifests = append(manifests, relative)
		}
	}
	sort.Strings(manifests) // dub.json before dub.sdl: dub reads dub.json first
	for _, relative := range manifests {
		directory := path.Dir(relative)
		if r.projects[directory] != nil {
			continue
		}
		data, ok := repository.Read(relative)
		if !ok {
			continue
		}
		p := &project{directory: directory, recipe: recipeReaders[path.Base(relative)](data), manifest: relative, subs: map[string]*project{}}
		p.root = p
		r.projects[directory] = p
	}
}

// linkSubPackages links every package to the sub-packages its recipe lists: a
// directory, or an inline recipe.
//
// Implements: REQ-DLANG-005
func (r *resolver) linkSubPackages() {
	for _, directory := range sortedKeys(r.projects) {
		p := r.projects[directory]
		for _, s := range p.recipe.subs {
			var subproject *project
			if s.inline != nil {
				subproject = &project{directory: p.directory, recipe: s.inline, inline: true, manifest: p.manifest, subs: p.subs}
				r.inlines = append(r.inlines, subproject)
			} else if subproject = r.projects[path.Join(p.directory, s.path)]; subproject == nil || subproject == p {
				continue
			}
			subproject.root = p
			subproject.subs = p.subs
			if subproject.recipe.name != "" {
				p.subs[subproject.recipe.name] = subproject
			}
		}
	}
}

// readSingleFiles reads the recipes embedded in D sources, each a single-file package.
//
// Implements: REQ-DLANG-005
func (r *resolver) readSingleFiles(repository *lang.Source) {
	for _, relative := range sortedKeys(r.Files) {
		if extension := path.Ext(relative); extension != ".d" && extension != ".di" {
			continue
		}
		absolute, _ := repository.Absolute(relative)
		if recipe := readSingle(absolute); recipe != nil {
			p := &project{directory: path.Dir(relative), recipe: recipe, manifest: relative, subs: map[string]*project{}}
			p.root = p
			r.single[relative] = p
		}
	}
}

// readLocks reads each root's dub.selections.json, then gives the other packages
// their root's: every root is read before any package inherits.
//
// Implements: REQ-DLANG-006
func readLocks(repository *lang.Source, projects []*project) {
	for _, p := range projects {
		if p.root != p {
			continue
		}
		if data, ok := repository.Read(path.Join(p.directory, "dub.selections.json")); ok {
			p.selections = readSelections(data)
		}
	}
	for _, p := range projects {
		if p.selections == nil {
			p.selections = p.root.selections
		}
	}
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
func readSingle(absolute string) *recipe {
	f, err := os.Open(absolute)
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
	recipe, _ := singleFile(head)
	return recipe
}

// dubDirectory reports whether a path lies in dub's .dub/ directory (build output
// and packages fetched with --cache=local).
func dubDirectory(p string) bool {
	return strings.HasPrefix(p, ".dub/") || strings.Contains(p, "/.dub/")
}

// paths are a package's import directories - its importPaths and sourcePaths,
// by default source/ or src/ where they exist - and its string import
// directories, by default views/.
func (r *resolver) paths(p *project) (imports, stringImports []string) {
	add := func(list []string, p string) []string {
		p = path.Clean(p)
		if !lang.Inside(p) {
			return list
		}
		for _, q := range list {
			if q == p {
				return list
			}
		}
		return append(list, p)
	}
	recipe := p.recipe
	for _, set := range []struct {
		list []string
		ok   bool
	}{{recipe.importPaths, recipe.importsSet}, {recipe.sourcePaths, recipe.sourcesSet}} {
		if set.ok {
			for _, q := range set.list {
				imports = add(imports, path.Join(p.directory, q))
			}
			continue
		}
		for _, defaultDirectory := range []string{"source", "src"} {
			if d := path.Join(p.directory, defaultDirectory); r.Directories[d] {
				imports = add(imports, d)
			}
		}
	}
	if recipe.stringSet {
		for _, q := range recipe.stringPaths {
			stringImports = add(stringImports, path.Join(p.directory, q))
		}
	} else if d := path.Join(p.directory, "views"); r.Directories[d] {
		stringImports = add(stringImports, d)
	}
	return imports, stringImports
}

// projectOf is the package governing a file: the nearest recipe at or above
// its directory, or an inline sub-package of it whose directories hold the file.
func (r *resolver) projectOf(file string) *project {
	if p := r.single[file]; p != nil {
		return p
	}
	p, ok := lang.Nearest(r.projects, file)
	if !ok {
		return nil
	}
	for _, in := range r.inlines {
		if in.root == p && under(file, in.imports) {
			return in
		}
	}
	return p
}

func under(file string, directories []string) bool {
	for _, d := range directories {
		if lang.Within(file, d) {
			return true
		}
	}
	return false
}

func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindImport:
		return r.module(file, rawImport.Module)
	case kindString:
		return r.stringImport(file, rawImport.Module)
	case kindDependency:
		if p := r.manifestProject(file, rawImport.Module); p != nil {
			return r.dubTarget(p, rawImport.Module)
		}
		return lang.Target{Ecosystem: ecosystemDub, Package: base(rawImport.Module)}
	case kindSelected:
		if p := r.projects[path.Dir(file)]; p != nil {
			return r.dubTarget(p, rawImport.Module)
		}
		return lang.Target{Ecosystem: ecosystemDub, Package: base(rawImport.Module)}
	case kindSubPath:
		if d := path.Join(path.Dir(file), rawImport.Module); r.Directories[d] && !lang.ClimbsOut(d) {
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

// probe finds the file of a module under directory: a/b.d, a/b.di, a/b/package.d or
// a/b/package.di.
func (r *resolver) probe(directory, module string) string {
	relative := strings.ReplaceAll(module, ".", "/")
	p := path.Join(directory, relative)
	if lang.ClimbsOut(p) {
		return ""
	}
	for _, f := range []string{p + ".d", p + ".di", p + "/package.d", p + "/package.di"} {
		if r.Files[f] {
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
		for _, dependency := range q.recipe.dependencyList {
			if localProject := r.localProject(q, dependency); localProject != nil {
				queue = append(queue, localProject)
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
	for d := range lang.Ancestors(file) {
		add(d)
	}
	if p == nil {
		for _, d := range []string{"source", "src", "import"} {
			if r.Directories[d] {
				add(d)
			}
		}
	}
	return out
}

// localProject is the repository's package a dependency names: a sub-package of
// the same root, or a path dependency whose directory has a recipe.
func (r *resolver) localProject(p *project, declared *dependency) *project {
	name := declared.name
	b, subpackage, isSub := strings.Cut(name, ":")
	if isSub && (b == "" || b == p.root.recipe.name) {
		return p.subs[subpackage]
	}
	if declared.path != "" {
		if localProject := r.projects[path.Join(p.directory, declared.path)]; localProject != nil {
			if isSub {
				return localProject.subs[subpackage]
			}
			return localProject
		}
	}
	if s := p.selections[b]; s != nil && s.path != "" {
		if localProject := r.projects[path.Join(p.root.directory, s.path)]; localProject != nil {
			if isSub {
				return localProject.subs[subpackage]
			}
			return localProject
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
		return lang.Target{Ecosystem: ecosystemStd, Package: std}
	}
	if name, ok := r.installed[module]; ok && p != nil {
		return r.dubTarget(p, name)
	}
	if name, ok := r.installed[module]; ok {
		return lang.Target{Ecosystem: ecosystemDub, Package: name}
	}
	segments := strings.Split(module, ".")
	var declared []string
	if p != nil {
		declared = r.declared(p)
	}
	// A declared package spelled by the module's leading segments (mir.random is
	// mir-random), or the one a curated table names.
	spelled, k := "", 0
	for n := len(segments); n >= 1 && spelled == ""; n-- {
		want := fold(strings.Join(segments[:n], "_"))
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
	candidates, token := table(module)
	if candidates != nil && token >= k {
		for _, c := range candidates {
			for _, name := range declared {
				if base(name) == c {
					return r.dubTarget(p, c)
				}
			}
		}
		if spelled == "" {
			if p != nil && r.own(p, candidates[0]) {
				// The package's own prefix, but not its file: mir-algorithm's
				// mir.exception is mir-core's.
				if name := startsLike(declared, segments[0]); name != "" {
					return r.dubTarget(p, name)
				}
				return lang.Target{}
			}
			return lang.Target{Ecosystem: ecosystemDub, Package: candidates[0], Unresolved: true}
		}
	}
	if spelled != "" {
		return r.dubTarget(p, spelled)
	}
	if name := startsLike(declared, segments[0]); name != "" {
		return r.dubTarget(p, name)
	}
	if p != nil && r.own(p, segments[0]) {
		return lang.Target{} // the package's own module, missing
	}
	return lang.Target{Ecosystem: ecosystemDub, Package: segments[0], Unresolved: true}
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
		for _, d := range q.recipe.dependencyList {
			if b := base(d.name); b != "" {
				set[b] = true
			}
		}
	}
	for name := range p.selections {
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
		for _, d := range q.recipe.dependencyList {
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
	b, subpackage, isSub := strings.Cut(name, ":")
	if isSub && (b == "" || b == p.root.recipe.name) {
		if subproject := p.subs[subpackage]; subproject != nil && !subproject.inline {
			return lang.Target{Local: subproject.directory}
		}
		return lang.Target{}
	}
	if !isSub && p != p.root && b == p.root.recipe.name {
		return lang.Target{Local: p.root.directory} // a sub-package depending on its package
	}
	d, dp := r.dependency(p, name)
	if d != nil && d.path != "" {
		if directory := path.Join(dp.directory, d.path); r.Directories[directory] && !lang.ClimbsOut(directory) {
			return lang.Target{Local: directory}
		}
		return lang.Target{}
	}
	t := lang.Target{Ecosystem: ecosystemDub, Package: b}
	if s := p.selections[b]; s != nil {
		switch {
		case s.path != "":
			if directory := path.Join(p.root.directory, s.path); r.Directories[directory] && !lang.ClimbsOut(directory) {
				return lang.Target{Local: directory}
			}
			return lang.Target{}
		case s.repository != "":
			t.Origin = repositoryURL(s.repository)
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
		if d.repository != "" {
			t.Origin = repositoryURL(d.repository)
		}
		pinRule(&t, d.version, d.repository != "")
		return t
	}
	if _, ok := r.recipes[b]; !ok {
		t.Unresolved = true
	}
	return t
}

// repositoryURL is a dub repository reference as a URL: git+https://x -> https://x.
func repositoryURL(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "git+") }

// stringImport resolves import("file") under the package's string import
// directories (views/ by default), else beside the file.
//
// Implements: REQ-DLANG-004
func (r *resolver) stringImport(file, name string) lang.Target {
	if strings.HasPrefix(name, "/") {
		return lang.Target{}
	}
	var directories []string
	if p := r.projectOf(file); p != nil {
		directories = append(directories, p.strings...)
	} else {
		for d := range lang.Ancestors(file) {
			if v := path.Join(d, "views"); r.Directories[v] {
				directories = append(directories, v)
			}
		}
	}
	directories = append(directories, path.Dir(file))
	for _, d := range directories {
		if f := path.Join(d, name); r.Files[f] && !lang.ClimbsOut(f) {
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
	recipe := r.recipes[t.Package]
	if t.Ecosystem != ecosystemDub || recipe == nil {
		return nil
	}
	var out []lang.Target
	seen := map[string]bool{t.Package: true}
	for _, d := range allDependencies(recipe) {
		b := base(d.name)
		if b == "" || seen[b] || d.optional || d.path != "" {
			continue
		}
		seen[b] = true
		dependencyTarget := lang.Target{Ecosystem: ecosystemDub, Package: b}
		if s := r.selected(b); s != nil && s.version != "" && s.path == "" {
			dependencyTarget.Version, dependencyTarget.Pinned = s.version, s.repository == "" && !strings.HasPrefix(s.version, "~") || lang.Commit(s.version)
			dependencyTarget.Floating = !dependencyTarget.Pinned
			if s.repository != "" {
				dependencyTarget.Origin = repositoryURL(s.repository)
			}
		} else {
			if d.repository != "" {
				dependencyTarget.Origin = repositoryURL(d.repository)
			}
			pinRule(&dependencyTarget, d.version, d.repository != "")
		}
		out = append(out, dependencyTarget)
	}
	return out
}

// Installed reports whether a package's dependencies come from what dub
// fetched onto this machine.
func (r *resolver) Installed(t lang.Target) bool {
	return t.Ecosystem == ecosystemDub && r.recipes[t.Package] != nil
}

// selected is the first selection of name among the repository's packages.
func (r *resolver) selected(name string) *selection {
	for _, p := range sortedProjects(r.projects) {
		if s := p.selections[name]; s != nil {
			return s
		}
	}
	return nil
}

// allDependencies are a recipe's dependencies and its inline sub-packages'.
func allDependencies(from *recipe) []*dependency {
	out := append([]*dependency(nil), from.dependencyList...)
	for _, s := range from.subs {
		if s.inline != nil {
			out = append(out, allDependencies(s.inline)...)
		}
	}
	return out
}
