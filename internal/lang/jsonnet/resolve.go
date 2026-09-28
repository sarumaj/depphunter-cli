package jsonnet

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is a directory with a jsonnetfile.json: jsonnet-bundler's root,
// whose vendor/ and lib/ are on the import path of the files below it.
type project struct {
	dir    string
	deps   []*dep // jsonnetfile.json
	lock   []*dep // jsonnetfile.lock.json
	legacy bool   // legacy import names are linked (vendor/<name>)
	// locked is the lock that installs this project's dependencies: its own,
	// else that of a project listing it as a local source, else an
	// ancestor's.
	locked []*dep
}

type resolver struct {
	root     string
	files    map[string]bool
	projects map[string]*project
	order    []*project // shallowest first
	jpath    []string   // JSONNET_PATH entries inside the repository
	exists   sync.Map   // repository-relative path -> bool, for what scan does not list
}

func readFile(abs string) []byte {
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() || st.Size() > lang.MaxParseSize {
		return nil
	}
	b, _ := os.ReadFile(abs)
	return b
}

// Implements: REQ-JSONNET-004, REQ-JSONNET-005, REQ-JSONNET-010
func newResolver(root string, all []*scan.File, getenv func(string) string) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, projects: map[string]*project{}}
	proj := func(dir string) *project {
		p := r.projects[dir]
		if p == nil {
			p = &project{dir: dir, legacy: true}
			r.projects[dir] = p
		}
		return p
	}
	for _, f := range all {
		r.files[f.Path] = true
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize || ignored(f) {
			continue
		}
		switch path.Base(f.Path) {
		case "jsonnetfile.json":
			p := proj(path.Dir(f.Path))
			p.deps, p.legacy = readJsonnetfile(readFile(f.Abs))
		case "jsonnetfile.lock.json":
			proj(path.Dir(f.Path)).lock, _ = readJsonnetfile(readFile(f.Abs))
		}
	}
	for dir, p := range r.projects {
		// A lock git ignores is still what jb installed.
		if p.lock == nil && !r.files[path.Join(dir, "jsonnetfile.lock.json")] {
			p.lock, _ = readJsonnetfile(readFile(filepath.Join(root, filepath.FromSlash(dir), "jsonnetfile.lock.json")))
		}
		r.order = append(r.order, p)
	}
	sort.Slice(r.order, func(i, j int) bool {
		di, dj := depth(r.order[i].dir), depth(r.order[j].dir)
		if di != dj {
			return di < dj
		}
		return r.order[i].dir < r.order[j].dir
	})
	for _, p := range r.order {
		p.locked = p.lock
		for _, q := range r.order {
			for _, d := range q.deps {
				if len(p.locked) == 0 && d.local() && len(q.lock) > 0 && path.Join(q.dir, d.dir) == p.dir {
					p.locked = q.lock
				}
			}
		}
		for d := p.dir; len(p.locked) == 0 && d != "."; {
			d = path.Dir(d)
			if a := r.projects[d]; a != nil {
				p.locked = a.lock
			}
		}
	}
	absRoot, _ := filepath.Abs(root)
	for _, e := range filepath.SplitList(getenv("JSONNET_PATH")) {
		if e == "" {
			continue
		}
		if !filepath.IsAbs(e) {
			e = filepath.Join(absRoot, e)
		}
		if rel, err := filepath.Rel(absRoot, e); err == nil && !strings.HasPrefix(filepath.ToSlash(rel), "../") && rel != ".." {
			r.jpath = append(r.jpath, filepath.ToSlash(rel))
		}
	}
	return r
}

func depth(dir string) int {
	if dir == "." {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// within reports whether p lies in dir, returning the rest.
func within(p, dir string) (string, bool) {
	if dir == "." {
		return p, true
	}
	return strings.CutPrefix(p, dir+"/")
}

// scope lists the projects whose directory holds file, nearest first.
func (r *resolver) scope(file string) []*project {
	var out []*project
	for d := path.Dir(file); ; d = path.Dir(d) {
		if p := r.projects[d]; p != nil {
			out = append(out, p)
		}
		if d == "." {
			return out
		}
	}
}

// present reports whether a repository path exists: listed, or on disk (what
// jb installed into vendor/ is not scanned).
func (r *resolver) present(p string) bool {
	if r.files[p] {
		return true
	}
	if v, ok := r.exists.Load(p); ok {
		return v.(bool)
	}
	st, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(p)))
	ok := err == nil && !st.IsDir()
	r.exists.Store(p, ok)
	return ok
}

