package zig

import (
	"bytes"
	"cmp"
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

// zigPackage is a Zig package of the repository: a directory with build.zig or
// build.zig.zon.
type zigPackage struct {
	directory string
	zon       *zonFile
	facts     *buildFacts
}

func (p *zigPackage) dependency(name string) *zonDependency {
	if p == nil || p.zon == nil {
		return nil
	}
	for i := range p.zon.dependencies {
		if p.zon.dependencies[i].key == name {
			return &p.zon.dependencies[i]
		}
	}
	fold := strings.ReplaceAll(name, "-", "_")
	for i := range p.zon.dependencies {
		if strings.ReplaceAll(p.zon.dependencies[i].key, "-", "_") == fold {
			return &p.zon.dependencies[i]
		}
	}
	return nil
}

type resolver struct {
	root        string
	files       map[string]bool
	directories map[string]bool
	packages    map[string]*zigPackage
	rootOf      map[string]string // file -> root source file of the compilation reaching it

	includeOnce sync.Once
	all         []*scan.File
	include     cpp.Includes

	mu           sync.Mutex
	hashes       map[string][]fetched     // package target key -> where it was fetched
	dependencies map[string][]lang.Target // memo of Dependencies
	found        map[string]bool
}

// fetched is a URL dependency with its hash, and the package that declared it.
type fetched struct {
	hash, directory string
}

// buildMarkers are words only build code uses; files without any are not
// evaluated for the build graph.
var buildMarkers = [][]byte{
	[]byte("addImport"), []byte("createModule"), []byte("addModule"), []byte("root_source_file"),
	[]byte("dependency("), []byte("Dependency("), []byte("addExecutable"), []byte("addTest"),
	[]byte("addLibrary"), []byte("addOptions"), []byte("source_file"),
}

func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, directories: map[string]bool{}, packages: map[string]*zigPackage{},
		rootOf: map[string]string{}, all: all, hashes: map[string][]fetched{}, dependencies: map[string][]lang.Target{}, found: map[string]bool{}}
	var sources []*scan.File
	layout := lang.Layout{Files: r.files, Directories: r.directories}
	for _, f := range all {
		layout.Add(f.Path)
		base := path.Base(f.Path)
		if skipped(f.Path) {
			continue
		}
		if base == "build.zig" || base == "build.zig.zon" {
			d := path.Dir(f.Path)
			if r.packages[d] == nil {
				r.packages[d] = &zigPackage{directory: d, facts: newFacts()}
			}
			if base == "build.zig.zon" {
				if source, ok := lang.ReadScanned(f); ok {
					r.packages[d].zon = readZon(source)
				}
			}
		}
		if path.Ext(f.Path) == ".zig" && lang.Readable(f) {
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
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil || !lang.Parseable(f, source) {
			continue
		}
		if p := r.packageOf(f.Path); p != nil && hasAny(source, buildMarkers) {
			evalBuild(source, p.directory, p.facts)
		}
		imports[f.Path] = importedNames(source)
	}
	for _, directory := range lang.SortedKeys(r.packages) {
		p := r.packages[directory]
		if p.zon == nil {
			continue
		}
		for _, d := range p.zon.dependencies {
			if d.url != "" && d.hash != "" {
				t := r.zonTarget(p.directory, d)
				r.hashes[key(t)] = append(r.hashes[key(t)], fetched{d.hash, p.directory})
			}
		}
	}
	r.roots(imports)
	return r
}

// importedNames lists what a file @imports, for following compilations from their
// roots: a byte scan, cheaper than lexing every file again (an @import in a comment
// only adds a harmless edge to the walk).
func importedNames(source []byte) []string {
	var out []string
	for {
		i := bytes.Index(source, []byte(`@import("`))
		if i < 0 {
			return out
		}
		source = source[i+9:]
		if j := bytes.IndexAny(source, "\"\n"); j > 0 && source[j] == '"' {
			out = append(out, string(source[:j]))
		}
	}
}

