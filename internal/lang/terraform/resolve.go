package terraform

import (
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// moduleDir is one Terraform module: a directory's configuration files read together.
type moduleDir struct {
	decl      map[string]string // symbol name -> first file declaring it
	providers map[string]*required
	calls     []string // directories of the local modules it calls
	lock      map[string]lockEntry
}

// required is what a module says about one provider local name, across its files.
type required struct {
	source      string
	constraints []string
}

type resolver struct {
	files   map[string]bool
	dirs    map[string]bool
	modules map[string]*moduleDir
	callers map[string][]string  // module directory -> directories calling it
	tg      map[string]*tgConfig // Terragrunt configuration -> its locals and includes
}

func dirOf(p string) string { return path.Dir(p) }

// newResolver reads every configuration file of the repository once more (the
// scanner is fast) to learn what each module declares and which providers it
// requires, and every lock file for the versions it pins.
//
// Implements: REQ-TERRAFORM-005, REQ-TERRAFORM-007, REQ-TERRAFORM-008
func newResolver(all []*scan.File) *resolver {
	r := &resolver{files: map[string]bool{}, dirs: map[string]bool{}, modules: map[string]*moduleDir{}, callers: map[string][]string{},
		tg: map[string]*tgConfig{}}
	var configs []*scan.File
	for _, f := range all {
		r.files[f.Path] = true
		for d := path.Dir(f.Path); !r.dirs[d]; d = path.Dir(d) {
			r.dirs[d] = true
			if d == "." {
				break
			}
		}
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize || ignored(f.Path) {
			continue
		}
		switch fileClass(f.Path) {
		case classConfig, classJSON, classLock, classTerragrunt:
			configs = append(configs, f)
		}
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].Path < configs[j].Path })
	for _, f := range configs {
		src, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		class := fileClass(f.Path)
		if class == classTerragrunt {
			r.tg[f.Path] = readTerragrunt(parse(src)).tg
			continue
		}
		m := r.module(dirOf(f.Path))
		if class == classLock {
			m.lock = map[string]lockEntry{}
			for _, e := range lockEntries(parse(src)) {
				m.lock[providerSource(e.addr)] = e
			}
			continue
		}
		fi := read(class, src)
		for _, s := range fi.symbols.List() {
			if _, ok := m.decl[s.Name]; !ok {
				m.decl[s.Name] = f.Path
			}
		}
		for _, local := range sortedKeys(fi.providers) {
			req := fi.providers[local]
			have := m.providers[local]
			if have == nil {
				have = &required{}
				m.providers[local] = have
			}
			if have.source == "" && req.source != "" {
				have.source = req.source
			}
			if req.constraint != "" && !contains(have.constraints, req.constraint) {
				have.constraints = append(have.constraints, req.constraint)
			}
		}
		for _, im := range fi.imports {
			if strings.HasPrefix(im.Name, kindModule+"|") {
				if s, ok := parseSource(im.Module); ok && s.local != "" {
					callee := path.Clean(path.Join(dirOf(f.Path), s.local))
					m.calls = append(m.calls, callee)
					r.callers[callee] = append(r.callers[callee], dirOf(f.Path))
				}
			}
		}
	}
	return r
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (r *resolver) module(dir string) *moduleDir {
	m := r.modules[dir]
	if m == nil {
		m = &moduleDir{decl: map[string]string{}, providers: map[string]*required{}}
		r.modules[dir] = m
	}
	return m
}

// lookup is module for Resolve, which runs concurrently and must not add one.
func (r *resolver) lookup(dir string) *moduleDir {
	if m := r.modules[dir]; m != nil {
		return m
	}
	return &moduleDir{}
}

// Implements: REQ-TERRAFORM-004, REQ-TERRAFORM-005, REQ-TERRAFORM-006, REQ-TERRAFORM-007, REQ-TERRAFORM-008, REQ-TERRAFORM-009, REQ-TERRAFORM-010
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	dir := dirOf(file)
	kind, arg, _ := strings.Cut(imp.Name, "|")
	switch kind {
	case kindModule:
		return r.moduleTarget(dir, imp.Module, arg)
	case kindTGSource:
		src := r.expandIncludes(file, imp.Module)
		if strings.Contains(src, "${") {
			return lang.Target{}
		}
		return r.moduleTarget(dir, src, "")
	case kindProvider:
		return r.provider(dir, imp.Module)
	case kindLock:
		src := providerSource(imp.Module)
		t := lang.Target{Ecosystem: ecoProvider, Package: src}
		if e := r.lookup(dir).lock[src]; e.version != "" {
			t.Version, t.Pinned, t.Requested = e.version, true, e.constraints
		}
		return t
	case kindRef:
		if f, ok := r.lookup(dir).decl[imp.Module]; ok && f != file {
			return lang.Target{Local: f}
		}
		return lang.Target{}
	case kindFile:
		return r.local(path.Join(dir, imp.Module))
	case kindTGDep:
		p := path.Clean(path.Join(dir, imp.Module))
		if r.files[path.Join(p, "terragrunt.hcl")] {
			return lang.Target{Local: path.Join(p, "terragrunt.hcl")}
		}
		return r.local(p)
	case kindTGParent:
		if f := r.parentFile(dir, imp.Module); f != "" {
			if arg == "" {
				return lang.Target{Local: f}
			}
			return r.local(path.Join(path.Dir(f), arg))
		}
	}
	return lang.Target{}
}

