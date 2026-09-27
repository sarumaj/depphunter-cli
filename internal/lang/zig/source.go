package zig

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindImport   = "import"   // @import("x"): a file, std/builtin/root or a module name
	kindEmbed    = "embed"    // @embedFile("x"): a file beside the importer
	kindCInclude = "cinclude" // @cInclude("x.h") inside @cImport
	kindBuild    = "bpath"    // b.path("x") or .{ .path = "x" }: from the build root
	kindDep      = "dep"      // b.dependency("x", ...): a build.zig.zon dependency
	kindZon      = "zon"      // a build.zig.zon dependency: Module "url\nhash\npath"
	kindZigVer   = "zigver"   // build.zig.zon .minimum_zig_version
)

// maxNest bounds the recursion into nested containers, so a pathological file
// cannot run the stack out.
const maxNest = 64

// source is what a Zig file declares and imports.
type source struct {
	tokens  []tok
	m       []int
	symbols lang.SymbolSet
	imports []lang.RawImport
	seen    map[string]bool
}

// readSource extracts a Zig file's imports and top-level declarations.
//
// Implements: REQ-ZIG-002, REQ-ZIG-003
func readSource(src []byte) *source {
	tokens := lex(src)
	s := &source{tokens: tokens, m: match(tokens), seen: map[string]bool{}}
	s.container(0, len(tokens), "", 0)
	s.readImports(buildAPI(src))
	return s
}

// buildAPI reports whether a file uses the build system's API, whose b.path("x")
// and .{ .path = "x" } name files from the build root. Other code writes .path =
// for its own purposes.
func buildAPI(src []byte) bool {
	s := string(src)
	return strings.Contains(s, "std.Build") || strings.Contains(s, "std.build") || strings.Contains(s, "*Build")
}

func (s *source) at(i int, kind int, text string) bool {
	return i >= 0 && i < len(s.tokens) && s.tokens[i].kind == kind && s.tokens[i].text == text
}

func (s *source) punct(i int, text string) bool { return s.at(i, tPunct, text) }

// skip returns the index after the token at i, stepping over a bracketed group.
func (s *source) skip(i int) int {
	if s.m[i] > i {
		return s.m[i] + 1
	}
	return i + 1
}

// until returns the index of the first of the stop punctuation at bracket depth 0
// from i, or end.
func (s *source) until(i, end int, stop ...string) int {
	for i < end {
		t := s.tokens[i]
		if t.kind == tPunct {
			for _, p := range stop {
				if t.text == p {
					return i
				}
			}
		}
		i = s.skip(i)
	}
	return end
}

var containerKW = map[string]bool{"struct": true, "enum": true, "union": true, "opaque": true}

// container reads the members of a container body (the file itself is one) from i
// to end: functions, tests, constants and variables, and nested container types,
// named Owner.Name. Declarations inside function bodies are not symbols.
func (s *source) container(i, end int, owner string, depth int) {
	qual := func(name string) string {
		if owner == "" {
			return name
		}
		return owner + "." + name
	}
	for i < end {
		t := s.tokens[i]
		if t.kind != tIdent {
			i = s.skip(i)
			continue
		}
		switch t.text {
		case "pub", "export", "inline", "noinline", "threadlocal":
			i++
			continue
		case "extern":
			i++
			if s.str(i) {
				i++ // extern "c"
			}
			continue
		case "fn":
			i = s.fn(i, end, owner, qual)
			continue
		case "test":
			j := i + 1
			if j < end && (s.tokens[j].kind == tStr || s.tokens[j].kind == tIdent) {
				s.symbols.Add(s.tokens[j].text, "test", t.line)
				j++
			}
			if s.punct(j, "{") {
				j = s.skip(j)
			}
			i = j
			continue
		case "comptime":
			if s.punct(i+1, "{") {
				i = s.skip(i + 1) // a comptime block declares nothing
			} else {
				i++ // a comptime field
			}
			continue
		case "usingnamespace":
			i = s.until(i+1, end, ";") + 1
			continue
		case "const", "var":
			i = s.decl(i, end, t.text, owner, qual, depth)
			continue
		}
		// A field (name: Type = default,) or anything else: to the next member.
		i = s.until(i, end, ",", ";") + 1
	}
}

func (s *source) text(i int) string {
	if i < 0 || i >= len(s.tokens) {
		return ""
	}
	return s.tokens[i].text
}

// fn records a function (a method inside a container) and steps over its header
// and body. The return type may hold braces of its own (error{A}!void, struct {
// ... }), which are not the body.
func (s *source) fn(i, end int, owner string, qual func(string) string) int {
	line := s.tokens[i].line
	i++
	if i < end && s.tokens[i].kind == tIdent {
		kind := "func"
		if owner != "" {
			kind = "method"
		}
		s.symbols.Add(qual(s.tokens[i].text), kind, line)
		i++
	}
	for i < end {
		t := s.tokens[i]
		if t.kind == tPunct && t.text == ";" {
			return i + 1 // extern fn or a function type
		}
		if t.kind == tPunct && t.text == "{" {
			if s.typeBody(i) {
				i = s.skip(i)
				continue
			}
			return s.skip(i)
		}
		i = s.skip(i)
	}
	return i
}

