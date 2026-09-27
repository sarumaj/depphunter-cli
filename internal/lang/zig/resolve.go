package zig

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// pkg is a Zig package of the repository: a directory with build.zig or
// build.zig.zon.
type pkg struct {
	dir   string
	zon   *zonFile
	facts *buildFacts
}

func (p *pkg) dep(name string) *zonDep {
	if p == nil || p.zon == nil {
		return nil
	}
	for i := range p.zon.deps {
		if p.zon.deps[i].key == name {
			return &p.zon.deps[i]
		}
	}
	fold := strings.ReplaceAll(name, "-", "_")
	for i := range p.zon.deps {
		if strings.ReplaceAll(p.zon.deps[i].key, "-", "_") == fold {
			return &p.zon.deps[i]
		}
	}
	return nil
}

type resolver struct {
	root   string
	files  map[string]bool
	dirs   map[string]bool
	pkgs   map[string]*pkg
	rootOf map[string]string // file -> root source file of the compilation reaching it

	incOnce sync.Once
	all     []*scan.File
	inc     cpp.Includes

	mu     sync.Mutex
	hashes map[string][]fetched     // package target key -> where it was fetched
	deps   map[string][]lang.Target // memo of Dependencies
	found  map[string]bool
}

// fetched is a URL dependency with its hash, and the package that declared it.
type fetched struct {
	hash, dir string
}

// buildMarkers are words only build code uses; files without any are not
// evaluated for the build graph.
var buildMarkers = [][]byte{
	[]byte("addImport"), []byte("createModule"), []byte("addModule"), []byte("root_source_file"),
	[]byte("dependency("), []byte("Dependency("), []byte("addExecutable"), []byte("addTest"),
	[]byte("addLibrary"), []byte("addOptions"), []byte("source_file"),
}

func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, dirs: map[string]bool{}, pkgs: map[string]*pkg{},
		rootOf: map[string]string{}, all: all, hashes: map[string][]fetched{}, deps: map[string][]lang.Target{}, found: map[string]bool{}}
	var sources []*scan.File
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); d != "." && !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
		}
		base := path.Base(f.Path)
		if skipped(f.Path) {
			continue
		}
		if base == "build.zig" || base == "build.zig.zon" {
			d := path.Dir(f.Path)
			if r.pkgs[d] == nil {
				r.pkgs[d] = &pkg{dir: d, facts: newFacts()}
			}
			if base == "build.zig.zon" && readable(f) {
				if src, err := os.ReadFile(f.Abs); err == nil {
					r.pkgs[d].zon = readZon(src)
				}
			}
		}
		if path.Ext(f.Path) == ".zig" && readable(f) {
			sources = append(sources, f)
		}
	}
	// build.zig first in each package, so what it wires wins over helper files.
	sort.Slice(sources, func(i, j int) bool {
		bi, bj := path.Base(sources[i].Path) == "build.zig", path.Base(sources[j].Path) == "build.zig"
		if bi != bj {
			return bi
		}
		return sources[i].Path < sources[j].Path
	})
	imports := map[string][]string{}
	for _, f := range sources {
		src, err := os.ReadFile(f.Abs)
		if err != nil || !lang.Parseable(f, src) {
			continue
		}
		if p := r.pkgOf(f.Path); p != nil && hasAny(src, buildMarkers) {
			evalBuild(src, p.dir, p.facts)
		}
		imports[f.Path] = importedNames(src)
	}
	for _, dir := range sortedKeys(r.pkgs) {
		p := r.pkgs[dir]
		if p.zon == nil {
			continue
		}
		for _, d := range p.zon.deps {
			if d.url != "" && d.hash != "" {
				t := r.zonTarget(p.dir, d)
				r.hashes[key(t)] = append(r.hashes[key(t)], fetched{d.hash, p.dir})
			}
		}
	}
	r.roots(imports)
	return r
}

