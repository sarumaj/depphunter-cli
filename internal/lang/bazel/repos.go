package bazel

import (
	"cmp"
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/starlark"
)

// A workspace is a Bazel repository of the project: a directory with a
// MODULE.bazel, a WORKSPACE file or a REPO.bazel, and what those declare. The
// project's root is one even without such a file (a sparse checkout).
type workspace struct {
	directory             string
	name                  string                      // module(name) or workspace(name): labels @name//x are local
	repositoryName        string                      // module(repo_name)
	dependencies          map[string]*bazelDependency // bazel_dep by apparent repository name
	byModule              map[string]*bazelDependency // bazel_dep by module name
	over                  map[string]*override
	repositories          map[string]*repositoryDeclaration // WORKSPACE, .bzl and use_repo_rule declarations
	extensionRepositories map[string]string                 // repository use_repo() imports -> the extension's .bzl label
	extensionFile         map[string]string                 // repository use_repo() imports -> the MODULE.bazel importing it
	hubs                  map[string]*hub
	goRepositories        map[string]lang.Target // Gazelle's repository names -> Go modules
	goMods                map[string]lang.Target // Go module paths -> Go modules
	selected              map[string]string      // module -> the version MODULE.bazel.lock selected
	graph                 map[string][]string    // "name@version" -> "name@version" (older lock files)
}

type bazelDependency struct {
	name, version, repositoryName string
}

// override is a *_override() of a module.
type override struct {
	kind                         string // single, git, archive, local
	version                      string
	remote, commit, tag, branch  string
	urls                         []string
	integrity, path, stripPrefix string
}

// repositoryDeclaration is a repository rule's call: http_archive, git_repository,
// local_repository, go_repository and their kin.
type repositoryDeclaration struct {
	rule, name                  string
	urls                        []string
	hash                        string // sha256 or integrity
	remote, commit, tag, branch string
	path                        string // local_repository, relative to the workspace
	importpath, version         string // go_repository
}

// hub is a hub repository a module extension or repository rule creates: one
// repository whose targets are the packages of another ecosystem.
type hub struct {
	ecosystem    string
	packages     map[string]lang.Target // normalized name -> package
	dependencies map[string][]string    // Maven lock: group:artifact -> what it needs
	lock         bool                   // a lock or requirements file listed the packages
	locked       map[string]bool
}

func newWorkspace(directory string) *workspace {
	return &workspace{directory: directory, dependencies: map[string]*bazelDependency{}, byModule: map[string]*bazelDependency{},
		over: map[string]*override{}, repositories: map[string]*repositoryDeclaration{}, extensionRepositories: map[string]string{},
		extensionFile: map[string]string{}, hubs: map[string]*hub{}, goRepositories: map[string]lang.Target{},
		goMods: map[string]lang.Target{}, selected: map[string]string{}, graph: map[string][]string{}}
}

func (w *workspace) hub(name, ecosystem string) *hub {
	h := w.hubs[name]
	if h == nil {
		h = &hub{ecosystem: ecosystem, packages: map[string]lang.Target{}, dependencies: map[string][]string{}, locked: map[string]bool{}}
		w.hubs[name] = h
	}
	return h
}

// labelPath is the project path a main-repository label of a file in this
// workspace names ("" for another repository's).
func (w *workspace) labelPath(s string) string {
	if strings.HasPrefix(s, "@@//") || strings.HasPrefix(s, "@//") {
		s = s[strings.Index(s, "//"):]
	}
	l, ok := parseLabel(s)
	if !ok || l.hasRepository && l.repository != "" && l.repository != w.name && l.repository != w.repositoryName {
		return ""
	}
	return path.Join(w.directory, l.packageName, l.target)
}

