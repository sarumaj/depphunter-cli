package cmake

import (
	"cmp"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Markers an evaluated path starts with: a path from the project root (what
// ${CMAKE_CURRENT_SOURCE_DIR} and its kin expand to), and the current source
// directory of a file that is not a CMakeLists.txt, which depends on who includes
// the file.
const (
	markRoot    = "\x02"
	markCurrent = "\x01"
)

const lists = "CMakeLists.txt"

type fetchReference struct {
	file      string
	rawImport lang.RawImport
}

type resolver struct {
	files  map[string]bool
	byBase map[string][]string // file name -> the project files so named
	// variables are the variables each CMake file leaves set, with the directory
	// variables of the file itself already expanded.
	variables        map[string]map[string]string
	projects         map[string]string // lower-case project name -> its directory
	projectDirectory map[string]string // directory -> the first project its CMakeLists.txt declares
	// modules are the directories every CMake file puts on CMAKE_MODULE_PATH: where a
	// module is looked for from a file whose own directories name none.
	modules  []string
	fetches  map[string][]fetchReference // lower-case name -> declarations, shallowest file first
	packages cpp.Packages
	content  []cpp.Fetched // the fetched packages, for the includes of their headers
}

// Implements: REQ-CMAKE-003
func newResolver(all []*scan.File) *resolver {
	r := &resolver{
		files: map[string]bool{}, byBase: map[string][]string{}, variables: map[string]map[string]string{},
		projects: map[string]string{}, projectDirectory: map[string]string{}, fetches: map[string][]fetchReference{},
		packages: cpp.ReadPackages(all),
	}
	var cmake []*scan.File
	for _, f := range all {
		r.files[f.Path] = true
		r.byBase[path.Base(f.Path)] = append(r.byBase[path.Base(f.Path)], f.Path)
		if (Plugin{}).Claims(f) && !presets(f.Path) && !f.TooLarge && f.Size <= lang.MaxParseSize {
			cmake = append(cmake, f)
		}
	}
	slices.SortFunc(cmake, func(a, b *scan.File) int {
		return cmp.Or(strings.Count(a.Path, "/")-strings.Count(b.Path, "/"), strings.Compare(a.Path, b.Path))
	})
	infos := map[string]*fileInfo{}
	for _, f := range cmake {
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		info := analyze(lex(source))
		infos[f.Path] = info
		directory := path.Dir(f.Path)
		if path.Base(f.Path) == lists && len(info.projects) > 0 {
			r.projectDirectory[directory] = info.projects[0]
			for _, p := range info.projects {
				if _, ok := r.projects[strings.ToLower(p)]; !ok {
					r.projects[strings.ToLower(p)] = directory
				}
			}
		}
		for name, rawImport := range info.fetches {
			r.fetches[name] = append(r.fetches[name], fetchReference{f.Path, rawImport})
		}
	}
	// Directory variables need every project known first (PROJECT_SOURCE_DIR,
	// <name>_SOURCE_DIR).
	for _, f := range cmake {
		info := infos[f.Path]
		if info == nil {
			continue
		}
		variables := make(map[string]string, len(info.variables))
		for k, v := range info.variables {
			variables[k], _ = expand(v, func(name string) (string, bool) { return r.builtin(f.Path, name) }, true)
		}
		r.variables[f.Path] = variables
	}
	for _, f := range cmake {
		r.modules = append(r.modules, r.modulePath(f.Path, false)...)
	}
	r.content = r.fetchedContent()
	r.packages.Fetch(r.content)
	return r
}

// fetchedContent is every package the project's CMake files fetch, under the names
// its headers may go by: the name it is declared under, its repository's name and
// owner-repository (nlohmann-json), so that find_package() and the sources' includes
// land on it as they land on a vcpkg or Conan package. Local content is the
// project's own files.
//
// Implements: REQ-CMAKE-007, REQ-CPP-017
func (r *resolver) fetchedContent() []cpp.Fetched {
	var out []cpp.Fetched
	for _, name := range slices.Sorted(maps.Keys(r.fetches)) {
		for _, reference := range r.fetches[name] {
			t := r.fetched(reference.file, reference.rawImport)
			if t.Ecosystem != ecosystemFetch {
				continue
			}
			names := []string{name}
			if parts := strings.Split(t.Package, "/"); len(parts) == 3 {
				names = append(names, parts[2], parts[1]+"-"+parts[2])
			}
			out = append(out, cpp.Fetched{Directory: path.Dir(reference.file), Names: names, Target: t})
		}
	}
	return out
}

// builtin is the value of CMake's own variable name in file: the directories of the
// file, of the project it belongs to and of the top-level project.
//
// Implements: REQ-CMAKE-003
func (r *resolver) builtin(file, name string) (string, bool) {
	directory := path.Dir(file)
	switch name {
	case "CMAKE_CURRENT_LIST_DIR":
		return markRoot + directory, true
	case "CMAKE_CURRENT_LIST_FILE":
		return markRoot + file, true
	case "CMAKE_CURRENT_SOURCE_DIR":
		if path.Base(file) == lists {
			return markRoot + directory, true
		}
		return markCurrent, true
	case "CMAKE_SOURCE_DIR", "CMAKE_HOME_DIRECTORY":
		return markRoot + r.top(directory), true
	case "PROJECT_SOURCE_DIR":
		return markRoot + r.project(directory), true
	case "PROJECT_NAME":
		if p, ok := r.projectDirectory[r.project(directory)]; ok {
			return p, true
		}
		return "", false
	}
	if p, ok := strings.CutSuffix(name, "_SOURCE_DIR"); ok {
		if d, ok := r.projects[strings.ToLower(p)]; ok {
			return markRoot + d, true
		}
	}
	return "", false
}

// top is the topmost directory above directory (or directory itself) with a CMakeLists.txt: the source
// directory of the build the file most likely belongs to.
func (r *resolver) top(directory string) string {
	top := "."
	for d := range lang.DirectoryAndAncestors(directory) {
		if r.files[path.Join(d, lists)] {
			top = d
		}
	}
	return top
}

// project is the nearest directory above directory (or directory itself) whose CMakeLists.txt declares
// a project, else the top.
func (r *resolver) project(directory string) string {
	for d := range lang.DirectoryAndAncestors(directory) {
		if _, ok := r.projectDirectory[d]; ok {
			return d
		}
	}
	return r.top(directory)
}

// chain is the files whose variables a file sees, nearest first: itself, then the
// CMakeLists.txt of its directory and of each directory above - the scopes
// add_subdirectory() nests, taken from the layout.
func (r *resolver) chain(file string) []string {
	out := []string{file}
	for d := range lang.Ancestors(file) {
		if p := path.Join(d, lists); p != file && r.files[p] {
			out = append(out, p)
		}
	}
	return out
}

// evalFrom expands s as seen from chain[from:]: a variable found in one file is
// expanded further from the files above it, so set(A ${A} x) reads the A above.
func (r *resolver) evalFrom(file string, chain []string, s string, from, depth int) (string, bool) {
	if depth > maxDepth {
		return "", false
	}
	return expand(s, func(name string) (string, bool) {
		for i := from; i < len(chain); i++ {
			if v, ok := r.variables[chain[i]][name]; ok {
				return r.evalFrom(file, chain, v, i+1, depth+1)
			}
		}
		return r.builtin(file, name)
	}, false)
}

// eval expands every variable reference in s as seen from file, failing when one is
// unknown or s holds what only the build knows (an environment variable, a
// generator expression, a configure_file @VAR@).
//
// Implements: REQ-CMAKE-003
func (r *resolver) eval(file, s string) (string, bool) {
	v, ok := r.evalFrom(file, r.chain(file), s, 0, 0)
	if !ok || strings.Contains(v, "$ENV{") || strings.Contains(v, "$<") || configVariable.MatchString(v) {
		return "", false
	}
	return v, true
}

var configVariable = regexp.MustCompile(`@[A-Za-z_][A-Za-z0-9_]*@`)

// paths are the project paths an evaluated path may name from file: one from the
// root, or else relative to the current source directory - the file's own for a
// CMakeLists.txt; for an included file, whose includer is unknown, its own
// directory and then each one above.
func (r *resolver) paths(file, s string) []string {
	if s == "" || strings.Contains(s, ";") {
		return nil
	}
	if rest, ok := strings.CutPrefix(s, markRoot); ok {
		if strings.ContainsAny(rest, markRoot+markCurrent) {
			return nil
		}
		return inside(path.Clean(rest))
	}
	if rest, ok := strings.CutPrefix(s, markCurrent); ok {
		s = strings.TrimLeft(rest, "/")
		if s == "" {
			s = "."
		}
	}
	if strings.ContainsAny(s, markRoot+markCurrent) || path.IsAbs(s) || (len(s) > 1 && s[1] == ':') {
		return nil // this machine's path
	}
	var out []string
	for d := range lang.Ancestors(file) {
		out = append(out, inside(path.Join(d, s))...)
		if path.Base(file) == lists {
			break
		}
	}
	return out
}

func inside(p string) []string {
	if p == ".." || strings.HasPrefix(p, "../") {
		return nil
	}
	return []string{p}
}

// local is the first candidate that is a project file other than file.
func (r *resolver) local(file string, candidates []string) lang.Target {
	for _, p := range candidates {
		if r.files[p] && p != file {
			return lang.Target{Local: p}
		}
	}
	return lang.Target{}
}

// Resolve maps an import to the project file it names or the package it uses.
//
// Implements: REQ-CMAKE-003, REQ-CMAKE-004, REQ-CMAKE-005, REQ-CMAKE-006, REQ-CMAKE-007
// Implements: REQ-CMAKE-009, REQ-CMAKE-011
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindSubdirectory:
		v, ok := r.eval(file, rawImport.Module)
		if !ok {
			return lang.Target{}
		}
		var candidates []string
		for _, d := range r.paths(file, v) {
			candidates = append(candidates, path.Join(d, lists))
		}
		return r.local(file, candidates)
	case kindSource, kindFile:
		v, ok := r.eval(file, rawImport.Module)
		if !ok {
			return lang.Target{}
		}
		return r.local(file, r.paths(file, v))
	case kindInclude:
		v, ok := r.eval(file, rawImport.Module)
		if !ok {
			return lang.Target{}
		}
		if strings.Contains(v, "/") || strings.HasSuffix(strings.ToLower(v), ".cmake") {
			return r.local(file, r.paths(file, v))
		}
		return r.module(file, v)
	case kindFind:
		return r.find(file, rawImport.Module)
	case kindGit, kindURL:
		return r.fetched(file, rawImport)
	case kindUse:
		return r.use(file, rawImport.Module)
	case kindPackage:
		return r.packageConfig(file, rawImport.Module)
	}
	return lang.Target{}
}

