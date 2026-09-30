package solidity

import (
	"cmp"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// hardhatConfigs are the names Hardhat reads its configuration from.
var hardhatConfigs = []string{
	"hardhat.config.js", "hardhat.config.ts", "hardhat.config.cjs", "hardhat.config.mjs",
	"hardhat.config.cts", "hardhat.config.mts",
}

func hardhatConfig(base string) bool {
	for _, n := range hardhatConfigs {
		if base == n {
			return true
		}
	}
	return false
}

// generated are the directories a tool writes beside its configuration:
// Foundry's installed libraries, Soldeer's dependencies and forge's build
// output beside foundry.toml, Hardhat's artifacts, cache and TypeChain
// bindings beside hardhat.config.*.
var generated = map[string][]string{
	"lib":             {"foundry.toml"},
	"dependencies":    {"foundry.toml"},
	"out":             {"foundry.toml"},
	"cache":           append([]string{"foundry.toml"}, hardhatConfigs...),
	"artifacts":       hardhatConfigs,
	"typechain-types": hardhatConfigs,
}

var besideMemo lang.Memo[string, bool] // absolute path of a marker file -> it exists

func exists(absolute string) bool { return besideMemo.Get(absolute, lang.Present) }

// absoluteRoot is the scan root of f: its absolute path without its relative one.
func absoluteRoot(f *scan.File) (string, bool) {
	absolute := filepath.ToSlash(f.AbsolutePath)
	if f.AbsolutePath == "" || !strings.HasSuffix(absolute, f.Path) {
		return "", false
	}
	return strings.TrimSuffix(absolute[:len(absolute)-len(f.Path)], "/"), true
}

// ignored reports whether f lies in what a package manager installed or a
// build wrote: node_modules anywhere, and the directories of generated beside
// their tool's configuration.
//
// Implements: REQ-SOLIDITY-001
func ignored(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	root, ok := absoluteRoot(f)
	for i, s := range segments[:len(segments)-1] {
		if s == "node_modules" {
			return true
		}
		markers := generated[s]
		if len(markers) == 0 || !ok {
			continue
		}
		directory := path.Join(append([]string{root}, segments[:i]...)...)
		for _, m := range markers {
			if exists(path.Join(directory, m)) {
				return true
			}
		}
	}
	return false
}

// project is a directory with a foundry.toml, a hardhat.config.* or a
// remappings.txt: the base import paths and remappings are relative to.
type project struct {
	directory    string
	foundry      bool
	config       foundryConfig
	remaps       []remapping // explicit first, then inferred
	dependencies map[string]soldeerDependency
	locked       map[string]lockEntry
	// installed is what Soldeer installed into dependencies/, by directory:
	// each package's own [dependencies] and soldeer.lock.
	installed map[string]*project
}

func (p *project) soldeer() bool { return len(p.dependencies) > 0 || len(p.locked) > 0 }

// subReference is a submodule with its path relative to the scan root.
type subReference struct {
	submodule
	commit   string
	children []*subReference // the submodules of a checked-out submodule
}

type resolver struct {
	root        string
	files       map[string]bool
	directories map[string]bool
	projects    map[string]*project
	subs        []*subReference // longest path first
	byPath      map[string]*subReference
	npm         *javascript.Packages
}

func join(directory, p string) string {
	if directory == "." || directory == "" {
		return path.Clean(p)
	}
	return path.Clean(directory + "/" + p)
}

// readFile reads a scanned file through lang.ReadScanned, nil when it cannot.
func readFile(f *scan.File) []byte {
	data, _ := lang.ReadScanned(f)
	return data
}

// newResolver reads the projects' foundry.toml, remappings.txt and
// soldeer.lock, the repository's .gitmodules (and the commits git records
// for them), and the package.json and lock files of npm.
//
// Implements: REQ-SOLIDITY-004, REQ-SOLIDITY-005, REQ-SOLIDITY-006, REQ-SOLIDITY-007, REQ-SOLIDITY-011
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{
		root: root, files: map[string]bool{}, directories: map[string]bool{},
		projects: map[string]*project{}, byPath: map[string]*subReference{},
	}
	projectAt := func(directory string) *project {
		if p := r.projects[directory]; p != nil {
			return p
		}
		p := &project{directory: directory, dependencies: map[string]soldeerDependency{}, locked: map[string]lockEntry{}}
		r.projects[directory] = p
		return p
	}
	npm := false
	var gitmodules []*scan.File
	txt := map[string][]string{}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.directories[d]; d = path.Dir(d) {
			r.directories[d] = true
		}
	}
	for _, f := range all {
		directory, base := path.Dir(f.Path), path.Base(f.Path)
		switch {
		case base == "package.json":
			npm = true
		case base == ".gitmodules" && !ignored(f):
			gitmodules = append(gitmodules, f)
		case base == "foundry.toml" && !ignored(f):
			p := projectAt(directory)
			p.foundry = true
			if c, ok := readFoundry(readFile(f)); ok {
				p.config = c
				for _, d := range c.dependencies {
					p.dependencies[d.name] = d
				}
			} else {
				p.config = foundryConfig{libraries: []string{"lib"}, autoDetect: true}
			}
		case base == "remappings.txt" && !ignored(f):
			projectAt(directory)
			txt[directory], _ = remappingLines(readFile(f))
		case hardhatConfig(base) && !ignored(f):
			projectAt(directory)
		}
	}
	// soldeer.lock is read from disk: it sits beside foundry.toml.
	repository := lang.OpenRoot(root)
	for directory, p := range r.projects {
		if !p.foundry {
			continue
		}
		if data, ok := repository.ReadBounded(filepath.Join(root, filepath.FromSlash(join(directory, "soldeer.lock")))); ok {
			for _, e := range readSoldeerLock(data) {
				p.locked[e.name] = e
			}
		}
		if p.soldeer() {
			p.installed = readInstalled(repository, filepath.Join(root, filepath.FromSlash(join(directory, "dependencies"))))
		}
	}
	r.readSubmodules(gitmodules)
	if npm {
		r.npm = javascript.ReadPackages(all)
	}
	for directory, p := range r.projects {
		p.remaps = r.remappings(p, txt[directory])
	}
	return r
}

