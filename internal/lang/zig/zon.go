package zig

import (
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A ZON value: an anonymous struct literal .{ .a = x } (fields), a tuple .{ x, y }
// (items), a string, an enum literal .name, or anything else by its text.
type zval struct {
	kind   byte // 's' struct, 't' tuple, '"' string, '.' enum literal, '?' other
	text   string
	line   int
	fields []zfield
	items  []zval
}

type zfield struct {
	name  string
	line  int
	value zval
}

func (v zval) get(name string) (zval, bool) {
	for _, f := range v.fields {
		if f.name == name {
			return f.value, true
		}
	}
	return zval{}, false
}

// stringField is a string or enum literal field's text: the package name is .shop in
// build.zig.zon since Zig 0.14 and "shop" before.
func (v zval) stringField(name string) string {
	f, ok := v.get(name)
	if ok && (f.kind == '"' || f.kind == '.') {
		return f.text
	}
	return ""
}

// parseZon reads a ZON document. It never fails: what it cannot read becomes an
// "other" value.
func parseZon(source []byte) zval {
	tokens := lex(source)
	p := &zparser{tokens: tokens}
	v, _ := p.value(0, 0)
	return v
}

type zparser struct{ tokens []token }

func (p *zparser) is(i int, kind int, text string) bool {
	return i < len(p.tokens) && p.tokens[i].kind == kind && p.tokens[i].text == text
}

func (p *zparser) value(i, depth int) (zval, int) {
	if i >= len(p.tokens) {
		return zval{kind: '?'}, i
	}
	t := p.tokens[i]
	switch {
	case t.kind == tString:
		return zval{kind: '"', text: t.text, line: t.line}, i + 1
	case p.is(i, tPunctuation, ".") && p.is(i+1, tPunctuation, "{"):
		return p.aggregate(i+2, t.line, depth)
	case p.is(i, tPunctuation, ".") && i+1 < len(p.tokens) && p.tokens[i+1].kind == tIdentifier:
		return zval{kind: '.', text: p.tokens[i+1].text, line: t.line}, i + 2
	}
	return zval{kind: '?', text: t.text, line: t.line}, i + 1
}

// aggregate reads the inside of .{ ... } from i, up to and past its closing brace.
func (p *zparser) aggregate(i, line, depth int) (zval, int) {
	v := zval{kind: 't', line: line}
	for i < len(p.tokens) {
		switch {
		case p.is(i, tPunctuation, "}"):
			return v, i + 1
		case p.is(i, tPunctuation, ","):
			i++
			continue
		}
		if depth >= maxNest {
			i++ // too deep: skip to the closing brace without reading
			continue
		}
		start := i
		if p.is(i, tPunctuation, ".") && i+2 < len(p.tokens) && p.tokens[i+1].kind == tIdentifier && p.is(i+2, tPunctuation, "=") {
			v.kind = 's'
			name, fline := p.tokens[i+1].text, p.tokens[i+1].line
			var value zval
			value, i = p.value(i+3, depth+1)
			v.fields = append(v.fields, zfield{name: name, line: fline, value: value})
		} else {
			var value zval
			value, i = p.value(i, depth+1)
			v.items = append(v.items, value)
		}
		if i == start {
			i++
		}
	}
	return v, i
}

// zonDependency is one entry of a build.zig.zon's .dependencies.
type zonDependency struct {
	key, url, hash, path string
	lazy                 bool
	line                 int
}

// zonFile is what a build.zig.zon declares.
type zonFile struct {
	name, version, minZig string
	nameLine, minZigLine  int
	dependencies          []zonDependency
	paths                 []string
}

// readZon reads a build.zig.zon: the package's name, version and minimum Zig
// version, its .paths and its dependencies with their url and hash, or path, and
// whether they are lazy.
//
// Implements: REQ-ZIG-007
func readZon(source []byte) *zonFile {
	root := parseZon(source)
	z := &zonFile{name: root.stringField("name"), version: root.stringField("version"), minZig: root.stringField("minimum_zig_version")}
	for _, f := range root.fields {
		switch f.name {
		case "name":
			z.nameLine = f.line
		case "minimum_zig_version":
			z.minZigLine = f.line
		case "paths":
			for _, it := range f.value.items {
				if it.kind == '"' {
					z.paths = append(z.paths, it.text)
				}
			}
		case "dependencies":
			for _, d := range f.value.fields {
				dependency := zonDependency{key: d.name, line: d.line, url: d.value.stringField("url"), hash: d.value.stringField("hash"), path: d.value.stringField("path")}
				if l, ok := d.value.get("lazy"); ok && l.text == "true" {
					dependency.lazy = true
				}
				z.dependencies = append(z.dependencies, dependency)
			}
		}
	}
	return z
}

// extractZon makes a build.zig.zon's dependencies imports (spec = the dependency's
// name) and its minimum Zig version one of the compiler; the package's name is its
// symbol.
//
// Implements: REQ-ZIG-007
func extractZon(source []byte) *lang.Extraction {
	z := readZon(source)
	extraction := &lang.Extraction{Symbols: []lang.Symbol{}}
	if z.name != "" {
		extraction.Symbols = append(extraction.Symbols, lang.Symbol{Name: z.name, Kind: "package", Line: max(z.nameLine, 1)})
	}
	if z.minZig != "" {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: ".minimum_zig_version", Module: z.minZig, Name: kindZigVersion, Line: z.minZigLine})
	}
	seen := map[string]bool{}
	for _, d := range z.dependencies {
		if seen[d.key] {
			continue
		}
		seen[d.key] = true
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: d.key, Module: d.url + "\n" + d.hash + "\n" + d.path, Name: kindZon, Line: d.line})
	}
	return extraction
}
