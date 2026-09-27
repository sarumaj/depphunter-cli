package bazel

import (
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/starlark"
)

// A workspace is a Bazel repository of the project: a directory with a
// MODULE.bazel, a WORKSPACE file or a REPO.bazel, and what those declare. The
// project's root is one even without such a file (a sparse checkout).
type workspace struct {
	dir      string
	name     string               // module(name) or workspace(name): labels @name//x are local
	repoName string               // module(repo_name)
	deps     map[string]*bazelDep // bazel_dep by apparent repository name
	byModule map[string]*bazelDep // bazel_dep by module name
	over     map[string]*override
	repos    map[string]*repoDecl // WORKSPACE, .bzl and use_repo_rule declarations
	extRepos map[string]string    // repository use_repo() imports -> the extension's .bzl label
	extFile  map[string]string    // repository use_repo() imports -> the MODULE.bazel importing it
	hubs     map[string]*hub
	goRepos  map[string]lang.Target // Gazelle's repository names -> Go modules
	goMods   map[string]lang.Target // Go module paths -> Go modules
	selected map[string]string      // module -> the version MODULE.bazel.lock selected
	graph    map[string][]string    // "name@version" -> "name@version" (older lock files)
}

type bazelDep struct {
	name, version, repoName string
}

// override is a *_override() of a module.
type override struct {
	kind                         string // single, git, archive, local
	version                      string
	remote, commit, tag, branch  string
	urls                         []string
	integrity, path, stripPrefix string
}

// repoDecl is a repository rule's call: http_archive, git_repository,
// local_repository, go_repository and their kin.
type repoDecl struct {
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
	eco    string
	pkgs   map[string]lang.Target // normalized name -> package
	deps   map[string][]string    // Maven lock: group:artifact -> what it needs
	lock   bool                   // a lock or requirements file listed the packages
	locked map[string]bool
}

func newWorkspace(dir string) *workspace {
	return &workspace{dir: dir, deps: map[string]*bazelDep{}, byModule: map[string]*bazelDep{},
		over: map[string]*override{}, repos: map[string]*repoDecl{}, extRepos: map[string]string{},
		extFile: map[string]string{}, hubs: map[string]*hub{}, goRepos: map[string]lang.Target{},
		goMods: map[string]lang.Target{}, selected: map[string]string{}, graph: map[string][]string{}}
}

