package haskell

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
)

// hpackDependencies reads an hpack dependencies value: a list of "name constraint" strings
// or of {name, version} maps, or a map of name to constraint (or to {version}).
func hpackDependencies(n *yaml.Node) []dependency {
	var out []dependency
	add := func(name, c string, line int) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		text := strings.TrimSpace(name + " " + strings.Join(strings.Fields(c), " "))
		if d, ok := parseDependency(cline{text: text, line: line}); ok {
			out = append(out, d)
		}
	}
	switch {
	case n == nil:
	case n.Kind == yaml.ScalarNode:
		for _, e := range splitList([]cline{{text: n.Value, line: n.Line}}) {
			add(e.text, "", e.line)
		}
	case n.Kind == yaml.SequenceNode:
		for _, c := range yamlnode.Items(n) {
			switch c.Kind {
			case yaml.ScalarNode:
				add(c.Value, "", c.Line)
			case yaml.MappingNode:
				if name := yamlnode.Get(c, "name"); name != nil {
					v := ""
					if version := yamlnode.Get(c, "version"); version != nil {
						v = version.Value
					}
					add(name.Value, v, c.Line)
				}
			}
		}
	case n.Kind == yaml.MappingNode:
		for _, entry := range yamlnode.Pairs(n) {
			k, v := entry.Key, entry.Value
			c := v.Value
			if v.Kind == yaml.MappingNode {
				c = ""
				if version := yamlnode.Get(v, "version"); version != nil {
					c = version.Value
				}
			}
			add(k.Value, c, k.Line)
		}
	}
	return out
}

// hpackSections are the keys of package.yaml holding components: one (library) or a
// map of them by name.
var hpackSections = []struct {
	key, kind string
	named     bool
}{
	{"library", "library", false},
	{"internal-libraries", "library", true},
	{"executables", "executable", true},
	{"executable", "executable", false},
	{"tests", "test-suite", true},
	{"benchmarks", "benchmark", true},
}

// readHpack reads an hpack package.yaml as a package description: top-level
// source-dirs and dependencies apply to every component, and each component adds its
// own, conditional (when:) ones included. Without source-dirs a component's sources
// are in the package directory.
//
// Implements: REQ-HASKELL-004, REQ-HASKELL-006, REQ-LANG-032
func readHpack(source []byte, file string) *cabalPackage {
	doc := yamlnode.Mapping(yamlnode.Parse(source))
	if doc == nil {
		return nil
	}
	p := &cabalPackage{file: file, directory: path.Dir(file)}
	if n := yamlnode.Get(doc, "name"); n != nil {
		p.name = n.Value
	}
	if v := yamlnode.Get(doc, "version"); v != nil {
		p.version = v.Value
	}
	// An anchor can make a when: contain itself, so each node is read once per component.
	type visit struct {
		component *component
		node      *yaml.Node
	}
	visited := map[visit]bool{}
	var fill func(c *component, m *yaml.Node)
	fill = func(c *component, m *yaml.Node) {
		if m == nil || visited[visit{c, m}] {
			return
		}
		visited[visit{c, m}] = true
		for _, s := range yamlnode.Scalars(yamlnode.Get(m, "source-dirs")) {
			c.directories = append(c.directories, path.Clean(s.Value))
		}
		for _, key := range []string{"exposed-modules", "other-modules"} {
			for _, s := range yamlnode.Scalars(yamlnode.Get(m, key)) {
				c.modules = append(c.modules, s.Value)
			}
		}
		c.dependencies = append(c.dependencies, hpackDependencies(yamlnode.Get(m, "dependencies"))...)
		when := yamlnode.Get(m, "when")
		if when != nil && when.Kind == yaml.MappingNode {
			fill(c, when)
			for _, k := range []string{"then", "else"} {
				fill(c, yamlnode.Get(when, k))
			}
		} else if when != nil && when.Kind == yaml.SequenceNode {
			for _, w := range yamlnode.Items(when) {
				fill(c, w)
				for _, k := range []string{"then", "else"} {
					fill(c, yamlnode.Get(w, k))
				}
			}
		}
	}
	if customSetup := yamlnode.Get(doc, "custom-setup"); customSetup != nil {
		p.setup = &component{kind: "custom-setup", line: customSetup.Line, dependencies: hpackDependencies(yamlnode.Get(customSetup, "dependencies"))}
	}
	common := &component{}
	fill(common, doc)
	for _, s := range hpackSections {
		n := yamlnode.Get(doc, s.key)
		if n == nil || n.Kind != yaml.MappingNode {
			continue
		}
		var list [][2]*yaml.Node
		if s.named {
			for _, entry := range yamlnode.Pairs(n) {
				list = append(list, [2]*yaml.Node{entry.Key, entry.Value})
			}
		} else {
			list = append(list, [2]*yaml.Node{nil, n})
		}
		for _, e := range list {
			c := &component{kind: s.kind, line: n.Line}
			if e[0] != nil {
				c.name, c.line = e[0].Value, e[0].Line
			}
			fill(c, e[1])
			c.directories = append(c.directories, common.directories...)
			c.dependencies = append(c.dependencies, common.dependencies...)
			if len(c.directories) == 0 {
				c.directories = []string{"."}
			}
			p.comps = append(p.comps, c)
		}
	}
	if len(p.comps) == 0 && (len(common.dependencies) > 0 || len(common.directories) > 0) {
		if len(common.directories) == 0 {
			common.directories = []string{"."}
		}
		common.kind = "library"
		p.comps = append(p.comps, common)
	}
	return p
}

