package swift

import (
	"cmp"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	root    string
	files   map[string]bool
	dirs    map[string]bool
	modules map[string][]string // module -> directories of the targets building it, shallowest first
	named   map[string][]string // directory base name -> directories holding Swift, shallowest first
	targets []string            // target directories, deepest first
	// projects are the directories with a Package.swift or an Xcode project,
	// shallowest first.
	projects []*project
	types    map[string][]string // type name -> files declaring it at the top level
}

// project is what one directory's manifests say: Package.swift, the Xcode projects
// and workspaces beside it, and their Package.resolved files.
type project struct {
	dir      string
	declared map[string]dependency // identity -> declaration
	pins     map[string]pin        // identity -> what Package.resolved pins
	products map[string]string     // product -> package identity (or pre-5.2 package name)
	names    map[string]string     // pre-5.2 `name:` of a package, lower case -> identity
}

// Implements: REQ-SWIFT-004, REQ-SWIFT-007, REQ-SWIFT-009, REQ-SWIFT-010, REQ-SWIFT-011, REQ-SWIFT-013
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, dirs: map[string]bool{}, modules: map[string][]string{},
		named: map[string][]string{}, types: map[string][]string{}}
	sorted := append([]*scan.File(nil), all...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	abs := map[string]string{}
	byDir := map[string]*project{}
	get := func(dir string) *project {
		p := byDir[dir]
		if p == nil {
			p = &project{dir: dir, declared: map[string]dependency{}, pins: map[string]pin{}, products: map[string]string{}, names: map[string]string{}}
			byDir[dir] = p
		}
		return p
	}
	read := func(rel string) (string, bool) {
		if a, ok := abs[rel]; ok {
			return readFile(a)
		}
		if root == "" || !inside(rel) {
			return "", false
		}
		return readFile(filepath.Join(root, filepath.FromSlash(rel))) // a file the scan left out
	}
	var swiftDirs []string
	for _, f := range sorted {
		r.files[f.Path] = true
		abs[f.Path] = f.Abs
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		if strings.HasSuffix(f.Path, ".swift") && !buildOutput(f.Path) {
			swiftDirs = append(swiftDirs, path.Dir(f.Path))
		}
	}
	var manifests []string
	for _, f := range sorted {
		if buildOutput(f.Path) {
			continue
		}
		dir, base := path.Dir(f.Path), path.Base(f.Path)
		switch {
		case base == "Package.swift":
			manifests = append(manifests, f.Path)
		case base == "project.pbxproj" && strings.HasSuffix(dir, ".xcodeproj"):
			src, _ := read(f.Path)
			p := get(path.Dir(dir))
			deps, products := xcodePackages(src)
			for _, d := range deps {
				p.declare(d, path.Dir(dir))
			}
			for k, v := range products {
				p.products[k] = v
			}
		case base == "Package.resolved" && strings.HasSuffix(dir, "xcshareddata/swiftpm"):
			// X.xcodeproj/project.xcworkspace/xcshareddata/swiftpm or
			// X.xcworkspace/xcshareddata/swiftpm: the project is beside X.
			d := dir
			for d != "." && !strings.HasSuffix(d, ".xcodeproj") && !strings.HasSuffix(d, ".xcworkspace") {
				d = path.Dir(d)
			}
			if strings.HasSuffix(d, ".xcworkspace") && strings.HasSuffix(path.Dir(d), ".xcodeproj") {
				d = path.Dir(d)
			}
			if d != "." {
				src, _ := read(f.Path)
				get(path.Dir(d)).pin(readResolved([]byte(src)))
			}
		}
	}
	for _, m := range manifests {
		dir := path.Dir(m)
		src, _ := read(m)
		src = stripComments(src)
		p := get(dir)
		for _, d := range dependencies(src) {
			p.declare(d, dir)
		}
		for _, t := range targets(src) {
			for k, v := range t.products {
				p.products[k] = v
			}
			if d := r.targetDir(dir, t); d != "" {
				name := c99name(t.name)
				r.modules[name] = append(r.modules[name], d)
				r.targets = append(r.targets, d)
			}
		}
		if lock, ok := read(path.Join(dir, "Package.resolved")); ok {
			p.pin(readResolved([]byte(lock)))
		}
	}
	for _, p := range byDir {
		r.projects = append(r.projects, p)
	}
	sort.Slice(r.projects, func(i, j int) bool {
		a, b := r.projects[i].dir, r.projects[j].dir
		if depth(a) != depth(b) {
			return depth(a) < depth(b)
		}
		return a < b
	})
	for name, dirs := range r.modules {
		sortShallow(dirs)
		r.modules[name] = dirs
	}
	sort.Slice(r.targets, func(i, j int) bool {
		a, b := r.targets[i], r.targets[j]
		if depth(a) != depth(b) {
			return depth(a) > depth(b)
		}
		return a < b
	})
	// Directories that may be modules of an Xcode project: every directory holding
	// Swift, or holding directories that do, by its name - except dependency
	// checkouts (CocoaPods, Carthage).
	seen := map[string]bool{}
	for _, d := range swiftDirs {
		for ; d != "." && !seen[d]; d = path.Dir(d) {
			seen[d] = true
			if !vendored(d) {
				r.named[path.Base(d)] = append(r.named[path.Base(d)], d)
			}
		}
	}
	for name, dirs := range r.named {
		sortShallow(dirs)
		r.named[name] = dirs
	}
	r.indexTypes(sorted)
	return r
}

