package clojure

import (
	"encoding/json"
	"os"
	"path"
	"slices"
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
	directory    string   // "." for the repository's root
	files        []string // its manifests, sorted
	name         string   // its own artifact (Leiningen's defproject)
	roots        []string // source paths, repository-relative
	dependencies map[string]coordinate
	order        []string          // artifacts in declaration order
	npm          map[string]string // package.json and deps.cljs npm dependencies
	bb           bool              // a bb.edn is among its manifests
	bbRoots      []string          // bb.edn's :paths, repository-relative
	other        bool              // a manifest other than bb.edn is
	local        []string          // directories of its :local/root dependencies
}

type resolver struct {
	files             map[string]bool
	projects          map[string]*project
	directories       []string                         // project directories, shallowest first
	byNS              map[string][]string              // namespace -> the files declaring it
	byJava            map[string][]string              // "Name.java" -> project paths
	coordinates       map[string]map[string]coordinate // manifest path -> lib -> its coordinate
	roots             map[string]bool                  // first segments of the project's namespaces
	named             map[string]*project              // own artifact -> project (defproject names)
	governingProjects map[string][]*project            // directory -> its governing projects
	byStem            map[string][]string              // source path without its extension -> sources
	memo              sync.Map                         // resolutions shared by the files of a directory
	classes           sync.Map                         // governing project dirs -> *classMatcher
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
		byJava: map[string][]string{}, coordinates: map[string]map[string]coordinate{}, roots: map[string]bool{},
		byStem: map[string][]string{},
	}
	projectAt := func(directory string) *project {
		p := r.projects[directory]
		if p == nil {
			p = &project{directory: directory, dependencies: map[string]coordinate{}, npm: map[string]string{}}
			r.projects[directory] = p
		}
		return p
	}
	readable := func(f *scan.File) bool { return !f.Binary && !f.TooLarge && f.Size <= lang.MaxParseSize }
	for _, f := range all {
		r.files[f.Path] = true
		base, directory := path.Base(f.Path), path.Dir(f.Path)
		switch {
		case !readable(f) || skipped(f.Path):
		case manifestNames[base]:
			source, err := os.ReadFile(f.AbsolutePath)
			if err != nil {
				continue
			}
			r.addManifest(projectAt(directory), f.Path, readManifest(base, source))
		case base == "package.json" || base == "deps.cljs":
			if source, err := os.ReadFile(f.AbsolutePath); err == nil {
				readNPM(projectAt(directory), base, source)
			}
		case sourceExtensions[path.Ext(base)] || f.Interpreter == "bb" && scan.Language(f.Path) == "":
			if source, err := os.ReadFile(f.AbsolutePath); err == nil {
				stem := strings.TrimSuffix(f.Path, path.Ext(f.Path))
				r.byStem[stem] = append(r.byStem[stem], f.Path)
				if namespace := namespaceName(source); namespace != "" {
					r.byNS[namespace] = append(r.byNS[namespace], f.Path)
					segment, _, _ := strings.Cut(namespace, ".")
					r.roots[segment] = true
				}
			}
		case strings.HasSuffix(base, ".java"):
			r.byJava[base] = append(r.byJava[base], f.Path)
		}
	}
	for directory, p := range r.projects {
		// A directory with only a package.json is no Clojure project.
		if len(p.files) == 0 {
			delete(r.projects, directory)
			continue
		}
		r.directories = append(r.directories, directory)
		sort.Strings(p.files)
	}
	sort.Slice(r.directories, func(i, j int) bool {
		a, b := r.directories[i], r.directories[j]
		if da, database := lang.Depth(a), lang.Depth(b); da != database {
			return da < database
		}
		return a < b
	})
	for _, files := range r.byNS {
		sort.Strings(files)
	}
	r.governingProjects = map[string][]*project{}
	for _, f := range all {
		if d := path.Dir(f.Path); r.governingProjects[d] == nil {
			r.governingProjects[d] = r.governingDirectory(d)
		}
	}
	r.named = map[string]*project{}
	for _, d := range r.directories {
		if p := r.projects[d]; p.name != "" && r.named[p.name] == nil {
			r.named[p.name] = p
		}
	}
	return r
}

