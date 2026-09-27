package dart

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// flutterSDK are the packages Flutter's SDK ships: a project depends on them with
// `sdk: flutter`, and no index serves them.
//
// Implements: REQ-DART-005
var flutterSDK = map[string]bool{
	"flutter": true, "flutter_test": true, "flutter_localizations": true, "flutter_driver": true,
	"flutter_web_plugins": true, "integration_test": true, "sky_engine": true,
}

// pkg is a pub package of the repository.
type pkg struct {
	dir     string
	spec    *pubspec
	deps    map[string]*dependency // by name; an override replaces the declaration
	lock    map[string]*locked     // its own pubspec.lock, or its workspace's
	lockDir string
	root    *pkg // the pub workspace this package is a member of
	melos   []*melosRepo
}

// melosRepo is a melos repository: the packages its globs select are linked to each
// other by `melos bootstrap`, whatever versions their pubspecs ask for.
type melosRepo struct {
	dir     string
	include []string
	exclude []string
}

type resolver struct {
	files map[string]bool
	dirs  map[string]bool
	pkgs  []*pkg // deepest first
}

// Implements: REQ-DART-004, REQ-DART-006, REQ-DART-007, REQ-DART-009
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}}
	abs := map[string]string{}
	var pubspecs, melos []string
	for _, f := range all {
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		if (Plugin{}).Claims(f) && path.Base(f.Path) == "pubspec.yaml" {
			pubspecs = append(pubspecs, f.Path)
		}
		if path.Base(f.Path) == "melos.yaml" {
			melos = append(melos, f.Path)
		}
	}
	sort.Strings(pubspecs)
	sort.Strings(melos)
	// pubspec.lock and pubspec_overrides.yaml are git-ignored as often as not; what is
	// on disk beside a pubspec is what pub resolved with.
	read := func(rel string) ([]byte, bool) {
		if a, ok := abs[rel]; ok {
			data, err := os.ReadFile(a)
			return data, err == nil
		}
		if root == "" || !inside(rel) {
			return nil, false
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		return data, err == nil
	}
	byDir := map[string]*pkg{}
	for _, f := range pubspecs {
		src, _ := read(f)
		spec, err := readPubspec(src)
		if err != nil {
			continue
		}
		p := &pkg{dir: path.Dir(f), spec: spec, deps: map[string]*dependency{}}
		for _, section := range []string{"dev_dependencies", "dependencies", "dependency_overrides"} {
			for _, d := range spec.deps {
				if d.section == section {
					p.deps[d.name] = d
				}
			}
		}
		if src, ok := read(path.Join(p.dir, "pubspec_overrides.yaml")); ok {
			if o, err := readPubspec(src); err == nil {
				for _, d := range o.deps {
					p.deps[d.name] = d
				}
			}
		}
		if src, ok := read(path.Join(p.dir, "pubspec.lock")); ok {
			p.lock, p.lockDir = readLock(src), p.dir
		}
		byDir[p.dir] = p
		r.pkgs = append(r.pkgs, p)
	}
	// A pub workspace: the root lists its members, which resolve together against
	// the root's pubspec.lock.
	for _, p := range r.pkgs {
		for _, m := range p.spec.workspace {
			for _, q := range r.pkgs {
				if q != p && q.root == nil && globMatch(strings.TrimSuffix(path.Join(p.dir, m), "/"), q.dir) {
					q.root = p
					if q.lock == nil {
						q.lock, q.lockDir = p.lock, p.lockDir
					}
				}
			}
		}
	}
	// melos: melos.yaml's packages globs, or melos 7's `melos:` section in the root
	// pubspec (whose packages are then its workspace, already read above).
	var repos []*melosRepo
	for _, f := range melos {
		src, _ := read(f)
		var doc struct {
			Packages []string `yaml:"packages"`
			Ignore   []string `yaml:"ignore"`
		}
		if yaml.Unmarshal(src, &doc) == nil && len(doc.Packages) > 0 {
			repos = append(repos, &melosRepo{dir: path.Dir(f), include: doc.Packages, exclude: doc.Ignore})
		}
	}
	for _, p := range r.pkgs {
		if p.spec.melos && len(p.spec.melosPkgs) > 0 {
			repos = append(repos, &melosRepo{dir: p.dir, include: p.spec.melosPkgs})
		}
	}
	for _, m := range repos {
		for _, p := range r.pkgs {
			if m.has(p.dir) {
				p.melos = append(p.melos, m)
			}
		}
	}
	sort.SliceStable(r.pkgs, func(i, j int) bool { return depth(r.pkgs[i].dir) > depth(r.pkgs[j].dir) })
	return r
}

