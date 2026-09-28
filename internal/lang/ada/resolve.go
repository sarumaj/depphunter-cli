package ada

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// crateDir is a directory with an alire.toml: its manifest, the lock file's
// solution and what Alire fetched into alire/cache/.
type crateDir struct {
	dir       string
	m         *manifest
	lock      map[string]*lockState
	installed map[string]*installedCrate
	units     map[string]string // unit -> installed crate
	projects  map[string]string // project name -> installed crate
}

// project is a GNAT project file and the source directories it names.
type project struct {
	path, dir string
	g         *gpr
	dirs      []srcDir
	refs      []string // the local project files it withs, extends or aggregates
}

type srcDir struct {
	dir       string
	recursive bool
}

type resolver struct {
	files   map[string]bool
	dirs    map[string]bool
	byBase  map[string][]string // lower-case base name -> files
	specs   map[string][]string // unit -> spec files
	bodies  map[string][]string // unit -> body files (and subunits' files)
	roots   map[string]bool     // first segments of the repository's units
	gprs    map[string]*project
	gprBase map[string][]string // project file name without .gpr, lower case -> paths
	crates  map[string]*crateDir
	order   []*crateDir // shallowest first
	own     map[string]bool
	// visible holds, per directory of an Ada source, the source directories of
	// the projects that have it and of the projects those import.
	visible map[string]map[*project]bool
	owners  map[string][]*project // directory -> projects whose sources it holds
}

// Implements: REQ-ADA-004, REQ-ADA-005, REQ-ADA-006, REQ-ADA-008
func newResolver(root string, all []*scan.File) *resolver {
	_ = root
	r := &resolver{
		files: map[string]bool{}, dirs: map[string]bool{}, byBase: map[string][]string{},
		specs: map[string][]string{}, bodies: map[string][]string{}, roots: map[string]bool{},
		gprs: map[string]*project{}, gprBase: map[string][]string{}, crates: map[string]*crateDir{},
		own: map[string]bool{}, visible: map[string]map[*project]bool{}, owners: map[string][]*project{},
	}
	var sources []*scan.File
	for _, f := range all {
		if generated(f) {
			continue
		}
		r.files[f.Path] = true
		base := lower(path.Base(f.Path))
		r.byBase[base] = append(r.byBase[base], f.Path)
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		readable := !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize
		switch {
		case path.Base(f.Path) == "alire.toml" && readable:
			if src, err := os.ReadFile(f.Abs); err == nil {
				c := &crateDir{dir: path.Dir(f.Path), m: readManifest(src), lock: map[string]*lockState{}}
				absDir := filepath.Dir(f.Abs)
				// Alire 1.1 and later keep the lock file in alire/; before, it
				// sat beside the manifest.
				for _, l := range []string{filepath.Join(absDir, "alire", "alire.lock"), filepath.Join(absDir, "alire.lock")} {
					if src, err := os.ReadFile(l); err == nil {
						c.lock = readLock(src)
						break
					}
				}
				c.readInstalled(absDir)
				c.readShared(sharedReleases(os.Getenv))
				r.crates[c.dir] = c
				r.order = append(r.order, c)
				if c.m.name != "" {
					r.own[c.m.name] = true
					r.own[c.m.name+"_config"] = true
				}
			}
		case gprFile(f.Path) && readable:
			if src, err := os.ReadFile(f.Abs); err == nil {
				p := &project{path: f.Path, dir: path.Dir(f.Path), g: readGPR(src)}
				r.gprs[p.path] = p
				name := lower(strings.TrimSuffix(path.Base(f.Path), path.Ext(f.Path)))
				r.gprBase[name] = append(r.gprBase[name], p.path)
				r.own[name] = true
				if p.g.low != "" {
					r.own[p.g.low] = true
				}
			}
		case source(f.Path) && readable:
			sources = append(sources, f)
		}
	}
	sort.Slice(r.order, func(i, j int) bool {
		di, dj := depth(r.order[i].dir), depth(r.order[j].dir)
		if di != dj {
			return di < dj
		}
		return r.order[i].dir < r.order[j].dir
	})
	for _, list := range r.gprBase {
		sort.Strings(list)
	}
	for _, d := range declarations(sources) {
		for _, u := range d.units {
			switch u.kind {
			case 's':
				r.specs[u.name] = append(r.specs[u.name], d.file)
			default:
				r.bodies[u.name] = append(r.bodies[u.name], d.file)
			}
			root, _, _ := strings.Cut(u.name, ".")
			r.roots[root] = true
		}
	}
	r.readProjects(sources)
	for _, m := range []map[string][]string{r.specs, r.bodies} {
		for _, list := range m {
			sort.Strings(list)
		}
	}
	return r
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

type declared struct {
	file  string
	units []unit
}

// declarations reads the unit every source declares (every unit of a .ada
// file), on all cores.
func declarations(files []*scan.File) []declared {
	out := make([]declared, len(files))
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f := files[i]
				out[i].file = f.Path
				if src, err := os.ReadFile(f.Abs); err == nil {
					out[i].units = units(src, strings.EqualFold(path.Ext(f.Path), ".ada"))
				}
			}
		}()
	}
	for i := range files {
		next <- i
	}
	close(next)
	wg.Wait()
	return out
}

