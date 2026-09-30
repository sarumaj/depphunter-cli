// Package cocoapods reads the dependency managers of Apple platform projects that
// are not SwiftPM: CocoaPods (Podfile, Podfile.lock, podspecs) and Carthage
// (Cartfile, Cartfile.resolved). It is shared by the objc plugin, whose headers and
// modules come from pods, and the swift plugin, whose `import Alamofire` may too.
//
// A pod is named by its root name (the subspec "Firebase/Analytics" is Firebase)
// and a Carthage dependency by its repository URL as lang.RepositoryName spells it
// (github.com/Alamofire/Alamofire), as the swiftpm island names packages.
package cocoapods

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Ecosystem ids.
const (
	Ecosystem = "cocoapods"
	Carthage  = "carthage"
)

// Ecosystems are the islands pods and Carthage dependencies land on, as every plugin
// using this package declares them.
func Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: Ecosystem, Name: "CocoaPods"},
		{ID: Carthage, Name: "Carthage"},
	}
}

// Kind is what a manifest file is, by its name: "podfile", "podspec",
// "podspec.json", "cartfile", or "" for none.
func Kind(p string) string {
	base := path.Base(p)
	switch {
	case base == "Podfile":
		return "podfile"
	case strings.HasSuffix(base, ".podspec"):
		return "podspec"
	case strings.HasSuffix(base, ".podspec.json"):
		return "podspec.json"
	case base == "Cartfile", base == "Cartfile.private":
		return "cartfile"
	}
	return ""
}

// Dependencies reads the dependency lines of a manifest of the given kind, for the plugin's
// imports: a Podfile's pods, a podspec's dependencies, a Cartfile's entries.
//
// Implements: REQ-OBJC-007, REQ-OBJC-010, REQ-OBJC-011
func Dependencies(kind string, source []byte) []Dependency {
	switch kind {
	case "podfile":
		return readPodfile(string(source)).dependencies
	case "podspec":
		return readPodspec(string(source)).lines
	case "podspec.json":
		return readPodspecJSON(string(source)).lines
	case "cartfile":
		var out []Dependency
		for _, c := range readCartfile(string(source)) {
			out = append(out, Dependency{Spec: c.kind + ` "` + c.source + `"`, Name: CartfileModule(c.kind, c.source), Line: c.line})
		}
		return out
	}
	return nil
}

// CartfileModule is how a Cartfile dependency travels in RawImport.Module:
// "carthage:" and its package name.
func CartfileModule(kind, source string) string { return "carthage:" + cartName(kind, source) }

// cartName names a Carthage dependency: `github "owner/repo"` is
// github.com/owner/repo, a URL (git, binary, a GitHub Enterprise github) is itself
// as lang.RepositoryName spells it.
func cartName(kind, source string) string {
	if kind == "github" && !strings.Contains(source, "://") && strings.Count(source, "/") == 1 {
		return "github.com/" + source
	}
	return lang.RepositoryName(source)
}

// project is what one directory's manifests say.
type project struct {
	directory string
	podfile   bool
	declared  map[string]*declaration // root name -> the first declaration
	locks     map[string]*locked      // root name -> Podfile.lock
	carts     map[string]*cart        // package -> Cartfile entry
	pins      map[string]*cart        // package -> Cartfile.resolved entry
}

// Index is what the project's CocoaPods and Carthage manifests say, per directory.
type Index struct {
	root        string
	projects    []*project        // shallowest first
	own         map[string]string // pod name built here (podspec name, module name) -> podspec path
	directories map[string]bool
	files       map[string]bool
}

