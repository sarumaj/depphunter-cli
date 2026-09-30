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
	directory    string
	nimble       *nimbleFile
	dependencies []dependency
	lock         []*locked // nimble.lock
	atlas        []*locked // atlas.lock
	packages     []*installed
	sourceRoot   string
	develop      map[string]*project // nimble.develop: folded package name -> the repository's package
}

type resolver struct {
	root        string
	files       map[string]bool
	directories map[string]bool
	projects    map[string]*project
	order       []*project          // shallowest first
	configs     map[string][]string // directory -> the search dirs its configurations add
	stdRoots    []string            // directories holding Nim's standard library (lib/ of Nim's repository)
	global      []*installed        // the nimble directory's packages the manifests name
	byAbsolute  map[string]*installed
}

func readFile(absolute string) []byte {
	fileInfo, err := os.Stat(absolute)
	if err != nil || fileInfo.Size() > lang.MaxParseSize || fileInfo.IsDir() {
		return nil
	}
	data, _ := os.ReadFile(absolute)
	return data
}

// newResolver reads the .nimble files and, beside them, nimble.lock,
// atlas.lock, nimble's plain requires file, nimble.paths, nimble.develop and
// what nimble and Atlas installed; the search paths of nim.cfg and NimScript configurations;
// and the packages of the nimble directory (NIMBLE_DIR, else ~/.nimble) the
// manifests name.
//
// Implements: REQ-NIM-004, REQ-NIM-005, REQ-NIM-006, REQ-NIM-007, REQ-NIM-011
func newResolver(root string, all []*scan.File, getenv func(string) string) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, directories: map[string]bool{}, projects: map[string]*project{},
		configs: map[string][]string{}, byAbsolute: map[string]*installed{}}
	onDisk := func(relative string) []byte {
		if root == "" {
			return nil
		}
		return readFile(filepath.Join(root, filepath.FromSlash(relative)))
	}
	var nimbles, configs []*scan.File
	for _, f := range all {
		if ignored(f) {
			continue
		}
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.directories[d]; d = path.Dir(d) {
			r.directories[d] = true
		}
		switch c := class(f.Path); {
		case strings.HasPrefix(c, classNimble):
			nimbles = append(nimbles, f)
		case c == classConfig, path.Ext(f.Path) == ".nims":
			configs = append(configs, f)
		}
	}
	for _, f := range nimbles {
		directory := path.Dir(f.Path)
		if r.projects[directory] != nil {
			continue
		}
		data := readFile(f.AbsolutePath)
		n := readNimble(data, strings.TrimSuffix(path.Base(f.Path), ".nimble"))
		p := &project{directory: directory, nimble: n, dependencies: n.dependencies, sourceRoot: directory}
		if n.sourceDirectory != "" {
			if d := path.Join(directory, n.sourceDirectory); r.directories[d] {
				p.sourceRoot = d
			}
		}
		p.dependencies = append(p.dependencies, readRequiresFile(onDisk(path.Join(directory, "requires")))...)
		// Libraries often keep their lock files out of git.
		p.lock = readNimbleLock(onDisk(path.Join(directory, "nimble.lock")))
		p.atlas = readAtlasLock(onDisk(path.Join(directory, "atlas.lock")))
		if root != "" {
			absolute := filepath.Join(root, filepath.FromSlash(directory))
			p.packages = append(p.packages, readPackages(filepath.Join(absolute, "nimbledeps"), true, nil, nil)...)
			if dependencies := atlasDependenciesDirectory(absolute); dependencies != "" {
				p.packages = append(p.packages, readAtlas(dependencies)...)
			}
			p.packages = append(p.packages, fromPaths(readConfig(onDisk(path.Join(directory, "nimble.paths"))))...)
		}
		for _, installedPackage := range p.packages {
			r.byAbsolute[filepath.Clean(installedPackage.root)] = installedPackage
		}
		r.projects[directory] = p
		r.order = append(r.order, p)
	}
	sort.Slice(r.order, func(i, j int) bool {
		depthI, depthJ := lang.Depth(r.order[i].directory), lang.Depth(r.order[j].directory)
		if depthI != depthJ {
			return depthI < depthJ
		}
		return r.order[i].directory < r.order[j].directory
	})
	for _, p := range r.order {
		p.develop = r.readDevelop(p.directory, onDisk)
	}
	for d := range r.directories {
		if path.Base(d) == "lib" && r.files[d+"/system.nim"] && r.directories[d+"/pure"] {
			r.stdRoots = append(r.stdRoots, d)
		}
	}
	if r.files["system.nim"] && r.directories["pure"] {
		r.stdRoots = append(r.stdRoots, ".")
	}
	sort.Strings(r.stdRoots)
	for _, f := range configs {
		directory := path.Dir(f.Path)
		var paths []pathSwitch
		if class(f.Path) == classConfig {
			paths = readConfig(readFile(f.AbsolutePath))
		} else {
			paths = scanSource(readFile(f.AbsolutePath)).paths()
		}
		// A <name>.nims or <name>.nim.cfg configures the compilation of
		// <name>.nim, whose imports reach the modules around it: read like
		// config.nims and nim.cfg, for the directory.
		for _, p := range paths {
			if d, absolute := expandPath(directory, p.value, r.library()); d != "" && !absolute && !strings.HasPrefix(d, "..") {
				r.configs[directory] = append(r.configs[directory], d)
			}
		}
	}
	r.readGlobal(getenv)
	return r
}

