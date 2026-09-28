package haskell

import (
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// cline is one line of a field's value in a cabal-format file.
type cline struct {
	text string
	line int
}

// cnode is a field or a section of a cabal-format file (a .cabal package description,
// cabal.project, cabal.project.freeze). A field has a key (lower case) and a value that
// continues on the lines indented deeper than the key; a section (library,
// executable foo, if flag(x), source-repository-package) has the fields and sections
// indented under it.
type cnode struct {
	key    string
	value  []cline
	head   string // a section's header line, words single-spaced
	line   int
	indent int
	kids   []*cnode
}

func (n *cnode) text() string {
	parts := make([]string, 0, len(n.value))
	for _, l := range n.value {
		if t := strings.TrimSpace(l.text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

// word is a section header's first word, lower case: library, executable, if, ...
func (n *cnode) word() string {
	w, _, _ := strings.Cut(n.head, " ")
	return strings.ToLower(w)
}

// argument is what follows the section header's first word: an executable's name.
func (n *cnode) argument() string {
	_, a, _ := strings.Cut(n.head, " ")
	return strings.TrimSpace(a)
}

var cabalKey = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*)\s*:(.*)$`)

// parseCabal reads a cabal-format file into its top-level fields and sections. Braces
// are not read (layout is how these files are written); -- comment lines are skipped.
func parseCabal(source []byte) []*cnode {
	root := &cnode{indent: -1}
	stack := []*cnode{root}
	var field *cnode
	for i, raw := range strings.Split(string(source), "\n") {
		raw = strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		indent := 0
		for _, c := range raw {
			if c == ' ' {
				indent++
			} else if c == '\t' {
				indent = indent/8*8 + 8
			} else {
				break
			}
		}
		if field != nil && indent > field.indent {
			field.value = append(field.value, cline{text: trimmed, line: i + 1})
			continue
		}
		field = nil
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		if trimmed == "{" || trimmed == "}" {
			continue
		}
		if m := cabalKey.FindStringSubmatch(trimmed); m != nil {
			field = &cnode{key: strings.ToLower(m[1]), line: i + 1, indent: indent,
				value: []cline{{text: strings.TrimSpace(m[2]), line: i + 1}}}
			parent.kids = append(parent.kids, field)
			continue
		}
		section := &cnode{head: strings.Join(strings.Fields(strings.TrimSuffix(trimmed, "{")), " "), line: i + 1, indent: indent}
		parent.kids = append(parent.kids, section)
		stack = append(stack, section)
	}
	return root.kids
}

// dependency is one entry of build-depends (or of hpack's dependencies): "aeson >=2.0 && <2.3".
type dependency struct {
	name       string
	constraint string // normalized; an exact ==1.2.3 is the bare 1.2.3
	text       string // as written, spaces single
	line       int
}

var dependencyEntry = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*)\s*(?::\s*(\{[^}]*\}|[A-Za-z0-9-]+))?\s*(.*)$`)

// splitList splits a field's value on the commas outside braces, each entry with the
// line it starts on (build-depends: pkg:{a, b} is one entry).
func splitList(lines []cline) []cline {
	var out []cline
	var current strings.Builder
	line, depth := 0, 0
	flush := func() {
		if t := strings.Join(strings.Fields(current.String()), " "); t != "" {
			out = append(out, cline{text: t, line: line})
		}
		current.Reset()
		line = 0
	}
	for _, l := range lines {
		for _, r := range l.text {
			switch {
			case r == '{':
				depth++
			case r == '}':
				depth--
			case r == ',' && depth <= 0:
				flush()
				continue
			}
			if line == 0 && r != ' ' && r != '\t' {
				line = l.line
			}
			current.WriteRune(r)
		}
		current.WriteByte(' ')
	}
	flush()
	return out
}

// parseDependency reads one dependency entry.
func parseDependency(e cline) (dependency, bool) {
	m := dependencyEntry.FindStringSubmatch(e.text)
	if m == nil {
		return dependency{}, false
	}
	return dependency{name: m[1], constraint: normalizeConstraint(m[3]), text: e.text, line: e.line}, true
}

// normalizeConstraint writes a version range with single spaces, "" for none (-any, >=0),
// and a single version (==1.2.3, == 1.2.3) bare, as lang.Pinned reads a pin.
func normalizeConstraint(c string) string {
	c = strings.Join(strings.Fields(c), " ")
	switch c {
	case "", "-any", "any", ">=0", ">= 0":
		return ""
	}
	if v, ok := strings.CutPrefix(c, "=="); ok {
		v = strings.TrimSpace(v)
		if lang.Pinned(v) {
			return v
		}
	}
	return c
}

// component is a library, executable, test suite, benchmark or common stanza.
type component struct {
	kind, name   string
	directories  []string // hs-source-dirs, relative to the package directory
	modules      []string // exposed-modules, other-modules, signatures
	dependencies []dependency
	imports      []string // common stanzas it imports
	line         int
}

// cabalPackage is a package description: a .cabal file, or an hpack package.yaml.
type cabalPackage struct {
	name, version   string
	file, directory string
	comps           []*component
	setup           *component // custom-setup: what Setup.hs is built with
}

// fields splits a list field (hs-source-dirs, exposed-modules) on commas and spaces.
func fields(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '"' })
}

