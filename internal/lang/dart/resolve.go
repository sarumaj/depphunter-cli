package dart

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// flutterSDK are the packages Flutter's SDK ships: a project depends on them with
// `sdk: flutter`, and no index serves them.
//
// Implements: REQ-DART-005
var flutterSDK = map[string]bool{
	"flutter": true, "flutter_test": true, "flutter_localizations": true, "flutter_driver": true,
	"flutter_web_plugins": true, "integration_test": true, "sky_engine": true,
}

// pubPackage is a pub package of the repository.
type pubPackage struct {
	directory     string
	spec          *pubspec
	dependencies  map[string]*dependency // by name; an override replaces the declaration
	lock          map[string]*locked     // its own pubspec.lock, or its workspace's
	lockDirectory string
	root          *pubPackage // the pub workspace this package is a member of
	melos         []*melosRepository
}

// melosRepository is a melos repository: the packages its globs select are linked to each
// other by `melos bootstrap`, whatever versions their pubspecs ask for.
type melosRepository struct {
	directory string
	include   []string
	exclude   []string
}

type resolver struct {
	lang.Layout
	packages []*pubPackage // deepest first
	lang.NoteList
}

// The notes a resolver keeps reach --explain only through lang.Noter.
var _ lang.Noter = (*resolver)(nil)

// Implements: REQ-DART-009
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{Layout: lang.NewLayout()}
	// pubspec.lock and pubspec_overrides.yaml are git-ignored as often as not; what is
	// on disk beside a pubspec is what pub resolved with.
	repository := lang.NewSource(root)
	pubspecs, melos := r.indexFiles(repository, all)
	for _, f := range pubspecs {
		r.readPackage(repository, f)
	}
	r.linkWorkspaces()
	r.linkMelos(repository, melos)
	sort.SliceStable(r.packages, func(i, j int) bool { return lang.Depth(r.packages[i].directory) > lang.Depth(r.packages[j].directory) })
	return r
}

func (r *resolver) indexFiles(repository *lang.Source, all []*scan.File) (pubspecs, melos []string) {
	for _, f := range all {
		r.Add(f.Path)
		repository.Add(f)
		if (Plugin{}).Claims(f) && path.Base(f.Path) == "pubspec.yaml" {
			pubspecs = append(pubspecs, f.Path)
		}
		if path.Base(f.Path) == "melos.yaml" {
			melos = append(melos, f.Path)
		}
	}
	sort.Strings(pubspecs)
	sort.Strings(melos)
	return pubspecs, melos
}

// readPackage reads a pubspec.yaml with the pubspec_overrides.yaml and pubspec.lock
// beside it. An override replaces a declaration, dependency_overrides beat
// dependencies, and dependencies beat dev_dependencies.
//
// Implements: REQ-DART-006
func (r *resolver) readPackage(repository *lang.Source, file string) {
	source, _ := repository.Read(file)
	spec, err := readPubspec(source)
	if err != nil {
		return
	}
	p := &pubPackage{directory: path.Dir(file), spec: spec, dependencies: map[string]*dependency{}}
	for _, section := range []string{"dev_dependencies", "dependencies", "dependency_overrides"} {
		for _, d := range spec.dependencies {
			if d.section == section {
				p.dependencies[d.name] = d
			}
		}
	}
	if source, ok := repository.Read(path.Join(p.directory, "pubspec_overrides.yaml")); ok {
		if o, err := readPubspec(source); err == nil {
			for _, d := range o.dependencies {
				p.dependencies[d.name] = d
			}
		}
	}
	r.readLock(repository, p)
	r.packages = append(r.packages, p)
}

// Implements: REQ-DART-007, REQ-TRC-017
func (r *resolver) readLock(repository *lang.Source, p *pubPackage) {
	source, ok := repository.Read(path.Join(p.directory, "pubspec.lock"))
	if !ok {
		return
	}
	p.lock, p.lockDirectory = readLock(source), p.directory
	lock := path.Join(p.directory, "pubspec.lock")
	if len(p.lock) == 0 {
		return
	}
	if !repository.Listed(lock) {
		r.NoteIgnored(lock)
	}
	r.Note(lock, trace.NoteFlat, "pubspec.lock pins versions but records no edges: offline, "+
		"--resolve-depth adds nothing past the packages it pins (--online asks pub)")
}

