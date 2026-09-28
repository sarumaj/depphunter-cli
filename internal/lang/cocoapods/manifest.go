package cocoapods

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// declaration is one pod a Podfile or a podspec declares, under its root name (the part
// before a "/": the subspec "Firebase/Analytics" is the pod Firebase).
type declaration struct {
	name         string // as written, subspec included
	requirements string // version requirements, joined with ", "
	// Where the pod comes from when not from a spec repository.
	git, tag, branch, commit string
	path, podspec            string // relative to the Podfile's directory
}

// Dependency is one dependency line of a manifest, for the plugin's imports.
type Dependency struct {
	Spec string // as shown: pod 'Firebase/Analytics'
	Name string // as written: Firebase/Analytics, or a Carthage source
	Line int
}

// Root is a pod's name without its subspec: "Firebase/Analytics" -> "Firebase".
func Root(name string) string {
	root, _, _ := strings.Cut(strings.TrimSpace(name), "/")
	return root
}

// statements reads a Ruby file (Podfile, podspec) as statements: comments stripped
// outside strings, a line ending in "," or "\" or with an open bracket joined with
// the next. Each carries its first line's number.
func statements(source string) []statement {
	var out []statement
	var current strings.Builder
	start, depth := 0, 0
	for i, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		code, d := stripRuby(line)
		if current.Len() == 0 {
			start = i + 1
		}
		current.WriteString(code)
		current.WriteByte(' ')
		depth += d
		t := strings.TrimSpace(code)
		if depth > 0 || strings.HasSuffix(t, ",") || strings.HasSuffix(t, "\\") {
			continue
		}
		if s := strings.TrimSpace(current.String()); s != "" {
			out = append(out, statement{s, start})
		}
		current.Reset()
		depth = 0
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		out = append(out, statement{s, start})
	}
	return out
}

type statement struct {
	text string
	line int
}

// stripRuby removes a line's comment and says how many brackets it leaves open.
// Strings are stepped over; heredocs and %w() lists are not special.
func stripRuby(line string) (string, int) {
	depth := 0
	for i := 0; i < len(line); i++ {
		switch c := line[i]; c {
		case '#':
			return line[:i], depth
		case '\'', '"':
			j := i + 1
			for j < len(line) && line[j] != c {
				if line[j] == '\\' {
					j++
				}
				j++
			}
			i = j
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		}
	}
	return line, depth
}

// call reads a statement `name args` or `name(args)`: the arguments, split at the
// top-level commas.
func call(s, name string) ([]string, bool) {
	rest, ok := strings.CutPrefix(s, name)
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t' && rest[0] != '(') {
		return nil, false
	}
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "(") && strings.HasSuffix(rest, ")") {
		rest = rest[1 : len(rest)-1]
	}
	return splitArguments(rest), true
}

