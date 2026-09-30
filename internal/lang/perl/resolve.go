package perl

import (
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a directory holding a distribution's or an application's manifests:
// what they require (per module and per distribution), the distribution's own name
// and the cpanfile.snapshot beside them.
type project struct {
	directory    string
	name         string
	requirements map[string]moduleRequirement // module -> its first requirement
	dists        map[string]moduleRequirement // distribution -> the first requirement of a module it provides
	snap         *snapshot
}

type resolver struct {
	files       map[string]bool
	projects    []*project
	byDirectory map[string]*project
	packages    map[string][]string // package -> the files declaring it
	suffixes    map[string][]string // Foo/Bar.pm -> the .pm files ending so
}

// ignored reports whether a path is inside what Carton installs into a project
// (local/lib/perl5, local/bin) or a build's blib.
//
// Implements: REQ-PERL-001
func ignored(p string) bool {
	segments := strings.Split(p, "/")
	for i, segment := range segments {
		if segment == "blib" {
			return true
		}
		if segment == "local" && i+1 < len(segments) && (segments[i+1] == "bin" || segments[i+1] == "man" || segments[i+1] == "cache" ||
			segments[i+1] == "lib" && i+2 < len(segments) && segments[i+2] == "perl5") {
			return true
		}
	}
	return false
}

// Implements: REQ-PERL-004, REQ-PERL-006, REQ-PERL-007
func newResolver(root string, all []*scan.File, p Plugin) *resolver {
	r := &resolver{files: map[string]bool{}, byDirectory: map[string]*project{}, packages: map[string][]string{}, suffixes: map[string][]string{}}
	repository := lang.OpenRoot(root)
	read := func(relative string) ([]byte, bool) {
		return repository.ReadBounded(repository.Join(relative))
	}
	projectOf := func(directory string) *project {
		governing := r.byDirectory[directory]
		if governing == nil {
			governing = &project{directory: directory, requirements: map[string]moduleRequirement{}, dists: map[string]moduleRequirement{}}
			r.byDirectory[directory] = governing
			r.projects = append(r.projects, governing)
		}
		return governing
	}
	manifests := map[string][]*manifest{}
	for _, f := range all {
		r.files[f.Path] = true
		if f.Binary || f.TooLarge || ignored(f.Path) {
			continue
		}
		base := path.Base(f.Path)
		directory := path.Dir(f.Path)
		if class := manifestClass(base); class != "" {
			if b, ok := read(f.Path); ok {
				manifests[directory] = append(manifests[directory], readManifest(class, base, b))
				projectOf(directory)
			}
			continue
		}
		if base == "cpanfile.snapshot" {
			projectOf(directory)
			continue
		}
		if strings.HasSuffix(f.Path, ".pm") {
			segments := strings.Split(f.Path, "/")
			for i := range segments {
				key := strings.Join(segments[i:], "/")
				r.suffixes[key] = append(r.suffixes[key], f.Path)
			}
		}
		if p.Claims(f) && f.Size <= lang.MaxParseSize {
			if b, ok := read(f.Path); ok {
				for _, packageName := range declaredPackages(b) {
					r.packages[packageName] = append(r.packages[packageName], f.Path)
				}
			}
		}
	}
	for _, project := range r.projects {
		// The snapshot is Carton's lock file; applications commit it, libraries
		// often ignore it, so it is read from disk.
		if b, ok := read(path.Join(project.directory, "cpanfile.snapshot")); ok {
			project.snap = readSnapshot(b)
		}
		ms := manifests[project.directory]
		// cpanfile first: it is what Carton installs from; then the build scripts
		// and the metadata generated from them.
		sort.SliceStable(ms, func(i, j int) bool { return len(ms[i].requirements) > 0 && len(ms[j].requirements) == 0 })
		for _, m := range ms {
			if project.name == "" {
				project.name = m.name
			}
			for _, requirement := range m.requirements {
				if requirement.relation == "conflicts" || requirement.module == "perl" {
					continue
				}
				if _, ok := project.requirements[requirement.module]; !ok {
					project.requirements[requirement.module] = requirement
				}
				d := project.distOf(requirement.module)
				if old, ok := project.dists[d]; !ok || old.relation != "requires" && requirement.relation == "requires" {
					project.dists[d] = requirement
				}
			}
		}
	}
	sort.SliceStable(r.projects, func(i, j int) bool { return lang.ShallowestFirst(r.projects[i].directory, r.projects[j].directory) })
	return r
}

// declaredPackages lists the packages (and classes) a source declares.
func declaredPackages(source []byte) []string {
	var out []string
	for _, s := range readSource(source).Symbols {
		if s.Kind == "class" {
			out = append(out, strings.SplitN(s.Name, "@", 2)[0])
		}
	}
	return out
}

// distOf is the distribution providing a module as this project knows it: by its
// snapshot, else by name.
func (p *project) distOf(module string) string {
	if p.snap != nil {
		if d, ok := p.snap.provides[module]; ok {
			return d
		}
	}
	return distOf(module)
}

// governing are the projects whose directory holds the file, nearest first; a file
// under none of them is governed by all, shallowest first.
func (r *resolver) governing(file string) []*project {
	var out []*project
	for i := len(r.projects) - 1; i >= 0; i-- {
		if lang.Within(file, r.projects[i].directory) {
			out = append(out, r.projects[i])
		}
	}
	if len(out) == 0 {
		return r.projects
	}
	return out
}

// distRoot is the directory of the nearest project above the file, else the root.
func (r *resolver) distRoot(file string) string {
	for i := len(r.projects) - 1; i >= 0; i-- {
		if lang.Within(file, r.projects[i].directory) {
			return r.projects[i].directory
		}
	}
	return "."
}

// Resolve maps one import.
//
// Implements: REQ-PERL-004, REQ-PERL-005, REQ-PERL-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	lines := strings.Split(rawImport.Name, "\n")
	switch lines[0] {
	case kindModule:
		return r.module(file, rawImport.Module, lines[1:], false)
	case kindIsa:
		return r.module(file, rawImport.Module, lines[1:], true)
	case kindFile:
		return r.file(file, rawImport.Module, lines[1:])
	case kindDependency:
		version, origin := "", ""
		if len(lines) > 1 {
			version = lines[1]
		}
		if len(lines) > 2 {
			origin = lines[2]
		}
		return r.dependency(file, rawImport.Module, version, origin)
	}
	return lang.Target{}
}