// typeBody reports whether the brace at i opens a type (error{...}, struct {...},
// enum(u8) {...}) rather than a block.
func (s *source) typeBody(i int) bool {
	p := i - 1
	if s.punct(p, ")") && s.m[p] >= 0 {
		p = s.m[p] - 1 // enum(u8), union(enum), packed struct(u32)
	}
	return p >= 0 && s.tokens[p].kind == tIdent && (containerKW[s.tokens[p].text] || s.tokens[p].text == "error")
}

// decl reads const/var Name [: Type] = value;. A container value becomes a type
// symbol whose members are read with Name as their owner; an @import or @cImport
// is not a symbol (it names another file); other values are constants or variables at the
// file's top level only.
func (s *source) decl(i, end int, kw, owner string, qual func(string) string, depth int) int {
	line := s.tokens[i].line
	i++
	if i >= end || s.tokens[i].kind != tIdent {
		return i
	}
	name := s.tokens[i].text
	stop := s.until(i+1, end, ";", "}")
	eq := s.until(i+1, stop, "=")
	if eq >= stop {
		if owner == "" {
			s.symbols.Add(name, kw, line) // extern var x: T;
		}
		return stop + 1
	}
	v := eq + 1
	for v < stop && s.tokens[v].kind == tIdent && (s.tokens[v].text == "extern" || s.tokens[v].text == "packed") {
		v++
	}
	if v < stop && s.tokens[v].kind == tIdent && (containerKW[s.tokens[v].text] || s.tokens[v].text == "error") {
		b := v + 1
		if s.punct(b, "(") {
			b = s.skip(b)
		}
		if s.punct(b, "{") && s.m[b] > b {
			s.symbols.Add(qual(name), "type", line)
			if depth < maxNest && s.tokens[v].text != "error" {
				s.container(b+1, s.m[b], qual(name), depth+1)
			}
			return stop + 1
		}
	}
	if owner == "" && !(v < stop && s.tokens[v].kind == tBuiltin && (s.tokens[v].text == "@import" || s.tokens[v].text == "@cImport")) {
		s.symbols.Add(name, kw, line)
	}
	return stop + 1
}

// readImports collects @import, @embedFile and @cInclude with a literal argument
// anywhere in the file, and in build code b.path("x"), the older .{ .path = "x" }
// and b.dependency("x", ...). Repeats of one spec are one import.
func (s *source) readImports(build bool) {
	for i, t := range s.tokens {
		switch {
		case t.kind == tBuiltin:
			kind := builtinKinds[t.text]
			if kind != "" && s.punct(i+1, "(") && s.str(i+2) && s.punct(i+3, ")") {
				arg := s.tokens[i+2].text
				s.add(t.text+"("+quote(arg)+")", arg, kind, t.line)
			}
		case t.kind == tIdent && (t.text == "dependency" || t.text == "lazyDependency"):
			if s.punct(i-1, ".") && s.punct(i+1, "(") && s.str(i+2) && i >= 2 && s.tokens[i-2].kind == tIdent {
				arg := s.tokens[i+2].text
				s.add(s.tokens[i-2].text+"."+t.text+"("+quote(arg)+")", arg, kindDep, t.line)
			}
		case !build:
		case t.kind == tIdent && t.text == "path" && s.punct(i-1, ".") && s.punct(i+1, "(") && s.str(i+2) && s.punct(i+3, ")"):
			if r := s.text(i - 2); (r == "b" || r == "builder") && !s.punct(i-3, ".") {
				arg := s.tokens[i+2].text
				s.add(r+".path("+quote(arg)+")", arg, kindBuild, t.line)
			}
		case t.kind == tIdent && t.text == "path" && s.punct(i-1, ".") && s.punct(i+1, "=") && s.str(i+2) && s.punct(i+3, "}") && s.punct(i-2, "{"):
			arg := s.tokens[i+2].text
			s.add(".{ .path = "+quote(arg)+" }", arg, kindBuild, t.line)
		}
	}
}

var builtinKinds = map[string]string{"@import": kindImport, "@embedFile": kindEmbed, "@cInclude": kindCInclude}

func (s *source) str(i int) bool { return i >= 0 && i < len(s.tokens) && s.tokens[i].kind == tStr }

func (s *source) add(spec, module, kind string, line int) {
	if module == "" || s.seen[spec] {
		return
	}
	s.seen[spec] = true
	s.imports = append(s.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

func quote(s string) string { return `"` + s + `"` }
