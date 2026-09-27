package solidity

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

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

var besideMemo sync.Map // absolute path of a marker file -> bool

func exists(abs string) bool {
	if v, ok := besideMemo.Load(abs); ok {
		return v.(bool)
	}
	_, err := os.Stat(abs)
	besideMemo.Store(abs, err == nil)
	return err == nil
}

// absRoot is the scan root of f: its absolute path without its relative one.
func absRoot(f *scan.File) (string, bool) {
	abs := filepath.ToSlash(f.Abs)
	if f.Abs == "" || !strings.HasSuffix(abs, f.Path) {
		return "", false
	}
	return strings.TrimSuffix(abs[:len(abs)-len(f.Path)], "/"), true
}

// ignored reports whether f lies in what a package manager installed or a
// build wrote: node_modules anywhere, and the directories of generated beside
// their tool's configuration.
//
// Implements: REQ-SOLIDITY-001
func ignored(f *scan.File) bool {
	segments := strings.Split(f.Path, "/")
	root, ok := absRoot(f)
	for i, s := range segments[:len(segments)-1] {
		if s == "node_modules" {
			return true
		}
		markers := generated[s]
		if len(markers) == 0 || !ok {
			continue
		}
		dir := path.Join(append([]string{root}, segments[:i]...)...)
		for _, m := range markers {
			if exists(path.Join(dir, m)) {
				return true
			}
		}
	}
	return false
}

// project is a directory with a foundry.toml, a hardhat.config.* or a
// remappings.txt: the base import paths and remappings are relative to.
type project struct {
	dir     string
	foundry bool
	config  foundryConfig
	remaps  []remapping // explicit first, then inferred
	deps    map[string]soldeerDep
	locked  map[string]lockEntry
}

func (p *project) soldeer() bool { return len(p.deps) > 0 || len(p.locked) > 0 }

// subRef is a submodule with its path relative to the scan root.
type subRef struct {
	submodule
	commit   string
	children []*subRef // the submodules of a checked-out submodule
}

type resolver struct {
	root     string
	files    map[string]bool
	dirs     map[string]bool
	projects map[string]*project
	subs     []*subRef // longest path first
	byPath   map[string]*subRef
	npm      *javascript.Packages
}

func join(dir, p string) string {
	if dir == "." || dir == "" {
		return path.Clean(p)
	}
	return path.Clean(dir + "/" + p)
}

func under(p, dir string) bool {
	return dir == "." || p == dir || strings.HasPrefix(p, dir+"/")
}

func readFile(f *scan.File) []byte {
	if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
		return nil
	}
	data, _ := os.ReadFile(f.Abs)
	return data
}

// newResolver reads the projects' foundry.toml, remappings.txt and
// soldeer.lock, the repository's .gitmodules (and the commits git records
// for them), and the package.json and lock files of npm.
//
// Implements: REQ-SOLIDITY-004, REQ-SOLIDITY-005, REQ-SOLIDITY-006, REQ-SOLIDITY-007, REQ-SOLIDITY-011
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{
		root: root, files: map[string]bool{}, dirs: map[string]bool{},
		projects: map[string]*project{}, byPath: map[string]*subRef{},
	}
	proj := func(dir string) *project {
		if p := r.projects[dir]; p != nil {
			return p
		}
		p := &project{dir: dir, deps: map[string]soldeerDep{}, locked: map[string]lockEntry{}}
		r.projects[dir] = p
		return p
	}
	npm := false
	var gitmodules []*scan.File
	txt := map[string][]string{}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
	}
	for _, f := range all {
		dir, base := path.Dir(f.Path), path.Base(f.Path)
		switch {
		case base == "package.json":
			npm = true
		case base == ".gitmodules" && !ignored(f):
			gitmodules = append(gitmodules, f)
		case base == "foundry.toml" && !ignored(f):
			p := proj(dir)
			p.foundry = true
			if c, ok := readFoundry(readFile(f)); ok {
				p.config = c
				for _, d := range c.deps {
					p.deps[d.name] = d
				}
			} else {
				p.config = foundryConfig{libs: []string{"lib"}, autoDetect: true}
			}
		case base == "remappings.txt" && !ignored(f):
			proj(dir)
			txt[dir], _ = remappingLines(readFile(f))
		case hardhatConfig(base) && !ignored(f):
			proj(dir)
		}
	}
	// soldeer.lock is read from disk: it sits beside foundry.toml.
	for dir, p := range r.projects {
		if !p.foundry {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(join(dir, "soldeer.lock")))); err == nil && len(data) <= lang.MaxParseSize {
			for _, e := range readSoldeerLock(data) {
				p.locked[e.name] = e
			}
		}
	}
	r.readSubmodules(gitmodules)
	if npm {
		r.npm = javascript.ReadPackages(all)
	}
	for dir, p := range r.projects {
		p.remaps = r.remappings(p, txt[dir])
	}
	return r
}