// readGlobal reads the packages of the nimble directory that a manifest names,
// the locked version if it is installed, else the newest.
func (r *resolver) readGlobal(getenv func(string) string) {
	directory := getenv("NIMBLE_DIR")
	if directory == "" {
		home := getenv("HOME")
		if home == "" {
			return
		}
		directory = filepath.Join(home, ".nimble")
	}
	want, prefer := map[string]bool{}, map[string]string{}
	for _, p := range r.order {
		for _, d := range p.dependencies {
			want[fold(d.name)] = true
			if d.url != "" {
				want[fold(repositoryBase(d.url))] = true
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
	r.global = readPackages(directory, false, want, prefer)
	for _, installedPackage := range r.global {
		r.byAbsolute[filepath.Clean(installedPackage.root)] = installedPackage
	}
}

// library is the repository's own standard library directory, or "".
func (r *resolver) library() string {
	if len(r.stdRoots) > 0 {
		return r.stdRoots[0]
	}
	return ""
}

// fold is how names are compared: case and `_`/`-` do not matter (Nim is
// style-insensitive), nor a repository's nim- prefix or -nim suffix.
func fold(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".git"), ".nim")
	s = strings.TrimPrefix(strings.TrimSuffix(s, "-nim"), "nim-")
	return strings.NewReplacer("_", "", "-", "").Replace(s)
}

// repositoryBase is the last path element of a repository URL.
func repositoryBase(url string) string {
	name := lang.RepositoryName(url)
	return name[strings.LastIndexByte(name, '/')+1:]
}

func (r *resolver) projectOf(directory string) *project {
	p, _ := lang.NearestAtOrAbove(r.projects, directory)
	return p
}

// scope is the projects whose requirements a file can use: its own, or, for a
// file no .nimble governs, every project, shallowest first.
func (r *resolver) scope(file string) []*project {
	if p := r.projectOf(path.Dir(file)); p != nil {
		return []*project{p}
	}
	return r.order
}

func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindImport, kindInclude:
		return r.module(file, rawImport.Module)
	case kindRequire:
		d, ok := parseDependency(rawImport.Module, rawImport.Line, "")
		if !ok || d.compiler() {
			return lang.Target{}
		}
		p := r.projects[path.Dir(file)]
		if p == nil {
			t := lang.Target{Ecosystem: ecosystemNimble, Package: d.packageName()}
			pinRule(&t, d.version)
			return t
		}
		return r.target(p, d.packageName())
	case kindBin:
		if p := r.projects[path.Dir(file)]; p != nil {
			for _, f := range []string{path.Join(p.sourceRoot, rawImport.Module+".nim"), path.Join(p.directory, rawImport.Module+".nim")} {
				if r.files[f] {
					return lang.Target{Local: f}
				}
			}
		}
		return lang.Target{}
	case kindPath:
		return r.pathTarget(file, rawImport.Module)
	case kindLock, kindAtlas:
		if p := r.projects[path.Dir(file)]; p != nil {
			return r.target(p, rawImport.Module)
		}
		return lang.Target{Ecosystem: ecosystemNimble, Package: rawImport.Module}
	}
	return lang.Target{}
}

