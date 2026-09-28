package ocaml

import (
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// OCaml has no import statement: a file uses another module by naming it. The
// reader collects the module paths a file names - `Foo.bar`, `Foo.Bar.t`,
// `Foo.(expr)`, `open Foo`, `include Foo`, `let open Foo in`, `module M = Foo`,
// functor arguments `F(Foo)`, `module type of Foo`, `(module Foo)` - and the
// top-level definitions. A capitalized name not followed by a dot is a constructor
// (`Some x`, `Ok`) unless it stands where only a module can (after open, include, =
// in a module binding); modules the file itself declares (`module M = struct`,
// functor parameters, `let module`, first-class `(module M : S)`) are not
// references.

// Import kinds, RawImport.Name's first line.
const (
	kindModule  = "mod"     // a module path in code
	kindOpen    = "open"    // open Foo, let open Foo in
	kindInclude = "include" // include Foo
	kindRequire = "require" // #require "lib" in a toplevel script
)

var keywords = map[string]bool{
	"and": true, "as": true, "assert": true, "begin": true, "class": true, "constraint": true, "do": true,
	"done": true, "downto": true, "else": true, "end": true, "exception": true, "external": true,
	"false": true, "for": true, "fun": true, "function": true, "functor": true, "if": true, "in": true,
	"include": true, "inherit": true, "initializer": true, "lazy": true, "let": true, "match": true,
	"method": true, "module": true, "mutable": true, "new": true, "nonrec": true, "object": true,
	"of": true, "open": true, "or": true, "private": true, "rec": true, "sig": true, "struct": true,
	"then": true, "to": true, "true": true, "try": true, "type": true, "val": true, "virtual": true,
	"when": true, "while": true, "with": true, "effect": true,
}

// operandEnd reports whether a token can end an expression: a `let` after it
// cannot continue that expression, so it starts a new definition.
func operandEnd(t token) bool {
	switch t.k {
	case tUpper, tString, tLiteral:
		return true
	case tLower:
		switch t.s {
		case "end", "done", "true", "false", "struct", "sig", "object":
			return true
		}
		return !keywords[t.s]
	case tOp:
		switch t.s {
		case ")", "]", "}", "|]", ";;", "..":
			return true
		}
	}
	return false
}

type frame struct {
	kind   byte // 's' struct/sig, 'x' [%%ext ...], 'o' object, 'b' begin, 'd' do, '(' '[' '{' brackets
	owner  string
	record bool // definitions here are the file's symbols
	lets   int  // lets waiting for their `in`
}

type sourceReader struct {
	tokens     []token
	mode       byte // 'm' .ml/.mli, 'l' ocamllex, 'y' menhir/ocamlyacc
	declared   map[string]bool
	moduleAt   []bool
	imports    []lang.RawImport
	seenImport map[string]bool
	opens      []string
	symbols    []lang.Symbol
	seenSymbol map[string]bool
	stack      []*frame
	// the definition being read: its keyword (for `and`), the module or class it
	// names (the owner of a struct/object that follows)
	item, pendingOwner, pendingClass string
}

// readSource extracts a source file; extension is .ml, .mli, .mll or .mly.
func readSource(source []byte, extension string) *lang.Extraction {
	mode := byte('m')
	switch extension {
	case ".mll":
		mode = 'l'
	case ".mly":
		mode = 'y'
	}
	r := &sourceReader{
		tokens: lex(source, mode == 'y'), mode: mode, declared: map[string]bool{},
		seenImport: map[string]bool{}, seenSymbol: map[string]bool{},
	}
	r.moduleAt = make([]bool, len(r.tokens)+1)
	r.declarations()
	r.read()
	if mode == 'y' {
		r.grammarRules(source)
	}
	opens := strings.Join(r.opens, ",")
	for i := range r.imports {
		if r.imports[i].Name != kindRequire {
			r.imports[i].Name += "\n" + opens
		}
	}
	var set lang.SymbolSet
	for _, s := range r.symbols {
		set.Add(s.Name, s.Kind, s.Line)
	}
	return &lang.Extraction{Imports: r.imports, Symbols: set.List()}
}

func (r *sourceReader) at(i int) token {
	if i < 0 || i >= len(r.tokens) {
		return token{}
	}
	return r.tokens[i]
}

func (r *sourceReader) is(i int, k kind, s string) bool {
	t := r.at(i)
	return t.k == k && t.s == s
}

// declarations collects the module names the file binds itself.
func (r *sourceReader) declarations() {
	for i, t := range r.tokens {
		switch {
		case t.k == tLower && t.s == "module":
			if r.is(i-1, tOp, "(") && !r.is(i+2, tOp, ":") {
				continue // (module Foo): a module packed, not bound
			}
			j := i + 1
			if r.is(j, tLower, "rec") {
				j++
			}
			if r.is(j, tLower, "type") {
				j++
				if r.is(j, tLower, "of") {
					continue
				}
			}
			if r.at(j).k == tUpper {
				r.declared[r.at(j).s] = true
			}
		case t.k == tOp && t.s == "(" && r.at(i+1).k == tUpper && r.is(i+2, tOp, ":"):
			r.declared[r.at(i+1).s] = true // a functor's parameter
		case t.k == tLower && t.s == "and" && r.at(i+1).k == tUpper &&
			(r.is(i+2, tOp, ":") || r.is(i+2, tOp, "=") || r.is(i+2, tOp, "(")):
			r.declared[r.at(i+1).s] = true // module rec A ... and B
		}
	}
}

func (r *sourceReader) top() *frame { return r.stack[len(r.stack)-1] }

func (r *sourceReader) push(kind byte, owner string, record bool) {
	r.stack = append(r.stack, &frame{kind: kind, owner: owner, record: record})
}

// popTo closes the innermost frame of one of kinds, and every frame opened after
// it; a closer without an opener in reach (not beyond a structure) is ignored.
func (r *sourceReader) popTo(kinds string, stops string) {
	for k := len(r.stack) - 1; k >= 1; k-- {
		f := r.stack[k]
		if strings.IndexByte(kinds, f.kind) >= 0 {
			r.stack = r.stack[:k]
			return
		}
		if strings.IndexByte(stops, f.kind) >= 0 {
			return
		}
	}
}

// structure reports whether definitions are read in the innermost frame.
func (r *sourceReader) structure() bool {
	k := r.top().kind
	return k == 's' || k == 'x' || k == 'o'
}

func (r *sourceReader) startItem(kind string) {
	r.top().lets = 0
	r.item = kind
	r.pendingOwner, r.pendingClass = "", ""
}

func (r *sourceReader) symbol(name, kind string, line int) {
	f := r.top()
	if !f.record || name == "" || name == "_" || r.mode == 'y' {
		return
	}
	if f.owner != "" {
		name = f.owner + "." + name
	}
	if r.seenSymbol[name] {
		return // shadowed, or declared by a signature and defined by its structure
	}
	r.seenSymbol[name] = true
	r.symbols = append(r.symbols, lang.Symbol{Name: name, Kind: kind, Line: line})
}

func (r *sourceReader) read() {
	r.stack = []*frame{{kind: 's', record: true}}
	for i := 0; i < len(r.tokens); i++ {
		t := r.tokens[i]
		switch t.k {
		case tOp:
			switch t.s {
			case "(":
				r.push('(', r.top().owner, false)
			case "[%%":
				// A structure item extension ([%%template ...]) holds definitions.
				f := r.top()
				r.push('x', f.owner, f.record && (f.kind == 's' || f.kind == 'x'))
			case "[", "[|", "[@", "[@@", "[@@@", "[%":
				r.push('[', r.top().owner, false)
			case "{":
				r.push('{', r.top().owner, false)
			case ")":
				r.popTo("(", "sobdx")
			case "]", "|]":
				r.popTo("[x", "sobd")
			case "}":
				r.popTo("{", "sobdx")
			case ";;":
				r.top().lets = 0
			}
		case tUpper:
			r.path(i)
		case tDirectory:
			if t.s == "require" && r.at(i+1).k == tString {
				for _, library := range strings.FieldsFunc(r.at(i+1).s, func(c rune) bool { return c == ',' || c == ' ' }) {
					r.addImport(lang.RawImport{Spec: "#require \"" + library + "\"", Module: library, Name: kindRequire, Line: t.line})
				}
			}
		case tLower:
			r.keyword(i, t)
		}
	}
}

func (r *sourceReader) keyword(i int, t token) {
	previous := r.at(i - 1)
	afterLet := previous.k == tLower && (previous.s == "let" || previous.s == "with" || previous.s == "and")
	switch t.s {
	case "struct", "sig":
		f := r.top()
		owner, record := f.owner, false
		if r.pendingOwner != "" {
			owner, record = join(owner, r.pendingOwner), f.record
		} else if previous.k == tLower && previous.s == "include" {
			record = f.record
		}
		r.push('s', owner, record)
	case "object":
		f := r.top()
		r.push('o', join(f.owner, r.pendingClass), f.record && r.pendingClass != "")
	case "begin":
		r.push('b', r.top().owner, false)
	case "do":
		r.push('d', r.top().owner, false)
	case "end":
		r.popTo("sob", "x")
	case "done":
		r.popTo("d", "sox")
	case "in":
		if f := r.top(); f.lets > 0 {
			f.lets--
		}
	case "let":
		f := r.top()
		if t.extension != "" && !(t.extension[0] >= 'a' && t.extension[0] <= 'z' || t.extension[0] == '_') {
			f.lets++ // let* x = ... in: a binding operator is always local
			return
		}
		if (f.kind == 's' || f.kind == 'x') && f.lets == 0 && (i == 0 || operandEnd(previous) || previous.k == tOp && previous.s == "[%%") {
			r.startItem("let")
			r.binding(i + 1)
			return
		}
		f.lets++
	case "and":
		if !r.structure() || r.top().lets > 0 || t.extension != "" && !(t.extension[0] >= 'a' && t.extension[0] <= 'z') {
			return
		}
		switch r.item {
		case "let":
			r.binding(i + 1)
		case "type":
			r.typeName(i + 1)
		case "module":
			if n := r.at(i + 1); n.k == tUpper {
				r.pendingOwner = n.s
				r.symbol(n.s, "module", n.line)
			}
		case "class":
			r.className(i + 1)
		case "rule":
			if n := r.at(i + 1); n.k == tLower && !keywords[n.s] {
				r.symbol(n.s, "rule", n.line)
			}
		}
	case "type":
		if !r.structure() || afterLet || previous.k == tLower && previous.s == "module" {
			return
		}
		r.startItem("type")
		r.typeName(i + 1)
	case "module":
		r.moduleBinding(i, afterLet)
	case "open", "include":
		k := i + 1
		if r.is(k, tOp, "!") {
			k++
		}
		r.moduleAt[k] = true
		if r.structure() && !afterLet {
			r.startItem(t.s)
		}
	case "exception":
		if !r.structure() || afterLet || previous.k == tOp && previous.s == "|" {
			return
		}
		r.startItem("exception")
		if n := r.at(i + 1); n.k == tUpper {
			r.symbol(n.s, "exception", n.line)
		}
	case "external":
		if !r.structure() {
			return
		}
		r.startItem("external")
		if name, line := r.valueName(i + 1); name != "" {
			r.symbol(name, "external", line)
		}
	case "class":
		if !r.structure() {
			return
		}
		r.startItem("class")
		r.className(i + 1)
	case "val":
		if k := r.top().kind; k != 's' && k != 'x' {
			return // an instance variable
		}
		r.startItem("val")
		j := i + 1
		for r.is(j, tLower, "mutable") || r.is(j, tLower, "virtual") {
			j++
		}
		if name, line := r.valueName(j); name != "" {
			kind := "value"
			if r.arrow(j + 1) {
				kind = "function"
			}
			r.symbol(name, kind, line)
		}
	case "method":
		if r.top().kind != 'o' {
			return
		}
		r.startItem("method")
		j := i + 1
		for r.is(j, tOp, "!") || r.is(j, tLower, "private") || r.is(j, tLower, "virtual") {
			j++
		}
		if n := r.at(j); n.k == tLower && !keywords[n.s] {
			r.symbol(n.s, "method", n.line)
		}
	case "rule":
		if r.mode == 'l' && len(r.stack) == 1 {
			r.startItem("rule")
			if n := r.at(i + 1); n.k == tLower && !keywords[n.s] {
				r.symbol(n.s, "rule", n.line)
			}
		}
	}
}

func join(owner, name string) string {
	if owner == "" {
		return name
	}
	if name == "" {
		return owner
	}
	return owner + "." + name
}

// binding reads the name a let binds: a value, a function (parameters follow it,
// or fun/function), an operator in parentheses. Patterns bind no symbol.
func (r *sourceReader) binding(j int) {
	j = r.attributes(j)
	for r.is(j, tLower, "rec") || r.is(j, tLower, "nonrec") {
		j = r.attributes(j + 1)
	}
	n := r.at(j)
	if n.k == tLower && !keywords[n.s] && n.s != "_" {
		kind := "function"
		next := r.at(j + 1)
		switch {
		case next.k == tOp && next.s == "=":
			if a := r.at(j + 2); !(a.k == tLower && (a.s == "fun" || a.s == "function")) {
				kind = "value"
			}
		case next.k == tOp && (next.s == ":" || next.s == ","):
			kind = "value"
		}
		if next.k == tOp && next.s == "," {
			return // let a, b = ...: a pattern
		}
		r.symbol(n.s, kind, n.line)
		return
	}
	if n.k == tOp && n.s == "(" && r.is(j+2, tOp, ")") && r.at(j+1).k == tOp {
		r.symbol(r.at(j+1).s, "function", r.at(j+1).line)
	}
}

// attributes skips attributes at j: let[@inline] f.
func (r *sourceReader) attributes(j int) int {
	for r.is(j, tOp, "[@") {
		for d := 0; j < len(r.tokens); j++ {
			if t := r.tokens[j]; t.k == tOp && strings.HasPrefix(t.s, "[") {
				d++
			} else if t.k == tOp && (t.s == "]" || t.s == "|]") {
				if d--; d == 0 {
					j++
					break
				}
			}
		}
	}
	return j
}

// typeName reads a type definition's name past its parameters ('a, ('a, 'b), _).
func (r *sourceReader) typeName(j int) {
	for r.is(j, tLower, "nonrec") {
		j++
	}
	switch {
	case r.is(j, tOp, "'") || r.is(j, tOp, "+'") || r.is(j, tOp, "-'"):
		j += 2
	case r.is(j, tOp, "+") || r.is(j, tOp, "-"):
		j += 3
	case r.is(j, tOp, "("):
		for d := 0; j < len(r.tokens); j++ {
			if r.is(j, tOp, "(") {
				d++
			} else if r.is(j, tOp, ")") {
				if d--; d == 0 {
					j++
					break
				}
			}
		}
	case r.is(j, tLower, "_"):
		j++
	}
	n := r.at(j)
	if n.k != tLower || keywords[n.s] || r.is(j+1, tOp, "+=") {
		return
	}
	r.symbol(n.s, "type", n.line)
}

// moduleBinding reads `module M ... = <module expression>`, `module type S`,
// `module type of M`, `let module M = ...` and `(module M)`.
func (r *sourceReader) moduleBinding(i int, afterLet bool) {
	previous := r.at(i - 1)
	if previous.k == tOp && previous.s == "(" {
		if r.at(i+1).k == tUpper {
			r.moduleAt[i+1] = true // (module M): packed; (module M : S) was declared
		}
		return
	}
	j := i + 1
	item := r.structure() && !afterLet && !(previous.k == tLower && (previous.s == "with" || previous.s == "and"))
	if r.is(j, tLower, "rec") {
		j++
	}
	kind := "module"
	if r.is(j, tLower, "type") {
		j++
		if r.is(j, tLower, "of") {
			r.moduleAt[j+1] = true
			return
		}
		kind = "module type"
	}
	n := r.at(j)
	if item {
		r.startItem("module")
	}
	if n.k != tUpper {
		return
	}
	if item {
		r.pendingOwner = n.s
		r.symbol(n.s, kind, n.line)
	}
	// Past functor parameters, `=` starts the module expression.
	j++
	for r.is(j, tOp, "(") {
		for d := 0; j < len(r.tokens); j++ {
			if r.is(j, tOp, "(") {
				d++
			} else if r.is(j, tOp, ")") {
				if d--; d == 0 {
					j++
					break
				}
			}
		}
	}
	if r.is(j, tOp, "=") {
		r.moduleAt[j+1] = true
	}
}

func (r *sourceReader) className(j int) {
	if r.is(j, tLower, "type") {
		j++
	}
	if r.is(j, tLower, "virtual") {
		j++
	}
	if r.is(j, tOp, "[") {
		for j < len(r.tokens) && !r.is(j, tOp, "]") {
			j++
		}
		j++
	}
	if n := r.at(j); n.k == tLower && !keywords[n.s] {
		r.pendingClass = n.s
		r.symbol(n.s, "class", n.line)
	}
}

// valueName reads the name of a val or external: an identifier or an operator in
// parentheses.
func (r *sourceReader) valueName(j int) (string, int) {
	n := r.at(j)
	if n.k == tLower && !keywords[n.s] {
		return n.s, n.line
	}
	if n.k == tOp && n.s == "(" && r.at(j+1).k == tOp && r.is(j+2, tOp, ")") {
		return r.at(j + 1).s, r.at(j + 1).line
	}
	return "", 0
}

// arrow reports whether a val's type is a function type: an arrow outside
// brackets before the next definition.
func (r *sourceReader) arrow(j int) bool {
	d := 0
	for k := j; k < len(r.tokens) && k < j+400; k++ {
		t := r.tokens[k]
		switch {
		case t.k == tOp && (t.s == "(" || t.s == "[" || t.s == "{"):
			d++
		case t.k == tOp && (t.s == ")" || t.s == "]" || t.s == "}"):
			d--
		case t.k == tOp && t.s == "->" && d == 0:
			return true
		case t.k == tLower && d == 0:
			switch t.s {
			case "val", "type", "module", "exception", "external", "include", "open", "class", "end", "method", "let":
				return false
			}
		}
		if d < 0 {
			return false
		}
	}
	return false
}

// path reads the module path starting at i, if one does.
func (r *sourceReader) path(i int) {
	previous := r.at(i - 1)
	if previous.k == tOp && previous.s == "." && r.at(i-2).k == tUpper {
		return // inside a path read from its start
	}
	ctx := r.moduleAt[i]
	var segments []string
	j := i
	for j < len(r.tokens) && r.tokens[j].k == tUpper {
		if r.is(j+1, tOp, ".") {
			segments = append(segments, r.tokens[j].s)
			j += 2
			continue
		}
		if ctx {
			segments = append(segments, r.tokens[j].s)
			j++
		}
		break
	}
	if ctx {
		// Functor arguments: F(A), F(A)(B).
		if r.is(j, tOp, "(") && r.at(j+1).k == tUpper {
			r.moduleAt[j+1] = true
		} else if r.is(j, tOp, ")") && r.is(j+1, tOp, "(") && r.at(j+2).k == tUpper {
			r.moduleAt[j+2] = true
		}
	}
	if len(segments) == 0 {
		return
	}
	// Foo__Bar is how dune names a wrapped library's module Foo.Bar.
	if a, b, ok := strings.Cut(segments[0], "__"); ok && a != "" && b != "" {
		segments = append([]string{a, strings.ToUpper(b[:1]) + b[1:]}, segments[1:]...)
	}
	if r.declared[segments[0]] {
		return
	}
	kind := kindModule
	k := i - 1
	if r.is(k, tOp, "!") {
		k--
	}
	if r.is(k, tLower, "open") {
		kind = kindOpen
	} else if r.is(k, tLower, "include") {
		kind = kindInclude
	}
	module := strings.Join(segments[:min(2, len(segments))], ".")
	if kind == kindOpen {
		r.opens = append(r.opens, module)
	}
	spec := module
	if kind != kindModule {
		spec = kind + " " + module
	}
	if r.seenImport[module] {
		return
	}
	r.seenImport[module] = true
	r.imports = append(r.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: r.tokens[i].line})
}

