package lua

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/luarocks"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

type rockspec struct {
	dir, path string
	spec      *luarocks.Rockspec
	modules   map[string]string // module -> project file that provides it
	lock      map[string]string // the luarocks.lock beside it (or the root's)
	declared  map[string]string // rock -> constraint
	rocks     []string          // declared and locked rocks, sorted
	folded    []string          // the same, folded
}

type wally struct {
	dir  string
	name string
	deps map[string]wallyDep // alias -> dependency
	lock *wallyLock
}

type resolver struct {
	files     map[string]bool
	dirs      map[string]bool
	suffixes  map[string][]string // module path suffix (a/b) -> files providing it
	rockspecs []*rockspec
	wallies   []*wally
	fwd       map[string]string // Rojo instance path (game/A/B) -> project path
	rev       map[string]string // project path -> Rojo instance path
	luarc     []string          // extra module roots
	luaurc    map[string]map[string]string
	// models are the model projects (default.project.json whose tree is not a
	// DataModel) by directory: a $path naming that directory builds its tree.
	models map[string]*rojoNode
	lang.NoteList
}

// The notes a resolver keeps reach --explain only through lang.Noter.
var _ lang.Noter = (*resolver)(nil)

// sourceExts are the extensions of Lua sources a module path may name, in the order
// they are tried.
var sourceExts = []string{".lua", ".luau", ".tl"}

// ignored reports whether a path is inside a tree a package manager installs into:
// LuaRocks' project tree (lua_modules, .luarocks) or Wally's Packages folders.
func ignored(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "lua_modules", ".luarocks", "Packages", "DevPackages", "ServerPackages":
			return true
		}
	}
	return false
}