// libraryDirectories are the directories a file's use lib entries name, project-relative. An
// entry evalPath could not work out was left out by Extract.
//
// Implements: REQ-PERL-004, REQ-PERL-009
func (r *resolver) libraryDirectories(file string, libraries []string) []string {
	var out []string
	for _, l := range libraries {
		switch {
		case l == "" || path.IsAbs(l) || strings.Contains(l[1:], selfMarker):
		case strings.HasPrefix(l, selfMarker):
			out = append(out, path.Clean(path.Join(file, l[len(selfMarker):])))
		default:
			// relative to the working directory: the distribution's root (where
			// prove and make test run), the repository's, the file's own.
			for _, base := range []string{r.distRoot(file), ".", path.Dir(file)} {
				out = append(out, path.Clean(path.Join(base, l)))
			}
		}
	}
	return out
}

// roots are the directories a module is looked for under, in order: the file's use
// lib directories, its distribution's lib, root and t/lib, the lib of every
// directory above the file, and the repository's lib and root.
func (r *resolver) roots(file string, libraries []string) []string {
	out := r.libraryDirectories(file, libraries)
	d := r.distRoot(file)
	out = append(out, path.Join(d, "lib"), d, path.Join(d, "t/lib"))
	for directory := range lang.Ancestors(file) {
		out = append(out, path.Join(directory, "lib"))
	}
	return append(out, "lib", ".")
}

func (r *resolver) local(p string) bool {
	return p != ".." && !strings.HasPrefix(p, "../") && r.files[p] && !ignored(p)
}