// readSubmodules reads each .gitmodules of the file list (and, when the scan
// did not list one, the root's), asks git for the commits recorded, and reads
// the .gitmodules of submodules that are checked out.
func (r *resolver) readSubmodules(gitmodules []*scan.File) {
	type source struct {
		directory string // relative to the scan root
		data      []byte
	}
	var sources []source
	for _, f := range gitmodules {
		sources = append(sources, source{path.Dir(f.Path), readFile(f)})
	}
	repository := lang.OpenRoot(r.root)
	if len(sources) == 0 {
		if data, ok := repository.ReadBounded(filepath.Join(r.root, ".gitmodules")); ok {
			sources = append(sources, source{".", data})
		}
	}
	var add func(directory string, data []byte, parent *subReference, depth int)
	add = func(directory string, data []byte, parent *subReference, depth int) {
		subs := readGitmodules(data)
		var paths []string
		for _, s := range subs {
			paths = append(paths, s.path)
		}
		links := gitlinks(filepath.Join(r.root, filepath.FromSlash(directory)), paths)
		for _, s := range subs {
			reference := &subReference{submodule: s, commit: links[s.path]}
			reference.path = join(directory, s.path)
			// A path that climbs out, ".." itself included, is no submodule of
			// the repository.
			if !lang.Inside(reference.path) || r.byPath[reference.path] != nil {
				continue
			}
			r.byPath[reference.path] = reference
			r.subs = append(r.subs, reference)
			if parent != nil {
				parent.children = append(parent.children, reference)
			}
			if depth < maxNesting {
				nested := filepath.Join(r.root, filepath.FromSlash(reference.path), ".gitmodules")
				if data, ok := repository.ReadBounded(nested); ok {
					add(reference.path, data, reference, depth+1)
				}
			}
		}
	}
	for _, s := range sources {
		add(s.directory, s.data, nil, 0)
	}
	sort.SliceStable(r.subs, func(i, j int) bool { return len(r.subs[i].path) > len(r.subs[j].path) })
}

// maxNesting bounds how deep checked-out submodules' own submodules are read.
const maxNesting = 4

