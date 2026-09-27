package bazel

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/starlark"
)

// Import kinds (RawImport.Name).
const (
	impLoad  = "load"  // a label load(), use_extension(), use_repo_rule(), include() or Label() names
	impLabel = "label" // a label an attribute names
	impGlob  = "glob"  // Module: include patterns joined by \x00, \x01, exclude patterns
	impDep   = "dep"   // a bazel_dep; Module = the module's name
	impRepo  = "repo"  // a repository rule's declaration; Module = the repository's name
	impMaven = "maven" // a Maven coordinate; Module = coordinate "\n" hub repository
	impPip   = "pip"   // requirement("x"); Module = hub repository "\n" distribution
	impGoMod = "gomod" // go_deps.module(path = ...); Module = the module path
	impCrate = "crate" // crate.spec(package = ...); Module = the crate
)

// repoRules are the repository rules that declare one external repository by
// name; hubRules those that declare a hub of packages of another ecosystem.
var (
	repoRules = map[string]bool{
		"http_archive": true, "http_file": true, "http_jar": true, "git_repository": true,
		"new_git_repository": true, "local_repository": true, "new_local_repository": true,
		"go_repository": true,
	}
	hubRules = map[string]bool{
		"maven_install": true, "pip_parse": true, "pip_install": true, "npm_translate_lock": true,
		"crates_repository": true,
	}
)

// labelAttrs are the rule attributes that hold labels, besides any attribute
// whose name ends in deps, srcs or hdrs.
var labelAttrs = map[string]bool{
	"srcs": true, "hdrs": true, "textual_hdrs": true, "data": true, "deps": true, "runtime_deps": true,
	"implementation_deps": true, "interface_deps": true, "exports": true, "proto": true, "protos": true,
	"embed": true, "plugins": true, "exported_plugins": true, "resources": true, "resource_jars": true,
	"main": true, "actual": true, "tools": true, "tests": true, "src": true, "srcs_jars": true,
	"library": true, "binary": true, "additional_linker_inputs": true, "linker_script": true,
	"win_def_file": true, "module_map": true, "entry_point": true, "entry_points": true,
	"compatible_with": false, "target_compatible_with": false,
}

func labelAttr(name string) bool {
	if v, ok := labelAttrs[name]; ok {
		return v
	}
	return strings.HasSuffix(name, "deps") || strings.HasSuffix(name, "_srcs") || strings.HasSuffix(name, "_hdrs")
}

// notTargets are the BUILD-file functions that declare no target.
var notTargets = map[string]bool{
	"load": true, "package": true, "licenses": true, "exports_files": true, "workspace": true,
	"package_group": false,
}

// symbolKinds names a .bzl global by the function that made it.
var symbolKinds = map[string]string{
	"rule": "rule", "macro": "macro", "repository_rule": "repository_rule",
	"module_extension": "module_extension", "provider": "provider", "aspect": "aspect",
	"tag_class": "tag_class", "transition": "transition", "struct": "struct",
}

// loaded is a symbol a load() brought in: from which label, under which name.
type loaded struct{ label, name string }

type extractor struct {
	kind    string
	dir     string
	loads   map[string]loaded
	exts    map[string]string // MODULE.bazel extension proxies: variable -> extension name
	rules   map[string]string // MODULE.bazel use_repo_rule: variable -> rule name
	ex      *lang.Extraction
	symbols lang.SymbolSet
	seen    map[string]bool
}

// extract reads what one Starlark file declares and names.
func extract(kind, dir string, src []byte) *lang.Extraction {
	f := starlark.Parse(src)
	x := &extractor{kind: kind, dir: dir, loads: map[string]loaded{}, exts: map[string]string{},
		rules: map[string]string{}, ex: &lang.Extraction{}, seen: map[string]bool{}}
	for _, st := range f.Stmts {
		if st.Def == "" && st.Kind == 'e' && st.X.Callee() == "load" {
			x.load(st.X)
		}
	}
	for _, st := range f.Stmts {
		x.stmt(st)
	}
	x.ex.Symbols = x.symbols.List()
	if x.ex.Symbols == nil {
		x.ex.Symbols = []lang.Symbol{}
	}
	return x.ex
}

