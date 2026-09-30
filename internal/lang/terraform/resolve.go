package terraform

import (
	"cmp"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// moduleDirectory is one Terraform module: a directory's configuration files read together.
type moduleDirectory struct {
	declaration map[string]string // symbol name -> first file declaring it
	providers   map[string]*required
	calls       []string // directories of the local modules it calls
	lock        map[string]lockEntry
	installed   []installedModule // .terraform/modules/modules.json, sorted by key
}

// required is what a module says about one provider local name, across its files.
type required struct {
	source      string
	constraints []string
}

type resolver struct {
	lang.Layout
	modules    map[string]*moduleDirectory
	callers    map[string][]string  // module directory -> directories calling it
	terragrunt map[string]*tgConfig // Terragrunt configuration -> its locals and includes
}

func directoryOf(p string) string { return path.Dir(p) }

// newResolver reads every configuration file of the repository once more (the
// scanner is fast) to learn what each module declares and which providers it
// requires, every lock file for the versions it pins, and what `terraform
// init` installed for each module.
//
// Implements: REQ-TERRAFORM-005, REQ-TERRAFORM-007, REQ-TERRAFORM-008
func newResolver(root string, all []*scan.File) *resolver {
	r := &resolver{Layout: lang.NewLayout(), modules: map[string]*moduleDirectory{}, callers: map[string][]string{},
		terragrunt: map[string]*tgConfig{}}
	var configs []*scan.File
	for _, f := range all {
		r.Add(f.Path)
		if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize || ignored(f.Path) {
			continue
		}
		switch fileClass(f.Path) {
		case classConfig, classJSON, classLock, classTerragrunt:
			configs = append(configs, f)
		}
	}
	if len(r.Files) > 0 {
		r.Directories["."] = true // the top level is a directory here, as soon as it holds a file
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].Path < configs[j].Path })
	for _, f := range configs {
		source, err := os.ReadFile(f.AbsolutePath)
		if err != nil {
			continue
		}
		class := fileClass(f.Path)
		if class == classTerragrunt {
			r.terragrunt[f.Path] = readTerragrunt(parse(source)).terragrunt
			continue
		}
		m := r.module(directoryOf(f.Path))
		if class == classLock {
			m.lock = map[string]lockEntry{}
			for _, e := range lockEntries(parse(source)) {
				m.lock[providerSource(e.address)] = e
			}
			continue
		}
		fileInfo := read(class, source)
		for _, s := range fileInfo.symbols.List() {
			if _, ok := m.declaration[s.Name]; !ok {
				m.declaration[s.Name] = f.Path
			}
		}
		for _, local := range lang.SortedKeys(fileInfo.providers) {
			requirement := fileInfo.providers[local]
			have := m.providers[local]
			if have == nil {
				have = &required{}
				m.providers[local] = have
			}
			if have.source == "" && requirement.source != "" {
				have.source = requirement.source
			}
			if requirement.constraint != "" && !slices.Contains(have.constraints, requirement.constraint) {
				have.constraints = append(have.constraints, requirement.constraint)
			}
		}
		for _, rawImport := range fileInfo.imports {
			if strings.HasPrefix(rawImport.Name, kindModule+"|") {
				if s, ok := parseSource(rawImport.Module); ok && s.local != "" {
					callee := path.Clean(path.Join(directoryOf(f.Path), s.local))
					m.calls = append(m.calls, callee)
					r.callers[callee] = append(r.callers[callee], directoryOf(f.Path))
				}
			}
		}
	}
	for d, m := range r.modules {
		m.installed = readInstalled(root, d)
	}
	return r
}

func (r *resolver) module(directory string) *moduleDirectory {
	m := r.modules[directory]
	if m == nil {
		m = &moduleDirectory{declaration: map[string]string{}, providers: map[string]*required{}}
		r.modules[directory] = m
	}
	return m
}

// lookup is module for Resolve, which runs concurrently and must not add one.
func (r *resolver) lookup(directory string) *moduleDirectory {
	if m := r.modules[directory]; m != nil {
		return m
	}
	return &moduleDirectory{}
}

// Implements: REQ-TERRAFORM-004, REQ-TERRAFORM-005, REQ-TERRAFORM-006, REQ-TERRAFORM-007, REQ-TERRAFORM-008, REQ-TERRAFORM-009, REQ-TERRAFORM-010
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	directory := directoryOf(file)
	kind, argument, _ := strings.Cut(rawImport.Name, "|")
	switch kind {
	case kindModule:
		return r.installedPin(directory, callName(rawImport.Spec), r.moduleTarget(directory, rawImport.Module, argument))
	case kindTGSource:
		source := r.expandIncludes(file, rawImport.Module)
		if strings.Contains(source, "${") {
			return lang.Target{}
		}
		return r.moduleTarget(directory, source, "")
	case kindProvider:
		return r.provider(directory, rawImport.Module)
	case kindLock:
		source := providerSource(rawImport.Module)
		t := lang.Target{Ecosystem: ecosystemProvider, Package: source}
		if e := r.lookup(directory).lock[source]; e.version != "" {
			t.Version, t.Pinned, t.Requested = e.version, true, e.constraints
		}
		return t
	case kindReference:
		if f, ok := r.lookup(directory).declaration[rawImport.Module]; ok && f != file {
			return lang.Target{Local: f}
		}
		return lang.Target{}
	case kindFile:
		return r.local(path.Join(directory, rawImport.Module))
	case kindTGDependency:
		p := path.Clean(path.Join(directory, rawImport.Module))
		if r.Files[path.Join(p, "terragrunt.hcl")] {
			return lang.Target{Local: path.Join(p, "terragrunt.hcl")}
		}
		return r.local(p)
	case kindTGParent:
		if f := r.parentFile(directory, rawImport.Module); f != "" {
			if argument == "" {
				return lang.Target{Local: f}
			}
			return r.local(path.Join(path.Dir(f), argument))
		}
	}
	return lang.Target{}
}

