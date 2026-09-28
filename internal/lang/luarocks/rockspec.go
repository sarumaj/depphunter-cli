// Package luarocks reads what LuaRocks writes in Lua: rockspecs, luarocks.lock,
// LuaRocks configuration files and a rocks server's manifest, and compares LuaRocks
// versions. It holds the Lua lexer (lex.go) the lua plugin reads sources with and a
// small evaluator of constant Lua (eval.go). It is shared by the lua plugin and the
// index client, which reads a rocks server's manifest and rockspecs with it.
package luarocks

import (
	"sort"
	"strings"
)

// Dep is one dependency of a rockspec: "penlight ~> 1.5" is Name penlight,
// Constraint "~> 1.5".
type Dependency struct {
	Name       string
	Constraint string
	Section    string // dependencies, build_dependencies or test_dependencies
	Spec       string // as written
	Line       int
}

// Module is an entry of a rockspec's build.modules (or build.install.lua): the Lua
// module name and the file that provides it, relative to the rock's source root.
type Module struct {
	Name string
	File string
	Line int
}

// Rockspec is what a rockspec declares about its rock.
type Rockspec struct {
	Package      string
	Version      string
	Dependencies []Dependency
	Modules      []Module
}

// sections are the dependency tables of a rockspec (rockspec_format 3.0 added the
// last two).
var sections = []string{"dependencies", "build_dependencies", "test_dependencies"}

// ReadRockspec reads a rockspec. Dependencies of every platform are included:
// which platform the map is drawn for is not known.
//
// Implements: REQ-LUA-006
func ReadRockspec(source []byte) *Rockspec {
	c := Eval(source)
	r := &Rockspec{Package: c.Globals["package"].Text, Version: c.Globals["version"].Text}
	for _, section := range sections {
		for _, v := range platformLists(c.Globals[section]) {
			name, constraint := ParseDependency(v.Text)
			if name != "" {
				r.Dependencies = append(r.Dependencies, Dependency{Name: name, Constraint: constraint, Section: section, Spec: v.Text, Line: v.Line})
			}
		}
	}
	build := c.Globals["build"]
	tables := []Value{build.Path("modules"), build.Path("install", "lua")}
	if p := build.Path("platforms"); p.Kind == TableValue {
		for _, f := range p.Table.Fields {
			tables = append(tables, f.Value.Path("modules"), f.Value.Path("install", "lua"))
		}
	}
	seen := map[string]bool{}
	for _, t := range tables {
		if t.Kind != TableValue {
			continue
		}
		for _, f := range t.Table.Fields {
			if file := moduleFile(f.Value); file != "" && !seen[f.Key] {
				seen[f.Key] = true
				r.Modules = append(r.Modules, Module{Name: f.Key, File: file, Line: f.Value.Line})
			}
		}
	}
	return r
}

// platformLists is a dependency table's strings and those of its per-platform
// tables (dependencies = { "a", platforms = { unix = { "b" } } }).
func platformLists(v Value) []Value {
	out := v.Strings()
	if p := v.Path("platforms"); p.Kind == TableValue {
		for _, f := range p.Table.Fields {
			out = append(out, f.Value.Strings()...)
		}
	}
	return out
}

// moduleFile is the file a build.modules entry names: the file itself for a Lua
// module, the first source of a C module.
func moduleFile(v Value) string {
	switch v.Kind {
	case StringValue:
		return v.Text
	case TableValue:
		if s := v.Path("sources"); s.Kind == StringValue {
			return s.Text
		} else if l := s.Strings(); len(l) > 0 {
			return l[0].Text
		}
		if l := v.Strings(); len(l) > 0 {
			return l[0].Text
		}
	}
	return ""
}

// ParseDependency splits a dependency string into the rock's name and its version
// constraints: "lua-cjson >= 2.1, < 3" -> ("lua-cjson", ">= 2.1, < 3"). Names are
// matched in lower case by LuaRocks and are returned so.
func ParseDependency(s string) (name, constraint string) {
	s = strings.TrimSpace(s)
	i := strings.IndexFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '<' || r == '>' || r == '=' || r == '~' || r == '!'
	})
	if i < 0 {
		return strings.ToLower(s), ""
	}
	return strings.ToLower(s[:i]), strings.TrimSpace(s[i:])
}

// Exact is the version a constraint fixes, if it fixes one: "== 1.2.3" and a bare
// "1.2.3" (LuaRocks reads a version without an operator as ==).
func Exact(constraint string) (string, bool) {
	c := strings.TrimSpace(constraint)
	if c == "" || strings.Contains(c, ",") {
		return "", false
	}
	if v, ok := strings.CutPrefix(c, "=="); ok {
		return strings.TrimSpace(v), true
	}
	if strings.ContainsAny(c[:1], "<>=~!") {
		return "", false
	}
	return c, true
}

// ReadLock reads a luarocks.lock: `return { dependencies = { name = "1.2.3-1" } }`,
// the versions `luarocks build --pin` installed, keyed by rock name.
//
// Implements: REQ-LUA-007
func ReadLock(source []byte) map[string]string {
	c := Eval(source)
	dependencies := c.Return.Path("dependencies")
	if dependencies.Kind != TableValue {
		dependencies = c.Globals["dependencies"]
	}
	out := map[string]string{}
	if dependencies.Kind != TableValue {
		return out
	}
	for _, f := range dependencies.Table.Fields {
		if f.Value.Kind == StringValue {
			out[strings.ToLower(f.Key)] = f.Value.Text
		}
	}
	return out
}

