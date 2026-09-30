package perl

import (
	"bytes"
	"cmp"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
)

// moduleRequirement is one requirement a manifest declares. Requirements name modules; the
// distributions that provide them are worked out by the resolver.
type moduleRequirement struct {
	module   string
	version  string // as written: "1.2", "== 1.2", ">= 1, < 2", "" or "0" for any
	phase    string // runtime, test, build, configure, develop
	relation string // requires, recommends, suggests, conflicts
	origin   string // a git or URL source (cpanfile git =>/url =>)
	line     int
}

// manifest is what a manifest says: the distribution's name (when it names it) and
// its requirements.
type manifest struct {
	name         string
	requirements []moduleRequirement
}

// Manifest kinds, by file name.
const (
	classCpanfile = "cpanfile"
	classMakefile = "Makefile.PL"
	classBuild    = "Build.PL"
	classMeta     = "META"
	classDistIni  = "dist.ini"
)

// manifestClass names the kind of manifest a file is, or "" for a source.
//
// Implements: REQ-PERL-001
func manifestClass(base string) string {
	switch base {
	case "cpanfile":
		return classCpanfile
	case "Makefile.PL":
		return classMakefile
	case "Build.PL":
		return classBuild
	case "META.json", "META.yml", "MYMETA.json", "MYMETA.yml":
		return classMeta
	case "dist.ini":
		return classDistIni
	}
	return ""
}

// readManifest reads a manifest of the given class.
//
// Implements: REQ-PERL-006
func readManifest(class, base string, source []byte) *manifest {
	switch class {
	case classCpanfile:
		return &manifest{requirements: readCpanfile(lex(source))}
	case classMakefile, classBuild:
		return readBuildScript(lex(source))
	case classMeta:
		if strings.HasSuffix(base, ".json") {
			return readMetaJSON(source)
		}
		return readMetaYAML(source)
	case classDistIni:
		return readDistIni(source)
	}
	return &manifest{}
}