// Implements: REQ-LUA-004, REQ-LUA-007, REQ-LUA-009, REQ-LUA-010
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, suffixes: map[string][]string{},
		fwd: map[string]string{}, rev: map[string]string{}, luaurc: map[string]map[string]string{},
		models: map[string]*rojoNode{}}
	read := func(rel string) ([]byte, bool) {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return b, err == nil && len(b) <= lang.MaxParseSize
	}
	var rockspecs, wallies, rojos []string
	for _, f := range all {
		p := f.Path
		r.files[p] = true
		for d := path.Dir(p); ; d = path.Dir(d) {
			if r.dirs[d] {
				break
			}
			r.dirs[d] = true
			if d == "." {
				break
			}
		}
		if f.Binary || f.TooLarge || ignored(p) {
			continue
		}
		base := path.Base(p)
		switch {
		case strings.HasSuffix(base, ".rockspec"):
			rockspecs = append(rockspecs, p)
		case base == "wally.toml":
			wallies = append(wallies, p)
		case strings.HasSuffix(base, ".project.json"):
			rojos = append(rojos, p)
		case base == ".luarc.json" || base == ".luarc.jsonc":
			if b, ok := read(p); ok {
				r.luarc = append(r.luarc, readLuarc(b, path.Dir(p))...)
			}
		case base == ".luaurc":
			if b, ok := read(p); ok {
				r.luaurc[path.Dir(p)] = readLuaurc(b)
			}
		}
		if ext := path.Ext(p); ext == ".lua" || ext == ".luau" || ext == ".tl" {
			mod := strings.TrimSuffix(p, ext)
			if path.Base(mod) == "init" {
				mod = path.Dir(mod)
			}
			segments := strings.Split(mod, "/")
			for i := range segments {
				key := strings.Join(segments[i:], "/")
				r.suffixes[key] = append(r.suffixes[key], p)
			}
		}
	}
	locks := map[string]map[string]string{}
	lockOf := func(dir string) map[string]string {
		if l, ok := locks[dir]; ok {
			return l
		}
		var l map[string]string
		if b, ok := read(path.Join(dir, "luarocks.lock")); ok {
			l = luarocks.ReadLock(b)
			// Implements: REQ-LUA-007, REQ-TRC-017
			if lock := path.Join(dir, "luarocks.lock"); len(l) > 0 {
				if !r.files[lock] {
					r.NoteIgnored(lock)
				}
				r.Note(lock, trace.NoteFlat, "luarocks.lock pins versions but records no edges: offline, "+
					"--resolve-depth adds nothing past the rocks it pins (--online asks the rocks servers)")
			}
		}
		locks[dir] = l
		return l
	}
	for _, p := range rockspecs {
		b, ok := read(p)
		if !ok {
			continue
		}
		rs := &rockspec{dir: path.Dir(p), path: p, spec: luarocks.ReadRockspec(b), modules: map[string]string{}, declared: map[string]string{}}
		rs.lock = lockOf(rs.dir)
		if rs.lock == nil {
			rs.lock = lockOf(".")
		}
		for _, d := range rs.spec.Deps {
			if _, ok := rs.declared[d.Name]; !ok {
				rs.declared[d.Name] = d.Constraint
			}
		}
		rs.rocks = sortedKeys(rs.declared, rs.lock)
		for _, rock := range rs.rocks {
			rs.folded = append(rs.folded, fold(rock))
		}
		for _, m := range rs.spec.Modules {
			if f := r.moduleFile(rs.dir, m.File); f != "" {
				rs.modules[m.Name] = f
			}
		}
		r.rockspecs = append(r.rockspecs, rs)
	}
	sort.SliceStable(r.rockspecs, func(i, j int) bool {
		return depth(r.rockspecs[i].dir) < depth(r.rockspecs[j].dir) ||
			depth(r.rockspecs[i].dir) == depth(r.rockspecs[j].dir) && r.rockspecs[i].path < r.rockspecs[j].path
	})
	for _, p := range wallies {
		b, ok := read(p)
		if !ok {
			continue
		}
		w := &wally{dir: path.Dir(p), deps: map[string]wallyDep{}}
		var deps []wallyDep
		w.name, deps = readWally(b)
		for _, d := range deps {
			if _, ok := w.deps[d.alias]; !ok {
				w.deps[d.alias] = d
			}
		}
		if b, ok := read(path.Join(w.dir, "wally.lock")); ok {
			w.lock = readWallyLock(b)
		}
		r.wallies = append(r.wallies, w)
	}
	sort.SliceStable(r.wallies, func(i, j int) bool { return depth(r.wallies[i].dir) < depth(r.wallies[j].dir) })
	// Rojo: place projects (a DataModel tree) say where each $path lands in the
	// game; default.project.json first, then the others by path.
	sort.SliceStable(rojos, func(i, j int) bool {
		di, dj := path.Base(rojos[i]) == "default.project.json", path.Base(rojos[j]) == "default.project.json"
		return di && !dj || di == dj && rojos[i] < rojos[j]
	})
	for _, p := range rojos {
		b, ok := read(p)
		if !ok {
			continue
		}
		tree, ok := readRojo(b)
		if ok && tree.class != "DataModel" && path.Base(p) == "default.project.json" {
			r.models[path.Dir(p)] = tree
		}
		if !ok || tree.class != "DataModel" {
			continue
		}
		var walk func(n *rojoNode, inst string)
		walk = func(n *rojoNode, inst string) {
			if n.path != "" {
				fs := path.Clean(path.Join(path.Dir(p), n.path))
				if _, ok := r.fwd[inst]; !ok {
					r.fwd[inst] = fs
				}
				if _, ok := r.rev[fs]; !ok {
					r.rev[fs] = inst
				}
			}
			for _, c := range n.children {
				walk(c, inst+"/"+c.name)
			}
		}
		walk(tree, "game")
	}
	return r
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// moduleFile finds the project file a rockspec's build.modules entry names: relative
// to the rockspec, else to the project root (a rockspecs/ directory's rockspecs name
// files of the source tree).
func (r *resolver) moduleFile(dir, file string) string {
	for _, p := range []string{path.Join(dir, file), path.Clean(file)} {
		if r.files[p] {
			return p
		}
	}
	return ""
}

