package bazel

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

type resolver struct {
	root         string
	files        map[string]bool
	directories  map[string]bool
	build        map[string]string // package directory -> its BUILD file
	workspaces   map[string]*workspace
	packageFiles map[string][]string // package directory -> its files, relative to it
}

var workspaceFiles = []string{"MODULE.bazel", "REPO.bazel", "WORKSPACE.bazel", "WORKSPACE", "WORKSPACE.bzlmod"}

// newResolver reads every workspace's MODULE.bazel (with its lock file and
// includes), WORKSPACE files and .bzl files, and indexes the packages.
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, directories: map[string]bool{".": true},
		build: map[string]string{}, workspaces: map[string]*workspace{}, packageFiles: map[string][]string{}}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.directories[d]; d = path.Dir(d) {
			r.directories[d] = true
		}
		directory, base := path.Dir(f.Path), path.Base(f.Path)
		switch base {
		case "BUILD.bazel":
			r.build[directory] = f.Path // preferred over BUILD, as Bazel does
		case "BUILD":
			if r.build[directory] == "" {
				r.build[directory] = f.Path
			}
		}
		for _, w := range workspaceFiles {
			if base == w && !skipped(f.Path) && r.workspaces[directory] == nil {
				r.workspaces[directory] = newWorkspace(directory)
			}
		}
	}
	if r.workspaces["."] == nil {
		r.workspaces["."] = newWorkspace(".")
	}
	for _, directory := range sortedKeys(r.workspaces) {
		w := r.workspaces[directory]
		if source, ok := r.read(path.Join(directory, "MODULE.bazel")); ok {
			r.readModule(w, path.Join(directory, "MODULE.bazel"), source, 0)
		}
		r.readLock(w)
		for _, name := range []string{"WORKSPACE.bazel", "WORKSPACE", "WORKSPACE.bzlmod"} {
			if source, ok := r.read(path.Join(directory, name)); ok {
				r.readRules(w, source, true)
			}
		}
	}
	// Repository rules in .bzl macros, each read into its own workspace.
	var bzl []string
	for _, f := range all {
		if fileKind(f.Path) == kindBzl && readable(f) && !skipped(f.Path) {
			bzl = append(bzl, f.Path)
		}
	}
	sort.Strings(bzl)
	for _, p := range bzl {
		if source, ok := r.read(p); ok {
			r.readRules(r.workspaceOf(p), source, false)
		}
	}
	// Package contents for glob(): a file belongs to the nearest package above it;
	// a workspace root without a BUILD file ends the search.
	nearest := map[string]string{}
	var packageOf func(d string) string
	packageOf = func(d string) string {
		if p, ok := nearest[d]; ok {
			return p
		}
		p := ""
		switch {
		case r.build[d] != "":
			p = d
		case r.workspaces[d] != nil || d == ".":
		default:
			p = packageOf(path.Dir(d))
		}
		nearest[d] = p
		return p
	}
	for _, f := range all {
		if p := packageOf(path.Dir(f.Path)); p != "" {
			relative := f.Path
			if p != "." {
				relative = strings.TrimPrefix(f.Path, p+"/")
			}
			r.packageFiles[p] = append(r.packageFiles[p], relative)
		}
	}
	for _, list := range r.packageFiles {
		sort.Strings(list)
	}
	return r
}

func readable(f *scan.File) bool { return !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize }

// read reads a project file from disk (lock files are not always scanned: they
// may be ignored or generated).
func (r *resolver) read(p string) ([]byte, bool) {
	if p == "" || strings.HasPrefix(p, "../") || p == ".." {
		return nil, false
	}
	full := filepath.Join(r.root, filepath.FromSlash(p))
	info, err := os.Stat(full)
	if err != nil || info.IsDir() || info.Size() > 8*lang.MaxParseSize {
		return nil, false
	}
	source, err := os.ReadFile(full)
	return source, err == nil
}

// workspaceOf is the workspace a file belongs to: the nearest directory above it
// with a MODULE.bazel, WORKSPACE or REPO.bazel, else the project's root.
func (r *resolver) workspaceOf(file string) *workspace {
	for d := path.Dir(file); ; d = path.Dir(d) {
		if w := r.workspaces[d]; w != nil {
			return w
		}
		if d == "." || d == "/" {
			return r.workspaces["."]
		}
	}
}

