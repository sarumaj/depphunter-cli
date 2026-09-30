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
	directory, path string
	spec            *luarocks.Rockspec
	modules         map[string]string // module -> project file that provides it
	lock            map[string]string // the luarocks.lock beside it (or the root's)
	declared        map[string]string // rock -> constraint
	rocks           []string          // declared and locked rocks, sorted
	folded          []string          // the same, folded
}

type wally struct {
	directory    string
	name         string
	dependencies map[string]wallyDependency // alias -> dependency
	lock         *wallyLock
}

type resolver struct {
	files       map[string]bool
	directories map[string]bool
	suffixes    map[string][]string // module path suffix (a/b) -> files providing it
	rockspecs   []*rockspec
	wallies     []*wally
	forward     map[string]string // Rojo instance path (game/A/B) -> project path
	rev         map[string]string // project path -> Rojo instance path
	luarc       []string          // extra module roots
	luaurc      map[string]map[string]string
	// models are the model projects (default.project.json whose tree is not a
	// DataModel) by directory: a $path naming that directory builds its tree.
	models map[string]*rojoNode
	// trees are the LuaRocks trees installed in the repository (lua_modules/,
	// .luarocks/), the root's first, each by rock name.
	trees []map[string]*installedRock
	lang.NoteList
}

// installedRock is a rock installed in a tree: its version and the rockspec
// LuaRocks keeps beside it.
type installedRock struct {
	version string
	spec    *luarocks.Rockspec
}

// The notes a resolver keeps reach --explain only through lang.Noter.
var _ lang.Noter = (*resolver)(nil)

// sourceExtensions are the extensions of Lua sources a module path may name, in the order
// they are tried.
var sourceExtensions = []string{".lua", ".luau", ".tl"}

// ignored reports whether a path is inside a tree a package manager installs into:
// LuaRocks' project tree (lua_modules, .luarocks) or Wally's Packages folders.
func ignored(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		switch segment {
		case "lua_modules", ".luarocks", "Packages", "DevPackages", "ServerPackages":
			return true
		}
	}
	return false
}

