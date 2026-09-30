package haxe

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// declaration is a library a manifest declares, with the version it asks for ("" for
// any).
type declaration struct {
	name, version string
}

// manifest is what one build file (.hxml, haxelib.json, project file) says.
type manifest struct {
	file       string
	libraries  []declaration
	classPaths []string // class paths, relative to the repository
	owner      string   // a haxelib.json's own library name
}

type resolver struct {
	lang.Layout
	modules     map[string][]string // module (a.b.C) -> files declaring it
	packages    map[string][]string // package (a.b) -> its modules' files
	packageOf   map[string]string   // module file -> the package it declares
	firsts      map[string]bool     // first segments of the repository's packages
	manifests   map[string]*manifest
	byDirectory map[string][]*manifest  // directory -> manifests in it
	all         []*manifest             // every manifest, shallowest first
	own         map[string]string       // library the repository is (haxelib.json name) -> its class path
	lix         map[string]*lixScope    // directory with haxe_libraries/ -> its pins
	lixFile     map[string]*lixLibrary  // haxe_libraries/<name>.hxml -> its pin
	installed   map[string]*installed   // lower-case library name -> as installed
	limeFiles   map[string]bool         // project files that are Lime's
	probed      lang.Memo[string, bool] // absolute path -> it exists
}

// Implements: REQ-HAXE-004, REQ-HAXE-005, REQ-HAXE-006, REQ-HAXE-007, REQ-HAXE-008
func newResolver(root string, all []*scan.File, getenv func(string) string) *resolver {
	r := &resolver{Layout: lang.NewLayout(), modules: map[string][]string{},
		packages: map[string][]string{}, packageOf: map[string]string{}, firsts: map[string]bool{}, manifests: map[string]*manifest{},
		byDirectory: map[string][]*manifest{}, own: map[string]string{}, lix: map[string]*lixScope{},
		lixFile: map[string]*lixLibrary{}, installed: map[string]*installed{}, limeFiles: map[string]bool{}}
	r.Directories["."] = true
	var sources, projects []*scan.File
	absolute := map[string]string{}
	for _, f := range all {
		if haxelibDirectory(f.Path) {
			continue
		}
		r.Add(f.Path)
		if !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize {
			absolute[f.Path] = f.AbsolutePath
		}
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
			continue
		}
		switch class(f.Path) {
		case classSource:
			sources = append(sources, f)
		case classHXML:
			if data, err := os.ReadFile(f.AbsolutePath); err == nil {
				h := readHXML(data)
				m := &manifest{file: f.Path}
				for _, l := range h.libraries {
					m.libraries = append(m.libraries, declaration{l.name, l.version})
				}
				for _, classPath := range h.classPaths {
					m.classPaths = append(m.classPaths, path.Join(path.Dir(f.Path), classPath))
				}
				r.manifests[f.Path] = m
			}
		case classHaxelib:
			if data, err := os.ReadFile(f.AbsolutePath); err == nil {
				if h, dependencies, ok := readHaxelib(data); ok {
					m := &manifest{file: f.Path, owner: h.Name}
					for _, d := range dependencies {
						m.libraries = append(m.libraries, declaration{d.name, d.version})
					}
					m.classPaths = []string{path.Join(path.Dir(f.Path), h.ClassPath)}
					r.manifests[f.Path] = m
				}
			}
		case classProject:
			projects = append(projects, f)
		}
	}
	for _, f := range projects {
		m := &manifest{file: f.Path}
		r.readProjectFile(m, f.Path, absolute, map[string]bool{})
		if len(m.libraries)+len(m.classPaths) > 0 || r.limeFiles[f.Path] {
			r.manifests[f.Path] = m
		}
	}
	for _, m := range r.manifests {
		r.all = append(r.all, m)
	}
	sort.Slice(r.all, func(i, j int) bool { return lang.ShallowestFirst(r.all[i].file, r.all[j].file) })
	for _, m := range r.all {
		r.byDirectory[path.Dir(m.file)] = append(r.byDirectory[path.Dir(m.file)], m)
		if m.owner != "" {
			if _, ok := r.own[strings.ToLower(m.owner)]; !ok {
				directory := path.Dir(m.file)
				if len(m.classPaths) > 0 && r.Directories[m.classPaths[0]] {
					directory = m.classPaths[0]
				}
				r.own[strings.ToLower(m.owner)] = directory
			}
		}
	}
	r.index(sources)
	r.readLix(root, all, getenv)
	r.readInstalled(root, getenv)
	return r
}

