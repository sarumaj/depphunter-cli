package haskell

import (
	"cmp"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// packageInfo is a package of the repository: a .cabal file, or a package.yaml without one.
type packageInfo struct {
	*cabalPackage
	project *projectInfo
}

// projectInfo is a directory building packages together - one with cabal.project,
// stack.yaml or cabal.project.freeze, or else a package's own directory - and what
// pins its dependencies there.
type projectInfo struct {
	directory    string
	pins         map[string]string
	plan         *buildPlan
	stack        *stackProject
	lock         *stackLock
	repositories map[string]srp
	extras       map[string]extraDependency
	members      []string // package directories cabal.project or stack.yaml name
}

type resolver struct {
	lang.Layout
	packages    []*packageInfo // deepest first
	byName      map[string]*packageInfo
	byDirectory map[string]*packageInfo
	projects    []*projectInfo      // deepest first
	modules     map[string][]string // module name -> files declaring it
	boots       map[string][]string // module name -> .hs-boot files
}

// ignored are the directories cabal and stack build into.
func ignored(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if segment == "dist-newstyle" || segment == ".stack-work" {
			return true
		}
	}
	return false
}

// Implements: REQ-HASKELL-004, REQ-HASKELL-006, REQ-HASKELL-007, REQ-HASKELL-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{Layout: lang.NewLayout(), byName: map[string]*packageInfo{},
		byDirectory: map[string]*packageInfo{}, modules: map[string][]string{}, boots: map[string][]string{}}
	repository := lang.NewSource(root)
	projectDirectories := map[string]bool{}
	var sources []*scan.File
	hpack := map[string]*scan.File{}
	for _, f := range all {
		if ignored(f.Path) {
			continue
		}
		r.Add(f.Path)
		repository.Add(f)
		base := path.Base(f.Path)
		switch {
		case base == "cabal.project" || base == "stack.yaml" || base == "cabal.project.freeze":
			projectDirectories[path.Dir(f.Path)] = true
		case strings.HasSuffix(base, ".cabal") && !f.Binary && !f.TooLarge:
			source, err := os.ReadFile(f.AbsolutePath)
			if err != nil {
				continue
			}
			p := readCabal(source, f.Path)
			p.name = cmp.Or(p.name, strings.TrimSuffix(base, ".cabal"))
			r.addPackage(p)
		case base == "package.yaml":
			hpack[path.Dir(f.Path)] = f
		case sourceExtension(f.Path) != "" && !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize:
			sources = append(sources, f)
		}
	}
	for directory, f := range hpack {
		if r.byDirectory[directory] != nil {
			continue // stack and hpack commit the .cabal they generate: it is the same package
		}
		if source, err := os.ReadFile(f.AbsolutePath); err == nil {
			if p := readHpack(source, f.Path); p != nil && p.name != "" {
				r.addPackage(p)
			}
		}
	}
	sort.Slice(r.packages, func(i, j int) bool { return lang.DeepestFirst(r.packages[i].directory, r.packages[j].directory) })
	for _, p := range r.packages {
		covered := false
		for d := range projectDirectories {
			if lang.Within(p.directory, d) || p.directory == d {
				covered = true
				break
			}
		}
		if !covered {
			projectDirectories[p.directory] = true
		}
	}
	for d := range projectDirectories {
		r.projects = append(r.projects, r.readProject(d, repository.Read))
	}
	sort.Slice(r.projects, func(i, j int) bool { return lang.DeepestFirst(r.projects[i].directory, r.projects[j].directory) })
	for _, p := range r.packages {
		p.project = r.projectOf(p.file)
	}
	r.index(sources)
	return r
}

