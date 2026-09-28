package bazel

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/starlark"
)

// Import kinds (RawImport.Name).
const (
	importLoad       = "load"  // a label load(), use_extension(), use_repo_rule(), include() or Label() names
	importLabel      = "label" // a label an attribute names
	importGlob       = "glob"  // Module: include patterns joined by \x00, \x01, exclude patterns
	importDependency = "dep"   // a bazel_dep; Module = the module's name
	importRepository = "repo"  // a repository rule's declaration; Module = the repository's name
	importMaven      = "maven" // a Maven coordinate; Module = coordinate "\n" hub repository
	importPip        = "pip"   // requirement("x"); Module = hub repository "\n" distribution
	importGoMod      = "gomod" // go_deps.module(path = ...); Module = the module path
	importCrate      = "crate" // crate.spec(package = ...); Module = the crate
)

// repositoryRules are the repository rules that declare one external repository by
// name; hubRules those that declare a hub of packages of another ecosystem.
var (
	repositoryRules = map[string]bool{
		"http_archive": true, "http_file": true, "http_jar": true, "git_repository": true,
		"new_git_repository": true, "local_repository": true, "new_local_repository": true,
		"go_repository": true,
	}
	hubRules = map[string]bool{
		"maven_install": true, "pip_parse": true, "pip_install": true, "npm_translate_lock": true,
		"crates_repository": true,
	}
)

// labelAttributes are the rule attributes that hold labels, besides any attribute
// whose name ends in deps, srcs or hdrs.
var labelAttributes = map[string]bool{
	"srcs": true, "hdrs": true, "textual_hdrs": true, "data": true, "deps": true, "runtime_deps": true,
	"implementation_deps": true, "interface_deps": true, "exports": true, "proto": true, "protos": true,
	"embed": true, "plugins": true, "exported_plugins": true, "resources": true, "resource_jars": true,
	"main": true, "actual": true, "tools": true, "tests": true, "src": true, "srcs_jars": true,
	"library": true, "binary": true, "additional_linker_inputs": true, "linker_script": true,
	"win_def_file": true, "module_map": true, "entry_point": true, "entry_points": true,
	"compatible_with": false, "target_compatible_with": false,
}

