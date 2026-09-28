package purescript

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/dhall"
)

// dependency is one package a manifest lists: its name, the range it asks for
// ("" for a bare name, which the package set or the lock decides) and where.
type dependency struct {
	name, versionRange string
	line               int
	test               bool
	origin             string // a bower dependency installed from a git URL
}

// extraPackage is a package a workspace adds to or overrides in its package
// set: spago.yaml extraPackages, or a packages.dhall override or addition.
type extraPackage struct {
	name                               string
	version                            string // a registry version (spago.yaml), or a git tag or commit (packages.dhall)
	git, reference, subdirectory       string
	path                               string // a local package, relative to the manifest's directory
	dependencies                       []string
	line                               int
	directory                          string // the directory of the manifest that says so
	hasDependencies, localAsRepository bool
}

// spagoYAML is what spago.yaml says: a package, a workspace, or both.
type spagoYAML struct {
	name         string
	nameLine     int
	isPackage    bool
	dependencies []dependency
	workspace    bool
	set          string // the package set: "registry 60.0.0" or a URL's name
	extra        []*extraPackage
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
func yamlDoc(source []byte) *yaml.Node {
	var doc yaml.Node
	if yaml.Unmarshal(source, &doc) != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
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
// (a registry version, or git with reference and subdirectory, or path, with optional
// dependencies). Anything else - not YAML, neither package nor workspace - is nil.
//
// Implements: REQ-PURESCRIPT-005
func readSpagoYAML(source []byte) *spagoYAML {
	top := yamlDoc(source)
	packageNode, workspaceNode := yamlGet(top, "package"), yamlGet(top, "workspace")
	if packageNode == nil && workspaceNode == nil {
		return nil
	}
	out := &spagoYAML{isPackage: packageNode != nil, workspace: workspaceNode != nil}
	if n := yamlGet(packageNode, "name"); n != nil && n.Kind == yaml.ScalarNode {
		out.name, out.nameLine = n.Value, n.Line
	}
	readDependencies := func(n *yaml.Node, test bool) {
		if n == nil || n.Kind != yaml.SequenceNode {
			return
		}
		for _, e := range n.Content {
			switch e.Kind {
			case yaml.ScalarNode:
				out.dependencies = append(out.dependencies, dependency{name: e.Value, line: e.Line, test: test})
			case yaml.MappingNode:
				for i := 0; i+1 < len(e.Content); i += 2 {
					versionRange := ""
					if v := e.Content[i+1]; v.Kind == yaml.ScalarNode && v.Value != "*" {
						versionRange = v.Value
					}
					out.dependencies = append(out.dependencies, dependency{name: e.Content[i].Value, versionRange: versionRange, line: e.Content[i].Line, test: test})
				}
			}
		}
	}
	readDependencies(yamlGet(packageNode, "dependencies"), false)
	readDependencies(yamlGet(yamlGet(packageNode, "test"), "dependencies"), true)
	if set := yamlGet(workspaceNode, "packageSet"); set != nil {
		if v := yamlGet(set, "registry"); v != nil && v.Value != "" {
			out.set = "registry " + v.Value
		} else if v := yamlGet(set, "url"); v != nil && v.Value != "" {
			out.set = setName(&dhall.Value{Kind: dhall.KindImport, Location: v.Value})
		}
	}
	if extra := yamlGet(workspaceNode, "extraPackages"); extra != nil && extra.Kind == yaml.MappingNode {
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
				}{{"git", &e.git}, {"ref", &e.reference}, {"subdir", &e.subdirectory}, {"path", &e.path}, {"version", &e.version}} {
					if n := yamlGet(v, f.key); n != nil && n.Kind == yaml.ScalarNode {
						*f.to = n.Value
					}
				}
				if d := yamlGet(v, "dependencies"); d != nil {
					e.hasDependencies = true
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
	name               string
	typeName           string // registry, git or local
	version            string
	url, rev           string
	subdirectory, path string
	dependencies       []string
	line               int
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
func readLock(source []byte) *spagoLock {
	top := yamlDoc(source)
	packages := yamlGet(top, "packages")
	if packages == nil || packages.Kind != yaml.MappingNode {
		return nil
	}
	out := &spagoLock{packages: map[string]*lockPackage{}, locals: map[string]string{}}
	for i := 0; i+1 < len(packages.Content); i += 2 {
		k, v := packages.Content[i], packages.Content[i+1]
		p := &lockPackage{name: k.Value, line: k.Line}
		for _, f := range []struct {
			key string
			to  *string
		}{{"type", &p.typeName}, {"version", &p.version}, {"url", &p.url}, {"rev", &p.rev}, {"subdir", &p.subdirectory}, {"path", &p.path}} {
			if n := yamlGet(v, f.key); n != nil && n.Kind == yaml.ScalarNode {
				*f.to = n.Value
			}
		}
		p.dependencies = yamlStrings(yamlGet(v, "dependencies"))
		if p.name != "" {
			out.packages[p.name] = p
		}
	}
	workspaceNode := yamlGet(top, "workspace")
	if locals := yamlGet(workspaceNode, "packages"); locals != nil && locals.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(locals.Content); i += 2 {
			if p := yamlGet(locals.Content[i+1], "path"); p != nil {
				out.locals[locals.Content[i].Value] = p.Value
			}
		}
	}
	if a := yamlGet(yamlGet(yamlGet(workspaceNode, "package_set"), "address"), "registry"); a != nil && a.Value != "" {
		out.set = "registry " + a.Value
	}
	return out
}

// bowerManifest is what a legacy bower.json says about PureScript packages.
type bowerManifest struct {
	name         string
	dependencies []dependency
}

// readBower reads bower.json's dependencies and devDependencies named
// purescript-*, as the registry names them (without the prefix). A version is a
// range as written ("^v6.0.0"); a git URL keeps its ref after `#`.
//
// Implements: REQ-PURESCRIPT-005
func readBower(source []byte) *bowerManifest {
	var raw struct {
		Name            string            `json:"name"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(source, &raw) != nil {
		return nil
	}
	out := &bowerManifest{name: strings.TrimPrefix(raw.Name, "purescript-")}
	lines := stringLines(source)
	for _, section := range []struct {
		m    map[string]string
		test bool
	}{{raw.Dependencies, false}, {raw.DevDependencies, true}} {
		for _, k := range sortedKeys(section.m) {
			name, ok := strings.CutPrefix(k, "purescript-")
			if !ok || name == "" {
				continue
			}
			d := dependency{name: name, versionRange: strings.TrimSpace(section.m[k]), line: lines[k], test: section.test}
			if u, reference, ok := strings.Cut(d.versionRange, "#"); ok && (strings.Contains(u, "://") || strings.HasPrefix(u, "git@")) {
				d.origin, d.versionRange = u, reference
			}
			out.dependencies = append(out.dependencies, d)
		}
	}
	if len(out.dependencies) == 0 && out.name == raw.Name {
		return nil // not a PureScript package
	}
	return out
}

// dhallConfig is a spago.dhall configuration: a record with dependencies and
// sources (and the package set under packages).
func dhallConfig(v *dhall.Value) bool {
	return v.Kind == dhall.KindRecord && (v.Fields["dependencies"] != nil || v.Fields["sources"] != nil)
}

// dhallExtras are the packages a package set record adds or overrides:
// { name = { dependencies, repo, version } }, or mkPackage's record.
func dhallExtras(set *dhall.Value, directory string) []*extraPackage {
	if set == nil || set.Kind != dhall.KindRecord {
		return nil
	}
	var out []*extraPackage
	for _, k := range set.Keys {
		v := set.Fields[k]
		if v.Kind != dhall.KindRecord {
			continue
		}
		e := &extraPackage{name: k, directory: directory, line: v.Line}
		if dependencies := v.Field("dependencies"); dependencies.Kind == dhall.KindList {
			e.hasDependencies = true
			for _, t := range dependencies.Texts() {
				e.dependencies = append(e.dependencies, t.Text)
			}
		}
		if repository := v.Field("repo"); repository.Kind == dhall.KindText {
			e.line = repository.Line
			if strings.Contains(repository.Text, "://") || strings.HasPrefix(repository.Text, "git@") {
				e.git = repository.Text
			} else if repository.Text != "" {
				e.path, e.localAsRepository = repository.Text, true
			}
		}
		if version := v.Field("version"); version.Kind == dhall.KindText {
			e.version = version.Text
			if e.line == 0 {
				e.line = version.Line
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
func extractSpagoYAML(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	m := readSpagoYAML(source)
	if m == nil {
		return extraction
	}
	seen := map[string]bool{}
	for _, d := range m.dependencies {
		if d.name != "" && !seen[d.name] {
			seen[d.name] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: d.name, Module: d.name, Name: kindDependency, Line: d.line})
		}
	}
	for _, e := range m.extra {
		if e.name != "" && !seen[e.name] {
			seen[e.name] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: e.name, Module: e.name, Name: kindExtra, Line: e.line})
		}
	}
	if m.name != "" {
		extraction.Symbols = []lang.Symbol{{Name: m.name, Kind: "package", Line: m.nameLine}}
	}
	return extraction
}

// extractLock makes every package spago.lock records an import of it.
//
// Implements: REQ-PURESCRIPT-005
func extractLock(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	l := readLock(source)
	if l == nil {
		return extraction
	}
	for _, name := range sortedKeys(l.packages) {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kindLock, Line: l.packages[name].line})
	}
	return extraction
}

// extractDhall reads a spago.dhall configuration (its dependencies, as far as
// the file itself lists them, are imports; its name a symbol) or a
// packages.dhall package set (each package it adds or overrides is an import).
// Imported files are not read here: the extraction depends on the content
// alone.
//
// Implements: REQ-PURESCRIPT-005
func extractDhall(source []byte, set bool) *lang.Extraction {
	extraction := &lang.Extraction{}
	v := dhall.Eval(source, ".", nil)
	if set {
		for _, e := range dhallExtras(v, ".") {
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: e.name, Module: e.name, Name: kindExtra, Line: e.line})
		}
		return extraction
	}
	if !dhallConfig(v) {
		return extraction
	}
	seen := map[string]bool{}
	for _, t := range v.Field("dependencies").Texts() {
		if t.Text != "" && !seen[t.Text] {
			seen[t.Text] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: t.Text, Module: t.Text, Name: kindDependency, Line: t.Line})
		}
	}
	if n := v.Field("name"); n.Kind == dhall.KindText && n.Text != "" {
		extraction.Symbols = []lang.Symbol{{Name: n.Text, Kind: "package", Line: n.Line}}
	}
	return extraction
}

// extractBower makes each purescript-* dependency of bower.json an import.
//
// Implements: REQ-PURESCRIPT-005
func extractBower(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	m := readBower(source)
	if m == nil {
		return extraction
	}
	for _, d := range m.dependencies {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "purescript-" + d.name, Module: d.name, Name: kindDependency, Line: d.line})
	}
	return extraction
}

var jsonString = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

// stringLines is the first line each JSON string is written on.
func stringLines(source []byte) map[string]int {
	out := map[string]int{}
	for i, l := range strings.Split(string(source), "\n") {
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
func gitName(url, subdirectory string) string {
	n := lang.RepositoryName(url)
	if s := strings.Trim(path.Clean("/"+subdirectory), "/"); s != "" && subdirectory != "" {
		n += "/" + s
	}
	return n
}
