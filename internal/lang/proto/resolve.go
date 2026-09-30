package proto

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// conventional are the directories, relative to the repository root, that protoc's
// -I is usually pointed at when no Buf configuration says where the import roots are.
var conventional = []string{".", "proto", "protos", "api", "src/main/proto"}

// config is one buf.yaml or buf.work.yaml.
type config struct {
	directory string
	file      *bufFile
	roots     []string // import roots, project-relative
	lock      map[string]locked
}

// dependency is a module a configuration declares or its lock records.
type dependency struct {
	name, reference string
	lock            *locked
}

type resolver struct {
	files       map[string]bool
	directories map[string]bool
	byBase      map[string][]string // file name -> the project files so named
	work        map[string]*config  // directory -> its buf.work.yaml or v2 buf.yaml
	modules     map[string]*config  // directory -> its v1 (or v1beta1) buf.yaml
	all         []*config           // every buf.yaml, shallowest first
	locks       map[string]map[string]locked
	protoc      []protocRoot // the -I directories build scripts give protoc
}

// newResolver reads every Buf configuration and lock file of the repository.
//
// Implements: REQ-PROTO-004, REQ-PROTO-005, REQ-PROTO-007
func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, directories: map[string]bool{}, byBase: map[string][]string{},
		work: map[string]*config{}, modules: map[string]*config{}, locks: map[string]map[string]locked{}}
	var configs []*scan.File
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); !r.directories[d]; d = path.Dir(d) {
			r.directories[d] = true
			if d == "." {
				break
			}
		}
		base := path.Base(f.Path)
		r.byBase[base] = append(r.byBase[base], f.Path)
		switch fileClass(f.Path) {
		case classYAML, classWork, classLock:
			if !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize {
				configs = append(configs, f)
			}
		}
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].Path < configs[j].Path })
	for _, f := range configs {
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		class, directory := fileClass(f.Path), path.Dir(f.Path)
		b := readBuf(class, source)
		if class == classLock {
			m := map[string]locked{}
			for _, l := range b.locks {
				m[l.name] = l
			}
			r.locks[directory] = m
			continue
		}
		c := &config{directory: directory, file: b}
		for _, root := range b.roots {
			if p := path.Join(directory, root.value); !strings.HasPrefix(p, "../") {
				c.roots = append(c.roots, p)
			}
		}
		switch {
		case class == classWork:
			r.work[directory] = c
		case b.v2:
			if len(c.roots) == 0 {
				c.roots = []string{directory}
			}
			r.work[directory] = c
			r.all = append(r.all, c)
		default:
			if len(c.roots) == 0 {
				c.roots = []string{directory}
			}
			r.modules[directory] = c
			r.all = append(r.all, c)
		}
	}
	for _, c := range r.all {
		c.lock = r.locks[c.directory]
	}
	r.protoc = readProtocRoots(all, r.directories)
	sort.SliceStable(r.all, func(i, j int) bool { return lang.Depth(r.all[i].directory) < lang.Depth(r.all[j].directory) })
	return r
}

// nearest finds the configuration in m of directory or its closest ancestor.
func nearest(m map[string]*config, directory string) *config {
	for d := directory; ; d = path.Dir(d) {
		if c := m[d]; c != nil {
			return c
		}
		if d == "." || d == "/" {
			return nil
		}
	}
}

// scope is what Buf's configuration says about one file: its workspace's import roots
// and the modules its module depends on.
type scope struct {
	buf          bool // a Buf configuration governs the file
	roots        []string
	dependencies []dependency
}

// scopeOf finds the Buf workspace (buf.work.yaml or a v2 buf.yaml) and the v1 module
// a file belongs to, nearest first. A file no configuration governs sees the
// dependencies of every buf.yaml in the repository, shallowest first.
//
// Implements: REQ-PROTO-004, REQ-PROTO-005
func (r *resolver) scopeOf(file string) scope {
	directory := path.Dir(file)
	workspaceConfig, module := nearest(r.work, directory), nearest(r.modules, directory)
	if workspaceConfig != nil && module != nil && !lang.WithinOrEqual(module.directory, workspaceConfig.directory) {
		module = nil // a module above the workspace is not part of it
	}
	var current scope
	var configs []*config
	switch {
	case workspaceConfig != nil:
		current.buf, current.roots = true, workspaceConfig.roots
		if workspaceConfig.file.v2 {
			configs = append(configs, workspaceConfig)
		}
		if module != nil {
			configs = append(configs, module)
		}
	case module != nil:
		current.buf, current.roots = true, module.roots
		configs = append(configs, module)
	default:
		configs = r.all
	}
	for _, c := range configs {
		seen := map[string]bool{}
		for _, d := range c.file.dependencies {
			name, reference := moduleReference(d.value)
			seen[name] = true
			var l *locked
			if e, ok := c.lock[name]; ok {
				l = &e
			}
			current.dependencies = append(current.dependencies, dependency{name: name, reference: reference, lock: l})
		}
		for _, name := range sortedLocks(c.lock) { // installed as a dependency's dependency
			if !seen[name] {
				e := c.lock[name]
				current.dependencies = append(current.dependencies, dependency{name: name, lock: &e})
			}
		}
	}
	return current
}