// splitArguments splits Ruby arguments at the commas outside strings and brackets.
func splitArguments(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\'', '"':
			j := i + 1
			for j < len(s) && s[j] != c {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			i = j
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if last := strings.TrimSpace(s[start:]); last != "" {
		out = append(out, last)
	}
	return out
}

// literal reads a string literal; one with interpolation is not one.
func literal(a string) (string, bool) {
	a = strings.TrimSpace(a)
	if len(a) < 2 || (a[0] != '\'' && a[0] != '"') || a[len(a)-1] != a[0] {
		return "", false
	}
	v := a[1 : len(a)-1]
	if a[0] == '"' && strings.Contains(v, "#{") {
		return "", false
	}
	return v, true
}

// option reads a keyword argument: `:git => 'x'`, `git: 'x'` or `"git" => 'x'`.
func option(a string) (key, value string, ok bool) {
	if k, v, found := strings.Cut(a, "=>"); found {
		key = strings.Trim(strings.TrimSpace(k), `:'"`)
		value = strings.TrimSpace(v)
	} else if k, v, found := strings.Cut(a, ":"); found && !strings.ContainsAny(k, " '\"") && k != "" {
		key, value = k, strings.TrimSpace(v)
	} else {
		return "", "", false
	}
	return key, value, key != ""
}

// podArguments reads the arguments after a pod's name: requirement strings and the
// options saying where it comes from.
func podArguments(d *declaration, arguments []string) {
	var requirements []string
	for _, a := range arguments {
		if v, ok := literal(a); ok {
			requirements = append(requirements, strings.Join(strings.Fields(v), " "))
			continue
		}
		k, v, ok := option(a)
		if !ok {
			continue
		}
		s, _ := literal(v)
		switch k {
		case "git":
			d.git = s
		case "tag":
			d.tag = s
		case "branch":
			d.branch = s
		case "commit":
			d.commit = s
		case "path":
			d.path = s
		case "podspec":
			d.podspec = s
		}
	}
	d.requirements = strings.Join(requirements, ", ")
}

// podfile is what a Podfile declares.
type podfile struct {
	pods         []*declaration
	dependencies []Dependency
	sources      []string
}

// readPodfile reads a Podfile's `pod` and `source` statements, wherever they are
// (targets, abstract targets, methods).
//
// Implements: REQ-OBJC-007
func readPodfile(source string) podfile {
	var out podfile
	for _, s := range statements(source) {
		if arguments, ok := call(s.text, "source"); ok && len(arguments) == 1 {
			if u, ok := literal(arguments[0]); ok {
				out.sources = append(out.sources, u)
			}
			continue
		}
		arguments, ok := call(s.text, "pod")
		if !ok || len(arguments) == 0 {
			continue
		}
		name, ok := literal(arguments[0])
		if !ok || name == "" {
			continue
		}
		d := &declaration{name: name}
		podArguments(d, arguments[1:])
		out.pods = append(out.pods, d)
		out.dependencies = append(out.dependencies, Dependency{Spec: "pod '" + name + "'", Name: name, Line: s.line})
	}
	return out
}

// podspec is what a podspec declares: its name (and module name, when set) and its
// dependencies, subspecs' and test specs' included.
type podspec struct {
	name, module string
	dependencies []*declaration
	lines        []Dependency
}

var (
	specName       = regexp.MustCompile(`^\w+\.name\s*=\s*(['"])([^'"]+)['"]`)
	specModule     = regexp.MustCompile(`^\w+\.module_name\s*=\s*(['"])([^'"]+)['"]`)
	specDependency = regexp.MustCompile(`^\w+\.dependency\b`)
)

// readPodspec reads a Ruby podspec's name, module_name and `dependency` calls.
//
// Implements: REQ-OBJC-010
func readPodspec(source string) podspec {
	var out podspec
	for _, s := range statements(source) {
		if m := specName.FindStringSubmatch(s.text); m != nil && out.name == "" {
			out.name = m[2]
			continue
		}
		if m := specModule.FindStringSubmatch(s.text); m != nil && out.module == "" {
			out.module = m[2]
			continue
		}
		if span := specDependency.FindStringIndex(s.text); span != nil {
			arguments, ok := call(s.text[strings.Index(s.text, "dependency"):], "dependency")
			if !ok || len(arguments) == 0 {
				continue
			}
			name, ok := literal(arguments[0])
			if !ok || name == "" {
				continue
			}
			d := &declaration{name: name}
			podArguments(d, arguments[1:])
			out.dependencies = append(out.dependencies, d)
			out.lines = append(out.lines, Dependency{Spec: "dependency '" + name + "'", Name: name, Line: s.line})
		}
	}
	return out
}

// jsonSpec is a podspec in JSON, as the CocoaPods CDN serves them too.
type jsonSpec struct {
	Name         string              `json:"name"`
	ModuleName   string              `json:"module_name"`
	Dependencies map[string][]string `json:"dependencies"`
	Subspecs     []jsonSpec          `json:"subspecs"`
	Testspecs    []jsonSpec          `json:"testspecs"`
}

// readPodspecJSON reads a podspec.json's name, module_name and dependencies; the
// lines of its dependencies are where their names first occur.
//
// Implements: REQ-OBJC-010
func readPodspecJSON(source string) podspec {
	var doc jsonSpec
	if json.Unmarshal([]byte(source), &doc) != nil {
		return podspec{}
	}
	out := podspec{name: doc.Name, module: doc.ModuleName}
	seen := map[string]bool{}
	var walk func(s jsonSpec)
	walk = func(s jsonSpec) {
		names := make([]string, 0, len(s.Dependencies))
		for n := range s.Dependencies {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if seen[n] {
				continue
			}
			seen[n] = true
			out.dependencies = append(out.dependencies, &declaration{name: n, requirements: strings.Join(s.Dependencies[n], ", ")})
			out.lines = append(out.lines, Dependency{Spec: "dependency '" + n + "'", Name: n, Line: lineOf(source, `"`+n+`"`)})
		}
		for _, subspec := range s.Subspecs {
			walk(subspec)
		}
		for _, subspec := range s.Testspecs {
			walk(subspec)
		}
	}
	walk(doc)
	return out
}

func lineOf(source, needle string) int {
	i := strings.Index(source, needle)
	if i < 0 {
		return 1
	}
	return strings.Count(source[:i], "\n") + 1
}

// locked is one root pod in a Podfile.lock.
type locked struct {
	version      string
	dependencies []string // root names of what its specs depend on
	repository   string   // the spec repository it was installed from ("trunk" or a URL)
	// EXTERNAL SOURCES and CHECKOUT OPTIONS: where it was installed from instead.
	git, tag, branch, commit string
	path, podspec            string
}

// podLine reads "Name/Sub (1.2.3)" or "Name (>= 1.0)".
func podLine(s string) (name, version string) {
	name, rest, _ := strings.Cut(strings.TrimSpace(s), " (")
	return name, strings.TrimSuffix(rest, ")")
}

// readLock reads a Podfile.lock: every pod with its version and dependencies
// (PODS), the repositories they came from (SPEC REPOS), and the git and path sources
// with what was checked out (EXTERNAL SOURCES, CHECKOUT OPTIONS).
//
// Implements: REQ-OBJC-008
func readLock(source []byte) (map[string]*locked, map[string][]string) {
	var doc struct {
		Pods         []any                        `yaml:"PODS"`
		Repositories map[string][]string          `yaml:"SPEC REPOS"`
		External     map[string]map[string]string `yaml:"EXTERNAL SOURCES"`
		Checkout     map[string]map[string]string `yaml:"CHECKOUT OPTIONS"`
	}
	if yaml.Unmarshal(source, &doc) != nil {
		return nil, nil
	}
	out := map[string]*locked{}
	get := func(name string) *locked {
		root := Root(name)
		l := out[root]
		if l == nil {
			l = &locked{}
			out[root] = l
		}
		return l
	}
	addDependencies := func(l *locked, self string, dependencies []any) {
		for _, d := range dependencies {
			s, _ := d.(string)
			n, _ := podLine(s)
			if r := Root(n); r != "" && r != self && !contains(l.dependencies, r) {
				l.dependencies = append(l.dependencies, r)
			}
		}
	}
	for _, p := range doc.Pods {
		switch v := p.(type) {
		case string:
			n, version := podLine(v)
			if l := get(n); l.version == "" {
				l.version = version
			}
		case map[string]any:
			for k, dependencies := range v {
				n, version := podLine(k)
				l := get(n)
				if l.version == "" {
					l.version = version
				}
				list, _ := dependencies.([]any)
				addDependencies(l, Root(n), list)
			}
		}
	}
	for _, l := range out {
		sort.Strings(l.dependencies)
	}
	for repository, names := range doc.Repositories {
		for _, n := range names {
			if l := out[Root(n)]; l != nil {
				l.repository = repository
			}
		}
	}
	for n, options := range doc.External {
		if l := out[Root(n)]; l != nil {
			l.git, l.tag, l.branch, l.commit = options[":git"], options[":tag"], options[":branch"], options[":commit"]
			l.path, l.podspec = options[":path"], options[":podspec"]
		}
	}
	for n, options := range doc.Checkout {
		if l := out[Root(n)]; l != nil {
			if options[":git"] != "" {
				l.git = options[":git"]
			}
			if options[":commit"] != "" {
				l.commit = options[":commit"]
			}
			if options[":tag"] != "" {
				l.tag = options[":tag"]
			}
		}
	}
	return out, doc.Repositories
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// cart is one dependency of a Cartfile or Cartfile.resolved.
type cart struct {
	kind        string // github, git or binary
	source      string // as written: owner/repo, a URL
	name        string // the package: its URL as lang.RepoName spells it
	requirement string // requirement: == 1.0, ~> 1.0, >= 1.0 or a quoted git ref
	reference   bool   // requirement is a git reference ("branch", "tag", a commit)
	line        int
}

var cartLine = regexp.MustCompile(`^(github|git|binary)\s+"([^"]+)"\s*(.*)$`)

// readCartfile reads a Cartfile, Cartfile.private or Cartfile.resolved.
//
// Implements: REQ-OBJC-011
func readCartfile(source string) []cart {
	var out []cart
	for i, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		code, _ := stripRuby(line)
		m := cartLine.FindStringSubmatch(strings.TrimSpace(code))
		if m == nil {
			continue
		}
		c := cart{kind: m[1], source: m[2], line: i + 1}
		c.name = cartName(c.kind, c.source)
		requirement := strings.TrimSpace(m[3])
		if v, ok := literal(requirement); ok {
			c.requirement, c.reference = v, true
		} else {
			c.requirement = strings.Join(strings.Fields(requirement), " ")
		}
		out = append(out, c)
	}
	return out
}
