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
	root       string
	files      map[string]bool
	dirs       map[string]bool
	build      map[string]string // package directory -> its BUILD file
	workspaces map[string]*workspace
	pkgFiles   map[string][]string // package directory -> its files, relative to it
}

var workspaceFiles = []string{"MODULE.bazel", "REPO.bazel", "WORKSPACE.bazel", "WORKSPACE", "WORKSPACE.bzlmod"}

// newResolver reads every workspace's MODULE.bazel (with its lock file and
// includes), WORKSPACE files and .bzl files, and indexes the packages.
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, dirs: map[string]bool{".": true},
		build: map[string]string{}, workspaces: map[string]*workspace{}, pkgFiles: map[string][]string{}}
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		dir, base := path.Dir(f.Path), path.Base(f.Path)
		switch base {
		case "BUILD.bazel":
			r.build[dir] = f.Path // preferred over BUILD, as Bazel does
		case "BUILD":
			if r.build[dir] == "" {
				r.build[dir] = f.Path
			}
		}
		for _, w := range workspaceFiles {
			if base == w && !skipped(f.Path) && r.workspaces[dir] == nil {
				r.workspaces[dir] = newWorkspace(dir)
			}
		}
	}
	if r.workspaces["."] == nil {
		r.workspaces["."] = newWorkspace(".")
	}
	for _, dir := range sortedKeys(r.workspaces) {
		w := r.workspaces[dir]
		if src, ok := r.read(path.Join(dir, "MODULE.bazel")); ok {
			r.readModule(w, path.Join(dir, "MODULE.bazel"), src, 0)
		}
		r.readLock(w)
		for _, name := range []string{"WORKSPACE.bazel", "WORKSPACE", "WORKSPACE.bzlmod"} {
			if src, ok := r.read(path.Join(dir, name)); ok {
				r.readRules(w, src, true)
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
		if src, ok := r.read(p); ok {
			r.readRules(r.workspaceOf(p), src, false)
		}
	}
	// Package contents for glob(): a file belongs to the nearest package above it;
	// a workspace root without a BUILD file ends the search.
	nearest := map[string]string{}
	var pkgOf func(d string) string
	pkgOf = func(d string) string {
		if p, ok := nearest[d]; ok {
			return p
		}
		p := ""
		switch {
		case r.build[d] != "":
			p = d
		case r.workspaces[d] != nil || d == ".":
		default:
			p = pkgOf(path.Dir(d))
		}
		nearest[d] = p
		return p
	}
	for _, f := range all {
		if p := pkgOf(path.Dir(f.Path)); p != "" {
			rel := f.Path
			if p != "." {
				rel = strings.TrimPrefix(f.Path, p+"/")
			}
			r.pkgFiles[p] = append(r.pkgFiles[p], rel)
		}
	}
	for _, list := range r.pkgFiles {
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
	src, err := os.ReadFile(full)
	return src, err == nil
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
		if r.build[d] != "" || d == w.dir || d == "." {
			break
		}
		d = path.Dir(d)
	}
	if r.build[d] == "" {
		d = path.Dir(file) // a .bzl outside any package: its own directory
	}
	return rel(w.dir, d)
}

func rel(base, p string) string {
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
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	w := r.workspaceOf(file)
	switch imp.Name {
	case impLoad, impLabel:
		return r.label(file, w, imp.Module)
	case impDep:
		if d := w.byModule[imp.Module]; d != nil {
			return r.depTarget(w, d)
		}
	case impRepo:
		if d := w.repos[imp.Module]; d != nil {
			return r.repoTarget(w, d)
		}
	case impMaven:
		coord, hubName, _ := strings.Cut(imp.Module, "\n")
		t, ok := mavenTarget(coord)
		if !ok {
			return lang.Target{}
		}
		if h := w.hubs[hubName]; h != nil {
			if locked, ok := h.pkgs[normMaven(t.Package)]; ok && h.locked[normMaven(t.Package)] {
				if t.Version != "" && t.Version != locked.Version {
					locked.Requested = t.Version
				} else {
					locked.Requested = ""
				}
				return locked
			}
		}
		return t
	case impPip:
		hubName, name, _ := strings.Cut(imp.Module, "\n")
		return r.pypi(w, hubName, name)
	case impGoMod:
		if t, ok := w.goMods[imp.Module]; ok {
			return t
		}
		return lang.Target{Ecosystem: ecoGo, Package: imp.Module}
	case impCrate:
		for _, name := range sortedKeys(w.hubs) {
			if h := w.hubs[name]; h.eco == ecoCrates {
				if t, ok := h.pkgs[normCrate(imp.Module)]; ok {
					return t
				}
			}
		}
		return lang.Target{Ecosystem: ecoCrates, Package: imp.Module}
	}
	return lang.Target{}
}

// label resolves a label a file names.
func (r *resolver) label(file string, w *workspace, s string) lang.Target {
	l, ok := parseLabel(s)
	if !ok {
		return lang.Target{}
	}
	pkg := l.pkg
	if !l.abs {
		pkg = r.packageOf(w, file)
	}
	if !l.hasRepo || l.repo == "" || !l.canonical && (l.repo == w.name || l.repo == w.repoName) ||
		l.canonical && moduleName(l.repo) == w.name {
		return r.local(file, w.dir, pkg, l.target)
	}
	return r.external(file, w, l)
}

// local resolves a label of a repository whose files are in the project at dir:
// the file it names, else the BUILD file of its package (a rule, or a file a rule
// generates), else a directory.
func (r *resolver) local(file, dir, pkg, target string) lang.Target {
	pkgDir := path.Join(dir, pkg)
	if name, ok := strings.CutPrefix(target, "node_modules/"); ok {
		// rules_js links a package as //<importer>:node_modules/<name>.
		w := r.workspaceOf(path.Join(dir, "x"))
		for _, hubName := range sortedKeys(w.hubs) {
			if h := w.hubs[hubName]; h.eco == ecoNPM {
				return npmTarget(h, name)
			}
		}
		return npmTarget(nil, name)
	}
	p := path.Join(pkgDir, target)
	switch {
	case p == file:
		return lang.Target{}
	case r.files[p]:
		return lang.Target{Local: p}
	case r.build[pkgDir] != "":
		if r.build[pkgDir] == file {
			return lang.Target{} // a target of the file's own package
		}
		return lang.Target{Local: r.build[pkgDir]}
	case r.dirs[p] && p != ".":
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

var hubNames = map[string]bool{"maven": true, "pypi": true, "pip": true, "npm": true, "crates": true, "crate_index": true}

// builtin reports whether a repository is one Bazel provides itself.
//
// Implements: REQ-BAZEL-010
func builtin(repo string) bool {
	switch {
	case repo == "bazel_tools", repo == "local_jdk", repo == "remote_coverage_tools", repo == "_builtins",
		strings.HasPrefix(repo, "local_config_"), strings.HasPrefix(repo, "remotejdk"),
		strings.HasPrefix(repo, "remote_java_tools"):
		return true
	}
	return false
}

// external resolves a label of another repository by what declares it. A
// repository nothing in the workspace declares is not guessed at.
//
// Implements: REQ-BAZEL-004, REQ-BAZEL-011
func (r *resolver) external(file string, w *workspace, l label) lang.Target {
	repo := l.repo
	if l.canonical {
		repo = moduleName(repo)
		if d := w.byModule[repo]; d != nil {
			return r.depLabel(file, w, d, l)
		}
	}
	if d := w.deps[repo]; d != nil {
		return r.depLabel(file, w, d, l)
	}
	if h := w.hubs[repo]; h != nil && !strings.HasSuffix(l.target, ".bzl") {
		return r.hubLabel(w, repo, h, l)
	}
	if t, ok := w.goRepos[repo]; ok {
		return t
	}
	if d := w.repos[repo]; d != nil {
		if d.rule == "local_repository" || d.rule == "new_local_repository" {
			if dir := r.localPath(w, d.path); dir != "" {
				if t := r.local(file, dir, l.pkg, l.target); t != (lang.Target{}) {
					return t
				}
				return r.localRoot(dir)
			}
			return lang.Target{}
		}
		return r.repoTarget(w, d)
	}
	for _, name := range sortedKeys(w.hubs) { // pip's older per-package repositories: @pypi_requests//:pkg
		if w.hubs[name].eco == ecoPyPI && strings.HasPrefix(repo, name+"_") {
			return r.pypi(w, name, strings.TrimPrefix(repo, name+"_"))
		}
	}
	if d := w.byModule[repo]; d != nil {
		// A module's own name where its repo_name was declared: a vendored copy of
		// another project (grpc's third_party/upb says @abseil-cpp).
		return r.depLabel(file, w, d, l)
	}
	if ext, ok := w.extRepos[repo]; ok {
		// A repository a module extension made: it comes with the module whose .bzl
		// defines the extension, or from the project's own extension.
		return r.label(w.extFile[repo], w, ext)
	}
	if builtin(repo) {
		return lang.Target{Ecosystem: ecoStd, Package: repo}
	}
	if w.hubs[repo] != nil || strings.HasSuffix(l.target, ".bzl") && hubNames[repo] {
		return lang.Target{} // a hub's generated .bzl (requirements.bzl, defs.bzl)
	}
	switch repo { // hub repositories by their conventional names
	case "maven":
		return r.hubLabel(w, repo, &hub{eco: ecoMaven}, l)
	case "pypi", "pip":
		return r.hubLabel(w, repo, &hub{eco: ecoPyPI}, l)
	case "npm":
		return r.hubLabel(w, repo, &hub{eco: ecoNPM}, l)
	case "crates", "crate_index":
		return r.hubLabel(w, repo, &hub{eco: ecoCrates}, l)
	}
	return lang.Target{Ecosystem: ecoBazel, Package: repo, Unresolved: true}
}

// depLabel resolves a label of a bazel_dep's repository: the module, or a file of
// the project when a local_path_override puts it there.
func (r *resolver) depLabel(file string, w *workspace, d *bazelDep, l label) lang.Target {
	if o := w.over[d.name]; o != nil && o.kind == "local" {
		if dir := r.localPath(w, o.path); dir != "" {
			if t := r.local(file, dir, l.pkg, l.target); t != (lang.Target{}) {
				return t
			}
		}
	}
	return r.depTarget(w, d)
}

// localPath is a local_repository's or local_path_override's directory in the
// project ("" outside it).
func (r *resolver) localPath(w *workspace, p string) string {
	if p == "" || path.IsAbs(p) || strings.HasPrefix(p, "~") {
		return ""
	}
	d := path.Join(w.dir, p)
	if d == ".." || strings.HasPrefix(d, "../") || !r.dirs[d] {
		return ""
	}
	return d
}

// localRoot is what stands for a local repository as a whole: its MODULE.bazel or
// WORKSPACE file, else its top BUILD file, else the directory.
func (r *resolver) localRoot(dir string) lang.Target {
	for _, name := range append(append([]string{}, workspaceFiles...), "BUILD.bazel", "BUILD") {
		if p := path.Join(dir, name); r.files[p] {
			return lang.Target{Local: p}
		}
	}
	return lang.Target{Local: dir}
}

// depTarget is a bazel_dep's module: a module of the bazel island at the version
// MODULE.bazel.lock selected, else the one declared - Bzlmod's minimal version
// selection is deterministic, and a registry never changes a published version,
// so a declared version pins, although another module may raise it - or what an
// override puts in its place.
//
// Implements: REQ-BAZEL-006, REQ-BAZEL-008
func (r *resolver) depTarget(w *workspace, d *bazelDep) lang.Target {
	t := lang.Target{Ecosystem: ecoBazel, Package: d.name, Version: d.version}
	o := w.over[d.name]
	switch {
	case o == nil:
		if sel := w.selected[d.name]; sel != "" {
			t.Version, t.Pinned = sel, true
			if d.version != "" && d.version != sel {
				t.Requested = d.version
			}
		} else if d.version != "" {
			t.Pinned = true
		} else {
			t.Floating = true
		}
	case o.kind == "local":
		if dir := r.localPath(w, o.path); dir != "" {
			return r.localRoot(dir)
		}
		return lang.Target{}
	case o.kind == "single":
		t.Version, t.Pinned = o.version, true
		if d.version != "" && d.version != o.version {
			t.Requested = d.version
		}
	case o.kind == "git":
		t.Origin = o.remote
		t = gitRef(t, o.commit, o.tag, o.branch)
		if t.Requested == "" && d.version != "" && d.version != t.Version {
			t.Requested = d.version
		}
	case o.kind == "archive":
		if u := pickURL(o.urls); u != "" {
			t.Origin = u
			if _, ref, _ := archiveName(u); ref != "" {
				t.Version = ref
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

// gitRef applies git's pin rule: a commit pins, a tag is neither (it can be
// moved), a branch or no ref floats.
func gitRef(t lang.Target, commit, tag, branch string) lang.Target {
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

// repoTarget is what a WORKSPACE repository rule fetches: a download named by its
// URL (lang.RepoName; a GitHub or GitLab archive or release by its repository,
// with the ref as the version) pinned by its sha256 or integrity, as CMake's
// URL_HASH pins; a git repository by the git rule; a go_repository's module; a
// local_repository's directory.
//
// Implements: REQ-BAZEL-007
func (r *resolver) repoTarget(w *workspace, d *repoDecl) lang.Target {
	switch d.rule {
	case "local_repository", "new_local_repository":
		if dir := r.localPath(w, d.path); dir != "" {
			return r.localRoot(dir)
		}
		return lang.Target{}
	case "go_repository":
		pkg := d.importpath
		if pkg == "" {
			pkg = d.name
		}
		t := lang.Target{Ecosystem: ecoGo, Package: pkg}
		if d.version != "" {
			t.Version, t.Pinned = d.version, lang.Pinned(d.version)
			return t
		}
		if d.remote != "" {
			t.Origin = d.remote
		}
		return gitRef(t, d.commit, d.tag, "")
	case "git_repository", "new_git_repository":
		if d.remote == "" {
			return lang.Target{Ecosystem: ecoRepo, Package: d.name}
		}
		t := lang.Target{Ecosystem: ecoRepo, Package: lang.RepoName(d.remote)}
		return gitRef(t, d.commit, d.tag, d.branch)
	}
	u := pickURL(d.urls)
	if u == "" {
		// The URLs are computed (a dict of locations, a macro's argument): the
		// repository is known by its name only.
		return lang.Target{Ecosystem: ecoRepo, Package: d.name, Pinned: d.hash != ""}
	}
	name, ref, head := archiveName(u)
	t := lang.Target{Ecosystem: ecoRepo, Package: name, Version: ref}
	switch {
	case d.hash != "":
		t.Pinned = true
		if t.Version == "" {
			t.Version = d.hash
		}
	case lang.Commit(ref):
		t.Pinned = true
	case ref == "" || head:
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
		if !strings.Contains(u, "{") && !isMirror(lang.RepoName(u)) {
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
	archiveRef = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+?)(?:/-)?/archive/(refs/tags/|refs/heads/)?([^/]+?)(?:/[^/]+)?(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
	tarballRef = regexp.MustCompile(`^(github\.com/[^/]+/[^/]+)/(?:tarball|zipball)/(.+)$`)
	releaseRef = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+)/releases/download/([^/]+)/`)
	// A download named name-<version or commit>.tar.gz.
	namedArchive = regexp.MustCompile(`^(.+)/([^/]+?)-(v?[0-9][0-9A-Za-z.+_-]*|[0-9a-f]{40})(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip|\.jar)$`)
	archiveExt   = regexp.MustCompile(`(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
)

// archiveName names a download by its URL, as the zig and cmake plugins do: a
// GitHub, GitLab, Codeberg or sourcehut archive or release by its repository,
// with the ref; name-1.2.3.tar.gz by its directory and name, with the version;
// anything else by the URL without the archive's extension. head says the ref is
// a branch.
func archiveName(u string) (name, ref string, head bool) {
	u, _, _ = strings.Cut(strings.TrimSpace(u), "#")
	u, _, _ = strings.Cut(u, "?")
	name = lang.RepoName(u)
	for _, m := range mirrors {
		name = strings.TrimPrefix(name, m)
	}
	if m := archiveRef.FindStringSubmatch(name); m != nil {
		return m[1], m[3], m[2] == "refs/heads/"
	}
	if m := tarballRef.FindStringSubmatch(name); m != nil {
		return m[1], m[2], false
	}
	if m := releaseRef.FindStringSubmatch(name); m != nil {
		return m[1], m[2], false
	}
	if m := namedArchive.FindStringSubmatch(name); m != nil {
		return m[1] + "/" + m[2], m[3], false
	}
	return archiveExt.ReplaceAllString(name, ""), "", false
}

// hubLabel resolves a label of a hub repository to its ecosystem's package:
// @maven//:com_google_guava_guava, @pypi//requests, @npm//lodash (rules_nodejs),
// @crates//:serde.
//
// Implements: REQ-BAZEL-009
func (r *resolver) hubLabel(w *workspace, repo string, h *hub, l label) lang.Target {
	switch h.eco {
	case ecoMaven:
		if t, ok := h.pkgs[normMaven(l.target)]; ok {
			return t
		}
		return lang.Target{Ecosystem: ecoMaven, Package: l.target, Unresolved: true}
	case ecoPyPI:
		name := l.target
		if l.pkg != "" {
			name, _, _ = strings.Cut(l.pkg, "/")
		}
		return r.pypi(w, repo, name)
	case ecoNPM:
		name := l.pkg
		if name == "" {
			name = strings.TrimPrefix(l.target, "node_modules/")
		}
		return npmTarget(h, name)
	case ecoCrates:
		if t, ok := h.pkgs[normCrate(l.target)]; ok {
			return t
		}
		return lang.Target{Ecosystem: ecoCrates, Package: l.target, Unresolved: h.lock}
	}
	return lang.Target{}
}

// pypi is a distribution of a pip hub, as its requirements lock spells it.
func (r *resolver) pypi(w *workspace, hubName, name string) lang.Target {
	if h := w.hubs[hubName]; h != nil {
		if t, ok := h.pkgs[normPy(name)]; ok {
			return t
		}
	}
	return lang.Target{Ecosystem: ecoPyPI, Package: strings.ReplaceAll(name, "_", "-"), Unresolved: true}
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
		if t, ok := h.pkgs[name]; ok {
			return t
		}
	}
	return lang.Target{Ecosystem: ecoNPM, Package: name}
}

// Expand turns a glob() into the files of the BUILD file's package it matches,
// not descending into subpackages (directories with a BUILD file of their own).
// A glob matching nothing stays one dropped import.
//
// Implements: REQ-BAZEL-005
func (r *resolver) Expand(file string, imp lang.RawImport) ([]lang.Import, bool) {
	if imp.Name != impGlob {
		return nil, false
	}
	incRaw, excRaw, _ := strings.Cut(imp.Module, "\x01")
	include := strings.Split(incRaw, "\x00")
	var exclude []string
	if excRaw != "" {
		exclude = strings.Split(excRaw, "\x00")
	}
	pkg := path.Dir(file)
	var out []lang.Import
	for _, f := range r.pkgFiles[pkg] {
		full := path.Join(pkg, f)
		if full == file || !anyMatch(include, f) || anyMatch(exclude, f) {
			continue
		}
		out = append(out, lang.Import{Spec: f, Line: imp.Line, Target: lang.Target{Local: full}})
		if len(out) >= maxGlob {
			break
		}
	}
	if len(out) == 0 {
		return []lang.Import{{Spec: imp.Spec, Line: imp.Line}}, true
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
	return matchSegs(strings.Split(pattern, "/"), strings.Split(name, "/"), 0)
}

func matchSegs(p, n []string, depth int) bool {
	for len(p) > 0 {
		if p[0] == "**" {
			if len(p) == 1 {
				return true
			}
			if depth > 4 {
				return false // pathological: many ** in one pattern
			}
			for i := 0; i <= len(n); i++ {
				if matchSegs(p[1:], n[i:], depth+1) {
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
	src, ok := r.read(path.Join(w.dir, "MODULE.bazel.lock"))
	if !ok {
		return
	}
	var doc struct {
		RegistryFileHashes map[string]any `json:"registryFileHashes"`
		ModuleDepGraph     map[string]struct {
			Name    string            `json:"name"`
			Version string            `json:"version"`
			Deps    map[string]string `json:"deps"`
		} `json:"moduleDepGraph"`
	}
	if json.Unmarshal(src, &doc) != nil {
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
	for key, m := range doc.ModuleDepGraph {
		if key == "<root>" || m.Name == "" {
			continue
		}
		w.selected[m.Name] = m.Version
		for _, dep := range sortedKeys(m.Deps) {
			if v := m.Deps[dep]; !strings.HasSuffix(v, "@_") {
				w.graph[m.Name+"@"+m.Version] = append(w.graph[m.Name+"@"+m.Version], v)
			}
		}
	}
}

// lessVersion orders Bazel module versions: release segments compared numerically
// when both are numbers, else as text, a version without a pre-release after the
// same version with one.
func lessVersion(a, b string) bool {
	relA, preA, _ := strings.Cut(a, "-")
	relB, preB, _ := strings.Cut(b, "-")
	if c := compareSegs(relA, relB); c != 0 {
		return c < 0
	}
	switch {
	case preA == preB:
		return false
	case preA == "":
		return false
	case preB == "":
		return true
	}
	return compareSegs(preA, preB) < 0
}

func compareSegs(a, b string) int {
	sa, sb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(sa) && i < len(sb); i++ {
		x, y := sa[i], sb[i]
		nx, ny := isNum(x), isNum(y)
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
	return len(sa) - len(sb)
}

func isNum(s string) bool {
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
	for _, dir := range sortedKeys(r.workspaces) {
		w := r.workspaces[dir]
		switch t.Ecosystem {
		case ecoBazel:
			for _, dep := range w.graph[t.Package+"@"+t.Version] {
				name, version, _ := strings.Cut(dep, "@")
				out = append(out, lang.Target{Ecosystem: ecoBazel, Package: name, Version: version, Pinned: version != ""})
			}
		case ecoMaven:
			for _, name := range sortedKeys(w.hubs) {
				h := w.hubs[name]
				for _, d := range h.deps[t.Package] {
					if dt, ok := h.pkgs[normMaven(d)]; ok {
						out = append(out, dt)
					} else {
						out = append(out, lang.Target{Ecosystem: ecoMaven, Package: d})
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
