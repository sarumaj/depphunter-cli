package haskell

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// What a source import's RawImport.Name says about it.
const (
	kindImport = "import" // import M
	kindSource = "source" // import {-# SOURCE #-} M: the module's .hs-boot file
	// kindPkg prefixes the package a PackageImports import names: "pkg:text".
	kindPkg = "pkg:"
)

// keywords can not name a top-level function.
var keywords = map[string]bool{
	"case": true, "class": true, "data": true, "default": true, "deriving": true, "do": true,
	"else": true, "foreign": true, "if": true, "import": true, "in": true, "infix": true,
	"infixl": true, "infixr": true, "instance": true, "let": true, "module": true, "newtype": true,
	"of": true, "then": true, "type": true, "where": true, "_": true, "mdo": true, "rec": true,
	"proc": true,
}

// chunk is one top-level declaration: the tokens from one that starts a line in the
// module's first column up to the next.
type chunk []tok

// chunks splits a module's tokens into top-level declarations. A declaration starts
// with a token first on its line in the leftmost column any line starts in (the
// first column, or the third after literate bird tracks), outside brackets, unless it
// only continues the previous one: a module header whose export list is written at
// the margin runs to its where, and a line starting with a closing bracket, a comma,
// where or deriving continues.
func chunks(tokens []tok) []chunk {
	base := 0
	for _, t := range tokens {
		if t.bol && t.k != tPragma && (base == 0 || t.col < base) {
			base = t.col
		}
	}
	var out []chunk
	start, depth, header := -1, 0, false
	for i, t := range tokens {
		if t.bol && t.col == base && depth <= 0 && !header && !continues(t) {
			if start >= 0 {
				out = append(out, tokens[start:i])
			}
			start, depth = i, 0
			header = t.k == tVar && t.s == "module"
		}
		switch {
		case t.k == tPunct && (t.s == "(" || t.s == "[" || t.s == "{"):
			depth++
		case t.k == tPunct && (t.s == ")" || t.s == "]" || t.s == "}"):
			depth--
		case t.k == tVar && t.s == "where" && depth <= 0:
			header = false
		}
	}
	if start >= 0 {
		out = append(out, tokens[start:])
	}
	return out
}

func continues(t tok) bool {
	switch {
	case t.k == tPunct:
		return t.s != "(" && t.s != "["
	case t.k == tVar:
		return t.s == "where" || t.s == "then" || t.s == "else" || t.s == "of"
	case t.k == tOp:
		return t.s == "|" || t.s == "=" || t.s == "->" || t.s == "=>" || t.s == "::"
	}
	return false
}

// words drops the pragmas of a declaration, which may sit anywhere in it
// (instance {-# OVERLAPPING #-} Show T).
func (c chunk) words() chunk {
	out := make(chunk, 0, len(c))
	for _, t := range c {
		if t.k != tPragma {
			out = append(out, t)
		}
	}
	return out
}

// top returns the index of the first token s outside brackets at or after from, or -1.
func (c chunk) top(from int, match func(tok) bool) int {
	depth := 0
	for i := from; i < len(c); i++ {
		t := c[i]
		switch {
		case t.k == tPunct && (t.s == "(" || t.s == "[" || t.s == "{"):
			depth++
		case t.k == tPunct && (t.s == ")" || t.s == "]" || t.s == "}"):
			depth--
		}
		if depth == 0 && match(t) {
			return i
		}
	}
	return -1
}

func is(k int, s string) func(tok) bool {
	return func(t tok) bool { return t.k == k && t.s == s }
}

// render writes tokens back as text, one space between them except inside brackets.
func render(ts chunk) string {
	var b strings.Builder
	for i, t := range ts {
		s := t.s
		switch t.k {
		case tStr:
			s = `"` + s + `"`
		}
		if i > 0 {
			prev := ts[i-1]
			if !(prev.k == tPunct && (prev.s == "(" || prev.s == "[")) &&
				!(t.k == tPunct && (t.s == ")" || t.s == "]" || t.s == ",")) &&
				!(prev.k == tOp && (prev.s == "'" || prev.s == "''")) {
				b.WriteByte(' ')
			}
		}
		b.WriteString(s)
		if b.Len() > 120 {
			return b.String()[:117] + "..."
		}
	}
	return b.String()
}