// module resolves a module name: a project file under the roots, a core module, a
// distribution the governing manifests or snapshots name, a project file ending in
// the module's path or declaring its package, else an unresolved distribution.
// isa is a class named without loading it, which is looked up among the declared
// packages first.
//
// Implements: REQ-PERL-004, REQ-PERL-005, REQ-PERL-008
func (r *resolver) module(file, m string, libraries []string, isa bool) lang.Target {
	declaration := r.packages[m]
	if isa {
		if slices.Contains(declaration, file) {
			return lang.Target{} // the same file declares it
		}
		if len(declaration) == 1 {
			return lang.Target{Local: declaration[0]}
		}
	}
	relative := strings.ReplaceAll(m, "::", "/") + ".pm"
	for _, root := range r.roots(file, libraries) {
		if p := path.Join(root, relative); r.local(p) {
			if p == file {
				return lang.Target{}
			}
			return lang.Target{Local: p}
		}
	}
	governing := r.governing(file)
	if core[m] && !r.installed(governing, m) {
		return lang.Target{Ecosystem: ecosystemStd, Package: m}
	}
	if t, ok := r.declared(governing, m, ""); ok {
		return t
	}
	// A project file ending in the module's path, when only one does: modules
	// kept outside the usual roots. One-segment names (EV, DBI) are left alone:
	// any Foo/EV.pm would match.
	if files := r.suffixes[relative]; len(files) == 1 && strings.Contains(m, "::") && files[0] != file {
		return lang.Target{Local: files[0]}
	}
	if slices.Contains(declaration, file) {
		return lang.Target{}
	}
	if len(declaration) == 1 {
		return lang.Target{Local: declaration[0]}
	}
	// A module of the project's own distribution that is not in the checkout (made
	// at build time) is not a dependency.
	for _, project := range governing {
		if own := strings.ReplaceAll(project.name, "-", "::"); own != "" && (m == own || strings.HasPrefix(m, own+"::") || distOf(m) == project.name) {
			return lang.Target{}
		}
	}
	if t, ok := r.declared(r.projects, m, ""); ok {
		return t // declared by a project that does not govern the file
	}
	return lang.Target{Ecosystem: ecosystemCPAN, Package: distOf(m), Unresolved: true}
}

// installed reports whether a core module is installed from CPAN instead: a
// snapshot records it, or a manifest requires it (or another module of its
// distribution) with a version, which perl's own copy may not satisfy.
//
// Implements: REQ-PERL-005
func (r *resolver) installed(governing []*project, m string) bool {
	for _, project := range governing {
		if project.snap != nil {
			if _, ok := project.snap.provides[m]; ok {
				return true
			}
		}
		if requirement, ok := project.requirements[m]; ok && versioned(requirement.version) {
			return true
		}
		if requirement, ok := project.dists[project.distOf(m)]; ok && versioned(requirement.version) {
			return true
		}
	}
	return false
}

// versioned reports whether a requirement names a version other than "any".
func versioned(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && strings.Trim(v, "0.") != ""
}

// declared finds the distribution of a module among what the projects' snapshots
// provide and manifests require - the module's own, then the namespaces above it
// (Plack::Request is Plack's). version, when set, is the requirement being resolved
// (a manifest's own line).
func (r *resolver) declared(governing []*project, m, version string) (lang.Target, bool) {
	segments := strings.Split(m, "::")
	for _, project := range governing {
		if project.snap == nil {
			continue
		}
		for k := len(segments); k >= 1; k-- {
			if k < len(segments) && walkStop[segments[k-1]] {
				break
			}
			if d, ok := project.snap.provides[strings.Join(segments[:k], "::")]; ok {
				return r.target(governing, d, m, version), true
			}
		}
	}
	for _, c := range candidates(m) {
		for _, project := range governing {
			_, declared := project.dists[c]
			_, locked := project.snap.dist(c)
			if declared || locked {
				return r.target(governing, c, m, version), true
			}
		}
	}
	return lang.Target{}, false
}

func (s *snapshot) dist(name string) (*snapDist, bool) {
	if s == nil {
		return nil, false
	}
	d, ok := s.dists[name]
	return d, ok
}