func (r *sourceReader) addImport(rawImport lang.RawImport) {
	if r.seenImport[rawImport.Spec] {
		return
	}
	r.seenImport[rawImport.Spec] = true
	r.imports = append(r.imports, rawImport)
}

// ruleLine is a grammar rule's head: `expr:`, `%public list(X):`, `%inline op:`,
// Menhir's `let expr :=`.
var ruleLine = regexp.MustCompile(`^(?:%public\s+)?(?:%inline\s+)?(?:let\s+)?([a-z_][A-Za-z0-9_']*)\s*(?:\([^)]*\))?\s*:`)

// grammarRules records a Menhir or ocamlyacc grammar's rules, which are what the
// generated parser exports.
func (r *sourceReader) grammarRules(source []byte) {
	section := 0
	for n, line := range strings.Split(string(source), "\n") {
		if strings.TrimSpace(line) == "%%" {
			section++
			continue
		}
		if section != 1 {
			continue
		}
		if m := ruleLine.FindStringSubmatch(line); m != nil && !strings.HasPrefix(line[len(m[0]):], ":") {
			if !r.seenSymbol[m[1]] {
				r.seenSymbol[m[1]] = true
				r.symbols = append(r.symbols, lang.Symbol{Name: m[1], Kind: "rule", Line: n + 1})
			}
		}
	}
}