// modulePath is the directories on CMAKE_MODULE_PATH as file sees them: those its
// own and its directories' CMakeLists.txt add, then (with all) those any CMake file
// of the project adds.
func (r *resolver) modulePath(file string, all bool) []string {
	chain := r.chain(file)
	var out []string
	for i, f := range chain {
		v, ok := r.variables[f]["CMAKE_MODULE_PATH"]
		if !ok || v == "" {
			continue
		}
		for _, entry := range splitList(v) {
			if e, ok := r.evalFrom(f, chain, entry, i+1, 0); ok {
				out = append(out, r.paths(f, e)...)
			}
		}
	}
	if all {
		out = append(out, r.modules...)
	}
	return out
}

// module resolves include(Name): Name.cmake in a CMAKE_MODULE_PATH directory, a
// module CMake ships (cmake-std), else the one project file so named - the nearest
// when there are several and one is nearer than the others. A module neither the
// project nor CMake has (one a package installs) is dropped.
//
// Implements: REQ-CMAKE-005
func (r *resolver) module(file, name string) lang.Target {
	for _, d := range r.modulePath(file, true) {
		if p := path.Join(d, name+".cmake"); r.files[p] && p != file {
			return lang.Target{Local: p}
		}
	}
	lower := strings.ToLower(name)
	if std, ok := stdModules[lower]; ok {
		return lang.Target{Ecosystem: ecosystemStd, Package: std}
	}
	if strings.HasPrefix(name, "Find") {
		return lang.Target{Ecosystem: ecosystemStd, Package: name}
	}
	var best string
	bestLength, tie := -1, false
	for _, p := range r.byBase[name+".cmake"] {
		if p == file {
			continue
		}
		n := lang.CommonSubdirectories(file, p)
		switch {
		case n > bestLength:
			best, bestLength, tie = p, n, false
		case n == bestLength:
			tie = true
		}
	}
	if best == "" || tie {
		return lang.Target{}
	}
	return lang.Target{Local: best}
}

