package swift

import (
	"cmp"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cocoapods"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

type resolver struct {
	root string
	lang.Layout
	modules map[string][]string // module -> directories of the targets building it, shallowest first
	named   map[string][]string // directory base name -> directories holding Swift, shallowest first
	targets []string            // target directories, deepest first
	// projects are the directories with a Package.swift or an Xcode project,
	// shallowest first.
	projects []*project
	types    map[string][]string // type name -> files declaring it at the top level
	pods     *cocoapods.Index    // what Podfiles and Cartfiles declare
	lang.NoteList
}

// The notes a resolver keeps reach --explain only through lang.Noter.
var _ lang.Noter = (*resolver)(nil)

// project is what one directory's manifests say: Package.swift, the Xcode projects
// and workspaces beside it, and their Package.resolved files.
type project struct {
	directory string
	declared  map[string]dependency // identity -> declaration
	pins      map[string]pin        // identity -> what Package.resolved pins
	products  map[string]string     // product -> package identity (or pre-5.2 package name)
	names     map[string]string     // pre-5.2 `name:` of a package, lower case -> identity
}

// Implements: REQ-SWIFT-004, REQ-SWIFT-007, REQ-SWIFT-009, REQ-SWIFT-010, REQ-SWIFT-011, REQ-SWIFT-013
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, Layout: lang.NewLayout(), modules: map[string][]string{},
		named: map[string][]string{}, types: map[string][]string{}}
	sorted := append([]*scan.File(nil), all...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	repository := lang.NewSource(root)
	byDirectory := map[string]*project{}
	get := func(directory string) *project {
		p := byDirectory[directory]
		if p == nil {
			p = &project{directory: directory, declared: map[string]dependency{}, pins: map[string]pin{}, products: map[string]string{}, names: map[string]string{}}
			byDirectory[directory] = p
		}
		return p
	}
	read := func(relative string) (string, bool) {
		data, ok := repository.Read(relative) // on disk too when the scan left it out
		return string(data), ok
	}
	var swiftDirectories []string
	for _, f := range sorted {
		r.Add(f.Path)
		repository.Add(f)
		if strings.HasSuffix(f.Path, ".swift") && !buildOutput(f.Path) {
			swiftDirectories = append(swiftDirectories, path.Dir(f.Path))
		}
	}
	var manifests []string
	for _, f := range sorted {
		if buildOutput(f.Path) {
			continue
		}
		directory, base := path.Dir(f.Path), path.Base(f.Path)
		switch {
		case base == "Package.swift":
			manifests = append(manifests, f.Path)
		case base == "project.pbxproj" && strings.HasSuffix(directory, ".xcodeproj"):
			source, _ := read(f.Path)
			p := get(path.Dir(directory))
			dependencies, products := xcodePackages(source)
			for _, d := range dependencies {
				p.declare(d, path.Dir(directory))
			}
			for k, v := range products {
				p.products[k] = v
			}
		case base == "Package.resolved" && strings.HasSuffix(directory, "xcshareddata/swiftpm"):
			// X.xcodeproj/project.xcworkspace/xcshareddata/swiftpm or
			// X.xcworkspace/xcshareddata/swiftpm: the project is beside X.
			d := directory
			for d != "." && !strings.HasSuffix(d, ".xcodeproj") && !strings.HasSuffix(d, ".xcworkspace") {
				d = path.Dir(d)
			}
			if strings.HasSuffix(d, ".xcworkspace") && strings.HasSuffix(path.Dir(d), ".xcodeproj") {
				d = path.Dir(d)
			}
			if d != "." {
				source, _ := read(f.Path)
				pins := readResolved([]byte(source))
				get(path.Dir(d)).pin(pins)
				r.noteResolved(f.Path, path.Dir(d), len(pins))
			}
		}
	}
	for _, m := range manifests {
		directory := path.Dir(m)
		source, _ := read(m)
		source = stripComments(source)
		p := get(directory)
		for _, d := range dependencies(source) {
			p.declare(d, directory)
		}
		for _, t := range targets(source) {
			for k, v := range t.products {
				p.products[k] = v
			}
			if d := r.targetDirectory(directory, t); d != "" {
				name := c99name(t.name)
				r.modules[name] = append(r.modules[name], d)
				r.targets = append(r.targets, d)
			}
		}
		if lock, ok := read(path.Join(directory, "Package.resolved")); ok {
			pins := readResolved([]byte(lock))
			p.pin(pins)
			if !repository.Listed(path.Join(directory, "Package.resolved")) && len(pins) > 0 {
				r.NoteIgnored(path.Join(directory, "Package.resolved"))
			}
			r.noteResolved(path.Join(directory, "Package.resolved"), directory, len(pins))
		}
	}
	for _, p := range byDirectory {
		r.projects = append(r.projects, p)
	}
	sort.Slice(r.projects, func(i, j int) bool { return lang.ShallowestFirst(r.projects[i].directory, r.projects[j].directory) })
	for name, directories := range r.modules {
		sortShallow(directories)
		r.modules[name] = directories
	}
	sort.Slice(r.targets, func(i, j int) bool { return lang.DeepestFirst(r.targets[i], r.targets[j]) })
	// Directories that may be modules of an Xcode project: every directory holding
	// Swift, or holding directories that do, by its name - except dependency
	// checkouts (CocoaPods, Carthage).
	seen := map[string]bool{}
	for _, d := range swiftDirectories {
		for ; d != "." && !seen[d]; d = path.Dir(d) {
			seen[d] = true
			if !vendored(d) {
				r.named[path.Base(d)] = append(r.named[path.Base(d)], d)
			}
		}
	}
	for name, directories := range r.named {
		sortShallow(directories)
		r.named[name] = directories
	}
	r.indexTypes(sorted)
	r.pods = cocoapods.Read(root, all)
	return r
}