// declare records a dependency; a path dependency's path is made relative to the
// repository root.
func (p *project) declare(d dependency, dir string) {
	switch {
	case d.url != "":
		p.declared[identity(d.url)] = d
		if d.name != "" {
			p.names[strings.ToLower(d.name)] = identity(d.url)
		}
	case d.path != "":
		d.path = path.Join(dir, d.path)
		p.declared[identity(d.path)] = d
		if d.name != "" {
			p.names[strings.ToLower(d.name)] = identity(d.path)
		}
	case d.id != "":
		p.declared[strings.ToLower(d.id)] = d
	}
}

func (p *project) pin(pins []pin) {
	for _, q := range pins {
		p.pins[q.identity] = q
	}
}

// has reports whether the project declares or pins a package identity.
func (p *project) has(id string) bool {
	_, declared := p.declared[id]
	_, pinned := p.pins[id]
	return declared || pinned
}

// targetDir is where a target's sources are: its path:, else SwiftPM's default -
// Tests/<name> for a test target, Plugins/<name> for a plugin, else
// Sources/<name> (or Source, src, srcs). A directory without files is none.
func (r *resolver) targetDir(dir string, t target) string {
	if t.path != "" {
		d := path.Join(dir, t.path)
		if inside(d) && (r.dirs[d] || d == ".") {
			return d
		}
		return ""
	}
	parents := []string{"Sources", "Source", "src", "srcs"}
	switch t.kind {
	case "testTarget":
		parents = []string{"Tests", "Sources", "Source", "src", "srcs"}
	case "plugin":
		parents = []string{"Plugins"}
	}
	for _, parent := range parents {
		if d := path.Join(dir, parent, t.name); r.dirs[d] {
			return d
		}
	}
	return ""
}

func sortShallow(dirs []string) {
	sort.SliceStable(dirs, func(i, j int) bool {
		if depth(dirs[i]) != depth(dirs[j]) {
			return depth(dirs[i]) < depth(dirs[j])
		}
		return dirs[i] < dirs[j]
	})
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// vendored reports whether a directory holds checkouts of dependencies.
func vendored(d string) bool {
	for _, seg := range strings.Split(d, "/") {
		switch seg {
		case "Pods", "Carthage", "checkouts", "SourcePackages", ".build":
			return true
		}
	}
	return false
}

func inside(p string) bool {
	return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p)
}

func readFile(abs string) (string, bool) {
	fi, err := os.Stat(abs)
	if err != nil || fi.IsDir() || fi.Size() > lang.MaxParseSize {
		return "", false
	}
	b, err := os.ReadFile(abs)
	return string(b), err == nil
}

// topLevelType matches a type declared at the start of a line, so at the top level of
// its file in all but unusually formatted code: attributes and modifiers before it,
// the name after.
var topLevelType = regexp.MustCompile(`(?m)^(?:@\w+(?:\([^)\n]*\))?\s+)*((?:(?:public|open|internal|fileprivate|private|package|final|indirect|nonisolated)\s+)*)(?:class|struct|enum|protocol|actor|typealias)\s+([A-Za-z_]\w*)`)

// indexTypes records the types every Swift file declares at its top level, the ones
// other files of its module can use without an import. Private ones are not visible
// to them.
//
// Implements: REQ-SWIFT-011
func (r *resolver) indexTypes(files []*scan.File) {
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".swift") || buildOutput(f.Path) || f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		src, ok := readFile(f.Abs)
		if !ok || manifest([]byte(src)) {
			continue
		}
		for _, m := range topLevelType.FindAllStringSubmatch(stripComments(src), -1) {
			if strings.Contains(m[1], "private") {
				continue
			}
			if list := r.types[m[2]]; len(list) == 0 || list[len(list)-1] != f.Path {
				r.types[m[2]] = append(list, f.Path)
			}
		}
	}
}