// target is a distribution as the projects require it and their snapshots pin it.
// A snapshot pins; "== 1.2" pins; a bare version is a minimum and floats, shown as
// ">= 1.2"; a range floats as written; no version (or 0) floats.
//
// Implements: REQ-PERL-007, REQ-PERL-008
func (r *resolver) target(governing []*project, dist, m, version string) lang.Target {
	t := lang.Target{Ecosystem: ecosystemCPAN, Package: dist}
	if version == "" {
		for _, project := range governing {
			if requirement, ok := project.dists[dist]; ok {
				version = requirement.version
				break
			}
		}
	}
	requested := requirement(version)
	for _, project := range governing {
		if d, ok := project.snap.dist(dist); ok {
			t.Version, t.Pinned = d.version, true
			if requested != "" && strings.TrimPrefix(requested, "== ") != d.version {
				t.Requested = requested
			}
			return t
		}
	}
	switch v := strings.TrimSpace(version); {
	case strings.HasPrefix(v, "=="):
		t.Version = strings.TrimSpace(strings.TrimPrefix(v, "=="))
		t.Pinned = lang.Pinned(t.Version)
		t.Floating = !t.Pinned
	case !versioned(v):
		t.Floating = true
	default:
		t.Version, t.Floating = requested, true
	}
	return t
}

// requirement shows a version requirement as CPAN::Meta reads it: a bare version is
// a minimum.
func requirement(v string) string {
	v = strings.TrimSpace(v)
	if !versioned(v) {
		return ""
	}
	if c := v[0]; c >= '0' && c <= '9' || c == 'v' {
		return ">= " + v
	}
	return v
}

// file resolves require/do of a path: relative to the file for a path computed from
// it, else under the use lib directories, the distribution's and repository's root
// (the working directory) and the file's own directory.
//
// Implements: REQ-PERL-004
func (r *resolver) file(file, p string, libraries []string) lang.Target {
	var candidates []string
	switch {
	case path.IsAbs(p) || strings.Contains(p[1:], selfMarker):
		return lang.Target{}
	case strings.HasPrefix(p, selfMarker):
		candidates = []string{path.Clean(path.Join(file, p[len(selfMarker):]))}
	default:
		for _, d := range append(r.libraryDirectories(file, libraries), r.distRoot(file), ".", path.Dir(file)) {
			candidates = append(candidates, path.Clean(path.Join(d, p)))
		}
	}
	for _, c := range candidates {
		if r.local(c) {
			return lang.Target{Local: c}
		}
	}
	return lang.Target{}
}

// dependency resolves a manifest's requirement of a module to its distribution; a core
// module required without a version is perl's own.
//
// Implements: REQ-PERL-005, REQ-PERL-006, REQ-PERL-008
func (r *resolver) dependency(file, m, version, origin string) lang.Target {
	governing := r.governing(file)
	if core[m] && !r.installed(governing, m) {
		return lang.Target{Ecosystem: ecosystemStd, Package: m}
	}
	if origin != "" {
		return lang.Target{Ecosystem: ecosystemCPAN, Package: distOf(m), Origin: origin, Version: requirement(version)}
	}
	if t, ok := r.declared(governing, m, version); ok {
		return t
	}
	return r.target(governing, distOf(m), m, version)
}

// Dependencies answers --resolve-depth from cpanfile.snapshot, which records what
// every installed distribution requires: each required module becomes the
// distribution the snapshot says provides it (pinned), a core module nothing
// installed is left out, and any other the distribution its name gives.
//
// Implements: REQ-PERL-007
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemCPAN {
		return nil
	}
	for _, project := range r.projects {
		d, ok := project.snap.dist(t.Package)
		if !ok || t.Pinned && t.Version != d.version {
			continue
		}
		out := []lang.Target{}
		seen := map[string]bool{t.Package: true}
		for _, required := range d.requires {
			if required.module == "perl" {
				continue
			}
			name, ok := project.snap.provides[required.module]
			switch {
			case ok:
			case core[required.module]:
				continue
			default:
				name = distOf(required.module)
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			dependencyTarget := lang.Target{Ecosystem: ecosystemCPAN, Package: name}
			if snapshotDist, ok := project.snap.dist(name); ok {
				dependencyTarget.Version, dependencyTarget.Pinned = snapshotDist.version, true
			} else if v := requirement(required.version); v != "" {
				dependencyTarget.Version, dependencyTarget.Floating = v, true
			} else {
				dependencyTarget.Floating = true
			}
			out = append(out, dependencyTarget)
		}
		return out
	}
	return nil
}
