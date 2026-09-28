package nim

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a directory with a .nimble file: the package, its requirements,
// its lock files and what is installed for it.
type project struct {
	dir     string
	nimble  *nimbleFile
	deps    []dep
	lock    []*locked // nimble.lock
	atlas   []*locked // atlas.lock
	pkgs    []*installed
	srcRoot string
}

type resolver struct {
	root     string
	files    map[string]bool
	dirs     map[string]bool
	projects map[string]*project
	order    []*project          // shallowest first
	configs  map[string][]string // directory -> the search dirs its configurations add
	stdRoots []string            // directories holding Nim's standard library (lib/ of Nim's repository)
	global   []*installed        // the nimble directory's packages the manifests name
	byAbs    map[string]*installed
}

func readFile(abs string) []byte {
	st, err := os.Stat(abs)
	if err != nil || st.Size() > lang.MaxParseSize || st.IsDir() {
		return nil
	}
	data, _ := os.ReadFile(abs)
	return data
}

// newResolver reads the .nimble files and, beside them, nimble.lock,
// atlas.lock, nimble's plain requires file, nimble.paths and what nimble and
// Atlas installed; the search paths of nim.cfg and NimScript configurations;
// and the packages of the nimble directory (NIMBLE_DIR, else ~/.nimble) the
// manifests name.
//
// Implements: REQ-NIM-004, REQ-NIM-005, REQ-NIM-006, REQ-NIM-007, REQ-NIM-011
func newResolver(root string, all []*scan.File, getenv func(string) string) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, dirs: map[string]bool{}, projects: map[string]*project{},
		configs: map[string][]string{}, byAbs: map[string]*installed{}}
	onDisk := func(rel string) []byte {
		if root == "" {
			return nil
		}
		return readFile(filepath.Join(root, filepath.FromSlash(rel)))
	}
	var nimbles, configs []*scan.File
	for _, f := range all {
		if ignored(f) {
			continue
		}
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		switch c := class(f.Path); {
		case strings.HasPrefix(c, classNimble):
			nimbles = append(nimbles, f)
		case c == classCfg, path.Ext(f.Path) == ".nims":
			configs = append(configs, f)
		}
	}
	for _, f := range nimbles {
		dir := path.Dir(f.Path)
		if r.projects[dir] != nil {
			continue
		}
		data := readFile(f.Abs)
		n := readNimble(data, strings.TrimSuffix(path.Base(f.Path), ".nimble"))
		p := &project{dir: dir, nimble: n, deps: n.deps, srcRoot: dir}
		if n.srcDir != "" {
			if d := path.Join(dir, n.srcDir); r.dirs[d] {
				p.srcRoot = d
			}
		}
		p.deps = append(p.deps, readRequiresFile(onDisk(path.Join(dir, "requires")))...)
		// Libraries often keep their lock files out of git.
		p.lock = readNimbleLock(onDisk(path.Join(dir, "nimble.lock")))
		p.atlas = readAtlasLock(onDisk(path.Join(dir, "atlas.lock")))
		if root != "" {
			abs := filepath.Join(root, filepath.FromSlash(dir))
			p.pkgs = append(p.pkgs, readPkgs(filepath.Join(abs, "nimbledeps"), true, nil, nil)...)
			if deps := atlasDepsDir(abs); deps != "" {
				p.pkgs = append(p.pkgs, readAtlas(deps)...)
			}
			p.pkgs = append(p.pkgs, fromPaths(readCfg(onDisk(path.Join(dir, "nimble.paths"))))...)
		}
		for _, ip := range p.pkgs {
			r.byAbs[filepath.Clean(ip.root)] = ip
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
	for d := range r.dirs {
		if path.Base(d) == "lib" && r.files[d+"/system.nim"] && r.dirs[d+"/pure"] {
			r.stdRoots = append(r.stdRoots, d)
		}
	}
	if r.files["system.nim"] && r.dirs["pure"] {
		r.stdRoots = append(r.stdRoots, ".")
	}
	sort.Strings(r.stdRoots)
	for _, f := range configs {
		dir := path.Dir(f.Path)
		var paths []pathSwitch
		if class(f.Path) == classCfg {
			paths = readCfg(readFile(f.Abs))
		} else {
			paths = scanSource(readFile(f.Abs)).paths()
		}
		// A <name>.nims or <name>.nim.cfg configures the compilation of
		// <name>.nim, whose imports reach the modules around it: read like
		// config.nims and nim.cfg, for the directory.
		for _, p := range paths {
			if d, abs := expandPath(dir, p.value, r.lib()); d != "" && !abs && !strings.HasPrefix(d, "..") {
				r.configs[dir] = append(r.configs[dir], d)
			}
		}
	}
	r.readGlobal(getenv)
	return r
}

// readGlobal reads the packages of the nimble directory that a manifest names,
// the locked version if it is installed, else the newest.
func (r *resolver) readGlobal(getenv func(string) string) {
	dir := getenv("NIMBLE_DIR")
	if dir == "" {
		home := getenv("HOME")
		if home == "" {
			return
		}
		dir = filepath.Join(home, ".nimble")
	}
	want, prefer := map[string]bool{}, map[string]string{}
	for _, p := range r.order {
		for _, d := range p.deps {
			want[fold(d.name)] = true
			if d.url != "" {
				want[fold(repoBase(d.url))] = true
			}
		}
		for _, l := range append(append([]*locked{}, p.lock...), p.atlas...) {
			want[fold(l.name)] = true
			if l.version != "" {
				prefer[fold(l.name)] = l.version
			}
		}
	}
	delete(want, "nim")
	if len(want) == 0 {
		return
	}
	r.global = readPkgs(dir, false, want, prefer)
	for _, ip := range r.global {
		r.byAbs[filepath.Clean(ip.root)] = ip
	}
}

// lib is the repository's own standard library directory, or "".
func (r *resolver) lib() string {
	if len(r.stdRoots) > 0 {
		return r.stdRoots[0]
	}
	return ""
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// fold is how names are compared: case and `_`/`-` do not matter (Nim is
// style-insensitive), nor a repository's nim- prefix or -nim suffix.
func fold(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".git"), ".nim")
	s = strings.TrimPrefix(strings.TrimSuffix(s, "-nim"), "nim-")
	return strings.NewReplacer("_", "", "-", "").Replace(s)
}