// governing are the rockspecs of a file: those in its directory and above, nearest
// first; a file under none is governed by every rockspec, shallowest first.
func (r *resolver) governing(file string) []*rockspec {
	var out []*rockspec
	for i := len(r.rockspecs) - 1; i >= 0; i-- {
		if rs := r.rockspecs[i]; within(file, rs.dir) {
			out = append(out, rs)
		}
	}
	if len(out) == 0 {
		return r.rockspecs
	}
	return out
}

func within(file, dir string) bool { return dir == "." || strings.HasPrefix(file, dir+"/") }

// Resolve maps one import.
//
// Implements: REQ-LUA-004, REQ-LUA-005, REQ-LUA-008, REQ-LUA-009, REQ-LUA-010
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, arg, _ := strings.Cut(imp.Name, ":")
	switch kind {
	case kindRequire:
		return r.require(file, imp.Module)
	case kindPath:
		return r.luauPath(file, imp.Module)
	case kindRoblox:
		return r.roblox(file, strings.Split(imp.Module, "/"))
	case kindFile:
		for _, p := range []string{path.Join(path.Dir(file), imp.Module), path.Clean(imp.Module)} {
			if r.files[p] {
				return lang.Target{Local: p}
			}
		}
	case kindDep:
		var lock map[string]string
		for _, rs := range r.rockspecs {
			if rs.path == file {
				lock = rs.lock
			}
		}
		return rockTarget(imp.Module, arg, lock[imp.Module])
	case kindModule:
		if p := r.moduleFile(path.Dir(file), imp.Module); p != "" {
			return lang.Target{Local: p}
		}
	case kindWally:
		for _, w := range r.wallies {
			if w.dir == path.Dir(file) {
				return wallyTarget(imp.Module, arg, w.lock)
			}
		}
		return wallyTarget(imp.Module, arg, nil)
	case kindRojo:
		p := path.Clean(path.Join(path.Dir(file), imp.Module))
		if r.files[p] || r.dirs[p] {
			return lang.Target{Local: p}
		}
	}
	return lang.Target{}
}

// rockTarget is a rock as a rockspec declares it and a lock pins it. A lock pins; so
// does "== 1.2.3" or a bare version; any other constraint is kept as the version,
// and no constraint floats.
//
// Implements: REQ-LUA-008
func rockTarget(name, constraint, locked string) lang.Target {
	t := lang.Target{Ecosystem: ecoRocks, Package: name}
	switch v, exact := luarocks.Exact(constraint); {
	case locked != "":
		t.Version, t.Pinned = locked, true
		if constraint != "" && v != locked {
			t.Requested = constraint
		}
	case exact:
		t.Version, t.Pinned = v, lang.Pinned(v)
	case constraint != "":
		t.Version = constraint
	default:
		t.Floating = true
	}
	return t
}

// wallyTarget is a Wally package: wally.lock pins; "=1.2.3" pins; a bare version is
// a caret range in Wally and floats like any other requirement.
//
// Implements: REQ-LUA-009
func wallyTarget(pkg, req string, lock *wallyLock) lang.Target {
	t := lang.Target{Ecosystem: ecoWally, Package: pkg}
	if lock != nil && lock.versions[pkg] != "" {
		t.Version, t.Pinned = lock.versions[pkg], true
		if req != "" && strings.TrimPrefix(req, "=") != t.Version {
			t.Requested = req
		}
		return t
	}
	switch {
	case strings.HasPrefix(req, "="):
		t.Version = strings.TrimSpace(strings.TrimPrefix(req, "="))
		t.Pinned = lang.Pinned(t.Version)
	case req != "":
		t.Version = req
	default:
		t.Floating = true
	}
	return t
}