// imports makes a manifest's requirements imports of what they name: requires and
// recommends of every phase. Suggestions and conflicts are declarations only, and
// perl itself is not a distribution.
//
// Implements: REQ-PERL-006
func (m *manifest) imports() []lang.RawImport {
	var out []lang.RawImport
	seen := map[string]bool{}
	for _, r := range m.requirements {
		if r.module == "perl" || r.relation == "suggests" || r.relation == "conflicts" || !moduleName(r.module) {
			continue
		}
		spec := r.relation + " " + r.module
		if r.phase != "runtime" {
			spec = r.phase + " " + spec
		}
		if v := strings.TrimSpace(r.version); v != "" && v != "0" {
			spec += " " + v
		}
		if seen[spec] {
			continue
		}
		seen[spec] = true
		out = append(out, lang.RawImport{Spec: spec, Module: r.module, Name: kindDependency + "\n" + r.version + "\n" + r.origin, Line: r.line})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

// cpanfileWords are the cpanfile (and Module::Install) statements declaring a
// requirement: relation and phase.
var cpanfileWords = map[string][2]string{
	"requires": {"", "requires"}, "recommends": {"", "recommends"}, "suggests": {"", "suggests"},
	"conflicts": {"", "conflicts"}, "test_requires": {"test", "requires"}, "build_requires": {"build", "requires"},
	"configure_requires": {"configure", "requires"}, "author_requires": {"develop", "requires"},
}

// readCpanfile reads cpanfile statements - `requires 'Mod', '1.2';`, the phase
// blocks `on 'test' => sub { ... }`, feature blocks, and cpm's and Carmel's
// `git =>`/`url =>` options - from a lexed file. Module::Install's Makefile.PL uses
// the same statements.
func readCpanfile(tokens []token) []moduleRequirement {
	var out []moduleRequirement
	type frame struct {
		depth int
		phase string
	}
	var phases []frame
	depth := 0
	pendingPhase := ""
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.kind == tPunctuation {
			switch t.text {
			case "{":
				depth++
				if pendingPhase != "" {
					phases = append(phases, frame{depth, pendingPhase})
					pendingPhase = ""
				}
			case "}":
				for len(phases) > 0 && phases[len(phases)-1].depth >= depth {
					phases = phases[:len(phases)-1]
				}
				depth--
			case ";":
				pendingPhase = ""
			}
			continue
		}
		if t.kind != tWord || i > 0 && tokens[i-1].kind == tPunctuation && tokens[i-1].text == "->" {
			continue
		}
		if t.text == "on" && i+1 < len(tokens) && (tokens[i+1].kind == tString || tokens[i+1].kind == tWord) {
			pendingPhase = tokens[i+1].text
			continue
		}
		keyword, ok := cpanfileWords[t.text]
		if !ok || i+1 >= len(tokens) || tokens[i+1].kind == tPunctuation && tokens[i+1].text == "=>" {
			continue
		}
		j := i + 1
		if tokens[j].kind == tPunctuation && tokens[j].text == "(" {
			j++
		}
		module := tokens[min(j, len(tokens)-1)]
		if module.kind != tString && module.kind != tWord || !moduleName(module.text) {
			continue
		}
		r := moduleRequirement{module: strings.TrimSuffix(module.text, "::"), relation: keyword[1], phase: keyword[0], line: t.line}
		if r.phase == "" {
			r.phase = "runtime"
			if n := len(phases); n > 0 {
				r.phase = phases[n-1].phase
			}
		}
		end := j + 1
		for end < len(tokens) && !(tokens[end].kind == tPunctuation && (tokens[end].text == ";" || tokens[end].text == "}")) {
			end++
		}
		items := splitArguments(tokens[j+1 : end])
		if len(items) > 0 && len(items[0]) == 0 {
			items = items[1:] // the comma after the module
		}
		if len(items) > 0 && len(items[0]) == 1 && (items[0][0].kind == tString || items[0][0].kind == tNumber) {
			r.version = strings.TrimSpace(items[0][0].text)
			items = items[1:]
		}
		for k := 0; k+1 < len(items); k += 2 {
			if len(items[k]) == 1 && len(items[k+1]) == 1 && items[k+1][0].kind == tString {
				switch items[k][0].text {
				case "git", "url", "dist":
					r.origin = cmp.Or(r.origin, items[k+1][0].text)
				}
			}
		}
		out = append(out, r)
		i = end - 1
	}
	return out
}

// buildKeys are the prerequisite hashes of ExtUtils::MakeMaker's WriteMakefile and
// Module::Build's new: key -> phase, relation.
var buildKeys = map[string][2]string{
	"PREREQ_PM": {"runtime", "requires"}, "BUILD_REQUIRES": {"build", "requires"},
	"TEST_REQUIRES": {"test", "requires"}, "CONFIGURE_REQUIRES": {"configure", "requires"},
	"requires": {"runtime", "requires"}, "recommends": {"runtime", "recommends"},
	"build_requires": {"build", "requires"}, "test_requires": {"test", "requires"},
	"configure_requires": {"configure", "requires"},
}

// readBuildScript reads a Makefile.PL or Build.PL: the literal prerequisite hashes
// (PREREQ_PM => {...}, requires => {...}), a META_MERGE's `prereqs`, the
// distribution's name (NAME, DISTNAME, module_name, dist_name, Module::Install's
// name) and Module::Install's cpanfile-like statements.
func readBuildScript(tokens []token) *manifest {
	m := &manifest{requirements: readCpanfile(tokens)}
	for i := 0; i+2 < len(tokens); i++ {
		t := tokens[i]
		if t.kind != tWord && t.kind != tString || !(tokens[i+1].kind == tPunctuation && tokens[i+1].text == "=>") {
			if t.kind == tWord && t.text == "name" && tokens[i+1].kind == tString && m.name == "" {
				m.name = tokens[i+1].text // Module::Install: name 'Foo-Bar';
			}
			continue
		}
		v := tokens[i+2]
		switch t.text {
		case "NAME", "module_name":
			if v.kind == tString && m.name == "" {
				m.name = strings.ReplaceAll(v.text, "::", "-")
			}
		case "DISTNAME", "dist_name":
			if v.kind == tString {
				m.name = v.text
			}
		case "prereqs":
			if v.kind == tPunctuation && v.text == "{" {
				value, _ := parseValue(tokens, i+2, 0)
				for _, phase := range value.keysOf() {
					releases := value.m[phase]
					for _, relationship := range releases.keysOf() {
						m.requirements = append(m.requirements, hashRequirements(releases.m[relationship], phase, relationship)...)
					}
				}
			}
		}
		if keyword, ok := buildKeys[t.text]; ok && v.kind == tPunctuation && v.text == "{" {
			value, _ := parseValue(tokens, i+2, 0)
			m.requirements = append(m.requirements, hashRequirements(value, keyword[0], keyword[1])...)
		}
	}
	return m
}

func hashRequirements(v *value, phase, relation string) []moduleRequirement {
	if v == nil {
		return nil
	}
	var out []moduleRequirement
	for _, k := range v.keys {
		if e := v.m[k]; e != nil && moduleName(k) {
			out = append(out, moduleRequirement{module: k, version: e.s, phase: phase, relation: relation, line: v.lines[k]})
		}
	}
	return out
}

// value is a Perl literal: a string (or number) or a hash, keys in order.
type value struct {
	s     string
	keys  []string
	m     map[string]*value
	lines map[string]int
}

func (v *value) keysOf() []string {
	if v == nil {
		return nil
	}
	return v.keys
}

// maxNesting bounds how deep parseValue follows nested literals.
const maxNesting = 64

// parseValue parses the literal starting at tokens[i]: a string, number or word, a
// hash in braces (or a list in parentheses, read as one) of such values; anything
// else is skipped to the next comma. It returns the value and the index after it.
func parseValue(tokens []token, i, depth int) (*value, int) {
	if i >= len(tokens) {
		return nil, i
	}
	t := tokens[i]
	switch {
	case t.kind == tString || t.kind == tNumber || t.kind == tWord:
		return &value{s: t.text}, i + 1
	case t.kind == tPunctuation && (t.text == "{" || t.text == "(" || t.text == "[") && depth < maxNesting:
		v := &value{m: map[string]*value{}, lines: map[string]int{}}
		cl := map[string]string{"{": "}", "(": ")", "[": "]"}[t.text]
		j := i + 1
		for j < len(tokens) && !(tokens[j].kind == tPunctuation && tokens[j].text == cl) {
			k := tokens[j]
			if (k.kind == tString || k.kind == tWord || k.kind == tNumber) && j+1 < len(tokens) && tokens[j+1].kind == tPunctuation && (tokens[j+1].text == "=>" || tokens[j+1].text == ",") {
				var e *value
				e, j = parseValue(tokens, j+2, depth+1)
				if _, duplicate := v.m[k.text]; !duplicate {
					v.keys = append(v.keys, k.text)
					v.lines[k.text] = k.line
				}
				v.m[k.text] = e
			} else {
				j = skipItem(tokens, j)
			}
			if j < len(tokens) && tokens[j].kind == tPunctuation && (tokens[j].text == "," || tokens[j].text == "=>") {
				j++
			}
		}
		return v, j + 1
	}
	return nil, skipItem(tokens, i)
}

// skipItem skips tokens to the next comma or closer at depth 0.
func skipItem(tokens []token, i int) int {
	depth := 0
	for ; i < len(tokens); i++ {
		t := tokens[i]
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			if depth == 0 {
				return i
			}
			depth--
		case ",", "=>", ";":
			if depth == 0 {
				return i
			}
		}
	}
	return i
}

