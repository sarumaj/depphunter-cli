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
	directory    string
	dependencies []*dependency // jsonnetfile.json
	lock         []*dependency // jsonnetfile.lock.json
	legacy       bool          // legacy import names are linked (vendor/<name>)
	// locked is the lock that installs this project's dependencies: its own,
	// else that of a project listing it as a local source, else an
	// ancestor's.
	locked []*dependency
}

type resolver struct {
	root     string
	files    map[string]bool
	projects map[string]*project
	order    []*project // shallowest first
	jpath    []string   // JSONNET_PATH entries inside the repository
	exists   sync.Map   // repository-relative path -> bool, for what scan does not list
}

func readFile(absolute string) []byte {
	fileInfo, err := os.Stat(absolute)
	if err != nil || fileInfo.IsDir() || fileInfo.Size() > lang.MaxParseSize {
		return nil
	}
	b, _ := os.ReadFile(absolute)
	return b
}

// Implements: REQ-JSONNET-004, REQ-JSONNET-005, REQ-JSONNET-010
func newResolver(root string, all []*scan.File, getenv func(string) string) *resolver {
	r := &resolver{root: root, files: map[string]bool{}, projects: map[string]*project{}}
	projectAt := func(directory string) *project {
		p := r.projects[directory]
		if p == nil {
			p = &project{directory: directory, legacy: true}
			r.projects[directory] = p
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
			p := projectAt(path.Dir(f.Path))
			p.dependencies, p.legacy = readJsonnetfile(readFile(f.AbsolutePath))
		case "jsonnetfile.lock.json":
			projectAt(path.Dir(f.Path)).lock, _ = readJsonnetfile(readFile(f.AbsolutePath))
		}
	}
	for directory, p := range r.projects {
		// A lock git ignores is still what jb installed.
		if p.lock == nil && !r.files[path.Join(directory, "jsonnetfile.lock.json")] {
			p.lock, _ = readJsonnetfile(readFile(filepath.Join(root, filepath.FromSlash(directory), "jsonnetfile.lock.json")))
		}
		r.order = append(r.order, p)
	}
	sort.Slice(r.order, func(i, j int) bool { return lang.ShallowestFirst(r.order[i].directory, r.order[j].directory) })
	for _, p := range r.order {
		p.locked = p.lock
		for _, q := range r.order {
			for _, d := range q.dependencies {
				if len(p.locked) == 0 && d.local() && len(q.lock) > 0 && path.Join(q.directory, d.directory) == p.directory {
					p.locked = q.lock
				}
			}
		}
		for d := p.directory; len(p.locked) == 0 && d != "."; {
			d = path.Dir(d)
			if a := r.projects[d]; a != nil {
				p.locked = a.lock
			}
		}
	}
	absoluteRoot, _ := filepath.Abs(root)
	for _, e := range filepath.SplitList(getenv("JSONNET_PATH")) {
		if e == "" {
			continue
		}
		if !filepath.IsAbs(e) {
			e = filepath.Join(absoluteRoot, e)
		}
		if relative, err := filepath.Rel(absoluteRoot, e); err == nil && !strings.HasPrefix(filepath.ToSlash(relative), "../") && relative != ".." {
			r.jpath = append(r.jpath, filepath.ToSlash(relative))
		}
	}
	return r
}

// within reports whether p lies in directory, returning the rest.
func within(p, directory string) (string, bool) {
	if directory == "." {
		return p, true
	}
	return strings.CutPrefix(p, directory+"/")
}

// scope lists the projects whose directory holds file, nearest first.
func (r *resolver) scope(file string) []*project {
	return lang.Chain(r.projects, file)
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
	fileInfo, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(p)))
	ok := err == nil && !fileInfo.IsDir()
	r.exists.Store(p, ok)
	return ok
}

// Implements: REQ-JSONNET-004, REQ-JSONNET-005, REQ-JSONNET-006
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindDependency, kindLock:
		p := r.projects[path.Dir(file)]
		if p == nil {
			return lang.Target{}
		}
		list := p.dependencies
		if rawImport.Name == kindLock {
			list = p.lock
		}
		for _, d := range list {
			if d.packageName() == rawImport.Module {
				return r.target(p, d)
			}
		}
		return lang.Target{}
	}
	return r.source(file, rawImport.Module)
}

