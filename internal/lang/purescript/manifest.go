package purescript

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one package a manifest lists: its name, the range it asks for
// ("" for a bare name, which the package set or the lock decides) and where.
type dependency struct {
	name, rng string
	line      int
	test      bool
	origin    string // a bower dependency installed from a git URL
}

// extraPackage is a package a workspace adds to or overrides in its package
// set: spago.yaml extraPackages, or a packages.dhall override or addition.
type extraPackage struct {
	name                 string
	version              string // a registry version (spago.yaml), or a git tag or commit (packages.dhall)
	git, ref, subdir     string
	path                 string // a local package, relative to the manifest's directory
	dependencies         []string
	line                 int
	dir                  string // the directory of the manifest that says so
	hasDeps, localAsRepo bool
}

// spagoYAML is what spago.yaml says: a package, a workspace, or both.
type spagoYAML struct {
	name      string
	nameLine  int
	isPackage bool
	deps      []dependency
	workspace bool
	set       string // the package set: "registry 60.0.0" or a URL's name
	extra     []*extraPackage
}

// yamlGet is a mapping node's value for key, or nil.
func yamlGet(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// yamlDoc is a YAML (or JSON) document's top node, or nil.
func yamlDoc(src []byte) *yaml.Node {
	var doc yaml.Node
	if yaml.Unmarshal(src, &doc) != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

// strings of a sequence node's scalars.
func yamlStrings(n *yaml.Node) []string {
	var out []string
	if n != nil && n.Kind == yaml.SequenceNode {
		for _, e := range n.Content {
			if e.Kind == yaml.ScalarNode {
				out = append(out, e.Value)
			}
		}
	}
	return out
}

// readSpagoYAML reads spago.yaml (spago 0.93 and later): package.name,
// package.dependencies and package.test.dependencies (a name, or name: range),
// workspace.packageSet (registry: 60.0.0 or url:) and workspace.extraPackages
// (a registry version, or git with ref and subdir, or path, with optional
// dependencies). Anything else - not YAML, neither package nor workspace - is nil.
//
// Implements: REQ-PURESCRIPT-005
func readSpagoYAML(src []byte) *spagoYAML {
	top := yamlDoc(src)
	pkg, ws := yamlGet(top, "package"), yamlGet(top, "workspace")
	if pkg == nil && ws == nil {
		return nil
	}
	out := &spagoYAML{isPackage: pkg != nil, workspace: ws != nil}
	if n := yamlGet(pkg, "name"); n != nil && n.Kind == yaml.ScalarNode {
		out.name, out.nameLine = n.Value, n.Line
	}
	readDeps := func(n *yaml.Node, test bool) {
		if n == nil || n.Kind != yaml.SequenceNode {
			return
		}
		for _, e := range n.Content {
			switch e.Kind {
			case yaml.ScalarNode:
				out.deps = append(out.deps, dependency{name: e.Value, line: e.Line, test: test})
			case yaml.MappingNode:
				for i := 0; i+1 < len(e.Content); i += 2 {
					rng := ""
					if v := e.Content[i+1]; v.Kind == yaml.ScalarNode && v.Value != "*" {
						rng = v.Value
					}
					out.deps = append(out.deps, dependency{name: e.Content[i].Value, rng: rng, line: e.Content[i].Line, test: test})
				}
			}
		}
	}
	readDeps(yamlGet(pkg, "dependencies"), false)
	readDeps(yamlGet(yamlGet(pkg, "test"), "dependencies"), true)
	if set := yamlGet(ws, "packageSet"); set != nil {
		if v := yamlGet(set, "registry"); v != nil && v.Value != "" {
			out.set = "registry " + v.Value
		} else if v := yamlGet(set, "url"); v != nil && v.Value != "" {
			out.set = setName(&dval{kind: dImport, loc: v.Value})
		}
	}
	if extra := yamlGet(ws, "extraPackages"); extra != nil && extra.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(extra.Content); i += 2 {
			k, v := extra.Content[i], extra.Content[i+1]
			e := &extraPackage{name: k.Value, line: k.Line}
			switch v.Kind {
			case yaml.ScalarNode:
				e.version = v.Value
			case yaml.MappingNode:
				for _, f := range []struct {
					key string
					to  *string
				}{{"git", &e.git}, {"ref", &e.ref}, {"subdir", &e.subdir}, {"path", &e.path}, {"version", &e.version}} {
					if n := yamlGet(v, f.key); n != nil && n.Kind == yaml.ScalarNode {
						*f.to = n.Value
					}
				}
				if d := yamlGet(v, "dependencies"); d != nil {
					e.hasDeps = true
					for _, n := range d.Content {
						if n.Kind == yaml.ScalarNode {
							e.dependencies = append(e.dependencies, n.Value)
						} else if n.Kind == yaml.MappingNode && len(n.Content) > 0 {
							e.dependencies = append(e.dependencies, n.Content[0].Value)
						}
					}
				}
			}
			out.extra = append(out.extra, e)
		}
	}
	return out
}

// lockPackage is one package spago.lock records.
type lockPackage struct {
	name         string
	typ          string // registry, git or local
	version      string
	url, rev     string
	subdir, path string
	dependencies []string
	line         int
}

// spagoLock is spago.lock: its packages, and the workspace's own packages with
// their directories.
type spagoLock struct {
	packages map[string]*lockPackage
	locals   map[string]string // workspace package -> its path
	set      string
}

// readLock reads spago.lock, which spago writes as YAML or (since 0.93.30) JSON:
// packages.<name> with type, version (registry), url and rev (git) or path
// (local), and dependencies; workspace.packages.<name>.path.
//
// Implements: REQ-PURESCRIPT-005
func readLock(src []byte) *spagoLock {
	top := yamlDoc(src)
	pkgs := yamlGet(top, "packages")
	if pkgs == nil || pkgs.Kind != yaml.MappingNode {
		return nil
	}
	out := &spagoLock{packages: map[string]*lockPackage{}, locals: map[string]string{}}
	for i := 0; i+1 < len(pkgs.Content); i += 2 {
		k, v := pkgs.Content[i], pkgs.Content[i+1]
		p := &lockPackage{name: k.Value, line: k.Line}
		for _, f := range []struct {
			key string
			to  *string
		}{{"type", &p.typ}, {"version", &p.version}, {"url", &p.url}, {"rev", &p.rev}, {"subdir", &p.subdir}, {"path", &p.path}} {
			if n := yamlGet(v, f.key); n != nil && n.Kind == yaml.ScalarNode {
				*f.to = n.Value
			}
		}
		p.dependencies = yamlStrings(yamlGet(v, "dependencies"))
		if p.name != "" {
			out.packages[p.name] = p
		}
	}
	ws := yamlGet(top, "workspace")
	if locals := yamlGet(ws, "packages"); locals != nil && locals.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(locals.Content); i += 2 {
			if p := yamlGet(locals.Content[i+1], "path"); p != nil {
				out.locals[locals.Content[i].Value] = p.Value
			}
		}
	}
	if a := yamlGet(yamlGet(yamlGet(ws, "package_set"), "address"), "registry"); a != nil && a.Value != "" {
		out.set = "registry " + a.Value
	}
	return out
}