// ReadManifest reads a rocks server's manifest (manifest-5.1): the versions (with
// their revision, "1.14.0-3") of every rock it serves a rockspec for.
func ReadManifest(source []byte) map[string][]string {
	repository := Eval(source).Globals["repository"]
	out := map[string][]string{}
	if repository.Kind != TableValue {
		return out
	}
	for _, rock := range repository.Table.Fields {
		if rock.Value.Kind != TableValue {
			continue
		}
		for _, v := range rock.Value.Table.Fields {
			if v.Value.Kind != TableValue {
				continue
			}
			for _, arch := range v.Value.Table.List {
				if arch.Path("arch").Text == "rockspec" {
					out[rock.Key] = append(out[rock.Key], v.Key)
					break
				}
			}
		}
	}
	return out
}

// Servers reads the rocks servers a LuaRocks configuration file names
// (rocks_servers = { "https://…", { "https://mirror", "https://other" } }), in
// order, a group's first entry first.
func Servers(source []byte) []string {
	c := Eval(source)
	var out []string
	var walk func(v Value)
	walk = func(v Value) {
		switch v.Kind {
		case StringValue:
			out = append(out, v.Text)
		case TableValue:
			for _, e := range v.Table.List {
				walk(e)
			}
		}
	}
	walk(c.Globals["rocks_servers"])
	return out
}

// Compare orders two LuaRocks versions ("1.10.0-1" after "1.9.2-3"). A revision
// after the last "-" breaks ties; "scm" and "dev" sort after every release, and
// letters inside a part (3.0rc1) before the plain number.
func Compare(a, b string) int {
	av, ar := splitRevision(a)
	bv, br := splitRevision(b)
	if c := compareParts(av, bv); c != 0 {
		return c
	}
	return compareParts(ar, br)
}

func splitRevision(v string) (string, string) {
	v = strings.ToLower(strings.TrimSpace(v))
	if i := strings.LastIndexByte(v, '-'); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

func compareParts(a, b string) int {
	aParts, bParts := strings.FieldsFunc(a, isSeparator), strings.FieldsFunc(b, isSeparator)
	for i := 0; i < len(aParts) || i < len(bParts); i++ {
		x, y := "0", "0"
		if i < len(aParts) {
			x = aParts[i]
		}
		if i < len(bParts) {
			y = bParts[i]
		}
		if c := comparePart(x, y); c != 0 {
			return c
		}
	}
	return 0
}

func isSeparator(r rune) bool { return r == '.' || r == '_' || r == '-' }

// comparePart compares one dot-separated part: numbers numerically, "scm"/"dev"
// above any number, a number with a suffix (0rc1) below the bare number.
func comparePart(x, y string) int {
	rank := func(s string) (int, int, string) {
		switch s {
		case "scm", "dev", "cvs":
			return 2, 0, s
		}
		n, i := 0, 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			if n < 1<<30 {
				n = n*10 + int(s[i]-'0')
			}
			i++
		}
		if i == 0 {
			return 0, 0, s // a word: before numbers
		}
		return 1, n, s[i:]
	}
	xk, xn, xs := rank(x)
	yk, yn, ys := rank(y)
	switch {
	case xk != yk:
		return xk - yk
	case xn != yn:
		if xn < yn {
			return -1
		}
		return 1
	case xs == ys:
		return 0
	case xs == "":
		return 1 // 1.0 after 1.0rc1
	case ys == "":
		return -1
	}
	return strings.Compare(xs, ys)
}

// Satisfies reports whether a version meets a constraint list (">= 1.0, < 2.0").
// "~> 1.5" is LuaRocks' pessimistic match: every part it names must be equal.
func Satisfies(version, constraint string) bool {
	for _, c := range strings.Split(constraint, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		operator := ""
		for _, o := range []string{"==", "~=", ">=", "<=", "~>", "!=", ">", "<", "="} {
			if strings.HasPrefix(c, o) {
				operator = o
				break
			}
		}
		want := strings.TrimSpace(c[len(operator):])
		cmp := Compare(version, want)
		if _, rev := splitRevision(want); rev == "" { // no revision asked for: any will do
			v, _ := splitRevision(version)
			cmp = compareParts(v, want)
		}
		ok := false
		switch operator {
		case "", "==", "=":
			ok = cmp == 0
		case "~=", "!=":
			ok = cmp != 0
		case ">=":
			ok = cmp >= 0
		case "<=":
			ok = cmp <= 0
		case ">":
			ok = cmp > 0
		case "<":
			ok = cmp < 0
		case "~>":
			v, _ := splitRevision(version)
			haveParts, wantParts := strings.FieldsFunc(v, isSeparator), strings.FieldsFunc(want, isSeparator)
			ok = len(haveParts) >= len(wantParts)
			for i := 0; ok && i < len(wantParts); i++ {
				ok = comparePart(haveParts[i], wantParts[i]) == 0
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// Newest is the newest of versions meeting a constraint, releases before "scm" and
// "dev" builds; "" when none does.
func Newest(versions []string, constraint string) string {
	sorted := append([]string(nil), versions...)
	sort.SliceStable(sorted, func(i, j int) bool { return Compare(sorted[i], sorted[j]) > 0 })
	fallback := ""
	for _, v := range sorted {
		if !Satisfies(v, constraint) {
			continue
		}
		if dev(v) {
			if fallback == "" {
				fallback = v
			}
			continue
		}
		return v
	}
	return fallback
}

func dev(v string) bool {
	v = strings.ToLower(v)
	return strings.HasPrefix(v, "scm") || strings.HasPrefix(v, "dev") || strings.HasPrefix(v, "cvs")
}