// readProjectFile reads a Lime project file into m, and the XML files it
// includes (<include path="other.xml"/>), which Lime reads as part of it.
func (r *resolver) readProjectFile(m *manifest, relative string, absolute map[string]string, seen map[string]bool) {
	if seen[relative] || len(seen) >= 16 || absolute[relative] == "" {
		return
	}
	seen[relative] = true
	data, err := os.ReadFile(absolute[relative])
	if err != nil || !limeProject(data) {
		return
	}
	r.limeFiles[relative] = true
	p := readProject(data)
	for _, d := range p.libraries {
		m.libraries = append(m.libraries, declaration{d.name, d.version})
	}
	for _, s := range p.sources {
		m.classPaths = append(m.classPaths, path.Join(path.Dir(relative), s))
	}
	for _, rawImport := range p.rawImports {
		if rawImport.Name == kindFile && strings.HasSuffix(rawImport.Module, ".xml") {
			r.readProjectFile(m, path.Join(path.Dir(relative), rawImport.Module), absolute, seen)
		}
	}
}

// index reads the package every module declares, concurrently: the module a.b.C
// is the file C.hx declaring package a.b, wherever its class path is.
func (r *resolver) index(sources []*scan.File) {
	packages := make([]string, len(sources))
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				packages[i] = packageOf(sources[i].AbsolutePath)
			}
		}()
	}
	for i := range sources {
		next <- i
	}
	close(next)
	wg.Wait()
	for i, f := range sources {
		name := strings.TrimSuffix(path.Base(f.Path), ".hx")
		if !upper(name) {
			continue // import.hx and other files that are no module
		}
		module := name
		if packages[i] != "" {
			module = packages[i] + "." + name
			first, _, _ := strings.Cut(packages[i], ".")
			r.firsts[first] = true
		}
		r.modules[module] = append(r.modules[module], f.Path)
		r.packages[packages[i]] = append(r.packages[packages[i]], f.Path)
		r.packageOf[f.Path] = packages[i]
	}
	for _, m := range []map[string][]string{r.modules, r.packages} {
		for _, list := range m {
			sort.Strings(list)
		}
	}
}

// packageOf reads the package a module declares from the head of its file.
func packageOf(absolute string) string {
	f, err := os.Open(absolute)
	if err != nil {
		return ""
	}
	defer f.Close()
	head := make([]byte, 64<<10)
	n, _ := io.ReadFull(f, head)
	return readPackage(head[:n])
}

// Resolve maps an import to a file of the repository, the standard library or
// a haxelib library.
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	directory := path.Dir(file)
	switch rawImport.Name {
	case kindImport, kindUsing, kindReference:
		return r.module(file, rawImport.Module, rawImport.Name == kindReference)
	case kindLibrary:
		name, version, _ := strings.Cut(rawImport.Module, ":")
		return r.libraryTarget(file, name, &declaration{name, version})
	case kindLix:
		if l := r.lixFile[file]; l != nil {
			return l.target()
		}
		return lang.Target{}
	case kindCP:
		if d := path.Join(directory, rawImport.Module); lang.Inside(d) && (r.Directories[d] || d == ".") {
			return lang.Target{Local: d}
		}
	case kindHXML, kindFile:
		if f := path.Join(directory, rawImport.Module); lang.Inside(f) && (r.Has(f)) {
			return lang.Target{Local: f}
		}
	case kindMain:
		return r.main(file, rawImport.Module)
	}
	return lang.Target{}
}

// main resolves a build's main class under the class paths it names, else
// wherever the module is.
//
// Implements: REQ-HAXE-005
func (r *resolver) main(file, module string) lang.Target {
	relative := strings.ReplaceAll(module, ".", "/") + ".hx"
	var classPaths []string
	if m := r.manifests[file]; m != nil {
		classPaths = m.classPaths
	}
	for _, classPath := range append(classPaths, path.Dir(file), path.Join(path.Dir(file), "src")) {
		if f := path.Join(classPath, relative); r.Files[f] {
			return lang.Target{Local: f}
		}
	}
	if f := r.local(file, strings.Split(module, ".")); f != "" {
		return lang.Target{Local: f}
	}
	return lang.Target{}
}

// local is the repository's file of the module an import names (the longest
// leading module path: a.b.C.D is the type D of a.b.C), the one nearest the
// importing file when several class paths have it.
func (r *resolver) local(file string, segments []string) string {
	for n := len(segments); n >= 1; n-- {
		if !upper(segments[n-1]) {
			continue
		}
		if list := r.modules[strings.Join(segments[:n], ".")]; len(list) > 0 {
			return nearest(file, list)
		}
	}
	return ""
}