// readModule reads a MODULE.bazel (or a file it include()s).
//
// Implements: REQ-BAZEL-006, REQ-BAZEL-008
func (r *resolver) readModule(w *workspace, file string, source []byte, depth int) {
	f := starlark.Parse(source)
	type extension struct{ label, name string }
	extensions := map[string]extension{}
	rules := map[string]string{}
	for _, statement := range f.Statements {
		if statement.Definition != "" || statement.X == nil || statement.X.Kind != starlark.Call {
			continue
		}
		n := statement.X
		callee := n.Callee()
		if statement.Kind == 'a' && len(statement.Targets) == 1 {
			switch callee {
			case "use_extension":
				l, _ := n.Position(0).StringValue()
				name, _ := n.Position(1).StringValue()
				extensions[statement.Targets[0]] = extension{l, name}
			case "use_repo_rule":
				rule, _ := n.Position(1).StringValue()
				rules[statement.Targets[0]] = rule
			}
			continue
		}
		switch callee {
		case "module":
			if w.name == "" {
				w.name, w.repositoryName = n.KeywordString("name"), n.KeywordString("repo_name")
			}
		case "bazel_dep":
			d := &bazelDependency{name: n.KeywordString("name"), version: n.KeywordString("version"), repositoryName: n.KeywordString("repo_name")}
			if d.name == "" {
				continue
			}
			apparent := cmp.Or(d.repositoryName, d.name)
			w.dependencies[apparent] = d
			w.byModule[d.name] = d
		case "single_version_override", "git_override", "archive_override", "local_path_override":
			name := n.KeywordString("module_name")
			if name == "" {
				continue
			}
			o := &override{kind: map[string]string{"single_version_override": "single", "git_override": "git",
				"archive_override": "archive", "local_path_override": "local"}[callee],
				version: n.KeywordString("version"), remote: n.KeywordString("remote"), commit: n.KeywordString("commit"),
				tag: n.KeywordString("tag"), branch: n.KeywordString("branch"), integrity: n.KeywordString("integrity"),
				path: n.KeywordString("path"), stripPrefix: n.KeywordString("strip_prefix"), urls: n.Keyword("urls").Strings()}
			if u := n.KeywordString("url"); u != "" {
				o.urls = append([]string{u}, o.urls...)
			}
			if o.kind == "single" && o.version == "" {
				continue // a registry or patches only: the version stays the selected one
			}
			w.over[name] = o
		case "use_repo":
			e, ok := extensions[n.Position(0).Name()]
			if !ok {
				continue
			}
			for i, a := range n.Arguments {
				if i == 0 || a.Star != "" {
					continue
				}
				s, ok := a.Value.StringValue()
				if !ok {
					continue
				}
				name := s
				if a.Name != "" {
					name = a.Name
				}
				w.extensionRepositories[name] = e.label
				w.extensionFile[name] = file
			}
		case "include":
			if l, ok := n.Position(0).StringValue(); ok && depth < 8 {
				if p := w.labelPath(l); p != "" {
					if source, ok := r.read(p); ok {
						r.readModule(w, p, source, depth+1)
					}
				}
			}
		default:
			if rule, ok := rules[callee]; ok {
				r.repositoryRule(w, rule, n)
				continue
			}
			object, tag, ok := strings.Cut(callee, ".")
			if e, isExtension := extensions[object]; ok && isExtension {
				r.extensionTag(w, e.name, tag, n)
			}
		}
	}
}

// extensionTag reads a tag of the module extensions whose hub repositories name
// packages of other ecosystems.
//
// Implements: REQ-BAZEL-009
func (r *resolver) extensionTag(w *workspace, extension, tag string, n *starlark.Node) {
	switch extension + "." + tag {
	case "maven.install", "maven.artifact":
		name := cmp.Or(n.KeywordString("name"), "maven")
		h := w.hub(name, lang.EcosystemMaven)
		if tag == "artifact" {
			if c := coordinate(n); c != "" {
				h.addMaven(c)
			}
			return
		}
		r.mavenInstall(w, h, n, "lock_file")
	case "pip.parse":
		name := n.KeywordString("hub_name")
		if name == "" {
			return
		}
		r.pipParse(w, w.hub(name, lang.EcosystemPyPI), n)
	case "go_deps.from_file":
		if p := w.labelPath(n.KeywordString("go_mod")); p != "" {
			if source, ok := r.read(p); ok {
				for _, t := range readGoMod(source) {
					w.addGo(t)
				}
			}
		}
	case "go_deps.module":
		if p := n.KeywordString("path"); p != "" {
			v := n.KeywordString("version")
			w.addGo(lang.Target{Ecosystem: lang.EcosystemGo, Package: p, Version: v, Pinned: lang.Pinned(v)})
		}
	case "npm.npm_translate_lock":
		name := cmp.Or(n.KeywordString("name"), "npm")
		r.pnpm(w, w.hub(name, lang.EcosystemNPM), n.KeywordString("pnpm_lock"))
	case "crate.from_cargo", "crate.from_specs":
		name := cmp.Or(n.KeywordString("name"), "crates")
		r.cargo(w, w.hub(name, lang.EcosystemCrates), n)
	case "crate.spec":
		name := cmp.Or(n.KeywordString("repositories"), "crates")
		if p := n.KeywordString("package"); p != "" {
			v := n.KeywordString("version")
			w.hub(name, lang.EcosystemCrates).add(normalizeCrate(p), lang.Target{Ecosystem: lang.EcosystemCrates, Package: p, Version: v, Pinned: exactCargo(v)}, false)
		}
	}
}