// has reports whether a package directory is one of the repository's.
func (m *melosRepo) has(dir string) bool {
	match := func(globs []string) bool {
		for _, g := range globs {
			if globMatch(strings.TrimSuffix(path.Join(m.dir, g), "/"), dir) {
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

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// inside reports whether a cleaned relative path stays in the repository.
func inside(p string) bool { return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p) }

// pkgOf is the package a file belongs to: the nearest pubspec.yaml above it.
func (r *resolver) pkgOf(file string) *pkg {
	for _, p := range r.pkgs {
		if p.dir == "." || strings.HasPrefix(file, p.dir+"/") {
			return p
		}
	}
	return nil
}

// Implements: REQ-DART-002, REQ-DART-004, REQ-DART-005, REQ-DART-006, REQ-DART-008
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, _, _ := strings.Cut(imp.Name, ":")
	switch kind {
	case kindMember:
		if f := path.Join(path.Dir(file), imp.Module, "pubspec.yaml"); r.files[f] {
			return lang.Target{Local: f}
		}
		return lang.Target{}
	case kindDep:
		p := r.pkgOf(file)
		if dir, ok := r.localPackage(p, imp.Module); ok {
			if f := path.Join(dir, "pubspec.yaml"); r.files[f] {
				return lang.Target{Local: f}
			}
			return r.localDir(dir)
		}
		return r.external(p, imp.Module)
	}
	u := imp.Module
	if rest, ok := strings.CutPrefix(u, "dart:"); ok {
		return lang.Target{Ecosystem: ecoStd, Package: "dart:" + rest}
	}
	if rest, ok := strings.CutPrefix(u, "package:"); ok {
		name, sub, _ := strings.Cut(rest, "/")
		p := r.pkgOf(file)
		if dir, ok := r.localPackage(p, name); ok {
			return r.lib(dir, sub)
		}
		if name == "flutter_gen" {
			return lang.Target{} // Flutter's synthetic package, generated under .dart_tool
		}
		return r.external(p, name)
	}
	if strings.Contains(u, ":") || strings.HasPrefix(u, "/") {
		return lang.Target{} // file:, http: - nothing the repository holds
	}
	if f := path.Join(path.Dir(file), u); inside(f) && r.files[f] {
		return lang.Target{Local: f}
	}
	return lang.Target{} // a generated part (x.g.dart) that is not committed
}

// lib is the file a package: URI names in a local package's lib/ directory, or when
// it is missing (generated, not committed), the directory itself.
func (r *resolver) lib(dir, sub string) lang.Target {
	if f := path.Join(dir, "lib", sub); r.files[f] {
		return lang.Target{Local: f}
	}
	if t := r.localDir(path.Join(dir, "lib")); t.Local != "" {
		return t
	}
	if f := path.Join(dir, "pubspec.yaml"); r.files[f] {
		return lang.Target{Local: f}
	}
	return r.localDir(dir)
}

func (r *resolver) localDir(dir string) lang.Target {
	if r.dirs[dir] {
		return lang.Target{Local: dir}
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
func (r *resolver) localPackage(p *pkg, name string) (string, bool) {
	if p == nil {
		return "", false
	}
	if p.spec.name == name {
		return p.dir, true
	}
	for _, q := range []*pkg{p, p.root} {
		if q == nil {
			continue
		}
		if d := q.deps[name]; d != nil && d.source == "path" {
			if dir := path.Join(q.dir, d.path); inside(dir) {
				return dir, true
			}
		}
	}
	if l := p.lock[name]; l != nil && l.source == "path" && l.relative {
		if dir := path.Join(p.lockDir, filepath.ToSlash(l.path)); inside(dir) {
			return dir, true
		}
	}
	root := p.root
	if root == nil && len(p.spec.workspace) > 0 {
		root = p
	}
	for _, q := range r.pkgs {
		if root != nil && (q == root || q.root == root) && q.spec.name == name {
			return q.dir, true
		}
	}
	for _, m := range p.melos {
		for _, q := range r.pkgs {
			if q.spec.name == name && m.has(q.dir) {
				return q.dir, true
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
func (r *resolver) external(p *pkg, name string) lang.Target {
	if p == nil {
		if flutterSDK[name] {
			return lang.Target{Ecosystem: ecoFlutter, Package: name}
		}
		return lang.Target{Ecosystem: ecoPub, Package: name, Unresolved: true}
	}
	d := p.deps[name]
	if d == nil && p.root != nil {
		d = p.root.deps[name]
	}
	if flutterSDK[name] || d != nil && d.source == "sdk" {
		return lang.Target{Ecosystem: ecoFlutter, Package: name}
	}
	if l := p.lock[name]; l != nil {
		t := lang.Target{Ecosystem: ecoPub, Package: name, Version: l.version}
		switch l.source {
		case "sdk":
			return lang.Target{Ecosystem: ecoFlutter, Package: name}
		case "git":
			t.Origin, t.Pinned = l.url, lang.Commit(l.resolvedRef)
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
		return lang.Target{Ecosystem: ecoPub, Package: name, Unresolved: true}
	}
	t := lang.Target{Ecosystem: ecoPub, Package: name, Version: d.constraint}
	switch d.source {
	case "git":
		t.Origin, t.Version = d.url, d.ref
		t.Pinned = lang.Commit(d.ref)
		t.Floating = !t.Pinned
	case "path":
		t.Origin = "path:" + d.path
	default:
		t.Pinned = pubPinned(d.constraint)
		t.Floating = d.constraint == "" || d.constraint == "any"
	}
	return t
}