// pathTarget resolves a configured search path: a directory of the project, or
// the installed package that directory is.
func (r *resolver) pathTarget(file, value string) lang.Target {
	d, absolute := expandPath(path.Dir(file), value, r.library())
	if d == "" {
		return lang.Target{}
	}
	if !absolute && !strings.HasPrefix(d, "..") && (r.directories[d] || d == ".") {
		return lang.Target{Local: d}
	}
	full := d
	if !absolute {
		full = filepath.Join(r.root, filepath.FromSlash(d))
	}
	if installedPackage := r.byAbsolute[filepath.Clean(full)]; installedPackage != nil {
		return r.installedTarget(file, installedPackage)
	}
	return lang.Target{}
}

// probe finds module under directory as the compiler does: as written, then in lower
// case (std/compileSettings is compilesettings.nim).
func (r *resolver) probe(directory, module string) string {
	f := path.Join(directory, module+".nim")
	if strings.HasPrefix(f, "../") || f == ".." {
		return ""
	}
	if r.files[f] {
		return f
	}
	if lower := path.Join(directory, strings.ToLower(module)+".nim"); r.files[lower] {
		return lower
	}
	return ""
}

// searchRoots are the directories an import by name is looked up in after the
// importing file's own: the package's sourceDirectory (or its directory), then the
// --path entries of the configurations of its directory and those above it,
// nearest first.
func (r *resolver) searchRoots(file string) []string {
	var out []string
	if p := r.projectOf(path.Dir(file)); p != nil {
		out = append(out, p.sourceRoot)
	}
	for d := range lang.Ancestors(file) {
		out = append(out, r.configs[d]...)
	}
	return out
}

// module resolves an import or include of module from file.
//
// Implements: REQ-NIM-004, REQ-NIM-007, REQ-NIM-008
func (r *resolver) module(file, module string) lang.Target {
	directory := path.Dir(file)
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
		return lang.Target{Ecosystem: ecosystemStd, Package: s}
	case strings.HasPrefix(module, "pkg/"):
		return r.packageModule(file, strings.TrimPrefix(module, "pkg/"))
	case strings.HasPrefix(module, "."):
		// ./x and ../x are only looked up beside the importing file.
		if f := r.probe(directory, module); f != "" {
			return lang.Target{Local: f}
		}
		return lang.Target{}
	}
	if base := path.Base(module); strings.Contains(base, ".") {
		// `include "nimble.paths"`: a file named with its extension.
		for _, d := range append([]string{directory}, r.searchRoots(file)...) {
			if f := path.Join(d, module); r.files[f] {
				return lang.Target{Local: f}
			}
		}
		return lang.Target{}
	}
	if f := r.probe(directory, module); f != "" && f != file {
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
		return lang.Target{Ecosystem: ecosystemStd, Package: module}
	}
	return r.packageModule(file, module)
}