// readMetaJSON reads META.json or MYMETA.json: version 2's prereqs (phase ->
// relation -> module -> version) or version 1's requires, build_requires,
// configure_requires and recommends.
func readMetaJSON(source []byte) *manifest {
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()
	var doc map[string]any
	if decoder.Decode(&doc) != nil {
		return &manifest{}
	}
	m := &manifest{}
	m.name, _ = doc["name"].(string)
	lines := strings.Split(string(source), "\n")
	add := func(h any, phase, relation string) {
		modules, _ := h.(map[string]any)
		for _, module := range sortedAny(modules) {
			m.requirements = append(m.requirements, moduleRequirement{module: module, version: jsonString(modules[module]), phase: phase, relation: relation, line: lineOf(lines, `"`+module+`"`)})
		}
	}
	if prereqs, ok := doc["prereqs"].(map[string]any); ok {
		for _, phase := range sortedAny(prereqs) {
			releases, _ := prereqs[phase].(map[string]any)
			for _, relationship := range sortedAny(releases) {
				add(releases[relationship], phase, relationship)
			}
		}
		return m
	}
	for key, keyword := range metaV1 {
		add(doc[key], keyword[0], keyword[1])
	}
	sort.SliceStable(m.requirements, func(i, j int) bool { return m.requirements[i].line < m.requirements[j].line })
	return m
}

// metaV1 are the prerequisite keys of CPAN::Meta::Spec version 1.4 (META.yml).
var metaV1 = map[string][2]string{
	"requires": {"runtime", "requires"}, "recommends": {"runtime", "recommends"},
	"build_requires": {"build", "requires"}, "configure_requires": {"configure", "requires"},
	"test_requires": {"test", "requires"},
}

func jsonString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	}
	return ""
}