// find resolves find_package(Name) (spec "Name" or "Name/component"): a project the
// repository builds itself is its CMakeLists.txt; else the library is attributed as
// its headers are (cpp.Packages.Library) - a declared vcpkg or Conan package wins;
// else the project's own FindName.cmake on CMAKE_MODULE_PATH; else content the
// project fetches under that name; else a find module CMake ships for a tool or the
// platform (cmake-std); else the unresolved c-external library.
//
// Implements: REQ-CMAKE-006
func (r *resolver) find(file, spec string) lang.Target {
	written, component, _ := strings.Cut(spec, "/")
	name, ok := r.eval(file, written)
	if !ok && component != "" && strings.HasPrefix(written, "Qt${") {
		name, ok = "Qt", true // Qt of a major version only the configure step knows
	}
	if !ok || name == "" {
		return lang.Target{}
	}
	lower := strings.ToLower(name)
	if directory, ok := r.projects[lower]; ok && directory != r.project(path.Dir(file)) {
		if t := r.local(file, []string{path.Join(directory, lists)}); t.Local != "" {
			return t
		}
	}
	include, extra := representative(name, component)
	t := r.packages.Library(file, include, extra...)
	if !t.Unresolved {
		return t
	}
	// The project's own find module (cmake/FindLLVMAr.cmake) is what CMake runs.
	for _, d := range r.modulePath(file, true) {
		if p := path.Join(d, "Find"+name+".cmake"); r.files[p] {
			return lang.Target{Local: p}
		}
	}
	if component == "" {
		if f := r.use(file, lower); f.Ecosystem != "" {
			return f
		}
	}
	if std, ok := systemFinds[lower]; ok {
		return lang.Target{Ecosystem: ecosystemStd, Package: "Find" + std}
	}
	return t
}

