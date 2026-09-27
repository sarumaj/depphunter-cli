package crystal

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a directory with a shard.yml: its shard, the lock and override files
// beside it, and the shards installed into its lib/.
type project struct {
	dir       string
	name      string
	deps      map[string]*dependency // declared, shard.override.yml applied
	lock      map[string]*locked
	installed map[string]*shard // lib/<name>/ -> its shard.yml (empty when it has none)
}

type resolver struct {
	files    map[string]bool
	dirs     map[string]bool
	sources  []string            // every .cr file, sorted, for globs
	projects map[string]*project // by directory
	libC     map[string][]string // a src/ directory with lib_c/ -> its target triples
	order    []*project          // shallowest first
}

// Implements: REQ-CRYSTAL-004, REQ-CRYSTAL-005, REQ-CRYSTAL-006, REQ-CRYSTAL-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, projects: map[string]*project{}}
	abs := map[string]string{}
	for _, f := range all {
		if installed(f) {
			continue
		}
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		if path.Ext(f.Path) == ".cr" {
			r.sources = append(r.sources, f.Path)
		}
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
	sort.Strings(r.sources)
	r.libC = map[string][]string{}
	for d := range r.dirs {
		if lib := path.Dir(d); path.Base(lib) == "lib_c" {
			r.libC[path.Dir(lib)] = append(r.libC[path.Dir(lib)], path.Base(d))
		}
	}
	for _, ts := range r.libC {
		sort.Slice(ts, func(i, j int) bool {
			ri, rj := rank(ts[i]), rank(ts[j])
			if ri != rj {
				return ri < rj
			}
			return ts[i] < ts[j]
		})
	}
	read := func(rel string) ([]byte, bool) {
		if a, ok := abs[rel]; ok {
			data, err := os.ReadFile(a)
			return data, err == nil
		}
		if root == "" {
			return nil, false
		}
		// shard.lock is often ignored by git in libraries, and lib/ always is.
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return data, err == nil
	}
	for rel := range abs {
		if path.Base(rel) != "shard.yml" {
			continue
		}
		data, ok := read(rel)
		if !ok {
			continue
		}
		dir := path.Dir(rel)
		sh := readShard(data)
		p := &project{dir: dir, name: sh.name, deps: sh.deps, lock: map[string]*locked{}, installed: map[string]*shard{}}
		if data, ok := read(path.Join(dir, "shard.lock")); ok {
			p.lock = readLock(data)
		}
		if data, ok := read(path.Join(dir, "shard.override.yml")); ok {
			for name, d := range readOverride(data) {
				if old := p.deps[name]; old != nil {
					d.dev = old.dev
				}
				p.deps[name] = d
			}
		}
		if root != "" {
			p.readInstalled(filepath.Join(root, filepath.FromSlash(dir), "lib"))
		}
		r.projects[dir] = p
		r.order = append(r.order, p)
	}
	sort.Slice(r.order, func(i, j int) bool {
		di, dj := depth(r.order[i].dir), depth(r.order[j].dir)
		if di != dj {
			return di < dj
		}
		return r.order[i].dir < r.order[j].dir
	})
	return r
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// readInstalled lists what shards installed into lib/: each directory (or the
// symlink shards makes for a path dependency) is a shard of that name, with the
// shard.yml it ships.
//
// Implements: REQ-CRYSTAL-008
func (p *project) readInstalled(lib string) {
	entries, err := os.ReadDir(lib)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if info, err := os.Stat(filepath.Join(lib, name)); err != nil || !info.IsDir() {
			continue
		}
		sh := &shard{deps: map[string]*dependency{}}
		if data, err := os.ReadFile(filepath.Join(lib, name, "shard.yml")); err == nil {
			sh = readShard(data)
		}
		p.installed[name] = sh
	}
}

// projectOf is the nearest project at or above dir.
func (r *resolver) projectOf(dir string) *project {
	for {
		if p := r.projects[dir]; p != nil {
			return p
		}
		if dir == "." || dir == "/" || dir == "" {
			return nil
		}
		dir = path.Dir(dir)
	}
}