// importedNames lists what a file @imports, for following compilations from their
// roots: a byte scan, cheaper than lexing every file again (an @import in a comment
// only adds a harmless edge to the walk).
func importedNames(src []byte) []string {
	var out []string
	for {
		i := bytes.Index(src, []byte(`@import("`))
		if i < 0 {
			return out
		}
		src = src[i+9:]
		if j := bytes.IndexAny(src, "\"\n"); j > 0 && src[j] == '"' {
			out = append(out, string(src[:j]))
		}
	}
}

func readable(f *scan.File) bool { return !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize }

func hasAny(src []byte, words [][]byte) bool {
	for _, w := range words {
		if bytes.Contains(src, w) {
			return true
		}
	}
	return false
}

func key(t lang.Target) string { return t.Ecosystem + "\x00" + t.Package + "\x00" + t.Version }

// pkgOf is the package a file belongs to: the nearest directory above it with
// build.zig or build.zig.zon.
func (r *resolver) pkgOf(file string) *pkg {
	for d := path.Dir(file); ; d = path.Dir(d) {
		if p := r.pkgs[d]; p != nil {
			return p
		}
		if d == "." || d == "/" {
			return nil
		}
	}
}

// roots finds, for each file, the root source file of the compilation that reaches
// it through relative and module imports: the one reaching it, else the one
// non-test root among several, roots of the file's own package first. Build code
// does not belong to one.
//
// Implements: REQ-ZIG-004
func (r *resolver) roots(imports map[string][]string) {
	reached := map[string][]croot{}
	for _, dir := range sortedKeys(r.pkgs) {
		p := r.pkgs[dir]
		seenRoot := map[string]int{}
		var roots []croot
		for _, c := range p.facts.roots {
			if i, ok := seenRoot[c.file]; ok {
				roots[i].test = roots[i].test && c.test
				continue
			}
			if !r.files[c.file] {
				continue
			}
			seenRoot[c.file] = len(roots)
			roots = append(roots, c)
		}
		if len(roots) > 200 {
			roots = roots[:200]
		}
		for _, c := range roots {
			seen := map[string]bool{c.file: true}
			queue := []string{c.file}
			for len(queue) > 0 {
				f := queue[0]
				queue = queue[1:]
				reached[f] = append(reached[f], c)
				for _, im := range imports[f] {
					var next string
					if strings.HasSuffix(im, ".zig") {
						next = path.Clean(path.Join(path.Dir(f), im))
					} else if t := r.module(f, im); strings.HasSuffix(t.Local, ".zig") {
						next = t.Local
					}
					if next != "" && !seen[next] && r.files[next] {
						seen[next] = true
						queue = append(queue, next)
					}
				}
			}
		}
	}
	for f, rs := range reached {
		// The roots of the file's own package, when it has any: an example
		// depending on the package by path reaches its files too.
		if p := r.pkgOf(f); p != nil {
			var own []croot
			for _, c := range rs {
				if q := r.pkgOf(c.file); q == p {
					own = append(own, c)
				}
			}
			if len(own) > 0 {
				rs = own
			}
		}
		if len(rs) == 1 {
			r.rootOf[f] = rs[0].file
			continue
		}
		var main []string
		for _, c := range rs {
			if !c.test {
				main = append(main, c.file)
			}
		}
		if len(main) == 1 {
			r.rootOf[f] = main[0]
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Resolve maps an import to its target.
//
// Implements: REQ-ZIG-004, REQ-ZIG-005, REQ-ZIG-006, REQ-ZIG-008, REQ-ZIG-010
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	m := imp.Module
	switch imp.Name {
	case kindImport:
		switch {
		case m == "std" || m == "builtin":
			return lang.Target{Ecosystem: ecoStd, Package: m}
		case m == "root":
			if f := r.rootOf[file]; f != "" && f != file {
				return lang.Target{Local: f}
			}
			return lang.Target{}
		case strings.HasSuffix(m, ".zig") || strings.HasSuffix(m, ".zon"):
			return r.local(path.Join(path.Dir(file), m), false)
		}
		return r.module(file, m)
	case kindEmbed:
		return r.local(path.Join(path.Dir(file), m), false)
	case kindCInclude:
		r.incOnce.Do(func() { r.inc = cpp.NewIncludes(r.root, r.all) })
		return r.inc.Resolve(file, lang.RawImport{Spec: imp.Spec, Module: m, Line: imp.Line}, nil)
	case kindBuild:
		if p := r.pkgOf(file); p != nil {
			return r.local(path.Join(p.dir, m), true)
		}
	case kindDep:
		p := r.pkgOf(file)
		if d := p.dep(m); d != nil {
			return r.zonTarget(p.dir, *d)
		}
	case kindZon:
		url, rest, _ := strings.Cut(m, "\n")
		hash, dir, _ := strings.Cut(rest, "\n")
		return r.zonTarget(path.Dir(file), zonDep{key: imp.Spec, url: url, hash: hash, path: dir})
	case kindZigVer:
		return lang.Target{Ecosystem: ecoStd, Package: "zig", Version: ">= " + m}
	}
	return lang.Target{}
}

// local is a project file, or with dirs also a directory holding files; anything
// else (generated, outside the repository) is dropped.
func (r *resolver) local(p string, dirs bool) lang.Target {
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return lang.Target{}
	}
	if r.files[p] || dirs && r.dirs[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// pkgDir is a package directory of the repository as a target: the directory,
// or the root's build.zig.zon (the root is no directory of the map).
func (r *resolver) pkgDir(dir string) lang.Target {
	if dir = path.Clean(dir); dir == "." {
		for _, f := range []string{"build.zig.zon", "build.zig"} {
			if r.files[f] {
				return lang.Target{Local: f}
			}
		}
	}
	return r.local(dir, true)
}

// module resolves @import("name") of a module: what the package's build code wires
// the name to, else a module some build code of the package exports under the name,
// else the package's build.zig.zon dependency of that name (in build.zig itself that
// is the dependency's build.zig), else a module another package of the repository
// exports. A name wired to a generated module (build options), or one the build
// code wires in a way it does not follow, is dropped.
//
// Implements: REQ-ZIG-006, REQ-ZIG-011
func (r *resolver) module(file, name string) lang.Target {
	p := r.pkgOf(file)
	build := path.Base(file) == "build.zig"
	if p != nil && !build {
		for _, v := range p.facts.wires[name] {
			if t, ok := r.value(p, v); ok {
				return t
			}
		}
		if f := p.facts.exports[name]; f != "" && r.files[f] {
			return lang.Target{Local: f}
		}
	}
	if d := p.dep(name); d != nil {
		if d.path != "" {
			q := r.pkgs[path.Clean(path.Join(p.dir, d.path))]
			if build && q != nil && r.files[path.Join(q.dir, "build.zig")] {
				return lang.Target{Local: path.Join(q.dir, "build.zig")}
			}
			if !build && q != nil {
				if f := q.facts.exports[name]; f != "" && r.files[f] {
					return lang.Target{Local: f}
				}
			}
		}
		return r.zonTarget(p.dir, *d)
	}
	if build {
		return lang.Target{}
	}
	var hit string
	for _, dir := range sortedKeys(r.pkgs) {
		if f := r.pkgs[dir].facts.exports[name]; f != "" && r.files[f] {
			if hit != "" && hit != f {
				return lang.Target{} // exported by several packages: ambiguous
			}
			hit = f
		}
	}
	if hit != "" {
		return lang.Target{Local: hit}
	}
	return lang.Target{}
}

// value is the target of a wired value; ok is false when the value is unknown, so
// the next candidate is tried. A generated module is ok with no target.
func (r *resolver) value(p *pkg, v val) (lang.Target, bool) {
	switch v.kind {
	case 'f', 'm', 'c':
		if v.path == "" {
			return lang.Target{}, true // generated
		}
		return r.local(v.path, false), true
	case 'g':
		return lang.Target{}, true
	case 'd':
		d := p.dep(v.dep)
		if d == nil {
			return lang.Target{}, false
		}
		if d.path != "" {
			dir := path.Clean(path.Join(p.dir, d.path))
			if q := r.pkgs[dir]; q != nil && v.mod != "" {
				if f := q.facts.exports[v.mod]; f != "" && r.files[f] {
					return lang.Target{Local: f}, true
				}
			}
		}
		return r.zonTarget(p.dir, *d), true
	}
	return lang.Target{}, false
}

var (
	// Archives of a ref: GitHub, Codeberg, sourcehut (archive/<ref>.tar.gz, also
	// archive/refs/tags/<ref>), GitLab (/-/archive/<ref>/<name>-<ref>.tar.gz), and
	// GitHub's tarball/<ref>.
	archiveRef = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+?)(?:/-)?/archive/(refs/tags/|refs/heads/)?([^/]+?)(?:/[^/]+)?(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
	tarballRef = regexp.MustCompile(`^(github\.com/[^/]+/[^/]+)/(?:tarball|zipball)/(.+)$`)
	releaseRef = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+)/releases/download/([^/]+)/`)
	// A download named name-<version or commit>.tar.gz.
	namedArchive = regexp.MustCompile(`^(.+)/([^/]+?)-(v?[0-9][0-9A-Za-z.+_-]*|[0-9a-f]{40})(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
	archiveExt   = regexp.MustCompile(`(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
)

// zonTarget is a build.zig.zon dependency declared in dir: a directory of the
// repository for .path, else a package of the zig island named by its URL
// (lang.RepoName): a GitHub, GitLab, Codeberg or sourcehut archive or a git+https
// URL by its repository, other downloads by their URL without the archive's
// version. A .hash pins it (Zig verifies the content against it, as CMake does a
// URL_HASH); without one a commit pins, a tag is neither, a branch or no ref floats.
// The version is the URL's ref, else the version in the hash (name-1.2.3-...),
// else the hash.
//
// Implements: REQ-ZIG-007, REQ-ZIG-008
func (r *resolver) zonTarget(dir string, d zonDep) lang.Target {
	if d.url == "" {
		if d.path != "" {
			return r.pkgDir(path.Join(dir, d.path))
		}
		return lang.Target{Ecosystem: ecoZig, Package: d.key, Unresolved: true, Floating: true}
	}
	t := lang.Target{Ecosystem: ecoZig}
	ref, head := "", false
	u := strings.TrimSpace(d.url)
	if git, ok := strings.CutPrefix(u, "git+"); ok {
		var q string
		u, ref, _ = strings.Cut(git, "#")
		u, q, _ = strings.Cut(u, "?")
		t.Package = lang.RepoName(u)
		if req := strings.TrimPrefix(q, "ref="); req != q && req != ref {
			t.Requested = req
		}
	} else {
		u, _, _ = strings.Cut(u, "#")
		u, _, _ = strings.Cut(u, "?")
		t.Package = lang.RepoName(u)
		switch m := archiveRef.FindStringSubmatch(t.Package); {
		case m != nil:
			t.Package, ref, head = m[1], m[3], m[2] == "refs/heads/"
		default:
			if m := tarballRef.FindStringSubmatch(t.Package); m != nil {
				t.Package, ref = m[1], m[2]
			} else if m := releaseRef.FindStringSubmatch(t.Package); m != nil {
				t.Package, ref = m[1], m[2]
			} else if m := namedArchive.FindStringSubmatch(t.Package); m != nil && !hashLike(m[2]+"-"+m[3]) {
				t.Package, ref = m[1]+"/"+m[2], m[3]
			} else if base := archiveExt.ReplaceAllString(path.Base(t.Package), ""); hashLike(base) || base == d.hash {
				t.Package = path.Dir(t.Package) + "/" + d.key // named by its hash: use the dependency's name
			} else {
				t.Package = archiveExt.ReplaceAllString(t.Package, "")
			}
		}
	}
	t.Version = ref
	switch {
	case d.hash != "":
		t.Pinned = true
		if t.Version == "" {
			t.Version = hashVersion(d.hash)
		}
	case lang.Commit(ref):
		t.Pinned = true
	case ref == "" || head:
		t.Floating = true
	}
	return t
}

// hashLike reports whether an archive's name is a package hash: the legacy
// multihash 1220<64 hex> or the N-V-... form of a package without name and version.
func hashLike(s string) bool {
	return strings.HasPrefix(s, "N-V-") || len(s) == 68 && strings.HasPrefix(s, "1220") && strings.Trim(s, "0123456789abcdef") == ""
}

// hashVersion is the version a package hash carries: name-1.2.3-<44 characters>
// since Zig 0.14; the whole hash otherwise.
func hashVersion(h string) string {
	name, rest, ok := strings.Cut(h, "-")
	if ok && name != "N" && len(rest) > 45 && rest[len(rest)-45] == '-' {
		return rest[:len(rest)-45]
	}
	return h
}

// Dependencies implements lang.Transitive from fetched packages on disk: the
// build.zig.zon of a package Zig fetched into the project's zig-pkg/<hash> (Zig
// 0.16) or the global cache's p/<hash> ($ZIG_GLOBAL_CACHE_DIR, else
// $XDG_CACHE_HOME/zig, %LOCALAPPDATA%\zig on Windows, or ~/.cache/zig).
//
// Implements: REQ-ZIG-009
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoZig {
		return nil
	}
	k := key(t)
	r.mu.Lock()
	defer r.mu.Unlock()
	if out, ok := r.deps[k]; ok {
		return out
	}
	var out []lang.Target
	for _, f := range r.hashes[k] {
		src := r.fetchedZon(f)
		if src == nil {
			continue
		}
		r.found[k] = true
		for _, d := range readZon(src).deps {
			if d.url != "" { // a path inside the fetched package is not the repository's
				out = append(out, r.zonTarget(".", d))
			}
		}
		break
	}
	r.deps[k] = out
	return out
}

// Installed reports that a package's dependencies came from its fetched copy.
func (r *resolver) Installed(t lang.Target) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.found[key(t)]
}

func (r *resolver) fetchedZon(f fetched) []byte {
	if strings.ContainsAny(f.hash, `/\`) || f.hash == "" || f.hash == "." || f.hash == ".." {
		return nil
	}
	var cands []string
	for d := f.dir; ; d = path.Dir(d) {
		cands = append(cands, filepath.Join(r.root, filepath.FromSlash(d), "zig-pkg", f.hash))
		if d == "." || d == "/" {
			break
		}
	}
	if c := os.Getenv("ZIG_GLOBAL_CACHE_DIR"); c != "" {
		cands = append(cands, filepath.Join(c, "p", f.hash))
	}
	for _, env := range []string{"XDG_CACHE_HOME", "LOCALAPPDATA"} { // LOCALAPPDATA: Windows
		if c := os.Getenv(env); c != "" {
			cands = append(cands, filepath.Join(c, "zig", "p", f.hash))
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		cands = append(cands, filepath.Join(h, ".cache", "zig", "p", f.hash))
	}
	for _, c := range cands {
		if src, err := os.ReadFile(filepath.Join(c, "build.zig.zon")); err == nil && len(src) <= lang.MaxParseSize {
			return src
		}
	}
	return nil
}

// skipped reports whether a path lies in Zig's build output or package caches.
func skipped(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if ignoredDirs[seg] {
			return true
		}
	}
	return false
}