// extractHpack turns every dependencies entry of package.yaml into an import.
//
// Implements: REQ-HASKELL-005
func extractHpack(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	p := readHpack(source, "package.yaml")
	if p == nil {
		return extraction
	}
	seen := map[string]bool{}
	comps := p.comps
	if p.setup != nil {
		comps = append(comps, p.setup)
	}
	for _, c := range comps {
		for _, d := range c.dependencies {
			spec := "dependencies: " + d.text
			if !seen[spec] {
				seen[spec] = true
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: d.name, Name: kindDependency, Line: d.line})
			}
		}
	}
	sort.SliceStable(extraction.Imports, func(i, j int) bool { return extraction.Imports[i].Line < extraction.Imports[j].Line })
	return extraction
}

// extraDependency is one stack.yaml extra-deps entry: a Hackage package at a version, a
// repository at a commit, or a local directory.
type extraDependency struct {
	name, version string
	origin        string // a repository URL; "path:<dir>" for a local one
	line          int
}

// stackProject is what stack.yaml says: the snapshot, the local packages, the
// extra-deps.
type stackProject struct {
	snapshot string
	members  []cline
	extras   []extraDependency
}

var hackageID = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*?)-([0-9]+(?:\.[0-9]+)*)(?:@.*)?$`)

// splitID splits "aeson-2.2.1.0@sha256:...,1234" into name and version.
func splitID(s string) (string, string, bool) {
	m := hackageID.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// readStack reads stack.yaml: resolver or snapshot, packages, and extra-deps (name-
// version with an optional @sha256 or @rev, {git|github, commit, subdirs}, a local
// path).
//
// Implements: REQ-HASKELL-007
func readStack(source []byte) *stackProject {
	doc := yamlnode.Mapping(yamlnode.Parse(source))
	if doc == nil {
		return nil
	}
	p := &stackProject{}
	for _, key := range []string{"snapshot", "resolver"} {
		if n := yamlnode.Get(doc, key); n != nil && n.Kind == yaml.ScalarNode && p.snapshot == "" {
			p.snapshot = n.Value
		}
	}
	for _, n := range yamlnode.Scalars(yamlnode.Get(doc, "packages")) {
		p.members = append(p.members, cline{text: n.Value, line: n.Line})
	}
	for _, e := range yamlnode.Items(yamlnode.Get(doc, "extra-deps")) {
		switch e.Kind {
		case yaml.ScalarNode:
			v := e.Value
			if name, version, ok := splitID(v); ok && !strings.Contains(v, "/") {
				p.extras = append(p.extras, extraDependency{name: name, version: version, line: e.Line})
			} else if strings.HasPrefix(v, ".") || strings.HasPrefix(v, "/") || !strings.Contains(v, "://") {
				p.extras = append(p.extras, extraDependency{name: path.Base(path.Clean(v)), origin: "path:" + v, line: e.Line})
			} else {
				p.extras = append(p.extras, extraDependency{name: archiveName(v), origin: v, line: e.Line})
			}
		case yaml.MappingNode:
			gitNode := ""
			if g := yamlnode.Get(e, "git"); g != nil {
				gitNode = g.Value
			} else if g := yamlnode.Get(e, "github"); g != nil {
				gitNode = "https://github.com/" + g.Value
			} else if u := yamlnode.Get(e, "url"); u != nil {
				p.extras = append(p.extras, extraDependency{name: archiveName(u.Value), origin: u.Value, line: e.Line})
				continue
			}
			if gitNode == "" {
				continue
			}
			commit := ""
			if c := yamlnode.Get(e, "commit"); c != nil {
				commit = c.Value
			}
			subs := yamlnode.Scalars(yamlnode.Get(e, "subdirs"))
			if len(subs) == 0 {
				p.extras = append(p.extras, extraDependency{name: repositoryName(gitNode, ""), version: commit, origin: gitNode, line: e.Line})
			}
			for _, s := range subs {
				p.extras = append(p.extras, extraDependency{name: repositoryName(gitNode, s.Value), version: commit, origin: gitNode, line: s.Line})
			}
		}
	}
	return p
}

// archiveName is the package an archive URL holds: name-1.2.3.tar.gz, else the
// file's name.
func archiveName(u string) string {
	base := path.Base(u)
	for _, extension := range []string{".tar.gz", ".tgz", ".zip", ".tar"} {
		base = strings.TrimSuffix(base, extension)
	}
	if name, _, ok := splitID(base); ok {
		return name
	}
	return base
}

// extractStack turns stack.yaml's packages and extra-deps into imports.
//
// Implements: REQ-HASKELL-005
func extractStack(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	p := readStack(source)
	if p == nil {
		return extraction
	}
	seen := map[string]bool{}
	for _, m := range p.members {
		spec := "packages: " + m.text
		if !seen[spec] {
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: m.text, Name: kindMember, Line: m.line})
		}
	}
	for _, e := range p.extras {
		spec := "extra-deps: " + e.name
		if e.version != "" && e.origin == "" {
			spec += "-" + e.version
		}
		if !seen[spec] {
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: e.name, Name: kindExtra, Line: e.line})
		}
	}
	return extraction
}

// stackLock is what stack.yaml.lock completed: each extra-dep's exact version (a
// Hackage one) or commit (a repository), and the snapshot.
type stackLock struct {
	packages map[string]extraDependency
}

// readStackLock reads stack.yaml.lock's packages: completed entries, "hackage:
// name-version@sha256:...,size" or name, version, git and commit.
//
// Implements: REQ-HASKELL-008
func readStackLock(source []byte) *stackLock {
	doc := yamlnode.Mapping(yamlnode.Parse(source))
	if doc == nil {
		return nil
	}
	l := &stackLock{packages: map[string]extraDependency{}}
	for _, e := range yamlnode.Items(yamlnode.Get(doc, "packages")) {
		c := yamlnode.Get(e, "completed")
		if c == nil {
			continue
		}
		if h := yamlnode.Get(c, "hackage"); h != nil {
			if name, version, ok := splitID(h.Value); ok {
				l.packages[name] = extraDependency{name: name, version: version}
			}
			continue
		}
		name, version, gitNode := yamlnode.Get(c, "name"), yamlnode.Get(c, "commit"), yamlnode.Get(c, "git")
		if name == nil {
			continue
		}
		d := extraDependency{name: name.Value}
		if gitNode != nil {
			d.origin = gitNode.Value
		}
		if version != nil {
			d.version = version.Value
		} else if v := yamlnode.Get(c, "version"); v != nil {
			d.version = v.Value
		}
		l.packages[d.name] = d
	}
	return l
}