// extractSource reads a module's imports and top-level declarations.
//
// Implements: REQ-HASKELL-002, REQ-HASKELL-003, REQ-HASKELL-011
func extractSource(src []byte, literate bool) *lang.Extraction {
	if literate {
		src = unlit(src)
	}
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	seen := map[string]bool{}
	add := func(name, kind string, line int) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		symbols.Add(name, kind, line)
	}
	specs := map[string]bool{}
	for _, raw := range chunks(lex(src)) {
		c := raw.words()
		if len(c) == 0 {
			continue
		}
		head := c[0]
		line := head.line
		switch {
		case head.k == tVar && head.s == "module":
			if len(c) > 1 && c[1].k == tCon {
				add(c[1].s, "module", line)
			}
		case head.k == tVar && head.s == "import":
			if im, ok := importOf(raw); ok && !specs[im.Spec] {
				specs[im.Spec] = true
				ex.Imports = append(ex.Imports, im)
			}
		case head.k == tVar && (head.s == "data" || head.s == "newtype" || head.s == "type"):
			name, kind := typeDecl(c)
			add(name, kind, line)
		case head.k == tVar && head.s == "class":
			name := typeName(c, 1)
			add(name, "class", line)
			for _, m := range classMethods(c) {
				add(name+"."+m.s, "method", m.line)
			}
		case head.k == tVar && (head.s == "instance" || head.s == "deriving"):
			if i := c.top(0, is(tVar, "instance")); i >= 0 {
				add(instanceName(c, i), "instance", line)
			}
		case head.k == tVar && head.s == "pattern" && patternNames(c) != nil:
			for _, n := range patternNames(c) {
				add(n, "pattern", line)
			}
		case head.k == tVar && head.s == "foreign":
			if i := c.top(0, is(tOp, "::")); i > 0 && c[i-1].k == tVar {
				add(c[i-1].s, "function", line)
			}
		case head.k == tVar && !keywords[head.s] || head.k == tPunct && head.s == "(":
			for _, n := range functionNames(c) {
				add(n, "function", line)
			}
		}
	}
	ex.Symbols = symbols.List()
	return ex
}

// importOf reads one import declaration:
//
//	import [{-# SOURCE #-}] [safe] [qualified] ["package"] M [qualified] [as N] [hiding] [(...)]
//
// Its spec is the declaration without its import list, so two imports of one module
// (one qualified, one not) stay apart.
func importOf(c chunk) (lang.RawImport, bool) {
	if len(c) < 2 || c[0].k != tVar || c[0].s != "import" {
		return lang.RawImport{}, false
	}
	im := lang.RawImport{Name: kindImport, Line: c[0].line}
	parts := []string{"import"}
loop:
	for _, t := range c[1:] {
		switch {
		case t.k == tPragma:
			if strings.EqualFold(t.s, "SOURCE") {
				im.Name = kindSource
				parts = append(parts, "{-# SOURCE #-}")
			}
			continue
		case t.k == tVar && (t.s == "safe" || t.s == "qualified") && im.Module == "":
			parts = append(parts, t.s)
			continue
		case t.k == tStr && im.Module == "":
			if im.Name == kindImport {
				im.Name = kindPkg + t.s
			}
			parts = append(parts, `"`+t.s+`"`)
			continue
		case t.k == tCon && im.Module == "":
			im.Module = t.s
			parts = append(parts, t.s)
			continue
		case im.Module == "":
			return lang.RawImport{}, false
		case t.k == tVar && t.s == "qualified":
			parts = append(parts, t.s)
			continue
		case t.k == tVar && t.s == "as":
			parts = append(parts, t.s)
			continue
		case t.k == tCon && parts[len(parts)-1] == "as":
			parts = append(parts, t.s)
			continue
		}
		break loop
	}
	if im.Module == "" {
		return lang.RawImport{}, false
	}
	im.Spec = strings.Join(parts, " ")
	return im, true
}

// typeDecl names a data, newtype or type declaration: its type, a type or data
// family, or an instance of one ("type instance F Int"); type role annotations are
// not declarations.
func typeDecl(c chunk) (string, string) {
	i := 1
	kind := "type"
	if i < len(c) && c[i].k == tVar {
		switch c[i].s {
		case "family":
			kind = c[0].s + " family"
			i++
		case "instance":
			end := c.top(i, func(t tok) bool {
				return t.k == tOp && (t.s == "=" || t.s == "::") || t.k == tVar && (t.s == "where" || t.s == "deriving")
			})
			if end < 0 {
				end = len(c)
			}
			return c[0].s + " instance " + render(c[i+1:end]), "instance"
		case "role":
			return "", ""
		}
	}
	return typeName(c, i), kind
}

// typeName is the name a data, newtype, class or family declaration introduces,
// after its context: a constructor name, a parenthesized type operator, or an infix
// one (data a :+: b).
func typeName(c chunk, from int) string {
	end := c.top(from, func(t tok) bool {
		return t.k == tOp && (t.s == "=" || t.s == "|") || t.k == tVar && t.s == "where"
	})
	if end < 0 {
		end = len(c)
	}
	if ctx := c[:end].top(from, is(tOp, "=>")); ctx >= 0 {
		from = ctx + 1
	}
	for i := from; i < end; i++ {
		t := c[i]
		switch {
		case t.k == tCon:
			// data a `Pair` b: the backquoted name, else the first constructor name.
			return t.s
		case t.k == tPunct && t.s == "(" && i+2 < end && c[i+1].k == tOp && c[i+2].s == ")":
			return c[i+1].s
		case t.k == tOp && t.s != "'" && t.s != "''" && t.s != "::" && t.s != "@":
			return t.s
		case t.k == tVar || t.k == tPunct && t.s == "`":
			continue
		default:
			return ""
		}
	}
	return ""
}