// repoBase is the last path element of a repository URL.
func repoBase(url string) string {
	name := lang.RepoName(url)
	return name[strings.LastIndexByte(name, '/')+1:]
}

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

// scope is the projects whose requirements a file can use: its own, or, for a
// file no .nimble governs, every project, shallowest first.
func (r *resolver) scope(file string) []*project {
	if p := r.projectOf(path.Dir(file)); p != nil {
		return []*project{p}
	}
	return r.order
}

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindImport, kindInclude:
		return r.module(file, imp.Module)
	case kindRequire:
		d, ok := parseDep(imp.Module, imp.Line, "")
		if !ok || d.compiler() {
			return lang.Target{}
		}
		p := r.projects[path.Dir(file)]
		if p == nil {
			t := lang.Target{Ecosystem: ecoNimble, Package: d.pkg()}
			pinRule(&t, d.ver)
			return t
		}
		return r.target(p, d.pkg())
	case kindBin:
		if p := r.projects[path.Dir(file)]; p != nil {
			for _, f := range []string{path.Join(p.srcRoot, imp.Module+".nim"), path.Join(p.dir, imp.Module+".nim")} {
				if r.files[f] {
					return lang.Target{Local: f}
				}
			}
		}
		return lang.Target{}
	case kindPath:
		return r.pathTarget(file, imp.Module)
	case kindLock, kindAtlas:
		if p := r.projects[path.Dir(file)]; p != nil {
			return r.target(p, imp.Module)
		}
		return lang.Target{Ecosystem: ecoNimble, Package: imp.Module}
	}
	return lang.Target{}
}

// pathTarget resolves a configured search path: a directory of the project, or
// the installed package that directory is.
func (r *resolver) pathTarget(file, value string) lang.Target {
	d, abs := expandPath(path.Dir(file), value, r.lib())
	if d == "" {
		return lang.Target{}
	}
	if !abs && !strings.HasPrefix(d, "..") && (r.dirs[d] || d == ".") {
		return lang.Target{Local: d}
	}
	full := d
	if !abs {
		full = filepath.Join(r.root, filepath.FromSlash(d))
	}
	if ip := r.byAbs[filepath.Clean(full)]; ip != nil {
		return r.installedTarget(file, ip)
	}
	return lang.Target{}
}

