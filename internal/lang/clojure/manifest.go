package clojure

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/edn"
)

// Manifest file names.
const (
	depsEdn    = "deps.edn"
	bbEdn      = "bb.edn"
	shadowEdn  = "shadow-cljs.edn"
	projectClj = "project.clj"
	buildBoot  = "build.boot"
)

var manifestNames = map[string]bool{depsEdn: true, bbEdn: true, shadowEdn: true, projectClj: true, buildBoot: true}

// coord is a dependency's coordinate as a manifest writes it.
type coord struct {
	lib     string // group/artifact as written ("ring" for ring/ring)
	section string // "" for the main dependencies, else the alias or profile
	plugin  bool   // a Leiningen plugin
	version string // :mvn/version, or Leiningen's version string
	local   string // :local/root, relative to the manifest's directory
	gitURL  string // :git/url, or the one the lib name implies (io.github.o/r)
	sha     string // :git/sha
	tag     string // :git/tag
	line    int
}

// manifest is what one Clojure manifest declares.
type manifest struct {
	kind  string   // deps.edn, bb.edn, shadow-cljs.edn, project.clj, build.boot
	name  string   // the project's own lib (Leiningen's defproject)
	paths []string // source roots, relative to the manifest's directory
	deps  []coord
	repos []string // repository URLs (:mvn/repos, :repositories)
	// shadowDeps marks a shadow-cljs.edn whose :deps true hands dependencies and
	// source paths to deps.edn.
	shadowDeps bool
	requires   []*edn.Node // bb.edn task :requires
}

// artifact is Maven's name for a lib: "group:artifact", where a lib without a group
// (Leiningen's [ring "1.9.0"]) is group and artifact of the same name.
//
// Implements: REQ-CLOJURE-008
func artifact(lib string) string {
	g, a, ok := strings.Cut(lib, "/")
	if !ok {
		return lib + ":" + lib
	}
	return g + ":" + a
}

// readManifest reads one manifest by its kind (the file's base name). Nothing is
// evaluated: computed values (~x, #=(...)) are unknown.
//
// Implements: REQ-CLOJURE-004, REQ-CLOJURE-008, REQ-CLOJURE-010
func readManifest(kind string, src []byte) *manifest {
	m := &manifest{kind: kind}
	forms := edn.Read(src)
	switch kind {
	case depsEdn, bbEdn:
		if len(forms) > 0 {
			m.readDeps(forms[0])
		}
		if len(m.paths) == 0 && kind == depsEdn {
			m.paths = []string{"src"} // tools.deps' default
		}
	case shadowEdn:
		if len(forms) > 0 {
			top := forms[0]
			m.paths = top.Get("source-paths").Strings()
			m.leinDeps(top.Get("dependencies"), "")
			if d := top.Get("deps"); d != nil && (d.Kind == edn.Map || d.Kind == edn.Symbol && d.Text == "true") {
				m.shadowDeps = true
			}
			m.leinRepos(top.Get("repositories"))
			if mv := top.Get("maven"); mv != nil {
				m.leinRepos(mv.Get("repositories"))
			}
		}
	case projectClj:
		for _, f := range forms {
			if f.Head() == "defproject" {
				m.readProject(f)
				break
			}
		}
	case buildBoot:
		for _, f := range forms {
			if f.Head() != "set-env!" {
				continue
			}
			for i := 1; i+1 < len(f.Kids); i += 2 {
				k, v := f.Kids[i], edn.Unquote(f.Kids[i+1])
				if k.Kind != edn.Keyword {
					continue
				}
				switch k.Text {
				case "source-paths", "resource-paths":
					m.paths = append(m.paths, v.Strings()...)
				case "dependencies":
					m.leinDeps(v, "")
				case "repositories":
					m.leinRepos(v)
				}
			}
		}
	}
	return m
}