// instanceName is "instance" and the instance head, without its context:
// instance Show a => Show (Tree a) is "instance Show (Tree a)".
func instanceName(c chunk, at int) string {
	end := c.top(at, func(t tok) bool { return t.k == tVar && t.s == "where" })
	if end < 0 {
		end = len(c)
	}
	from := at + 1
	if ctx := c[:end].top(from, is(tOp, "=>")); ctx >= 0 {
		from = ctx + 1
	}
	if from >= end {
		return ""
	}
	return "instance " + render(c[from:end])
}

// classMethods are the signatures of a class body: names first on a line in the
// body's leftmost column, followed by :: or by more names.
func classMethods(c chunk) []tok {
	w := c.top(0, is(tVar, "where"))
	if w < 0 {
		return nil
	}
	col := 0
	for _, t := range c[w+1:] {
		if t.bol && (col == 0 || t.col < col) {
			col = t.col
		}
	}
	var out []tok
	for i := w + 1; i < len(c); i++ {
		if !c[i].bol || c[i].col != col {
			continue
		}
		out = append(out, sigNamesAt(c, i)...)
	}
	return out
}

// sigNames reads "f, (<+>), g ::" from index from, the names before the colons.
func sigNames(c chunk, from int) []string {
	var out []string
	for _, t := range sigNamesAt(c, from) {
		out = append(out, t.s)
	}
	return out
}

func sigNamesAt(c chunk, from int) []tok {
	var out []tok
	i := from
	for i < len(c) {
		switch {
		case c[i].k == tVar && !keywords[c[i].s] || c[i].k == tCon:
			out = append(out, c[i])
			i++
		case c[i].k == tPunct && c[i].s == "(" && i+2 < len(c) && c[i+1].k == tOp && c[i+2].s == ")":
			t := c[i+1]
			out = append(out, t)
			i += 3
		default:
			return nil
		}
		if i >= len(c) {
			return nil
		}
		switch {
		case c[i].k == tOp && c[i].s == "::":
			return out
		case c[i].k == tPunct && c[i].s == ",":
			i++
		default:
			return nil
		}
	}
	return nil
}

// functionNames reads a top-level signature (f, g :: T) or a definition: f x = ...,
// f x | guard = ..., (<+>) a b = ..., infix a <+> b = ... or a `op` b = .... A
// line that neither signs nor defines (a Template Haskell splice such as
// makeLenses ”T, a pattern binding) names nothing.
func functionNames(c chunk) []string {
	if names := sigNames(c, 0); names != nil {
		for _, n := range names {
			if n[0] >= 'A' && n[0] <= 'Z' {
				return nil // constructors: not a signature
			}
		}
		return names
	}
	eq := c.top(0, func(t tok) bool { return t.k == tOp && (t.s == "=" || t.s == "|") })
	if eq < 1 {
		return nil
	}
	head := c[0]
	if head.k == tPunct {
		if len(c) > 3 && c[1].k == tOp && c[2].s == ")" && eq > 2 {
			return []string{c[1].s}
		}
		return nil
	}
	if eq >= 3 && c[1].k == tPunct && c[1].s == "`" && c[2].k == tVar {
		return []string{c[2].s} // a `op` b = ...
	}
	if eq >= 2 && c[1].k == tOp && !reservedOp[c[1].s] && !prefixPattern(c, 1) {
		return []string{c[1].s} // a <+> b = ...
	}
	if c[1].k == tOp && c[1].s == "@" || c[1].k == tOp && c[1].s == "::" {
		return nil
	}
	return []string{head.s}
}

var reservedOp = map[string]bool{"=": true, "|": true, "@": true, "::": true, "'": true, "''": true, "..": true, "\\": true, "->": true, "<-": true}

// prefixPattern tells a bang or lazy pattern (f !x = ...) from an operator defined
// infix (m ! k = ...): the pattern's ! is spaced before and not after.
func prefixPattern(c chunk, i int) bool {
	t := c[i]
	if t.s != "!" && t.s != "~" {
		return false
	}
	return i+1 < len(c) && c[i+1].line == t.line && c[i+1].col == t.end && c[i-1].end < t.col
}

// patternNames are the synonyms a PatternSynonyms declaration introduces: pattern P,
// Q :: T, pattern P a <- ..., pattern (:>) ..., pattern x :> y = .... nil means the
// line defines a function called pattern.
func patternNames(c chunk) []string {
	if len(c) < 3 {
		return nil
	}
	if names := sigNames(c, 1); names != nil {
		return names
	}
	switch {
	case c[1].k == tCon:
		return []string{c[1].s}
	case c[1].k == tPunct && c[1].s == "(" && c[2].k == tOp && strings.HasPrefix(c[2].s, ":"):
		return []string{c[2].s}
	case c[1].k == tVar && c[2].k == tOp && strings.HasPrefix(c[2].s, ":") && c[2].s != "::":
		return []string{c[2].s}
	}
	return nil
}