// scope is the projects whose shards a file can require: its own, or, for a file
// no shard.yml governs, every project, shallowest first.
func (r *resolver) scope(file string) []*project {
	if p := r.projectOf(path.Dir(file)); p != nil {
		return []*project{p}
	}
	return r.order
}

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindRequire:
		return r.require(file, imp.Module)
	case kindMain:
		if p := path.Join(path.Dir(file), imp.Module); r.files[p] {
			return lang.Target{Local: p}
		}
		return lang.Target{}
	case kindDep, kindDevDep, kindLocked, kindOverride:
		if p := r.projects[path.Dir(file)]; p != nil {
			return r.shardTarget(p, imp.Module)
		}
		return lang.Target{Ecosystem: ecoShards, Package: imp.Module}
	}
	return lang.Target{}
}

// relative reports whether a require names a file relative to the requiring one.
func relative(spec string) bool {
	return strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")
}

// glob splits `x/*` and `x/**` into the directory and whether it is recursive.
func glob(spec string) (dir string, recursive, ok bool) {
	if d, ok := strings.CutSuffix(spec, "/**"); ok {
		return d, true, true
	}
	if d, ok := strings.CutSuffix(spec, "/*"); ok {
		return d, false, true
	}
	return "", false, false
}

// probe finds the file a require names under base: x.cr, then x/x.cr (the
// compiler's rule for a directory). base "" is the repository root.
func (r *resolver) probe(base, spec string) string {
	p := path.Join(base, spec)
	if strings.HasPrefix(p, "../") || p == ".." {
		return ""
	}
	if strings.HasSuffix(p, ".cr") {
		if r.files[p] {
			return p
		}
		return ""
	}
	if r.files[p+".cr"] {
		return p + ".cr"
	}
	if f := path.Join(p, path.Base(p)+".cr"); r.files[f] {
		return f
	}
	return ""
}

// require resolves `require "spec"` from file.
//
// Implements: REQ-CRYSTAL-004, REQ-CRYSTAL-007
func (r *resolver) require(file, spec string) lang.Target {
	if relative(spec) {
		if f := r.probe(path.Dir(file), spec); f != "" && f != file {
			return lang.Target{Local: f}
		}
		return lang.Target{}
	}
	if strings.HasPrefix(spec, "/") {
		return lang.Target{}
	}
	first, _, _ := strings.Cut(spec, "/")
	scope := r.scope(file)
	// lib/ first: the directory shards installed is the shard of that name.
	for _, p := range r.libScope(file) {
		if _, ok := p.installed[first]; ok {
			return r.refine(r.shardTarget(p, first), spec)
		}
	}
	// The project itself: a shard requiring itself by name (its bin/ templates
	// and specs do), or Crystal's standard library requiring itself from src/.
	if p := r.projectOf(path.Dir(file)); p != nil && p.name != "" && fold(p.name) == fold(first) {
		if f := r.inShard(p.dir, spec); f != "" {
			return lang.Target{Local: f}
		}
	}
	for _, base := range r.srcRoots(file) {
		if f := r.probe(base, spec); f != "" {
			return lang.Target{Local: f}
		}
	}
	if first == "c" {
		return r.requireC(file, spec)
	}
	for _, p := range scope {
		if p.deps[first] != nil || p.lock[first] != nil {
			return r.refine(r.shardTarget(p, first), spec)
		}
	}
	if stdlib[first] {
		return lang.Target{Ecosystem: ecoStd, Package: first}
	}
	for _, p := range scope {
		if name := p.match(first); name != "" {
			return r.refine(r.shardTarget(p, name), spec)
		}
	}
	for _, p := range scope {
		if p.name != "" && fold(p.name) == fold(first) {
			return lang.Target{} // its own file, missing
		}
	}
	return lang.Target{Ecosystem: ecoShards, Package: first, Unresolved: true}
}

// triples are the targets whose src/lib_c/<triple>/ answers `require "c/x"` in
// the standard library's own sources, in order: the compiler adds the directory
// of the target it builds for to CRYSTAL_PATH.
var triples = []string{"x86_64-linux-gnu", "aarch64-linux-gnu", "x86_64-darwin", "aarch64-darwin", "x86_64-windows-msvc"}

func rank(triple string) int {
	for i, t := range triples {
		if t == triple {
			return i
		}
	}
	return len(triples)
}