// nearest is the file sharing the longest directory prefix with from, the
// first in path order on a tie; from itself only when nothing else is there.
func nearest(from string, list []string) string {
	best, bestN := "", -1
	for _, f := range list {
		if f == from {
			continue
		}
		if n := lang.CommonDirectories(from, f); n > bestN {
			best, bestN = f, n
		}
	}
	if best == "" && len(list) > 0 {
		return list[0]
	}
	return best
}

// module resolves an import, a using or a qualified name.
//
// Implements: REQ-HAXE-004, REQ-HAXE-007
func (r *resolver) module(file, module string, reference bool) lang.Target {
	wild := strings.HasSuffix(module, ".*")
	segments := strings.Split(strings.TrimSuffix(module, ".*"), ".")
	if segments[0] == "std" && len(segments) > 1 {
		segments = segments[1:] // std.Any, std.format.Data: from the root package
	}
	if f := r.local(file, segments); f != "" {
		if f == file {
			return lang.Target{}
		}
		return lang.Target{Local: f}
	}
	if wild {
		if list := r.packages[strings.Join(segments, ".")]; len(list) > 0 && segments[0] != "" {
			return lang.Target{Local: path.Dir(nearest(file, list))}
		}
	}
	if name := r.installedFor(file, segments, wild); name != "" {
		return r.library(file, name)
	}
	library, k, std := table(segments)
	declarations := r.declared(file)
	spelled, ks := spell(declarations, segments)
	if spelled != "" && ks > k {
		return r.library(file, spelled)
	}
	if std {
		return lang.Target{Ecosystem: ecosystemStd, Package: stdName(segments)}
	}
	if library != "" {
		for _, d := range declarations {
			if strings.EqualFold(d.name, library) {
				return r.library(file, d.name)
			}
		}
		if r.known(file, library) {
			return r.library(file, library)
		}
		if reference || r.firsts[segments[0]] {
			return lang.Target{}
		}
		return lang.Target{Ecosystem: ecosystemHaxelib, Package: library, Unresolved: true}
	}
	if spelled != "" {
		return r.library(file, spelled)
	}
	if reference || len(segments) == 1 || r.firsts[segments[0]] || !lower(segments[0]) {
		return lang.Target{} // the repository's own module, missing; or no package at all
	}
	return lang.Target{Ecosystem: ecosystemHaxelib, Package: segments[0], Unresolved: true}
}

// library is the haxelib library a module belongs to; nothing when that is
// the repository's own library, whose file is missing.
func (r *resolver) library(file, name string) lang.Target {
	if _, ok := r.own[strings.ToLower(name)]; ok {
		return lang.Target{}
	}
	return r.libraryTarget(file, name, nil)
}

// spell is the declared library the leading package segments of a module
// spell (tink.core is tink_core, thx.promise thx.promise, hxnodejs hxnodejs)
// and how many segments it took, longest first.
func spell(declarations []declaration, segments []string) (string, int) {
	packageLength := 0
	for packageLength < len(segments) && lower(segments[packageLength]) {
		packageLength++
	}
	for n := packageLength; n >= 1; n-- {
		for _, separator := range []string{"_", "-", ".", ""} {
			want := strings.ToLower(strings.Join(segments[:n], separator))
			for _, d := range declarations {
				if strings.ToLower(d.name) == want {
					return d.name, n
				}
			}
		}
	}
	return "", 0
}

// scope lists the manifests governing a file: those in its directory and its
// ancestors, nearest first; every manifest when there are none.
func (r *resolver) scope(file string) []*manifest {
	var out []*manifest
	for d := range lang.Ancestors(file) {
		out = append(out, r.byDirectory[d]...)
	}
	if len(out) == 0 {
		return r.all
	}
	return out
}

// declared lists the libraries a file's manifests declare, nearest first, and
// the libraries lix pins for it.
func (r *resolver) declared(file string) []declaration {
	var out []declaration
	seen := map[string]bool{}
	for _, m := range r.scope(file) {
		for _, d := range m.libraries {
			if k := strings.ToLower(d.name); !seen[k] {
				seen[k] = true
				out = append(out, d)
			}
		}
	}
	if s := r.lixScope(file); s != nil {
		for _, k := range s.names {
			if !seen[k] {
				seen[k] = true
				out = append(out, declaration{name: s.libraries[k].name})
			}
		}
	}
	return out
}