// Implements: REQ-LUA-004, REQ-LUA-007, REQ-LUA-009, REQ-LUA-010
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, directories: map[string]bool{}, suffixes: map[string][]string{},
		forward: map[string]string{}, rev: map[string]string{}, luaurc: map[string]map[string]string{},
		models: map[string]*rojoNode{}}
	read := func(relative string) ([]byte, bool) {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		return b, err == nil && len(b) <= lang.MaxParseSize
	}
	var rockspecs, wallies, rojos []string
	for _, f := range all {
		p := f.Path
		r.files[p] = true
		for d := path.Dir(p); ; d = path.Dir(d) {
			if r.directories[d] {
				break
			}
			r.directories[d] = true
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
		if extension := path.Ext(p); extension == ".lua" || extension == ".luau" || extension == ".tl" {
			module := strings.TrimSuffix(p, extension)
			if path.Base(module) == "init" {
				module = path.Dir(module)
			}
			segments := strings.Split(module, "/")
			for i := range segments {
				key := strings.Join(segments[i:], "/")
				r.suffixes[key] = append(r.suffixes[key], p)
			}
		}
	}
	locks := map[string]map[string]string{}
	lockOf := func(directory string) map[string]string {
		if l, ok := locks[directory]; ok {
			return l
		}
		var l map[string]string
		if b, ok := read(path.Join(directory, "luarocks.lock")); ok {
			l = luarocks.ReadLock(b)
			// Implements: REQ-LUA-007, REQ-TRC-017
			if lock := path.Join(directory, "luarocks.lock"); len(l) > 0 {
				if !r.files[lock] {
					r.NoteIgnored(lock)
				}
				r.Note(lock, trace.NoteFlat, "luarocks.lock pins versions but records no edges: offline, "+
					"--resolve-depth follows only rocks installed in lua_modules/ or .luarocks/ (--online asks the rocks servers)")
			}
		}
		locks[directory] = l
		return l
	}
	for _, p := range rockspecs {
		b, ok := read(p)
		if !ok {
			continue
		}
		newRockspec := &rockspec{directory: path.Dir(p), path: p, spec: luarocks.ReadRockspec(b), modules: map[string]string{}, declared: map[string]string{}}
		newRockspec.lock = lockOf(newRockspec.directory)
		if newRockspec.lock == nil {
			newRockspec.lock = lockOf(".")
		}
		for _, d := range newRockspec.spec.Dependencies {
			if _, ok := newRockspec.declared[d.Name]; !ok {
				newRockspec.declared[d.Name] = d.Constraint
			}
		}
		newRockspec.rocks = sortedKeys(newRockspec.declared, newRockspec.lock)
		for _, rock := range newRockspec.rocks {
			newRockspec.folded = append(newRockspec.folded, lang.FoldSeparators(rock))
		}
		for _, m := range newRockspec.spec.Modules {
			if f := r.moduleFile(newRockspec.directory, m.File); f != "" {
				newRockspec.modules[m.Name] = f
			}
		}
		r.rockspecs = append(r.rockspecs, newRockspec)
	}
	sort.SliceStable(r.rockspecs, func(i, j int) bool {
		return lang.Depth(r.rockspecs[i].directory) < lang.Depth(r.rockspecs[j].directory) ||
			lang.Depth(r.rockspecs[i].directory) == lang.Depth(r.rockspecs[j].directory) && r.rockspecs[i].path < r.rockspecs[j].path
	})
	seen, directories := map[string]bool{}, []string{"."}
	for _, rockspec := range r.rockspecs {
		directories = append(directories, rockspec.directory)
	}
	for _, directory := range directories {
		for _, tree := range []string{"lua_modules", ".luarocks"} {
			if t := path.Join(directory, tree); !seen[t] {
				seen[t] = true
				if rocks := readTree(filepath.Join(root, filepath.FromSlash(t))); len(rocks) > 0 {
					r.trees = append(r.trees, rocks)
				}
			}
		}
	}
	for _, p := range wallies {
		b, ok := read(p)
		if !ok {
			continue
		}
		w := &wally{directory: path.Dir(p), dependencies: map[string]wallyDependency{}}
		var dependencies []wallyDependency
		w.name, dependencies = readWally(b)
		for _, d := range dependencies {
			if _, ok := w.dependencies[d.alias]; !ok {
				w.dependencies[d.alias] = d
			}
		}
		if b, ok := read(path.Join(w.directory, "wally.lock")); ok {
			w.lock = readWallyLock(b)
		}
		r.wallies = append(r.wallies, w)
	}
	sort.SliceStable(r.wallies, func(i, j int) bool { return lang.Depth(r.wallies[i].directory) < lang.Depth(r.wallies[j].directory) })
	// Rojo: place projects (a DataModel tree) say where each $path lands in the
	// game; default.project.json first, then the others by path.
	sort.SliceStable(rojos, func(i, j int) bool {
		defaultI, defaultJ := path.Base(rojos[i]) == "default.project.json", path.Base(rojos[j]) == "default.project.json"
		return defaultI && !defaultJ || defaultI == defaultJ && rojos[i] < rojos[j]
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
		var walk func(n *rojoNode, instance string)
		walk = func(n *rojoNode, instance string) {
			if n.path != "" {
				filePath := path.Clean(path.Join(path.Dir(p), n.path))
				if _, ok := r.forward[instance]; !ok {
					r.forward[instance] = filePath
				}
				if _, ok := r.rev[filePath]; !ok {
					r.rev[filePath] = instance
				}
			}
			for _, c := range n.children {
				walk(c, instance+"/"+c.name)
			}
		}
		walk(tree, "game")
	}
	return r
}

// readTree reads the rocks installed in a LuaRocks tree:
// lib/luarocks/rocks-<lua version>/<rock>/<version>/<rock>-<version>.rockspec.
// Of two versions of a rock the newer is kept.
//
// Implements: REQ-LUA-013
func readTree(tree string) map[string]*installedRock {
	out := map[string]*installedRock{}
	base := filepath.Join(tree, "lib", "luarocks")
	luas, _ := os.ReadDir(base)
	for _, l := range luas {
		if !l.IsDir() || !strings.HasPrefix(l.Name(), "rocks-") {
			continue
		}
		rocks, _ := os.ReadDir(filepath.Join(base, l.Name()))
		for _, rock := range rocks {
			versions, _ := os.ReadDir(filepath.Join(base, l.Name(), rock.Name()))
			for _, v := range versions {
				name, version := rock.Name(), v.Name()
				if old := out[name]; old != nil && luarocks.Compare(old.version, version) >= 0 {
					continue
				}
				spec := filepath.Join(base, l.Name(), name, version, name+"-"+version+".rockspec")
				if b, err := os.ReadFile(spec); err == nil && len(b) <= lang.MaxParseSize {
					out[name] = &installedRock{version: version, spec: luarocks.ReadRockspec(b)}
				}
			}
		}
	}
	return out
}

// moduleFile finds the project file a rockspec's build.modules entry names: relative
// to the rockspec, else to the project root (a rockspecs/ directory's rockspecs name
// files of the source tree).
func (r *resolver) moduleFile(directory, file string) string {
	for _, p := range []string{path.Join(directory, file), path.Clean(file)} {
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
		if rockspec := r.rockspecs[i]; lang.Within(file, rockspec.directory) {
			out = append(out, rockspec)
		}
	}
	if len(out) == 0 {
		return r.rockspecs
	}
	return out
}

// Resolve maps one import.
//
// Implements: REQ-LUA-004, REQ-LUA-005, REQ-LUA-008, REQ-LUA-009, REQ-LUA-010
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, argument, _ := strings.Cut(rawImport.Name, ":")
	switch kind {
	case kindRequire:
		return r.require(file, rawImport.Module)
	case kindPath:
		return r.luauPath(file, rawImport.Module)
	case kindRoblox:
		return r.roblox(file, strings.Split(rawImport.Module, "/"))
	case kindFile:
		for _, p := range []string{path.Join(path.Dir(file), rawImport.Module), path.Clean(rawImport.Module)} {
			if r.files[p] {
				return lang.Target{Local: p}
			}
		}
	case kindDependency:
		var lock map[string]string
		for _, rockspec := range r.rockspecs {
			if rockspec.path == file {
				lock = rockspec.lock
			}
		}
		return rockTarget(rawImport.Module, argument, lock[rawImport.Module])
	case kindModule:
		if p := r.moduleFile(path.Dir(file), rawImport.Module); p != "" {
			return lang.Target{Local: p}
		}
	case kindWally:
		for _, w := range r.wallies {
			if w.directory == path.Dir(file) {
				return wallyTarget(rawImport.Module, argument, w.lock)
			}
		}
		return wallyTarget(rawImport.Module, argument, nil)
	case kindRojo:
		p := path.Clean(path.Join(path.Dir(file), rawImport.Module))
		if r.files[p] || r.directories[p] {
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
	t := lang.Target{Ecosystem: ecosystemRocks, Package: name}
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
func wallyTarget(packageName, requirement string, lock *wallyLock) lang.Target {
	t := lang.Target{Ecosystem: ecosystemWally, Package: packageName}
	if lock != nil && lock.versions[packageName] != "" {
		t.Version, t.Pinned = lock.versions[packageName], true
		if requirement != "" && strings.TrimPrefix(requirement, "=") != t.Version {
			t.Requested = requirement
		}
		return t
	}
	switch {
	case strings.HasPrefix(requirement, "="):
		t.Version = strings.TrimSpace(strings.TrimPrefix(requirement, "="))
		t.Pinned = lang.Pinned(t.Version)
	case requirement != "":
		t.Version = requirement
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
	governing := r.governing(file)
	if std[first] && !r.declares(governing, strings.ToLower(first)) {
		return lang.Target{Ecosystem: ecosystemStd, Package: first}
	}
	for _, list := range [][]*rockspec{governing, r.rockspecs} {
		for _, rockspec := range list {
			if f, ok := rockspec.modules[m]; ok {
				return lang.Target{Local: f}
			}
		}
	}
	p := strings.ReplaceAll(m, ".", "/")
	if f := r.probeRoots(file, p); f != "" {
		return lang.Target{Local: f}
	}
	for _, c := range candidates(m) {
		if t, ok := r.declared(governing, c, first); ok {
			return t
		}
	}
	if host := runtime(m); host != "" {
		return lang.Target{Ecosystem: ecosystemRuntime, Package: host}
	}
	if first == "cjson" && r.openresty(governing) {
		return lang.Target{Ecosystem: ecosystemRuntime, Package: "openresty"} // bundled with it
	}
	if files := r.suffixes[p]; len(files) == 1 && !ignored(files[0]) {
		return lang.Target{Local: files[0]}
	}
	for _, rockspec := range governing {
		if strings.EqualFold(rockspec.spec.Package, first) {
			return lang.Target{} // the rock's own module, not in this checkout
		}
	}
	name := strings.ToLower(first)
	if _, ok := aliases[m]; ok || aliases[first] != nil || first == "resty" {
		name = candidates(m)[0]
	}
	return lang.Target{Ecosystem: ecosystemRocks, Package: name, Unresolved: true}
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
	for _, extension := range sourceExtensions {
		if r.files[p+extension] {
			return p + extension
		}
	}
	for _, extension := range sourceExtensions {
		if r.files[p+"/init"+extension] {
			return p + "/init" + extension
		}
	}
	return ""
}

// openresty reports whether a project runs on OpenResty, as declaring lua-resty
// rocks says: it then gets lua-cjson from OpenResty, not from a rock.
func (r *resolver) openresty(governing []*rockspec) bool {
	for _, rockspec := range governing {
		for rock := range rockspec.declared {
			if strings.HasPrefix(rock, "lua-resty-") {
				return true
			}
		}
	}
	return false
}

// declares reports whether any of the rockspecs declares or locks a rock.
func (r *resolver) declares(governing []*rockspec, name string) bool {
	_, ok := r.declared(governing, name, "")
	return ok
}

// declared finds a rock the governing rockspecs declare or their locks hold, by
// name; with first set also a fork named <owner>-<first> (kong-pgmoon for pgmoon).
func (r *resolver) declared(governing []*rockspec, name, first string) (lang.Target, bool) {
	want, fork := lang.FoldSeparators(name), "-"+strings.ToLower(first)
	for _, rockspec := range governing {
		for k, rock := range rockspec.rocks {
			if rockspec.folded[k] == want || (first != "" && strings.HasSuffix(rock, fork)) {
				return rockTarget(rock, rockspec.declared[rock], rockspec.lock[rock]), true
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
	directory := path.Dir(file)
	init := stem(file) == "init"
	var base, rest string
	switch {
	case strings.HasPrefix(spec, "@self"):
		base, rest = path.Join(directory, stem(file)), strings.TrimPrefix(spec, "@self")
		if init {
			base = directory
		}
	case strings.HasPrefix(spec, "@"):
		alias, tail, _ := strings.Cut(spec[1:], "/")
		if strings.EqualFold(alias, "lune") {
			return lang.Target{Ecosystem: ecosystemRuntime, Package: "lune"}
		}
		target, at, ok := r.alias(directory, strings.ToLower(alias))
		if !ok {
			return r.wallyIn(file, strings.Split(alias+"/"+tail, "/"))
		}
		base, rest = path.Join(at, target), tail
	default:
		base, rest = directory, spec
		if init {
			base = path.Dir(directory)
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

// alias finds a .luaurc alias in directory or above: its value and the .luaurc's directory.
func (r *resolver) alias(directory, name string) (string, string, bool) {
	for d := directory; ; d = path.Dir(d) {
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
func (r *resolver) wallyIn(file string, elements []string) lang.Target {
	for k := 0; k+1 < len(elements); k++ {
		if !packages(elements[k]) {
			continue
		}
		alias := elements[k+1]
		for _, w := range r.wallyOf(file) {
			d, ok := w.dependencies[alias]
			if !ok {
				for a, dd := range w.dependencies {
					if strings.EqualFold(a, alias) {
						d, ok = dd, true
					}
				}
			}
			if ok {
				return wallyTarget(d.packageName, d.requirement, w.lock)
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
		if lang.Within(file, r.wallies[i].directory) {
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
func (r *resolver) roblox(file string, elements []string) lang.Target {
	if t := r.wallyIn(file, elements); t.Package != "" {
		return t
	}
	var filePath, instance string
	switch elements[0] {
	case "script":
		filePath = file
		if stem(file) == "init" {
			filePath = path.Dir(file)
		}
		instance = r.instanceOf(filePath)
	case "game":
		instance = "game"
	default:
		return lang.Target{}
	}
	onDisk := func(p string) bool { return r.files[p] || r.directories[p] }
	mapped := func(i string) string {
		if p, ok := r.forward[i]; ok && onDisk(p) {
			return p
		}
		return ""
	}
	if instance != "" && filePath == "" {
		filePath = mapped(instance)
	}
	for _, e := range elements[1:] {
		isDirectory := filePath != "" && r.directories[filePath] && !r.files[filePath]
		if e == ".." {
			_, root := r.rev[filePath]
			switch {
			case instance != "" && strings.Contains(instance, "/"):
				instance = instance[:strings.LastIndex(instance, "/")]
				if m := mapped(instance); m != "" {
					filePath = m
				} else if filePath != "" && !root && filePath != "." {
					filePath = path.Dir(filePath)
				} else {
					filePath = ""
				}
			case instance == "" && filePath != "" && filePath != ".":
				filePath = path.Dir(filePath)
			default:
				return lang.Target{}
			}
			continue
		}
		next := ""
		if instance != "" {
			instance += "/" + e
			next = mapped(instance)
		}
		if next == "" && instance != "" && packages(path.Base(r.forward[instance[:strings.LastIndex(instance, "/")]])) {
			// A folder Wally installs into, mapped by the project.
			if t := r.wallyIn(file, []string{"Packages", e}); t.Package != "" {
				return t
			}
		}
		if next == "" && isDirectory {
			if m := r.models[filePath]; m != nil {
				filePath, isDirectory = r.model(filePath, m, e)
				if isDirectory || filePath != "" {
					continue
				}
			}
			child := path.Join(filePath, e)
			switch {
			case r.directories[child] && !r.files[child]:
				next = child
			case r.files[child+".lua"]:
				next = child + ".lua"
			case r.files[child+".luau"]:
				next = child + ".luau"
			}
		}
		filePath = next
		if filePath == "" && instance == "" {
			return lang.Target{}
		}
	}
	switch {
	case filePath == "":
	case r.files[filePath]:
		return lang.Target{Local: filePath}
	default:
		for _, extension := range sourceExtensions {
			if r.files[path.Join(filePath, "init"+extension)] {
				return lang.Target{Local: path.Join(filePath, "init"+extension)}
			}
		}
	}
	return lang.Target{}
}

// model steps into a child of a model project's root instance: a child the tree
// names, else one of the directory its root $path names.
func (r *resolver) model(directory string, root *rojoNode, e string) (string, bool) {
	for _, c := range root.children {
		if c.name == e && c.path != "" {
			p := path.Clean(path.Join(directory, c.path))
			return p, r.directories[p] && !r.files[p]
		}
	}
	if root.path != "" {
		p := path.Clean(path.Join(directory, root.path, e))
		switch {
		case r.directories[p] && !r.files[p]:
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
func (r *resolver) instanceOf(filePath string) string {
	var tail []string
	for p := filePath; ; p = path.Dir(p) {
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
// records every locked package's dependencies, and for rocks from what a LuaRocks
// tree of the repository has installed. luarocks.lock records none.
//
// Implements: REQ-LUA-009, REQ-LUA-013
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem == ecosystemRocks {
		return r.installedDependencies(t)
	}
	if t.Ecosystem != ecosystemWally {
		return nil
	}
	for _, w := range r.wallies {
		if w.lock == nil {
			continue
		}
		dependencies, ok := w.lock.dependencies[t.Package]
		if !ok {
			continue
		}
		out := []lang.Target{}
		for _, d := range dependencies {
			name, version, _ := strings.Cut(d, "@")
			out = append(out, lang.Target{Ecosystem: ecosystemWally, Package: name, Version: version, Pinned: version != ""})
		}
		return out
	}
	return nil
}

// installedDependencies is what the rockspec of an installed rock requires (its
// dependencies, not its build or test dependencies, nor Lua itself), from the
// first tree holding the rock; each is pinned to the version that tree holds.
//
// Implements: REQ-LUA-013
func (r *resolver) installedDependencies(t lang.Target) []lang.Target {
	for _, tree := range r.trees {
		rock := tree[t.Package]
		if rock == nil {
			continue
		}
		out := []lang.Target{}
		seen := map[string]bool{}
		for _, d := range rock.spec.Dependencies {
			if d.Section != "dependencies" || d.Name == "lua" || seen[d.Name] {
				continue
			}
			seen[d.Name] = true
			locked := ""
			if dependency := tree[d.Name]; dependency != nil {
				locked = dependency.version
			}
			out = append(out, rockTarget(d.Name, d.Constraint, locked))
		}
		return out
	}
	return nil
}

// Installed reports whether a rock's dependencies come from a LuaRocks tree.
func (r *resolver) Installed(t lang.Target) bool {
	if t.Ecosystem != ecosystemRocks {
		return false
	}
	for _, tree := range r.trees {
		if tree[t.Package] != nil {
			return true
		}
	}
	return false
}
