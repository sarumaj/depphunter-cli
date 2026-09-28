package haskell

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// What a source import's RawImport.Name says about it.
const (
	kindImport = "import" // import M
	kindSource = "source" // import {-# SOURCE #-} M: the module's .hs-boot file
	// kindPackage prefixes the package a PackageImports import names: "pkg:text".
	kindPackage = "pkg:"
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
type chunk []token

// chunks splits a module's tokens into top-level declarations. A declaration starts
// with a token first on its line in the leftmost column any line starts in (the
// first column, or the third after literate bird tracks), outside brackets, unless it
// only continues the previous one: a module header whose export list is written at
// the margin runs to its where, and a line starting with a closing bracket, a comma,
// where or deriving continues.
func chunks(tokens []token) []chunk {
	base := 0
	for _, t := range tokens {
		if t.lineStart && t.k != tPragma && (base == 0 || t.column < base) {
			base = t.column
		}
	}
	var out []chunk
	start, depth, header := -1, 0, false
	for i, t := range tokens {
		if t.lineStart && t.column == base && depth <= 0 && !header && !continues(t) {
			if start >= 0 {
				out = append(out, tokens[start:i])
			}
			start, depth = i, 0
			header = t.k == tVariable && t.s == "module"
		}
		switch {
		case t.k == tPunctuation && (t.s == "(" || t.s == "[" || t.s == "{"):
			depth++
		case t.k == tPunctuation && (t.s == ")" || t.s == "]" || t.s == "}"):
			depth--
		case t.k == tVariable && t.s == "where" && depth <= 0:
			header = false
		}
	}
	if start >= 0 {
		out = append(out, tokens[start:])
	}
	return out
}

func continues(t token) bool {
	switch {
	case t.k == tPunctuation:
		return t.s != "(" && t.s != "["
	case t.k == tVariable:
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
func (c chunk) top(from int, match func(token) bool) int {
	depth := 0
	for i := from; i < len(c); i++ {
		t := c[i]
		switch {
		case t.k == tPunctuation && (t.s == "(" || t.s == "[" || t.s == "{"):
			depth++
		case t.k == tPunctuation && (t.s == ")" || t.s == "]" || t.s == "}"):
			depth--
		}
		if depth == 0 && match(t) {
			return i
		}
	}
	return -1
}

func is(k int, s string) func(token) bool {
	return func(t token) bool { return t.k == k && t.s == s }
}

// render writes tokens back as text, one space between them except inside brackets.
func render(text chunk) string {
	var b strings.Builder
	for i, t := range text {
		s := t.s
		switch t.k {
		case tString:
			s = `"` + s + `"`
		}
		if i > 0 {
			previous := text[i-1]
			if !(previous.k == tPunctuation && (previous.s == "(" || previous.s == "[")) &&
				!(t.k == tPunctuation && (t.s == ")" || t.s == "]" || t.s == ",")) &&
				!(previous.k == tOp && (previous.s == "'" || previous.s == "''")) {
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
func extractSource(source []byte, literate bool) *lang.Extraction {
	if literate {
		source = unlit(source)
	}
	extraction := &lang.Extraction{}
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
	for _, raw := range chunks(lex(source)) {
		c := raw.words()
		if len(c) == 0 {
			continue
		}
		head := c[0]
		line := head.line
		switch {
		case head.k == tVariable && head.s == "module":
			if len(c) > 1 && c[1].k == tCon {
				add(c[1].s, "module", line)
			}
		case head.k == tVariable && head.s == "import":
			if rawImport, ok := importOf(raw); ok && !specs[rawImport.Spec] {
				specs[rawImport.Spec] = true
				extraction.Imports = append(extraction.Imports, rawImport)
			}
		case head.k == tVariable && (head.s == "data" || head.s == "newtype" || head.s == "type"):
			name, kind := typeDeclaration(c)
			add(name, kind, line)
		case head.k == tVariable && head.s == "class":
			name := typeName(c, 1)
			add(name, "class", line)
			for _, m := range classMethods(c) {
				add(name+"."+m.s, "method", m.line)
			}
		case head.k == tVariable && (head.s == "instance" || head.s == "deriving"):
			if i := c.top(0, is(tVariable, "instance")); i >= 0 {
				add(instanceName(c, i), "instance", line)
			}
		case head.k == tVariable && head.s == "pattern" && patternNames(c) != nil:
			for _, n := range patternNames(c) {
				add(n, "pattern", line)
			}
		case head.k == tVariable && head.s == "foreign":
			if i := c.top(0, is(tOp, "::")); i > 0 && c[i-1].k == tVariable {
				add(c[i-1].s, "function", line)
			}
		case head.k == tVariable && !keywords[head.s] || head.k == tPunctuation && head.s == "(":
			for _, n := range functionNames(c) {
				add(n, "function", line)
			}
		}
	}
	extraction.Symbols = symbols.List()
	return extraction
}

// importOf reads one import declaration:
//
//	import [{-# SOURCE #-}] [safe] [qualified] ["package"] M [qualified] [as N] [hiding] [(...)]
//
// Its spec is the declaration without its import list, so two imports of one module
// (one qualified, one not) stay apart.
func importOf(c chunk) (lang.RawImport, bool) {
	if len(c) < 2 || c[0].k != tVariable || c[0].s != "import" {
		return lang.RawImport{}, false
	}
	rawImport := lang.RawImport{Name: kindImport, Line: c[0].line}
	parts := []string{"import"}
loop:
	for _, t := range c[1:] {
		switch {
		case t.k == tPragma:
			if strings.EqualFold(t.s, "SOURCE") {
				rawImport.Name = kindSource
				parts = append(parts, "{-# SOURCE #-}")
			}
			continue
		case t.k == tVariable && (t.s == "safe" || t.s == "qualified") && rawImport.Module == "":
			parts = append(parts, t.s)
			continue
		case t.k == tString && rawImport.Module == "":
			if rawImport.Name == kindImport {
				rawImport.Name = kindPackage + t.s
			}
			parts = append(parts, `"`+t.s+`"`)
			continue
		case t.k == tCon && rawImport.Module == "":
			rawImport.Module = t.s
			parts = append(parts, t.s)
			continue
		case rawImport.Module == "":
			return lang.RawImport{}, false
		case t.k == tVariable && t.s == "qualified":
			parts = append(parts, t.s)
			continue
		case t.k == tVariable && t.s == "as":
			parts = append(parts, t.s)
			continue
		case t.k == tCon && parts[len(parts)-1] == "as":
			parts = append(parts, t.s)
			continue
		}
		break loop
	}
	if rawImport.Module == "" {
		return lang.RawImport{}, false
	}
	rawImport.Spec = strings.Join(parts, " ")
	return rawImport, true
}

// typeDeclaration names a data, newtype or type declaration: its type, a type or data
// family, or an instance of one ("type instance F Int"); type role annotations are
// not declarations.
func typeDeclaration(c chunk) (string, string) {
	i := 1
	kind := "type"
	if i < len(c) && c[i].k == tVariable {
		switch c[i].s {
		case "family":
			kind = c[0].s + " family"
			i++
		case "instance":
			end := c.top(i, func(t token) bool {
				return t.k == tOp && (t.s == "=" || t.s == "::") || t.k == tVariable && (t.s == "where" || t.s == "deriving")
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
	end := c.top(from, func(t token) bool {
		return t.k == tOp && (t.s == "=" || t.s == "|") || t.k == tVariable && t.s == "where"
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
		case t.k == tPunctuation && t.s == "(" && i+2 < end && c[i+1].k == tOp && c[i+2].s == ")":
			return c[i+1].s
		case t.k == tOp && t.s != "'" && t.s != "''" && t.s != "::" && t.s != "@":
			return t.s
		case t.k == tVariable || t.k == tPunctuation && t.s == "`":
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
	end := c.top(at, func(t token) bool { return t.k == tVariable && t.s == "where" })
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
func classMethods(c chunk) []token {
	w := c.top(0, is(tVariable, "where"))
	if w < 0 {
		return nil
	}
	column := 0
	for _, t := range c[w+1:] {
		if t.lineStart && (column == 0 || t.column < column) {
			column = t.column
		}
	}
	var out []token
	for i := w + 1; i < len(c); i++ {
		if !c[i].lineStart || c[i].column != column {
			continue
		}
		out = append(out, signatureNamesAt(c, i)...)
	}
	return out
}

// signatureNames reads "f, (<+>), g ::" from index from, the names before the colons.
func signatureNames(c chunk, from int) []string {
	var out []string
	for _, t := range signatureNamesAt(c, from) {
		out = append(out, t.s)
	}
	return out
}

func signatureNamesAt(c chunk, from int) []token {
	var out []token
	i := from
	for i < len(c) {
		switch {
		case c[i].k == tVariable && !keywords[c[i].s] || c[i].k == tCon:
			out = append(out, c[i])
			i++
		case c[i].k == tPunctuation && c[i].s == "(" && i+2 < len(c) && c[i+1].k == tOp && c[i+2].s == ")":
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
		case c[i].k == tPunctuation && c[i].s == ",":
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
	if names := signatureNames(c, 0); names != nil {
		for _, n := range names {
			if n[0] >= 'A' && n[0] <= 'Z' {
				return nil // constructors: not a signature
			}
		}
		return names
	}
	equalsAt := c.top(0, func(t token) bool { return t.k == tOp && (t.s == "=" || t.s == "|") })
	if equalsAt < 1 {
		return nil
	}
	head := c[0]
	if head.k == tPunctuation {
		if len(c) > 3 && c[1].k == tOp && c[2].s == ")" && equalsAt > 2 {
			return []string{c[1].s}
		}
		return nil
	}
	if equalsAt >= 3 && c[1].k == tPunctuation && c[1].s == "`" && c[2].k == tVariable {
		return []string{c[2].s} // a `op` b = ...
	}
	if equalsAt >= 2 && c[1].k == tOp && !reservedOp[c[1].s] && !prefixPattern(c, 1) {
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
	return i+1 < len(c) && c[i+1].line == t.line && c[i+1].column == t.end && c[i-1].end < t.column
}

// patternNames are the synonyms a PatternSynonyms declaration introduces: pattern P,
// Q :: T, pattern P a <- ..., pattern (:>) ..., pattern x :> y = .... nil means the
// line defines a function called pattern.
func patternNames(c chunk) []string {
	if len(c) < 3 {
		return nil
	}
	if names := signatureNames(c, 1); names != nil {
		return names
	}
	switch {
	case c[1].k == tCon:
		return []string{c[1].s}
	case c[1].k == tPunctuation && c[1].s == "(" && c[2].k == tOp && strings.HasPrefix(c[2].s, ":"):
		return []string{c[2].s}
	case c[1].k == tVariable && c[2].k == tOp && strings.HasPrefix(c[2].s, ":") && c[2].s != "::":
		return []string{c[2].s}
	}
	return nil
}