// probe finds module under dir as the compiler does: as written, then in lower
// case (std/compileSettings is compilesettings.nim).
func (r *resolver) probe(dir, module string) string {
	f := path.Join(dir, module+".nim")
	if strings.HasPrefix(f, "../") || f == ".." {
		return ""
	}
	if r.files[f] {
		return f
	}
	if lower := path.Join(dir, strings.ToLower(module)+".nim"); r.files[lower] {
		return lower
	}
	return ""
}

// searchRoots are the directories an import by name is looked up in after the
// importing file's own: the package's srcDir (or its directory), then the
// --path entries of the configurations of its directory and those above it,
// nearest first.
func (r *resolver) searchRoots(file string) []string {
	var out []string
	if p := r.projectOf(path.Dir(file)); p != nil {
		out = append(out, p.srcRoot)
	}
	for d := path.Dir(file); ; d = path.Dir(d) {
		out = append(out, r.configs[d]...)
		if d == "." || d == "/" {
			break
		}
	}
	return out
}

// module resolves an import or include of module from file.
//
// Implements: REQ-NIM-004, REQ-NIM-007, REQ-NIM-008
func (r *resolver) module(file, module string) lang.Target {
	dir := path.Dir(file)
	if rest, ok := strings.CutPrefix(module, "$lib/"); ok {
		module = "std/" + rest
	}
	switch {
	case strings.HasPrefix(module, "$"), strings.HasPrefix(module, "/"):
		return lang.Target{}
	case strings.HasPrefix(module, "std/"):
		s := strings.TrimPrefix(module, "std/")
		if f := r.stdFile(s, true); f != "" {
			return lang.Target{Local: f}
		}
		return lang.Target{Ecosystem: ecoStd, Package: s}
	case strings.HasPrefix(module, "pkg/"):
		return r.pkgModule(file, strings.TrimPrefix(module, "pkg/"))
	case strings.HasPrefix(module, "."):
		// ./x and ../x are only looked up beside the importing file.
		if f := r.probe(dir, module); f != "" {
			return lang.Target{Local: f}
		}
		return lang.Target{}
	}
	if base := path.Base(module); strings.Contains(base, ".") {
		// `include "nimble.paths"`: a file named with its extension.
		for _, d := range append([]string{dir}, r.searchRoots(file)...) {
			if f := path.Join(d, module); r.files[f] {
				return lang.Target{Local: f}
			}
		}
		return lang.Target{}
	}
	if f := r.probe(dir, module); f != "" && f != file {
		return lang.Target{Local: f}
	}
	for _, root := range r.searchRoots(file) {
		if f := r.probe(root, module); f != "" && f != file {
			return lang.Target{Local: f}
		}
	}
	if f := r.stdFile(module, false); f != "" {
		return lang.Target{Local: f}
	}
	if isStd(module) || isStd(strings.ToLower(module)) {
		if name := movedModules[module]; name != "" {
			for _, p := range r.scope(file) {
				if node, _, _, _, ok := r.node(p, name); ok {
					return r.target(p, node)
				}
			}
		}
		return lang.Target{Ecosystem: ecoStd, Package: module}
	}
	return r.pkgModule(file, module)
}

// stdFile finds a module of the standard library in the repository (Nim's own
// lib/), where the compiler would: lib/<stdlibDir>/x.nim, then lib/x.nim
// (lib/std/x.nim for std/x).
func (r *resolver) stdFile(module string, explicit bool) string {
	for _, root := range r.stdRoots {
		for _, d := range stdlibDirs {
			if f := r.probe(path.Join(root, d), module); f != "" {
				return f
			}
		}
		if explicit {
			if f := r.probe(path.Join(root, "std"), module); f != "" {
				return f
			}
		}
		if f := r.probe(root, module); f != "" {
			return f
		}
	}
	return ""
}