// packageOf is the package a file is in, relative to its workspace: the nearest
// directory above it with a BUILD file, within the workspace.
func (r *resolver) packageOf(w *workspace, file string) string {
	d := path.Dir(file)
	for {
		if r.build[d] != "" || d == w.directory || d == "." {
			break
		}
		d = path.Dir(d)
	}
	if r.build[d] == "" {
		d = path.Dir(file) // a .bzl outside any package: its own directory
	}
	return relative(w.directory, d)
}

func relative(base, p string) string {
	if base == "." {
		if p == "." {
			return ""
		}
		return p
	}
	if p == base {
		return ""
	}
	return strings.TrimPrefix(p, base+"/")
}

// Resolve maps an import to a project file, a Bazel module or repository, or a
// package of the ecosystem a hub repository serves.
//
// Implements: REQ-BAZEL-003, REQ-BAZEL-004, REQ-BAZEL-006, REQ-BAZEL-007, REQ-BAZEL-009
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	w := r.workspaceOf(file)
	switch rawImport.Name {
	case importLoad, importLabel:
		return r.label(file, w, rawImport.Module)
	case importDependency:
		if d := w.byModule[rawImport.Module]; d != nil {
			return r.dependencyTarget(w, d)
		}
	case importRepository:
		if d := w.repositories[rawImport.Module]; d != nil {
			return r.repositoryTarget(w, d)
		}
	case importMaven:
		coordinate, hubName, _ := strings.Cut(rawImport.Module, "\n")
		t, ok := mavenTarget(coordinate)
		if !ok {
			return lang.Target{}
		}
		if h := w.hubs[hubName]; h != nil {
			if locked, ok := h.packages[normalizeMaven(t.Package)]; ok && h.locked[normalizeMaven(t.Package)] {
				if t.Version != "" && t.Version != locked.Version {
					locked.Requested = t.Version
				} else {
					locked.Requested = ""
				}
				return locked
			}
		}
		return t
	case importPip:
		hubName, name, _ := strings.Cut(rawImport.Module, "\n")
		return r.pypi(w, hubName, name)
	case importGoMod:
		if t, ok := w.goMods[rawImport.Module]; ok {
			return t
		}
		return lang.Target{Ecosystem: ecosystemGo, Package: rawImport.Module}
	case importCrate:
		for _, name := range sortedKeys(w.hubs) {
			if h := w.hubs[name]; h.ecosystem == ecosystemCrates {
				if t, ok := h.packages[normalizeCrate(rawImport.Module)]; ok {
					return t
				}
			}
		}
		return lang.Target{Ecosystem: ecosystemCrates, Package: rawImport.Module}
	}
	return lang.Target{}
}

// label resolves a label a file names.
func (r *resolver) label(file string, w *workspace, s string) lang.Target {
	l, ok := parseLabel(s)
	if !ok {
		return lang.Target{}
	}
	packageName := l.packageName
	if !l.absolute {
		packageName = r.packageOf(w, file)
	}
	if !l.hasRepository || l.repository == "" || !l.canonical && (l.repository == w.name || l.repository == w.repositoryName) ||
		l.canonical && moduleName(l.repository) == w.name {
		return r.local(file, w.directory, packageName, l.target)
	}
	return r.external(file, w, l)
}