// readCabal reads a .cabal package description: its name and version, and every
// component with its source directories, modules and build-depends, the conditional
// blocks included and common stanzas merged into the components importing them.
//
// Implements: REQ-HASKELL-004, REQ-HASKELL-006
func readCabal(source []byte, file string) *cabalPackage {
	p := &cabalPackage{file: file, directory: path.Dir(file)}
	commons := map[string]*component{}
	top := &component{kind: "library"}
	var fill func(c *component, nodes []*cnode)
	fill = func(c *component, nodes []*cnode) {
		for _, n := range nodes {
			switch {
			case n.head != "":
				fill(c, n.kids) // if/else blocks
			case n.key == "hs-source-dirs":
				c.directories = append(c.directories, fields(n.text())...)
			case n.key == "exposed-modules" || n.key == "other-modules" || n.key == "signatures":
				c.modules = append(c.modules, fields(n.text())...)
			case n.key == "build-depends":
				for _, e := range splitList(n.value) {
					if d, ok := parseDependency(e); ok {
						c.dependencies = append(c.dependencies, d)
					}
				}
			case n.key == "import":
				c.imports = append(c.imports, fields(n.text())...)
			}
		}
	}
	var topFields []*cnode
	for _, n := range parseCabal(source) {
		switch {
		case n.key == "name":
			p.name = n.text()
		case n.key == "version":
			p.version = n.text()
		case n.key != "":
			topFields = append(topFields, n)
		default:
			kind := n.word()
			switch kind {
			case "library", "executable", "test-suite", "benchmark", "foreign-library", "common":
			case "custom-setup":
				p.setup = &component{kind: kind, line: n.line}
				for _, k := range n.kids {
					if k.key == "setup-depends" {
						for _, e := range splitList(k.value) {
							if d, ok := parseDependency(e); ok {
								p.setup.dependencies = append(p.setup.dependencies, d)
							}
						}
					}
				}
				continue
			default:
				continue // flag, source-repository
			}
			c := &component{kind: kind, name: n.argument(), line: n.line}
			fill(c, n.kids)
			if kind == "common" {
				commons[c.name] = c
				continue
			}
			p.comps = append(p.comps, c)
		}
	}
	// A description from before sections has its build-depends at the top level.
	fill(top, topFields)
	if len(top.dependencies) > 0 || len(top.modules) > 0 {
		p.comps = append(p.comps, top)
	}
	var merge func(c *component, seen map[string]bool)
	merge = func(c *component, seen map[string]bool) {
		for _, name := range c.imports {
			common := commons[name]
			if common == nil || seen[name] {
				continue
			}
			seen[name] = true
			merge(common, seen)
			c.directories = append(c.directories, common.directories...)
			c.modules = append(c.modules, common.modules...)
			c.dependencies = append(c.dependencies, common.dependencies...)
		}
	}
	for _, c := range p.comps {
		merge(c, map[string]bool{})
		if len(c.directories) == 0 {
			c.directories = []string{"."}
		}
		for i, d := range c.directories {
			c.directories[i] = path.Clean(d)
		}
	}
	return p
}

// kinds of manifest imports, in RawImport.Name.
const (
	kindDependency = "dep"    // build-depends, hpack dependencies
	kindMember     = "member" // cabal.project / stack.yaml packages
	kindRepository = "repo"   // cabal.project source-repository-package
	kindExtra      = "extra"  // stack.yaml extra-deps
)

// kindInclude is a cabal.project `import:` of another project file.
const kindInclude = "include"

// extractCabal turns a package description's build-depends into imports of the
// packages they name (once per entry as written) and its components into symbols.
//
// Implements: REQ-HASKELL-005
func extractCabal(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	seen := map[string]bool{}
	var walk func(nodes []*cnode)
	walk = func(nodes []*cnode) {
		for _, n := range nodes {
			if n.head != "" {
				walk(n.kids)
				continue
			}
			if n.key != "build-depends" && n.key != "setup-depends" {
				continue
			}
			for _, e := range splitList(n.value) {
				d, ok := parseDependency(e)
				if !ok {
					continue
				}
				spec := n.key + ": " + d.text
				if seen[spec] {
					continue
				}
				seen[spec] = true
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: d.name, Name: kindDependency, Line: d.line})
			}
		}
	}
	nodes := parseCabal(source)
	walk(nodes)
	for _, n := range nodes {
		switch n.word() {
		case "library", "executable", "test-suite", "benchmark", "foreign-library", "common":
			symbols.Add(n.head, "component", n.line)
		}
	}
	extraction.Symbols = symbols.List()
	return extraction
}