// readProjects works out each project's source directories and imports, the
// Naming packages' explicit entries, and which projects' sources every Ada
// source's directory can see.
func (r *resolver) readProjects(sources []*scan.File) {
	paths := sortedKeys(r.gprs)
	for _, pp := range paths {
		p := r.gprs[pp]
		if !p.g.dirsSet {
			p.dirs = []srcDir{{dir: p.dir}}
		}
		for _, d := range p.g.sourceDirs {
			dir, rec := dirSpec(d.s)
			if path.IsAbs(dir) {
				continue
			}
			dir = path.Join(p.dir, dir)
			if strings.HasPrefix(dir, "../") || dir == ".." {
				continue
			}
			p.dirs = append(p.dirs, srcDir{dir: dir, recursive: rec})
		}
		var refs []item
		refs = append(refs, p.g.withs...)
		if p.g.extends != nil {
			refs = append(refs, *p.g.extends)
		}
		refs = append(refs, p.g.projectFiles...)
		for _, ref := range refs {
			if t := r.localProject(p.path, ref.s); t != "" {
				p.refs = append(p.refs, t)
			}
		}
	}
	for _, pp := range paths {
		p := r.gprs[pp]
		for unit, file := range p.g.specs {
			r.specs[unit] = append(r.specs[unit], r.inProject(p, file)...)
		}
		for unit, file := range p.g.bodies {
			r.bodies[unit] = append(r.bodies[unit], r.inProject(p, file)...)
		}
	}
	if len(r.gprs) == 0 {
		return
	}
	for _, f := range sources {
		d := path.Dir(f.Path)
		if _, ok := r.visible[d]; ok {
			continue
		}
		vis := map[*project]bool{}
		for _, p := range r.projectsOf(d) {
			r.closure(p, vis)
		}
		r.visible[d] = vis
	}
}

// projectsOf lists the projects whose source directories hold dir.
func (r *resolver) projectsOf(dir string) []*project {
	if ps, ok := r.owners[dir]; ok {
		return ps
	}
	var out []*project
	for _, pp := range sortedKeys(r.gprs) {
		p := r.gprs[pp]
		for _, sd := range p.dirs {
			if dir == sd.dir || sd.recursive && (sd.dir == "." || strings.HasPrefix(dir, sd.dir+"/")) {
				out = append(out, p)
				break
			}
		}
	}
	r.owners[dir] = out
	return out
}

// closure adds p and the projects it imports, transitively (at most 64).
func (r *resolver) closure(p *project, vis map[*project]bool) {
	stack := []*project{p}
	for len(stack) > 0 && len(vis) < 64 {
		q := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if vis[q] {
			continue
		}
		vis[q] = true
		for _, ref := range q.refs {
			if g := r.gprs[ref]; g != nil && !vis[g] {
				stack = append(stack, g)
			}
		}
	}
}

