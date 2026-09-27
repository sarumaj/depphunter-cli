package julia

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds a source gives, in RawImport.Name before a newline and the dotted
// module path (within the file) the statement is in.
const (
	kindUsing   = "using"
	kindInclude = "include"
)

// source is what the reader takes from one Julia file.
type source struct {
	imports []lang.RawImport
	symbols []lang.Symbol
	// modules are the modules the file defines, by their dotted path within the
	// file: "Shop", "Shop.Internal".
	modules []string
	// includes are the files it includes (relative to its directory, as written)
	// with the module path the include is in.
	includes []include
}

type include struct{ path, module string }

// Frame kinds of the reader's stack. Brackets are frames too, so an `end` inside
// brackets (a[end]) is an index, not a closer.
const (
	fModule = iota
	fFunc   // function, macro: definitions inside are local
	fType   // struct, abstract type, primitive type
	fBlock  // begin, if, try: transparent, definitions inside are top level
	fScope  // let, for, while, do, quote: definitions inside are not
	fParen
	fBracket
	fBrace
)

type frame struct {
	kind int
	name string // a module's name
}

type reader struct {
	tokens []tok
	src    []byte
	stack  []frame
	// opaque counts the frames in the stack that make a definition local, and
	// paths holds the dotted module path at each module frame: both kept as the
	// stack changes, so neither costs a walk of the stack per token.
	opaque  int
	paths   []string
	match   []int // the index of each bracket's partner, -1 when unclosed
	out     *source
	seen    map[string]bool // import specs
	symbols map[string]bool // symbol names, first definition wins
	set     lang.SymbolSet
}

// readSource reads a Julia file's imports (using, import, include), top-level
// definitions and module structure.
//
// Implements: REQ-JULIA-002, REQ-JULIA-003, REQ-JULIA-011
func readSource(src []byte) *source {
	r := &reader{tokens: lex(src), src: src, out: &source{}, seen: map[string]bool{}, symbols: map[string]bool{}}
	r.matchBrackets()
	r.run()
	r.out.symbols = r.set.List()
	return r.out
}

func (r *reader) peek(i int) *tok {
	if i < 0 || i >= len(r.tokens) {
		return &tok{kind: -1}
	}
	return &r.tokens[i]
}

func (r *reader) is(i int, kind int, text string) bool {
	t := r.peek(i)
	return t.kind == kind && t.text == text
}

// topLevel reports whether a definition here is the module's: nothing but modules
// and transparent blocks enclose it.
func (r *reader) topLevel() bool {
	return r.opaque == 0
}

// modulePath is the dotted path of the modules enclosing the current position.
func (r *reader) modulePath() string {
	if len(r.paths) == 0 {
		return ""
	}
	return r.paths[len(r.paths)-1]
}

// inBrackets reports whether the innermost frames are brackets holding a `[`: there
// `begin` and `end` are indices.
func (r *reader) indexContext() bool {
	for i := len(r.stack) - 1; i >= 0 && i >= len(r.stack)-maxScan; i-- {
		switch r.stack[i].kind {
		case fBracket:
			return true
		case fParen, fBrace:
			continue
		}
		return false
	}
	return false
}

func (r *reader) topIsBracket() bool {
	if len(r.stack) == 0 {
		return false
	}
	k := r.stack[len(r.stack)-1].kind
	return k == fParen || k == fBracket || k == fBrace
}

// maxScan bounds how far down the stack a closer looks for its opener, and
// maxModules how deep module paths are recorded: nesting beyond either is not
// code anybody wrote, and following it would make the reader quadratic.
const (
	maxScan    = 256
	maxModules = 64
)

func (r *reader) push(kind int, name string) {
	r.stack = append(r.stack, frame{kind: kind, name: name})
	switch kind {
	case fModule:
		p := r.modulePath()
		if len(r.paths) < maxModules {
			p = joinModule(p, name)
		}
		r.paths = append(r.paths, p)
	case fBlock:
	default:
		r.opaque++
	}
}

// popTo removes the frames from index j up.
func (r *reader) popTo(j int) {
	for k := len(r.stack) - 1; k >= j; k-- {
		switch r.stack[k].kind {
		case fModule:
			r.paths = r.paths[:len(r.paths)-1]
		case fBlock:
		default:
			r.opaque--
		}
	}
	r.stack = r.stack[:j]
}