func labelAttribute(name string) bool {
	if v, ok := labelAttributes[name]; ok {
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
	kind       string
	directory  string
	loads      map[string]loaded
	extensions map[string]string // MODULE.bazel extension proxies: variable -> extension name
	rules      map[string]string // MODULE.bazel use_repo_rule: variable -> rule name
	extraction *lang.Extraction
	symbols    lang.SymbolSet
	seen       map[string]bool
}

// extract reads what one Starlark file declares and names.
func extract(kind, directory string, source []byte) *lang.Extraction {
	f := starlark.Parse(source)
	x := &extractor{kind: kind, directory: directory, loads: map[string]loaded{}, extensions: map[string]string{},
		rules: map[string]string{}, extraction: &lang.Extraction{}, seen: map[string]bool{}}
	for _, statement := range f.Statements {
		if statement.Definition == "" && statement.Kind == 'e' && statement.X.Callee() == "load" {
			x.load(statement.X)
		}
	}
	for _, statement := range f.Statements {
		x.statement(statement)
	}
	x.extraction.Symbols = x.symbols.List()
	if x.extraction.Symbols == nil {
		x.extraction.Symbols = []lang.Symbol{}
	}
	return x.extraction
}

func (x *extractor) add(kind, spec, module string, line int) {
	if spec == "" || x.seen[spec] {
		return
	}
	x.seen[spec] = true
	x.extraction.Imports = append(x.extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

// load records a load() statement's label and the names it binds.
func (x *extractor) load(call *starlark.Node) {
	label, ok := call.Position(0).StringValue()
	if !ok {
		return
	}
	x.add(importLoad, label, label, call.Line)
	for i, a := range call.Arguments {
		if i == 0 || a.Star != "" {
			continue
		}
		name, ok := a.Value.StringValue()
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

func (x *extractor) statement(statement starlark.Statement) {
	top := statement.Definition == ""
	switch {
	case statement.Kind == 'd' && top:
		x.symbols.Add(statement.Name, "func", statement.Line)
		return
	case statement.Kind == 'a' && top && x.kind != kindBuild:
		callee := statement.X.Callee()
		if x.kind == kindModule {
			switch callee {
			case "use_extension":
				if extension, ok := statement.X.Position(1).StringValue(); ok && len(statement.Targets) == 1 {
					x.extensions[statement.Targets[0]] = extension
				}
			case "use_repo_rule":
				if rule, ok := statement.X.Position(1).StringValue(); ok && len(statement.Targets) == 1 {
					x.rules[statement.Targets[0]] = rule
				}
			}
		} else if statement.Column == 0 {
			kind := symbolKinds[callee]
			if kind == "" {
				kind = "var"
			}
			for _, t := range statement.Targets {
				x.symbols.Add(t, kind, statement.Line)
			}
		}
	}
	if statement.X == nil {
		return
	}
	if x.kind == kindBuild {
		if top && statement.Kind == 'e' && statement.X.Kind == starlark.Call {
			x.target(statement.X)
		}
		return
	}
	walk(statement.X, func(n *starlark.Node) {
		if n.Kind == starlark.Call {
			x.call(n, top)
		}
	})
}

// walk calls function for n and every expression inside it.
func walk(n *starlark.Node, function func(*starlark.Node)) {
	if n == nil {
		return
	}
	stack := []*starlark.Node{n}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		function(n)
		if n.X != nil {
			stack = append(stack, n.X)
		}
		for _, a := range n.Arguments {
			if a.Value != nil {
				stack = append(stack, a.Value)
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
		if name := call.Keyword("name"); name != nil && name.Kind == starlark.String {
			packageName := x.directory
			if packageName == "." {
				packageName = ""
			}
			x.symbols.Add("//"+packageName+":"+name.Text, callee, name.Line)
		}
	}
	for _, a := range call.Arguments {
		if a.Name != "" && labelAttribute(a.Name) {
			x.labels(a.Value)
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
			x.add(importLabel, v.Text, v.Text, v.Line)
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
	case starlark.Condition:
		x.labels(v.Items[0])
		x.labels(v.Items[2])
	case starlark.Call:
		switch callee := v.Callee(); x.original(callee) {
		case "glob":
			x.glob(v)
		case "select":
			if d := v.Position(0); d != nil && d.Kind == starlark.Dictionary {
				for i := 1; i < len(d.Items); i += 2 {
					x.labels(d.Items[i])
				}
			}
		case "requirement":
			name, ok := v.Position(0).StringValue()
			if l, loadedFrom := x.loads[callee]; ok && loadedFrom {
				if repository, _, ok := splitRepository(l.label); ok && repository != "" {
					x.add(importPip, "requirement("+name+")", repository+"\n"+name, v.Line)
				}
			}
		case "artifact", "maven_artifact":
			coordinate, ok := v.Position(0).StringValue()
			if _, loadedFrom := x.loads[callee]; ok && loadedFrom {
				hub := v.KeywordString("repository_name")
				if hub == "" {
					hub = "maven"
				}
				x.add(importMaven, "artifact("+coordinate+")", coordinate+"\n"+hub, v.Line)
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
	include := call.Position(0)
	if include == nil {
		include = call.Keyword("include")
	}
	includeList := include.Strings()
	excluded := call.Keyword("exclude").Strings()
	if len(includeList) == 0 {
		return
	}
	spec := "glob(" + quoted(includeList) + ")"
	if len(excluded) > 0 {
		spec = "glob(" + quoted(includeList) + ", exclude = " + quoted(excluded) + ")"
	}
	x.add(importGlob, spec, strings.Join(includeList, "\x00")+"\x01"+strings.Join(excluded, "\x00"), call.Line)
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
		if l, ok := n.Position(0).StringValue(); ok {
			x.add(importLoad, l, l, n.Line)
		}
		return
	case x.kind == kindModule && top:
		if x.module(n, callee) {
			return
		}
	case x.kind == kindWorkspace && top && callee == "workspace":
		if name := n.Keyword("name"); name != nil && name.Kind == starlark.String {
			x.symbols.Add(name.Text, "workspace", name.Line)
		}
		return
	}
	rule := x.original(callee)
	if rule == "maybe" { // maybe(http_archive, name = ...) from bazel_tools' utils.bzl
		rule = x.original(n.Position(0).Name())
	}
	if r, ok := x.rules[callee]; ok {
		rule = r
	}
	switch {
	case repositoryRules[rule]:
		if name := n.KeywordString("name"); name != "" {
			x.add(importRepository, name, name, n.Line)
		}
		x.lockLabels(n)
	case hubRules[rule]:
		if rule == "maven_install" {
			hub := n.KeywordString("name")
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
	for _, a := range n.Arguments {
		if a.Name == "" || a.Name == "name" {
			continue
		}
		var values []*starlark.Node
		switch a.Value.Kind {
		case starlark.String:
			values = []*starlark.Node{a.Value}
		case starlark.List, starlark.Tuple:
			values = a.Value.Items
		case starlark.Dictionary: // requirements_by_platform = {"//:req.txt": "linux_*"}
			for i := 0; i < len(a.Value.Items); i += 2 {
				values = append(values, a.Value.Items[i])
			}
		}
		for _, v := range values {
			if s, ok := v.StringValue(); ok && localLabel(s) {
				x.add(importLabel, s, s, v.Line)
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
	for _, it := range listItems(n.Keyword("artifacts")) {
		switch {
		case it.Kind == starlark.String:
			x.add(importMaven, it.Text, it.Text+"\n"+hub, it.Line)
		case it.Kind == starlark.Call && strings.HasSuffix(it.Callee(), "artifact"):
			// maven.artifact(group = ..., artifact = ..., version = ...) as a value
			if c := coordinate(it); c != "" {
				x.add(importMaven, c, c+"\n"+hub, it.Line)
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
	g, a := n.KeywordString("group"), n.KeywordString("artifact")
	if g == "" || a == "" {
		return ""
	}
	c := g + ":" + a
	if v := n.KeywordString("version"); v != "" {
		c += ":" + v
	}
	return c
}

// module reads a MODULE.bazel call; false leaves it to call.
func (x *extractor) module(n *starlark.Node, callee string) bool {
	switch callee {
	case "module":
		if name := n.Keyword("name"); name != nil && name.Kind == starlark.String {
			x.symbols.Add(name.Text, "module", name.Line)
		}
	case "bazel_dep":
		if name := n.KeywordString("name"); name != "" {
			x.add(importDependency, name, name, n.Line)
		}
	case "use_extension", "use_repo_rule", "include":
		if l, ok := n.Position(0).StringValue(); ok {
			x.add(importLoad, l, l, n.Line)
		}
	case "register_toolchains", "register_execution_platforms":
		for _, a := range n.Arguments {
			if s, ok := a.Value.StringValue(); ok && a.Name == "" && !strings.HasSuffix(s, ":all") && !strings.HasSuffix(s, "/...") {
				x.add(importLabel, s, s, a.Value.Line)
			}
		}
	case "use_repo", "local_path_override", "git_override", "archive_override", "single_version_override",
		"multiple_version_override", "bazel_lib_override":
	default:
		object, tag, ok := strings.Cut(callee, ".")
		extension, isExtension := x.extensions[object]
		if !ok || !isExtension {
			return false
		}
		x.tag(extension, tag, n)
	}
	return true
}

// tag reads a module extension's tag: the project files it reads, and the
// packages maven.install, go_deps.module and crate.spec name.
func (x *extractor) tag(extension, tag string, n *starlark.Node) {
	x.lockLabels(n)
	switch {
	case extension == "maven" && tag == "install":
		hub := n.KeywordString("name")
		if hub == "" {
			hub = "maven"
		}
		x.artifacts(n, hub)
	case extension == "maven" && tag == "artifact":
		hub := n.KeywordString("name")
		if hub == "" {
			hub = "maven"
		}
		if c := coordinate(n); c != "" {
			x.add(importMaven, c, c+"\n"+hub, n.Line)
		}
	case extension == "go_deps" && tag == "module":
		if p := n.KeywordString("path"); p != "" {
			x.add(importGoMod, p, p, n.Line)
		}
	case extension == "crate" && tag == "spec":
		if p := n.KeywordString("package"); p != "" {
			x.add(importCrate, p, p, n.Line)
		}
	}
}