func (x *extractor) add(kind, spec, module string, line int) {
	if spec == "" || x.seen[spec] {
		return
	}
	x.seen[spec] = true
	x.ex.Imports = append(x.ex.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

// load records a load() statement's label and the names it binds.
func (x *extractor) load(call *starlark.Node) {
	label, ok := call.Pos(0).Str()
	if !ok {
		return
	}
	x.add(impLoad, label, label, call.Line)
	for i, a := range call.Args {
		if i == 0 || a.Star != "" {
			continue
		}
		name, ok := a.Val.Str()
		if !ok {
			continue
		}
		local := a.Name
		if local == "" {
			local = name
		}
		x.loads[local] = loaded{label: label, name: name}
	}
}

// original is the name a called function has where it is defined: what a load()
// bound it from, without native.
func (x *extractor) original(callee string) string {
	if l, ok := x.loads[callee]; ok {
		return l.name
	}
	return strings.TrimPrefix(callee, "native.")
}

func (x *extractor) stmt(st starlark.Stmt) {
	top := st.Def == ""
	switch {
	case st.Kind == 'd' && top:
		x.symbols.Add(st.Name, "func", st.Line)
		return
	case st.Kind == 'a' && top && x.kind != kindBuild:
		callee := st.X.Callee()
		if x.kind == kindModule {
			switch callee {
			case "use_extension":
				if ext, ok := st.X.Pos(1).Str(); ok && len(st.Targets) == 1 {
					x.exts[st.Targets[0]] = ext
				}
			case "use_repo_rule":
				if rule, ok := st.X.Pos(1).Str(); ok && len(st.Targets) == 1 {
					x.rules[st.Targets[0]] = rule
				}
			}
		} else if st.Col == 0 {
			kind := symbolKinds[callee]
			if kind == "" {
				kind = "var"
			}
			for _, t := range st.Targets {
				x.symbols.Add(t, kind, st.Line)
			}
		}
	}
	if st.X == nil {
		return
	}
	if x.kind == kindBuild {
		if top && st.Kind == 'e' && st.X.Kind == starlark.Call {
			x.target(st.X)
		}
		return
	}
	walk(st.X, func(n *starlark.Node) {
		if n.Kind == starlark.Call {
			x.call(n, top)
		}
	})
}

// walk calls fn for n and every expression inside it.
func walk(n *starlark.Node, fn func(*starlark.Node)) {
	if n == nil {
		return
	}
	stack := []*starlark.Node{n}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		fn(n)
		if n.X != nil {
			stack = append(stack, n.X)
		}
		for _, a := range n.Args {
			if a.Val != nil {
				stack = append(stack, a.Val)
			}
		}
		for _, it := range n.Items {
			if it != nil {
				stack = append(stack, it)
			}
		}
	}
}

// target reads a BUILD file's top-level call: a target (its name is a symbol
// //dir:name of the rule's kind) and the labels its attributes name.
func (x *extractor) target(call *starlark.Node) {
	callee := call.Callee()
	if callee == "load" {
		return
	}
	if !notTargets[callee] {
		if name := call.Kw("name"); name != nil && name.Kind == starlark.String {
			pkg := x.dir
			if pkg == "." {
				pkg = ""
			}
			x.symbols.Add("//"+pkg+":"+name.Text, callee, name.Line)
		}
	}
	for _, a := range call.Args {
		if a.Name != "" && labelAttr(a.Name) {
			x.labels(a.Val)
		}
	}
}

// labels records the labels an attribute value names: strings, lists of them,
// concatenations, both branches of a conditional, every select() branch, glob()
// patterns, and rules_python's requirement() and rules_jvm_external's artifact().
func (x *extractor) labels(v *starlark.Node) {
	if v == nil {
		return
	}
	switch v.Kind {
	case starlark.String:
		if plausibleLabel(v.Text) {
			x.add(impLabel, v.Text, v.Text, v.Line)
		}
	case starlark.List, starlark.Tuple:
		for _, it := range v.Items {
			x.labels(it)
		}
	case starlark.Binary:
		if v.Text == "+" && v.Items[0].Kind != starlark.String && v.Items[1].Kind != starlark.String {
			x.labels(v.Items[0])
			x.labels(v.Items[1])
		}
	case starlark.Cond:
		x.labels(v.Items[0])
		x.labels(v.Items[2])
	case starlark.Call:
		switch callee := v.Callee(); x.original(callee) {
		case "glob":
			x.glob(v)
		case "select":
			if d := v.Pos(0); d != nil && d.Kind == starlark.Dict {
				for i := 1; i < len(d.Items); i += 2 {
					x.labels(d.Items[i])
				}
			}
		case "requirement":
			name, ok := v.Pos(0).Str()
			if l, loadedFrom := x.loads[callee]; ok && loadedFrom {
				if repo, _, ok := splitRepo(l.label); ok && repo != "" {
					x.add(impPip, "requirement("+name+")", repo+"\n"+name, v.Line)
				}
			}
		case "artifact", "maven_artifact":
			coord, ok := v.Pos(0).Str()
			if _, loadedFrom := x.loads[callee]; ok && loadedFrom {
				hub := v.KwStr("repository_name")
				if hub == "" {
					hub = "maven"
				}
				x.add(impMaven, "artifact("+coord+")", coord+"\n"+hub, v.Line)
			}
		}
	}
}

// plausibleLabel rejects attribute strings that cannot be labels: make variables,
// flags, empty strings.
func plausibleLabel(s string) bool {
	return s != "" && !strings.ContainsAny(s, "$ \t\n") && !strings.HasPrefix(s, "-")
}

// glob records a glob() call as one import, expanded by the resolver.
func (x *extractor) glob(call *starlark.Node) {
	include := call.Pos(0)
	if include == nil {
		include = call.Kw("include")
	}
	inc := include.Strings()
	exc := call.Kw("exclude").Strings()
	if len(inc) == 0 {
		return
	}
	spec := "glob(" + quoted(inc) + ")"
	if len(exc) > 0 {
		spec = "glob(" + quoted(inc) + ", exclude = " + quoted(exc) + ")"
	}
	x.add(impGlob, spec, strings.Join(inc, "\x00")+"\x01"+strings.Join(exc, "\x00"), call.Line)
}

func quoted(ss []string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = `"` + s + `"`
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// call reads a call anywhere in a .bzl, WORKSPACE or MODULE.bazel file.
func (x *extractor) call(n *starlark.Node, top bool) {
	callee := n.Callee()
	switch {
	case callee == "Label":
		if l, ok := n.Pos(0).Str(); ok {
			x.add(impLoad, l, l, n.Line)
		}
		return
	case x.kind == kindModule && top:
		if x.module(n, callee) {
			return
		}
	case x.kind == kindWorkspace && top && callee == "workspace":
		if name := n.Kw("name"); name != nil && name.Kind == starlark.String {
			x.symbols.Add(name.Text, "workspace", name.Line)
		}
		return
	}
	rule := x.original(callee)
	if rule == "maybe" { // maybe(http_archive, name = ...) from bazel_tools' utils.bzl
		rule = x.original(n.Pos(0).Name())
	}
	if r, ok := x.rules[callee]; ok {
		rule = r
	}
	switch {
	case repoRules[rule]:
		if name := n.KwStr("name"); name != "" {
			x.add(impRepo, name, name, n.Line)
		}
		x.lockLabels(n)
	case hubRules[rule]:
		if rule == "maven_install" {
			hub := n.KwStr("name")
			if hub == "" {
				hub = "maven"
			}
			x.artifacts(n, hub)
		}
		x.lockLabels(n)
	}
}

// lockLabels records the labels of project files a repository rule or module
// extension tag reads: lock files, requirements, go.mod, Cargo manifests.
func (x *extractor) lockLabels(n *starlark.Node) {
	for _, a := range n.Args {
		if a.Name == "" || a.Name == "name" {
			continue
		}
		var vals []*starlark.Node
		switch a.Val.Kind {
		case starlark.String:
			vals = []*starlark.Node{a.Val}
		case starlark.List, starlark.Tuple:
			vals = a.Val.Items
		case starlark.Dict: // requirements_by_platform = {"//:req.txt": "linux_*"}
			for i := 0; i < len(a.Val.Items); i += 2 {
				vals = append(vals, a.Val.Items[i])
			}
		}
		for _, v := range vals {
			if s, ok := v.Str(); ok && localLabel(s) {
				x.add(impLabel, s, s, v.Line)
			}
		}
	}
}

// localLabel reports whether s is a label of the main repository: //..., :...,
// @//..., @@//....
func localLabel(s string) bool {
	return strings.HasPrefix(s, "//") || strings.HasPrefix(s, "@//") || strings.HasPrefix(s, "@@//") ||
		strings.HasPrefix(s, ":") && len(s) > 1
}

// artifacts records maven_install's or maven.install's artifacts.
func (x *extractor) artifacts(n *starlark.Node, hub string) {
	for _, it := range listItems(n.Kw("artifacts")) {
		switch {
		case it.Kind == starlark.String:
			x.add(impMaven, it.Text, it.Text+"\n"+hub, it.Line)
		case it.Kind == starlark.Call && strings.HasSuffix(it.Callee(), "artifact"):
			// maven.artifact(group = ..., artifact = ..., version = ...) as a value
			if c := coordinate(it); c != "" {
				x.add(impMaven, c, c+"\n"+hub, it.Line)
			}
		}
	}
}

func listItems(n *starlark.Node) []*starlark.Node {
	if n == nil || n.Kind != starlark.List && n.Kind != starlark.Tuple {
		return nil
	}
	return n.Items
}

// coordinate is group:artifact:version of a call naming them by keyword.
func coordinate(n *starlark.Node) string {
	g, a := n.KwStr("group"), n.KwStr("artifact")
	if g == "" || a == "" {
		return ""
	}
	c := g + ":" + a
	if v := n.KwStr("version"); v != "" {
		c += ":" + v
	}
	return c
}

// module reads a MODULE.bazel call; false leaves it to call.
func (x *extractor) module(n *starlark.Node, callee string) bool {
	switch callee {
	case "module":
		if name := n.Kw("name"); name != nil && name.Kind == starlark.String {
			x.symbols.Add(name.Text, "module", name.Line)
		}
	case "bazel_dep":
		if name := n.KwStr("name"); name != "" {
			x.add(impDep, name, name, n.Line)
		}
	case "use_extension", "use_repo_rule", "include":
		if l, ok := n.Pos(0).Str(); ok {
			x.add(impLoad, l, l, n.Line)
		}
	case "register_toolchains", "register_execution_platforms":
		for _, a := range n.Args {
			if s, ok := a.Val.Str(); ok && a.Name == "" && !strings.HasSuffix(s, ":all") && !strings.HasSuffix(s, "/...") {
				x.add(impLabel, s, s, a.Val.Line)
			}
		}
	case "use_repo", "local_path_override", "git_override", "archive_override", "single_version_override",
		"multiple_version_override", "bazel_lib_override":
	default:
		obj, tag, ok := strings.Cut(callee, ".")
		ext, isExt := x.exts[obj]
		if !ok || !isExt {
			return false
		}
		x.tag(ext, tag, n)
	}
	return true
}

// tag reads a module extension's tag: the project files it reads, and the
// packages maven.install, go_deps.module and crate.spec name.
func (x *extractor) tag(ext, tag string, n *starlark.Node) {
	x.lockLabels(n)
	switch {
	case ext == "maven" && tag == "install":
		hub := n.KwStr("name")
		if hub == "" {
			hub = "maven"
		}
		x.artifacts(n, hub)
	case ext == "maven" && tag == "artifact":
		hub := n.KwStr("name")
		if hub == "" {
			hub = "maven"
		}
		if c := coordinate(n); c != "" {
			x.add(impMaven, c, c+"\n"+hub, n.Line)
		}
	case ext == "go_deps" && tag == "module":
		if p := n.KwStr("path"); p != "" {
			x.add(impGoMod, p, p, n.Line)
		}
	case ext == "crate" && tag == "spec":
		if p := n.KwStr("package"); p != "" {
			x.add(impCrate, p, p, n.Line)
		}
	}
}