// stmtStart reports whether token i starts a statement: the first token, after a
// `;`, or first on its line after a token that does not continue an expression.
func (r *reader) stmtStart(i int) bool {
	if i == 0 {
		return true
	}
	p := r.peek(i - 1)
	if p.kind == tPunct && p.text == ";" {
		return true
	}
	if p.kind == tIdent && (p.text == "begin" || p.text == "else" || p.text == "try" || p.text == "finally" || p.text == "quote") {
		return true
	}
	if !r.peek(i).nl {
		return false
	}
	if p.kind == tPunct {
		switch p.text {
		case ")", "]", "}", "'":
			return true
		}
		return false
	}
	return true
}

func (r *reader) run() {
	for i := 0; i < len(r.tokens); i++ {
		t := &r.tokens[i]
		afterDot := r.is(i-1, tPunct, ".")
		switch t.kind {
		case tPunct:
			switch t.text {
			case "(":
				r.push(fParen, "")
			case "[":
				r.push(fBracket, "")
			case "{":
				r.push(fBrace, "")
			case ")", "]", "}":
				want := map[string]int{")": fParen, "]": fBracket, "}": fBrace}[t.text]
				for j := len(r.stack) - 1; j >= 0 && j >= len(r.stack)-maxScan; j-- {
					if r.stack[j].kind == want {
						r.popTo(j)
						break
					}
				}
			}
			continue
		case tMacro:
			if t.text == "enum" && r.topLevel() {
				r.enum(i)
			}
			if r.stmtStart(i) && r.topLevel() {
				// @inline f(x) = ..., Base.@kwdef struct: the definition after the
				// macros is read where it starts.
				j := i
				for r.peek(j).kind == tMacro && r.peek(j).text != "enum" {
					j++
				}
				if j > i && (r.peek(j).kind == tIdent && !keywords[r.peek(j).text] || r.is(j, tPunct, "(")) {
					r.shortDef(j)
				}
			}
			continue
		case tIdent:
		default:
			continue
		}
		if afterDot {
			if t.text == "include" || t.text == "includet" {
				r.include(i)
			}
			continue
		}
		switch t.text {
		case "module", "baremodule":
			name := ""
			if n := r.peek(i + 1); n.kind == tIdent {
				name = n.text
				if r.topLevel() {
					r.symbol(name, "module", n.line)
				}
				if len(r.paths) < maxModules {
					r.out.modules = append(r.out.modules, joinModule(r.modulePath(), name))
				}
			}
			r.push(fModule, name)
		case "function", "macro":
			if r.topLevel() {
				r.funcName(i+1, t.text)
			}
			r.push(fFunc, "")
		case "struct":
			if r.topLevel() {
				r.typeName(i+1, "type")
			}
			r.push(fType, "")
		case "abstract", "primitive":
			if r.is(i+1, tIdent, "type") {
				if r.topLevel() {
					r.typeName(i+2, "type")
				}
				r.push(fType, "")
				i++
			}
		case "begin":
			if !r.indexContext() {
				r.push(fBlock, "")
			}
		case "if":
			if !r.topIsBracket() || !r.prevOperand(i) {
				r.push(fBlock, "")
			}
		case "try":
			r.push(fBlock, "")
		case "for":
			if !r.topIsBracket() || !r.prevOperand(i) {
				r.push(fScope, "")
			}
		case "let", "while", "quote", "do":
			r.push(fScope, "")
		case "end":
			if !r.topIsBracket() && len(r.stack) > 0 {
				r.popTo(len(r.stack) - 1)
			}
		case "using", "import":
			i = r.imports(i) - 1
		case "const":
			if r.topLevel() {
				r.consts(i + 1)
			}
		case "include", "includet":
			r.include(i)
		default:
			if r.stmtStart(i) && r.topLevel() && !keywords[t.text] {
				r.shortDef(i)
			}
		}
	}
}

// prevOperand reports whether the token before i ends an operand: in brackets, a
// `for` or `if` after one is a comprehension's clause, which has no `end`.
func (r *reader) prevOperand(i int) bool {
	p := r.peek(i - 1)
	switch p.kind {
	case tIdent:
		return !keywords[p.text] || p.text == "end" || p.text == "true" || p.text == "false"
	case tNum, tStr, tChar, tCmd, tSym:
		return true
	case tPunct:
		return p.text == ")" || p.text == "]" || p.text == "}" || p.text == "'"
	}
	return false
}

