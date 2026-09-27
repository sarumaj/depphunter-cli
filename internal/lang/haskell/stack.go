package haskell

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// yamlDoc parses a YAML file to its top mapping node; nil when it is not one.
func yamlDoc(src []byte) *yaml.Node {
	var doc yaml.Node
	if yaml.Unmarshal(src, &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return doc.Content[0]
}

// get is the value of key in a mapping node.
func get(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// scalars are a node's strings: the scalar itself, or a sequence's scalars.
func scalars(n *yaml.Node) []*yaml.Node {
	switch {
	case n == nil:
		return nil
	case n.Kind == yaml.ScalarNode:
		return []*yaml.Node{n}
	case n.Kind == yaml.SequenceNode:
		var out []*yaml.Node
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				out = append(out, c)
			}
		}
		return out
	}
	return nil
}

// hpackDeps reads an hpack dependencies value: a list of "name constraint" strings
// or of {name, version} maps, or a map of name to constraint (or to {version}).
func hpackDeps(n *yaml.Node) []dep {
	var out []dep
	add := func(name, c string, line int) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		text := strings.TrimSpace(name + " " + strings.Join(strings.Fields(c), " "))
		if d, ok := parseDep(cline{text: text, line: line}); ok {
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
		for _, c := range n.Content {
			switch c.Kind {
			case yaml.ScalarNode:
				add(c.Value, "", c.Line)
			case yaml.MappingNode:
				if name := get(c, "name"); name != nil {
					v := ""
					if ver := get(c, "version"); ver != nil {
						v = ver.Value
					}
					add(name.Value, v, c.Line)
				}
			}
		}
	case n.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			c := v.Value
			if v.Kind == yaml.MappingNode {
				c = ""
				if ver := get(v, "version"); ver != nil {
					c = ver.Value
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
// Implements: REQ-HASKELL-004, REQ-HASKELL-006
func readHpack(src []byte, file string) *cabalPkg {
	doc := yamlDoc(src)
	if doc == nil {
		return nil
	}
	p := &cabalPkg{file: file, dir: path.Dir(file)}
	if n := get(doc, "name"); n != nil {
		p.name = n.Value
	}
	if v := get(doc, "version"); v != nil {
		p.version = v.Value
	}
	var fill func(c *component, m *yaml.Node)
	fill = func(c *component, m *yaml.Node) {
		for _, s := range scalars(get(m, "source-dirs")) {
			c.dirs = append(c.dirs, path.Clean(s.Value))
		}
		for _, key := range []string{"exposed-modules", "other-modules"} {
			for _, s := range scalars(get(m, key)) {
				c.modules = append(c.modules, s.Value)
			}
		}
		c.deps = append(c.deps, hpackDeps(get(m, "dependencies"))...)
		when := get(m, "when")
		if when != nil && when.Kind == yaml.MappingNode {
			fill(c, when)
			for _, k := range []string{"then", "else"} {
				fill(c, get(when, k))
			}
		} else if when != nil && when.Kind == yaml.SequenceNode {
			for _, w := range when.Content {
				fill(c, w)
				for _, k := range []string{"then", "else"} {
					fill(c, get(w, k))
				}
			}
		}
	}
	if cs := get(doc, "custom-setup"); cs != nil {
		p.setup = &component{kind: "custom-setup", line: cs.Line, deps: hpackDeps(get(cs, "dependencies"))}
	}
	common := &component{}
	fill(common, doc)
	for _, s := range hpackSections {
		n := get(doc, s.key)
		if n == nil || n.Kind != yaml.MappingNode {
			continue
		}
		var list [][2]*yaml.Node
		if s.named {
			for i := 0; i+1 < len(n.Content); i += 2 {
				list = append(list, [2]*yaml.Node{n.Content[i], n.Content[i+1]})
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
			c.dirs = append(c.dirs, common.dirs...)
			c.deps = append(c.deps, common.deps...)
			if len(c.dirs) == 0 {
				c.dirs = []string{"."}
			}
			p.comps = append(p.comps, c)
		}
	}
	if len(p.comps) == 0 && (len(common.deps) > 0 || len(common.dirs) > 0) {
		if len(common.dirs) == 0 {
			common.dirs = []string{"."}
		}
		common.kind = "library"
		p.comps = append(p.comps, common)
	}
	return p
}

// extractHpack turns every dependencies entry of package.yaml into an import.
//
// Implements: REQ-HASKELL-005
func extractHpack(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	p := readHpack(src, "package.yaml")
	if p == nil {
		return ex
	}
	seen := map[string]bool{}
	comps := p.comps
	if p.setup != nil {
		comps = append(comps, p.setup)
	}
	for _, c := range comps {
		for _, d := range c.deps {
			spec := "dependencies: " + d.text
			if !seen[spec] {
				seen[spec] = true
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: d.name, Name: kindDep, Line: d.line})
			}
		}
	}
	sort.SliceStable(ex.Imports, func(i, j int) bool { return ex.Imports[i].Line < ex.Imports[j].Line })
	return ex
}

// extraDep is one stack.yaml extra-deps entry: a Hackage package at a version, a
// repository at a commit, or a local directory.
type extraDep struct {
	name, version string
	origin        string // a repository URL; "path:<dir>" for a local one
	line          int
}

// stackProject is what stack.yaml says: the snapshot, the local packages, the
// extra-deps.
type stackProject struct {
	snapshot string
	members  []cline
	extras   []extraDep
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
func readStack(src []byte) *stackProject {
	doc := yamlDoc(src)
	if doc == nil {
		return nil
	}
	p := &stackProject{}
	for _, key := range []string{"snapshot", "resolver"} {
		if n := get(doc, key); n != nil && n.Kind == yaml.ScalarNode && p.snapshot == "" {
			p.snapshot = n.Value
		}
	}
	for _, n := range scalars(get(doc, "packages")) {
		p.members = append(p.members, cline{text: n.Value, line: n.Line})
	}
	extras := get(doc, "extra-deps")
	if extras == nil || extras.Kind != yaml.SequenceNode {
		return p
	}
	for _, e := range extras.Content {
		switch e.Kind {
		case yaml.ScalarNode:
			v := e.Value
			if name, ver, ok := splitID(v); ok && !strings.Contains(v, "/") {
				p.extras = append(p.extras, extraDep{name: name, version: ver, line: e.Line})
			} else if strings.HasPrefix(v, ".") || strings.HasPrefix(v, "/") || !strings.Contains(v, "://") {
				p.extras = append(p.extras, extraDep{name: path.Base(path.Clean(v)), origin: "path:" + v, line: e.Line})
			} else {
				p.extras = append(p.extras, extraDep{name: archiveName(v), origin: v, line: e.Line})
			}
		case yaml.MappingNode:
			loc := ""
			if g := get(e, "git"); g != nil {
				loc = g.Value
			} else if g := get(e, "github"); g != nil {
				loc = "https://github.com/" + g.Value
			} else if u := get(e, "url"); u != nil {
				p.extras = append(p.extras, extraDep{name: archiveName(u.Value), origin: u.Value, line: e.Line})
				continue
			}
			if loc == "" {
				continue
			}
			commit := ""
			if c := get(e, "commit"); c != nil {
				commit = c.Value
			}
			subs := scalars(get(e, "subdirs"))
			if len(subs) == 0 {
				p.extras = append(p.extras, extraDep{name: repoName(loc, ""), version: commit, origin: loc, line: e.Line})
			}
			for _, s := range subs {
				p.extras = append(p.extras, extraDep{name: repoName(loc, s.Value), version: commit, origin: loc, line: s.Line})
			}
		}
	}
	return p
}

// archiveName is the package an archive URL holds: name-1.2.3.tar.gz, else the
// file's name.
func archiveName(u string) string {
	base := path.Base(u)
	for _, ext := range []string{".tar.gz", ".tgz", ".zip", ".tar"} {
		base = strings.TrimSuffix(base, ext)
	}
	if name, _, ok := splitID(base); ok {
		return name
	}
	return base
}

// extractStack turns stack.yaml's packages and extra-deps into imports.
//
// Implements: REQ-HASKELL-005
func extractStack(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	p := readStack(src)
	if p == nil {
		return ex
	}
	seen := map[string]bool{}
	for _, m := range p.members {
		spec := "packages: " + m.text
		if !seen[spec] {
			seen[spec] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: m.text, Name: kindMember, Line: m.line})
		}
	}
	for _, e := range p.extras {
		spec := "extra-deps: " + e.name
		if e.version != "" && e.origin == "" {
			spec += "-" + e.version
		}
		if !seen[spec] {
			seen[spec] = true
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: e.name, Name: kindExtra, Line: e.line})
		}
	}
	return ex
}