// parentFile is what find_in_parent_folders(name) finds for a configuration in dir:
// the nearest file of that name in a directory above it.
func (r *resolver) parentFile(dir, name string) string {
	if dir == "." {
		return ""
	}
	for d := path.Dir(dir); ; d = path.Dir(d) {
		if f := path.Join(d, name); r.files[f] {
			return f
		}
		if d == "." {
			return ""
		}
	}
}

var includeLocal = regexp.MustCompile(`\$\{\s*include\.([A-Za-z_][A-Za-z0-9_-]*)\.locals\.([A-Za-z_][A-Za-z0-9_-]*)\s*\}`)

// expandIncludes substitutes the locals of included files into a Terragrunt source:
// "${include.envcommon.locals.base_source_url}?ref=v0.8.0", with the include's path
// found as Terragrunt finds it and the local a string in that file.
//
// Implements: REQ-TERRAFORM-010
func (r *resolver) expandIncludes(file, s string) string {
	cfg := r.tg[file]
	if cfg == nil {
		return s
	}
	return includeLocal.ReplaceAllStringFunc(s, func(m string) string {
		sub := includeLocal.FindStringSubmatch(m)
		inc, ok := cfg.includes[sub[1]]
		if !ok {
			return m
		}
		target := ""
		if rest, ok := strings.CutPrefix(inc, "parent:"); ok {
			name, tail, _ := strings.Cut(rest, "|")
			if f := r.parentFile(dirOf(file), name); f != "" {
				target = path.Clean(path.Join(path.Dir(f), tail))
				if tail == "" {
					target = f
				}
			}
		} else if inc != "" {
			target = path.Clean(path.Join(dirOf(file), inc))
		}
		if other := r.tg[target]; other != nil {
			if v, ok := other.locals[sub[2]]; ok && !strings.Contains(v, "${") {
				return v
			}
		}
		return m
	})
}

// local is a file or directory of the repository, else nothing.
func (r *resolver) local(p string) lang.Target {
	p = path.Clean(p)
	if p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
		return lang.Target{}
	}
	if r.files[p] || r.dirs[p] {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// moduleTarget resolves a module source: a local module's directory, a registry
// module pinned by an exact version, or a remote module pinned by a commit.
//
// Implements: REQ-TERRAFORM-004, REQ-TERRAFORM-009
func (r *resolver) moduleTarget(dir, source, version string) lang.Target {
	s, ok := parseSource(source)
	switch {
	case !ok:
		// Terraform before 0.12 took a bare directory name ("subnets") as a path.
		if !strings.ContainsAny(source, ":?") {
			return r.local(path.Join(dir, source))
		}
		return lang.Target{}
	case s.local != "":
		return r.local(path.Join(dir, s.local))
	case s.origin != "":
		// A tag names a release but can be moved; only a commit pins, and without a
		// ref the default branch floats (the rule of GitHub Actions, REQ-CI-011).
		t := lang.Target{Ecosystem: ecoModule, Package: s.pkg, Version: s.ref, Origin: s.origin,
			Pinned: lang.Commit(s.ref), Floating: s.ref == ""}
		if s.archive && s.ref == "" {
			t.Floating = true
		}
		return t
	}
	if version == "" {
		version = s.ref // tfr://...?version=
	}
	t := lang.Target{Ecosystem: ecoModule, Package: s.pkg, Version: strings.TrimSpace(version)}
	if v, ok := exactVersion(version); ok {
		t.Version, t.Pinned = v, true
	}
	t.Floating = t.Version == ""
	return t
}

// provider resolves a provider local name in a module: its source from the module's
// required_providers (else HashiCorp's provider of that name, as Terraform assumes),
// its version from the lock file of the module or of a module calling it, else from
// the module's constraints.
//
// Implements: REQ-TERRAFORM-007, REQ-TERRAFORM-008, REQ-TERRAFORM-009
func (r *resolver) provider(dir, local string) lang.Target {
	req := r.lookup(dir).providers[local]
	if req == nil {
		req = &required{}
	}
	src := providerSource(local)
	if req.source != "" {
		src = providerSource(req.source)
	}
	if src == "terraform.io/builtin/terraform" {
		return lang.Target{}
	}
	constraint := strings.Join(req.constraints, ", ")
	t := lang.Target{Ecosystem: ecoProvider, Package: src, Version: constraint}
	if e, ok := r.lockFor(dir, src); ok {
		t.Version, t.Pinned, t.Requested = e.version, true, constraint
		if t.Requested == "" {
			t.Requested = e.constraints
		}
		return t
	}
	if v, ok := exactVersion(constraint); ok && len(req.constraints) == 1 {
		t.Version, t.Pinned = v, true
	}
	t.Floating = t.Version == ""
	return t
}

// lockFor finds the lock entry that decides a provider's version for a module: the
// module's own lock file, else one of a module calling it, directly or not (a root
// module locks the providers of every module it calls). A lock file in a directory
// above is not taken: that module need not call this one.
func (r *resolver) lockFor(dir, src string) (lockEntry, bool) {
	seen := map[string]bool{dir: true}
	queue := []string{dir}
	for len(queue) > 0 {
		d := queue[0]
		queue = queue[1:]
		if m := r.modules[d]; m != nil && m.lock != nil {
			if e, ok := m.lock[src]; ok && e.version != "" {
				return e, true
			}
		}
		callers := append([]string(nil), r.callers[d]...)
		sort.Strings(callers)
		for _, c := range callers {
			if !seen[c] {
				seen[c] = true
				queue = append(queue, c)
			}
		}
	}
	return lockEntry{}, false
}