// readRules reads the repository rules a WORKSPACE or .bzl file calls, at any
// depth: .bzl macros (grpc_deps(), rules_go_dependencies()) declare most of a
// WORKSPACE project's repositories. A name declared already keeps its first
// declaration: WORKSPACE files are read before .bzl files.
//
// Implements: REQ-BAZEL-007
func (r *resolver) readRules(w *workspace, source []byte, isWorkspace bool) {
	f := starlark.Parse(source)
	loads := map[string]string{}
	for _, statement := range f.Statements {
		if statement.Definition == "" && statement.X.Callee() == "load" {
			for i, a := range statement.X.Arguments {
				if s, ok := a.Value.StringValue(); ok && i > 0 {
					local := cmp.Or(a.Name, s)
					loads[local] = s
				}
			}
		}
	}
	original := func(callee string) string {
		if o, ok := loads[callee]; ok {
			return o
		}
		return strings.TrimPrefix(callee, "native.")
	}
	for _, statement := range f.Statements {
		if statement.X == nil {
			continue
		}
		if isWorkspace && statement.Definition == "" && statement.X.Callee() == "workspace" && w.name == "" {
			w.name = statement.X.KeywordString("name")
		}
		walk(statement.X, func(n *starlark.Node) {
			if n.Kind != starlark.Call {
				return
			}
			rule := original(n.Callee())
			if rule == "maybe" {
				rule = original(n.Position(0).Name())
			}
			if repositoryRules[rule] || hubRules[rule] {
				r.repositoryRule(w, rule, n)
			}
		})
	}
}

// repositoryRule records one repository rule call.
func (r *resolver) repositoryRule(w *workspace, rule string, n *starlark.Node) {
	name := n.KeywordString("name")
	switch rule {
	case "maven_install":
		name = cmp.Or(name, "maven")
		r.mavenInstall(w, w.hub(name, lang.EcosystemMaven), n, "maven_install_json")
		return
	case "pip_parse", "pip_install":
		name = cmp.Or(name, "pip")
		r.pipParse(w, w.hub(name, lang.EcosystemPyPI), n)
		return
	case "npm_translate_lock":
		name = cmp.Or(name, "npm")
		r.pnpm(w, w.hub(name, lang.EcosystemNPM), n.KeywordString("pnpm_lock"))
		return
	case "crates_repository":
		name = cmp.Or(name, "crates")
		r.cargo(w, w.hub(name, lang.EcosystemCrates), n)
		return
	}
	if name == "" || w.repositories[name] != nil {
		return
	}
	d := &repositoryDeclaration{rule: rule, name: name, urls: n.Keyword("urls").Strings(), remote: n.KeywordString("remote"),
		commit: n.KeywordString("commit"), tag: n.KeywordString("tag"), branch: n.KeywordString("branch"), path: n.KeywordString("path"),
		importpath: n.KeywordString("importpath"), version: n.KeywordString("version")}
	if u := n.KeywordString("url"); u != "" {
		d.urls = append([]string{u}, d.urls...)
	}
	if d.hash = n.KeywordString("sha256"); d.hash == "" {
		d.hash = n.KeywordString("integrity")
	}
	w.repositories[name] = d
	if rule == "go_repository" && d.importpath != "" {
		t := r.repositoryTarget(w, d)
		w.goRepositories[name] = t
		if _, ok := w.goMods[d.importpath]; !ok {
			w.goMods[d.importpath] = t
		}
	}
}

// addGo records a Go module under its path and under the repository name Gazelle
// gives it.
func (w *workspace) addGo(t lang.Target) {
	if _, ok := w.goMods[t.Package]; !ok {
		w.goMods[t.Package] = t
	}
	if name := goRepositoryName(t.Package); w.goRepositories[name].Package == "" {
		w.goRepositories[name] = t
	}
}

// goRepositoryName is the repository name Gazelle derives from a Go import path:
// github.com/pkg/errors -> com_github_pkg_errors.
func goRepositoryName(importpath string) string {
	parts := strings.Split(strings.ToLower(importpath), "/")
	host := strings.Split(parts[0], ".")
	for i, j := 0, len(host)-1; i < j; i, j = i+1, j-1 {
		host[i], host[j] = host[j], host[i]
	}
	name := strings.Join(append(host, parts[1:]...), ".")
	return strings.NewReplacer("-", "_", ".", "_").Replace(name)
}

// add records a package of the hub. What a lock file says wins over what a
// manifest declares; the declared version then becomes Requested.
func (h *hub) add(key string, t lang.Target, fromLock bool) {
	old, ok := h.packages[key]
	switch {
	case !ok:
	case fromLock && !h.locked[key]:
		if old.Version != "" && old.Version != t.Version {
			t.Requested = old.Version
		}
	case !fromLock && h.locked[key]:
		if t.Version != "" && t.Version != old.Version && old.Requested == "" {
			old.Requested = t.Version
			h.packages[key] = old
		}
		return
	default:
		return // the first declaration stands
	}
	if fromLock {
		h.lock = true
		h.locked[key] = true
	}
	h.packages[key] = t
}