// remappings are a project's remappings in the order they apply: foundry.toml's
// (every profile's, the default's first) and remappings.txt's, then what
// Foundry infers for the libraries installed in its libs directories
// (`dep/=lib/dep/src/` when the library has a src directory, else
// `dep/=lib/dep/`, one level of nested libraries too) and what Soldeer
// generates for its dependencies (`name-version/=dependencies/name-version/`).
// An inferred remapping never replaces an explicit one of the same prefix.
//
// Implements: REQ-SOLIDITY-005
func (r *resolver) remappings(p *project, txt []string) []remapping {
	var out []remapping
	have := map[string]bool{}
	addLine := func(s string) {
		if remapping, ok := parseRemapping(s); ok && !have[remapping.context+":"+remapping.prefix] {
			have[remapping.context+":"+remapping.prefix] = true
			out = append(out, remapping)
		}
	}
	for _, s := range p.config.remappings {
		addLine(s)
	}
	for _, s := range txt {
		addLine(s)
	}
	infer := func(prefix, target string) {
		if !have[":"+prefix] && !have[":"+strings.TrimSuffix(prefix, "/")] {
			have[":"+prefix] = true
			out = append(out, remapping{prefix: prefix, target: target})
		}
	}
	if p.soldeer() {
		names := make([]string, 0, len(p.dependencies)+len(p.locked))
		for n := range p.dependencies {
			names = append(names, n)
		}
		for n := range p.locked {
			if _, ok := p.dependencies[n]; !ok {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		for _, n := range names {
			v := cmp.Or(p.locked[n].version, p.dependencies[n].version)
			if v == "" {
				continue
			}
			key := p.config.soldeerPrefix + n
			if !p.config.soldeerNoVersion {
				key += "-" + v
			}
			infer(key+"/", "dependencies/"+n+"-"+v+"/")
		}
	}
	if !p.foundry || !p.config.autoDetect {
		return out
	}
	var nested [][2]string
	for _, library := range p.config.libraries {
		if library == "" || library == "node_modules" {
			continue // npm packages are resolved as npm resolves them
		}
		for _, dependency := range r.libraries(join(p.directory, library)) {
			relative := library + "/" + dependency
			absolute := r.absolute(join(p.directory, relative))
			if lang.OpenRoot(r.root).IsDirectory(filepath.Join(absolute, "src")) {
				infer(dependency+"/", relative+"/src/")
			} else {
				infer(dependency+"/", relative+"/")
			}
			for _, x := range r.libraries(join(p.directory, relative+"/lib")) {
				nested = append(nested, [2]string{x, relative + "/lib/" + x})
			}
		}
	}
	for _, n := range nested {
		if lang.OpenRoot(r.root).IsDirectory(filepath.Join(r.absolute(join(p.directory, n[1])), "src")) {
			infer(n[0]+"/", n[1]+"/src/")
		} else {
			infer(n[0]+"/", n[1]+"/")
		}
	}
	return out
}

func (r *resolver) absolute(relative string) string {
	return filepath.Join(r.root, filepath.FromSlash(relative))
}

// libraries lists the libraries in a libs directory: the submodules git
// records there and the directories on disk (checked out or vendored).
func (r *resolver) libraries(directory string) []string {
	set := map[string]bool{}
	for _, s := range r.subs {
		if rest, ok := strings.CutPrefix(s.path, directory+"/"); ok && !strings.Contains(rest, "/") {
			set[rest] = true
		}
	}
	if entries, err := lang.OpenRoot(r.root).ReadDir(r.absolute(directory)); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				set[e.Name()] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// projectOf is the nearest project above file.
func (r *resolver) projectOf(file string) *project {
	p, _ := lang.Nearest(r.projects, file)
	return p
}

// remap applies p's remappings to an import path of file the way solc does:
// the remapping with the longest context the file's path starts with, then
// the longest prefix, the first listed on a tie. The result is relative to
// the scan root.
//
// Implements: REQ-SOLIDITY-005
func (p *project) remap(file, spec string) (string, bool) {
	relative := file
	if p.directory != "." {
		relative = strings.TrimPrefix(file, p.directory+"/")
	}
	best := -1
	for i, remapping := range p.remaps {
		if !strings.HasPrefix(spec, remapping.prefix) || remapping.context != "" && !strings.HasPrefix(relative, remapping.context) {
			continue
		}
		if best < 0 || len(remapping.context) > len(p.remaps[best].context) ||
			len(remapping.context) == len(p.remaps[best].context) && len(remapping.prefix) > len(p.remaps[best].prefix) {
			best = i
		}
	}
	if best < 0 {
		return "", false
	}
	remapping := p.remaps[best]
	return join(p.directory, remapping.target+spec[len(remapping.prefix):]), true
}

func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	directory := path.Dir(file)
	switch rawImport.Name {
	case kindRemap:
		remapping, ok := parseRemapping(rawImport.Module)
		if !ok {
			return lang.Target{}
		}
		t, _ := r.located(join(directory, remapping.target), r.projects[directory], file)
		return t
	case kindDependency, kindLock:
		if p := r.projects[directory]; p != nil {
			return p.soldeerTarget(rawImport.Module)
		}
		return lang.Target{}
	case kindSubmodule:
		if s := r.byPath[join(directory, rawImport.Module)]; s != nil {
			return subTarget(s)
		}
		return lang.Target{}
	}
	return r.resolveImport(file, rawImport.Module)
}

// resolveImport resolves an import path in the order solc and the tools look:
// a relative path from the importing file; the project's remappings; an npm
// package a package.json declares (Hardhat reads imports from node_modules);
// a path from the project's root, then the repository's; else a package named
// by the path's first segment (an npm scope and name for @scope/name) that
// nothing declares.
//
// Implements: REQ-SOLIDITY-004
func (r *resolver) resolveImport(file, spec string) lang.Target {
	if spec == "" || strings.Contains(spec, "://") || strings.HasPrefix(spec, "/") {
		return lang.Target{}
	}
	if spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
		t, _ := r.located(join(path.Dir(file), spec), r.projectOf(file), file)
		return t // a relative path to nothing is outside the project: dropped
	}
	p := r.projectOf(file)
	if p != nil {
		if q, ok := p.remap(file, spec); ok {
			t, _ := r.located(q, p, file)
			return t
		}
	}
	if r.npm != nil {
		if t, ok := r.npm.Package(spec, file); ok {
			return t
		}
	}
	bases := []string{"."}
	if p != nil && p.directory != "." {
		bases = []string{p.directory, "."}
	}
	for _, b := range bases {
		if q := join(b, spec); r.files[q] {
			return lang.Target{Local: q}
		}
	}
	name := javascript.PackageName(spec)
	first, _, _ := strings.Cut(spec, "/")
	if name == "" || name == "." || name == ".." {
		return lang.Target{}
	}
	for _, b := range bases {
		if r.directories[join(b, first)] {
			return lang.Target{} // a missing file of the project's own directories
		}
	}
	if !strings.HasPrefix(name, "@") && p != nil && p.foundry {
		return lang.Target{Ecosystem: ecosystemGit, Package: first, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecosystemNPM, Package: name, Unresolved: true}
}

// located resolves a path relative to the scan root that an import or a
// remapping arrived at: into a submodule, an npm package's directory, a
// library in a libs directory, a Soldeer dependency, or a file or directory
// of the project. Dependencies are packages even when their files are on
// disk: they are not the project's own code. ok is false when the path is
// none of these; outside the repository it is dropped.
func (r *resolver) located(q string, p *project, file string) (lang.Target, bool) {
	if q == ".." || strings.HasPrefix(q, "../") {
		return lang.Target{}, true
	}
	for _, s := range r.subs {
		if lang.WithinOrEqual(q, s.path) {
			return subTarget(s), true
		}
	}
	if i := strings.Index("/"+q, "/node_modules/"); i >= 0 {
		rest := q[i+len("node_modules/"):]
		name := javascript.PackageName(rest)
		if name == "" || strings.HasPrefix(name, "@") && !strings.Contains(strings.TrimSuffix(name, "/"), "/") {
			// node_modules/ or a scope's directory: no one package
			return lang.Target{}, true
		}
		if r.npm != nil {
			if t, ok := r.npm.Package(rest, file); ok && t.Ecosystem != "" {
				return t, true
			}
		}
		return lang.Target{Ecosystem: ecosystemNPM, Package: name, Unresolved: true}, true
	}
	if p != nil && p.soldeer() {
		if rest, ok := strings.CutPrefix(q, join(p.directory, "dependencies")+"/"); ok {
			directory, _, _ := strings.Cut(rest, "/")
			return p.soldeerTarget(p.soldeerName(directory)), true
		}
	}
	if p != nil && p.foundry {
		for _, library := range p.config.libraries {
			if library == "" || library == "node_modules" {
				continue
			}
			if rest, ok := strings.CutPrefix(q, join(p.directory, library)+"/"); ok {
				dependency, _, _ := strings.Cut(rest, "/")
				return lang.Target{Ecosystem: ecosystemGit, Package: dependency, Unresolved: true}, true
			}
		}
	}
	if r.files[q] || r.directories[q] {
		return lang.Target{Local: q}, true
	}
	return lang.Target{}, false
}

var versionSuffix = regexp.MustCompile(`^(.+)-v?[0-9][0-9A-Za-z.+_-]*$`)

// soldeerName names the dependency Soldeer installed into dependencies/<dir>:
// the declared or locked name the directory is "<name>-<version>" of.
func (p *project) soldeerName(directory string) string {
	best := ""
	for _, names := range []map[string]bool{keys(p.dependencies), keysLock(p.locked)} {
		for n := range names {
			if (directory == n || strings.HasPrefix(directory, n+"-")) && len(n) > len(best) {
				best = n
			}
		}
	}
	if best != "" {
		return best
	}
	if m := versionSuffix.FindStringSubmatch(directory); m != nil {
		return m[1]
	}
	return directory
}

func keys(m map[string]soldeerDependency) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

func keysLock(m map[string]lockEntry) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// soldeerTarget is a Soldeer dependency of the project. soldeer.lock pins it
// (the declared version as requested when it differs); without the lock an
// exact version pins, a git rev that is a commit pins, a tag is shown, and a
// branch, a version requirement or no version at all floats.
//
// Implements: REQ-SOLIDITY-007
func (p *project) soldeerTarget(name string) lang.Target {
	t := lang.Target{Ecosystem: ecosystemSoldeer, Package: name}
	d, declared := p.dependencies[name]
	e, locked := p.locked[name]
	switch {
	case locked:
		t.Version, t.Pinned = e.version, true
		if e.rev != "" {
			t.Version = e.rev
		}
		if t.Version == "" {
			t.Pinned, t.Floating = false, true
		}
		if declared && d.version != "" && d.version != t.Version {
			t.Requested = d.version
		}
		if e.git != "" && !lang.PublicForge(e.git) {
			t.Origin = e.git
		}
	case declared:
		switch {
		case d.rev != "":
			t.Version, t.Pinned = d.rev, lang.Commit(d.rev)
			if d.version != "" && d.version != d.rev {
				t.Requested = d.version
			}
		case d.tag != "":
			t.Version = d.tag
		case d.branch != "":
			t.Version, t.Floating = d.branch, true
		case d.git != "":
			t.Version, t.Floating = d.version, true
		default:
			t.Version, t.Pinned = d.version, lang.Pinned(d.version)
			t.Floating = !t.Pinned
		}
		switch {
		case d.git != "" && !lang.PublicForge(d.git):
			t.Origin = d.git
		case d.url != "":
			t.Origin = d.url
		}
	default:
		t.Unresolved = true
	}
	return t
}

// subTarget is a git submodule, named by its repository. The commit git
// records for it pins it (its .gitmodules branch as requested); without one,
// a branch or nothing at all floats.
//
// Implements: REQ-SOLIDITY-006
func subTarget(s *subReference) lang.Target {
	t := lang.Target{Ecosystem: ecosystemGit, Package: repositoryName(s.url, s.path)}
	switch {
	case s.commit != "":
		t.Version, t.Pinned, t.Requested = s.commit, true, s.branch
	case s.branch != "":
		t.Version, t.Floating = s.branch, true
	default:
		t.Floating = true
	}
	if s.url != "" && !lang.PublicForge(s.url) && !relativeURL(s.url) {
		t.Origin = s.url
	}
	return t
}

func relativeURL(u string) bool { return strings.HasPrefix(u, "./") || strings.HasPrefix(u, "../") }

// repositoryName names a submodule by its URL (lang.RepositoryName), lower-cased on the
// public forges, which ignore case: openzeppelin/openzeppelin-contracts and
// OpenZeppelin/openzeppelin-contracts are one repository. A relative URL
// names no repository, so its path's last element names it.
func repositoryName(url, p string) string {
	if url == "" || relativeURL(url) {
		return path.Base(p)
	}
	n := lang.RepositoryName(url)
	if lang.PublicForge(url) {
		n = strings.ToLower(n)
	}
	return n
}

// Dependencies answers for npm packages from the lock files, as the
// JavaScript resolver does, for a submodule that is checked out from its
// own .gitmodules, and for a Soldeer package from what it installed.
//
// Implements: REQ-SOLIDITY-006, REQ-SOLIDITY-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	switch t.Ecosystem {
	case ecosystemNPM:
		if r.npm != nil {
			return r.npm.Dependencies(t)
		}
	case ecosystemSoldeer:
		return r.soldeerDependencies(t)
	case ecosystemGit:
		var out []lang.Target
		seen := map[string]bool{}
		for _, s := range r.subs {
			if len(s.children) == 0 || repositoryName(s.url, s.path) != t.Package {
				continue
			}
			for _, c := range s.children {
				if d := subTarget(c); !seen[d.Package] {
					seen[d.Package] = true
					out = append(out, d)
				}
			}
		}
		return out
	}
	return nil
}

