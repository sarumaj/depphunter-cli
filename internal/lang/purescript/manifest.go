package purescript

import (
	"encoding/json"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/dhall"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
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

// readSpagoYAML reads spago.yaml (spago 0.93 and later): package.name,
// package.dependencies and package.test.dependencies (a name, or name: range),
// workspace.packageSet (registry: 60.0.0 or url:) and workspace.extraPackages
// (a registry version, or git with reference and subdirectory, or path, with optional
// dependencies). Anything else - not YAML, neither package nor workspace - is nil.
//
// Implements: REQ-PURESCRIPT-005
func readSpagoYAML(source []byte) *spagoYAML {
	top := yamlnode.Parse(source)
	packageNode, workspaceNode := yamlnode.Get(top, "package"), yamlnode.Get(top, "workspace")
	if packageNode == nil && workspaceNode == nil {
		return nil
	}
	out := &spagoYAML{isPackage: packageNode != nil, workspace: workspaceNode != nil}
	if n := yamlnode.Get(packageNode, "name"); n != nil && n.Kind == yaml.ScalarNode {
		out.name, out.nameLine = n.Value, n.Line
	}
	readDependencies := func(n *yaml.Node, test bool) {
		for _, e := range yamlnode.Items(n) {
			if e.Kind == yaml.ScalarNode {
				out.dependencies = append(out.dependencies, dependency{name: e.Value, line: e.Line, test: test})
			}
			for _, entry := range yamlnode.Pairs(e) {
				versionRange := ""
				if v := entry.Value; v.Kind == yaml.ScalarNode && v.Value != "*" {
					versionRange = v.Value
				}
				out.dependencies = append(out.dependencies, dependency{name: entry.Key.Value, versionRange: versionRange, line: entry.Key.Line, test: test})
			}
		}
	}
	readDependencies(yamlnode.Get(packageNode, "dependencies"), false)
	readDependencies(yamlnode.Get(packageNode, "test", "dependencies"), true)
	if set := yamlnode.Get(workspaceNode, "packageSet"); set != nil {
		if v := yamlnode.Get(set, "registry"); v != nil && v.Value != "" {
			out.set = "registry " + v.Value
		} else if v := yamlnode.Get(set, "url"); v != nil && v.Value != "" {
			out.set = setName(&dhall.Value{Kind: dhall.KindImport, Location: v.Value})
		}
	}
	for _, entry := range yamlnode.Pairs(yamlnode.Get(workspaceNode, "extraPackages")) {
		v := entry.Value
		e := &extraPackage{name: entry.Key.Value, line: entry.Key.Line}
		switch v.Kind {
		case yaml.ScalarNode:
			e.version = v.Value
		case yaml.MappingNode:
			for _, f := range []struct {
				key string
				to  *string
			}{{"git", &e.git}, {"ref", &e.reference}, {"subdir", &e.subdirectory}, {"path", &e.path}, {"version", &e.version}} {
				*f.to = yamlnode.Scalar(v, f.key)
			}
			if d := yamlnode.Get(v, "dependencies"); d != nil {
				e.hasDependencies = true
				// As a package's own: "- prelude" or "- prelude: >=6.0.0", aliases
				// resolved like any other value.
				for _, n := range yamlnode.Items(d) {
					if n.Kind == yaml.ScalarNode {
						e.dependencies = append(e.dependencies, n.Value)
					}
					for _, entry := range yamlnode.Pairs(n) {
						e.dependencies = append(e.dependencies, entry.Key.Value)
					}
				}
			}
		}
		out.extra = append(out.extra, e)
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
	top := yamlnode.Parse(source)
	packages := yamlnode.Mapping(yamlnode.Get(top, "packages"))
	if packages == nil {
		return nil
	}
	out := &spagoLock{packages: map[string]*lockPackage{}, locals: map[string]string{}}
	for _, entry := range yamlnode.Pairs(packages) {
		v := entry.Value
		p := &lockPackage{name: entry.Key.Value, line: entry.Key.Line}
		for _, f := range []struct {
			key string
			to  *string
		}{{"type", &p.typeName}, {"version", &p.version}, {"url", &p.url}, {"rev", &p.rev}, {"subdir", &p.subdirectory}, {"path", &p.path}} {
			*f.to = yamlnode.Scalar(v, f.key)
		}
		for _, dependency := range yamlnode.Items(yamlnode.Get(v, "dependencies")) {
			if dependency.Kind == yaml.ScalarNode {
				p.dependencies = append(p.dependencies, dependency.Value)
			}
		}
		if p.name != "" {
			out.packages[p.name] = p
		}
	}
	workspaceNode := yamlnode.Get(top, "workspace")
	for _, local := range yamlnode.Pairs(yamlnode.Get(workspaceNode, "packages")) {
		if p := yamlnode.Get(local.Value, "path"); p != nil {
			out.locals[local.Key.Value] = p.Value
		}
	}
	if a := yamlnode.Get(workspaceNode, "package_set", "address", "registry"); a != nil && a.Value != "" {
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
	lines := lang.JSONStringLines(source)
	for _, section := range []struct {
		m    map[string]string
		test bool
	}{{raw.Dependencies, false}, {raw.DevDependencies, true}} {
		for _, k := range lang.SortedKeys(section.m) {
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
	for _, name := range lang.SortedKeys(l.packages) {
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

// gitName names a package installed from git by its repository (and the
// subdirectory it lives in), as other plugins name git dependencies.
func gitName(url, subdirectory string) string {
	n := lang.RepositoryName(url)
	if s := strings.Trim(path.Clean("/"+subdirectory), "/"); s != "" && subdirectory != "" {
		n += "/" + s
	}
	return n
}