// require resolves a module name as package.path would find it, then as a rock.
//
// Implements: REQ-LUA-004, REQ-LUA-005, REQ-LUA-008
func (r *resolver) require(file, module string) lang.Target {
	m := strings.TrimSpace(module)
	if m == "" {
		return lang.Target{}
	}
	first := m
	if i := strings.IndexAny(m, "./"); i >= 0 {
		first = m[:i]
	}
	gov := r.governing(file)
	if std[first] && !r.declares(gov, strings.ToLower(first)) {
		return lang.Target{Ecosystem: ecoStd, Package: first}
	}
	for _, list := range [][]*rockspec{gov, r.rockspecs} {
		for _, rs := range list {
			if f, ok := rs.modules[m]; ok {
				return lang.Target{Local: f}
			}
		}
	}
	p := strings.ReplaceAll(m, ".", "/")
	if f := r.probeRoots(file, p); f != "" {
		return lang.Target{Local: f}
	}
	for _, c := range candidates(m) {
		if t, ok := r.declared(gov, c, first); ok {
			return t
		}
	}
	if host := runtime(m); host != "" {
		return lang.Target{Ecosystem: ecoRuntime, Package: host}
	}
	if first == "cjson" && r.openresty(gov) {
		return lang.Target{Ecosystem: ecoRuntime, Package: "openresty"} // bundled with it
	}
	if fs := r.suffixes[p]; len(fs) == 1 && !ignored(fs[0]) {
		return lang.Target{Local: fs[0]}
	}
	for _, rs := range gov {
		if strings.EqualFold(rs.spec.Package, first) {
			return lang.Target{} // the rock's own module, not in this checkout
		}
	}
	name := strings.ToLower(first)
	if _, ok := aliases[m]; ok || aliases[first] != nil || first == "resty" {
		name = candidates(m)[0]
	}
	return lang.Target{Ecosystem: ecoRocks, Package: name, Unresolved: true}
}

// probeRoots looks for a module path under the directories package.path usually
// names: each directory from the file's own up to the root, with its lua/, src/ and
// lib/ (a Neovim plugin's modules live in lua/), then .luarc.json's roots.
//
// Implements: REQ-LUA-004, REQ-LUA-011
func (r *resolver) probeRoots(file, p string) string {
	var roots []string
	for d := path.Dir(file); ; d = path.Dir(d) {
		roots = append(roots, d, path.Join(d, "lua"), path.Join(d, "src"), path.Join(d, "lib"))
		if d == "." {
			break
		}
	}
	roots = append(roots, r.luarc...)
	for _, root := range roots {
		if f := r.probe(path.Join(root, p)); f != "" {
			return f
		}
	}
	return ""
}

// probe finds the source a module path names: p.lua, p.luau, p.tl, p/init.*.
func (r *resolver) probe(p string) string {
	if strings.HasPrefix(p, "../") || ignored(p) {
		return ""
	}
	for _, ext := range sourceExts {
		if r.files[p+ext] {
			return p + ext
		}
	}
	for _, ext := range sourceExts {
		if r.files[p+"/init"+ext] {
			return p + "/init" + ext
		}
	}
	return ""
}

// openresty reports whether a project runs on OpenResty, as declaring lua-resty
// rocks says: it then gets lua-cjson from OpenResty, not from a rock.
func (r *resolver) openresty(gov []*rockspec) bool {
	for _, rs := range gov {
		for rock := range rs.declared {
			if strings.HasPrefix(rock, "lua-resty-") {
				return true
			}
		}
	}
	return false
}

// declares reports whether any of the rockspecs declares or locks a rock.
func (r *resolver) declares(gov []*rockspec, name string) bool {
	_, ok := r.declared(gov, name, "")
	return ok
}

// declared finds a rock the governing rockspecs declare or their locks hold, by
// name; with first set also a fork named <owner>-<first> (kong-pgmoon for pgmoon).
func (r *resolver) declared(gov []*rockspec, name, first string) (lang.Target, bool) {
	want, fork := fold(name), "-"+strings.ToLower(first)
	for _, rs := range gov {
		for k, rock := range rs.rocks {
			if rs.folded[k] == want || (first != "" && strings.HasSuffix(rock, fork)) {
				return rockTarget(rock, rs.declared[rock], rs.lock[rock]), true
			}
		}
	}
	return lang.Target{}, false
}