// Implements: REQ-JSONNET-004, REQ-JSONNET-005, REQ-JSONNET-006
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	switch imp.Name {
	case kindDep, kindLock:
		p := r.projects[path.Dir(file)]
		if p == nil {
			return lang.Target{}
		}
		list := p.deps
		if imp.Name == kindLock {
			list = p.lock
		}
		for _, d := range list {
			if d.pkg() == imp.Module {
				return r.target(p, d)
			}
		}
		return lang.Target{}
	}
	return r.source(file, imp.Module)
}

// source resolves an import as jsonnet does - relative to the importing file,
// then along the library path: each governing project's vendor/ and lib/,
// then JSONNET_PATH - and, when nothing is installed, by the manifests.
func (r *resolver) source(file, mod string) lang.Target {
	if mod == "" || strings.HasPrefix(mod, "/") {
		return lang.Target{}
	}
	chain := r.scope(file)
	candidates := []string{path.Join(path.Dir(file), mod)}
	for _, p := range chain {
		candidates = append(candidates, path.Join(p.dir, "vendor", mod), path.Join(p.dir, "lib", mod))
	}
	for _, j := range r.jpath {
		candidates = append(candidates, path.Join(j, mod))
	}
	for _, c := range candidates {
		if c == ".." || strings.HasPrefix(c, "../") {
			continue
		}
		if p, rest, ok := r.inVendor(c); ok {
			if r.present(c) {
				return r.vendored(p, rest)
			}
			continue
		}
		if r.files[c] {
			return lang.Target{Local: c}
		}
	}
	// A path under an ancestor of the file: jsonnet run with -J at a parent
	// directory (mimir's tests import mimir/ from operations/).
	if !strings.HasPrefix(mod, "./") && !strings.HasPrefix(mod, "../") {
		for d := path.Dir(file); d != "."; {
			d = path.Dir(d)
			if c := path.Join(d, mod); r.files[c] {
				return lang.Target{Local: c}
			}
		}
	}
	// The manifests: the governing projects', then (a dependency reached
	// through another project of the repository) every project's.
	for _, list := range [][]*project{chain, r.order} {
		for _, p := range list {
			if d, rest, ok := p.match(mod); ok {
				if d.local() {
					if f := path.Join(p.dir, d.dir, rest); r.files[f] {
						return lang.Target{Local: f}
					}
					return lang.Target{}
				}
				return r.target(p, d)
			}
		}
	}
	if strings.HasPrefix(mod, "./") || strings.HasPrefix(mod, "../") {
		return lang.Target{} // a relative file that is not there
	}
	return guess(mod)
}

// inVendor reports whether c lies in a project's vendor/ (the nearest).
func (r *resolver) inVendor(c string) (*project, string, bool) {
	var best *project
	rest := ""
	for _, p := range r.order {
		if x, ok := within(c, path.Join(p.dir, "vendor")); ok {
			best, rest = p, x
		}
	}
	return best, rest, best != nil
}

// vendored names a file jb installed under p's vendor/ by the dependency
// that installed it (full path or legacy link); a directory no manifest
// lists is an unresolved package.
func (r *resolver) vendored(p *project, rest string) lang.Target {
	if d, sub, ok := p.match(rest); ok {
		if d.local() {
			return lang.Target{Local: path.Join(p.dir, d.dir, sub)}
		}
		return r.target(p, d)
	}
	t := guess(rest)
	if t.Package == "" {
		return lang.Target{}
	}
	return t
}