func sortedLocks(m map[string]locked) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Resolve follows an import to a project file, a well-known type or a Buf Schema
// Registry module; a Buf file's entries to the modules, plugins and directories they
// name.
//
// Implements: REQ-PROTO-004, REQ-PROTO-005, REQ-PROTO-006, REQ-PROTO-007, REQ-PROTO-008
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	switch rawImport.Name {
	case kindDirectory:
		if p := path.Join(path.Dir(file), rawImport.Module); r.directories[p] && !strings.HasPrefix(p, "../") {
			return lang.Target{Local: p}
		}
		return lang.Target{}
	case kindDependency:
		name, reference := moduleReference(rawImport.Module)
		var l *locked
		if e, ok := r.locks[path.Dir(file)][name]; ok {
			l = &e
		}
		return dependency{name: name, reference: reference, lock: l}.target()
	case kindLock:
		l := r.locks[path.Dir(file)][rawImport.Module]
		return dependency{name: rawImport.Module, lock: &l}.target()
	case kindPlugin:
		name, version := moduleReference(rawImport.Module)
		return lang.Target{Ecosystem: ecoBuf, Package: name, Version: version, Pinned: lang.Pinned(version), Floating: version == "",
			Registry: remotePlugin}
	}
	return r.resolveImport(file, rawImport.Module)
}

// target is a declared module: pinned by its lock's commit, else by a commit reference; a
// label, tag or branch reference is shown but moves; no reference floats.
//
// Implements: REQ-PROTO-008
func (d dependency) target() lang.Target {
	t := lang.Target{Ecosystem: ecoBuf, Package: d.name}
	switch {
	case d.lock != nil && d.lock.commit != "":
		t.Version, t.Pinned = d.lock.commit, true
		if d.reference != "" && d.reference != d.lock.commit {
			t.Requested = d.reference
		}
	case d.reference != "":
		t.Version, t.Pinned = d.reference, commit(d.reference)
	default:
		t.Floating = true
	}
	return t
}

// commit reports whether a module reference is a commit: a Buf Schema Registry commit id
// (32 hex digits, or a dashed UUID) or a git commit.
func commit(reference string) bool {
	if lang.Commit(reference) {
		return true
	}
	s := strings.ReplaceAll(reference, "-", "")
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// resolveImport resolves `import "name";` from file.
//
// Implements: REQ-PROTO-004, REQ-PROTO-006
func (r *resolver) resolveImport(file, name string) lang.Target {
	name = strings.ReplaceAll(name, `\`, "/")
	if name == "" || path.IsAbs(name) {
		return lang.Target{}
	}
	name = path.Clean(name)
	if name == ".." || strings.HasPrefix(name, "../") {
		return lang.Target{}
	}
	current := r.scopeOf(file)
	roots := current.roots
	if !current.buf {
		roots = append(r.protocRootsFor(file), heuristic(file)...)
	}
	if p := r.find(roots, name); p != "" {
		return lang.Target{Local: p}
	}
	if wellKnown(name) {
		return lang.Target{Ecosystem: ecosystemStd, Package: name}
	}
	if d := current.match(name); d != nil {
		return d.target()
	}
	if current.buf { // protoc may be pointed elsewhere than Buf is
		if p := r.find(append(r.protocRootsFor(file), heuristic(file)...), name); p != "" {
			return lang.Target{Local: p}
		}
	}
	if p := r.bySuffix(file, name); p != "" {
		return lang.Target{Local: p}
	}
	if modules := known(name); modules != nil {
		return lang.Target{Ecosystem: ecoBuf, Package: modules[0], Unresolved: true}
	}
	first, _, _ := strings.Cut(name, "/")
	return lang.Target{Ecosystem: ecoBuf, Package: strings.TrimSuffix(first, ".proto"), Unresolved: true}
}

// heuristic lists the import roots protoc is usually given: the repository root and
// its conventional proto directories, then the importer's directory and each of its
// ancestors, nearest first.
func heuristic(file string) []string {
	roots := append([]string{}, conventional...)
	for d := path.Dir(file); d != "."; d = path.Dir(d) {
		roots = append(roots, d)
	}
	return roots
}

func (r *resolver) find(roots []string, name string) string {
	for _, root := range roots {
		if p := path.Join(root, name); r.files[p] {
			return p
		}
	}
	return ""
}

// match finds the declared module an import comes from: one the table of known protos
// names for its path, else the one whose repository (or, alone with it, owner) is the
// path's first directory, compared without case, dashes, dots or underscores.
func (current scope) match(name string) *dependency {
	for _, m := range known(name) {
		for i := range current.dependencies {
			if current.dependencies[i].name == m {
				return &current.dependencies[i]
			}
		}
	}
	first, _, ok := strings.Cut(name, "/")
	if !ok {
		return nil
	}
	first = lang.FoldSeparators(first)
	var byRepository, byOwner []*dependency
	for i := range current.dependencies {
		segments := strings.Split(current.dependencies[i].name, "/")
		if len(segments) < 3 {
			continue
		}
		if lang.FoldSeparators(segments[len(segments)-1]) == first {
			byRepository = append(byRepository, &current.dependencies[i])
		}
		if lang.FoldSeparators(segments[len(segments)-2]) == first {
			byOwner = append(byOwner, &current.dependencies[i])
		}
	}
	switch {
	case len(byRepository) > 0:
		return byRepository[0] // the nearest configuration's comes first
	case len(byOwner) == 1:
		return byOwner[0]
	}
	return nil
}

// bySuffix finds the project file whose path ends in the import ("foo/bar.proto" in
// third_party/foo/bar.proto): the only one, or else the one closest to the importer
// when that is closer than all the others.
func (r *resolver) bySuffix(file, name string) string {
	var best string
	bestLength, tie := -1, false
	for _, p := range r.byBase[path.Base(name)] {
		if !strings.HasSuffix(p, "/"+name) {
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
	if tie {
		return ""
	}
	return best
}