func sortedKeys(maps ...map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range maps {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

// luauPath resolves a Luau require by path: "./x" and "../x" from the requiring
// file's directory (from an init file's parent: the init file stands for its
// directory), "@self/x" from the module itself, "@alias/x" through the nearest
// .luaurc's aliases.
func (r *resolver) luauPath(file, spec string) lang.Target {
	dir := path.Dir(file)
	init := stem(file) == "init"
	var base, rest string
	switch {
	case strings.HasPrefix(spec, "@self"):
		base, rest = path.Join(dir, stem(file)), strings.TrimPrefix(spec, "@self")
		if init {
			base = dir
		}
	case strings.HasPrefix(spec, "@"):
		alias, tail, _ := strings.Cut(spec[1:], "/")
		if strings.EqualFold(alias, "lune") {
			return lang.Target{Ecosystem: ecoRuntime, Package: "lune"}
		}
		target, at, ok := r.alias(dir, strings.ToLower(alias))
		if !ok {
			return r.wallyIn(file, strings.Split(alias+"/"+tail, "/"))
		}
		base, rest = path.Join(at, target), tail
	default:
		base, rest = dir, spec
		if init {
			base = path.Dir(dir)
		}
	}
	p := path.Clean(path.Join(base, rest))
	if r.files[p] {
		return lang.Target{Local: p}
	}
	if f := r.probe(p); f != "" {
		return lang.Target{Local: f}
	}
	return r.wallyIn(file, strings.Split(p, "/"))
}

// alias finds a .luaurc alias in dir or above: its value and the .luaurc's dir.
func (r *resolver) alias(dir, name string) (string, string, bool) {
	for d := dir; ; d = path.Dir(d) {
		if v, ok := r.luaurc[d][name]; ok {
			return v, d, true
		}
		if d == "." {
			return "", "", false
		}
	}
}

// wallyIn resolves an instance or file path through a Wally Packages folder
// (Packages/Roact) to the package the governing wally.toml names so.
func (r *resolver) wallyIn(file string, elems []string) lang.Target {
	for k := 0; k+1 < len(elems); k++ {
		if !packages(elems[k]) {
			continue
		}
		alias := elems[k+1]
		for _, w := range r.wallyOf(file) {
			d, ok := w.deps[alias]
			if !ok {
				for a, dd := range w.deps {
					if strings.EqualFold(a, alias) {
						d, ok = dd, true
					}
				}
			}
			if ok {
				return wallyTarget(d.pkg, d.req, w.lock)
			}
		}
	}
	return lang.Target{}
}

// wallyOf are the wally.toml files governing a file: in its directory and above,
// nearest first; else all of them.
func (r *resolver) wallyOf(file string) []*wally {
	var out []*wally
	for i := len(r.wallies) - 1; i >= 0; i-- {
		if within(file, r.wallies[i].dir) {
			out = append(out, r.wallies[i])
		}
	}
	if len(out) == 0 {
		return r.wallies
	}
	return out
}

// roblox resolves a Roblox instance path (script/../X, game/ReplicatedStorage/X)
// to the file Rojo builds that instance from. A location is a project path, an
// instance path, or both: script is the requiring file (an init file stands for its
// directory), placed in the game when a Rojo place project maps a directory above
// it; .. is the parent instance, else directory; a child is what the project maps
// under the instance, else the child file or directory. A path through a Packages
// folder is a Wally package.
func (r *resolver) roblox(file string, elems []string) lang.Target {
	if t := r.wallyIn(file, elems); t.Package != "" {
		return t
	}
	var fs, inst string
	switch elems[0] {
	case "script":
		fs = file
		if stem(file) == "init" {
			fs = path.Dir(file)
		}
		inst = r.instanceOf(fs)
	case "game":
		inst = "game"
	default:
		return lang.Target{}
	}
	onDisk := func(p string) bool { return r.files[p] || r.dirs[p] }
	mapped := func(i string) string {
		if p, ok := r.fwd[i]; ok && onDisk(p) {
			return p
		}
		return ""
	}
	if inst != "" && fs == "" {
		fs = mapped(inst)
	}
	for _, e := range elems[1:] {
		isDir := fs != "" && r.dirs[fs] && !r.files[fs]
		if e == ".." {
			_, root := r.rev[fs]
			switch {
			case inst != "" && strings.Contains(inst, "/"):
				inst = inst[:strings.LastIndex(inst, "/")]
				if m := mapped(inst); m != "" {
					fs = m
				} else if fs != "" && !root && fs != "." {
					fs = path.Dir(fs)
				} else {
					fs = ""
				}
			case inst == "" && fs != "" && fs != ".":
				fs = path.Dir(fs)
			default:
				return lang.Target{}
			}
			continue
		}
		next := ""
		if inst != "" {
			inst += "/" + e
			next = mapped(inst)
		}
		if next == "" && inst != "" && packages(path.Base(r.fwd[inst[:strings.LastIndex(inst, "/")]])) {
			// A folder Wally installs into, mapped by the project.
			if t := r.wallyIn(file, []string{"Packages", e}); t.Package != "" {
				return t
			}
		}
		if next == "" && isDir {
			if m := r.models[fs]; m != nil {
				fs, isDir = r.model(fs, m, e)
				if isDir || fs != "" {
					continue
				}
			}
			child := path.Join(fs, e)
			switch {
			case r.dirs[child] && !r.files[child]:
				next = child
			case r.files[child+".lua"]:
				next = child + ".lua"
			case r.files[child+".luau"]:
				next = child + ".luau"
			}
		}
		fs = next
		if fs == "" && inst == "" {
			return lang.Target{}
		}
	}
	switch {
	case fs == "":
	case r.files[fs]:
		return lang.Target{Local: fs}
	default:
		for _, ext := range sourceExts {
			if r.files[path.Join(fs, "init"+ext)] {
				return lang.Target{Local: path.Join(fs, "init"+ext)}
			}
		}
	}
	return lang.Target{}
}

// model steps into a child of a model project's root instance: a child the tree
// names, else one of the directory its root $path names.
func (r *resolver) model(dir string, root *rojoNode, e string) (string, bool) {
	for _, c := range root.children {
		if c.name == e && c.path != "" {
			p := path.Clean(path.Join(dir, c.path))
			return p, r.dirs[p] && !r.files[p]
		}
	}
	if root.path != "" {
		p := path.Clean(path.Join(dir, root.path, e))
		switch {
		case r.dirs[p] && !r.files[p]:
			return p, true
		case r.files[p+".lua"]:
			return p + ".lua", false
		case r.files[p+".luau"]:
			return p + ".luau", false
		}
	}
	return "", false
}

func packages(name string) bool {
	return name == "Packages" || name == "DevPackages" || name == "ServerPackages"
}

// stem is a file's base name without its extension.
func stem(p string) string { return strings.TrimSuffix(path.Base(p), path.Ext(p)) }

// instanceOf is the instance path a Rojo place project gives a project path: that of
// the nearest mapped directory above it, followed by the names below (a file named
// without its extension and .server/.client suffix). "" when none maps it.
func (r *resolver) instanceOf(fs string) string {
	var tail []string
	for p := fs; ; p = path.Dir(p) {
		if i, ok := r.rev[p]; ok {
			for k := len(tail) - 1; k >= 0; k-- {
				i += "/" + tail[k]
			}
			return i
		}
		if p == "." {
			return ""
		}
		name := path.Base(p)
		if r.files[p] {
			name = strings.TrimSuffix(strings.TrimSuffix(stem(p), ".server"), ".client")
		}
		tail = append(tail, name)
	}
}

// Dependencies answers --resolve-depth for Wally packages from wally.lock, which
// records every locked package's dependencies. luarocks.lock records none.
//
// Implements: REQ-LUA-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoWally {
		return nil
	}
	for _, w := range r.wallies {
		if w.lock == nil {
			continue
		}
		deps, ok := w.lock.deps[t.Package]
		if !ok {
			continue
		}
		out := []lang.Target{}
		for _, d := range deps {
			name, ver, _ := strings.Cut(d, "@")
			out = append(out, lang.Target{Ecosystem: ecoWally, Package: name, Version: ver, Pinned: ver != ""})
		}
		return out
	}
	return nil
}