// requireC resolves `require "c/x"`: the C bindings of the standard library, in
// src/lib_c/<target>/ of Crystal's own repository (the first target, in the
// order of triples, that has the file), else the standard library.
func (r *resolver) requireC(file, spec string) lang.Target {
	for _, root := range r.srcRoots(file) {
		for _, t := range r.libC[root] {
			if f := r.probe(path.Join(root, "lib_c", t), spec); f != "" {
				return lang.Target{Local: f}
			}
		}
		// Bindings shared by the targets of one OS: c/linux/x is lib_c/linux/x.cr.
		if f := r.probe(path.Join(root, "lib_c"), strings.TrimPrefix(spec, "c/")); f != "" {
			return lang.Target{Local: f}
		}
	}
	return lang.Target{Ecosystem: ecoStd, Package: "lib_c"}
}

// refine turns a path dependency's directory into the file of it a require
// names.
func (r *resolver) refine(t lang.Target, spec string) lang.Target {
	if t.Local == "" {
		return t
	}
	if f := r.inShard(t.Local, spec); f != "" {
		return lang.Target{Local: f}
	}
	return t
}

// inShard finds the file a require names in a shard whose root is dir, by the
// compiler's rules for a shard in lib/: "x" is src/x.cr (or x.cr), and "x/y" is
// y.cr at the shard's root, src/y.cr or src/x/y.cr.
func (r *resolver) inShard(dir, spec string) string {
	first, rest, ok := strings.Cut(spec, "/")
	if !ok {
		for _, f := range []string{path.Join(dir, "src", first+".cr"), path.Join(dir, first+".cr")} {
			if r.files[f] {
				return f
			}
		}
		return ""
	}
	for _, base := range []string{dir, path.Join(dir, "src")} {
		if f := r.probe(base, rest); f != "" {
			return f
		}
	}
	return r.probe(path.Join(dir, "src"), spec)
}

// libScope is the projects whose lib/ the compiler would read for file: its own
// and the repository's root project (the directory shards install runs in).
func (r *resolver) libScope(file string) []*project {
	var out []*project
	if p := r.projectOf(path.Dir(file)); p != nil {
		out = append(out, p)
	}
	if p := r.projects["."]; p != nil && (len(out) == 0 || out[0] != p) {
		out = append(out, p)
	}
	return out
}

// srcRoots are the src/ directories a require by name may find the project's own
// files in: the file's project's, else the repository's.
func (r *resolver) srcRoots(file string) []string {
	if p := r.projectOf(path.Dir(file)); p != nil {
		return []string{path.Join(p.dir, "src")}
	}
	return []string{"src"}
}

// match is the shard of the project a require's first segment names once
// spelling is folded, or "".
func (p *project) match(first string) string {
	want := fold(first)
	for _, names := range [][]string{sortedKeys(p.deps), sortedKeys(p.lock)} {
		for _, name := range names {
			if fold(name) == want {
				return name
			}
		}
	}
	return ""
}

// shardTarget is the shard name as project p depends on it.
//
// Implements: REQ-CRYSTAL-006
func (r *resolver) shardTarget(p *project, name string) lang.Target {
	d, l := p.deps[name], p.lock[name]
	local := ""
	if d != nil && d.path != "" {
		local = d.path
	} else if d == nil && l != nil && l.path != "" {
		local = l.path
	}
	if local != "" {
		if dir := path.Join(p.dir, local); r.dirs[dir] {
			return lang.Target{Local: dir}
		}
		return lang.Target{}
	}
	t := lang.Target{Ecosystem: ecoShards, Package: name}
	url := ""
	if d != nil {
		url = d.url
	}
	if l != nil && l.url != "" && url == "" {
		url = l.url
	}
	if !public(url) {
		t.Origin = url
	}
	switch {
	case l != nil && l.version != "":
		t.Version, t.Pinned = l.version, true
		if d != nil && d.requirement() != l.version {
			t.Requested = d.requirement()
		}
	case d != nil:
		pinRule(&t, d)
	default:
		if _, ok := p.installed[name]; !ok {
			t.Unresolved = true
		}
	}
	return t
}

// pinRule is shards' pinning without a lock: a commit pins; a tag or an exact
// version names one release, but a tag can be moved, so it is shown, neither
// pinned nor floating; a branch, a range and no requirement at all float.
//
// Implements: REQ-CRYSTAL-006
func pinRule(t *lang.Target, d *dependency) {
	switch {
	case d.commit != "":
		t.Version, t.Pinned = d.commit, true
	case d.tag != "":
		t.Version = d.tag
	case d.branch != "":
		t.Version, t.Floating = d.branch, true
	case d.version != "" && lang.Pinned(d.version):
		t.Version = d.version
	default:
		t.Version, t.Floating = d.version, true
	}
}