// inProject finds a file named by a project's Naming package in its source
// directories.
func (r *resolver) inProject(p *project, file string) []string {
	var out []string
	for _, f := range r.byBase[lower(path.Base(file))] {
		d := path.Dir(f)
		for _, sd := range p.dirs {
			if d == sd.dir || sd.recursive && (sd.dir == "." || strings.HasPrefix(d, sd.dir+"/")) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// localProject resolves a project file reference to a project file of the
// repository: relative to the referencing file, else the project file of that
// name nearest to it (GPR_PROJECT_PATH is not known).
func (r *resolver) localProject(from, ref string) string {
	ref = strings.ReplaceAll(ref, "\\", "/")
	if !gprFile(ref) {
		ref += ".gpr"
	}
	if !path.IsAbs(ref) {
		if t := path.Join(path.Dir(from), ref); r.gprs[t] != nil {
			return t
		}
	}
	name := lower(strings.TrimSuffix(path.Base(ref), path.Ext(ref)))
	return pick(from, r.gprBase[name])
}

// pick chooses among files the one sharing the longest directory prefix with
// from (from itself is none); ties go to the first in order.
func pick(from string, files []string) string {
	best, bestLen := "", -1
	for _, f := range files {
		if f == from {
			continue
		}
		if n := commonDirs(f, from); n > bestLen {
			best, bestLen = f, n
		}
	}
	return best
}

func commonDirs(a, b string) int {
	as, bs := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// choose picks the unit's file for an importing file: among the files in
// source directories of projects the importer's projects see, if any, else
// among all, the nearest.
func (r *resolver) choose(from string, files []string) string {
	if vis := r.visible[path.Dir(from)]; len(vis) > 0 {
		var seen []string
		for _, f := range files {
			if f == from {
				continue
			}
			for _, p := range r.owners[path.Dir(f)] {
				if vis[p] {
					seen = append(seen, f)
					break
				}
			}
		}
		if len(seen) > 0 {
			return pick(from, seen)
		}
	}
	return pick(from, files)
}

// scope is the crates whose manifests speak for file: the nearest alire.toml
// above it, or, for a file none governs, every one, shallowest first.
func (r *resolver) scope(file string) []*crateDir {
	for d := path.Dir(file); ; d = path.Dir(d) {
		if c, ok := r.crates[d]; ok {
			return []*crateDir{c}
		}
		if d == "." || d == "/" || d == "" {
			return r.order
		}
	}
}

// Implements: REQ-ADA-004, REQ-ADA-005, REQ-ADA-006
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindWith, kindParent:
		return r.unit(file, imp.Module)
	case kindBody:
		if f := r.choose(file, r.specs[imp.Module]); f != "" {
			return lang.Target{Local: f}
		}
	case kindSeparate:
		if f := r.choose(file, r.bodies[imp.Module]); f != "" {
			return lang.Target{Local: f}
		}
	case kindProject:
		return r.project(file, imp.Module)
	case kindDir:
		dir, _ := dirSpec(imp.Module)
		if t := path.Join(path.Dir(file), dir); t != "." && !strings.HasPrefix(t, "../") && r.dirs[t] {
			return lang.Target{Local: t}
		}
	case kindMain:
		if f := r.main(file, imp.Module); f != "" {
			return lang.Target{Local: f}
		}
	case kindDep, kindPin:
		if c := r.crates[path.Dir(file)]; c != nil {
			return r.crate(c, imp.Module)
		}
	case kindProjectFile:
		if t := path.Join(path.Dir(file), imp.Module); r.files[t] {
			return lang.Target{Local: t}
		}
	}
	return lang.Target{}
}

// unit resolves a library unit: a unit of the repository (its spec, else a
// body that is its own declaration), a predefined unit, a unit (or the nearest
// parent unit) of a crate Alire fetched, a declared crate the curated table
// names or the unit's name spells, and else an unresolved crate. A unit whose crate is the repository's own, or
// whose first segment only the repository's units have, is dropped (a file
// generated at build time).
//
// Implements: REQ-ADA-004, REQ-ADA-007
func (r *resolver) unit(file, u string) lang.Target {
	if f := r.choose(file, r.specs[u]); f != "" {
		return lang.Target{Local: f}
	}
	if f := r.choose(file, r.bodies[u]); f != "" {
		return lang.Target{Local: f}
	}
	if len(r.specs[u])+len(r.bodies[u]) > 0 {
		return lang.Target{} // the file's own unit
	}
	if pkg, ok := stdPackage(u); ok {
		return lang.Target{Ecosystem: ecoStd, Package: pkg}
	}
	scope := r.scope(file)
	// the crate Alire fetched that has the unit, else its nearest parent unit
	segments := strings.Split(u, ".")
	for k := len(segments); k > 0; k-- {
		for _, c := range scope {
			if name, ok := c.units[strings.Join(segments[:k], ".")]; ok {
				return r.crate(c, name)
			}
		}
	}
	for _, c := range scope {
		if t, ok := r.declaredCrate(c, u); ok {
			return t
		}
	}
	tc, _ := knownCrate(u)
	root, _, _ := strings.Cut(u, ".")
	switch {
	case tc != "" && r.own[tc]:
		return lang.Target{}
	case tc != "":
		return lang.Target{Ecosystem: ecoAlire, Package: tc, Unresolved: true}
	case r.roots[root] || r.own[root]:
		return lang.Target{}
	}
	for _, c := range scope {
		if c.m.name != "" && spellsCrate(u, c.m.name) > 0 {
			return lang.Target{}
		}
	}
	return lang.Target{Ecosystem: ecoAlire, Package: root, Unresolved: true}
}

// declaredCrate is the crate c knows (declared, pinned, locked or installed)
// that the curated table names for u or that u's name spells, the longer
// match winning and the table on a tie.
func (r *resolver) declaredCrate(c *crateDir, u string) (lang.Target, bool) {
	tc, tk := knownCrate(u)
	sc, sk := "", 0
	for _, name := range c.known() {
		if k := spellsCrate(u, name); k > sk {
			sc, sk = name, k
		}
	}
	switch {
	case tc != "" && c.knows(tc) && tk >= sk:
		return r.crate(c, tc), true
	case sc != "":
		return r.crate(c, sc), true
	case tc != "" && c.knows(tc):
		return r.crate(c, tc), true
	}
	return lang.Target{}, false
}

// known lists the crates c knows, sorted.
func (c *crateDir) known() []string {
	set := map[string]bool{}
	for n := range c.m.deps {
		set[n] = true
	}
	for n := range c.m.pins {
		set[n] = true
	}
	for n := range c.lock {
		set[n] = true
	}
	for n := range c.installed {
		set[n] = true
	}
	delete(set, c.m.name)
	return sortedKeys(set)
}

func (c *crateDir) knows(name string) bool {
	if name == c.m.name {
		return false
	}
	_, a := c.m.deps[name]
	_, b := c.m.pins[name]
	_, l := c.lock[name]
	_, i := c.installed[name]
	return a || b || l || i
}

// spellsCrate is how many leading segments of unit spell crate: joined by _
// (Semantic_Versioning, GNATCOLL.SQL is gnatcoll_sql), with an ada_ prefix or
// an _ada or ada suffix (TOML is ada_toml, URI uri_ada, ANSI ansiada) or a lib
// prefix (libgpr).
func spellsCrate(unit, crate string) int {
	segments := strings.Split(unit, ".")
	for k := len(segments); k > 0; k-- {
		s := strings.Join(segments[:k], "_")
		switch crate {
		case s, s + "_ada", "ada_" + s, s + "ada", "lib" + s:
			return k
		}
	}
	return 0
}

// project resolves a project file a .gpr imports: a project file of the
// repository, else a crate's (the one Alire fetched that ships it, a declared
// crate of that name or the table's), else an unresolved crate named after
// it. A missing project named by a path (config/x_config.gpr) or after the
// repository's own crate is dropped.
//
// Implements: REQ-ADA-005
func (r *resolver) project(file, ref string) lang.Target {
	if t := r.localProject(file, ref); t != "" {
		return lang.Target{Local: t}
	}
	ref = strings.ReplaceAll(ref, "\\", "/")
	name := lower(strings.TrimSuffix(path.Base(ref), path.Ext(ref)))
	if !gprFile(ref) {
		name = lower(path.Base(ref))
	}
	scope := r.scope(file)
	for _, c := range scope {
		if crate, ok := c.projects[name]; ok {
			return r.crate(c, crate)
		}
	}
	crate := name
	if k, ok := knownProjects[name]; ok {
		crate = k
	}
	for _, c := range scope {
		for _, n := range []string{name, crate} {
			if c.knows(n) {
				return r.crate(c, n)
			}
		}
	}
	if strings.Contains(ref, "/") || r.own[name] || r.own[crate] {
		return lang.Target{}
	}
	return lang.Target{Ecosystem: ecoAlire, Package: crate, Unresolved: true}
}

// main finds a project's main file in its source directories, else the
// nearest file of that name.
func (r *resolver) main(file, m string) string {
	names := []string{lower(path.Base(m))}
	if !source(m) {
		names = append(names, names[0]+".adb", strings.ReplaceAll(names[0], ".", "-")+".adb")
	}
	for _, n := range names {
		candidates := r.byBase[n]
		if p := r.gprs[file]; p != nil {
			if in := r.inProject(p, n); len(in) > 0 {
				candidates = in
			}
		}
		if f := pick(file, candidates); f != "" {
			return f
		}
	}
	return ""
}

// crate is the crate name as c's manifest pins or depends on it:
//   - a pin to a directory is that directory (its alire.toml);
//   - a pin to a git repository at a commit pins it, on a branch (or none)
//     floats; a pin to a version pins it;
//   - else the lock file's solution pins the version it chose;
//   - else =1.2.3 or a bare 1.2.3 pins (Alire reads a bare version as exact)
//     and ^, ~, >=, * and & ranges float, the version Alire fetched shown.
//
// A git server other than the public forges is the crate's origin.
//
// Implements: REQ-ADA-006
func (r *resolver) crate(c *crateDir, name string) lang.Target {
	t := lang.Target{Ecosystem: ecoAlire, Package: name}
	d := c.m.deps[name]
	constraint := ""
	if d != nil {
		constraint = d.constraint
	}
	if p := c.m.pins[name]; p != nil {
		return r.pinned(c, t, constraint, p.path, p.url, p.commit, p.branch, p.version)
	}
	if st := c.lock[name]; st != nil {
		if st.linkPath != "" || st.linkURL != "" {
			// a pin of a crate the lock file's solution took over from a
			// manifest (the crate's own or a linked crate's)
			return r.pinned(c, t, constraint, st.linkPath, st.linkURL, st.linkCommit, st.linkBranch, "")
		}
		if st.version != "" {
			t.Version, t.Pinned = st.version, true
			if constraint == "" {
				constraint = st.versions // a crate only the solution has
			}
			if constraint != "" && constraint != st.version && constraint != "="+st.version {
				t.Requested = constraint
			}
			return t
		}
	}
	installed := ""
	if ic := c.installed[name]; ic != nil {
		installed = ic.version
	}
	switch {
	case constraint != "":
		if v, ok := exactVersion(constraint); ok {
			t.Version, t.Pinned = v, true
			return t
		}
		t.Version, t.Floating = constraint, true
		if installed != "" {
			t.Version, t.Requested = installed, constraint
		}
	case installed != "":
		t.Version = installed
	default:
		t.Floating = true
	}
	return t
}

// pinned is a crate a pin fulfils: a directory of the repository is a local
// edge (a directory elsewhere floats, with the directory as origin); a git
// repository pins at a commit and floats on a branch or its default branch,
// with a git server other than the public forges as origin (a git pin's
// checkout under alire/cache/pins is not a local edge); a version pins.
//
// Implements: REQ-ADA-006
func (r *resolver) pinned(c *crateDir, t lang.Target, constraint, dir, url, commit, branch, version string) lang.Target {
	switch {
	case url != "":
		url = strings.TrimPrefix(url, "git+")
		switch {
		case commit != "":
			t.Version, t.Pinned = commit, true
		case branch != "":
			t.Version, t.Floating = branch, true
		default:
			t.Floating = true
		}
		if !public(url) {
			t.Origin = url
		}
	case dir != "":
		if lt := r.localCrate(c.dir, dir); lt.Local != "" {
			return lt
		}
		t.Origin, t.Floating = dir, true
		return t
	case version != "":
		t.Version, t.Pinned = strings.TrimPrefix(version, "="), true
	}
	if constraint != "" && constraint != t.Version && t.Pinned {
		t.Requested = constraint
	}
	return t
}

// localCrate is a crate in a directory of the repository: its alire.toml, else
// the directory; outside the repository, nothing.
func (r *resolver) localCrate(dir, rel string) lang.Target {
	rel = strings.ReplaceAll(rel, "\\", "/")
	if path.IsAbs(rel) {
		return lang.Target{}
	}
	d := path.Join(dir, rel)
	switch {
	case r.files[path.Join(d, "alire.toml")]:
		return lang.Target{Local: path.Join(d, "alire.toml")}
	case d != "." && r.dirs[d]:
		return lang.Target{Local: d}
	}
	return lang.Target{}
}

// public reports whether a repository is on a public forge (or unknown), as
// opposed to a git server of the organization's own.
func public(url string) bool {
	if url == "" {
		return true
	}
	host, _, _ := strings.Cut(lang.RepoName(url), "/")
	switch host {
	case "github.com", "gitlab.com", "bitbucket.org", "codeberg.org", "git.sr.ht", "sr.ht":
		return true
	}
	return false
}

// Dependencies lists what a crate depends on: the lock file's solution says
// (each at the version it chose), else the alire.toml of the crate Alire
// fetched.
//
// Implements: REQ-ADA-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoAlire {
		return nil
	}
	for _, c := range r.order {
		st := c.lock[t.Package]
		if st == nil || st.version == "" {
			continue
		}
		var out []lang.Target
		for _, name := range st.deps {
			if dt := r.dependency(c, name, st.constraints[name]); dt.Ecosystem != "" {
				out = append(out, dt)
			}
		}
		return out
	}
	for _, c := range r.order {
		ic := c.installed[t.Package]
		if ic == nil {
			continue
		}
		var out []lang.Target
		for _, name := range sortedKeys(ic.m.deps) {
			if dt := r.dependency(c, name, ic.m.deps[name].constraint); dt.Ecosystem != "" {
				out = append(out, dt)
			}
		}
		return out
	}
	return nil
}

// dependency is a crate some crate depends on with a constraint: as the
// project's pins and lock file have it, else by the constraint.
func (r *resolver) dependency(c *crateDir, name, constraint string) lang.Target {
	if c.m.pins[name] != nil || c.lock[name] != nil || c.m.deps[name] != nil {
		t := r.crate(c, name)
		if t.Pinned && t.Requested == "" && constraint != "" && constraint != t.Version {
			t.Requested = constraint
		}
		return t
	}
	t := lang.Target{Ecosystem: ecoAlire, Package: name}
	if v, ok := exactVersion(constraint); ok {
		t.Version, t.Pinned = v, true
	} else {
		t.Version, t.Floating = constraint, true
	}
	return t
}

// Installed reports whether a crate's dependencies come from what Alire
// fetched rather than from a lock file.
func (r *resolver) Installed(t lang.Target) bool {
	if t.Ecosystem != ecoAlire {
		return false
	}
	for _, c := range r.order {
		if st := c.lock[t.Package]; st != nil && st.version != "" {
			return false
		}
	}
	for _, c := range r.order {
		if _, ok := c.installed[t.Package]; ok {
			return true
		}
	}
	return false
}