func hasAny(source []byte, words [][]byte) bool {
	for _, w := range words {
		if bytes.Contains(source, w) {
			return true
		}
	}
	return false
}

func key(t lang.Target) string { return t.Ecosystem + "\x00" + t.Package + "\x00" + t.Version }

// packageOf is the package a file belongs to: the nearest directory above it with
// build.zig or build.zig.zon.
func (r *resolver) packageOf(file string) *zigPackage {
	p, _ := lang.Nearest(r.packages, file)
	return p
}

// roots finds, for each file, the root source file of the compilation that reaches
// it through relative and module imports: the one reaching it, else the one
// non-test root among several, roots of the file's own package first. Build code
// does not belong to one.
//
// Implements: REQ-ZIG-004
func (r *resolver) roots(imports map[string][]string) {
	reached := map[string][]croot{}
	for _, directory := range lang.SortedKeys(r.packages) {
		p := r.packages[directory]
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
				for _, importPath := range imports[f] {
					var next string
					if strings.HasSuffix(importPath, ".zig") {
						next = path.Clean(path.Join(path.Dir(f), importPath))
					} else if t := r.module(f, importPath); strings.HasSuffix(t.Local, ".zig") {
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
	for f, roots := range reached {
		// The roots of the file's own package, when it has any: an example
		// depending on the package by path reaches its files too.
		if p := r.packageOf(f); p != nil {
			var own []croot
			for _, c := range roots {
				if q := r.packageOf(c.file); q == p {
					own = append(own, c)
				}
			}
			if len(own) > 0 {
				roots = own
			}
		}
		if len(roots) == 1 {
			r.rootOf[f] = roots[0].file
			continue
		}
		var main []string
		for _, c := range roots {
			if !c.test {
				main = append(main, c.file)
			}
		}
		if len(main) == 1 {
			r.rootOf[f] = main[0]
		}
	}
}

// Resolve maps an import to its target.
//
// Implements: REQ-ZIG-004, REQ-ZIG-005, REQ-ZIG-006, REQ-ZIG-008, REQ-ZIG-010
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	m := rawImport.Module
	switch rawImport.Name {
	case kindImport:
		switch {
		case m == "std" || m == "builtin":
			return lang.Target{Ecosystem: ecosystemStd, Package: m}
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
		r.includeOnce.Do(func() { r.include = cpp.NewIncludes(r.root, r.all) })
		return r.include.Resolve(file, lang.RawImport{Spec: rawImport.Spec, Module: m, Line: rawImport.Line}, nil)
	case kindBuild:
		if p := r.packageOf(file); p != nil {
			return r.local(path.Join(p.directory, m), true)
		}
	case kindDependency:
		p := r.packageOf(file)
		if d := p.dependency(m); d != nil {
			return r.zonTarget(p.directory, *d)
		}
	case kindZon:
		url, rest, _ := strings.Cut(m, "\n")
		hash, directory, _ := strings.Cut(rest, "\n")
		return r.zonTarget(path.Dir(file), zonDependency{key: rawImport.Spec, url: url, hash: hash, path: directory})
	case kindZigVersion:
		return lang.Target{Ecosystem: ecosystemStd, Package: "zig", Version: ">= " + m}
	}
	return lang.Target{}
}

// local is a project file, or with directories also a directory holding files; anything
// else (generated, outside the repository) is dropped.
func (r *resolver) local(p string, directories bool) lang.Target {
	p = path.Clean(p)
	if p == "." || lang.ClimbsOut(p) {
		return lang.Target{}
	}
	if r.files[p] || directories && r.directories[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// packageDirectory is a package directory of the repository as a target: the directory,
// or the root's build.zig.zon (the root is no directory of the map).
func (r *resolver) packageDirectory(directory string) lang.Target {
	if directory = path.Clean(directory); directory == "." {
		for _, f := range []string{"build.zig.zon", "build.zig"} {
			if r.files[f] {
				return lang.Target{Local: f}
			}
		}
	}
	return r.local(directory, true)
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
	p := r.packageOf(file)
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
	if d := p.dependency(name); d != nil {
		if d.path != "" {
			q := r.packages[path.Clean(path.Join(p.directory, d.path))]
			if build && q != nil && r.files[path.Join(q.directory, "build.zig")] {
				return lang.Target{Local: path.Join(q.directory, "build.zig")}
			}
			if !build && q != nil {
				if f := q.facts.exports[name]; f != "" && r.files[f] {
					return lang.Target{Local: f}
				}
			}
		}
		return r.zonTarget(p.directory, *d)
	}
	if build {
		return lang.Target{}
	}
	var hit string
	for _, directory := range lang.SortedKeys(r.packages) {
		if f := r.packages[directory].facts.exports[name]; f != "" && r.files[f] {
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
func (r *resolver) value(p *zigPackage, v value) (lang.Target, bool) {
	switch v.kind {
	case 'f', 'm', 'c':
		if v.path == "" {
			return lang.Target{}, true // generated
		}
		return r.local(v.path, false), true
	case 'g':
		return lang.Target{}, true
	case 'd':
		d := p.dependency(v.dependency)
		if d == nil {
			return lang.Target{}, false
		}
		if d.path != "" {
			directory := path.Clean(path.Join(p.directory, d.path))
			if q := r.packages[directory]; q != nil && v.module != "" {
				if f := q.facts.exports[v.module]; f != "" && r.files[f] {
					return lang.Target{Local: f}, true
				}
			}
		}
		return r.zonTarget(p.directory, *d), true
	}
	return lang.Target{}, false
}

var (
	// Archives of a ref: GitHub, Codeberg, sourcehut (archive/<ref>.tar.gz, also
	// archive/refs/tags/<ref>), GitLab (/-/archive/<ref>/<name>-<ref>.tar.gz), and
	// GitHub's tarball/<ref>.
	archiveReference = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+?)(?:/-)?/archive/(refs/tags/|refs/heads/)?([^/]+?)(?:/[^/]+)?(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
	tarballReference = regexp.MustCompile(`^(github\.com/[^/]+/[^/]+)/(?:tarball|zipball)/(.+)$`)
	releaseReference = regexp.MustCompile(`^([^/]+/[^/]+/[^/]+)/releases/download/([^/]+)/`)
	// A download named name-<version or commit>.tar.gz.
	namedArchive     = regexp.MustCompile(`^(.+)/([^/]+?)-(v?[0-9][0-9A-Za-z.+_-]*|[0-9a-f]{40})(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
	archiveExtension = regexp.MustCompile(`(?:\.tar\.gz|\.tgz|\.tar\.xz|\.tar\.zst|\.tar\.bz2|\.tar|\.zip)$`)
)

// zonTarget is a build.zig.zon dependency declared in directory: a directory of the
// repository for .path, else a package of the zig island named by its URL
// (lang.RepositoryName): a GitHub, GitLab, Codeberg or sourcehut archive or a git+https
// URL by its repository, other downloads by their URL without the archive's
// version. A .hash pins it (Zig verifies the content against it, as CMake does a
// URL_HASH); without one a commit pins, a tag is neither, a branch or no reference floats.
// The version is the URL's reference, else the version in the hash (name-1.2.3-...),
// else the hash.
//
// Implements: REQ-ZIG-007, REQ-ZIG-008
func (r *resolver) zonTarget(directory string, d zonDependency) lang.Target {
	if d.url == "" {
		if d.path != "" {
			return r.packageDirectory(path.Join(directory, d.path))
		}
		return lang.Target{Ecosystem: ecosystemZig, Package: d.key, Unresolved: true, Floating: true}
	}
	t := lang.Target{Ecosystem: ecosystemZig}
	reference, head := "", false
	u := strings.TrimSpace(d.url)
	if git, ok := strings.CutPrefix(u, "git+"); ok {
		var q string
		u, reference, _ = strings.Cut(git, "#")
		u, q, _ = strings.Cut(u, "?")
		t.Package = lang.RepositoryName(u)
		if requested := strings.TrimPrefix(q, "ref="); requested != q && requested != reference {
			t.Requested = requested
		}
	} else {
		u, _, _ = strings.Cut(u, "#")
		u, _, _ = strings.Cut(u, "?")
		t.Package = lang.RepositoryName(u)
		switch m := archiveReference.FindStringSubmatch(t.Package); {
		case m != nil:
			t.Package, reference, head = m[1], m[3], m[2] == "refs/heads/"
		default:
			if m := tarballReference.FindStringSubmatch(t.Package); m != nil {
				t.Package, reference = m[1], m[2]
			} else if m := releaseReference.FindStringSubmatch(t.Package); m != nil {
				t.Package, reference = m[1], m[2]
			} else if m := namedArchive.FindStringSubmatch(t.Package); m != nil && !hashLike(m[2]+"-"+m[3]) {
				t.Package, reference = m[1]+"/"+m[2], m[3]
			} else if base := archiveExtension.ReplaceAllString(path.Base(t.Package), ""); hashLike(base) || base == d.hash {
				t.Package = path.Dir(t.Package) + "/" + d.key // named by its hash: use the dependency's name
			} else {
				t.Package = archiveExtension.ReplaceAllString(t.Package, "")
			}
		}
	}
	t.Version = reference
	switch {
	case d.hash != "":
		t.Pinned = true
		t.Version = cmp.Or(t.Version, hashVersion(d.hash))
	case lang.Commit(reference):
		t.Pinned = true
	case reference == "" || head:
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
	if t.Ecosystem != ecosystemZig {
		return nil
	}
	k := key(t)
	r.mu.Lock()
	defer r.mu.Unlock()
	if out, ok := r.dependencies[k]; ok {
		return out
	}
	var out []lang.Target
	for _, f := range r.hashes[k] {
		source := r.fetchedZon(f)
		if source == nil {
			continue
		}
		r.found[k] = true
		for _, d := range readZon(source).dependencies {
			if d.url != "" { // a path inside the fetched package is not the repository's
				out = append(out, r.zonTarget(".", d))
			}
		}
		break
	}
	r.dependencies[k] = out
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
	// The project's own zig-pkg/ is read through the repository's Root, the
	// global caches of this machine through Machine.
	type candidate struct {
		files     lang.Root
		directory string
	}
	var candidates []candidate
	repository := lang.OpenRoot(r.root)
	for d := range lang.DirectoryAndAncestors(f.directory) {
		candidates = append(candidates, candidate{repository, filepath.Join(r.root, filepath.FromSlash(d), "zig-pkg", f.hash)})
	}
	if c := os.Getenv("ZIG_GLOBAL_CACHE_DIR"); c != "" {
		candidates = append(candidates, candidate{lang.Machine, filepath.Join(c, "p", f.hash)})
	}
	for _, environment := range []string{"XDG_CACHE_HOME", "LOCALAPPDATA"} { // LOCALAPPDATA: Windows
		if c := os.Getenv(environment); c != "" {
			candidates = append(candidates, candidate{lang.Machine, filepath.Join(c, "zig", "p", f.hash)})
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, candidate{lang.Machine, filepath.Join(h, ".cache", "zig", "p", f.hash)})
	}
	for _, c := range candidates {
		if source, ok := c.files.ReadBounded(filepath.Join(c.directory, "build.zig.zon")); ok {
			return source
		}
	}
	return nil
}

// skipped reports whether a path lies in Zig's build output or package caches.
func skipped(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if ignoredDirectories[segment] {
			return true
		}
	}
	return false
}