// pkgModule resolves a module of a nimble package: the installed package that
// has the module (authoritative), else the requirement or locked package its
// first segment names, else an unresolved package of that name.
//
// Implements: REQ-NIM-007
func (r *resolver) pkgModule(file, module string) lang.Target {
	first, _, _ := strings.Cut(module, "/")
	scope := r.scope(file)
	for _, p := range scope {
		for _, ip := range p.pkgs {
			if ip.modules[module] {
				return r.installedTarget(file, ip)
			}
		}
	}
	for _, ip := range r.global {
		if ip.modules[module] {
			return r.installedTarget(file, ip)
		}
	}
	for _, p := range scope {
		if node, _, _, _, ok := r.node(p, first); ok {
			t := r.target(p, node)
			if t.Local != "" {
				// A requirement from a directory of the repository: its module.
				for _, d := range []string{t.Local, path.Join(t.Local, "src")} {
					if f := r.probe(d, module); f != "" {
						return lang.Target{Local: f}
					}
				}
			}
			return t
		}
	}
	for _, p := range scope {
		if fold(p.nimble.name) == fold(first) {
			return lang.Target{} // the package's own module, missing
		}
	}
	// A directory of the project (a module missing from it) is not a package.
	for _, root := range append([]string{path.Dir(file)}, r.searchRoots(file)...) {
		if r.dirs[path.Join(root, first)] {
			return lang.Target{}
		}
	}
	return lang.Target{Ecosystem: ecoNimble, Package: first, Unresolved: true}
}

// installedTarget is the package an installed directory is, as the file's
// project requires or locks it.
func (r *resolver) installedTarget(file string, ip *installed) lang.Target {
	for _, p := range r.scope(file) {
		if node, _, _, _, ok := r.node(p, ip.name); ok {
			return r.target(p, node)
		}
		if ip.url != "" {
			if node, _, _, _, ok := r.node(p, repoBase(ip.url)); ok {
				return r.target(p, node)
			}
		}
	}
	return lang.Target{Ecosystem: ecoNimble, Package: ip.name, Version: ip.version}
}

// node finds the package a project knows by name: a requirement of that name,
// a URL requirement whose repository is so named (nim-chronos is chronos), a
// nimble.lock or atlas.lock entry. It returns the node's name - a URL
// requirement's repository, else the package's name - with what declares and
// locks it.
func (r *resolver) node(p *project, name string) (node string, d *dep, l, al *locked, ok bool) {
	want := fold(name)
	if want == "" || want == "nim" {
		return "", nil, nil, nil, false
	}
	for i := range p.deps {
		x := &p.deps[i]
		if x.compiler() {
			continue
		}
		if x.url == "" && fold(x.name) == want || x.url != "" && (fold(repoBase(x.url)) == want || x.pkg() == name) {
			d = x
			break
		}
	}
	for _, x := range p.lock {
		if fold(x.name) == want || d != nil && d.url != "" && x.url != "" && lang.RepoName(x.url) == d.pkg() {
			l = x
			break
		}
	}
	for _, x := range p.atlas {
		if fold(x.name) == want || x.url != "" && (fold(repoBase(x.url)) == want || d != nil && d.url != "" && lang.RepoName(x.url) == d.pkg()) {
			al = x
			break
		}
	}
	switch {
	case d != nil:
		return d.pkg(), d, l, al, true
	case l != nil:
		// A lock entry of a URL requirement is named by the requirement.
		for i := range p.deps {
			if x := &p.deps[i]; x.url != "" && l.url != "" && lang.RepoName(l.url) == x.pkg() {
				return x.pkg(), x, l, al, true
			}
		}
		return l.name, nil, l, al, true
	case al != nil:
		for i := range p.deps {
			if x := &p.deps[i]; x.url != "" && al.url != "" && lang.RepoName(al.url) == x.pkg() {
				return x.pkg(), x, l, al, true
			}
		}
		return al.name, nil, l, al, true
	}
	return "", nil, nil, nil, false
}