func (r *resolver) addPackage(p *cabalPackage) {
	info := &packageInfo{cabalPackage: p}
	r.packages = append(r.packages, info)
	r.byDirectory[p.directory] = info
	if q := r.byName[p.name]; q == nil || lang.Depth(p.directory) < lang.Depth(q.directory) {
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
func (r *resolver) readProject(directory string, read func(string) ([]byte, bool)) *projectInfo {
	p := &projectInfo{directory: directory, pins: map[string]string{}, repositories: map[string]srp{}, extras: map[string]extraDependency{}}
	pin := func(name, c string) {
		if _, ok := p.pins[name]; !ok && lang.Pinned(c) {
			p.pins[name] = c
		}
	}
	if source, ok := read(path.Join(directory, "cabal.project.freeze")); ok {
		for name, c := range readCabalProject(source).constraints {
			pin(name, c)
		}
	}
	for i, project := range projectFiles(path.Join(directory, "cabal.project"), read) {
		for name, c := range project.constraints {
			pin(name, c)
		}
		for _, s := range project.repositories {
			if _, duplicate := p.repositories[s.name]; !duplicate || i == 0 {
				p.repositories[s.name] = s
			}
		}
		for _, m := range project.members {
			p.members = append(p.members, r.memberDirectories(directory, m.text)...)
		}
	}
	if source, ok := read(path.Join(directory, "dist-newstyle", "cache", "plan.json")); ok {
		p.plan = readPlan(source)
	}
	if source, ok := read(path.Join(directory, "stack.yaml")); ok {
		p.stack = readStack(source)
		if p.stack != nil {
			for _, e := range p.stack.extras {
				if _, duplicate := p.extras[e.name]; !duplicate {
					p.extras[e.name] = e
				}
			}
			for _, m := range p.stack.members {
				p.members = append(p.members, path.Join(directory, m.text))
			}
		}
	}
	if source, ok := read(path.Join(directory, "stack.yaml.lock")); ok {
		p.lock = readStackLock(source)
	}
	if p.lock != nil {
		// A repository extra-dep is named after its repository until the lock
		// says which package it holds.
		for _, l := range p.lock.packages {
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

// sourceExtension is the extension of a Haskell source file the plugin reads, "" for any
// other file.
func sourceExtension(p string) string {
	base := path.Base(p)
	if strings.HasSuffix(base, ".hs-boot") {
		return ".hs-boot"
	}
	switch extension := path.Ext(base); extension {
	case ".hs", ".lhs", ".hsc":
		return extension
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
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil || !lang.Parseable(f, source) {
			continue
		}
		module := moduleName(source, sourceExtension(f.Path) == ".lhs")
		if module == "" || module == "Main" {
			continue
		}
		if sourceExtension(f.Path) == ".hs-boot" {
			r.boots[module] = append(r.boots[module], f.Path)
		} else {
			r.modules[module] = append(r.modules[module], f.Path)
		}
	}
}

// moduleName is the name a module header declares, "" without one.
func moduleName(source []byte, literate bool) string {
	if literate {
		source = unlit(source)
	}
	tokens := lex(source)
	for i, t := range tokens {
		switch {
		case t.k == tPragma:
			continue
		case t.k == tVariable && t.s == "module" && i+1 < len(tokens) && tokens[i+1].k == tCon:
			return tokens[i+1].s
		}
		return ""
	}
	return ""
}

func (r *resolver) packageInfoOf(file string) *packageInfo {
	for _, p := range r.packages {
		if lang.Within(file, p.directory) {
			return p
		}
	}
	return nil
}

func (r *resolver) projectOf(file string) *projectInfo {
	for _, p := range r.projects {
		if lang.Within(file, p.directory) {
			return p
		}
	}
	return nil
}

// components are those of a package whose source directories hold file; all of them
// for a file in none (Setup.hs) and for the package's manifest.
func (p *packageInfo) components(file string) []*component {
	var out []*component
	best := -1
	for _, c := range p.comps {
		n := -1
		for _, d := range c.directories {
			if directory := path.Join(p.directory, d); lang.Within(file, directory) {
				n = max(n, lang.Depth(directory))
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
func (r *resolver) declared(file string, own *packageInfo, manifest bool) map[string]dependency {
	out := map[string]dependency{}
	if own == nil {
		return out
	}
	comps := own.comps
	if setup(file, own) {
		// Setup.hs is built against its custom-setup's setup-depends, or else
		// against Cabal and base.
		comps = []*component{own.setup}
		if own.setup == nil {
			return map[string]dependency{"Cabal": {name: "Cabal"}, "base": {name: "base"}}
		}
	} else if !manifest {
		comps = own.components(file)
	} else if own.setup != nil {
		comps = append(append([]*component(nil), comps...), own.setup)
	}
	for _, c := range comps {
		for _, d := range c.dependencies {
			if previous, ok := out[d.name]; !ok || previous.constraint == "" && d.constraint != "" {
				out[d.name] = d
			}
		}
	}
	return out
}

// setup reports whether file is a package's Setup script.
func setup(file string, own *packageInfo) bool {
	base := path.Base(file)
	return path.Dir(file) == own.directory && (base == "Setup.hs" || base == "Setup.lhs")
}

// Implements: REQ-HASKELL-002, REQ-HASKELL-005, REQ-HASKELL-006, REQ-HASKELL-007, REQ-HASKELL-009
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch {
	case rawImport.Name == kindImport || rawImport.Name == kindSource || strings.HasPrefix(rawImport.Name, kindPackage):
		return r.module(file, rawImport)
	case rawImport.Name == kindDependency:
		own := r.byDirectory[path.Dir(file)]
		if own == nil {
			own = r.packageInfoOf(file)
		}
		if own != nil && rawImport.Module == own.name {
			return lang.Target{} // a component using the package's own library
		}
		if localPackage := r.byName[rawImport.Module]; localPackage != nil {
			return lang.Target{Local: localPackage.file}
		}
		return r.packageTarget(file, own, rawImport.Module, true)
	case rawImport.Name == kindMember:
		return r.member(file, rawImport.Module)
	case rawImport.Name == kindInclude:
		return r.include(file, rawImport.Module)
	case rawImport.Name == kindRepository:
		if p := r.projectOf(file); p != nil {
			if s, ok := p.repositories[rawImport.Module]; ok {
				return originTarget(rawImport.Module, s.location, s.tag)
			}
		}
	case rawImport.Name == kindExtra:
		p := r.projectOf(file)
		if p == nil {
			return lang.Target{}
		}
		e, ok := p.extras[rawImport.Module]
		if !ok {
			return lang.Target{}
		}
		if directory, ok := strings.CutPrefix(e.origin, "path:"); ok {
			if localPackage := r.byDirectory[path.Join(p.directory, directory)]; localPackage != nil {
				return lang.Target{Local: localPackage.file}
			}
			return lang.Target{Ecosystem: ecosystemHackage, Package: e.name, Origin: e.origin}
		}
		if l, ok := p.lock.get(e.name); ok && e.origin != "" && l.origin != "" {
			e = l
		}
		if e.origin != "" {
			return originTarget(e.name, e.origin, e.version)
		}
		return lang.Target{Ecosystem: ecosystemHackage, Package: e.name, Version: e.version, Pinned: lang.Pinned(e.version)}
	}
	return lang.Target{}
}

func (l *stackLock) get(name string) (extraDependency, bool) {
	if l == nil {
		return extraDependency{}, false
	}
	e, ok := l.packages[name]
	return e, ok
}

// originTarget is a package built from a repository at a tag or commit: pinned by a
// commit only.
func originTarget(name, location, reference string) lang.Target {
	t := lang.Target{Ecosystem: ecosystemHackage, Package: name, Origin: location, Version: reference}
	t.Pinned = lang.Commit(reference)
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
	if strings.HasSuffix(p, ".cabal") && r.Files[p] {
		return lang.Target{Local: p}
	}
	if localPackage := r.byDirectory[p]; localPackage != nil {
		return lang.Target{Local: localPackage.file}
	}
	if r.Directories[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// module resolves an imported module: to the file of the project that declares it
// (the importer's own package first, then the packages of its project and those it
// depends on), and else to the package providing it (external).
//
// Implements: REQ-HASKELL-006, REQ-HASKELL-009
func (r *resolver) module(file string, rawImport lang.RawImport) lang.Target {
	module := rawImport.Module
	own := r.packageInfoOf(file)
	boot := rawImport.Name == kindSource
	if packageName, ok := strings.CutPrefix(rawImport.Name, kindPackage); ok {
		// PackageImports names the package: only its modules are candidates.
		var in *packageInfo
		switch {
		case packageName == "this" || own != nil && packageName == own.name:
			in = own
		case r.byName[packageName] != nil:
			in = r.byName[packageName]
		default:
			return r.packageTarget(file, own, packageName, false)
		}
		if in == nil {
			return lang.Target{}
		}
		if f := r.localIn(file, module, boot, func(p *packageInfo) bool { return p == in }); f != "" {
			return local(file, f)
		}
		if f := r.probe(file, in, module, boot); f != "" {
			return local(file, f)
		}
		if in == own {
			return lang.Target{}
		}
		return lang.Target{Local: in.file}
	}
	if f := r.local(file, own, module, boot); f != "" {
		return local(file, f)
	}
	if module == "Main" {
		return lang.Target{} // a program, not a library module
	}
	for _, prefix := range []string{"Paths_", "PackageInfo_", "Build_"} {
		// Modules cabal (or a package's Setup.hs) generates from the package
		// description.
		if name, ok := strings.CutPrefix(module, prefix); ok {
			if localPackage := r.byName[strings.ReplaceAll(name, "_", "-")]; localPackage != nil {
				return lang.Target{Local: localPackage.file}
			}
			return lang.Target{}
		}
	}
	return r.packageTarget(file, own, r.packageOf(file, own, module), false)
}

func local(file, f string) lang.Target {
	if f == file {
		return lang.Target{}
	}
	return lang.Target{Local: f}
}

// local finds the file declaring module: in the importer's own package (its component's
// source directories first, then a generated module's source - .y, .x, .chs - probed
// there), else in a package of the same project or one the importer depends on. A
// file of no package takes the nearest declaration anywhere.
func (r *resolver) local(file string, own *packageInfo, module string, boot bool) string {
	if own == nil {
		return r.localIn(file, module, boot, nil)
	}
	if f := r.localIn(file, module, boot, func(p *packageInfo) bool { return p == own }); f != "" {
		return f
	}
	if f := r.probe(file, own, module, boot); f != "" {
		return f
	}
	declared := r.declared(file, own, false)
	return r.localIn(file, module, boot, func(p *packageInfo) bool {
		if p == nil {
			return false
		}
		_, dependency := declared[p.name]
		return dependency || p.project != nil && p.project == own.project || own.project != nil && member(own.project, p)
	})
}

func member(project *projectInfo, p *packageInfo) bool {
	for _, m := range project.members {
		if m == p.directory || m == p.file {
			return true
		}
	}
	return false
}

// localIn picks among the files declaring module those whose package keep accepts (nil
// accepts all): one in the importer's component directories, else the nearest.
func (r *resolver) localIn(file, module string, boot bool, keep func(*packageInfo) bool) string {
	candidates := r.modules[module]
	if boot {
		candidates = append(append([]string(nil), r.boots[module]...), candidates...)
	}
	var ok []string
	for _, c := range candidates {
		if keep == nil || keep(r.packageInfoOf(c)) {
			ok = append(ok, c)
		}
	}
	if len(ok) == 0 {
		return ""
	}
	if own := r.packageInfoOf(file); own != nil {
		for _, component := range own.components(file) {
			for _, d := range component.directories {
				for _, c := range ok {
					if lang.Within(c, path.Join(own.directory, d)) {
						return c
					}
				}
			}
		}
	}
	best, bestLength := ok[0], -1
	for _, c := range ok {
		if n := lang.CommonDirectories(file, c); n > bestLength {
			best, bestLength = c, n
		}
	}
	return best
}

// probe looks for the source of a module under the importer's component directories
// by its path, for the modules no header declares: those generated from Alex, Happy,
// c2hs or hsc2hs sources.
func (r *resolver) probe(file string, own *packageInfo, module string, boot bool) string {
	relative := strings.ReplaceAll(module, ".", "/")
	extensions := []string{".hs", ".lhs", ".hsc", ".y", ".ly", ".x", ".chs", ".hsig"}
	if boot {
		extensions = append([]string{".hs-boot"}, extensions...)
	}
	for _, c := range own.components(file) {
		for _, d := range c.directories {
			for _, extension := range extensions {
				if p := path.Join(own.directory, d, relative+extension); r.Files[p] {
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
func (r *resolver) packageOf(file string, own *packageInfo, module string) string {
	declared := r.declared(file, own, false)
	isDeclared := func(p string) bool { _, ok := declared[p]; return ok && !stdPackages[p] }
	for _, p := range moduleTable[module] {
		if stdPackages[p] || isDeclared(p) {
			return p
		}
	}
	if _, packages := tableMatch(module, isDeclared); len(packages) > 0 {
		return packages[0]
	}
	names := map[string]string{}
	for name := range declared {
		if !stdPackages[name] {
			names[lang.FoldSeparators(name)] = name
		}
	}
	if p := runMatch(module, names); p != "" {
		return p
	}
	project := r.projectOf(file)
	if _, packages := tableMatch(module, nil); len(packages) > 0 {
		for _, p := range packages {
			if project.pinned(p) {
				return p
			}
		}
		return packages[0]
	}
	// A declared package named after its first word (hermes-json for Data.Hermes).
	firsts := map[string]string{}
	for name := range declared {
		first, _, _ := strings.Cut(name, "-")
		if !stdPackages[name] && first != name {
			if _, duplicate := firsts[lang.FoldSeparators(first)]; duplicate {
				firsts[lang.FoldSeparators(first)] = ""
			} else {
				firsts[lang.FoldSeparators(first)] = name
			}
		}
	}
	if p := runMatch(module, firsts); p != "" {
		return p
	}
	if project != nil {
		known := map[string]string{}
		for _, name := range project.names() {
			known[lang.FoldSeparators(name)] = name
		}
		if p := runMatch(module, known); p != "" {
			return p
		}
	}
	return guessName(module)
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
	_, d := p.repositories[name]
	e := p.plan != nil && p.plan.packages[name] != nil
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
		for n := range p.lock.packages {
			out = append(out, n)
		}
	}
	if p.plan != nil {
		for n := range p.plan.packages {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// packageTarget is a package as the importer's project pins it: GHC's own packages to
// haskell-std; a package of the repository to its description; and else a Hackage
// package at the version the build plan, the freeze file or an exact constraint of
// cabal.project, a source-repository-package, stack.yaml.lock or stack.yaml's
// extra-deps fix (pinned, the declared range kept as requested), else as build-depends
// declares it: an exact ==x pins, a range does not, no version floats unless a stack
// snapshot fixes it (then its version is the snapshot's, unknown offline, and shown
// as the snapshot's name). A package nothing declares or pins is unresolved.
//
// Implements: REQ-HASKELL-007, REQ-HASKELL-008, REQ-HASKELL-009, REQ-HASKELL-010
func (r *resolver) packageTarget(file string, own *packageInfo, name string, manifest bool) lang.Target {
	if stdPackages[name] {
		return lang.Target{Ecosystem: ecosystemStd, Package: name}
	}
	if localPackage := r.byName[name]; localPackage != nil {
		return lang.Target{Local: localPackage.file}
	}
	d, declared := r.declared(file, own, manifest)[name]
	if !declared && !manifest && own != nil {
		// Declared by another component of the package: the flag or stanza that
		// guards it is not evaluated, so it counts.
		d, declared = r.declared(file, own, true)[name]
	}
	project := r.projectOf(file)
	if own != nil && own.project != nil {
		project = own.project
	}
	t := lang.Target{Ecosystem: ecosystemHackage, Package: name}
	exact := func(v string) lang.Target {
		t.Version, t.Pinned = v, true
		if declared && d.constraint != "" && d.constraint != v {
			t.Requested = d.constraint
		}
		return t
	}
	if project != nil {
		if project.plan != nil {
			if planned := project.plan.packages[name]; planned != nil && !planned.local {
				if planned.origin != "" {
					return originTarget(name, planned.origin, planned.tag)
				}
				return exact(planned.version)
			}
		}
		if v, ok := project.pins[name]; ok {
			return exact(v)
		}
		if s, ok := project.repositories[name]; ok {
			return originTarget(name, s.location, s.tag)
		}
		if l, ok := project.lock.get(name); ok {
			if l.origin != "" {
				return originTarget(name, l.origin, l.version)
			}
			return exact(l.version)
		}
		if e, ok := project.extras[name]; ok {
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
		if project != nil && project.stack != nil && project.stack.snapshot != "" {
			t.Version = project.stack.snapshot
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
	if t.Ecosystem != ecosystemHackage {
		return nil
	}
	for _, p := range r.projects {
		if p.plan == nil {
			continue
		}
		planned := p.plan.packages[t.Package]
		if planned == nil || t.Version != planned.version && t.Version != planned.tag {
			continue
		}
		var out []lang.Target
		for _, name := range planned.depends {
			dependency := p.plan.packages[name]
			if stdPackages[name] || dependency == nil || dependency.local {
				continue
			}
			if dependency.origin != "" {
				out = append(out, originTarget(name, dependency.origin, dependency.tag))
				continue
			}
			out = append(out, lang.Target{Ecosystem: ecosystemHackage, Package: name, Version: dependency.version, Pinned: true})
		}
		return out
	}
	return nil
}