// local resolves a label of a repository whose files are in the project at directory:
// the file it names, else the BUILD file of its package (a rule, or a file a rule
// generates), else a directory.
func (r *resolver) local(file, directory, packageName, target string) lang.Target {
	packageDirectory := path.Join(directory, packageName)
	if name, ok := strings.CutPrefix(target, "node_modules/"); ok {
		// rules_js links a package as //<importer>:node_modules/<name>.
		w := r.workspaceOf(path.Join(directory, "x"))
		for _, hubName := range sortedKeys(w.hubs) {
			if h := w.hubs[hubName]; h.ecosystem == ecosystemNPM {
				return npmTarget(h, name)
			}
		}
		return npmTarget(nil, name)
	}
	p := path.Join(packageDirectory, target)
	switch {
	case p == file:
		return lang.Target{}
	case r.files[p]:
		return lang.Target{Local: p}
	case r.build[packageDirectory] != "":
		if r.build[packageDirectory] == file {
			return lang.Target{} // a target of the file's own package
		}
		return lang.Target{Local: r.build[packageDirectory]}
	case r.directories[p] && p != ".":
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

var hubNames = map[string]bool{"maven": true, "pypi": true, "pip": true, "npm": true, "crates": true, "crate_index": true}

// builtin reports whether a repository is one Bazel provides itself.
//
// Implements: REQ-BAZEL-010
func builtin(repository string) bool {
	switch {
	case repository == "bazel_tools", repository == "local_jdk", repository == "remote_coverage_tools", repository == "_builtins",
		strings.HasPrefix(repository, "local_config_"), strings.HasPrefix(repository, "remotejdk"),
		strings.HasPrefix(repository, "remote_java_tools"):
		return true
	}
	return false
}

// external resolves a label of another repository by what declares it. A
// repository nothing in the workspace declares is not guessed at.
//
// Implements: REQ-BAZEL-004, REQ-BAZEL-011
func (r *resolver) external(file string, w *workspace, l label) lang.Target {
	repository := l.repository
	if l.canonical {
		repository = moduleName(repository)
		if d := w.byModule[repository]; d != nil {
			return r.dependencyLabel(file, w, d, l)
		}
	}
	if d := w.dependencies[repository]; d != nil {
		return r.dependencyLabel(file, w, d, l)
	}
	if h := w.hubs[repository]; h != nil && !strings.HasSuffix(l.target, ".bzl") {
		return r.hubLabel(w, repository, h, l)
	}
	if t, ok := w.goRepositories[repository]; ok {
		return t
	}
	if d := w.repositories[repository]; d != nil {
		if d.rule == "local_repository" || d.rule == "new_local_repository" {
			if directory := r.localPath(w, d.path); directory != "" {
				if t := r.local(file, directory, l.packageName, l.target); t != (lang.Target{}) {
					return t
				}
				return r.localRoot(directory)
			}
			return lang.Target{}
		}
		return r.repositoryTarget(w, d)
	}
	for _, name := range sortedKeys(w.hubs) { // pip's older per-package repositories: @pypi_requests//:pkg
		if w.hubs[name].ecosystem == ecosystemPyPI && strings.HasPrefix(repository, name+"_") {
			return r.pypi(w, name, strings.TrimPrefix(repository, name+"_"))
		}
	}
	if d := w.byModule[repository]; d != nil {
		// A module's own name where its repo_name was declared: a vendored copy of
		// another project (grpc's third_party/upb says @abseil-cpp).
		return r.dependencyLabel(file, w, d, l)
	}
	if extension, ok := w.extensionRepositories[repository]; ok {
		// A repository a module extension made: it comes with the module whose .bzl
		// defines the extension, or from the project's own extension.
		return r.label(w.extensionFile[repository], w, extension)
	}
	if builtin(repository) {
		return lang.Target{Ecosystem: ecosystemStd, Package: repository}
	}
	if w.hubs[repository] != nil || strings.HasSuffix(l.target, ".bzl") && hubNames[repository] {
		return lang.Target{} // a hub's generated .bzl (requirements.bzl, defs.bzl)
	}
	switch repository { // hub repositories by their conventional names
	case "maven":
		return r.hubLabel(w, repository, &hub{ecosystem: ecosystemMaven}, l)
	case "pypi", "pip":
		return r.hubLabel(w, repository, &hub{ecosystem: ecosystemPyPI}, l)
	case "npm":
		return r.hubLabel(w, repository, &hub{ecosystem: ecosystemNPM}, l)
	case "crates", "crate_index":
		return r.hubLabel(w, repository, &hub{ecosystem: ecosystemCrates}, l)
	}
	return lang.Target{Ecosystem: ecosystemBazel, Package: repository, Unresolved: true}
}

// dependencyLabel resolves a label of a bazel_dep's repository: the module, or a file of
// the project when a local_path_override puts it there.
func (r *resolver) dependencyLabel(file string, w *workspace, d *bazelDependency, l label) lang.Target {
	if o := w.over[d.name]; o != nil && o.kind == "local" {
		if directory := r.localPath(w, o.path); directory != "" {
			if t := r.local(file, directory, l.packageName, l.target); t != (lang.Target{}) {
				return t
			}
		}
	}
	return r.dependencyTarget(w, d)
}

// localPath is a local_repository's or local_path_override's directory in the
// project ("" outside it).
func (r *resolver) localPath(w *workspace, p string) string {
	if p == "" || path.IsAbs(p) || strings.HasPrefix(p, "~") {
		return ""
	}
	d := path.Join(w.directory, p)
	if d == ".." || strings.HasPrefix(d, "../") || !r.directories[d] {
		return ""
	}
	return d
}

// localRoot is what stands for a local repository as a whole: its MODULE.bazel or
// WORKSPACE file, else its top BUILD file, else the directory.
func (r *resolver) localRoot(directory string) lang.Target {
	for _, name := range append(append([]string{}, workspaceFiles...), "BUILD.bazel", "BUILD") {
		if p := path.Join(directory, name); r.files[p] {
			return lang.Target{Local: p}
		}
	}
	return lang.Target{Local: directory}
}

// dependencyTarget is a bazel_dep's module: a module of the bazel island at the version
// MODULE.bazel.lock selected, else the one declared - Bzlmod's minimal version
// selection is deterministic, and a registry never changes a published version,
// so a declared version pins, although another module may raise it - or what an
// override puts in its place.
//
// Implements: REQ-BAZEL-006, REQ-BAZEL-008
func (r *resolver) dependencyTarget(w *workspace, d *bazelDependency) lang.Target {
	t := lang.Target{Ecosystem: ecosystemBazel, Package: d.name, Version: d.version}
	o := w.over[d.name]
	switch {
	case o == nil:
		if selected := w.selected[d.name]; selected != "" {
			t.Version, t.Pinned = selected, true
			if d.version != "" && d.version != selected {
				t.Requested = d.version
			}
		} else if d.version != "" {
			t.Pinned = true
		} else {
			t.Floating = true
		}
	case o.kind == "local":
		if directory := r.localPath(w, o.path); directory != "" {
			return r.localRoot(directory)
		}
		return lang.Target{}
	case o.kind == "single":
		t.Version, t.Pinned = o.version, true
		if d.version != "" && d.version != o.version {
			t.Requested = d.version
		}
	case o.kind == "git":
		t.Origin = o.remote
		t = gitReference(t, o.commit, o.tag, o.branch)
		if t.Requested == "" && d.version != "" && d.version != t.Version {
			t.Requested = d.version
		}
	case o.kind == "archive":
		if u := pickURL(o.urls); u != "" {
			t.Origin = u
			if _, reference, _ := archiveName(u); reference != "" {
				t.Version = reference
			}
		}
		t.Pinned = o.integrity != ""
		t.Floating = !t.Pinned
		if d.version != "" && d.version != t.Version {
			t.Requested = d.version
		}
	}
	return t
}

// gitReference applies git's pin rule: a commit pins, a tag is neither (it can be
// moved), a branch or no reference floats.
func gitReference(t lang.Target, commit, tag, branch string) lang.Target {
	switch {
	case commit != "":
		t.Version, t.Pinned = commit, lang.Commit(commit)
		if tag != "" {
			t.Requested = tag
		}
	case tag != "":
		t.Version = tag
	case branch != "":
		t.Version, t.Floating = branch, true
	default:
		t.Version, t.Floating = "", true
	}
	return t
}

// repositoryTarget is what a WORKSPACE repository rule fetches: a download named by its
// URL (lang.RepositoryName; a GitHub or GitLab archive or release by its repository,
// with the reference as the version) pinned by its sha256 or integrity, as CMake's
// URL_HASH pins; a git repository by the git rule; a go_repository's module; a
// local_repository's directory.
//
// Implements: REQ-BAZEL-007
func (r *resolver) repositoryTarget(w *workspace, d *repositoryDeclaration) lang.Target {
	switch d.rule {
	case "local_repository", "new_local_repository":
		if directory := r.localPath(w, d.path); directory != "" {
			return r.localRoot(directory)
		}
		return lang.Target{}
	case "go_repository":
		packageName := d.importpath
		if packageName == "" {
			packageName = d.name
		}
		t := lang.Target{Ecosystem: ecosystemGo, Package: packageName}
		if d.version != "" {
			t.Version, t.Pinned = d.version, lang.Pinned(d.version)
			return t
		}
		if d.remote != "" {
			t.Origin = d.remote
		}
		return gitReference(t, d.commit, d.tag, "")
	case "git_repository", "new_git_repository":
		if d.remote == "" {
			return lang.Target{Ecosystem: ecosystemRepository, Package: d.name}
		}
		t := lang.Target{Ecosystem: ecosystemRepository, Package: lang.RepositoryName(d.remote)}
		return gitReference(t, d.commit, d.tag, d.branch)
	}
	u := pickURL(d.urls)
	if u == "" {
		// The URLs are computed (a dict of locations, a macro's argument): the
		// repository is known by its name only.
		return lang.Target{Ecosystem: ecosystemRepository, Package: d.name, Pinned: d.hash != ""}
	}
	name, reference, head := archiveName(u)
	t := lang.Target{Ecosystem: ecosystemRepository, Package: name, Version: reference}
	switch {
	case d.hash != "":
		t.Pinned = true
		if t.Version == "" {
			t.Version = d.hash
		}
	case lang.Commit(reference):
		t.Pinned = true
	case reference == "" || head:
		t.Floating = true
	}
	return t
}

// mirrors are download mirrors that put the original URL after their own prefix.
var mirrors = []string{"mirror.bazel.build/", "storage.googleapis.com/mirror.tensorflow.org/", "storage.googleapis.com/grpc-bazel-mirror/"}

// pickURL is the URL a repository is named by: the first that is not a mirror's,
// else the first.
func pickURL(urls []string) string {
	for _, u := range urls {
		if !strings.Contains(u, "{") && !isMirror(lang.RepositoryName(u)) {
			return u
		}
	}
	for _, u := range urls {
		if !strings.Contains(u, "{") {
			return u
		}
	}
	return ""
}

func isMirror(name string) bool {
	for _, m := range mirrors {
		if strings.HasPrefix(name, m) {
			return true
		}
	}
	return false
}

var (
	// Archives of a ref: GitHub, Codeberg, sourcehut (archive/<ref>.tar.gz, also
	// archive/refs/tags/<ref>), GitLab (/-/archive/<ref>/<name>-<ref>.tar.gz).
	archiveReference = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+?)(?:/-)?/archive/(refs/tags/|refs/heads/)?([^/]+?)(?:/[^/]+)?(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
	tarballReference = regexp.MustCompile(`^(github\.com/[^/]+/[^/]+)/(?:tarball|zipball)/(.+)$`)
	releaseReference = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+)/releases/download/([^/]+)/`)
	// A download named name-<version or commit>.tar.gz.
	namedArchive     = regexp.MustCompile(`^(.+)/([^/]+?)-(v?[0-9][0-9A-Za-z.+_-]*|[0-9a-f]{40})(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip|\.jar)$`)
	archiveExtension = regexp.MustCompile(`(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
)

// archiveName names a download by its URL, as the zig and cmake plugins do: a
// GitHub, GitLab, Codeberg or sourcehut archive or release by its repository,
// with the reference; name-1.2.3.tar.gz by its directory and name, with the version;
// anything else by the URL without the archive's extension. head says the reference is
// a branch.
func archiveName(u string) (name, reference string, head bool) {
	u, _, _ = strings.Cut(strings.TrimSpace(u), "#")
	u, _, _ = strings.Cut(u, "?")
	name = lang.RepositoryName(u)
	for _, m := range mirrors {
		name = strings.TrimPrefix(name, m)
	}
	if m := archiveReference.FindStringSubmatch(name); m != nil {
		return m[1], m[3], m[2] == "refs/heads/"
	}
	if m := tarballReference.FindStringSubmatch(name); m != nil {
		return m[1], m[2], false
	}
	if m := releaseReference.FindStringSubmatch(name); m != nil {
		return m[1], m[2], false
	}
	if m := namedArchive.FindStringSubmatch(name); m != nil {
		return m[1] + "/" + m[2], m[3], false
	}
	return archiveExtension.ReplaceAllString(name, ""), "", false
}

// hubLabel resolves a label of a hub repository to its ecosystem's package:
// @maven//:com_google_guava_guava, @pypi//requests, @npm//lodash (rules_nodejs),
// @crates//:serde.
//
// Implements: REQ-BAZEL-009
func (r *resolver) hubLabel(w *workspace, repository string, h *hub, l label) lang.Target {
	switch h.ecosystem {
	case ecosystemMaven:
		if t, ok := h.packages[normalizeMaven(l.target)]; ok {
			return t
		}
		return lang.Target{Ecosystem: ecosystemMaven, Package: l.target, Unresolved: true}
	case ecosystemPyPI:
		name := l.target
		if l.packageName != "" {
			name, _, _ = strings.Cut(l.packageName, "/")
		}
		return r.pypi(w, repository, name)
	case ecosystemNPM:
		name := l.packageName
		if name == "" {
			name = strings.TrimPrefix(l.target, "node_modules/")
		}
		return npmTarget(h, name)
	case ecosystemCrates:
		if t, ok := h.packages[normalizeCrate(l.target)]; ok {
			return t
		}
		return lang.Target{Ecosystem: ecosystemCrates, Package: l.target, Unresolved: h.lock}
	}
	return lang.Target{}
}

// pypi is a distribution of a pip hub, as its requirements lock spells it.
func (r *resolver) pypi(w *workspace, hubName, name string) lang.Target {
	if h := w.hubs[hubName]; h != nil {
		if t, ok := h.packages[normalizePy(name)]; ok {
			return t
		}
	}
	return lang.Target{Ecosystem: ecosystemPyPI, Package: strings.ReplaceAll(name, "_", "-"), Unresolved: true}
}

// npmTarget is an npm package a label names (@types/node keeps its scope).
func npmTarget(h *hub, name string) lang.Target {
	parts := strings.Split(name, "/")
	name = parts[0]
	if strings.HasPrefix(name, "@") && len(parts) > 1 {
		name += "/" + parts[1]
	}
	if name == "" {
		return lang.Target{}
	}
	if h != nil {
		if t, ok := h.packages[name]; ok {
			return t
		}
	}
	return lang.Target{Ecosystem: ecosystemNPM, Package: name}
}

// Expand turns a glob() into the files of the BUILD file's package it matches,
// not descending into subpackages (directories with a BUILD file of their own).
// A glob matching nothing stays one dropped import.
//
// Implements: REQ-BAZEL-005
func (r *resolver) Expand(file string, rawImport lang.RawImport) ([]lang.Import, bool) {
	if rawImport.Name != importGlob {
		return nil, false
	}
	includeRaw, excRaw, _ := strings.Cut(rawImport.Module, "\x01")
	include := strings.Split(includeRaw, "\x00")
	var exclude []string
	if excRaw != "" {
		exclude = strings.Split(excRaw, "\x00")
	}
	packageName := path.Dir(file)
	var out []lang.Import
	for _, f := range r.packageFiles[packageName] {
		full := path.Join(packageName, f)
		if full == file || !anyMatch(include, f) || anyMatch(exclude, f) {
			continue
		}
		out = append(out, lang.Import{Spec: f, Line: rawImport.Line, Target: lang.Target{Local: full}})
		if len(out) >= maxGlob {
			break
		}
	}
	if len(out) == 0 {
		return []lang.Import{{Spec: rawImport.Spec, Line: rawImport.Line}}, true
	}
	return out, true
}

// maxGlob bounds the files one glob() links: a glob(["**"]) at the top of a huge
// package says little more with its ten-thousandth file.
const maxGlob = 5000

func anyMatch(patterns []string, name string) bool {
	for _, p := range patterns {
		if globMatch(p, name) {
			return true
		}
	}
	return false
}

// globMatch matches a glob() pattern against a path relative to the package: *
// within one path segment, ** across any number of them.
func globMatch(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"), 0)
}

func matchSegments(p, n []string, depth int) bool {
	for len(p) > 0 {
		if p[0] == "**" {
			if len(p) == 1 {
				return true
			}
			if depth > 4 {
				return false // pathological: many ** in one pattern
			}
			for i := 0; i <= len(n); i++ {
				if matchSegments(p[1:], n[i:], depth+1) {
					return true
				}
			}
			return false
		}
		if len(n) == 0 {
			return false
		}
		if ok, err := path.Match(p[0], n[0]); err != nil || !ok {
			return false
		}
		p, n = p[1:], n[1:]
	}
	return len(n) == 0
}

// ---------------------------------------------------------------- lock

// readLock reads MODULE.bazel.lock beside a MODULE.bazel: the versions Bzlmod
// selected - the highest version of each module among the registry files it
// fetched (registryFileHashes, lock files since Bazel 7.2), or the resolved
// dependency graph older lock files keep (moduleDepGraph).
//
// Implements: REQ-BAZEL-008
func (r *resolver) readLock(w *workspace) {
	source, ok := r.read(path.Join(w.directory, "MODULE.bazel.lock"))
	if !ok {
		return
	}
	var doc struct {
		RegistryFileHashes    map[string]any `json:"registryFileHashes"`
		ModuleDependencyGraph map[string]struct {
			Name         string            `json:"name"`
			Version      string            `json:"version"`
			Dependencies map[string]string `json:"deps"`
		} `json:"moduleDepGraph"`
	}
	if json.Unmarshal(source, &doc) != nil {
		return
	}
	for u := range doc.RegistryFileHashes {
		i := strings.LastIndex(u, "/modules/")
		if i < 0 || !strings.HasSuffix(u, "/MODULE.bazel") {
			continue
		}
		parts := strings.Split(strings.TrimSuffix(u[i+len("/modules/"):], "/MODULE.bazel"), "/")
		if len(parts) != 2 {
			continue
		}
		if old := w.selected[parts[0]]; old == "" || lessVersion(old, parts[1]) {
			w.selected[parts[0]] = parts[1]
		}
	}
	for key, m := range doc.ModuleDependencyGraph {
		if key == "<root>" || m.Name == "" {
			continue
		}
		w.selected[m.Name] = m.Version
		for _, dependency := range sortedKeys(m.Dependencies) {
			if v := m.Dependencies[dependency]; !strings.HasSuffix(v, "@_") {
				w.graph[m.Name+"@"+m.Version] = append(w.graph[m.Name+"@"+m.Version], v)
			}
		}
	}
}

// lessVersion orders Bazel module versions: release segments compared numerically
// when both are numbers, else as text, a version without a pre-release after the
// same version with one.
func lessVersion(a, b string) bool {
	releaseA, prereleaseA, _ := strings.Cut(a, "-")
	releaseB, prereleaseB, _ := strings.Cut(b, "-")
	if c := compareSegments(releaseA, releaseB); c != 0 {
		return c < 0
	}
	switch {
	case prereleaseA == prereleaseB:
		return false
	case prereleaseA == "":
		return false
	case prereleaseB == "":
		return true
	}
	return compareSegments(prereleaseA, prereleaseB) < 0
}

func compareSegments(a, b string) int {
	aParts, bParts := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		x, y := aParts[i], bParts[i]
		nx, ny := isNumber(x), isNumber(y)
		switch {
		case nx && ny:
			x, y = strings.TrimLeft(x, "0"), strings.TrimLeft(y, "0")
			if len(x) != len(y) {
				return len(x) - len(y)
			}
			if x != y {
				return strings.Compare(x, y)
			}
		case nx != ny:
			if nx {
				return -1 // numbers before words
			}
			return 1
		case x != y:
			return strings.Compare(x, y)
		}
	}
	return len(aParts) - len(bParts)
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Dependencies implements lang.Transitive from the lock files: an older
// MODULE.bazel.lock's module graph, and rules_jvm_external's lock file for Maven
// artifacts.
//
// Implements: REQ-BAZEL-008, REQ-BAZEL-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	var out []lang.Target
	for _, directory := range sortedKeys(r.workspaces) {
		w := r.workspaces[directory]
		switch t.Ecosystem {
		case ecosystemBazel:
			for _, dependency := range w.graph[t.Package+"@"+t.Version] {
				name, version, _ := strings.Cut(dependency, "@")
				out = append(out, lang.Target{Ecosystem: ecosystemBazel, Package: name, Version: version, Pinned: version != ""})
			}
		case ecosystemMaven:
			for _, name := range sortedKeys(w.hubs) {
				h := w.hubs[name]
				for _, d := range h.dependencies[t.Package] {
					if dependencyTarget, ok := h.packages[normalizeMaven(d)]; ok {
						out = append(out, dependencyTarget)
					} else {
						out = append(out, lang.Target{Ecosystem: ecosystemMaven, Package: d})
					}
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return out
}