// declare records a dependency; a path dependency's path is made relative to the
// repository root.
func (p *project) declare(d dependency, directory string) {
	switch {
	case d.url != "":
		p.declared[identity(d.url)] = d
		if d.name != "" {
			p.names[strings.ToLower(d.name)] = identity(d.url)
		}
	case d.path != "":
		d.path = path.Join(directory, d.path)
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

// noteResolved notes a Package.resolved that pins packages the walk cannot go past:
// it is flat, and Dependencies reads edges only from the checkouts SwiftPM leaves
// under .build/checkouts, which this project has none of.
//
// Implements: REQ-SWIFT-012, REQ-TRC-017
func (r *resolver) noteResolved(file, directory string, pins int) {
	if pins == 0 {
		return
	}
	if r.root != "" {
		if lang.OpenRoot(r.root).IsDirectory(filepath.Join(r.root, filepath.FromSlash(directory), ".build", "checkouts")) {
			return
		}
	}
	r.Note(file, trace.NoteFlat, "Package.resolved pins versions but records no edges, and no .build/checkouts "+
		"is on disk: --resolve-depth adds nothing past the packages it pins")
}

// has reports whether the project declares or pins a package identity.
func (p *project) has(id string) bool {
	_, declared := p.declared[id]
	_, pinned := p.pins[id]
	return declared || pinned
}

// targetDirectory is where a target's sources are: its path:, else SwiftPM's default -
// Tests/<name> for a test target, Plugins/<name> for a plugin, else
// Sources/<name> (or Source, src, srcs). A directory without files is none.
func (r *resolver) targetDirectory(directory string, t target) string {
	if t.path != "" {
		d := path.Join(directory, t.path)
		if lang.Inside(d) && (r.Directories[d] || d == ".") {
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
		if d := path.Join(directory, parent, t.name); r.Directories[d] {
			return d
		}
	}
	return ""
}

func sortShallow(directories []string) {
	sort.SliceStable(directories, func(i, j int) bool { return lang.ShallowestFirst(directories[i], directories[j]) })
}

// vendored reports whether a directory holds checkouts of dependencies.
func vendored(d string) bool {
	for _, segment := range strings.Split(d, "/") {
		switch segment {
		case "Pods", "Carthage", "checkouts", "SourcePackages", ".build":
			return true
		}
	}
	return false
}

// readFile reads a file on disk as text, through lang.ReadBounded.
func readFile(absolute string) (string, bool) {
	data, ok := lang.ReadBounded(absolute)
	return string(data), ok
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
		source, ok := readFile(f.AbsolutePath)
		if !ok || manifest([]byte(source)) {
			continue
		}
		for _, m := range topLevelType.FindAllStringSubmatch(stripComments(source), -1) {
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
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, scope, _ := strings.Cut(rawImport.Name, ":")
	switch kind {
	case kindImport:
		return r.module(file, rawImport.Module)
	case kindURL:
		return r.packageName(r.projectsOf(file), identity(rawImport.Module), rawImport.Module)
	case kindID:
		return r.packageName(r.projectsOf(file), strings.ToLower(rawImport.Module), rawImport.Module)
	case kindPath:
		d := path.Clean(path.Dir(file) + strings.TrimPrefix(rawImport.Module, "__DIR__"))
		if lang.Inside(d) && r.Directories[d] {
			return lang.Target{Local: d}
		}
	case kindType:
		return r.typeReference(file, rawImport.Module, scope)
	}
	return lang.Target{}
}

// module resolves an imported module: a target of the project's own manifests, the
// toolchain's and Apple's libraries, a package the project declares, a directory
// named after the module (an Xcode project's framework), else an unresolved package.
// A module no SwiftPM manifest provides may be a pod a Podfile declares or a
// Carthage framework (REQ-OBJC-012).
//
// Implements: REQ-SWIFT-004, REQ-SWIFT-005, REQ-SWIFT-007, REQ-OBJC-012
func (r *resolver) module(file, m string) lang.Target {
	own := r.moduleOf(file)
	if directories := r.modules[m]; len(directories) > 0 {
		d := r.nearest(file, directories)
		if d == own {
			return lang.Target{}
		}
		return lang.Target{Local: d}
	}
	if stdModules[m] {
		return lang.Target{Ecosystem: ecosystemStd, Package: m}
	}
	if appleModules[m] {
		return lang.Target{Ecosystem: ecosystemApple, Package: m}
	}
	projects := r.projectsOf(file)
	if id, ok := r.packageOf(projects, m); ok {
		return r.packageName(projects, id, "")
	}
	if t, ok := r.pods.Module(file, m); ok {
		return t // a pod or Carthage framework of an app built with CocoaPods or Carthage
	}
	if directories := r.named[m]; len(directories) > 0 {
		if d := r.nearest(file, directories); d != own {
			return lang.Target{Local: d}
		}
		return lang.Target{}
	}
	if url := knownPackages[m]; url != "" {
		return lang.Target{Ecosystem: ecosystemSwiftPM, Package: url, Unresolved: true}
	}
	return lang.Target{Ecosystem: ecosystemSwiftPM, Package: m, Unresolved: true}
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
	want := lang.FoldAlphanumeric(m)
	best, bestLength := "", 0
	for _, p := range projects {
		for _, id := range p.identities() {
			f := lang.FoldAlphanumeric(id)
			short := strings.TrimPrefix(f, "swift")
			_, after, dotted := strings.Cut(id, ".") // a registry id: scope.name
			switch {
			case f == want || short == want || strings.TrimSuffix(f, "swift") == want || dotted && lang.FoldAlphanumeric(after) == want:
				return id, true
			case len(short) >= 3 && strings.HasPrefix(want, short) && len(short) > bestLength:
				best, bestLength = id, len(short)
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

// packageName is the target of a package identity: pinned by the nearest project whose
// Package.resolved has it, else as the nearest project declaring it asks for it. A
// path dependency is its directory.
//
// Implements: REQ-SWIFT-006, REQ-SWIFT-008, REQ-SWIFT-009
func (r *resolver) packageName(projects []*project, id, spelled string) lang.Target {
	var d *dependency
	for _, p := range projects {
		if dependency, ok := p.declared[id]; ok {
			d = &dependency
			break
		}
	}
	if d != nil && d.path != "" {
		if lang.Inside(d.path) && r.Directories[d.path] {
			return lang.Target{Local: d.path}
		}
		return lang.Target{}
	}
	for _, p := range projects {
		q, ok := p.pins[id]
		if !ok {
			continue
		}
		t := lang.Target{Ecosystem: ecosystemSwiftPM, Package: packageName(q.location), Version: q.version, Pinned: true}
		if q.registry {
			t.Package = q.location
		}
		t.Version = cmp.Or(t.Version, q.revision)
		if t.Version == "" {
			t.Pinned, t.Floating = false, true
		}
		if d != nil && d.requirement != "" && d.requirement != t.Version {
			t.Requested = d.requirement
		}
		return t
	}
	if d == nil {
		name := cmp.Or(spelled, id)
		if strings.Contains(name, "/") {
			name = packageName(name)
		}
		return lang.Target{Ecosystem: ecosystemSwiftPM, Package: name, Unresolved: true}
	}
	name := d.id
	if d.url != "" {
		name = packageName(d.url)
	}
	return lang.Target{Ecosystem: ecosystemSwiftPM, Package: name, Version: d.requirement, Pinned: d.pinned, Floating: d.floating}
}

// projectsOf lists the projects a file belongs to, nearest first; a file under none
// takes every project, the shallowest first.
func (r *resolver) projectsOf(file string) []*project {
	var out []*project
	for i := len(r.projects) - 1; i >= 0; i-- {
		if p := r.projects[i]; lang.Within(file, p.directory) {
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
func (r *resolver) nearest(file string, directories []string) string {
	best, bestLength := directories[0], -1
	for _, d := range directories {
		n := lang.CommonSegments(file, d)
		if n > bestLength {
			best, bestLength = d, n
		}
	}
	return best
}

// moduleOf is the directory of the module a file belongs to: the deepest target
// directory around it, else - an Xcode project, whose targets are not read - the
// top-level directory it sits in under its project (MyApp/, MyAppTests/), else the
// repository's top-level directory.
//
// Implements: REQ-SWIFT-011
func (r *resolver) moduleOf(file string) string {
	for _, d := range r.targets {
		if lang.Within(file, d) {
			return d
		}
	}
	base := "."
	for i := len(r.projects) - 1; i >= 0; i-- {
		if p := r.projects[i]; lang.Within(file, p.directory) {
			base = p.directory
			break
		}
	}
	rest := file
	if base != "." {
		rest = strings.TrimPrefix(file, base+"/")
	}
	segment, _, ok := strings.Cut(rest, "/")
	if !ok {
		return base
	}
	return path.Join(base, segment)
}

// typeReference resolves a type name a file uses to the file declaring it: in the file's
// own module, else in one of the project's modules the file imports. Anything else -
// the standard library's, a package's, the file's own - is dropped.
//
// Implements: REQ-SWIFT-011
func (r *resolver) typeReference(file, name, imported string) lang.Target {
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
		if directory := r.module(file, m).Local; directory != "" {
			if t := pick(func(c string) bool { return lang.Within(c, directory) }); t.Local != "" {
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
	if t.Ecosystem == cocoapods.Ecosystem || t.Ecosystem == cocoapods.Carthage {
		return r.pods.Dependencies(t)
	}
	if t.Ecosystem != ecosystemSwiftPM || r.root == "" {
		return nil
	}
	for _, p := range r.projects {
		for id, q := range p.pins {
			if packageName(q.location) != t.Package {
				continue
			}
			// id comes from Package.resolved: the Root refuses one that climbs
			// out of the repository, by ".." or through a link.
			data, ok := lang.OpenRoot(r.root).ReadBounded(filepath.Join(r.root, filepath.FromSlash(p.directory), ".build", "checkouts", id, "Package.swift"))
			source := string(data)
			if !ok {
				return nil
			}
			var out []lang.Target
			for _, d := range dependencies(stripComments(source)) {
				if d.url == "" && d.id == "" {
					continue
				}
				dependency := identity(d.url)
				if d.url == "" {
					dependency = strings.ToLower(d.id)
				}
				test := r.packageName([]*project{p}, dependency, cmp.Or(d.url, d.id))
				if test.Unresolved {
					name := d.id
					if d.url != "" {
						name = packageName(d.url)
					}
					test = lang.Target{Ecosystem: ecosystemSwiftPM, Package: name, Version: d.requirement, Pinned: d.pinned, Floating: d.floating}
				} else if test.Requested == "" && d.requirement != "" && d.requirement != test.Version {
					test.Requested = d.requirement
				}
				out = append(out, test)
			}
			return out
		}
	}
	return nil
}

// Installed says a Carthage dependency's dependencies come from its checkout.
func (r *resolver) Installed(t lang.Target) bool { return r.pods.Installed(t) }
