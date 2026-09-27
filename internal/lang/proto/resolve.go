package proto

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// conventional are the directories, relative to the repository root, that protoc's
// -I is usually pointed at when no Buf configuration says where the import roots are.
var conventional = []string{".", "proto", "protos", "api", "src/main/proto"}

// config is one buf.yaml or buf.work.yaml.
type config struct {
	dir   string
	file  *bufFile
	roots []string // import roots, project-relative
	lock  map[string]locked
}

// dep is a module a configuration declares or its lock records.
type dep struct {
	name, ref string
	lock      *locked
}

type resolver struct {
	files   map[string]bool
	dirs    map[string]bool
	byBase  map[string][]string // file name -> the project files so named
	work    map[string]*config  // directory -> its buf.work.yaml or v2 buf.yaml
	modules map[string]*config  // directory -> its v1 (or v1beta1) buf.yaml
	all     []*config           // every buf.yaml, shallowest first
	locks   map[string]map[string]locked
}

func depth(p string) int {
	if p == "." {
		return 0
	}
	return strings.Count(p, "/") + 1
}

// newResolver reads every Buf configuration and lock file of the repository.
//
// Implements: REQ-PROTO-004, REQ-PROTO-005, REQ-PROTO-007
func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, byBase: map[string][]string{},
		work: map[string]*config{}, modules: map[string]*config{}, locks: map[string]map[string]locked{}}
	var configs []*scan.File
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
			if d == "." {
				break
			}
		}
		base := path.Base(f.Path)
		r.byBase[base] = append(r.byBase[base], f.Path)
		switch fileClass(f.Path) {
		case classYAML, classWork, classLock:
			if !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize {
				configs = append(configs, f)
			}
		}
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].Path < configs[j].Path })
	for _, f := range configs {
		src, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		class, dir := fileClass(f.Path), path.Dir(f.Path)
		b := readBuf(class, src)
		if class == classLock {
			m := map[string]locked{}
			for _, l := range b.locks {
				m[l.name] = l
			}
			r.locks[dir] = m
			continue
		}
		c := &config{dir: dir, file: b}
		for _, root := range b.roots {
			if p := path.Join(dir, root.value); !strings.HasPrefix(p, "../") {
				c.roots = append(c.roots, p)
			}
		}
		switch {
		case class == classWork:
			r.work[dir] = c
		case b.v2:
			if len(c.roots) == 0 {
				c.roots = []string{dir}
			}
			r.work[dir] = c
			r.all = append(r.all, c)
		default:
			if len(c.roots) == 0 {
				c.roots = []string{dir}
			}
			r.modules[dir] = c
			r.all = append(r.all, c)
		}
	}
	for _, c := range r.all {
		c.lock = r.locks[c.dir]
	}
	sort.SliceStable(r.all, func(i, j int) bool { return depth(r.all[i].dir) < depth(r.all[j].dir) })
	return r
}

// nearest finds the configuration in m of dir or its closest ancestor.
func nearest(m map[string]*config, dir string) *config {
	for d := dir; ; d = path.Dir(d) {
		if c := m[d]; c != nil {
			return c
		}
		if d == "." || d == "/" {
			return nil
		}
	}
}

// under reports whether p is dir or inside it.
func under(p, dir string) bool {
	return dir == "." || p == dir || strings.HasPrefix(p, dir+"/")
}

// scope is what Buf's configuration says about one file: its workspace's import roots
// and the modules its module depends on.
type scope struct {
	buf   bool // a Buf configuration governs the file
	roots []string
	deps  []dep
}

// scopeOf finds the Buf workspace (buf.work.yaml or a v2 buf.yaml) and the v1 module
// a file belongs to, nearest first. A file no configuration governs sees the
// dependencies of every buf.yaml in the repository, shallowest first.
//
// Implements: REQ-PROTO-004, REQ-PROTO-005
func (r *resolver) scopeOf(file string) scope {
	dir := path.Dir(file)
	ws, mod := nearest(r.work, dir), nearest(r.modules, dir)
	if ws != nil && mod != nil && !under(mod.dir, ws.dir) {
		mod = nil // a module above the workspace is not part of it
	}
	var sc scope
	var configs []*config
	switch {
	case ws != nil:
		sc.buf, sc.roots = true, ws.roots
		if ws.file.v2 {
			configs = append(configs, ws)
		}
		if mod != nil {
			configs = append(configs, mod)
		}
	case mod != nil:
		sc.buf, sc.roots = true, mod.roots
		configs = append(configs, mod)
	default:
		configs = r.all
	}
	for _, c := range configs {
		seen := map[string]bool{}
		for _, d := range c.file.deps {
			name, ref := moduleRef(d.value)
			seen[name] = true
			var l *locked
			if e, ok := c.lock[name]; ok {
				l = &e
			}
			sc.deps = append(sc.deps, dep{name: name, ref: ref, lock: l})
		}
		for _, name := range sortedLocks(c.lock) { // installed as a dependency's dependency
			if !seen[name] {
				e := c.lock[name]
				sc.deps = append(sc.deps, dep{name: name, lock: &e})
			}
		}
	}
	return sc
}