// known reports whether a library is pinned by lix or installed for a file.
func (r *resolver) known(file, name string) bool {
	k := strings.ToLower(name)
	if s := r.lixScope(file); s != nil && s.libraries[k] != nil {
		return true
	}
	return r.installed[k] != nil
}

// libraryTarget is a haxelib library as the file's manifests and lix pin it. own
// is the declaration of the manifest being resolved, which comes first.
//
// Implements: REQ-HAXE-006
func (r *resolver) libraryTarget(file, name string, own *declaration) lang.Target {
	k := strings.ToLower(name)
	if directory, ok := r.own[k]; ok {
		return lang.Target{Local: directory} // the repository's own library
	}
	d := own
	if d == nil || d.version == "" {
	search:
		for _, m := range r.scope(file) {
			for _, x := range m.libraries {
				if strings.ToLower(x.name) == k && (d == nil || x.version != "") {
					d = &x
					if x.version != "" {
						break search // the nearest manifest asking for a version
					}
				}
			}
		}
	}
	if own != nil {
		name = own.name
	} else if d != nil {
		name = d.name
	}
	if s := r.lixScope(file); s != nil && s.libraries[k] != nil {
		t := s.libraries[k].target()
		if d != nil && d.version != "" && d.version != t.Version && t.Local == "" {
			t.Requested = d.version
		}
		return t
	}
	t := lang.Target{Ecosystem: ecosystemHaxelib, Package: name}
	if d != nil && d.version != "" {
		pinRule(&t, d.version)
		return t
	}
	in := r.installed[k]
	switch {
	case in != nil:
		t.Version, t.Floating = in.version, true
		if d == nil {
			t.Package = in.name
		}
	case d != nil:
		t.Floating = true
	default:
		t.Unresolved = true
	}
	return t
}

// pinRule reads a haxelib version: an exact version pins, git:<url>#<commit>
// pins, a tag (#v1.2.3) is shown, neither pinned nor floating, and a branch,
// a git URL alone or no version floats. A git server other than the public
// forges is the library's origin.
//
// Implements: REQ-HAXE-006
func pinRule(t *lang.Target, v string) {
	v = strings.TrimSpace(v)
	if vcs, rest, ok := strings.Cut(v, ":"); ok && (vcs == "git" || vcs == "hg") {
		url, reference, _ := strings.Cut(rest, "#")
		if !lang.PublicOrUnnamed(url) {
			t.Origin = url
		}
		switch {
		case lang.Commit(reference):
			t.Version, t.Pinned = reference, true
		case tagLike(reference):
			t.Version = reference
		default:
			t.Version, t.Floating = reference, true
		}
		return
	}
	if lang.Pinned(v) {
		t.Version, t.Pinned = v, true
		return
	}
	t.Version, t.Floating = v, true
}

func tagLike(reference string) bool {
	s := strings.TrimPrefix(reference, "v")
	return s != "" && s[0] >= '0' && s[0] <= '9'
}

