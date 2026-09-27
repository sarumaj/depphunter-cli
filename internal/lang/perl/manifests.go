package perl

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// req is one requirement a manifest declares. Requirements name modules; the
// distributions that provide them are worked out by the resolver.
type req struct {
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
	name string
	reqs []req
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
func readManifest(class, base string, src []byte) *manifest {
	switch class {
	case classCpanfile:
		return &manifest{reqs: readCpanfile(lex(src))}
	case classMakefile, classBuild:
		return readBuildScript(lex(src))
	case classMeta:
		if strings.HasSuffix(base, ".json") {
			return readMetaJSON(src)
		}
		return readMetaYAML(src)
	case classDistIni:
		return readDistIni(src)
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
	for _, r := range m.reqs {
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
		out = append(out, lang.RawImport{Spec: spec, Module: r.module, Name: kindDep + "\n" + r.version + "\n" + r.origin, Line: r.line})
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
func readCpanfile(tokens []token) []req {
	var out []req
	type frame struct {
		depth int
		phase string
	}
	var phases []frame
	depth := 0
	pendingPhase := ""
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.kind == tPunct {
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
		if t.kind != tWord || i > 0 && tokens[i-1].kind == tPunct && tokens[i-1].text == "->" {
			continue
		}
		if t.text == "on" && i+1 < len(tokens) && (tokens[i+1].kind == tStr || tokens[i+1].kind == tWord) {
			pendingPhase = tokens[i+1].text
			continue
		}
		kw, ok := cpanfileWords[t.text]
		if !ok || i+1 >= len(tokens) || tokens[i+1].kind == tPunct && tokens[i+1].text == "=>" {
			continue
		}
		j := i + 1
		if tokens[j].kind == tPunct && tokens[j].text == "(" {
			j++
		}
		mod := tokens[min(j, len(tokens)-1)]
		if mod.kind != tStr && mod.kind != tWord || !moduleName(mod.text) {
			continue
		}
		r := req{module: strings.TrimSuffix(mod.text, "::"), relation: kw[1], phase: kw[0], line: t.line}
		if r.phase == "" {
			r.phase = "runtime"
			if n := len(phases); n > 0 {
				r.phase = phases[n-1].phase
			}
		}
		end := j + 1
		for end < len(tokens) && !(tokens[end].kind == tPunct && (tokens[end].text == ";" || tokens[end].text == "}")) {
			end++
		}
		items := splitArgs(tokens[j+1 : end])
		if len(items) > 0 && len(items[0]) == 0 {
			items = items[1:] // the comma after the module
		}
		if len(items) > 0 && len(items[0]) == 1 && (items[0][0].kind == tStr || items[0][0].kind == tNum) {
			r.version = strings.TrimSpace(items[0][0].text)
			items = items[1:]
		}
		for k := 0; k+1 < len(items); k += 2 {
			if len(items[k]) == 1 && len(items[k+1]) == 1 && items[k+1][0].kind == tStr {
				switch items[k][0].text {
				case "git", "url", "dist":
					if r.origin == "" {
						r.origin = items[k+1][0].text
					}
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
	m := &manifest{reqs: readCpanfile(tokens)}
	for i := 0; i+2 < len(tokens); i++ {
		t := tokens[i]
		if t.kind != tWord && t.kind != tStr || !(tokens[i+1].kind == tPunct && tokens[i+1].text == "=>") {
			if t.kind == tWord && t.text == "name" && tokens[i+1].kind == tStr && m.name == "" {
				m.name = tokens[i+1].text // Module::Install: name 'Foo-Bar';
			}
			continue
		}
		v := tokens[i+2]
		switch t.text {
		case "NAME", "module_name":
			if v.kind == tStr && m.name == "" {
				m.name = strings.ReplaceAll(v.text, "::", "-")
			}
		case "DISTNAME", "dist_name":
			if v.kind == tStr {
				m.name = v.text
			}
		case "prereqs":
			if v.kind == tPunct && v.text == "{" {
				val, _ := parseValue(tokens, i+2, 0)
				for _, phase := range val.keysOf() {
					releases := val.m[phase]
					for _, rel := range releases.keysOf() {
						m.reqs = append(m.reqs, hashReqs(releases.m[rel], phase, rel)...)
					}
				}
			}
		}
		if kw, ok := buildKeys[t.text]; ok && v.kind == tPunct && v.text == "{" {
			val, _ := parseValue(tokens, i+2, 0)
			m.reqs = append(m.reqs, hashReqs(val, kw[0], kw[1])...)
		}
	}
	return m
}

func hashReqs(v *value, phase, relation string) []req {
	if v == nil {
		return nil
	}
	var out []req
	for _, k := range v.keys {
		if e := v.m[k]; e != nil && moduleName(k) {
			out = append(out, req{module: k, version: e.s, phase: phase, relation: relation, line: v.lines[k]})
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
	case t.kind == tStr || t.kind == tNum || t.kind == tWord:
		return &value{s: t.text}, i + 1
	case t.kind == tPunct && (t.text == "{" || t.text == "(" || t.text == "[") && depth < maxNesting:
		v := &value{m: map[string]*value{}, lines: map[string]int{}}
		cl := map[string]string{"{": "}", "(": ")", "[": "]"}[t.text]
		j := i + 1
		for j < len(tokens) && !(tokens[j].kind == tPunct && tokens[j].text == cl) {
			k := tokens[j]
			if (k.kind == tStr || k.kind == tWord || k.kind == tNum) && j+1 < len(tokens) && tokens[j+1].kind == tPunct && (tokens[j+1].text == "=>" || tokens[j+1].text == ",") {
				var e *value
				e, j = parseValue(tokens, j+2, depth+1)
				if _, dup := v.m[k.text]; !dup {
					v.keys = append(v.keys, k.text)
					v.lines[k.text] = k.line
				}
				v.m[k.text] = e
			} else {
				j = skipItem(tokens, j)
			}
			if j < len(tokens) && tokens[j].kind == tPunct && (tokens[j].text == "," || tokens[j].text == "=>") {
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
		if t.kind != tPunct {
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
func readMetaJSON(src []byte) *manifest {
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	var doc map[string]any
	if dec.Decode(&doc) != nil {
		return &manifest{}
	}
	m := &manifest{}
	m.name, _ = doc["name"].(string)
	lines := strings.Split(string(src), "\n")
	add := func(h any, phase, relation string) {
		mods, _ := h.(map[string]any)
		for _, mod := range sortedAny(mods) {
			m.reqs = append(m.reqs, req{module: mod, version: jsonString(mods[mod]), phase: phase, relation: relation, line: lineOf(lines, `"`+mod+`"`)})
		}
	}
	if prereqs, ok := doc["prereqs"].(map[string]any); ok {
		for _, phase := range sortedAny(prereqs) {
			releases, _ := prereqs[phase].(map[string]any)
			for _, rel := range sortedAny(releases) {
				add(releases[rel], phase, rel)
			}
		}
		return m
	}
	for key, kw := range metaV1 {
		add(doc[key], kw[0], kw[1])
	}
	sort.SliceStable(m.reqs, func(i, j int) bool { return m.reqs[i].line < m.reqs[j].line })
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
func readMetaYAML(src []byte) *manifest {
	var doc yaml.Node
	if yaml.Unmarshal(src, &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return &manifest{}
	}
	root := doc.Content[0]
	m := &manifest{}
	get := func(n *yaml.Node, key string) *yaml.Node {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		for k := 0; k+1 < len(n.Content); k += 2 {
			if n.Content[k].Value == key {
				return n.Content[k+1]
			}
		}
		return nil
	}
	if n := get(root, "name"); n != nil {
		m.name = n.Value
	}
	add := func(n *yaml.Node, phase, relation string) {
		if n == nil || n.Kind != yaml.MappingNode {
			return
		}
		for k := 0; k+1 < len(n.Content); k += 2 {
			m.reqs = append(m.reqs, req{module: n.Content[k].Value, version: n.Content[k+1].Value, phase: phase, relation: relation, line: n.Content[k].Line})
		}
	}
	if p := get(root, "prereqs"); p != nil && p.Kind == yaml.MappingNode {
		for k := 0; k+1 < len(p.Content); k += 2 {
			releases := p.Content[k+1]
			if releases.Kind != yaml.MappingNode {
				continue
			}
			for r := 0; r+1 < len(releases.Content); r += 2 {
				add(releases.Content[r+1], p.Content[k].Value, releases.Content[r].Value)
			}
		}
	}
	for key, kw := range metaV1 {
		add(get(root, key), kw[0], kw[1])
	}
	sort.SliceStable(m.reqs, func(i, j int) bool { return m.reqs[i].line < m.reqs[j].line })
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
func readDistIni(src []byte) *manifest {
	m := &manifest{}
	type section struct {
		start           int
		phase, relation string
		entries         []req
	}
	var cur *section
	flush := func() {
		if cur == nil {
			return
		}
		for _, e := range cur.entries {
			e.phase, e.relation = cur.phase, cur.relation
			m.reqs = append(m.reqs, e)
		}
		cur = nil
	}
	for n, line := range strings.Split(string(src), "\n") {
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
			cur = &section{start: n + 1, phase: "runtime", relation: "requires"}
			if sm := prereqsLabel.FindStringSubmatch(strings.TrimSpace(label)); sm != nil {
				cur.phase, cur.relation = strings.ToLower(sm[1]), strings.ToLower(sm[2])
			}
			continue
		}
		key, val, ok := strings.Cut(s, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if cur == nil {
			if key == "name" && m.name == "" {
				m.name = val
			}
			continue
		}
		switch key {
		case "-phase":
			cur.phase = val
		case "-relationship", "-type":
			cur.relation = val
		default:
			if !strings.HasPrefix(key, "-") && moduleName(key) {
				cur.entries = append(cur.entries, req{module: key, version: val, line: n + 1})
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
	requires      []req
}

// distVersion splits "libwww-perl-6.72" into the distribution and its version.
var distVersion = regexp.MustCompile(`^(.+)-(v?[0-9][0-9._]*(?:-TRIAL)?)$`)

// readSnapshot reads a cpanfile.snapshot (carton snapshot format 1.0).
//
// Implements: REQ-PERL-007
func readSnapshot(src []byte) *snapshot {
	s := &snapshot{dists: map[string]*snapDist{}, provides: map[string]string{}}
	var cur *snapDist
	section := ""
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimRight(line, "\r")
		indent := len(line) - len(strings.TrimLeft(line, " "))
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		switch {
		case indent == 2:
			cur, section = nil, ""
			if sm := distVersion.FindStringSubmatch(text); sm != nil {
				cur = &snapDist{name: sm[1], version: sm[2]}
				s.dists[cur.name] = cur
			}
		case indent == 4 && cur != nil:
			section = strings.TrimSuffix(strings.Fields(text)[0], ":")
		case indent >= 6 && cur != nil:
			f := strings.Fields(text)
			v := ""
			if len(f) > 1 {
				v = f[1]
			}
			switch section {
			case "provides":
				if _, ok := s.provides[f[0]]; !ok {
					s.provides[f[0]] = cur.name
				}
			case "requirements":
				cur.requires = append(cur.requires, req{module: f[0], version: v})
			}
		}
	}
	return s
}