// readDeps reads deps.edn or bb.edn: :paths, :deps, :aliases (:extra-deps,
// :replace-deps, :deps, :override-deps, :extra-paths), :mvn/repos, bb's :tasks
// :requires.
func (m *manifest) readDeps(top *edn.Node) {
	m.paths = top.Get("paths").Strings()
	m.mapDeps(top.Get("deps"), "")
	if al := top.Get("aliases"); al != nil && al.Kind == edn.Map {
		for i := 0; i+1 < len(al.Kids); i += 2 {
			name, body := al.Kids[i], al.Kids[i+1]
			if name.Kind != edn.Keyword || body.Kind != edn.Map {
				continue
			}
			for _, key := range []string{"extra-deps", "replace-deps", "deps", "override-deps", "default-deps"} {
				m.mapDeps(body.Get(key), name.Text)
			}
			m.paths = append(m.paths, body.Get("extra-paths").Strings()...)
		}
	}
	if repos := top.Get("mvn/repos"); repos != nil && repos.Kind == edn.Map {
		for i := 1; i < len(repos.Kids); i += 2 {
			if u := repos.Kids[i].Get("url"); u != nil && u.Kind == edn.String {
				m.repos = append(m.repos, u.Text)
			}
		}
	}
	if tasks := top.Get("tasks"); tasks != nil && tasks.Kind == edn.Map {
		if r := tasks.Get("requires"); r != nil {
			m.requires = append(m.requires, r.Kids...)
		}
		for i := 1; i < len(tasks.Kids); i += 2 {
			if r := tasks.Kids[i].Get("requires"); r != nil {
				m.requires = append(m.requires, r.Kids...)
			}
		}
	}
}

// mapDeps reads a tools.deps dependency map {lib coord}.
func (m *manifest) mapDeps(deps *edn.Node, section string) {
	if deps == nil || deps.Kind != edn.Map {
		return
	}
	for i := 0; i+1 < len(deps.Kids); i += 2 {
		lib, c := deps.Kids[i], deps.Kids[i+1]
		if lib.Kind != edn.Symbol {
			continue
		}
		d := coord{lib: lib.Text, section: section, line: lib.Line}
		str := func(keys ...string) string {
			for _, k := range keys {
				if v := c.Get(k); v != nil && v.Kind == edn.String {
					return v.Text
				}
			}
			return ""
		}
		d.version = str("mvn/version")
		d.local = str("local/root")
		d.gitURL = str("git/url")
		d.sha = str("git/sha", "sha")
		d.tag = str("git/tag", "tag")
		if d.gitURL == "" && (d.sha != "" || d.tag != "") {
			d.gitURL = inferGitURL(lib.Text)
		}
		m.deps = append(m.deps, d)
	}
}

// inferGitURL is the repository tools.deps infers from a git lib's name:
// io.github.o/r is https://github.com/o/r.git (likewise com.github, io.gitlab,
// com.gitlab, io.bitbucket, org.bitbucket, ht.sr for sourcehut).
//
// Implements: REQ-CLOJURE-008
func inferGitURL(lib string) string {
	g, r, ok := strings.Cut(lib, "/")
	if !ok {
		return ""
	}
	for prefix, host := range map[string]string{
		"io.github.": "github.com", "com.github.": "github.com", "io.gitlab.": "gitlab.com",
		"com.gitlab.": "gitlab.com", "io.bitbucket.": "bitbucket.org", "org.bitbucket.": "bitbucket.org",
		"ht.sr.": "git.sr.ht/~",
	} {
		if owner, ok := strings.CutPrefix(g, prefix); ok && owner != "" {
			if strings.HasSuffix(host, "~") {
				return "https://" + host + owner + "/" + r
			}
			return "https://" + host + "/" + owner + "/" + r + ".git"
		}
	}
	return ""
}