// Expand turns `import a.b.*` of a package of the repository into one import
// per module of the package, in path order and capped, and a module's import.hx
// import into the import.hx files that apply to it.
//
// Implements: REQ-HAXE-004
func (r *resolver) Expand(file string, rawImport lang.RawImport) ([]lang.Import, bool) {
	if rawImport.Name == kindHx {
		return r.importHx(file, rawImport), true
	}
	if (rawImport.Name != kindImport && rawImport.Name != kindUsing) || !strings.HasSuffix(rawImport.Module, ".*") {
		return nil, false
	}
	packageName := strings.TrimSuffix(rawImport.Module, ".*")
	segments := strings.Split(packageName, ".")
	if upper(segments[len(segments)-1]) || r.local(file, segments) != "" {
		return nil, false // a module's fields: import a.b.C.*
	}
	list := r.packages[packageName]
	// One file per module: several class paths may declare the same one.
	byModule := map[string][]string{}
	var names []string
	for _, f := range list {
		n := path.Base(f)
		if byModule[n] == nil {
			names = append(names, n)
		}
		byModule[n] = append(byModule[n], f)
	}
	sort.Strings(names)
	var out []lang.Import
	for _, n := range names {
		f := nearest(file, byModule[n])
		if f == file {
			continue
		}
		if len(out) == maxExpand {
			break
		}
		out = append(out, lang.Import{Spec: rawImport.Spec + " (" + f + ")", Line: rawImport.Line, Target: lang.Target{Local: f}})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

const maxExpand = 100

// importHx lists the import.hx files whose imports and usings the compiler
// applies to a module: one in the module's directory or a directory above it
// up to its class path (the directory its package starts in), nearest first.
// A module whose directory does not spell its package takes its own
// directory's only; import.hx itself takes none.
//
// Implements: REQ-HAXE-004
func (r *resolver) importHx(file string, rawImport lang.RawImport) []lang.Import {
	if path.Base(file) == "import.hx" {
		return nil
	}
	directory := path.Dir(file)
	root := directory
	if packageName := r.packageOf[file]; packageName != "" {
		packagePath := strings.ReplaceAll(packageName, ".", "/")
		switch {
		case directory == packagePath:
			root = "."
		case strings.HasSuffix(directory, "/"+packagePath):
			root = strings.TrimSuffix(directory, "/"+packagePath)
		}
	}
	var out []lang.Import
	for d := range lang.DirectoryAndAncestors(directory) {
		if f := path.Join(d, "import.hx"); r.Files[f] {
			out = append(out, lang.Import{Spec: rawImport.Spec + " (" + f + ")", Line: rawImport.Line, Target: lang.Target{Local: f}})
		}
		if d == root {
			break
		}
	}
	return out
}

// Dependencies lists what a library depends on: the -lib lines of its lix pin,
// else the haxelib.json of the version installed.
//
// Implements: REQ-HAXE-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecosystemHaxelib {
		return nil
	}
	k := strings.ToLower(t.Package)
	for _, directory := range lang.SortedKeys(r.lix) {
		s := r.lix[directory]
		l := s.libraries[k]
		if l == nil {
			continue
		}
		var out []lang.Target
		for _, dependency := range l.dependencies {
			if dl := s.libraries[strings.ToLower(dependency)]; dl != nil {
				out = append(out, dl.target())
			} else {
				out = append(out, lang.Target{Ecosystem: ecosystemHaxelib, Package: dependency, Floating: true})
			}
		}
		return out
	}
	in := r.installed[k]
	if in == nil {
		return nil
	}
	var out []lang.Target
	for _, d := range in.dependencies {
		dependencyTarget := lang.Target{Ecosystem: ecosystemHaxelib, Package: d.name}
		if d.version != "" {
			pinRule(&dependencyTarget, d.version)
		} else if x := r.installed[strings.ToLower(d.name)]; x != nil {
			dependencyTarget.Version, dependencyTarget.Floating = x.version, true
		} else {
			dependencyTarget.Floating = true
		}
		out = append(out, dependencyTarget)
	}
	return out
}

// Installed reports whether a library's dependencies come from what haxelib
// installed on this machine rather than from lix's pins.
func (r *resolver) Installed(t lang.Target) bool {
	if t.Ecosystem != ecosystemHaxelib {
		return false
	}
	k := strings.ToLower(t.Package)
	for _, s := range r.lix {
		if s.libraries[k] != nil {
			return false
		}
	}
	return r.installed[k] != nil
}

// exists reports whether an absolute path exists through files, remembered.
func (r *resolver) exists(files lang.Root, p string) bool {
	return r.probed.Get(p, func(p string) bool {
		_, err := files.Stat(p)
		return err == nil
	})
}

// installedFor is the library installed for a file (lix's cache, a haxelib
// repository) whose class path has the module, "" when none has.
func (r *resolver) installedFor(file string, segments []string, wild bool) string {
	// lix's class paths are in its cache on this machine; a local library's
	// are read through the repository's Root.
	try := func(files lang.Root, classPaths []string) bool {
		for _, classPath := range classPaths {
			if wild && r.exists(files, filepath.Join(classPath, filepath.Join(segments...))) {
				return true
			}
			for n := len(segments); n >= 1; n-- {
				if upper(segments[n-1]) && r.exists(files, filepath.Join(classPath, filepath.Join(segments[:n]...)+".hx")) {
					return true
				}
			}
		}
		return false
	}
	if len(segments) > maxSegments {
		return ""
	}
	seen := map[string]bool{}
	s := r.lixScope(file)
	for _, d := range r.declared(file) {
		k := strings.ToLower(d.name)
		seen[k] = true
		if s != nil && s.libraries[k] != nil {
			if try(lang.Machine, s.libraries[k].absolute) {
				return s.libraries[k].name
			}
		} else if in := r.installed[k]; in != nil && try(in.files, in.classPaths) {
			return in.name
		}
	}
	for _, k := range lang.SortedKeys(r.installed) {
		if in := r.installed[k]; !seen[k] && in.local && try(in.files, in.classPaths) {
			return in.name
		}
	}
	return ""
}