// Installed says a submodule's dependencies come from its checkout, and a Soldeer
// package's from what Soldeer installed.
func (r *resolver) Installed(t lang.Target) bool {
	return t.Ecosystem == ecosystemGit || t.Ecosystem == ecosystemSoldeer
}

// readInstalled reads what Soldeer installed into a dependencies/ directory:
// the foundry.toml [dependencies] and soldeer.lock each <name>-<version>/
// directory ships.
//
// Implements: REQ-SOLIDITY-007
func readInstalled(repository lang.Root, directory string) map[string]*project {
	entries, _ := repository.ReadDir(directory)
	out := map[string]*project{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subproject := &project{directory: e.Name(), dependencies: map[string]soldeerDependency{}, locked: map[string]lockEntry{}}
		read := func(name string) []byte {
			data, _ := repository.ReadBounded(filepath.Join(directory, e.Name(), name))
			return data
		}
		if c, ok := readFoundry(read("foundry.toml")); ok {
			for _, d := range c.dependencies {
				subproject.dependencies[d.name] = d
			}
		}
		for _, l := range readSoldeerLock(read("soldeer.lock")) {
			subproject.locked[l.name] = l
		}
		out[e.Name()] = subproject
	}
	return out
}

// soldeerDependencies is what the Soldeer package t, installed into a project's
// dependencies/<name>-<version>/ (the directory of t's version first), depends
// on: its own [dependencies] and soldeer.lock entries, pinned as the project
// pins them when it names them too, else as the package's own files do.
//
// Implements: REQ-SOLIDITY-007
func (r *resolver) soldeerDependencies(t lang.Target) []lang.Target {
	for _, d := range lang.SortedKeys(r.projects) {
		p := r.projects[d]
		subproject := p.installed[t.Package+"-"+t.Version]
		for _, directory := range lang.SortedKeys(p.installed) {
			if subproject != nil {
				break
			}
			if p.soldeerName(directory) == t.Package {
				subproject = p.installed[directory]
			}
		}
		if subproject == nil {
			continue
		}
		names := keys(subproject.dependencies)
		for n := range subproject.locked {
			names[n] = true
		}
		var out []lang.Target
		for _, n := range lang.SortedKeys(names) {
			_, declared := p.dependencies[n]
			if _, locked := p.locked[n]; declared || locked {
				out = append(out, p.soldeerTarget(n))
			} else {
				out = append(out, subproject.soldeerTarget(n))
			}
		}
		return out
	}
	return nil
}