// symbol adds a definition, the first of a name only: Julia methods add to one
// function, so f(x::Int) and f(x::String) are one symbol.
func (r *reader) symbol(name, kind string, line int) {
	if name == "" || r.symbols[name] {
		return
	}
	r.symbols[name] = true
	r.set.Add(name, kind, line)
}

// dotted reads a name at i: Foo, Base.show, Base.:+, (+), Base.:(==). It returns
// the name and the index after it; "" when there is none.
func (r *reader) dotted(i int) (string, int) {
	var parts []string
	for {
		t := r.peek(i)
		switch {
		case t.kind == tIdent && !keywords[t.text]:
			parts = append(parts, t.text)
			i++
		case t.kind == tSym && len(parts) > 0: // Base.:+ lexes as a symbol only for names
			parts = append(parts, t.text)
			i++
		case t.kind == tPunct && t.text == ":" && len(parts) > 0 && r.peek(i+1).kind == tPunct && r.peek(i+1).text != "(":
			parts = append(parts, r.peek(i+1).text) // Base.:+
			i += 2
		case t.kind == tPunct && (t.text == "(" || t.text == ":" && r.is(i+1, tPunct, "(")):
			// (+) or Base.:(==): an operator in parentheses.
			j := i + 1
			if t.text == ":" {
				j++
			}
			op := r.peek(j)
			if (op.kind == tPunct || op.kind == tIdent) && r.is(j+1, tPunct, ")") && op.text != ")" && !(op.kind == tIdent && len(parts) == 0 && !r.is(j+2, tPunct, "(")) {
				parts = append(parts, op.text)
				i = j + 2
			} else {
				return strings.Join(parts, "."), i
			}
		default:
			return strings.Join(parts, "."), i
		}
		if !r.is(i, tPunct, ".") || r.peek(i).sp {
			return strings.Join(parts, "."), i
		}
		i++
	}
}

// funcName reads a `function` or `macro` definition's name.
func (r *reader) funcName(i int, kw string) {
	name, _ := r.dotted(i)
	if name == "" {
		return // function (x) ... end: anonymous; (f::Functor)(x): a call operator
	}
	kind := "function"
	if kw == "macro" {
		kind = "macro"
	}
	r.symbol(name, kind, r.peek(i).line)
}

// typeName reads a struct's or abstract type's name, after `mutable` too.
func (r *reader) typeName(i int, kind string) {
	if t := r.peek(i); t.kind == tIdent && !keywords[t.text] {
		r.symbol(t.text, kind, t.line)
	}
}

// consts reads `const A = ...` and `const A, B = ...` (after `global` too).
func (r *reader) consts(i int) {
	if r.is(i, tIdent, "global") {
		i++
	}
	for {
		t := r.peek(i)
		if t.kind != tIdent || keywords[t.text] {
			return
		}
		r.symbol(t.text, "const", t.line)
		i++
		if r.is(i, tPunct, "::") { // const X::Int = 1
			return
		}
		if !r.is(i, tPunct, ",") {
			return
		}
		i++
	}
}

// enum reads `@enum Name v1 v2` and `@enum Name::UInt8 begin ... end`.
func (r *reader) enum(i int) {
	if t := r.peek(i + 1); t.kind == tIdent && !keywords[t.text] {
		r.symbol(t.text, "enum", t.line)
	} else if r.is(i+1, tPunct, "(") && r.peek(i+2).kind == tIdent {
		r.symbol(r.peek(i+2).text, "enum", r.peek(i+2).line)
	}
}

// shortDef reads the one-line definition form at a statement's start:
// f(x) = ..., f(x)::T = ..., f(x) where T = ..., Base.show(io, x) = ...,
// Foo{T}(x) = ..., (+)(a, b) = ...
func (r *reader) shortDef(i int) {
	name, j := r.dotted(i)
	if name == "" {
		return
	}
	if r.is(j, tPunct, "{") && !r.peek(j).sp {
		j = r.skipGroup(j)
	}
	if !r.is(j, tPunct, "(") || r.peek(j).sp {
		return
	}
	j = r.skipGroup(j)
	line := r.peek(i).line
	typed := false // after :: or where, a type: Base.Vector{T}, T <: Real
	for n := 0; n < 256; n++ {
		t := r.peek(j)
		switch {
		case t.kind == tPunct && t.text == "=":
			r.symbol(name, "function", line)
			return
		case t.kind == -1, t.nl && !r.is(j-1, tPunct, "::") && !r.is(j-1, tIdent, "where"):
			return
		case t.kind == tPunct && t.text == "::":
			typed = true
		case t.kind == tIdent && t.text == "where":
			typed = true
		case !typed:
			return
		case t.kind == tPunct && (t.text == "{" || t.text == "("):
			j = r.skipGroup(j)
			continue
		case t.kind == tPunct && (t.text == "." || t.text == "<:" || t.text == ">:" || t.text == ","):
		case t.kind == tIdent && !keywords[t.text]:
		default:
			return
		}
		j++
	}
}