// target is a package as project p requires and locks it: nimble.lock, else
// atlas.lock, pins it, with the requirement as requested; else the
// requirement's own pin rule; a package p does not know is unresolved unless
// it is installed.
//
// Implements: REQ-NIM-006
func (r *resolver) target(p *project, name string) lang.Target {
	node, d, l, al, ok := r.node(p, name)
	if !ok {
		for _, ip := range p.pkgs {
			if fold(ip.name) == fold(name) {
				return lang.Target{Ecosystem: ecoNimble, Package: ip.name, Version: ip.version}
			}
		}
		return lang.Target{Ecosystem: ecoNimble, Package: name, Unresolved: true}
	}
	if d != nil {
		if dir, ok := d.file(); ok {
			// A package from a directory: the directory when it is the
			// repository's, else a floating package from there.
			if !path.IsAbs(dir) {
				if local := path.Join(p.dir, dir); r.dirs[local] {
					return lang.Target{Local: local}
				}
			}
			return lang.Target{Ecosystem: ecoNimble, Package: node, Floating: true, Origin: dir}
		}
	}
	t := lang.Target{Ecosystem: ecoNimble, Package: node}
	url := ""
	if d != nil {
		url = d.url
	}
	if lk := l; lk != nil || al != nil {
		if lk == nil {
			lk = al
		}
		t.Version, t.Pinned = lk.shown(), true
		if url == "" {
			url = lk.url
		}
		if d != nil && d.ver != "" && d.ver != t.Version && d.ver != "=="+t.Version && d.ver != "== "+t.Version {
			t.Requested = d.ver
		}
	} else {
		pinRule(&t, d.ver)
	}
	if !public(url) {
		t.Origin = url
	}
	return t
}

// public reports whether a package's repository is on a public forge (or
// unknown), as opposed to a server of the organization's own, which is
// recorded as the package's origin.
func public(url string) bool {
	if url == "" || strings.HasPrefix(url, "file://") {
		return url == ""
	}
	host, _, _ := strings.Cut(lang.RepoName(url), "/")
	switch host {
	case "github.com", "gitlab.com", "bitbucket.org", "codeberg.org", "git.sr.ht", "sr.ht":
		return true
	}
	return false
}

// Dependencies lists what a nimble package depends on: nimble.lock's
// dependencies of it, else the requirements of its installed .nimble file,
// each pinned as the project that locks it pins it.
//
// Implements: REQ-NIM-006, REQ-NIM-007
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoNimble {
		return nil
	}
	for _, p := range r.order {
		if _, _, l, _, ok := r.node(p, t.Package); ok && l != nil {
			var out []lang.Target
			for _, name := range l.deps {
				if fold(name) == "nim" {
					continue
				}
				out = append(out, r.target(p, name))
			}
			return out
		}
	}
	if ip, p := r.installedOf(t); ip != nil {
		var out []lang.Target
		for _, d := range ip.deps {
			if d.compiler() {
				continue
			}
			if p != nil {
				if node, _, _, _, ok := r.node(p, d.pkg()); ok {
					out = append(out, r.target(p, node))
					continue
				}
			}
			dt := lang.Target{Ecosystem: ecoNimble, Package: d.pkg()}
			pinRule(&dt, d.ver)
			out = append(out, dt)
		}
		return out
	}
	return nil
}

// installedOf is the installed package a target names, with the project that
// installed it (nil for the nimble directory's).
func (r *resolver) installedOf(t lang.Target) (*installed, *project) {
	for _, p := range r.order {
		for _, ip := range p.pkgs {
			if names(ip, t.Package) {
				return ip, p
			}
		}
	}
	for _, ip := range r.global {
		if names(ip, t.Package) {
			for _, p := range r.order {
				if _, _, _, _, ok := r.node(p, ip.name); ok {
					return ip, p
				}
			}
			return ip, nil
		}
	}
	return nil, nil
}

// names reports whether an installed package is the node named pkg: by its
// name, its checkout's repository, or a repository named like it.
func names(ip *installed, pkg string) bool {
	switch {
	case ip.name == pkg:
		return true
	case ip.url != "" && lang.RepoName(ip.url) == pkg:
		return true
	}
	return strings.Contains(pkg, "/") && fold(repoBase(pkg)) == fold(ip.name)
}

// Installed reports whether a package's dependencies come from what is
// installed rather than from a lock file.
func (r *resolver) Installed(t lang.Target) bool {
	if t.Ecosystem != ecoNimble {
		return false
	}
	for _, p := range r.order {
		if _, _, l, _, ok := r.node(p, t.Package); ok && l != nil {
			return false
		}
	}
	ip, _ := r.installedOf(t)
	return ip != nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