// stackLock is what stack.yaml.lock completed: each extra-dep's exact version (a
// Hackage one) or commit (a repository), and the snapshot.
type stackLock struct {
	pkgs map[string]extraDep
}

// readStackLock reads stack.yaml.lock's packages: completed entries, "hackage:
// name-version@sha256:...,size" or name, version, git and commit.
//
// Implements: REQ-HASKELL-008
func readStackLock(src []byte) *stackLock {
	doc := yamlDoc(src)
	if doc == nil {
		return nil
	}
	l := &stackLock{pkgs: map[string]extraDep{}}
	pkgs := get(doc, "packages")
	if pkgs == nil || pkgs.Kind != yaml.SequenceNode {
		return l
	}
	for _, e := range pkgs.Content {
		c := get(e, "completed")
		if c == nil {
			continue
		}
		if h := get(c, "hackage"); h != nil {
			if name, ver, ok := splitID(h.Value); ok {
				l.pkgs[name] = extraDep{name: name, version: ver}
			}
			continue
		}
		name, ver, loc := get(c, "name"), get(c, "commit"), get(c, "git")
		if name == nil {
			continue
		}
		d := extraDep{name: name.Value}
		if loc != nil {
			d.origin = loc.Value
		}
		if ver != nil {
			d.version = ver.Value
		} else if v := get(c, "version"); v != nil {
			d.version = v.Value
		}
		l.pkgs[d.name] = d
	}
	return l
}