// srp is a source-repository-package of cabal.project: a package built from a
// repository at a tag or commit.
type srp struct {
	name, location, tag string
	line                int
}

// cabalProject is what cabal.project (and cabal.project.freeze) say.
type cabalProject struct {
	members      []cline // packages: and optional-packages: entries
	imports      []cline // import: other project files (a path or a URL)
	repositories []srp
	constraints  map[string]string // package -> the version ==x pins it to, else the range
}

// readCabalProject reads cabal.project or its freeze file: packages, the
// project files it imports, the source-repository-package stanzas and the constraints (any.aeson ==2.2.1.0),
// conditional blocks included. Setup-dependency constraints (setup.Cabal) and flag or
// "installed" constraints are left out.
//
// Implements: REQ-HASKELL-007, REQ-HASKELL-008
func readCabalProject(source []byte) *cabalProject {
	p := &cabalProject{constraints: map[string]string{}}
	var walk func(nodes []*cnode)
	walk = func(nodes []*cnode) {
		for _, n := range nodes {
			switch {
			case n.key == "packages" || n.key == "optional-packages":
				for _, l := range n.value {
					for _, f := range globFields(l.text) {
						p.members = append(p.members, cline{text: strings.Trim(f, `'`), line: l.line})
					}
				}
			case n.key == "import":
				for _, l := range n.value {
					for _, f := range fields(l.text) {
						p.imports = append(p.imports, cline{text: f, line: l.line})
					}
				}
			case n.key == "constraints":
				for _, e := range splitList(n.value) {
					name, c, ok := constraint(e.text)
					if ok {
						if _, duplicate := p.constraints[name]; !duplicate {
							p.constraints[name] = c
						}
					}
				}
			case n.word() == "source-repository-package":
				r := srp{line: n.line}
				var subdirectory string
				for _, k := range n.kids {
					switch k.key {
					case "location":
						r.location = k.text()
					case "tag":
						r.tag = k.text()
					case "subdir":
						if f := fields(k.text()); len(f) > 0 {
							subdirectory = f[0]
						}
					}
				}
				r.name = repositoryName(r.location, subdirectory)
				if r.name != "" {
					p.repositories = append(p.repositories, r)
				}
			case n.head != "":
				walk(n.kids)
			}
		}
	}
	walk(parseCabal(source))
	return p
}

// constraint reads one cabal constraint: [any.|qualifier.]name (==v | range).
func constraint(e string) (string, string, bool) {
	e = strings.TrimSpace(e)
	i := strings.IndexAny(e, " <>=^!")
	if i < 0 {
		return "", "", false
	}
	name, rest := e[:i], strings.TrimSpace(e[i:])
	if strings.Contains(name, ":") || strings.HasPrefix(name, "setup.") {
		return "", "", false // a setup dependency's constraint
	}
	name = strings.TrimPrefix(name, "any.")
	if name == "" || strings.Contains(name, ".") || rest == "" || rest == "installed" ||
		rest[0] == '+' || rest[0] == '-' || strings.HasPrefix(rest, "source") {
		return "", "", false
	}
	return name, normalizeConstraint(rest), true
}

// repositoryName is the package a repository holds: its subdirectory's name, else the
// repository's.
func repositoryName(location, subdirectory string) string {
	if subdirectory != "" && subdirectory != "." {
		return path.Base(strings.TrimSuffix(subdirectory, "/"))
	}
	gitNode := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(location), "/"), ".git")
	if gitNode == "" {
		return ""
	}
	return path.Base(gitNode)
}

// extractCabalProject turns cabal.project's packages, imports of other project
// files and source-repository-package stanzas into imports.
//
// Implements: REQ-HASKELL-005
func extractCabalProject(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	p := readCabalProject(source)
	seen := map[string]bool{}
	for _, m := range p.members {
		spec := "packages: " + m.text
		if !seen[spec] {
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: m.text, Name: kindMember, Line: m.line})
		}
	}
	for _, m := range p.imports {
		spec := "import: " + m.text
		if !seen[spec] {
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: m.text, Name: kindInclude, Line: m.line})
		}
	}
	for _, r := range p.repositories {
		spec := "source-repository-package: " + r.name
		if !seen[spec] {
			seen[spec] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: r.name, Name: kindRepository, Line: r.line})
		}
	}
	return extraction
}