// use resolves FetchContent_MakeAvailable(name) and its kin to the content declared
// under that name: in the same file, else in a file of a directory above, else the
// shallowest declaration.
//
// Implements: REQ-CMAKE-007
func (r *resolver) use(file, name string) lang.Target {
	references := r.fetches[strings.ToLower(name)]
	if len(references) == 0 {
		return lang.Target{}
	}
	best := references[0]
	for _, reference := range references {
		if reference.file == file {
			best = reference
			break
		}
	}
	if best.file != file {
		for _, reference := range references {
			if d := path.Dir(reference.file); d == "." || strings.HasPrefix(file, d+"/") {
				best = reference
				break
			}
		}
	}
	return r.fetched(best.file, best.rawImport)
}

var (
	// GitHub and GitLab archives of a reference and GitHub release assets.
	archiveReference = regexp.MustCompile(`^((?:github\.com|gitlab\.com)/[^/]+/[^/]+?)(?:/-)?/archive/(refs/tags/|refs/heads/)?(?:[^/]+/)?([^/]+?)(\.zip|\.tar\.gz|\.tgz|\.tar\.bz2|\.tar\.xz)$`)
	releaseReference = regexp.MustCompile(`^(github\.com/[^/]+/[^/]+)/releases/download/([^/]+)/`)
)