// bowerManifest is what a legacy bower.json says about PureScript packages.
type bowerManifest struct {
	name string
	deps []dependency
}

// readBower reads bower.json's dependencies and devDependencies named
// purescript-*, as the registry names them (without the prefix). A version is a
// range as written ("^v6.0.0"); a git URL keeps its ref after `#`.
//
// Implements: REQ-PURESCRIPT-005
func readBower(src []byte) *bowerManifest {
	var raw struct {
		Name    string            `json:"name"`
		Deps    map[string]string `json:"dependencies"`
		DevDeps map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(src, &raw) != nil {
		return nil
	}
	out := &bowerManifest{name: strings.TrimPrefix(raw.Name, "purescript-")}
	lines := stringLines(src)
	for _, sec := range []struct {
		m    map[string]string
		test bool
	}{{raw.Deps, false}, {raw.DevDeps, true}} {
		for _, k := range sortedKeys(sec.m) {
			name, ok := strings.CutPrefix(k, "purescript-")
			if !ok || name == "" {
				continue
			}
			d := dependency{name: name, rng: strings.TrimSpace(sec.m[k]), line: lines[k], test: sec.test}
			if u, ref, ok := strings.Cut(d.rng, "#"); ok && (strings.Contains(u, "://") || strings.HasPrefix(u, "git@")) {
				d.origin, d.rng = u, ref
			}
			out.deps = append(out.deps, d)
		}
	}
	if len(out.deps) == 0 && out.name == raw.Name {
		return nil // not a PureScript package
	}
	return out
}

// dhallConfig is a spago.dhall configuration: a record with dependencies and
// sources (and the package set under packages).
func dhallConfig(v *dval) bool {
	return v.kind == dRecord && (v.fields["dependencies"] != nil || v.fields["sources"] != nil)
}

// dhallExtras are the packages a package set record adds or overrides:
// { name = { dependencies, repo, version } }, or mkPackage's record.
func dhallExtras(set *dval, dir string) []*extraPackage {
	if set == nil || set.kind != dRecord {
		return nil
	}
	var out []*extraPackage
	for _, k := range set.keys {
		v := set.fields[k]
		if v.kind != dRecord {
			continue
		}
		e := &extraPackage{name: k, dir: dir, line: v.line}
		if deps := v.field("dependencies"); deps.kind == dList {
			e.hasDeps = true
			for _, t := range deps.texts() {
				e.dependencies = append(e.dependencies, t.text)
			}
		}
		if repo := v.field("repo"); repo.kind == dText {
			e.line = repo.line
			if strings.Contains(repo.text, "://") || strings.HasPrefix(repo.text, "git@") {
				e.git = repo.text
			} else if repo.text != "" {
				e.path, e.localAsRepo = repo.text, true
			}
		}
		if ver := v.field("version"); ver.kind == dText {
			e.version = ver.text
			if e.line == 0 {
				e.line = ver.line
			}
		}
		out = append(out, e)
	}
	return out
}

// extractSpagoYAML makes each dependency of the package (test dependencies
// too) and each extra package of the workspace an import; the package's name is
// its symbol.
//
// Implements: REQ-PURESCRIPT-005
func extractSpagoYAML(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	m := readSpagoYAML(src)
	if m == nil {
		return ex
	}
	seen := map[string]bool{}
	for _, d := range m.deps {
		if d.name != "" && !seen[d.name] {
			seen[d.name] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: d.name, Module: d.name, Name: kindDep, Line: d.line})
		}
	}
	for _, e := range m.extra {
		if e.name != "" && !seen[e.name] {
			seen[e.name] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: e.name, Module: e.name, Name: kindExtra, Line: e.line})
		}
	}
	if m.name != "" {
		ex.Symbols = []lang.Symbol{{Name: m.name, Kind: "package", Line: m.nameLine}}
	}
	return ex
}