// public reports whether a shard's repository is on a public forge (or unknown),
// as opposed to a git server of the organization's own, which is recorded as the
// shard's origin.
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

// Expand turns `require "./dir/*"` (the .cr files of dir) and
// `require "./dir/**"` (of dir and its subdirectories) into one import per file,
// in path order. A glob in the project's own src/ is expanded the same way; one
// into an installed or declared shard is an import of the shard.
//
// Implements: REQ-CRYSTAL-004
func (r *resolver) Expand(file string, imp lang.RawImport) ([]lang.Import, bool) {
	if imp.Name != kindRequire {
		return nil, false
	}
	dir, recursive, ok := glob(imp.Module)
	if !ok {
		return nil, false
	}
	var base string
	switch {
	case relative(imp.Module):
		base = path.Join(path.Dir(file), dir)
	case strings.HasPrefix(imp.Module, "/"):
		return []lang.Import{{Spec: imp.Spec, Line: imp.Line}}, true
	default:
		first, _, _ := strings.Cut(dir, "/")
		for _, p := range r.libScope(file) {
			if _, ok := p.installed[first]; ok {
				return []lang.Import{{Spec: imp.Spec, Line: imp.Line, Target: r.shardTarget(p, first)}}, true
			}
		}
		for _, root := range r.globRoots(file, dir) {
			if r.dirs[root] {
				base = root
				break
			}
		}
		if base == "" {
			return []lang.Import{{Spec: imp.Spec, Line: imp.Line, Target: r.require(file, imp.Module)}}, true
		}
	}
	prefix := strings.TrimSuffix(imp.Module, "*")
	prefix = strings.TrimSuffix(prefix, "*")
	var out []lang.Import
	lo := sort.SearchStrings(r.sources, base+"/")
	for _, f := range r.sources[lo:] {
		rest, ok := strings.CutPrefix(f, base+"/")
		if !ok {
			break
		}
		if f == file || !recursive && strings.Contains(rest, "/") {
			continue
		}
		out = append(out, lang.Import{Spec: prefix + rest, Line: imp.Line, Target: lang.Target{Local: f}})
		if len(out) >= maxGlob {
			break
		}
	}
	if len(out) == 0 {
		return []lang.Import{{Spec: imp.Spec, Line: imp.Line}}, true
	}
	return out, true
}

// globRoots are the directories a glob by name may list in the project itself:
// its own shard's (as inShard reads a require) and its src/ directory's.
func (r *resolver) globRoots(file, dir string) []string {
	var out []string
	first, rest, _ := strings.Cut(dir, "/")
	if p := r.projectOf(path.Dir(file)); p != nil && p.name != "" && fold(p.name) == fold(first) && rest != "" {
		out = append(out, path.Join(p.dir, rest), path.Join(p.dir, "src", rest))
	}
	for _, root := range r.srcRoots(file) {
		out = append(out, path.Join(root, dir))
	}
	return out
}

// maxGlob bounds the files one glob links.
const maxGlob = 5000

// Dependencies lists what a shard installed into lib/ depends on, from the
// shard.yml it ships: shard.lock is flat, so without lib/ nothing is known
// offline. Each is pinned as the installing project's lock pins it.
//
// Implements: REQ-CRYSTAL-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoShards {
		return nil
	}
	for _, p := range r.order {
		sh, ok := p.installed[t.Package]
		if !ok {
			continue
		}
		var out []lang.Target
		for _, name := range sortedKeys(sh.deps) {
			d := sh.deps[name]
			if d.dev {
				continue
			}
			if l := p.lock[name]; l != nil || p.deps[name] != nil {
				if dt := r.shardTarget(p, name); dt.Ecosystem != "" {
					out = append(out, dt)
				}
				continue
			}
			dt := lang.Target{Ecosystem: ecoShards, Package: name}
			if d.path == "" {
				pinRule(&dt, d)
				if !public(d.url) {
					dt.Origin = d.url
				}
				out = append(out, dt)
			}
		}
		return out
	}
	return nil
}

// Installed reports whether a shard's dependencies come from what shards
// installed into lib/.
func (r *resolver) Installed(t lang.Target) bool {
	for _, p := range r.order {
		if _, ok := p.installed[t.Package]; ok && t.Ecosystem == ecoShards {
			return true
		}
	}
	return false
}