// fetched is content a FetchContent_Declare(), ExternalProject_Add() or CPM call
// declared in file: a git repository named by its URL (lang.RepositoryName) at a GIT_TAG
// - a commit pins it, a branch or no tag floats, a tag (which can be moved) is
// neither - or a download named by its URL, pinned by a URL_HASH (or a commit's
// archive). A GitHub or GitLab archive or release asset is named by its repository,
// with the reference as the version. A URL naming a project file is that file.
//
// Implements: REQ-CMAKE-007
func (r *resolver) fetched(file string, rawImport lang.RawImport) lang.Target {
	rawURL, rawReference, _ := strings.Cut(rawImport.Module, "\n")
	u, ok := r.eval(file, rawURL)
	if !ok || u == "" {
		return lang.Target{}
	}
	reference, referenceKnown := r.eval(file, rawReference)
	if !strings.Contains(u, "://") && !(rawImport.Name == kindGit && strings.Contains(u, "@") && strings.Contains(u, ":")) {
		// A path: a tarball in the repository, or a directory of it - a checkout
		// of the repository itself (tests of a project's own FetchContent use) -
		// which is the CMakeLists.txt it builds.
		var candidates []string
		for _, p := range r.paths(file, u) {
			candidates = append(candidates, p, path.Join(p, lists))
		}
		return r.local(file, candidates)
	}
	t := lang.Target{Ecosystem: ecosystemFetch}
	if rawImport.Name == kindGit {
		t.Package = lang.RepositoryName(u)
		switch {
		case !referenceKnown:
		case reference == "":
			t.Floating = true
		case lang.Commit(reference):
			t.Version, t.Pinned = reference, true
		default:
			t.Version = reference
			_, t.Floating = branches[strings.ToLower(strings.TrimPrefix(reference, "origin/"))]
		}
		return t
	}
	u, _, _ = strings.Cut(u, "?")
	t.Package = lang.RepositoryName(u)
	head := false
	if m := archiveReference.FindStringSubmatch(t.Package); m != nil {
		t.Package, t.Version, head = m[1], m[3], m[2] == "refs/heads/"
	} else if m := releaseReference.FindStringSubmatch(t.Package); m != nil {
		t.Package, t.Version = m[1], m[2]
	}
	switch {
	case referenceKnown && reference != "":
		t.Pinned = true
		if t.Version == "" {
			t.Version = reference
		}
	case lang.Commit(t.Version):
		t.Pinned = true
	case t.Version == "" || head:
		t.Floating = true
	}
	return t
}

var packageVersionSuffix = regexp.MustCompile(`-[0-9][0-9.]*$`)

// packageConfig resolves a pkg_check_modules() module ("glib-2.0>=2.40"): the package a
// vcpkg or Conan manifest declares under its name, else the module of the
// pkg-config island with the constraint as its version; "=" pins.
//
// Implements: REQ-CMAKE-011
func (r *resolver) packageConfig(file, spec string) lang.Target {
	spec, ok := r.eval(file, spec)
	if !ok || spec == "" {
		return lang.Target{}
	}
	name, constraint := spec, ""
	if i := strings.IndexAny(spec, "<>="); i > 0 {
		name, constraint = strings.TrimSpace(spec[:i]), strings.TrimSpace(spec[i:])
	}
	base := packageVersionSuffix.ReplaceAllString(strings.TrimPrefix(name, "lib"), "")
	if t := r.packages.Library(file, base+"/", name); !t.Unresolved {
		return t
	}
	t := lang.Target{Ecosystem: ecosystemPackage, Package: name, Version: constraint}
	if v, ok := strings.CutPrefix(constraint, "="); ok && !strings.HasPrefix(v, "=") {
		t.Version, t.Pinned = strings.TrimSpace(v), true
	}
	return t
}