// Read reads the manifests among the scanned files; a Podfile.lock or
// Cartfile.resolved the scan left out (git-ignored) is read from disk beside its
// Podfile or Cartfile.
//
// Implements: REQ-OBJC-007, REQ-OBJC-008, REQ-OBJC-010, REQ-OBJC-011
func Read(root string, all []*scan.File) *Index {
	x := &Index{root: root, own: map[string]string{}, directories: map[string]bool{}, files: map[string]bool{}}
	byDirectory := map[string]*project{}
	get := func(directory string) *project {
		p := byDirectory[directory]
		if p == nil {
			p = &project{directory: directory, declared: map[string]*declaration{}, locks: map[string]*locked{},
				carts: map[string]*cart{}, pins: map[string]*cart{}}
			byDirectory[directory] = p
		}
		return p
	}
	sorted := append([]*scan.File(nil), all...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	repository := lang.NewSource(root, lang.SourceOptions{Bounded: true})
	for _, f := range sorted {
		repository.Add(f)
		x.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !x.directories[d]; d = path.Dir(d) {
			x.directories[d] = true
		}
	}
	read := func(relative string) (string, bool) {
		data, ok := repository.Read(relative)
		return string(data), ok
	}
	for _, f := range sorted {
		if f.Binary || vendored(f.Path) {
			continue
		}
		directory := path.Dir(f.Path)
		switch Kind(f.Path) {
		case "podfile":
			source, _ := read(f.Path)
			p := get(directory)
			p.podfile = true
			for _, d := range readPodfile(source).pods {
				p.declare(d)
			}
			if lock, ok := read(path.Join(directory, "Podfile.lock")); ok {
				p.locks, _ = readLock([]byte(lock))
				if p.locks == nil {
					p.locks = map[string]*locked{}
				}
			}
		case "podspec", "podspec.json":
			source, _ := read(f.Path)
			spec := readPodspec(source)
			if Kind(f.Path) == "podspec.json" {
				spec = readPodspecJSON(source)
			}
			if spec.name == "" {
				continue
			}
			for _, n := range []string{spec.name, spec.module} {
				if n != "" {
					if _, duplicate := x.own[n]; !duplicate {
						x.own[n] = f.Path
					}
				}
			}
			p := get(directory)
			for _, d := range spec.dependencies {
				if Root(d.name) != spec.name {
					p.declare(d)
				}
			}
		case "cartfile":
			source, _ := read(f.Path)
			p := get(directory)
			for _, c := range readCartfile(source) {
				if _, ok := p.carts[c.name]; !ok {
					c := c
					p.carts[c.name] = &c
				}
			}
			if lock, ok := read(path.Join(directory, "Cartfile.resolved")); ok {
				for _, c := range readCartfile(lock) {
					c := c
					p.pins[c.name] = &c
				}
			}
		}
	}
	for _, p := range byDirectory {
		x.projects = append(x.projects, p)
	}
	sort.Slice(x.projects, func(i, j int) bool { return lang.ShallowestFirst(x.projects[i].directory, x.projects[j].directory) })
	return x
}

func (p *project) declare(d *declaration) {
	if _, ok := p.declared[Root(d.name)]; !ok {
		p.declared[Root(d.name)] = d
	}
}

// vendored reports whether a path is inside a dependency checkout: CocoaPods' Pods
// directory, Carthage's Checkouts and Build.
func vendored(p string) bool {
	for _, segment := range strings.Split(path.Dir(p), "/") {
		if segment == "Pods" || segment == "Carthage" {
			return true
		}
	}
	return false
}

// projectsOf lists the manifest directories over a file, nearest first; a file under
// none takes every one, shallowest first.
func (x *Index) projectsOf(file string) []*project {
	var out []*project
	for i := len(x.projects) - 1; i >= 0; i-- {
		if p := x.projects[i]; p.directory == "." || strings.HasPrefix(file, p.directory+"/") {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return x.projects
	}
	return out
}

// Governed reports whether any CocoaPods or Carthage manifest covers a file, which
// makes a framework header no manifest names an unresolved pod rather than an
// unknown C library.
func (x *Index) Governed(file string) bool {
	for _, p := range x.projectsOf(file) {
		if p.podfile || len(p.declared) > 0 || len(p.locks) > 0 {
			return true
		}
	}
	return false
}

// Pod is the target of a pod a manifest over file names (a Podfile's `pod` line, a
// podspec's dependency): pinned by the nearest Podfile.lock holding it, else as the
// nearest declaration asks, a path pod being its directory or podspec. A pod the
// repository builds itself, named from its own podspec, is dropped.
//
// Implements: REQ-OBJC-009
func (x *Index) Pod(file, name string) lang.Target {
	root := Root(name)
	if spec, ok := x.own[root]; ok && strings.Contains(Kind(file), "podspec") && path.Dir(spec) == path.Dir(file) {
		return lang.Target{} // a podspec's own subspec
	}
	return x.target(file, x.projectsOf(file), root)
}

// target resolves a root pod name in the given projects.
func (x *Index) target(file string, projects []*project, root string) lang.Target {
	var d *declaration
	var dp *project
	for _, p := range projects {
		if dd, ok := p.declared[root]; ok {
			d, dp = dd, p
			break
		}
	}
	var l *locked
	var localProject *project
	for _, p := range projects {
		if ll, ok := p.locks[root]; ok {
			l, localProject = ll, p
			break
		}
	}
	if d != nil && d.path != "" {
		return x.local(path.Join(dp.directory, d.path), root)
	}
	if l != nil && l.path != "" {
		return x.local(path.Join(localProject.directory, l.path), root)
	}
	t := lang.Target{Ecosystem: Ecosystem, Package: root}
	switch {
	case l != nil:
		t.Version, t.Pinned = l.version, l.version != ""
		git, tag, branch, commit := l.git, l.tag, l.branch, l.commit
		if git == "" && d != nil {
			git, tag, branch, commit = d.git, d.tag, d.branch, d.commit
		}
		switch {
		case git != "":
			t.Origin = git
			gitPin(&t, tag, branch, commit)
		case l.podspec != "" || (d != nil && d.podspec != ""):
			t.Origin = "podspec:" + path.Join(localProject.directory, first(l.podspec, podspecOf(d)))
		}
		if d != nil && d.requirements != "" && d.requirements != t.Version {
			if v, ok := exact(d.requirements); !ok || v != t.Version {
				t.Requested = d.requirements
			}
		}
	case d != nil:
		switch {
		case d.git != "":
			t.Origin = d.git
			gitPin(&t, d.tag, d.branch, d.commit)
		case d.podspec != "":
			t.Origin = "podspec:" + path.Join(dp.directory, d.podspec)
			requirement(&t, d.requirements)
		default:
			requirement(&t, d.requirements)
		}
	default:
		if spec, ok := x.own[root]; ok {
			return lang.Target{Local: spec}
		}
		t.Unresolved = true
	}
	return t
}

func podspecOf(d *declaration) string {
	if d == nil {
		return ""
	}
	return d.podspec
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// gitPin applies the git rule: a commit pins, a tag is shown but moves when its
// owner moves it (neither pinned nor floating), a branch or nothing floats.
//
// Implements: REQ-OBJC-009
func gitPin(t *lang.Target, tag, branch, commit string) {
	switch {
	case commit != "":
		t.Version, t.Pinned, t.Floating = commit, true, false
	case tag != "":
		t.Version, t.Pinned, t.Floating = tag, false, false
	case branch != "":
		t.Version, t.Pinned, t.Floating = branch, false, true
	default:
		t.Pinned, t.Floating = false, true
	}
}

// requirement applies a declared requirement: one exact version ("1.2.3", "= 1.2.3")
// pins and is shown bare; an optimistic or open range ("~> 1.2", ">= 1.0") floats;
// none floats.
//
// Implements: REQ-OBJC-009
func requirement(t *lang.Target, requirements string) {
	if v, ok := exact(requirements); ok {
		t.Version, t.Pinned = v, true
		return
	}
	t.Version, t.Floating = requirements, true
}

// exact reads a requirement naming one version.
func exact(requirement string) (string, bool) {
	v := strings.TrimSpace(requirement)
	if strings.Contains(v, ",") {
		return "", false
	}
	if rest, ok := strings.CutPrefix(v, "="); ok {
		v = strings.TrimSpace(rest)
	}
	return v, v != "" && lang.Pinned(v)
}

// local is a path pod: its podspec (Name.podspec or .podspec.json) when the
// directory has one, else the directory; nothing when neither is in the project.
func (x *Index) local(p, root string) lang.Target {
	p = path.Clean(p)
	if strings.HasPrefix(p, "../") || p == ".." || path.IsAbs(p) {
		return lang.Target{}
	}
	if x.files[p] {
		return lang.Target{Local: p}
	}
	for _, name := range []string{root + ".podspec", root + ".podspec.json"} {
		if f := path.Join(p, name); x.files[f] {
			return lang.Target{Local: f}
		}
	}
	if x.directories[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// Dependencies implements lang.Transitive from Podfile.lock: what each pod's specs
// depend on, pinned by the same lock; and from Carthage's checkouts.
//
// Implements: REQ-OBJC-008
func (x *Index) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem == Carthage {
		return x.checkedOut(t)
	}
	if t.Ecosystem != Ecosystem {
		return nil
	}
	for _, p := range x.projects {
		l, ok := p.locks[t.Package]
		if !ok || l.version == "" {
			continue
		}
		var out []lang.Target
		for _, dependency := range l.dependencies {
			if d := x.target("", []*project{p}, dependency); d.Package != "" {
				out = append(out, d)
			}
		}
		return out
	}
	return nil
}

// checkedOut is what a Carthage dependency checked out into
// Carthage/Checkouts/<name>/ beside the Cartfile naming it (a binary one has no
// checkout) depends on: the entries of its own Cartfile (Carthage leaves a
// dependency's Cartfile.private out), else of its Cartfile.resolved. The
// project's Cartfile.resolved pins those it lists, as Carthage resolves the whole
// graph into it; the checkout's pins the rest.
//
// Implements: REQ-OBJC-011
func (x *Index) checkedOut(t lang.Target) []lang.Target {
	if x.root == "" {
		return nil
	}
	for _, p := range x.projects {
		c := p.carts[t.Package]
		if c == nil {
			c = p.pins[t.Package]
		}
		if c == nil {
			continue
		}
		directory := filepath.Join(x.root, filepath.FromSlash(p.directory), "Carthage", "Checkouts",
			strings.TrimSuffix(path.Base(strings.TrimSuffix(c.source, "/")), ".git"))
		own := &project{carts: map[string]*cart{}, pins: map[string]*cart{}}
		read := func(name string, into map[string]*cart) bool {
			source, ok := lang.ReadCapped(filepath.Join(directory, name))
			if !ok {
				return false
			}
			for _, c := range readCartfile(string(source)) {
				c := c
				into[c.name] = &c
			}
			return true
		}
		resolved, declared := read("Cartfile.resolved", own.pins), read("Cartfile", own.carts)
		if !resolved && !declared {
			continue
		}
		names := own.carts
		if !declared {
			names = own.pins
		}
		var out []lang.Target
		for _, n := range sortedCarts(names) {
			if _, ok := p.pins[n]; ok {
				out = append(out, p.cartTarget(n))
			} else {
				out = append(out, own.cartTarget(n))
			}
		}
		return out
	}
	return nil
}

func sortedCarts(m map[string]*cart) []string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Installed reports that a Carthage dependency's dependencies come from its
// checkout.
func (x *Index) Installed(t lang.Target) bool { return t.Ecosystem == Carthage }

// Module attributes a module or a framework header's directory (<AFNetworking/..>,
// `@import Firebase;`, Swift's `import Alamofire`) to a pod or a Carthage dependency
// the manifests over file declare or lock, by name: the pod's own name, its module
// spelling (libPhoneNumber-iOS is libPhoneNumber_iOS), the module name a known pod
// goes by (GRDB is GRDB.swift's), or its name without a platform suffix (lottie-ios
// is Lottie). A Carthage dependency is matched by its repository's name.
//
// Implements: REQ-OBJC-006, REQ-OBJC-011, REQ-OBJC-012
func (x *Index) Module(file, module string) (lang.Target, bool) {
	if module == "" {
		return lang.Target{}, false
	}
	projects := x.projectsOf(file)
	want := fold(module)
	names := append([]string{module}, moduleAliases[module]...)
	for _, p := range projects {
		for _, n := range names {
			if p.has(n) {
				return x.target(file, projects, n), true
			}
		}
	}
	for _, p := range projects {
		for _, n := range p.names() {
			if podMatches(n, want) {
				return x.target(file, projects, n), true
			}
		}
	}
	for _, p := range projects {
		if c := p.cart(want); c != "" {
			return p.cartTarget(c), true
		}
	}
	if spec, ok := x.own[module]; ok {
		return lang.Target{Local: spec}, true
	}
	return lang.Target{}, false
}

func (p *project) has(root string) bool {
	_, d := p.declared[root]
	_, l := p.locks[root]
	return d || l
}

// names are the root pods a project declares or locks, sorted.
func (p *project) names() []string {
	seen := map[string]bool{}
	var out []string
	for n := range p.declared {
		seen[n] = true
		out = append(out, n)
	}
	for n := range p.locks {
		if !seen[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// podMatches reports whether a pod goes by a (folded) module name.
func podMatches(pod, want string) bool {
	f := fold(pod)
	return f == want || trimPlatform(f) == want || f == "lib"+want
}

// trimPlatform takes the suffix off a folded pod name that says which language or
// platform it is for: GRDB.swift, lottie-ios, ReactiveObjC's kin.
func trimPlatform(f string) string {
	for _, suffix := range []string{"swift", "ios", "objc", "osx"} {
		if t := strings.TrimSuffix(f, suffix); t != f && len(t) >= 3 {
			return t
		}
	}
	return f
}

// fold is the key pods and modules are matched by, under the name the tests use.
func fold(s string) string { return lang.FoldAlphanumeric(s) }

// cart finds the Carthage dependency (declared or resolved) whose repository is
// named like a module.
func (p *project) cart(want string) string {
	var names []string
	for n := range p.carts {
		names = append(names, n)
	}
	for n := range p.pins {
		if _, ok := p.carts[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		base := fold(strings.TrimSuffix(path.Base(n), ".json"))
		if base == want || trimPlatform(base) == want {
			return n
		}
	}
	return ""
}

// cartTarget applies Carthage's pins: Cartfile.resolved fixes the version; else
// `== 1.2` pins, `~>` and `>=` float, a quoted git reference pins when it is a
// commit and is shown otherwise, and nothing floats.
//
// Implements: REQ-OBJC-011
func (p *project) cartTarget(name string) lang.Target {
	t := lang.Target{Ecosystem: Carthage, Package: name}
	c := p.carts[name]
	if pin, ok := p.pins[name]; ok && pin.requirement != "" {
		t.Version, t.Pinned = pin.requirement, true
		if c != nil && c.requirement != "" && c.requirement != pin.requirement {
			t.Requested = c.requirement
		}
		return t
	}
	if c == nil || c.requirement == "" {
		t.Floating = true
		return t
	}
	switch {
	case c.reference && lang.Commit(c.requirement):
		t.Version, t.Pinned = c.requirement, true
	case c.reference:
		t.Version = c.requirement
	case strings.HasPrefix(c.requirement, "=="):
		t.Version = strings.TrimSpace(strings.TrimPrefix(c.requirement, "=="))
		t.Pinned = lang.Pinned(t.Version)
		t.Floating = !t.Pinned
	default:
		t.Version, t.Floating = c.requirement, true
	}
	return t
}

// Cart is the target of a Cartfile entry, by its package name, in the projects over
// file.
func (x *Index) Cart(file, name string) lang.Target {
	for _, p := range x.projectsOf(file) {
		if _, ok := p.carts[name]; ok {
			return p.cartTarget(name)
		}
		if _, ok := p.pins[name]; ok {
			return p.cartTarget(name)
		}
	}
	return lang.Target{Ecosystem: Carthage, Package: name, Floating: true}
}

// Vendored attributes a project file inside a dependency checkout to its
// dependency: Pods/<Name>/..., Pods/Headers/Public/<Name>/... are the pod Name,
// Carthage/Checkouts/<repo>/... and Carthage/Build/.../<Name>.framework/... the
// Carthage dependency of that name. A header found there is the pod's, not the
// project's own code.
//
// Implements: REQ-OBJC-006
func (x *Index) Vendored(file, local string) (lang.Target, bool) {
	segments := strings.Split(local, "/")
	for i, s := range segments {
		switch s {
		case "Pods":
			if i+1 >= len(segments)-1 {
				return lang.Target{}, false
			}
			name := segments[i+1]
			if name == "Headers" && i+3 < len(segments)-1 {
				name = segments[i+3] // Headers/Public/<Name>/x.h
			}
			if name == "Headers" || name == "Target Support Files" || strings.HasPrefix(name, "Pods") || strings.HasSuffix(name, ".xcodeproj") {
				return lang.Target{}, false
			}
			return x.target(file, x.projectsOf(file), name), true
		case "Carthage":
			name := ""
			if i+2 < len(segments) && segments[i+1] == "Checkouts" {
				name = segments[i+2]
			} else {
				for _, s := range segments[i+1 : len(segments)-1] {
					if n, ok := strings.CutSuffix(s, ".framework"); ok {
						name = n
					} else if n, ok := strings.CutSuffix(s, ".xcframework"); ok {
						name = n
					}
				}
			}
			if name == "" || i+2 >= len(segments) {
				return lang.Target{}, false
			}
			for _, p := range x.projectsOf(file) {
				if c := p.cart(fold(name)); c != "" {
					return p.cartTarget(c), true
				}
			}
			return lang.Target{Ecosystem: Carthage, Package: name, Unresolved: true}, true
		}
	}
	return lang.Target{}, false
}