// match finds the dependency an import path names: the longest full-path
// prefix (host/owner/repo/subdir), else a legacy name as the first element.
// jsonnetfile.json is asked before the lock (which adds transitive ones).
func (p *project) match(mod string) (*dep, string, bool) {
	for _, list := range [][]*dep{p.deps, p.locked} {
		var best *dep
		rest := ""
		for _, d := range list {
			if d.local() {
				continue
			}
			if x, ok := within(mod, d.pkg()); ok && (best == nil || len(d.pkg()) > len(best.pkg())) {
				best, rest = d, x
			}
		}
		if best != nil {
			return best, rest, true
		}
	}
	if !p.legacy {
		return nil, "", false
	}
	first, rest, _ := strings.Cut(mod, "/")
	for _, list := range [][]*dep{p.deps, p.locked} {
		for _, d := range list {
			if d.legacy() == first {
				return d, rest, true
			}
		}
	}
	return nil, "", false
}

var forges = map[string]bool{"github.com": true, "gitlab.com": true, "bitbucket.org": true, "codeberg.org": true, "git.sr.ht": true}

// guess names the package of an import no manifest explains: a full path by
// its repository (host/owner/repo on a forge, else host/first), a legacy
// path by its first element. A bare file name is dropped.
func guess(mod string) lang.Target {
	segments := strings.Split(mod, "/")
	if len(segments) < 2 {
		return lang.Target{}
	}
	dirs := segments[:len(segments)-1]
	name := dirs[0]
	if strings.Contains(dirs[0], ".") {
		n := 2
		if forges[dirs[0]] {
			n = 3
		}
		name = strings.ToLower(dirs[0]) + strings.TrimPrefix(strings.Join(dirs[:min(n, len(dirs))], "/"), dirs[0])
	}
	return lang.Target{Ecosystem: ecoJB, Package: name, Unresolved: true}
}

// target is a git dependency of p: the lock's version pins (the declared one
// requested when it differs); else a commit pins, a tag is shown neither
// pinned nor floating, and a branch or nothing floats.
//
// Implements: REQ-JSONNET-006
func (r *resolver) target(p *project, d *dep) lang.Target {
	if d.local() {
		return lang.Target{Local: path.Join(p.dir, d.dir)}
	}
	name := d.pkg()
	t := lang.Target{Ecosystem: ecoJB, Package: name}
	if !public(d.remote) {
		t.Origin = d.remote
	}
	declared := d
	for _, x := range p.deps {
		if x.pkg() == name {
			declared = x
			break
		}
	}
	for _, l := range p.locked {
		if l.pkg() == name && l.version != "" {
			t.Version, t.Pinned = l.version, true
			if declared != l && declared.version != "" && declared.version != l.version {
				t.Requested = declared.version
			}
			return t
		}
	}
	v := declared.version
	t.Version = v
	switch {
	case lang.Commit(v):
		t.Pinned = true
	case tagLike.MatchString(v):
	default:
		t.Floating = true
	}
	return t
}

func public(url string) bool {
	host, _, _ := strings.Cut(lang.RepoName(url), "/")
	return forges[host]
}

// Dependencies lists what an installed package's own jsonnetfile.json
// declares (vendor/<package>/jsonnetfile.json), versioned by the installing
// project's lock.
//
// Implements: REQ-JSONNET-007
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	p, deps := r.installed(t)
	if p == nil {
		return nil
	}
	var out []lang.Target
	for _, d := range deps {
		if !d.local() {
			out = append(out, r.target(p, d))
		}
	}
	return out
}

func (r *resolver) installed(t lang.Target) (*project, []*dep) {
	if t.Ecosystem != ecoJB || t.Package == "" || strings.Contains(t.Package, "..") {
		return nil, nil
	}
	for _, p := range r.order {
		src := readFile(filepath.Join(r.root, filepath.FromSlash(p.dir), "vendor", filepath.FromSlash(t.Package), "jsonnetfile.json"))
		if src != nil {
			deps, _ := readJsonnetfile(src)
			return p, deps
		}
	}
	return nil, nil
}

// Installed reports that the dependencies come from what jb installed.
func (r *resolver) Installed(t lang.Target) bool {
	p, _ := r.installed(t)
	return p != nil
}