// stdFile finds a module of the standard library in the repository (Nim's own
// lib/), where the compiler would: lib/<stdlibDir>/x.nim, then lib/x.nim
// (lib/std/x.nim for std/x).
func (r *resolver) stdFile(module string, explicit bool) string {
	for _, root := range r.stdRoots {
		for _, d := range stdlibDirectories {
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

// packageModule resolves a module of a nimble package: the repository's package
// nimble.develop develops, the installed package that has the module
// (authoritative), else the requirement or locked package its
// first segment names, else an unresolved package of that name.
//
// Implements: REQ-NIM-007
func (r *resolver) packageModule(file, module string) lang.Target {
	first, _, _ := strings.Cut(module, "/")
	scope := r.scope(file)
	// A package the project develops from a directory of the repository wins
	// over what is installed.
	for _, p := range scope {
		if q := p.developed(first); q != nil {
			for _, d := range []string{q.sourceRoot, q.directory} {
				if f := r.probe(d, module); f != "" {
					return lang.Target{Local: f}
				}
			}
			return lang.Target{Local: q.directory}
		}
	}
	for _, p := range scope {
		for _, installedPackage := range p.packages {
			if installedPackage.modules[module] {
				return r.installedTarget(file, installedPackage)
			}
		}
	}
	for _, installedPackage := range r.global {
		if installedPackage.modules[module] {
			return r.installedTarget(file, installedPackage)
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
		if r.directories[path.Join(root, first)] {
			return lang.Target{}
		}
	}
	return lang.Target{Ecosystem: ecosystemNimble, Package: first, Unresolved: true}
}

// installedTarget is the package an installed directory is, as the file's
// project requires or locks it.
func (r *resolver) installedTarget(file string, installedPackage *installed) lang.Target {
	for _, p := range r.scope(file) {
		if node, _, _, _, ok := r.node(p, installedPackage.name); ok {
			return r.target(p, node)
		}
		if installedPackage.url != "" {
			if node, _, _, _, ok := r.node(p, repositoryBase(installedPackage.url)); ok {
				return r.target(p, node)
			}
		}
	}
	return lang.Target{Ecosystem: ecosystemNimble, Package: installedPackage.name, Version: installedPackage.version}
}

// node finds the package a project knows by name: a requirement of that name,
// a URL requirement whose repository is so named (nim-chronos is chronos), a
// nimble.lock or atlas.lock entry. It returns the node's name - a URL
// requirement's repository, else the package's name - with what declares and
// locks it.
func (r *resolver) node(p *project, name string) (node string, d *dependency, l, al *locked, ok bool) {
	want := fold(name)
	if want == "" || want == "nim" {
		return "", nil, nil, nil, false
	}
	for i := range p.dependencies {
		x := &p.dependencies[i]
		if x.compiler() {
			continue
		}
		if x.url == "" && fold(x.name) == want || x.url != "" && (fold(repositoryBase(x.url)) == want || x.packageName() == name) {
			d = x
			break
		}
	}
	for _, x := range p.lock {
		if fold(x.name) == want || d != nil && d.url != "" && x.url != "" && lang.RepositoryName(x.url) == d.packageName() {
			l = x
			break
		}
	}
	for _, x := range p.atlas {
		if fold(x.name) == want || x.url != "" && (fold(repositoryBase(x.url)) == want || d != nil && d.url != "" && lang.RepositoryName(x.url) == d.packageName()) {
			al = x
			break
		}
	}
	switch {
	case d != nil:
		return d.packageName(), d, l, al, true
	case l != nil:
		// A lock entry of a URL requirement is named by the requirement.
		for i := range p.dependencies {
			if x := &p.dependencies[i]; x.url != "" && l.url != "" && lang.RepositoryName(l.url) == x.packageName() {
				return x.packageName(), x, l, al, true
			}
		}
		return l.name, nil, l, al, true
	case al != nil:
		for i := range p.dependencies {
			if x := &p.dependencies[i]; x.url != "" && al.url != "" && lang.RepositoryName(al.url) == x.packageName() {
				return x.packageName(), x, l, al, true
			}
		}
		return al.name, nil, l, al, true
	}
	return "", nil, nil, nil, false
}

// target is a package as project p requires and locks it: the directory of
// the repository's package when nimble.develop develops it; else nimble.lock,
// else atlas.lock, pins it, with the requirement as requested; else the
// requirement's own pin rule; a package p does not know is unresolved unless
// it is installed.
//
// Implements: REQ-NIM-006
func (r *resolver) target(p *project, name string) lang.Target {
	if q := p.developed(name); q != nil {
		return lang.Target{Local: q.directory}
	}
	node, d, l, al, ok := r.node(p, name)
	if !ok {
		for _, installedPackage := range p.packages {
			if fold(installedPackage.name) == fold(name) {
				return lang.Target{Ecosystem: ecosystemNimble, Package: installedPackage.name, Version: installedPackage.version}
			}
		}
		return lang.Target{Ecosystem: ecosystemNimble, Package: name, Unresolved: true}
	}
	if d != nil {
		if directory, ok := d.file(); ok {
			// A package from a directory: the directory when it is the
			// repository's, else a floating package from there.
			if !path.IsAbs(directory) {
				if local := path.Join(p.directory, directory); r.directories[local] {
					return lang.Target{Local: local}
				}
			}
			return lang.Target{Ecosystem: ecosystemNimble, Package: node, Floating: true, Origin: directory}
		}
	}
	t := lang.Target{Ecosystem: ecosystemNimble, Package: node}
	url := ""
	if d != nil {
		url = d.url
	}
	if lockedPackage := l; lockedPackage != nil || al != nil {
		if lockedPackage == nil {
			lockedPackage = al
		}
		t.Version, t.Pinned = lockedPackage.shown(), true
		if url == "" {
			url = lockedPackage.url
		}
		if d != nil && d.version != "" && d.version != t.Version && d.version != "=="+t.Version && d.version != "== "+t.Version {
			t.Requested = d.version
		}
	} else {
		pinRule(&t, d.version)
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
	return !strings.HasPrefix(url, "file://") && lang.PublicOrUnnamed(url)
}

// Dependencies lists what a nimble package depends on: nimble.lock's
// dependencies of it, else the requirements of its installed .nimble file,
// each pinned as the project that locks it pins it.
//
// Implements: REQ-NIM-006, REQ-NIM-007
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemNimble {
		return nil
	}
	for _, p := range r.order {
		if _, _, l, _, ok := r.node(p, t.Package); ok && l != nil {
			var out []lang.Target
			for _, name := range l.dependencies {
				if fold(name) == "nim" {
					continue
				}
				out = append(out, r.target(p, name))
			}
			return out
		}
	}
	if installedPackage, p := r.installedOf(t); installedPackage != nil {
		var out []lang.Target
		for _, d := range installedPackage.dependencies {
			if d.compiler() {
				continue
			}
			if p != nil {
				if node, _, _, _, ok := r.node(p, d.packageName()); ok {
					out = append(out, r.target(p, node))
					continue
				}
			}
			dependencyTarget := lang.Target{Ecosystem: ecosystemNimble, Package: d.packageName()}
			pinRule(&dependencyTarget, d.version)
			out = append(out, dependencyTarget)
		}
		return out
	}
	return nil
}

// installedOf is the installed package a target names, with the project that
// installed it (nil for the nimble directory's).
func (r *resolver) installedOf(t lang.Target) (*installed, *project) {
	for _, p := range r.order {
		for _, installedPackage := range p.packages {
			if names(installedPackage, t.Package) {
				return installedPackage, p
			}
		}
	}
	for _, installedPackage := range r.global {
		if names(installedPackage, t.Package) {
			for _, p := range r.order {
				if _, _, _, _, ok := r.node(p, installedPackage.name); ok {
					return installedPackage, p
				}
			}
			return installedPackage, nil
		}
	}
	return nil, nil
}

// names reports whether an installed package is the node named packageName: by its
// name, its checkout's repository, or a repository named like it.
func names(installedPackage *installed, packageName string) bool {
	switch {
	case installedPackage.name == packageName:
		return true
	case installedPackage.url != "" && lang.RepositoryName(installedPackage.url) == packageName:
		return true
	}
	return strings.Contains(packageName, "/") && fold(repositoryBase(packageName)) == fold(installedPackage.name)
}

// Installed reports whether a package's dependencies come from what is
// installed rather than from a lock file.
func (r *resolver) Installed(t lang.Target) bool {
	if t.Ecosystem != ecosystemNimble {
		return false
	}
	for _, p := range r.order {
		if _, _, l, _, ok := r.node(p, t.Package); ok && l != nil {
			return false
		}
	}
	installedPackage, _ := r.installedOf(t)
	return installedPackage != nil
}