// skipGroup returns the index after the bracket group opening at i.
func (r *reader) skipGroup(i int) int {
	if i < len(r.match) && r.match[i] > i {
		return r.match[i] + 1
	}
	return len(r.tokens)
}

// matchBrackets pairs every bracket with its partner in one pass; a closer of the
// wrong kind is left unpaired.
func (r *reader) matchBrackets() {
	r.match = make([]int, len(r.tokens))
	var open []int
	for i, t := range r.tokens {
		r.match[i] = -1
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			open = append(open, i)
		case ")", "]", "}":
			if n := len(open); n > 0 && closes(r.tokens[open[n-1]].text, t.text) {
				r.match[open[n-1]], r.match[i] = i, open[n-1]
				open = open[:n-1]
			}
		}
	}
}

func closes(open, close string) bool {
	return open == "(" && close == ")" || open == "[" && close == "]" || open == "{" && close == "}"
}

// imports reads a using or import statement at i and returns the index after it:
// using A, B.C; using A: x, y; import A.B: c; import A as B; using .Sub, ..Parent.
func (r *reader) imports(i int) int {
	kw := r.tokens[i].text
	mp := r.modulePath()
	j := i + 1
	for {
		start := j
		dots := 0
		for r.peek(j).kind == tPunct && strings.Trim(r.peek(j).text, ".") == "" && r.peek(j).text != "" {
			dots += len(r.peek(j).text)
			j++
		}
		var parts []string
		for {
			t := r.peek(j)
			if t.kind != tIdent || keywords[t.text] {
				break
			}
			parts = append(parts, t.text)
			j++
			if !r.is(j, tPunct, ".") {
				break
			}
			j++
		}
		if len(parts) == 0 && dots == 0 {
			return max(j, i+1)
		}
		module := strings.Repeat(".", dots) + strings.Join(parts, ".")
		spec := kw + " " + module
		if !r.seen[spec] {
			r.seen[spec] = true
			r.out.imports = append(r.out.imports, lang.RawImport{Spec: spec, Module: module, Name: kindUsing + "\n" + mp, Line: r.peek(start).line})
		}
		if r.is(j, tIdent, "as") {
			j += 2
		}
		switch {
		case r.is(j, tPunct, ","):
			j++
			continue
		case r.is(j, tPunct, ":"):
			// using A: x, y as z, @m, (+)
			j++
			for n := 0; n < 4096; n++ {
				t := r.peek(j)
				if t.kind == -1 || t.kind == tIdent && keywords[t.text] && t.text != "as" {
					return j
				}
				if r.is(j, tPunct, "(") {
					j = r.skipGroup(j)
				} else {
					j++
				}
				if r.is(j, tIdent, "as") {
					j += 2
				}
				if !r.is(j, tPunct, ",") {
					return j
				}
				j++
			}
			return j
		}
		return j
	}
}

// include reads include("x.jl"), include(joinpath(@__DIR__, "x.jl")), Base.include
// (Mod, "x.jl") and Revise's includet at i.
func (r *reader) include(i int) {
	if !r.is(i+1, tPunct, "(") || r.peek(i+1).sp || r.is(i-1, tIdent, "function") {
		return
	}
	end := r.skipGroup(i + 1)
	if !r.is(end-1, tPunct, ")") || end-1 <= i+1 {
		return
	}
	args := splitTokens(r.tokens[i+2 : end-1])
	p, ok := r.evalPath(args[len(args)-1])
	first := i
	if r.is(i-1, tPunct, ".") && r.peek(i-2).kind == tIdent {
		first = i - 2
	}
	spec := string(r.src[r.tokens[first].start:r.tokens[end-1].end])
	if r.seen[spec] {
		return
	}
	r.seen[spec] = true
	if !ok {
		p = "" // an include of a computed path: shown, not resolved
	} else {
		r.out.includes = append(r.out.includes, include{path: p, module: r.modulePath()})
	}
	r.out.imports = append(r.out.imports, lang.RawImport{Spec: spec, Module: p, Name: kindInclude + "\n" + r.modulePath(), Line: r.tokens[i].line})
}

