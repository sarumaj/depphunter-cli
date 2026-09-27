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
	name string
	line int
	val  zval
}

func (v zval) get(name string) (zval, bool) {
	for _, f := range v.fields {
		if f.name == name {
			return f.val, true
		}
	}
	return zval{}, false
}

// str is a string or enum literal field's text: the package name is .shop in
// build.zig.zon since Zig 0.14 and "shop" before.
func (v zval) str(name string) string {
	f, ok := v.get(name)
	if ok && (f.kind == '"' || f.kind == '.') {
		return f.text
	}
	return ""
}

// parseZon reads a ZON document. It never fails: what it cannot read becomes an
// "other" value.
func parseZon(src []byte) zval {
	tokens := lex(src)
	p := &zparser{tokens: tokens}
	v, _ := p.value(0, 0)
	return v
}

type zparser struct{ tokens []tok }

func (p *zparser) is(i int, kind int, text string) bool {
	return i < len(p.tokens) && p.tokens[i].kind == kind && p.tokens[i].text == text
}

func (p *zparser) value(i, depth int) (zval, int) {
	if i >= len(p.tokens) {
		return zval{kind: '?'}, i
	}
	t := p.tokens[i]
	switch {
	case t.kind == tStr:
		return zval{kind: '"', text: t.text, line: t.line}, i + 1
	case p.is(i, tPunct, ".") && p.is(i+1, tPunct, "{"):
		return p.aggregate(i+2, t.line, depth)
	case p.is(i, tPunct, ".") && i+1 < len(p.tokens) && p.tokens[i+1].kind == tIdent:
		return zval{kind: '.', text: p.tokens[i+1].text, line: t.line}, i + 2
	}
	return zval{kind: '?', text: t.text, line: t.line}, i + 1
}

// aggregate reads the inside of .{ ... } from i, up to and past its closing brace.
func (p *zparser) aggregate(i, line, depth int) (zval, int) {
	v := zval{kind: 't', line: line}
	for i < len(p.tokens) {
		switch {
		case p.is(i, tPunct, "}"):
			return v, i + 1
		case p.is(i, tPunct, ","):
			i++
			continue
		}
		if depth >= maxNest {
			i++ // too deep: skip to the closing brace without reading
			continue
		}
		start := i
		if p.is(i, tPunct, ".") && i+2 < len(p.tokens) && p.tokens[i+1].kind == tIdent && p.is(i+2, tPunct, "=") {
			v.kind = 's'
			name, fline := p.tokens[i+1].text, p.tokens[i+1].line
			var val zval
			val, i = p.value(i+3, depth+1)
			v.fields = append(v.fields, zfield{name: name, line: fline, val: val})
		} else {
			var val zval
			val, i = p.value(i, depth+1)
			v.items = append(v.items, val)
		}
		if i == start {
			i++
		}
	}
	return v, i
}

// zonDep is one entry of a build.zig.zon's .dependencies.
type zonDep struct {
	key, url, hash, path string
	lazy                 bool
	line                 int
}

// zonFile is what a build.zig.zon declares.
type zonFile struct {
	name, version, minZig string
	nameLine, minZigLine  int
	deps                  []zonDep
	paths                 []string
}

// readZon reads a build.zig.zon: the package's name, version and minimum Zig
// version, its .paths and its dependencies with their url and hash, or path, and
// whether they are lazy.
//
// Implements: REQ-ZIG-007
func readZon(src []byte) *zonFile {
	root := parseZon(src)
	z := &zonFile{name: root.str("name"), version: root.str("version"), minZig: root.str("minimum_zig_version")}
	for _, f := range root.fields {
		switch f.name {
		case "name":
			z.nameLine = f.line
		case "minimum_zig_version":
			z.minZigLine = f.line
		case "paths":
			for _, it := range f.val.items {
				if it.kind == '"' {
					z.paths = append(z.paths, it.text)
				}
			}
		case "dependencies":
			for _, d := range f.val.fields {
				dep := zonDep{key: d.name, line: d.line, url: d.val.str("url"), hash: d.val.str("hash"), path: d.val.str("path")}
				if l, ok := d.val.get("lazy"); ok && l.text == "true" {
					dep.lazy = true
				}
				z.deps = append(z.deps, dep)
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
func extractZon(src []byte) *lang.Extraction {
	z := readZon(src)
	ex := &lang.Extraction{Symbols: []lang.Symbol{}}
	if z.name != "" {
		ex.Symbols = append(ex.Symbols, lang.Symbol{Name: z.name, Kind: "package", Line: max(z.nameLine, 1)})
	}
	if z.minZig != "" {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: ".minimum_zig_version", Module: z.minZig, Name: kindZigVer, Line: z.minZigLine})
	}
	seen := map[string]bool{}
	for _, d := range z.deps {
		if seen[d.key] {
			continue
		}
		seen[d.key] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: d.key, Module: d.url + "\n" + d.hash + "\n" + d.path, Name: kindZon, Line: d.line})
	}
	return ex
}