// linkWorkspaces links a pub workspace: the root lists its members, which resolve
// together against the root's pubspec.lock.
//
// Implements: REQ-DART-004
func (r *resolver) linkWorkspaces() {
	for _, p := range r.packages {
		for _, m := range p.spec.workspace {
			r.linkMembers(p, strings.TrimSuffix(path.Join(p.directory, m), "/"))
		}
	}
}

func (r *resolver) linkMembers(root *pubPackage, glob string) {
	for _, q := range r.packages {
		if q == root || q.root != nil || !globMatch(glob, q.directory) {
			continue
		}
		q.root = root
		if q.lock == nil {
			q.lock, q.lockDirectory = root.lock, root.lockDirectory
		}
	}
}

// linkMelos links every package to the melos repositories selecting it: melos.yaml's
// packages globs, or melos 7's `melos:` section in the root pubspec (whose packages
// are then its workspace, already read).
//
// Implements: REQ-DART-004
func (r *resolver) linkMelos(repository *lang.Source, melos []string) {
	var repositories []*melosRepository
	for _, f := range melos {
		source, _ := repository.Read(f)
		var doc struct {
			Packages []string `yaml:"packages"`
			Ignore   []string `yaml:"ignore"`
		}
		if yaml.Unmarshal(source, &doc) == nil && len(doc.Packages) > 0 {
			repositories = append(repositories, &melosRepository{directory: path.Dir(f), include: doc.Packages, exclude: doc.Ignore})
		}
	}
	for _, p := range r.packages {
		if p.spec.melos && len(p.spec.melosPackages) > 0 {
			repositories = append(repositories, &melosRepository{directory: p.directory, include: p.spec.melosPackages})
		}
	}
	for _, m := range repositories {
		for _, p := range r.packages {
			if m.has(p.directory) {
				p.melos = append(p.melos, m)
			}
		}
	}
}

// has reports whether a package directory is one of the repository's.
func (m *melosRepository) has(directory string) bool {
	match := func(globs []string) bool {
		for _, g := range globs {
			if globMatch(strings.TrimSuffix(path.Join(m.directory, g), "/"), directory) {
				return true
			}
		}
		return false
	}
	return match(m.include) && !match(m.exclude)
}

// globMatch matches a slash-separated path against a glob where `**` stands for any
// number of directories.
func globMatch(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(p, n []string) bool {
	if len(p) == 0 {
		return len(n) == 0
	}
	if p[0] == "**" {
		for i := 0; i <= len(n); i++ {
			if matchSegments(p[1:], n[i:]) {
				return true
			}
		}
		return false
	}
	if len(n) == 0 {
		return false
	}
	ok, _ := path.Match(p[0], n[0])
	return ok && matchSegments(p[1:], n[1:])
}

// packageOf is the package a file belongs to: the nearest pubspec.yaml above it.
func (r *resolver) packageOf(file string) *pubPackage {
	for _, p := range r.packages {
		if lang.Within(file, p.directory) {
			return p
		}
	}
	return nil
}

// Implements: REQ-DART-002, REQ-DART-004, REQ-DART-005, REQ-DART-006, REQ-DART-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, _, _ := strings.Cut(rawImport.Name, ":")
	switch kind {
	case kindMember:
		if f := path.Join(path.Dir(file), rawImport.Module, "pubspec.yaml"); r.Files[f] {
			return lang.Target{Local: f}
		}
		return lang.Target{}
	case kindDependency:
		p := r.packageOf(file)
		if directory, ok := r.localPackage(p, rawImport.Module); ok {
			if f := path.Join(directory, "pubspec.yaml"); r.Files[f] {
				return lang.Target{Local: f}
			}
			return r.localDirectory(directory)
		}
		return r.external(p, rawImport.Module)
	}
	u := rawImport.Module
	if rest, ok := strings.CutPrefix(u, "dart:"); ok {
		return lang.Target{Ecosystem: ecosystemStd, Package: "dart:" + rest}
	}
	if rest, ok := strings.CutPrefix(u, "package:"); ok {
		name, subpath, _ := strings.Cut(rest, "/")
		p := r.packageOf(file)
		if directory, ok := r.localPackage(p, name); ok {
			return r.library(directory, subpath)
		}
		if name == "flutter_gen" {
			return lang.Target{} // Flutter's synthetic package, generated under .dart_tool
		}
		return r.external(p, name)
	}
	if strings.Contains(u, ":") || strings.HasPrefix(u, "/") {
		return lang.Target{} // file:, http: - nothing the repository holds
	}
	if f := path.Join(path.Dir(file), u); lang.Inside(f) && r.Files[f] {
		return lang.Target{Local: f}
	}
	return lang.Target{} // a generated part (x.g.dart) that is not committed
}