func (r *resolver) addManifest(p *project, file string, m *manifest) {
	p.files = append(p.files, file)
	if m.kind == bbEdn {
		p.bb = true
		for _, root := range m.paths {
			p.bbRoots = append(p.bbRoots, path.Join(p.directory, root))
		}
	} else {
		p.other = true
	}
	if m.name != "" && p.name == "" {
		p.name = artifact(m.name)
	}
	for _, root := range m.paths {
		root = path.Join(p.directory, root)
		if !strings.HasPrefix(root, "../") && root != ".." && !slices.Contains(p.roots, root) {
			p.roots = append(p.roots, root)
		}
	}
	libraries := map[string]coordinate{}
	for _, d := range m.dependencies {
		if _, ok := libraries[d.library]; !ok {
			libraries[d.library] = d
		}
		a := artifact(d.library)
		if _, ok := p.dependencies[a]; ok || d.plugin {
			continue // a plugin extends the build tool, not the code's classpath
		}
		p.dependencies[a] = d
		p.order = append(p.order, a)
		if d.local != "" {
			p.local = append(p.local, path.Join(p.directory, d.local))
		}
	}
	r.coordinates[file] = libraries
}

// readNPM reads the npm dependencies ClojureScript builds install: package.json's
// dependencies and devDependencies, a library's deps.cljs :npm-deps.
func readNPM(p *project, base string, source []byte) {
	if base == "deps.cljs" {
		forms := edn.Read(source)
		if len(forms) > 0 {
			if dependencies := forms[0].Get("npm-deps"); dependencies != nil && dependencies.Kind == edn.Map {
				for i := 0; i+1 < len(dependencies.Kids); i += 2 {
					k, v := dependencies.Kids[i], dependencies.Kids[i+1]
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
	var packageName struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(source, &packageName) != nil {
		return
	}
	for _, m := range []map[string]string{packageName.Dependencies, packageName.DevDependencies} {
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
	if governing, ok := r.governingProjects[path.Dir(file)]; ok {
		return governing
	}
	return r.governingDirectory(path.Dir(file))
}

func (r *resolver) governingDirectory(start string) []*project {
	var out []*project
	seen := map[string]bool{}
	var visit func(directory string, hops int)
	visit = func(directory string, hops int) {
		p := r.projects[directory]
		if p == nil || seen[directory] || hops > 8 {
			return
		}
		seen[directory] = true
		out = append(out, p)
		for _, l := range p.local {
			visit(l, hops+1)
		}
	}
	for directory := start; ; directory = path.Dir(directory) {
		visit(directory, 0)
		if directory == "." || directory == "/" {
			break
		}
	}
	if len(out) == 0 {
		for _, d := range r.directories {
			out = append(out, r.projects[d])
		}
	}
	return out
}

// Implements: REQ-CLOJURE-004, REQ-CLOJURE-005, REQ-CLOJURE-006, REQ-CLOJURE-007
func (r *resolver) Resolve(file string, rawImport lang.RawImport) lang.Target {
	kind, _, _ := strings.Cut(rawImport.Name, ":")
	switch kind {
	case kindDependency, kindPlugin:
		c, ok := r.coordinates[file][rawImport.Module]
		if !ok {
			return lang.Target{}
		}
		return r.coordinateTarget(path.Dir(file), c)
	case kindNS:
		return r.namespace(file, rawImport.Module)
	case kindNPM:
		return r.npm(file, rawImport.Module, false)
	case kindClass:
		return r.class(file, rawImport.Module)
	case kindLoad, kindLoadFile:
		return r.load(file, rawImport.Module, kind == kindLoadFile)
	}
	return lang.Target{}
}

// coordinateTarget is where a manifest's coordinate points: a project directory for
// :local/root, a Maven artifact otherwise - pinned by an exact version, or for a git
// dependency by a full :git/sha (a tag alone pins nothing: it can be moved).
//
// Implements: REQ-CLOJURE-008
func (r *resolver) coordinateTarget(directory string, c coordinate) lang.Target {
	switch {
	case c.local != "":
		p := path.Join(directory, c.local)
		if q := r.projects[p]; q != nil {
			return lang.Target{Local: q.files[0]}
		}
		return lang.Target{}
	case c.gitURL != "":
		t := lang.Target{Ecosystem: ecosystemMaven, Package: artifact(c.library), Origin: c.gitURL}
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
	if q := r.named[artifact(c.library)]; q != nil {
		return lang.Target{Local: q.files[0]} // built in this repository (a Leiningen monorepo)
	}
	pinned := lang.PinnedMaven(c.version)
	return lang.Target{Ecosystem: ecosystemMaven, Package: artifact(c.library), Version: c.version, Pinned: pinned,
		Floating: c.version != "" && !pinned}
}

// extensionPreferences are the source extensions a file of a platform loads, preferred first:
// Clojure .clj, ClojureScript .cljs (and .clj for its macros), both .cljc (and a
// .cljc's reader conditionals reach either platform's files), babashka .bb.
func extensionPreferences(file string) []string {
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
// a governing project's source paths, else any file whose namespace form declares it.
//
// Implements: REQ-CLOJURE-004
func (r *resolver) localNS(file, namespace string, governing []*project) string {
	extensions := extensionPreferences(file)
	relative := strings.ReplaceAll(strings.ReplaceAll(namespace, ".", "/"), "-", "_")
	for _, p := range governing {
		for _, root := range p.roots {
			stem := relative
			if root != "." {
				stem = root + "/" + relative
			}
			if files := r.byStem[stem]; files != nil {
				for _, extension := range extensions {
					if c := stem + extension; c != file && slices.Contains(files, c) {
						return c
					}
				}
			}
		}
	}
	best, bestRank := "", 1<<30
	for _, c := range r.byNS[namespace] {
		if c == file {
			continue
		}
		rank := len(extensions)
		for i, extension := range extensions {
			if strings.HasSuffix(c, extension) {
				rank = i
			}
		}
		if rank == len(extensions) && path.Ext(c) != "" {
			continue // another platform's file
		}
		rank *= 4
		if !under(c, governing) {
			rank += 2
		}
		if rank < bestRank {
			best, bestRank = c, rank
		}
	}
	return best
}

func under(file string, governing []*project) bool {
	for _, p := range governing {
		if p.directory == "." || strings.HasPrefix(file, p.directory+"/") {
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
func std(file, namespace string) (string, bool) {
	switch {
	case clojureStd[namespace]:
		return namespace, true
	case namespace == "goog" || strings.HasPrefix(namespace, "goog."):
		return "goog", true
	case strings.HasPrefix(namespace, "cljs."):
		for k := namespace; k != ""; k = parentNS(k) {
			if _, ok := cljsNotStd[k]; ok {
				return "", false
			}
		}
		return namespace, true
	case path.Ext(file) == ".cljs" && strings.HasPrefix(namespace, "clojure.spec."):
		return namespace, true
	}
	return "", false
}

// clojureOwn are artifacts org.clojure/clojure itself depends on: on every
// classpath, declared or not.
var clojureOwn = map[string]bool{"org.clojure:spec.alpha": true, "org.clojure:core.specs.alpha": true}

func (r *resolver) bbContext(file string, governing []*project) bool {
	switch {
	case path.Ext(file) == ".bb", path.Base(file) == bbEdn, path.Ext(file) == "":
		return true
	case len(governing) > 0 && governing[0].bb && !governing[0].other:
		return true
	}
	for _, p := range governing {
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
func (r *resolver) namespace(file, namespace string) lang.Target {
	if slices.Contains(r.byNS[namespace], file) {
		// Its own namespace: in ClojureScript, the macros of the same name (a .clj
		// beside the .cljs, or the .cljc itself, which is no edge).
		if p := r.localNS(file, namespace, r.governing(file)); p != "" {
			return lang.Target{Local: p}
		}
		return lang.Target{}
	}
	// Everything below depends on the file's directory, extension and name only
	// through the governing projects, extensionPreferences and bbContext.
	key := path.Dir(file) + "|" + path.Ext(file) + "|" + strconv.FormatBool(path.Base(file) == bbEdn) + "|" + namespace
	if t, ok := r.memo.Load(key); ok {
		return t.(lang.Target)
	}
	t := r.resolveNS(file, namespace)
	r.memo.Store(key, t)
	return t
}

func (r *resolver) resolveNS(file, namespace string) lang.Target {
	governing := r.governing(file)
	if p := r.localNS(file, namespace, governing); p != "" {
		return lang.Target{Local: p}
	}
	if packageName, ok := std(file, namespace); ok {
		return lang.Target{Ecosystem: ecosystemStd, Package: packageName}
	}
	a := contrib(namespace)
	if a == "" {
		a = known(namespace)
	}
	if clojureOwn[a] {
		if t, ok := r.lookup(a, governing); ok {
			return t
		}
		return lang.Target{Ecosystem: ecosystemMaven, Package: a}
	}
	if t, ok := r.declared(namespace, governing, 50); ok {
		return t
	}
	if r.bbContext(file, governing) {
		for _, b := range babashkaBuiltins {
			if namespace == strings.TrimSuffix(b, ".") || strings.HasSuffix(b, ".") && strings.HasPrefix(namespace, b) {
				return lang.Target{Ecosystem: ecosystemStd, Package: "babashka"}
			}
		}
	}
	if extension := path.Ext(file); extension == ".cljs" || extension == ".cljc" {
		if t := r.npm(file, namespace, true); t.Package != "" {
			return t // shadow-cljs: (:require [react :as r]) names the npm package
		}
	}
	segment, _, _ := strings.Cut(namespace, ".")
	if a == "" && !r.roots[segment] {
		// Only the group's name in common (datomic.api from com.datomic/datomic-pro):
		// not for a namespace the table knows, nor one under the project's own root.
		if t, ok := r.declared(namespace, governing, 1); ok {
			return t
		}
	}
	if r.roots[segment] && a == "" {
		return lang.Target{} // the project's own namespace, not in the repository (generated, removed)
	}
	if a == "" {
		a = guess(namespace)
	}
	return lang.Target{Ecosystem: ecosystemMaven, Package: a, Unresolved: true}
}

// guess names the artifact of a namespace nothing declares and no table knows:
// com.climate.claypoole is com.climate:claypoole, foo.core foo:foo.
func guess(namespace string) string {
	segments := strings.Split(namespace, ".")
	switch segments[0] {
	case "com", "org", "net", "io", "me", "dev", "tech", "ai", "co", "de", "nl", "se", "fi", "ch", "uk":
		if len(segments) >= 3 {
			return segments[0] + "." + segments[1] + ":" + segments[2]
		}
	}
	return segments[0] + ":" + segments[0]
}

// lookup is a governing project's declaration of an artifact.
func (r *resolver) lookup(a string, governing []*project) (lang.Target, bool) {
	for _, p := range governing {
		if c, ok := p.dependencies[a]; ok {
			return r.coordinateTarget(p.directory, c), true
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
func (r *resolver) declared(namespace string, governing []*project, min int) (lang.Target, bool) {
	for _, a := range []string{contrib(namespace), known(namespace)} {
		if a != "" {
			if t, ok := r.lookup(a, governing); ok {
				return t, true
			}
		}
	}
	best, bestScore := "", 0
	var bestProject *project
	for _, p := range governing {
		for _, a := range p.order {
			if a == p.name || a == "org.clojure:clojure" || a == "org.clojure:clojurescript" {
				continue
			}
			if s := score(namespace, a); s > bestScore || s == bestScore && s > 0 && a < best && bestProject == p {
				best, bestScore, bestProject = a, s, p
			}
		}
	}
	if bestScore < min || bestScore == 0 {
		return lang.Target{}, false
	}
	return r.coordinateTarget(bestProject.directory, bestProject.dependencies[best]), true
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
func score(namespace, art string) int {
	g, a, _ := strings.Cut(art, ":")
	segments := strings.Split(namespace, ".")
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
	namespaceWords := map[string]bool{}
	for _, s := range segments {
		for _, w := range strings.FieldsFunc(s, func(c rune) bool { return c == '-' || c == '_' }) {
			namespaceWords[w] = true
		}
	}
	if len(words) > 1 && strings.HasPrefix(namespace, words[0]) {
		all := true
		for _, w := range words {
			all = all && namespaceWords[w]
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
		for _, extension := range []string{"", ".js", ".mjs", ".cjs", ".ts", "/index.js"} {
			if r.files[p+extension] {
				return lang.Target{Local: p + extension}
			}
		}
		return lang.Target{}
	}
	name := lang.NPMPackageName(strings.TrimPrefix(spec, "node:"))
	if !onlyDeclared && (strings.HasPrefix(spec, "node:") || jsBuiltins[name]) {
		return lang.Target{Ecosystem: ecosystemNode, Package: name}
	}
	for _, p := range r.governing(file) {
		if v, ok := p.npm[name]; ok {
			return lang.Target{Ecosystem: ecosystemNPM, Package: name, Version: v, Pinned: lang.PinnedSemver(v)}
		}
	}
	if onlyDeclared {
		return lang.Target{}
	}
	return lang.Target{Ecosystem: ecosystemNPM, Package: name, Unresolved: true}
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
func (r *resolver) class(file, class string) lang.Target {
	key := "class|" + path.Dir(file) + "|" + path.Ext(file) + "|" + class
	if t, ok := r.memo.Load(key); ok {
		return t.(lang.Target)
	}
	t := r.resolveClass(file, class)
	r.memo.Store(key, t)
	return t
}

func (r *resolver) resolveClass(file, class string) lang.Target {
	if t, ok := java.JDK(class); ok {
		return t
	}
	switch {
	case strings.HasPrefix(class, "clojure.lang."):
		return lang.Target{Ecosystem: ecosystemStd, Package: "clojure.lang"}
	case strings.HasPrefix(class, "goog."):
		return lang.Target{Ecosystem: ecosystemStd, Package: "goog"}
	}
	packageName, name := parentNS(class), class[strings.LastIndexByte(class, '.')+1:]
	governing := r.governing(file)
	if packageName != "" {
		if p := r.localNS(file, strings.ReplaceAll(packageName, "_", "-"), governing); p != "" {
			return lang.Target{Local: p} // deftype/defrecord compile to classes of their namespace
		}
	}
	relative := strings.ReplaceAll(class, ".", "/") + ".java"
	for _, p := range r.byJava[name+".java"] {
		if p == relative || strings.HasSuffix(p, "/"+relative) {
			return lang.Target{Local: p}
		}
	}
	if t, ok := r.javaArtifact(class, governing); ok {
		return t
	}
	best, bestScore := "", 0
	var bestProject *project
	for _, p := range governing {
		for _, a := range p.order {
			if s := classScore(class, a); s > bestScore || s == bestScore && s > 0 && bestProject == p && a < best {
				best, bestScore, bestProject = a, s, p
			}
		}
	}
	if bestScore > 0 {
		return r.coordinateTarget(bestProject.directory, bestProject.dependencies[best])
	}
	// A class a library's deftype or defrecord compiles to (methodical.interface.Cache).
	if packageName != "" {
		if t, ok := r.declared(strings.ReplaceAll(packageName, "_", "-"), governing, 50); ok {
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
func (r *resolver) javaArtifact(class string, governing []*project) (lang.Target, bool) {
	if len(governing) == 0 {
		return lang.Target{}, false
	}
	directories := make([]string, len(governing))
	for i, p := range governing {
		directories[i] = p.directory
	}
	v, _ := r.classes.LoadOrStore(strings.Join(directories, "\x00"), &classMatcher{})
	matcher := v.(*classMatcher)
	matcher.once.Do(func() {
		// Clojure's own root packages are no artifact's: org.clojure/clojure does not
		// ship clojure.core.async's deftypes.
		matcher.arts = java.NewArtifacts(java.Language{Prefixes: []string{"clojure.", "cljs."}})
		for _, p := range governing {
			for _, groupArtifact := range p.order {
				g, a, _ := strings.Cut(groupArtifact, ":")
				matcher.arts.Declare(g, a, p.dependencies[groupArtifact].version)
			}
		}
		matcher.arts.Finish()
	})
	m, ok := matcher.arts.Match(class)
	if !ok {
		return lang.Target{}, false
	}
	if !m.Virtual {
		for _, p := range governing {
			if c, ok := p.dependencies[m.Name]; ok {
				return r.coordinateTarget(p.directory, c), true
			}
		}
	}
	pinned := lang.PinnedMaven(m.Version)
	return lang.Target{Ecosystem: ecosystemMaven, Package: m.Name, Version: m.Version, Pinned: pinned,
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
func classScore(class, art string) int {
	g, a, _ := strings.Cut(art, ":")
	classParts, groupParts := strings.Split(class, "."), strings.Split(g, ".")
	packageName := classParts[:len(classParts)-1]
	score := 0
	if fa := fold(a); !genericNames[a] && !classGeneric[a] {
		for k := 0; k < len(packageName); k++ {
			if k+2 <= len(packageName) && fold(packageName[k]+packageName[k+1]) == fa {
				score = 40
			} else if fold(packageName[k]) == fa && k < 3 {
				score = max(score, 5)
			}
		}
	}
	common := 0
	for common < len(groupParts) && common < len(packageName) && (groupParts[common] == packageName[common] || strings.HasPrefix(groupParts[common], packageName[common]+"-")) {
		common++
	}
	switch {
	case common == len(groupParts):
		score = max(score, 10*common+5)
	case common >= 3:
		score = max(score, 10*common)
	case len(packageName) > 0 && packageName[0] == groupParts[len(groupParts)-1]:
		score = max(score, 5)
	}
	if score == 0 {
		return 0
	}
	segments := map[string]bool{}
	for _, s := range packageName {
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
	governing := r.governing(file)
	switch {
	case isFile:
		for _, g := range governing {
			bases = append(bases, path.Join(g.directory, p))
		}
		bases = append(bases, path.Join(path.Dir(file), p), path.Clean(p))
	case strings.HasPrefix(p, "/"):
		for _, g := range governing {
			for _, root := range g.roots {
				bases = append(bases, path.Join(root, p))
			}
		}
	default:
		bases = append(bases, path.Join(path.Dir(file), p))
	}
	for _, b := range bases {
		for _, extension := range []string{"", ".clj", ".cljc", ".cljs", ".bb"} {
			if c := b + extension; r.files[c] && c != file {
				return lang.Target{Local: c}
			}
		}
	}
	return lang.Target{}
}