func (w *workspace) hub(name, eco string) *hub {
	h := w.hubs[name]
	if h == nil {
		h = &hub{eco: eco, pkgs: map[string]lang.Target{}, deps: map[string][]string{}, locked: map[string]bool{}}
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
	if !ok || l.hasRepo && l.repo != "" && l.repo != w.name && l.repo != w.repoName {
		return ""
	}
	return path.Join(w.dir, l.pkg, l.target)
}

// readModule reads a MODULE.bazel (or a file it include()s).
//
// Implements: REQ-BAZEL-006, REQ-BAZEL-008
func (r *resolver) readModule(w *workspace, file string, src []byte, depth int) {
	f := starlark.Parse(src)
	type ext struct{ label, name string }
	exts := map[string]ext{}
	rules := map[string]string{}
	for _, st := range f.Stmts {
		if st.Def != "" || st.X == nil || st.X.Kind != starlark.Call {
			continue
		}
		n := st.X
		callee := n.Callee()
		if st.Kind == 'a' && len(st.Targets) == 1 {
			switch callee {
			case "use_extension":
				l, _ := n.Pos(0).Str()
				name, _ := n.Pos(1).Str()
				exts[st.Targets[0]] = ext{l, name}
			case "use_repo_rule":
				rule, _ := n.Pos(1).Str()
				rules[st.Targets[0]] = rule
			}
			continue
		}
		switch callee {
		case "module":
			if w.name == "" {
				w.name, w.repoName = n.KwStr("name"), n.KwStr("repo_name")
			}
		case "bazel_dep":
			d := &bazelDep{name: n.KwStr("name"), version: n.KwStr("version"), repoName: n.KwStr("repo_name")}
			if d.name == "" {
				continue
			}
			apparent := d.repoName
			if apparent == "" {
				apparent = d.name
			}
			w.deps[apparent] = d
			w.byModule[d.name] = d
		case "single_version_override", "git_override", "archive_override", "local_path_override":
			name := n.KwStr("module_name")
			if name == "" {
				continue
			}
			o := &override{kind: map[string]string{"single_version_override": "single", "git_override": "git",
				"archive_override": "archive", "local_path_override": "local"}[callee],
				version: n.KwStr("version"), remote: n.KwStr("remote"), commit: n.KwStr("commit"),
				tag: n.KwStr("tag"), branch: n.KwStr("branch"), integrity: n.KwStr("integrity"),
				path: n.KwStr("path"), stripPrefix: n.KwStr("strip_prefix"), urls: n.Kw("urls").Strings()}
			if u := n.KwStr("url"); u != "" {
				o.urls = append([]string{u}, o.urls...)
			}
			if o.kind == "single" && o.version == "" {
				continue // a registry or patches only: the version stays the selected one
			}
			w.over[name] = o
		case "use_repo":
			e, ok := exts[n.Pos(0).Name()]
			if !ok {
				continue
			}
			for i, a := range n.Args {
				if i == 0 || a.Star != "" {
					continue
				}
				s, ok := a.Val.Str()
				if !ok {
					continue
				}
				name := s
				if a.Name != "" {
					name = a.Name
				}
				w.extRepos[name] = e.label
				w.extFile[name] = file
			}
		case "include":
			if l, ok := n.Pos(0).Str(); ok && depth < 8 {
				if p := w.labelPath(l); p != "" {
					if src, ok := r.read(p); ok {
						r.readModule(w, p, src, depth+1)
					}
				}
			}
		default:
			if rule, ok := rules[callee]; ok {
				r.repoRule(w, rule, n)
				continue
			}
			obj, tag, ok := strings.Cut(callee, ".")
			if e, isExt := exts[obj]; ok && isExt {
				r.extTag(w, e.name, tag, n)
			}
		}
	}
}

// extTag reads a tag of the module extensions whose hub repositories name
// packages of other ecosystems.
//
// Implements: REQ-BAZEL-009
func (r *resolver) extTag(w *workspace, ext, tag string, n *starlark.Node) {
	switch ext + "." + tag {
	case "maven.install", "maven.artifact":
		name := n.KwStr("name")
		if name == "" {
			name = "maven"
		}
		h := w.hub(name, ecoMaven)
		if tag == "artifact" {
			if c := coordinate(n); c != "" {
				h.addMaven(c)
			}
			return
		}
		r.mavenInstall(w, h, n, "lock_file")
	case "pip.parse":
		name := n.KwStr("hub_name")
		if name == "" {
			return
		}
		r.pipParse(w, w.hub(name, ecoPyPI), n)
	case "go_deps.from_file":
		if p := w.labelPath(n.KwStr("go_mod")); p != "" {
			if src, ok := r.read(p); ok {
				for _, t := range readGoMod(src) {
					w.addGo(t)
				}
			}
		}
	case "go_deps.module":
		if p := n.KwStr("path"); p != "" {
			v := n.KwStr("version")
			w.addGo(lang.Target{Ecosystem: ecoGo, Package: p, Version: v, Pinned: lang.Pinned(v)})
		}
	case "npm.npm_translate_lock":
		name := n.KwStr("name")
		if name == "" {
			name = "npm"
		}
		r.pnpm(w, w.hub(name, ecoNPM), n.KwStr("pnpm_lock"))
	case "crate.from_cargo", "crate.from_specs":
		name := n.KwStr("name")
		if name == "" {
			name = "crates"
		}
		r.cargo(w, w.hub(name, ecoCrates), n)
	case "crate.spec":
		name := n.KwStr("repositories")
		if name == "" {
			name = "crates"
		}
		if p := n.KwStr("package"); p != "" {
			v := n.KwStr("version")
			w.hub(name, ecoCrates).add(normCrate(p), lang.Target{Ecosystem: ecoCrates, Package: p, Version: v, Pinned: exactCargo(v)}, false)
		}
	}
}

// readRules reads the repository rules a WORKSPACE or .bzl file calls, at any
// depth: .bzl macros (grpc_deps(), rules_go_dependencies()) declare most of a
// WORKSPACE project's repositories. A name declared already keeps its first
// declaration: WORKSPACE files are read before .bzl files.
//
// Implements: REQ-BAZEL-007
func (r *resolver) readRules(w *workspace, src []byte, isWorkspace bool) {
	f := starlark.Parse(src)
	loads := map[string]string{}
	for _, st := range f.Stmts {
		if st.Def == "" && st.X.Callee() == "load" {
			for i, a := range st.X.Args {
				if s, ok := a.Val.Str(); ok && i > 0 {
					local := a.Name
					if local == "" {
						local = s
					}
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
	for _, st := range f.Stmts {
		if st.X == nil {
			continue
		}
		if isWorkspace && st.Def == "" && st.X.Callee() == "workspace" && w.name == "" {
			w.name = st.X.KwStr("name")
		}
		walk(st.X, func(n *starlark.Node) {
			if n.Kind != starlark.Call {
				return
			}
			rule := original(n.Callee())
			if rule == "maybe" {
				rule = original(n.Pos(0).Name())
			}
			if repoRules[rule] || hubRules[rule] {
				r.repoRule(w, rule, n)
			}
		})
	}
}

// repoRule records one repository rule call.
func (r *resolver) repoRule(w *workspace, rule string, n *starlark.Node) {
	name := n.KwStr("name")
	switch rule {
	case "maven_install":
		if name == "" {
			name = "maven"
		}
		r.mavenInstall(w, w.hub(name, ecoMaven), n, "maven_install_json")
		return
	case "pip_parse", "pip_install":
		if name == "" {
			name = "pip"
		}
		r.pipParse(w, w.hub(name, ecoPyPI), n)
		return
	case "npm_translate_lock":
		if name == "" {
			name = "npm"
		}
		r.pnpm(w, w.hub(name, ecoNPM), n.KwStr("pnpm_lock"))
		return
	case "crates_repository":
		if name == "" {
			name = "crates"
		}
		r.cargo(w, w.hub(name, ecoCrates), n)
		return
	}
	if name == "" || w.repos[name] != nil {
		return
	}
	d := &repoDecl{rule: rule, name: name, urls: n.Kw("urls").Strings(), remote: n.KwStr("remote"),
		commit: n.KwStr("commit"), tag: n.KwStr("tag"), branch: n.KwStr("branch"), path: n.KwStr("path"),
		importpath: n.KwStr("importpath"), version: n.KwStr("version")}
	if u := n.KwStr("url"); u != "" {
		d.urls = append([]string{u}, d.urls...)
	}
	if d.hash = n.KwStr("sha256"); d.hash == "" {
		d.hash = n.KwStr("integrity")
	}
	w.repos[name] = d
	if rule == "go_repository" && d.importpath != "" {
		t := r.repoTarget(w, d)
		w.goRepos[name] = t
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
	if name := goRepoName(t.Package); w.goRepos[name].Package == "" {
		w.goRepos[name] = t
	}
}

// goRepoName is the repository name Gazelle derives from a Go import path:
// github.com/pkg/errors -> com_github_pkg_errors.
func goRepoName(importpath string) string {
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
	old, ok := h.pkgs[key]
	switch {
	case !ok:
	case fromLock && !h.locked[key]:
		if old.Version != "" && old.Version != t.Version {
			t.Requested = old.Version
		}
	case !fromLock && h.locked[key]:
		if t.Version != "" && t.Version != old.Version && old.Requested == "" {
			old.Requested = t.Version
			h.pkgs[key] = old
		}
		return
	default:
		return // the first declaration stands
	}
	if fromLock {
		h.lock = true
		h.locked[key] = true
	}
	h.pkgs[key] = t
}

// sortedKeys lists a map's keys in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