// Implements: REQ-SWIFT-004, REQ-SWIFT-005, REQ-SWIFT-006, REQ-SWIFT-007, REQ-SWIFT-011
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, scope, _ := strings.Cut(imp.Name, ":")
	switch kind {
	case kindImport:
		return r.module(file, imp.Module)
	case kindURL:
		return r.pkg(r.projectsOf(file), identity(imp.Module), imp.Module)
	case kindID:
		return r.pkg(r.projectsOf(file), strings.ToLower(imp.Module), imp.Module)
	case kindPath:
		d := path.Clean(path.Dir(file) + strings.TrimPrefix(imp.Module, "__DIR__"))
		if inside(d) && r.dirs[d] {
			return lang.Target{Local: d}
		}
	case kindType:
		return r.typeRef(file, imp.Module, scope)
	}
	return lang.Target{}
}

// module resolves an imported module: a target of the project's own manifests, the
// toolchain's and Apple's libraries, a package the project declares, a directory
// named after the module (an Xcode project's framework), else an unresolved package.
//
// Implements: REQ-SWIFT-004, REQ-SWIFT-005, REQ-SWIFT-007
func (r *resolver) module(file, m string) lang.Target {
	own := r.moduleOf(file)
	if dirs := r.modules[m]; len(dirs) > 0 {
		d := r.nearest(file, dirs)
		if d == own {
			return lang.Target{}
		}
		return lang.Target{Local: d}
	}
	if stdModules[m] {
		return lang.Target{Ecosystem: ecoStd, Package: m}
	}
	if appleModules[m] {
		return lang.Target{Ecosystem: ecoApple, Package: m}
	}
	projects := r.projectsOf(file)
	if id, ok := r.packageOf(projects, m); ok {
		return r.pkg(projects, id, "")
	}
	if dirs := r.named[m]; len(dirs) > 0 {
		if d := r.nearest(file, dirs); d != own {
			return lang.Target{Local: d}
		}
		return lang.Target{}
	}
	if url := knownPackages[m]; url != "" {
		return lang.Target{Ecosystem: ecoSwiftPM, Package: url, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecoSwiftPM, Package: m, Unresolved: true}
}

// packageOf names the declared package a module comes from: the package a target
// takes it from as a product (.product(name:package:), an Xcode product
// dependency), the package the known-packages table names, else the declared
// package whose identity is the module's name, or starts it (NIOCore is swift-nio's,
// NIOHTTP2 swift-nio-http2's - the longest wins), a leading "swift" ignored.
//
// Implements: REQ-SWIFT-007
func (r *resolver) packageOf(projects []*project, m string) (string, bool) {
	has := func(id string) bool {
		for _, p := range projects {
			if p.has(id) {
				return true
			}
		}
		return false
	}
	for _, p := range projects {
		if v, ok := p.products[m]; ok {
			id := strings.ToLower(v)
			for _, q := range projects {
				if alias, ok := q.names[id]; ok {
					id = alias
				}
			}
			if has(id) {
				return id, true
			}
		}
	}
	if url := knownPackages[m]; url != "" && has(identity(url)) {
		return identity(url), true
	}
	want := fold(m)
	best, bestLen := "", 0
	for _, p := range projects {
		for _, id := range p.identities() {
			f := fold(id)
			short := strings.TrimPrefix(f, "swift")
			_, after, dotted := strings.Cut(id, ".") // a registry id: scope.name
			switch {
			case f == want || short == want || strings.TrimSuffix(f, "swift") == want || dotted && fold(after) == want:
				return id, true
			case len(short) >= 3 && strings.HasPrefix(want, short) && len(short) > bestLen:
				best, bestLen = id, len(short)
			}
		}
	}
	return best, best != ""
}

func (p *project) identities() []string {
	seen := map[string]bool{}
	var out []string
	for id := range p.declared {
		seen[id] = true
		out = append(out, id)
	}
	for id := range p.pins {
		if !seen[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// pkg is the target of a package identity: pinned by the nearest project whose
// Package.resolved has it, else as the nearest project declaring it asks for it. A
// path dependency is its directory.
//
// Implements: REQ-SWIFT-006, REQ-SWIFT-008, REQ-SWIFT-009
func (r *resolver) pkg(projects []*project, id, spelled string) lang.Target {
	var d *dependency
	for _, p := range projects {
		if dep, ok := p.declared[id]; ok {
			d = &dep
			break
		}
	}
	if d != nil && d.path != "" {
		if inside(d.path) && r.dirs[d.path] {
			return lang.Target{Local: d.path}
		}
		return lang.Target{}
	}
	for _, p := range projects {
		q, ok := p.pins[id]
		if !ok {
			continue
		}
		t := lang.Target{Ecosystem: ecoSwiftPM, Package: packageName(q.location), Version: q.version, Pinned: true}
		if q.registry {
			t.Package = q.location
		}
		if t.Version == "" {
			t.Version = q.revision
		}
		if t.Version == "" {
			t.Pinned, t.Floating = false, true
		}
		if d != nil && d.requirement != "" && d.requirement != t.Version {
			t.Requested = d.requirement
		}
		return t
	}
	if d == nil {
		name := spelled
		if name == "" {
			name = id
		}
		if strings.Contains(name, "/") {
			name = packageName(name)
		}
		return lang.Target{Ecosystem: ecoSwiftPM, Package: name, Unresolved: true}
	}
	name := d.id
	if d.url != "" {
		name = packageName(d.url)
	}
	return lang.Target{Ecosystem: ecoSwiftPM, Package: name, Version: d.requirement, Pinned: d.pinned, Floating: d.floating}
}

// projectsOf lists the projects a file belongs to, nearest first; a file under none
// takes every project, the shallowest first.
func (r *resolver) projectsOf(file string) []*project {
	var out []*project
	for i := len(r.projects) - 1; i >= 0; i-- {
		if p := r.projects[i]; p.dir == "." || strings.HasPrefix(file, p.dir+"/") {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return r.projects
	}
	return out
}

// nearest picks, of the directories a module could be, the one sharing the longest
// path prefix with the file; ties go to the shallowest.
func (r *resolver) nearest(file string, dirs []string) string {
	best, bestLen := dirs[0], -1
	for _, d := range dirs {
		n := common(file, d)
		if n > bestLen {
			best, bestLen = d, n
		}
	}
	return best
}

// common counts the leading path segments two paths share.
func common(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// moduleOf is the directory of the module a file belongs to: the deepest target
// directory around it, else - an Xcode project, whose targets are not read - the
// top-level directory it sits in under its project (MyApp/, MyAppTests/), else the
// repository's top-level directory.
//
// Implements: REQ-SWIFT-011
func (r *resolver) moduleOf(file string) string {
	for _, d := range r.targets {
		if d == "." || strings.HasPrefix(file, d+"/") {
			return d
		}
	}
	base := "."
	for i := len(r.projects) - 1; i >= 0; i-- {
		if p := r.projects[i]; p.dir == "." || strings.HasPrefix(file, p.dir+"/") {
			base = p.dir
			break
		}
	}
	rest := file
	if base != "." {
		rest = strings.TrimPrefix(file, base+"/")
	}
	seg, _, ok := strings.Cut(rest, "/")
	if !ok {
		return base
	}
	return path.Join(base, seg)
}

// typeRef resolves a type name a file uses to the file declaring it: in the file's
// own module, else in one of the project's modules the file imports. Anything else -
// the standard library's, a package's, the file's own - is dropped.
//
// Implements: REQ-SWIFT-011
func (r *resolver) typeRef(file, name, imported string) lang.Target {
	candidates := r.types[name]
	if slices.Contains(candidates, file) {
		return lang.Target{} // declared here after all, in a form the query missed
	}
	pick := func(in func(c string) bool) lang.Target {
		for _, c := range candidates {
			if in(c) {
				return lang.Target{Local: c}
			}
		}
		return lang.Target{}
	}
	own := r.moduleOf(file)
	if t := pick(func(c string) bool { return r.moduleOf(c) == own }); t.Local != "" {
		return t
	}
	for _, m := range strings.Split(imported, ",") {
		if m == "" {
			continue
		}
		if dir := r.module(file, m).Local; dir != "" {
			if t := pick(func(c string) bool { return dir == "." || strings.HasPrefix(c, dir+"/") }); t.Local != "" {
				return t
			}
		}
	}
	return lang.Target{}
}

// Dependencies implements lang.Transitive where SwiftPM has checked a package out
// under .build/checkouts: its Package.swift declares what it depends on, and the
// project's Package.resolved pins those too. Package.resolved alone is flat.
//
// Implements: REQ-SWIFT-012
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoSwiftPM || r.root == "" {
		return nil
	}
	for _, p := range r.projects {
		for id, q := range p.pins {
			if packageName(q.location) != t.Package {
				continue
			}
			src, ok := readFile(filepath.Join(r.root, filepath.FromSlash(p.dir), ".build", "checkouts", id, "Package.swift"))
			if !ok {
				return nil
			}
			var out []lang.Target
			for _, d := range dependencies(stripComments(src)) {
				if d.url == "" && d.id == "" {
					continue
				}
				dep := identity(d.url)
				if d.url == "" {
					dep = strings.ToLower(d.id)
				}
				tt := r.pkg([]*project{p}, dep, cmp.Or(d.url, d.id))
				if tt.Unresolved {
					name := d.id
					if d.url != "" {
						name = packageName(d.url)
					}
					tt = lang.Target{Ecosystem: ecoSwiftPM, Package: name, Version: d.requirement, Pinned: d.pinned, Floating: d.floating}
				} else if tt.Requested == "" && d.requirement != "" && d.requirement != tt.Version {
					tt.Requested = d.requirement
				}
				out = append(out, tt)
			}
			return out
		}
	}
	return nil
}