func sortedLocks(m map[string]locked) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Resolve follows an import to a project file, a well-known type or a Buf Schema
// Registry module; a Buf file's entries to the modules, plugins and directories they
// name.
//
// Implements: REQ-PROTO-004, REQ-PROTO-005, REQ-PROTO-006, REQ-PROTO-007, REQ-PROTO-008
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindDirectory:
		if p := path.Join(path.Dir(file), imp.Module); r.dirs[p] && !strings.HasPrefix(p, "../") {
			return lang.Target{Local: p}
		}
		return lang.Target{}
	case kindDep:
		name, ref := moduleRef(imp.Module)
		var l *locked
		if e, ok := r.locks[path.Dir(file)][name]; ok {
			l = &e
		}
		return dep{name: name, ref: ref, lock: l}.target()
	case kindLock:
		l := r.locks[path.Dir(file)][imp.Module]
		return dep{name: imp.Module, lock: &l}.target()
	case kindPlugin:
		name, version := moduleRef(imp.Module)
		return lang.Target{Ecosystem: ecoBuf, Package: name, Version: version, Pinned: lang.Pinned(version), Floating: version == ""}
	}
	return r.resolveImport(file, imp.Module)
}

// target is a declared module: pinned by its lock's commit, else by a commit ref; a
// label, tag or branch ref is shown but moves; no ref floats.
//
// Implements: REQ-PROTO-008
func (d dep) target() lang.Target {
	t := lang.Target{Ecosystem: ecoBuf, Package: d.name}
	switch {
	case d.lock != nil && d.lock.commit != "":
		t.Version, t.Pinned = d.lock.commit, true
		if d.ref != "" && d.ref != d.lock.commit {
			t.Requested = d.ref
		}
	case d.ref != "":
		t.Version, t.Pinned = d.ref, commit(d.ref)
	default:
		t.Floating = true
	}
	return t
}

// commit reports whether a module ref is a commit: a Buf Schema Registry commit id
// (32 hex digits, or a dashed UUID) or a git commit.
func commit(ref string) bool {
	if lang.Commit(ref) {
		return true
	}
	s := strings.ReplaceAll(ref, "-", "")
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// resolveImport resolves `import "name";` from file.
//
// Implements: REQ-PROTO-004, REQ-PROTO-006
func (r *resolver) resolveImport(file, name string) lang.Target {
	name = strings.ReplaceAll(name, `\`, "/")
	if name == "" || path.IsAbs(name) {
		return lang.Target{}
	}
	name = path.Clean(name)
	if name == ".." || strings.HasPrefix(name, "../") {
		return lang.Target{}
	}
	sc := r.scopeOf(file)
	roots := sc.roots
	if !sc.buf {
		roots = heuristic(file)
	}
	if p := r.find(roots, name); p != "" {
		return lang.Target{Local: p}
	}
	if wellKnown(name) {
		return lang.Target{Ecosystem: ecoStd, Package: name}
	}
	if d := sc.match(name); d != nil {
		return d.target()
	}
	if sc.buf { // protoc may be pointed elsewhere than Buf is
		if p := r.find(heuristic(file), name); p != "" {
			return lang.Target{Local: p}
		}
	}
	if p := r.bySuffix(file, name); p != "" {
		return lang.Target{Local: p}
	}
	if mods := known(name); mods != nil {
		return lang.Target{Ecosystem: ecoBuf, Package: mods[0], Unresolved: true}
	}
	first, _, _ := strings.Cut(name, "/")
	return lang.Target{Ecosystem: ecoBuf, Package: strings.TrimSuffix(first, ".proto"), Unresolved: true}
}

// heuristic lists the import roots protoc is usually given: the repository root and
// its conventional proto directories, then the importer's directory and each of its
// ancestors, nearest first.
func heuristic(file string) []string {
	roots := append([]string{}, conventional...)
	for d := path.Dir(file); d != "."; d = path.Dir(d) {
		roots = append(roots, d)
	}
	return roots
}

func (r *resolver) find(roots []string, name string) string {
	for _, root := range roots {
		if p := path.Join(root, name); r.files[p] {
			return p
		}
	}
	return ""
}

// match finds the declared module an import comes from: one the table of known protos
// names for its path, else the one whose repository (or, alone with it, owner) is the
// path's first directory, compared without case, dashes, dots or underscores.
func (sc scope) match(name string) *dep {
	for _, m := range known(name) {
		for i := range sc.deps {
			if sc.deps[i].name == m {
				return &sc.deps[i]
			}
		}
	}
	first, _, ok := strings.Cut(name, "/")
	if !ok {
		return nil
	}
	first = fold(first)
	var byRepo, byOwner []*dep
	for i := range sc.deps {
		segs := strings.Split(sc.deps[i].name, "/")
		if len(segs) < 3 {
			continue
		}
		if fold(segs[len(segs)-1]) == first {
			byRepo = append(byRepo, &sc.deps[i])
		}
		if fold(segs[len(segs)-2]) == first {
			byOwner = append(byOwner, &sc.deps[i])
		}
	}
	switch {
	case len(byRepo) > 0:
		return byRepo[0] // the nearest configuration's comes first
	case len(byOwner) == 1:
		return byOwner[0]
	}
	return nil
}

func fold(s string) string {
	return strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(s))
}

// bySuffix finds the project file whose path ends in the import ("foo/bar.proto" in
// third_party/foo/bar.proto): the only one, or else the one closest to the importer
// when that is closer than all the others.
func (r *resolver) bySuffix(file, name string) string {
	var best string
	bestLen, tie := -1, false
	for _, p := range r.byBase[path.Base(name)] {
		if !strings.HasSuffix(p, "/"+name) {
			continue
		}
		n := commonDirs(file, p)
		switch {
		case n > bestLen:
			best, bestLen, tie = p, n, false
		case n == bestLen:
			tie = true
		}
	}
	if tie {
		return ""
	}
	return best
}

// commonDirs counts the leading directories two paths share.
func commonDirs(a, b string) int {
	as, bs := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] && as[n] != "." {
		n++
	}
	return n
}