// parentFile is what find_in_parent_folders(name) finds for a configuration in directory:
// the nearest file of that name in a directory above it.
func (r *resolver) parentFile(directory, name string) string {
	for d := range lang.Ancestors(directory) {
		if f := path.Join(d, name); r.Files[f] {
			return f
		}
	}
	return ""
}

var includeLocal = regexp.MustCompile(`\$\{\s*include\.([A-Za-z_][A-Za-z0-9_-]*)\.locals\.([A-Za-z_][A-Za-z0-9_-]*)\s*\}`)

// expandIncludes substitutes the locals of included files into a Terragrunt source:
// "${include.envcommon.locals.base_source_url}?ref=v0.8.0", with the include's path
// found as Terragrunt finds it and the local a string in that file.
//
// Implements: REQ-TERRAFORM-010
func (r *resolver) expandIncludes(file, s string) string {
	config := r.terragrunt[file]
	if config == nil {
		return s
	}
	return includeLocal.ReplaceAllStringFunc(s, func(m string) string {
		groups := includeLocal.FindStringSubmatch(m)
		include, ok := config.includes[groups[1]]
		if !ok {
			return m
		}
		target := ""
		if rest, ok := strings.CutPrefix(include, "parent:"); ok {
			name, tail, _ := strings.Cut(rest, "|")
			if f := r.parentFile(directoryOf(file), name); f != "" {
				target = path.Clean(path.Join(path.Dir(f), tail))
				if tail == "" {
					target = f
				}
			}
		} else if include != "" {
			target = path.Clean(path.Join(directoryOf(file), include))
		}
		if other := r.terragrunt[target]; other != nil {
			if v, ok := other.locals[groups[2]]; ok && !strings.Contains(v, "${") {
				return v
			}
		}
		return m
	})
}

// local is a file or directory of the repository, else nothing.
func (r *resolver) local(p string) lang.Target {
	p = path.Clean(p)
	if !lang.Inside(p) {
		return lang.Target{}
	}
	if r.Has(p) {
		return lang.Target{Local: p}
	}
	return lang.Target{}
}

// moduleTarget resolves a module source: a local module's directory, a registry
// module pinned by an exact version, or a remote module pinned by a commit.
//
// Implements: REQ-TERRAFORM-004, REQ-TERRAFORM-009
func (r *resolver) moduleTarget(directory, source, version string) lang.Target {
	s, ok := parseSource(source)
	switch {
	case !ok:
		// Terraform before 0.12 took a bare directory name ("subnets") as a path.
		if !strings.ContainsAny(source, ":?") {
			return r.local(path.Join(directory, source))
		}
		return lang.Target{}
	case s.local != "":
		return r.local(path.Join(directory, s.local))
	case s.origin != "":
		// A tag names a release but can be moved; only a commit pins, and without a
		// reference the default branch floats (the rule of GitHub Actions, REQ-CI-011).
		t := lang.Target{Ecosystem: ecosystemModule, Package: s.packageName, Version: s.reference, Origin: s.origin,
			Pinned: lang.Commit(s.reference), Floating: s.reference == ""}
		if s.archive && s.reference == "" {
			t.Floating = true
		}
		return t
	}
	version = cmp.Or(version, s.reference) // tfr://...?version=
	t := lang.Target{Ecosystem: ecosystemModule, Package: s.packageName, Version: strings.TrimSpace(version)}
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
func (r *resolver) provider(directory, local string) lang.Target {
	requiredProvider := r.lookup(directory).providers[local]
	if requiredProvider == nil {
		requiredProvider = &required{}
	}
	source := providerSource(local)
	if requiredProvider.source != "" {
		source = providerSource(requiredProvider.source)
	}
	if source == "terraform.io/builtin/terraform" {
		return lang.Target{}
	}
	constraint := strings.Join(requiredProvider.constraints, ", ")
	t := lang.Target{Ecosystem: ecosystemProvider, Package: source, Version: constraint}
	if e, ok := r.lockFor(directory, source); ok {
		t.Version, t.Pinned, t.Requested = e.version, true, constraint
		t.Requested = cmp.Or(t.Requested, e.constraints)
		return t
	}
	if v, ok := exactVersion(constraint); ok && len(requiredProvider.constraints) == 1 {
		t.Version, t.Pinned = v, true
	}
	t.Floating = t.Version == ""
	return t
}

// lockFor finds the lock entry that decides a provider's version for a module: the
// module's own lock file, else one of a module calling it, directly or not (a root
// module locks the providers of every module it calls). A lock file in a directory
// above is not taken: that module need not call this one.
func (r *resolver) lockFor(directory, source string) (e lockEntry, ok bool) {
	r.callersOf(directory, func(d string) bool {
		if m := r.modules[d]; m != nil && m.lock != nil {
			if x, found := m.lock[source]; found && x.version != "" {
				e, ok = x, true
			}
		}
		return ok
	})
	return e, ok
}

// callersOf visits directory, then the modules calling it, directly or not, breadth
// first, until visit reports it is done.
func (r *resolver) callersOf(directory string, visit func(string) bool) {
	seen := map[string]bool{directory: true}
	queue := []string{directory}
	for len(queue) > 0 {
		d := queue[0]
		queue = queue[1:]
		if visit(d) {
			return
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
}