// source resolves an import as jsonnet does - relative to the importing file,
// then along the library path: each governing project's vendor/ and lib/,
// then JSONNET_PATH - and, when nothing is installed, by the manifests.
func (r *resolver) source(file, module string) lang.Target {
	if module == "" || strings.HasPrefix(module, "/") {
		return lang.Target{}
	}
	chain := r.scope(file)
	candidates := []string{path.Join(path.Dir(file), module)}
	for _, p := range chain {
		candidates = append(candidates, path.Join(p.directory, "vendor", module), path.Join(p.directory, "lib", module))
	}
	for _, j := range r.jpath {
		candidates = append(candidates, path.Join(j, module))
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
	if !strings.HasPrefix(module, "./") && !strings.HasPrefix(module, "../") {
		for d := range lang.Ancestors(path.Dir(file)) {
			if c := path.Join(d, module); r.files[c] {
				return lang.Target{Local: c}
			}
		}
	}
	// The manifests: the governing projects', then (a dependency reached
	// through another project of the repository) every project's.
	for _, list := range [][]*project{chain, r.order} {
		for _, p := range list {
			if d, rest, ok := p.match(module); ok {
				if d.local() {
					if f := path.Join(p.directory, d.directory, rest); r.files[f] {
						return lang.Target{Local: f}
					}
					return lang.Target{}
				}
				return r.target(p, d)
			}
		}
	}
	if strings.HasPrefix(module, "./") || strings.HasPrefix(module, "../") {
		return lang.Target{} // a relative file that is not there
	}
	return guess(module)
}

// inVendor reports whether c lies in a project's vendor/ (the nearest).
func (r *resolver) inVendor(c string) (*project, string, bool) {
	var best *project
	rest := ""
	for _, p := range r.order {
		if x, ok := within(c, path.Join(p.directory, "vendor")); ok {
			best, rest = p, x
		}
	}
	return best, rest, best != nil
}

// vendored names a file jb installed under p's vendor/ by the dependency
// that installed it (full path or legacy link); a directory no manifest
// lists is an unresolved package.
func (r *resolver) vendored(p *project, rest string) lang.Target {
	if d, subdirectory, ok := p.match(rest); ok {
		if d.local() {
			return lang.Target{Local: path.Join(p.directory, d.directory, subdirectory)}
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
func (p *project) match(module string) (*dependency, string, bool) {
	for _, list := range [][]*dependency{p.dependencies, p.locked} {
		var best *dependency
		rest := ""
		for _, d := range list {
			if d.local() {
				continue
			}
			if x, ok := within(module, d.packageName()); ok && (best == nil || len(d.packageName()) > len(best.packageName())) {
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
	first, rest, _ := strings.Cut(module, "/")
	for _, list := range [][]*dependency{p.dependencies, p.locked} {
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
func guess(module string) lang.Target {
	segments := strings.Split(module, "/")
	if len(segments) < 2 {
		return lang.Target{}
	}
	directories := segments[:len(segments)-1]
	name := directories[0]
	if strings.Contains(directories[0], ".") {
		n := 2
		if forges[directories[0]] {
			n = 3
		}
		name = strings.ToLower(directories[0]) + strings.TrimPrefix(strings.Join(directories[:min(n, len(directories))], "/"), directories[0])
	}
	return lang.Target{Ecosystem: ecosystemJB, Package: name, Unresolved: true}
}

// target is a git dependency of p: the lock's version pins (the declared one
// requested when it differs); else a commit pins, a tag is shown neither
// pinned nor floating, and a branch or nothing floats.
//
// Implements: REQ-JSONNET-006
func (r *resolver) target(p *project, d *dependency) lang.Target {
	if d.local() {
		return lang.Target{Local: path.Join(p.directory, d.directory)}
	}
	name := d.packageName()
	t := lang.Target{Ecosystem: ecosystemJB, Package: name}
	if !public(d.remote) {
		t.Origin = d.remote
	}
	declared := d
	for _, x := range p.dependencies {
		if x.packageName() == name {
			declared = x
			break
		}
	}
	for _, l := range p.locked {
		if l.packageName() == name && l.version != "" {
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
	host, _, _ := strings.Cut(lang.RepositoryName(url), "/")
	return forges[host]
}

// Dependencies lists what an installed package's own jsonnetfile.json
// declares (vendor/<package>/jsonnetfile.json), versioned by the installing
// project's lock.
//
// Implements: REQ-JSONNET-007
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	p, dependencies := r.installed(t)
	if p == nil {
		return nil
	}
	var out []lang.Target
	for _, d := range dependencies {
		if !d.local() {
			out = append(out, r.target(p, d))
		}
	}
	return out
}

func (r *resolver) installed(t lang.Target) (*project, []*dependency) {
	if t.Ecosystem != ecosystemJB || t.Package == "" || strings.Contains(t.Package, "..") {
		return nil, nil
	}
	for _, p := range r.order {
		source := readFile(filepath.Join(r.root, filepath.FromSlash(p.directory), "vendor", filepath.FromSlash(t.Package), "jsonnetfile.json"))
		if source != nil {
			dependencies, _ := readJsonnetfile(source)
			return p, dependencies
		}
	}
	return nil, nil
}

// Installed reports that the dependencies come from what jb installed.
func (r *resolver) Installed(t lang.Target) bool {
	p, _ := r.installed(t)
	return p != nil
}