// extractLock makes every package spago.lock records an import of it.
//
// Implements: REQ-PURESCRIPT-005
func extractLock(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	l := readLock(src)
	if l == nil {
		return ex
	}
	for _, name := range sortedKeys(l.packages) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kindLock, Line: l.packages[name].line})
	}
	return ex
}

// extractDhall reads a spago.dhall configuration (its dependencies, as far as
// the file itself lists them, are imports; its name a symbol) or a
// packages.dhall package set (each package it adds or overrides is an import).
// Imported files are not read here: the extraction depends on the content
// alone.
//
// Implements: REQ-PURESCRIPT-005
func extractDhall(src []byte, set bool) *lang.Extraction {
	ex := &lang.Extraction{}
	v := evalDhall(src, ".", nil)
	if set {
		for _, e := range dhallExtras(v, ".") {
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: e.name, Module: e.name, Name: kindExtra, Line: e.line})
		}
		return ex
	}
	if !dhallConfig(v) {
		return ex
	}
	seen := map[string]bool{}
	for _, t := range v.field("dependencies").texts() {
		if t.text != "" && !seen[t.text] {
			seen[t.text] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: t.text, Module: t.text, Name: kindDep, Line: t.line})
		}
	}
	if n := v.field("name"); n.kind == dText && n.text != "" {
		ex.Symbols = []lang.Symbol{{Name: n.text, Kind: "package", Line: n.line}}
	}
	return ex
}

// extractBower makes each purescript-* dependency of bower.json an import.
//
// Implements: REQ-PURESCRIPT-005
func extractBower(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	m := readBower(src)
	if m == nil {
		return ex
	}
	for _, d := range m.deps {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "purescript-" + d.name, Module: d.name, Name: kindDep, Line: d.line})
	}
	return ex
}

var jsonString = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

// stringLines is the first line each JSON string is written on.
func stringLines(src []byte) map[string]int {
	out := map[string]int{}
	for i, l := range strings.Split(string(src), "\n") {
		for _, m := range jsonString.FindAllStringSubmatch(l, -1) {
			if _, ok := out[m[1]]; !ok {
				out[m[1]] = i + 1
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// gitName names a package installed from git by its repository (and the
// subdirectory it lives in), as other plugins name git dependencies.
func gitName(url, subdir string) string {
	n := lang.RepoName(url)
	if s := strings.Trim(path.Clean("/"+subdir), "/"); s != "" && subdir != "" {
		n += "/" + s
	}
	return n
}
