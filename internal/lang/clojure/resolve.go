package clojure

import (
	"encoding/json"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/edn"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// project is what the manifests of one directory declare together: a deps.edn beside
// a shadow-cljs.edn, a bb.edn and a package.json is one project.
type project struct {
	dir     string   // "." for the repository's root
	files   []string // its manifests, sorted
	name    string   // its own artifact (Leiningen's defproject)
	roots   []string // source paths, repository-relative
	deps    map[string]coord
	order   []string          // artifacts in declaration order
	npm     map[string]string // package.json and deps.cljs npm dependencies
	bb      bool              // a bb.edn is among its manifests
	bbRoots []string          // bb.edn's :paths, repository-relative
	other   bool              // a manifest other than bb.edn is
	local   []string          // directories of its :local/root dependencies
}

type resolver struct {
	files    map[string]bool
	projects map[string]*project
	dirs     []string                    // project directories, shallowest first
	byNS     map[string][]string         // namespace -> the files declaring it
	byJava   map[string][]string         // "Name.java" -> project paths
	coords   map[string]map[string]coord // manifest path -> lib -> its coordinate
	roots    map[string]bool             // first segments of the project's namespaces
	named    map[string]*project         // own artifact -> project (defproject names)
	gov      map[string][]*project       // directory -> its governing projects
	byStem   map[string][]string         // source path without its extension -> sources
	memo     sync.Map                    // resolutions shared by the files of a directory
	classes  sync.Map                    // governing project dirs -> *classMatcher
}

// classMatcher is the Java plugin's artifact matcher over the Maven artifacts a set
// of governing projects declares, built on first use.
type classMatcher struct {
	once sync.Once
	arts *java.Artifacts
}

func newResolver(_ string, all []*scan.File) *resolver {
	r := &resolver{
		files: map[string]bool{}, projects: map[string]*project{}, byNS: map[string][]string{},
		byJava: map[string][]string{}, coords: map[string]map[string]coord{}, roots: map[string]bool{},
		byStem: map[string][]string{},
	}
	proj := func(dir string) *project {
		p := r.projects[dir]
		if p == nil {
			p = &project{dir: dir, deps: map[string]coord{}, npm: map[string]string{}}
			r.projects[dir] = p
		}
		return p
	}
	readable := func(f *scan.File) bool { return !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize }
	for _, f := range all {
		r.files[f.Path] = true
		base, dir := path.Base(f.Path), path.Dir(f.Path)
		switch {
		case !readable(f) || skipped(f.Path):
		case manifestNames[base]:
			src, err := os.ReadFile(f.Abs)
			if err != nil {
				continue
			}
			r.addManifest(proj(dir), f.Path, readManifest(base, src))
		case base == "package.json" || base == "deps.cljs":
			if src, err := os.ReadFile(f.Abs); err == nil {
				readNPM(proj(dir), base, src)
			}
		case sourceExts[path.Ext(base)] || f.Interpreter == "bb" && scan.Language(f.Path) == "":
			if src, err := os.ReadFile(f.Abs); err == nil {
				stem := strings.TrimSuffix(f.Path, path.Ext(f.Path))
				r.byStem[stem] = append(r.byStem[stem], f.Path)
				if ns := nsName(src); ns != "" {
					r.byNS[ns] = append(r.byNS[ns], f.Path)
					seg, _, _ := strings.Cut(ns, ".")
					r.roots[seg] = true
				}
			}
		case strings.HasSuffix(base, ".java"):
			r.byJava[base] = append(r.byJava[base], f.Path)
		}
	}
	for dir, p := range r.projects {
		// A directory with only a package.json is no Clojure project.
		if len(p.files) == 0 {
			delete(r.projects, dir)
			continue
		}
		r.dirs = append(r.dirs, dir)
		sort.Strings(p.files)
	}
	sort.Slice(r.dirs, func(i, j int) bool {
		a, b := r.dirs[i], r.dirs[j]
		if da, db := depth(a), depth(b); da != db {
			return da < db
		}
		return a < b
	})
	for _, files := range r.byNS {
		sort.Strings(files)
	}
	r.gov = map[string][]*project{}
	for _, f := range all {
		if d := path.Dir(f.Path); r.gov[d] == nil {
			r.gov[d] = r.governingDir(d)
		}
	}
	r.named = map[string]*project{}
	for _, d := range r.dirs {
		if p := r.projects[d]; p.name != "" && r.named[p.name] == nil {
			r.named[p.name] = p
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

func (r *resolver) addManifest(p *project, file string, m *manifest) {
	p.files = append(p.files, file)
	if m.kind == bbEdn {
		p.bb = true
		for _, root := range m.paths {
			p.bbRoots = append(p.bbRoots, path.Join(p.dir, root))
		}
	} else {
		p.other = true
	}
	if m.name != "" && p.name == "" {
		p.name = artifact(m.name)
	}
	for _, root := range m.paths {
		root = path.Join(p.dir, root)
		if !strings.HasPrefix(root, "../") && root != ".." && !contains(p.roots, root) {
			p.roots = append(p.roots, root)
		}
	}
	libs := map[string]coord{}
	for _, d := range m.deps {
		if _, ok := libs[d.lib]; !ok {
			libs[d.lib] = d
		}
		a := artifact(d.lib)
		if _, ok := p.deps[a]; ok || d.plugin {
			continue // a plugin extends the build tool, not the code's classpath
		}
		p.deps[a] = d
		p.order = append(p.order, a)
		if d.local != "" {
			p.local = append(p.local, path.Join(p.dir, d.local))
		}
	}
	r.coords[file] = libs
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// readNPM reads the npm dependencies ClojureScript builds install: package.json's
// dependencies and devDependencies, a library's deps.cljs :npm-deps.
func readNPM(p *project, base string, src []byte) {
	if base == "deps.cljs" {
		forms := edn.Read(src)
		if len(forms) > 0 {
			if deps := forms[0].Get("npm-deps"); deps != nil && deps.Kind == edn.Map {
				for i := 0; i+1 < len(deps.Kids); i += 2 {
					k, v := deps.Kids[i], deps.Kids[i+1]
					if (k.Kind == edn.String || k.Kind == edn.Symbol || k.Kind == edn.Keyword) && v.Kind == edn.String {
						if _, ok := p.npm[k.Text]; !ok {
							p.npm[k.Text] = v.Text
						}
					}
				}
			}
		}
		return
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(src, &pkg) != nil {
		return
	}
	for _, m := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
		for k, v := range m {
			if _, ok := p.npm[k]; !ok {
				p.npm[k] = v
			}
		}
	}
}

// governing lists the projects whose dependencies a file sees: those of the
// directories above it, nearest first, and the projects their :local/root
// dependencies name. A file under no manifest sees every project, shallowest first.
//
// Implements: REQ-CLOJURE-004
func (r *resolver) governing(file string) []*project {
	if gov, ok := r.gov[path.Dir(file)]; ok {
		return gov
	}
	return r.governingDir(path.Dir(file))
}

func (r *resolver) governingDir(start string) []*project {
	var out []*project
	seen := map[string]bool{}
	var visit func(dir string, hops int)
	visit = func(dir string, hops int) {
		p := r.projects[dir]
		if p == nil || seen[dir] || hops > 8 {
			return
		}
		seen[dir] = true
		out = append(out, p)
		for _, l := range p.local {
			visit(l, hops+1)
		}
	}
	for dir := start; ; dir = path.Dir(dir) {
		visit(dir, 0)
		if dir == "." || dir == "/" {
			break
		}
	}
	if len(out) == 0 {
		for _, d := range r.dirs {
			out = append(out, r.projects[d])
		}
	}
	return out
}

// Implements: REQ-CLOJURE-004, REQ-CLOJURE-005, REQ-CLOJURE-006, REQ-CLOJURE-007
func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	kind, _, _ := strings.Cut(imp.Name, ":")
	switch kind {
	case kindDep, kindPlugin:
		c, ok := r.coords[file][imp.Module]
		if !ok {
			return lang.Target{}
		}
		return r.coordTarget(path.Dir(file), c)
	case kindNS:
		return r.namespace(file, imp.Module)
	case kindNPM:
		return r.npm(file, imp.Module, false)
	case kindClass:
		return r.class(file, imp.Module)
	case kindLoad, kindLoadFile:
		return r.load(file, imp.Module, kind == kindLoadFile)
	}
	return lang.Target{}
}

// coordTarget is where a manifest's coordinate points: a project directory for
// :local/root, a Maven artifact otherwise - pinned by an exact version, or for a git
// dependency by a full :git/sha (a tag alone pins nothing: it can be moved).
//
// Implements: REQ-CLOJURE-008
func (r *resolver) coordTarget(dir string, c coord) lang.Target {
	switch {
	case c.local != "":
		p := path.Join(dir, c.local)
		if q := r.projects[p]; q != nil {
			return lang.Target{Local: q.files[0]}
		}
		return lang.Target{}
	case c.gitURL != "":
		t := lang.Target{Ecosystem: ecoMaven, Package: artifact(c.lib), Origin: c.gitURL}
		switch {
		case c.sha != "":
			t.Version, t.Pinned = c.sha, lang.Commit(c.sha)
			if c.tag != "" {
				t.Requested = c.tag
			}
		case c.tag != "":
			t.Version = c.tag
		default:
			t.Floating = true
		}
		return t
	}
	if q := r.named[artifact(c.lib)]; q != nil {
		return lang.Target{Local: q.files[0]} // built in this repository (a Leiningen monorepo)
	}
	pinned := lang.PinnedMaven(c.version)
	return lang.Target{Ecosystem: ecoMaven, Package: artifact(c.lib), Version: c.version, Pinned: pinned,
		Floating: c.version != "" && !pinned}
}

// extPrefs are the source extensions a file of a platform loads, preferred first:
// Clojure .clj, ClojureScript .cljs (and .clj for its macros), both .cljc (and a
// .cljc's reader conditionals reach either platform's files), babashka .bb.
func extPrefs(file string) []string {
	switch path.Ext(file) {
	case ".cljs":
		return []string{".cljs", ".cljc", ".clj"} // .clj: a namespace of macros
	case ".cljc":
		return []string{".cljc", ".clj", ".cljs"}
	case ".clj":
		return []string{".clj", ".cljc"}
	}
	return []string{".bb", ".clj", ".cljc"} // .bb, bb.edn, a bb script
}

// localNS finds the project file defining a namespace: a.b-c at a/b_c.<ext> under
// a governing project's source paths, else any file whose ns form declares it.
//
// Implements: REQ-CLOJURE-004
func (r *resolver) localNS(file, ns string, gov []*project) string {
	exts := extPrefs(file)
	rel := strings.ReplaceAll(strings.ReplaceAll(ns, ".", "/"), "-", "_")
	for _, p := range gov {
		for _, root := range p.roots {
			stem := rel
			if root != "." {
				stem = root + "/" + rel
			}
			if files := r.byStem[stem]; files != nil {
				for _, ext := range exts {
					if c := stem + ext; c != file && contains(files, c) {
						return c
					}
				}
			}
		}
	}
	best, bestRank := "", 1<<30
	for _, c := range r.byNS[ns] {
		if c == file {
			continue
		}
		rank := len(exts)
		for i, ext := range exts {
			if strings.HasSuffix(c, ext) {
				rank = i
			}
		}
		if rank == len(exts) && path.Ext(c) != "" {
			continue // another platform's file
		}
		rank *= 4
		if !under(c, gov) {
			rank += 2
		}
		if rank < bestRank {
			best, bestRank = c, rank
		}
	}
	return best
}

func under(file string, gov []*project) bool {
	for _, p := range gov {
		if p.dir == "." || strings.HasPrefix(file, p.dir+"/") {
			return true
		}
	}
	return false
}

// std places a namespace on the clojure-std island: what org.clojure/clojure ships,
// ClojureScript's cljs.* and the Closure Library (goog) it bundles, and in
// ClojureScript clojure.spec.alpha, which is cljs.spec.alpha there.
//
// Implements: REQ-CLOJURE-005
func std(file, ns string) (string, bool) {
	switch {
	case clojureStd[ns]:
		return ns, true
	case ns == "goog" || strings.HasPrefix(ns, "goog."):
		return "goog", true
	case strings.HasPrefix(ns, "cljs."):
		for k := ns; k != ""; k = parentNS(k) {
			if _, ok := cljsNotStd[k]; ok {
				return "", false
			}
		}
		return ns, true
	case path.Ext(file) == ".cljs" && strings.HasPrefix(ns, "clojure.spec."):
		return ns, true
	}
	return "", false
}

// clojureOwn are artifacts org.clojure/clojure itself depends on: on every
// classpath, declared or not.
var clojureOwn = map[string]bool{"org.clojure:spec.alpha": true, "org.clojure:core.specs.alpha": true}

func (r *resolver) bbContext(file string, gov []*project) bool {
	switch {
	case path.Ext(file) == ".bb", path.Base(file) == bbEdn, path.Ext(file) == "":
		return true
	case len(gov) > 0 && gov[0].bb && !gov[0].other:
		return true
	}
	for _, p := range gov {
		for _, root := range p.bbRoots {
			if strings.HasPrefix(file, root+"/") {
				return true // on bb.edn's :paths: babashka runs it
			}
		}
	}
	return false
}

// namespace resolves a required namespace.
//
// Implements: REQ-CLOJURE-004, REQ-CLOJURE-005, REQ-CLOJURE-007
func (r *resolver) namespace(file, ns string) lang.Target {
	if contains(r.byNS[ns], file) {
		// Its own namespace: in ClojureScript, the macros of the same name (a .clj
		// beside the .cljs, or the .cljc itself, which is no edge).
		if p := r.localNS(file, ns, r.governing(file)); p != "" {
			return lang.Target{Local: p}
		}
		return lang.Target{}
	}
	// Everything below depends on the file's directory, extension and name only
	// through the governing projects, extPrefs and bbContext.
	key := path.Dir(file) + "|" + path.Ext(file) + "|" + strconv.FormatBool(path.Base(file) == bbEdn) + "|" + ns
	if t, ok := r.memo.Load(key); ok {
		return t.(lang.Target)
	}
	t := r.resolveNS(file, ns)
	r.memo.Store(key, t)
	return t
}

func (r *resolver) resolveNS(file, ns string) lang.Target {
	gov := r.governing(file)
	if p := r.localNS(file, ns, gov); p != "" {
		return lang.Target{Local: p}
	}
	if pkg, ok := std(file, ns); ok {
		return lang.Target{Ecosystem: ecoStd, Package: pkg}
	}
	a := contrib(ns)
	if a == "" {
		a = known(ns)
	}
	if clojureOwn[a] {
		if t, ok := r.lookup(a, gov); ok {
			return t
		}
		return lang.Target{Ecosystem: ecoMaven, Package: a}
	}
	if t, ok := r.declared(ns, gov, 50); ok {
		return t
	}
	if r.bbContext(file, gov) {
		for _, b := range babashkaBuiltins {
			if ns == strings.TrimSuffix(b, ".") || strings.HasSuffix(b, ".") && strings.HasPrefix(ns, b) {
				return lang.Target{Ecosystem: ecoStd, Package: "babashka"}
			}
		}
	}
	if ext := path.Ext(file); ext == ".cljs" || ext == ".cljc" {
		if t := r.npm(file, ns, true); t.Package != "" {
			return t // shadow-cljs: (:require [react :as r]) names the npm package
		}
	}
	seg, _, _ := strings.Cut(ns, ".")
	if a == "" && !r.roots[seg] {
		// Only the group's name in common (datomic.api from com.datomic/datomic-pro):
		// not for a namespace the table knows, nor one under the project's own root.
		if t, ok := r.declared(ns, gov, 1); ok {
			return t
		}
	}
	if r.roots[seg] && a == "" {
		return lang.Target{} // the project's own namespace, not in the repository (generated, removed)
	}
	if a == "" {
		a = guess(ns)
	}
	return lang.Target{Ecosystem: ecoMaven, Package: a, Unresolved: true}
}

// guess names the artifact of a namespace nothing declares and no table knows:
// com.climate.claypoole is com.climate:claypoole, foo.core foo:foo.
func guess(ns string) string {
	segments := strings.Split(ns, ".")
	switch segments[0] {
	case "com", "org", "net", "io", "me", "dev", "tech", "ai", "co", "de", "nl", "se", "fi", "ch", "uk":
		if len(segments) >= 3 {
			return segments[0] + "." + segments[1] + ":" + segments[2]
		}
	}
	return segments[0] + ":" + segments[0]
}

// lookup is a governing project's declaration of an artifact.
func (r *resolver) lookup(a string, gov []*project) (lang.Target, bool) {
	for _, p := range gov {
		if c, ok := p.deps[a]; ok {
			return r.coordTarget(p.dir, c), true
		}
	}
	return lang.Target{}, false
}

// declared finds the declared artifact that provides a namespace: the table's
// artifact when a governing project declares it, else the best-named declared
// artifact scoring at least min (see score). Clojure and ClojureScript themselves
// are never matched by name: their namespaces are the std island's.
//
// Implements: REQ-CLOJURE-007
func (r *resolver) declared(ns string, gov []*project, min int) (lang.Target, bool) {
	for _, a := range []string{contrib(ns), known(ns)} {
		if a != "" {
			if t, ok := r.lookup(a, gov); ok {
				return t, true
			}
		}
	}
	best, bestScore := "", 0
	var bestProj *project
	for _, p := range gov {
		for _, a := range p.order {
			if a == p.name || a == "org.clojure:clojure" || a == "org.clojure:clojurescript" {
				continue
			}
			if s := score(ns, a); s > bestScore || s == bestScore && s > 0 && a < best && bestProj == p {
				best, bestScore, bestProj = a, s, p
			}
		}
	}
	if bestScore < min || bestScore == 0 {
		return lang.Target{}, false
	}
	return r.coordTarget(bestProj.dir, bestProj.deps[best]), true
}

// genericNames are artifact names too common as namespace segments to identify one.
var genericNames = map[string]bool{"core": true, "api": true, "impl": true, "util": true, "utils": true,
	"common": true, "client": true, "server": true, "test": true, "spec": true, "alpha": true, "main": true}

// classGeneric are artifact names that name a format or protocol package in many
// libraries' classes (com.google.api.client.json is not org.babashka/json).
var classGeneric = map[string]bool{"json": true, "xml": true, "yaml": true, "http": true, "jdbc": true,
	"sql": true, "csv": true, "io": true, "net": true, "time": true, "log": true, "logging": true}

// fold compares names regardless of case, dots, dashes and underscores.
func fold(s string) string {
	return strings.Map(func(c rune) rune {
		switch c {
		case '.', '-', '_':
			return -1
		}
		if c >= 'A' && c <= 'Z' {
			return c + 'a' - 'A'
		}
		return c
	}, s)
}

// score rates how well an artifact's name matches a namespace:
//   - the namespace's leading segments are the artifact's name (next.jdbc from
//     com.github.seancorfield/next.jdbc, honey.sql from honeysql, reitit.ring from
//     reitit-ring, cheshire.core from cheshire): 100 + 10 per segment;
//   - the same, for the artifact's name without a clj-/cljs-/clojure- prefix or a
//     -clj/-cljs/-clojure suffix (jmh.core from jmh-clojure): 95 + 10 per segment;
//   - they are the group's last segment (or whole group) and then the artifact
//     (taoensso.timbre from com.taoensso/timbre, clojure.data.json from
//     org.clojure/data.json, day8.re-frame.http-fx from day8.re-frame/http-fx): 105 +
//     10 per artifact segment;
//   - every word of the artifact is a word of the namespace, the first word first
//     (ring.mock.request from ring/ring-mock): 50 + the number of words;
//   - the first segment is the group's last segment and the second a word of the
//     artifact (cognitect.transit from com.cognitect/transit-clj): 40;
//   - the first segment is the group's last segment and the second is generic or
//     the artifact carries the group's name (datomic.api from
//     com.datomic/datomic-pro): 10, 12 for the group's main artifact.
func score(ns, art string) int {
	g, a, _ := strings.Cut(art, ":")
	segments := strings.Split(ns, ".")
	fa := fold(a)
	for k := len(segments); k >= 1; k-- {
		if fold(strings.Join(segments[:k], ".")) == fa {
			return 100 + 10*k
		}
	}
	// A Clojure wrapper named after what it wraps: jmh-clojure provides jmh.core,
	// cljs-ajax ajax.core, clj-yaml's namespaces may be yaml.*.
	for _, affix := range []string{"clj-", "cljs-", "clojure-", "-clj", "-cljs", "-clojure"} {
		b := strings.TrimSuffix(strings.TrimPrefix(a, affix), affix)
		if b == a || b == "" {
			continue
		}
		for k := len(segments); k >= 1; k-- {
			if fold(strings.Join(segments[:k], ".")) == fold(b) {
				return 95 + 10*k
			}
		}
	}
	gsegments := strings.Split(g, ".")
	glast := gsegments[len(gsegments)-1]
	rest := []string(nil)
	switch {
	case len(segments) > len(gsegments) && strings.Join(segments[:len(gsegments)], ".") == g:
		rest = segments[len(gsegments):]
	case len(segments) > 1 && segments[0] == glast:
		rest = segments[1:]
	}
	for k := len(rest); k >= 1; k-- {
		if fold(strings.Join(rest[:k], ".")) == fa {
			return 105 + 10*k
		}
	}
	words := strings.FieldsFunc(a, func(c rune) bool { return c == '-' || c == '.' || c == '_' })
	nsWords := map[string]bool{}
	for _, s := range segments {
		for _, w := range strings.FieldsFunc(s, func(c rune) bool { return c == '-' || c == '_' }) {
			nsWords[w] = true
		}
	}
	if len(words) > 1 && strings.HasPrefix(ns, words[0]) {
		all := true
		for _, w := range words {
			all = all && nsWords[w]
		}
		if all {
			return 50 + len(words)
		}
	}
	if segments[0] == glast && len(segments) > 1 {
		// Only the group in common: datomic.api from com.datomic/datomic-pro, but not
		// babashka.curl from babashka/fs - a named sub-namespace is another library.
		switch {
		case a == glast || a == glast+"-core":
			return 12
		case strings.Contains(a, glast) || genericNames[segments[1]]:
			return 10
		}
		for _, w := range words {
			if w == segments[1] {
				return 40 // cognitect.transit from com.cognitect/transit-clj
			}
		}
	}
	return 0
}

// npmName is the package a JavaScript module specifier names: "@mui/material/Button"
// is @mui/material, "react-dom/client" is react-dom.
func npmName(spec string) string {
	parts := strings.Split(spec, "/")
	if strings.HasPrefix(spec, "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// npm resolves a ClojureScript string require to an npm package declared by a
// governing project's package.json (or a library's deps.cljs), or to Node's own
// module. onlyDeclared is for symbol requires, which name npm packages only when
// one is declared.
//
// Implements: REQ-CLOJURE-009
func (r *resolver) npm(file, spec string, onlyDeclared bool) lang.Target {
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/") {
		if onlyDeclared {
			return lang.Target{}
		}
		p := path.Join(path.Dir(file), spec)
		for _, ext := range []string{"", ".js", ".mjs", ".cjs", ".ts", "/index.js"} {
			if r.files[p+ext] {
				return lang.Target{Local: p + ext}
			}
		}
		return lang.Target{}
	}
	name := npmName(strings.TrimPrefix(spec, "node:"))
	if !onlyDeclared && (strings.HasPrefix(spec, "node:") || jsBuiltins[name]) {
		return lang.Target{Ecosystem: ecoNode, Package: name}
	}
	for _, p := range r.governing(file) {
		if v, ok := p.npm[name]; ok {
			return lang.Target{Ecosystem: ecoNPM, Package: name, Version: v, Pinned: lang.PinnedSemver(v)}
		}
	}
	if onlyDeclared {
		return lang.Target{}
	}
	return lang.Target{Ecosystem: ecoNPM, Package: name, Unresolved: true}
}

// class resolves an imported class: the JDK's, Clojure's own clojure.lang, the
// Closure Library's, a record or type of a project namespace, a Java file of the
// project; then a Maven artifact as the Java plugin matches a Java import to one
// (java.Artifacts: a known or name-derived package prefix, or the group) - a
// declared artifact, or one of its group arriving with a declared one at their
// shared version (jackson-annotations beside jackson-databind); then a declared
// artifact whose coordinates spell the class's package (classScore) or whose
// namespace the package is (a library's deftype). Anything else is dropped rather
// than named unresolved as Java would: classes of transitive jars are imported
// routinely (quartzite's org.quartz), and a guessed node for each would be noise.
//
// Implements: REQ-CLOJURE-006
func (r *resolver) class(file, cls string) lang.Target {
	key := "class|" + path.Dir(file) + "|" + path.Ext(file) + "|" + cls
	if t, ok := r.memo.Load(key); ok {
		return t.(lang.Target)
	}
	t := r.resolveClass(file, cls)
	r.memo.Store(key, t)
	return t
}

func (r *resolver) resolveClass(file, cls string) lang.Target {
	if t, ok := java.JDK(cls); ok {
		return t
	}
	switch {
	case strings.HasPrefix(cls, "clojure.lang."):
		return lang.Target{Ecosystem: ecoStd, Package: "clojure.lang"}
	case strings.HasPrefix(cls, "goog."):
		return lang.Target{Ecosystem: ecoStd, Package: "goog"}
	}
	pkg, name := parentNS(cls), cls[strings.LastIndexByte(cls, '.')+1:]
	gov := r.governing(file)
	if pkg != "" {
		if p := r.localNS(file, strings.ReplaceAll(pkg, "_", "-"), gov); p != "" {
			return lang.Target{Local: p} // deftype/defrecord compile to classes of their namespace
		}
	}
	rel := strings.ReplaceAll(cls, ".", "/") + ".java"
	for _, p := range r.byJava[name+".java"] {
		if p == rel || strings.HasSuffix(p, "/"+rel) {
			return lang.Target{Local: p}
		}
	}
	if t, ok := r.javaArtifact(cls, gov); ok {
		return t
	}
	best, bestScore := "", 0
	var bestProj *project
	for _, p := range gov {
		for _, a := range p.order {
			if s := classScore(cls, a); s > bestScore || s == bestScore && s > 0 && bestProj == p && a < best {
				best, bestScore, bestProj = a, s, p
			}
		}
	}
	if bestScore > 0 {
		return r.coordTarget(bestProj.dir, bestProj.deps[best])
	}
	// A class a library's deftype or defrecord compiles to (methodical.interface.Cache).
	if pkg != "" {
		if t, ok := r.declared(strings.ReplaceAll(pkg, "_", "-"), gov, 50); ok {
			return t
		}
	}
	return lang.Target{}
}

// javaArtifact matches a class to a Maven artifact by the Java plugin's rules, over
// the artifacts the governing projects declare. A declared artifact is the first
// governing project's coordinate (its version, pin, git origin or local root); an
// artifact arriving with its group has the version its group's declared artifacts
// share.
//
// Implements: REQ-CLOJURE-006
func (r *resolver) javaArtifact(cls string, gov []*project) (lang.Target, bool) {
	if len(gov) == 0 {
		return lang.Target{}, false
	}
	dirs := make([]string, len(gov))
	for i, p := range gov {
		dirs[i] = p.dir
	}
	v, _ := r.classes.LoadOrStore(strings.Join(dirs, "\x00"), &classMatcher{})
	cm := v.(*classMatcher)
	cm.once.Do(func() {
		// Clojure's own root packages are no artifact's: org.clojure/clojure does not
		// ship clojure.core.async's deftypes.
		cm.arts = java.NewArtifacts(java.Language{Prefixes: []string{"clojure.", "cljs."}})
		for _, p := range gov {
			for _, ga := range p.order {
				g, a, _ := strings.Cut(ga, ":")
				cm.arts.Declare(g, a, p.deps[ga].version)
			}
		}
		cm.arts.Finish()
	})
	m, ok := cm.arts.Match(cls)
	if !ok {
		return lang.Target{}, false
	}
	if !m.Virtual {
		for _, p := range gov {
			if c, ok := p.deps[m.Name]; ok {
				return r.coordTarget(p.dir, c), true
			}
		}
	}
	pinned := lang.PinnedMaven(m.Version)
	return lang.Target{Ecosystem: ecoMaven, Package: m.Name, Version: m.Version, Pinned: pinned,
		Floating: m.Version != "" && !pinned}, true
}

// classScore rates how well a declared artifact's coordinates match a Java class:
// 40 when the artifact's name spells two of the class's package segments
// (org.apache.commons.io from commons-io, org.eclipse.jetty.server from
// jetty-server), 10 per leading package segment of the whole group (+5; org.quartz
// in org.quartz-scheduler counts) or of at least three segments of it, 5 when the
// class's first segment is the group's last (liquibase.Liquibase from
// org.liquibase/liquibase-core) or the artifact one of its first three package
// segments (org.h2 from com.h2database/h2); then +2 for each word of the artifact the class
// names and -3 for each it does not. 0 when nothing connects them.
func classScore(cls, art string) int {
	g, a, _ := strings.Cut(art, ":")
	cs, gs := strings.Split(cls, "."), strings.Split(g, ".")
	pkg := cs[:len(cs)-1]
	score := 0
	if fa := fold(a); !genericNames[a] && !classGeneric[a] {
		for k := 0; k < len(pkg); k++ {
			if k+2 <= len(pkg) && fold(pkg[k]+pkg[k+1]) == fa {
				score = 40
			} else if fold(pkg[k]) == fa && k < 3 {
				score = max(score, 5)
			}
		}
	}
	common := 0
	for common < len(gs) && common < len(pkg) && (gs[common] == pkg[common] || strings.HasPrefix(gs[common], pkg[common]+"-")) {
		common++
	}
	switch {
	case common == len(gs):
		score = max(score, 10*common+5)
	case common >= 3:
		score = max(score, 10*common)
	case len(pkg) > 0 && pkg[0] == gs[len(gs)-1]:
		score = max(score, 5)
	}
	if score == 0 {
		return 0
	}
	segments := map[string]bool{}
	for _, s := range pkg {
		segments[strings.ToLower(s)] = true
	}
	for _, w := range strings.FieldsFunc(a, func(c rune) bool { return c == '-' || c == '.' }) {
		if segments[strings.ToLower(w)] {
			score += 2
		} else {
			score -= 3
		}
	}
	return max(score, 1)
}

// load resolves (load "x/y") - relative to the namespace's directory, which is the
// file's own, or to a source path when it starts with "/" - and (load-file "p"),
// relative to the working directory (the project's) or the file.
//
// Implements: REQ-CLOJURE-002
func (r *resolver) load(file, p string, isFile bool) lang.Target {
	var bases []string
	gov := r.governing(file)
	switch {
	case isFile:
		for _, g := range gov {
			bases = append(bases, path.Join(g.dir, p))
		}
		bases = append(bases, path.Join(path.Dir(file), p), path.Clean(p))
	case strings.HasPrefix(p, "/"):
		for _, g := range gov {
			for _, root := range g.roots {
				bases = append(bases, path.Join(root, p))
			}
		}
	default:
		bases = append(bases, path.Join(path.Dir(file), p))
	}
	for _, b := range bases {
		for _, ext := range []string{"", ".clj", ".cljc", ".cljs", ".bb"} {
			if c := b + ext; r.files[c] && c != file {
				return lang.Target{Local: c}
			}
		}
	}
	return lang.Target{}
}