// readProject reads (defproject name "version" & options): :dependencies,
// :plugins, :profiles, :managed-dependencies, :repositories, :source-paths,
// :test-paths. Computed values (~x) are skipped.
func (m *manifest) readProject(f *edn.Node) {
	if len(f.Kids) > 1 && f.Kids[1].Kind == edn.Symbol {
		m.name = f.Kids[1].Text
	}
	start := 2 // the version: a string, or computed (~(...), #=(...))
	if len(f.Kids) > 2 && f.Kids[2].Kind != edn.Keyword {
		start = 3
	}
	opts := map[string]*edn.Node{}
	for i := start; i+1 < len(f.Kids); i += 2 {
		if k := f.Kids[i]; k.Kind == edn.Keyword {
			if _, dup := opts[k.Text]; !dup {
				opts[k.Text] = f.Kids[i+1]
			}
		}
	}
	managed := map[string]string{}
	for _, d := range leinVector(opts["managed-dependencies"]) {
		if v := d.Kids[1:]; len(v) > 0 && v[0].Kind == edn.String {
			managed[artifact(d.Kids[0].Text)] = v[0].Text
		}
	}
	m.paths = append(m.paths, opts["source-paths"].Strings()...)
	m.paths = append(m.paths, opts["test-paths"].Strings()...)
	if opts["source-paths"] == nil {
		m.paths = append(m.paths, "src")
	}
	if opts["test-paths"] == nil {
		m.paths = append(m.paths, "test")
	}
	m.leinDeps(opts["dependencies"], "")
	m.leinPlugins(opts["plugins"], "")
	if prof := opts["profiles"]; prof != nil && prof.Kind == edn.Map {
		for i := 0; i+1 < len(prof.Kids); i += 2 {
			name, body := prof.Kids[i], prof.Kids[i+1]
			if name.Kind != edn.Keyword || body.Kind != edn.Map {
				continue
			}
			m.leinDeps(body.Get("dependencies"), name.Text)
			m.leinPlugins(body.Get("plugins"), name.Text)
			m.paths = append(m.paths, body.Get("source-paths").Strings()...)
			m.paths = append(m.paths, body.Get("test-paths").Strings()...)
		}
	}
	for i := range m.deps {
		if d := &m.deps[i]; d.version == "" {
			d.version = managed[artifact(d.lib)]
		}
	}
	m.leinRepos(opts["repositories"])
}

// leinVector lists the [lib "version" & options] entries of a dependency vector.
func leinVector(n *edn.Node) []*edn.Node {
	n = edn.Unquote(n)
	if n == nil || n.Kind != edn.Vector && n.Kind != edn.List {
		return nil
	}
	var out []*edn.Node
	for _, d := range n.Kids {
		if d.Kind == edn.Vector && len(d.Kids) > 0 && d.Kids[0].Kind == edn.Symbol {
			out = append(out, d)
		}
	}
	return out
}

func (m *manifest) leinDeps(n *edn.Node, section string) {
	for _, d := range leinVector(n) {
		c := coord{lib: d.Kids[0].Text, section: section, line: d.Kids[0].Line}
		if len(d.Kids) > 1 && d.Kids[1].Kind == edn.String {
			c.version = d.Kids[1].Text
		}
		m.deps = append(m.deps, c)
	}
}

func (m *manifest) leinPlugins(n *edn.Node, section string) {
	before := len(m.deps)
	m.leinDeps(n, section)
	for i := before; i < len(m.deps); i++ {
		m.deps[i].plugin = true
	}
}

// leinRepos reads Leiningen's :repositories: [["name" "url"]] or [["name" {:url
// "url"}]], or a map of the same.
func (m *manifest) leinRepos(n *edn.Node) {
	n = edn.Unquote(n)
	if n == nil {
		return
	}
	var values []*edn.Node
	switch n.Kind {
	case edn.Map:
		for i := 1; i < len(n.Kids); i += 2 {
			values = append(values, n.Kids[i])
		}
	case edn.Vector, edn.List:
		for _, e := range n.Kids {
			if (e.Kind == edn.Vector || e.Kind == edn.List) && len(e.Kids) == 2 {
				values = append(values, e.Kids[1])
			}
		}
	}
	for _, v := range values {
		if v.Kind == edn.String {
			m.repos = append(m.repos, v.Text)
		} else if u := v.Get("url"); u != nil && u.Kind == edn.String {
			m.repos = append(m.repos, u.Text)
		}
	}
}

// manifestImports makes a manifest's dependencies imports: the lib as written is
// the spec, and each lib once (the main dependencies before aliases and profiles).
func manifestImports(m *manifest) []lang.RawImport {
	var out []lang.RawImport
	seen := map[string]bool{}
	for _, d := range m.deps {
		if seen[d.lib] {
			continue
		}
		seen[d.lib] = true
		kind := kindDep
		if d.plugin {
			kind = kindPlugin
		}
		if d.section != "" {
			kind += ":" + d.section
		}
		out = append(out, lang.RawImport{Spec: d.lib, Module: d.lib, Name: kind, Line: d.line})
	}
	return out
}