// library is the file a package: URI names in a local package's lib/ directory, or when
// it is missing (generated, not committed), the directory itself.
func (r *resolver) library(directory, subpath string) lang.Target {
	if f := path.Join(directory, "lib", subpath); r.Files[f] {
		return lang.Target{Local: f}
	}
	if t := r.localDirectory(path.Join(directory, "lib")); t.Local != "" {
		return t
	}
	if f := path.Join(directory, "pubspec.yaml"); r.Files[f] {
		return lang.Target{Local: f}
	}
	return r.localDirectory(directory)
}

func (r *resolver) localDirectory(directory string) lang.Target {
	if r.Directories[directory] {
		return lang.Target{Local: directory}
	}
	return lang.Target{}
}

// localPackage finds the directory of a package the project builds from source: the
// importing package itself, a path dependency (or override, pubspec_overrides.yaml
// included), a path package of the lock, a member of the same pub workspace, or a
// package of the same melos repository. Any other package comes from pub, even when
// the repository holds one of that name.
//
// Implements: REQ-DART-004
func (r *resolver) localPackage(p *pubPackage, name string) (string, bool) {
	if p == nil {
		return "", false
	}
	if p.spec.name == name {
		return p.directory, true
	}
	for _, q := range []*pubPackage{p, p.root} {
		if q == nil {
			continue
		}
		if d := q.dependencies[name]; d != nil && d.source == "path" {
			if directory := path.Join(q.directory, d.path); lang.Inside(directory) {
				return directory, true
			}
		}
	}
	if l := p.lock[name]; l != nil && l.source == "path" && l.relative {
		if directory := path.Join(p.lockDirectory, filepath.ToSlash(l.path)); lang.Inside(directory) {
			return directory, true
		}
	}
	root := p.root
	if root == nil && len(p.spec.workspace) > 0 {
		root = p
	}
	for _, q := range r.packages {
		if root != nil && (q == root || q.root == root) && q.spec.name == name {
			return q.directory, true
		}
	}
	for _, m := range p.melos {
		for _, q := range r.packages {
			if q.spec.name == name && m.has(q.directory) {
				return q.directory, true
			}
		}
	}
	return "", false
}

// external resolves a package the project does not build: Flutter's SDK packages to
// their island, anything else to pub as pubspec.lock pinned it, or else as the
// pubspec declares it. The lock pins a hosted package at its version, the
// constraint kept as the requested one, and a git package by the commit it resolved
// to; without the lock, a bare version and a git commit pin, and a caret, a range,
// `any` and a branch float.
//
// Implements: REQ-DART-005, REQ-DART-007, REQ-DART-008
func (r *resolver) external(p *pubPackage, name string) lang.Target {
	if p == nil {
		if flutterSDK[name] {
			return lang.Target{Ecosystem: ecosystemFlutter, Package: name}
		}
		return lang.Target{Ecosystem: ecosystemPub, Package: name, Unresolved: true}
	}
	d := p.dependencies[name]
	if d == nil && p.root != nil {
		d = p.root.dependencies[name]
	}
	if flutterSDK[name] || d != nil && d.source == "sdk" {
		return lang.Target{Ecosystem: ecosystemFlutter, Package: name}
	}
	if l := p.lock[name]; l != nil {
		t := lang.Target{Ecosystem: ecosystemPub, Package: name, Version: l.version}
		switch l.source {
		case "sdk":
			return lang.Target{Ecosystem: ecosystemFlutter, Package: name}
		case "git":
			t.Origin, t.Pinned = l.url, lang.Commit(l.resolvedReference)
		case "path":
			t.Origin = "path:" + filepath.ToSlash(l.path)
		default:
			t.Pinned = true
			if d != nil && d.constraint != "" && d.constraint != l.version {
				t.Requested = d.constraint
			}
		}
		return t
	}
	if d == nil {
		return lang.Target{Ecosystem: ecosystemPub, Package: name, Unresolved: true}
	}
	t := lang.Target{Ecosystem: ecosystemPub, Package: name, Version: d.constraint}
	switch d.source {
	case "git":
		t.Origin, t.Version = d.url, d.reference
		t.Pinned = lang.Commit(d.reference)
		t.Floating = !t.Pinned
	case "path":
		t.Origin = "path:" + d.path
	default:
		t.Pinned = pubPinned(d.constraint)
		t.Floating = d.constraint == "" || d.constraint == "any"
	}
	return t
}