func sortedAny(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// lineOf is the 1-based line of the first line containing s, else 1.
func lineOf(lines []string, s string) int {
	for i, l := range lines {
		if strings.Contains(l, s) {
			return i + 1
		}
	}
	return 1
}

// readMetaYAML reads META.yml or MYMETA.yml (version 1.4, or version 2 written as
// YAML), keeping versions as written: 1.10 is not 1.1.
func readMetaYAML(source []byte) *manifest {
	root := yamlnode.Mapping(yamlnode.Parse(source))
	if root == nil {
		return &manifest{}
	}
	m := &manifest{}
	if n := yamlnode.Get(root, "name"); n != nil {
		m.name = n.Value
	}
	add := func(n *yaml.Node, phase, relation string) {
		for _, entry := range yamlnode.Pairs(n) {
			m.requirements = append(m.requirements, moduleRequirement{module: entry.Key.Value, version: entry.Value.Value, phase: phase, relation: relation, line: entry.Key.Line})
		}
	}
	for _, phase := range yamlnode.Pairs(yamlnode.Get(root, "prereqs")) {
		for _, relation := range yamlnode.Pairs(phase.Value) {
			add(relation.Value, phase.Key.Value, relation.Key.Value)
		}
	}
	for key, keyword := range metaV1 {
		add(yamlnode.Get(root, key), keyword[0], keyword[1])
	}
	sort.SliceStable(m.requirements, func(i, j int) bool { return m.requirements[i].line < m.requirements[j].line })
	return m
}

// prereqsLabel reads a Dist::Zilla [Prereqs / Label] name: TestRequires,
// RuntimeRecommends, DevelopSuggests.
var prereqsLabel = regexp.MustCompile(`^(Runtime|Test|Build|Configure|Develop)(Requires|Recommends|Suggests|Conflicts)$`)

// readDistIni reads a Dist::Zilla dist.ini: `name`, and the [Prereqs] sections -
// `[Prereqs]` is runtime requires, `[Prereqs / TestRequires]` names its phase and
// relation, or -phase and -relationship say them. Author dependencies (the
// ; authordep comments, plugin bundles) are the author's tools, not the
// distribution's, and are not read; neither is [AutoPrereqs], whose prerequisites
// are what the sources use.
func readDistIni(source []byte) *manifest {
	m := &manifest{}
	type section struct {
		start           int
		phase, relation string
		entries         []moduleRequirement
	}
	var current *section
	flush := func() {
		if current == nil {
			return
		}
		for _, e := range current.entries {
			e.phase, e.relation = current.phase, current.relation
			m.requirements = append(m.requirements, e)
		}
		current = nil
	}
	for n, line := range strings.Split(string(source), "\n") {
		s := strings.TrimSpace(line)
		if i := strings.Index(s, " ;"); i >= 0 {
			s = strings.TrimSpace(s[:i])
		}
		if s == "" || s[0] == ';' || s[0] == '#' {
			continue
		}
		if s[0] == '[' {
			flush()
			name := strings.TrimSpace(strings.Trim(s, "[]"))
			plugin, label, _ := strings.Cut(name, "/")
			if strings.TrimSpace(plugin) != "Prereqs" {
				continue
			}
			current = &section{start: n + 1, phase: "runtime", relation: "requires"}
			if match := prereqsLabel.FindStringSubmatch(strings.TrimSpace(label)); match != nil {
				current.phase, current.relation = strings.ToLower(match[1]), strings.ToLower(match[2])
			}
			continue
		}
		key, value, ok := strings.Cut(s, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if current == nil {
			if key == "name" && m.name == "" {
				m.name = value
			}
			continue
		}
		switch key {
		case "-phase":
			current.phase = value
		case "-relationship", "-type":
			current.relation = value
		default:
			if !strings.HasPrefix(key, "-") && moduleName(key) {
				current.entries = append(current.entries, moduleRequirement{module: key, version: value, line: n + 1})
			}
		}
	}
	flush()
	return m
}

// snapshot is a Carton cpanfile.snapshot: the distributions installed into local/,
// each with its version, the modules it provides and the modules it requires.
type snapshot struct {
	dists    map[string]*snapDist
	provides map[string]string // module -> distribution
}

type snapDist struct {
	name, version string
	requires      []moduleRequirement
}

// distVersion splits "libwww-perl-6.72" into the distribution and its version.
var distVersion = regexp.MustCompile(`^(.+)-(v?[0-9][0-9._]*(?:-TRIAL)?)$`)

// readSnapshot reads a cpanfile.snapshot (carton snapshot format 1.0).
//
// Implements: REQ-PERL-007
func readSnapshot(source []byte) *snapshot {
	s := &snapshot{dists: map[string]*snapDist{}, provides: map[string]string{}}
	var current *snapDist
	section := ""
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimRight(line, "\r")
		indent := len(line) - len(strings.TrimLeft(line, " "))
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		switch {
		case indent == 2:
			current, section = nil, ""
			if match := distVersion.FindStringSubmatch(text); match != nil {
				current = &snapDist{name: match[1], version: match[2]}
				s.dists[current.name] = current
			}
		case indent == 4 && current != nil:
			section = strings.TrimSuffix(strings.Fields(text)[0], ":")
		case indent >= 6 && current != nil:
			f := strings.Fields(text)
			v := ""
			if len(f) > 1 {
				v = f[1]
			}
			switch section {
			case "provides":
				if _, ok := s.provides[f[0]]; !ok {
					s.provides[f[0]] = current.name
				}
			case "requirements":
				current.requires = append(current.requires, moduleRequirement{module: f[0], version: v})
			}
		}
	}
	return s
}