// evalPath evaluates an include argument to a path relative to the including file's
// directory, which is where Julia looks: "x.jl", joinpath(@__DIR__, "a", "b.jl"),
// joinpath(dirname(@__FILE__), ...), @__DIR__ * "/x.jl", "$(@__DIR__)/x.jl",
// normpath/abspath of those. False for anything computed at run time.
func (r *reader) evalPath(ts []tok) (string, bool) {
	s, ok := r.eval(ts, 0)
	if !ok || s == "" || strings.HasPrefix(s, "/") {
		return "", false
	}
	return strings.TrimPrefix(s, "./"), true
}

func (r *reader) eval(ts []tok, depth int) (string, bool) {
	if len(ts) == 0 || depth > 16 {
		return "", false
	}
	// a * b * c: string concatenation
	var parts [][]tok
	d, from := 0, 0
	for j, t := range ts {
		if t.kind == tPunct {
			switch t.text {
			case "(", "[", "{":
				d++
			case ")", "]", "}":
				d--
			case "*":
				if d == 0 {
					parts = append(parts, ts[from:j])
					from = j + 1
				}
			}
		}
	}
	if len(parts) > 0 {
		parts = append(parts, ts[from:])
		out := ""
		for _, p := range parts {
			s, ok := r.eval(p, depth+1)
			if !ok {
				return "", false
			}
			out += s
		}
		return out, true
	}
	t := ts[0]
	switch {
	case t.kind == tPunct && t.text == "$" && len(ts) >= 3 && ts[1].text == "(" && ts[len(ts)-1].text == ")":
		return r.eval(ts[2:len(ts)-1], depth+1) // include($(joinpath(...))) in an @eval
	case len(ts) == 1 && t.kind == tStr && !t.prefixed:
		return interpolated(t.text)
	case len(ts) == 1 && t.kind == tStr && t.text != "" && strings.HasPrefix(string(r.src[t.start:t.end]), "raw"):
		return t.text, true
	case t.kind == tMacro && t.text == "__DIR__" && (len(ts) == 1 || len(ts) == 3 && ts[1].text == "(" && ts[2].text == ")"):
		return ".", true
	case t.kind == tIdent && len(ts) >= 3 && ts[1].text == "(" && ts[len(ts)-1].text == ")":
		args := splitTokens(ts[2 : len(ts)-1])
		switch t.text {
		case "joinpath", "string":
			out := ""
			for k, a := range args {
				s, ok := r.eval(a, depth+1)
				if !ok {
					return "", false
				}
				if t.text == "joinpath" && k > 0 && out != "" && !strings.HasSuffix(out, "/") {
					out += "/"
				}
				out += s
			}
			return out, true
		case "normpath", "abspath", "realpath":
			if len(args) == 1 {
				return r.eval(args[0], depth+1)
			}
		case "dirname":
			if len(args) == 1 {
				if a := args[0]; len(a) == 1 && a[0].kind == tMacro && a[0].text == "__FILE__" ||
					len(a) == 3 && a[0].kind == tMacro && a[0].text == "__FILE__" {
					return ".", true
				}
				s, ok := r.eval(args[0], depth+1)
				if !ok {
					return "", false
				}
				return s + "/..", true
			}
		}
	}
	return "", false
}

// splitTokens splits tokens on top-level commas.
func splitTokens(ts []tok) [][]tok {
	var out [][]tok
	var cur []tok
	depth := 0
	for _, t := range ts {
		if t.kind == tPunct {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			case ",":
				if depth == 0 {
					out = append(out, cur)
					cur = nil
					continue
				}
			}
		}
		cur = append(cur, t)
	}
	return append(out, cur)
}

// interpolated evaluates a string's text whose only interpolation is the directory
// of the file: "$(@__DIR__)/x.jl".
func interpolated(s string) (string, bool) {
	for _, dir := range []string{"$(@__DIR__)", "$(dirname(@__FILE__))"} {
		if rest, ok := strings.CutPrefix(s, dir); ok {
			s = "." + rest
			break
		}
	}
	if strings.ContainsAny(s, "$\\") {
		return "", false
	}
	return s, true
}