// readSubmodules reads each .gitmodules of the file list (and, when the scan
// did not list one, the root's), asks git for the commits recorded, and reads
// the .gitmodules of submodules that are checked out.
func (r *resolver) readSubmodules(gitmodules []*scan.File) {
	type source struct {
		dir  string // relative to the scan root
		data []byte
	}
	var sources []source
	for _, f := range gitmodules {
		sources = append(sources, source{path.Dir(f.Path), readFile(f)})
	}
	if len(sources) == 0 {
		if data, err := os.ReadFile(filepath.Join(r.root, ".gitmodules")); err == nil && len(data) <= lang.MaxParseSize {
			sources = append(sources, source{".", data})
		}
	}
	var add func(dir string, data []byte, parent *subRef, depth int)
	add = func(dir string, data []byte, parent *subRef, depth int) {
		subs := readGitmodules(data)
		var paths []string
		for _, s := range subs {
			paths = append(paths, s.path)
		}
		links := gitlinks(filepath.Join(r.root, filepath.FromSlash(dir)), paths)
		for _, s := range subs {
			ref := &subRef{submodule: s, commit: links[s.path]}
			ref.path = join(dir, s.path)
			if strings.HasPrefix(ref.path, "../") || r.byPath[ref.path] != nil {
				continue
			}
			r.byPath[ref.path] = ref
			r.subs = append(r.subs, ref)
			if parent != nil {
				parent.children = append(parent.children, ref)
			}
			if depth < maxNesting {
				nested := filepath.Join(r.root, filepath.FromSlash(ref.path), ".gitmodules")
				if data, err := os.ReadFile(nested); err == nil && len(data) <= lang.MaxParseSize {
					add(ref.path, data, ref, depth+1)
				}
			}
		}
	}
	for _, s := range sources {
		add(s.dir, s.data, nil, 0)
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
		if rm, ok := parseRemapping(s); ok && !have[rm.context+":"+rm.prefix] {
			have[rm.context+":"+rm.prefix] = true
			out = append(out, rm)
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
		names := make([]string, 0, len(p.deps)+len(p.locked))
		for n := range p.deps {
			names = append(names, n)
		}
		for n := range p.locked {
			if _, ok := p.deps[n]; !ok {
				names = append(names, n)
			}
		}
		sort.Strings(names)
		for _, n := range names {
			v := p.locked[n].version
			if v == "" {
				v = p.deps[n].version
			}
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
	for _, lib := range p.config.libs {
		if lib == "" || lib == "node_modules" {
			continue // npm packages are resolved as npm resolves them
		}
		for _, dep := range r.libraries(join(p.dir, lib)) {
			rel := lib + "/" + dep
			abs := r.abs(join(p.dir, rel))
			if isDir(filepath.Join(abs, "src")) {
				infer(dep+"/", rel+"/src/")
			} else {
				infer(dep+"/", rel+"/")
			}
			for _, x := range r.libraries(join(p.dir, rel+"/lib")) {
				nested = append(nested, [2]string{x, rel + "/lib/" + x})
			}
		}
	}
	for _, n := range nested {
		if isDir(filepath.Join(r.abs(join(p.dir, n[1])), "src")) {
			infer(n[0]+"/", n[1]+"/src/")
		} else {
			infer(n[0]+"/", n[1]+"/")
		}
	}
	return out
}

func (r *resolver) abs(rel string) string {
	return filepath.Join(r.root, filepath.FromSlash(rel))
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// libraries lists the libraries in a libs directory: the submodules git
// records there and the directories on disk (checked out or vendored).
func (r *resolver) libraries(dir string) []string {
	set := map[string]bool{}
	for _, s := range r.subs {
		if rest, ok := strings.CutPrefix(s.path, dir+"/"); ok && !strings.Contains(rest, "/") {
			set[rest] = true
		}
	}
	if entries, err := os.ReadDir(r.abs(dir)); err == nil {
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
	for d := path.Dir(file); ; d = path.Dir(d) {
		if p := r.projects[d]; p != nil {
			return p
		}
		if d == "." || d == "/" {
			return nil
		}
	}
}

// remap applies p's remappings to an import path of file the way solc does:
// the remapping with the longest context the file's path starts with, then
// the longest prefix, the first listed on a tie. The result is relative to
// the scan root.
//
// Implements: REQ-SOLIDITY-005
func (p *project) remap(file, spec string) (string, bool) {
	rel := file
	if p.dir != "." {
		rel = strings.TrimPrefix(file, p.dir+"/")
	}
	best := -1
	for i, rm := range p.remaps {
		if !strings.HasPrefix(spec, rm.prefix) || rm.context != "" && !strings.HasPrefix(rel, rm.context) {
			continue
		}
		if best < 0 || len(rm.context) > len(p.remaps[best].context) ||
			len(rm.context) == len(p.remaps[best].context) && len(rm.prefix) > len(p.remaps[best].prefix) {
			best = i
		}
	}
	if best < 0 {
		return "", false
	}
	rm := p.remaps[best]
	return join(p.dir, rm.target+spec[len(rm.prefix):]), true
}

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	dir := path.Dir(file)
	switch imp.Name {
	case kindRemap:
		rm, ok := parseRemapping(imp.Module)
		if !ok {
			return lang.Target{}
		}
		t, _ := r.located(join(dir, rm.target), r.projects[dir], file)
		return t
	case kindDep, kindLock:
		if p := r.projects[dir]; p != nil {
			return p.soldeerTarget(imp.Module)
		}
		return lang.Target{}
	case kindSubmodule:
		if s := r.byPath[join(dir, imp.Module)]; s != nil {
			return subTarget(s)
		}
		return lang.Target{}
	}
	return r.resolveImport(file, imp.Module)
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
	if p != nil && p.dir != "." {
		bases = []string{p.dir, "."}
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
		if r.dirs[join(b, first)] {
			return lang.Target{} // a missing file of the project's own directories
		}
	}
	if !strings.HasPrefix(name, "@") && p != nil && p.foundry {
		return lang.Target{Ecosystem: ecoGit, Package: first, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecoNPM, Package: name, Unresolved: true}
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
		if under(q, s.path) {
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
		return lang.Target{Ecosystem: ecoNPM, Package: name, Unresolved: true}, true
	}
	if p != nil && p.soldeer() {
		if rest, ok := strings.CutPrefix(q, join(p.dir, "dependencies")+"/"); ok {
			dir, _, _ := strings.Cut(rest, "/")
			return p.soldeerTarget(p.soldeerName(dir)), true
		}
	}
	if p != nil && p.foundry {
		for _, lib := range p.config.libs {
			if lib == "" || lib == "node_modules" {
				continue
			}
			if rest, ok := strings.CutPrefix(q, join(p.dir, lib)+"/"); ok {
				dep, _, _ := strings.Cut(rest, "/")
				return lang.Target{Ecosystem: ecoGit, Package: dep, Unresolved: true}, true
			}
		}
	}
	if r.files[q] || r.dirs[q] {
		return lang.Target{Local: q}, true
	}
	return lang.Target{}, false
}

var versionSuffix = regexp.MustCompile(`^(.+)-v?[0-9][0-9A-Za-z.+_-]*$`)

// soldeerName names the dependency Soldeer installed into dependencies/<dir>:
// the declared or locked name the directory is "<name>-<version>" of.
func (p *project) soldeerName(dir string) string {
	best := ""
	for _, names := range []map[string]bool{keys(p.deps), keysLock(p.locked)} {
		for n := range names {
			if (dir == n || strings.HasPrefix(dir, n+"-")) && len(n) > len(best) {
				best = n
			}
		}
	}
	if best != "" {
		return best
	}
	if m := versionSuffix.FindStringSubmatch(dir); m != nil {
		return m[1]
	}
	return dir
}

func keys(m map[string]soldeerDep) map[string]bool {
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
	t := lang.Target{Ecosystem: ecoSoldeer, Package: name}
	d, declared := p.deps[name]
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
		if e.git != "" && !public(e.git) {
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
		case d.git != "" && !public(d.git):
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
func subTarget(s *subRef) lang.Target {
	t := lang.Target{Ecosystem: ecoGit, Package: repoName(s.url, s.path)}
	switch {
	case s.commit != "":
		t.Version, t.Pinned, t.Requested = s.commit, true, s.branch
	case s.branch != "":
		t.Version, t.Floating = s.branch, true
	default:
		t.Floating = true
	}
	if s.url != "" && !public(s.url) && !relativeURL(s.url) {
		t.Origin = s.url
	}
	return t
}

func relativeURL(u string) bool { return strings.HasPrefix(u, "./") || strings.HasPrefix(u, "../") }

// repoName names a submodule by its URL (lang.RepoName), lower-cased on the
// public forges, which ignore case: openzeppelin/openzeppelin-contracts and
// OpenZeppelin/openzeppelin-contracts are one repository. A relative URL
// names no repository, so its path's last element names it.
func repoName(url, p string) string {
	if url == "" || relativeURL(url) {
		return path.Base(p)
	}
	n := lang.RepoName(url)
	if public(url) {
		n = strings.ToLower(n)
	}
	return n
}

// public reports whether a git URL is on a public forge, whose repositories
// are named, not origins.
func public(url string) bool {
	host, _, _ := strings.Cut(lang.RepoName(url), "/")
	switch host {
	case "github.com", "gitlab.com", "bitbucket.org", "codeberg.org", "git.sr.ht", "sr.ht":
		return true
	}
	return false
}

// Dependencies answers for npm packages from the lock files, as the
// JavaScript resolver does, and for a submodule that is checked out from its
// own .gitmodules.
//
// Implements: REQ-SOLIDITY-006, REQ-SOLIDITY-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	switch t.Ecosystem {
	case ecoNPM:
		if r.npm != nil {
			return r.npm.Dependencies(t)
		}
	case ecoGit:
		var out []lang.Target
		seen := map[string]bool{}
		for _, s := range r.subs {
			if len(s.children) == 0 || repoName(s.url, s.path) != t.Package {
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

// Installed says a submodule's dependencies come from its checkout.
func (r *resolver) Installed(t lang.Target) bool { return t.Ecosystem == ecoGit }
