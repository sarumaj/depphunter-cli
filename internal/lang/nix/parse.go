package nix

import "strings"

// A tolerant recursive-descent parser for Nix expressions. It builds a small tree
// with what extraction needs - attribute sets and their bindings, let, with,
// lambdas and their formals, applications, lists, selections, strings, paths - and
// recovers from anything else by skipping to the next binding or closing bracket.
// Brackets are matched once up front, nesting is capped, and every loop consumes a
// token, so any input parses in linear time.

type nodeKind uint8

const (
	nOther nodeKind = iota
	nIdent
	nSelect // kids[0] base, path; kids[1] the `or` default
	nString // text when literal; interpolate; kids = interpolated expressions
	nPath   // text (literal part); interpolate; kids = interpolated expressions
	nSPath  // <text>
	nURI
	nAttrs  // binds; rec
	nLet    // binds; kids[0] body
	nWith   // kids[0] environment, kids[1] body
	nAssert // kids[0] condition, kids[1] body
	nIf     // kids[0..2]
	nLambda // arg, formals (defaults in binds), set; kids[0] body
	nApply  // kids[0] function, kids[1:] arguments
	nList   // kids
	nBinary // text = operator; kids = operands (a chain of one operator is flat)
	nUnary  // kids[0]
	nParen  // kids[0]
)

type node struct {
	kind        nodeKind
	line        int
	text        string
	interpolate bool
	rec         bool
	set         bool     // a lambda with a { formals } pattern
	path        []string // a selection's attribute path; "" for a dynamic part
	kids        []*node
	binds       []*bind
	formals     []string
}

// bind is `a.b.c = value;` or one name of `inherit (from) a b;` (path = [name],
// value = from, which may be nil).
type bind struct {
	path    []string
	line    int
	value   *node
	inherit bool
}

const maxDepth = 400

type parser struct {
	tokens []token
	match  []int // index of the matching closer for {, (, [, ${ and string/path openers
	i      int
	depth  int
	// recovered counts the tokens skipped to recover from what the parser did
	// not understand (tests and measurements read it).
	recovered int
}

// parse reads a Nix file's expression.
//
// Implements: REQ-NIX-011
func parse(src []byte) *node {
	tokens := lex(src)
	p := &parser{tokens: tokens, match: matchBrackets(tokens)}
	root := p.expr()
	return root
}

// matchBrackets pairs openers with closers. A closer only closes an opener among
// the innermost few, so one stray bracket does not unpair the rest of the file.
func matchBrackets(tokens []token) []int {
	m := make([]int, len(tokens))
	for i := range m {
		m[i] = -1
	}
	var stack []int
	closes := func(open token, c token) bool {
		switch open.kind {
		case tStrOpen:
			return c.kind == tStrClose
		case tPathOpen:
			return c.kind == tPathClose
		case tInterp:
			return c.kind == tInterpEnd
		}
		switch open.text {
		case "{":
			return c.kind == tPunct && c.text == "}"
		case "(":
			return c.kind == tPunct && c.text == ")"
		case "[":
			return c.kind == tPunct && c.text == "]"
		}
		return false
	}
	for i, t := range tokens {
		switch {
		case t.kind == tStrOpen || t.kind == tPathOpen || t.kind == tInterp,
			t.kind == tPunct && (t.text == "{" || t.text == "(" || t.text == "["):
			stack = append(stack, i)
		case t.kind == tStrClose || t.kind == tPathClose || t.kind == tInterpEnd,
			t.kind == tPunct && (t.text == "}" || t.text == ")" || t.text == "]"):
			for k := len(stack) - 1; k >= 0 && k >= len(stack)-8; k-- {
				if closes(tokens[stack[k]], t) {
					m[stack[k]] = i
					m[i] = stack[k]
					stack = stack[:k]
					break
				}
			}
		}
	}
	return m
}

func (p *parser) tok() token { return p.tokens[p.i] }

func (p *parser) peekTok(k int) token {
	if p.i+k < len(p.tokens) {
		return p.tokens[p.i+k]
	}
	return p.tokens[len(p.tokens)-1]
}

func (p *parser) is(text string) bool {
	t := p.tokens[p.i]
	return t.kind == tPunct && t.text == text
}

func (p *parser) kw(word string) bool {
	t := p.tokens[p.i]
	return t.kind == tIdent && t.text == word
}

func (p *parser) next() token {
	t := p.tokens[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) eof() bool { return p.tokens[p.i].kind == tEOF }

// skipGroup steps over the bracketed group opening at p.i, or one token.
func (p *parser) skipGroup() {
	if m := p.match[p.i]; m > p.i {
		p.i = m + 1
		return
	}
	p.next()
}

var keywords = map[string]bool{"if": true, "then": true, "else": true, "assert": true, "with": true,
	"let": true, "in": true, "rec": true, "inherit": true, "or": true}

// expr parses an expression: a lambda, let, with, assert, if or an operator
// expression.
func (p *parser) expr() *node {
	p.depth++
	defer func() { p.depth-- }()
	t := p.tok()
	if p.depth > maxDepth {
		p.skipGroup()
		return &node{kind: nOther, line: t.line}
	}
	switch {
	case t.kind == tIdent && !keywords[t.text] && p.peekTok(1).kind == tPunct && p.peekTok(1).text == ":":
		p.i += 2
		return &node{kind: nLambda, line: t.line, text: t.text, kids: []*node{p.expr()}}
	case t.kind == tIdent && !keywords[t.text] && p.peekTok(1).kind == tPunct && p.peekTok(1).text == "@":
		p.i += 2
		if p.is("{") {
			n := p.formals()
			n.text = t.text
			return n
		}
		return &node{kind: nOther, line: t.line}
	case t.kind == tPunct && t.text == "{" && p.isFormals():
		return p.formals()
	case t.kind == tIdent && t.text == "let" && !(p.peekTok(1).kind == tPunct && p.peekTok(1).text == "{"):
		p.next()
		n := &node{kind: nLet, line: t.line}
		n.binds = p.binds(func() bool { return p.kw("in") }, false)
		if p.kw("in") {
			p.next()
		}
		n.kids = []*node{p.expr()}
		return n
	case t.kind == tIdent && (t.text == "with" || t.text == "assert"):
		p.next()
		k := nWith
		if t.text == "assert" {
			k = nAssert
		}
		env := p.expr()
		if p.is(";") {
			p.next()
		}
		return &node{kind: k, line: t.line, kids: []*node{env, p.expr()}}
	case t.kind == tIdent && t.text == "if":
		p.next()
		n := &node{kind: nIf, line: t.line}
		c := p.expr()
		var a, b *node
		if p.kw("then") {
			p.next()
			a = p.expr()
		}
		if p.kw("else") {
			p.next()
			b = p.expr()
		}
		n.kids = nonNil(c, a, b)
		return n
	}
	return p.binary(0)
}

func nonNil(ns ...*node) []*node {
	out := ns[:0]
	for _, n := range ns {
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}

// isFormals tells `{ a, b ? 1, ... }:` (or `{ ... } @ args:`) from an attribute
// set at a `{`.
func (p *parser) isFormals() bool {
	a, b := p.peekTok(1), p.peekTok(2)
	switch {
	case a.kind == tPunct && a.text == "}":
		c := p.peekTok(2)
		return c.kind == tPunct && (c.text == ":" || c.text == "@")
	case a.kind == tPunct && a.text == "...":
		return true
	case a.kind == tIdent && !keywords[a.text]:
		if b.kind == tPunct && (b.text == "," || b.text == "?") {
			return true
		}
		if b.kind == tPunct && b.text == "}" {
			c := p.peekTok(3)
			return c.kind == tPunct && (c.text == ":" || c.text == "@")
		}
	}
	return false
}

// formals parses `{ a, b ? default, ... } [@ name] : body` at a `{`.
func (p *parser) formals() *node {
	open := p.i
	n := &node{kind: nLambda, line: p.tok().line, set: true}
	p.next()
	for !p.eof() && !p.is("}") {
		start := p.i
		t := p.tok()
		switch {
		case t.kind == tIdent:
			p.next()
			n.formals = append(n.formals, t.text)
			if p.is("?") {
				p.next()
				n.binds = append(n.binds, &bind{path: []string{t.text}, line: t.line, value: p.expr()})
			}
		case t.kind == tPunct && (t.text == "," || t.text == "..."):
			p.next()
		default:
			p.skipGroup()
		}
		if p.i == start {
			p.next()
		}
		if m := p.match[open]; m > 0 && p.i > m {
			p.i = m
		}
	}
	if p.is("}") {
		p.next()
	}
	if p.is("@") {
		p.next()
		if p.tok().kind == tIdent {
			n.text = p.next().text
		}
	}
	if p.is(":") {
		p.next()
	}
	n.kids = []*node{p.expr()}
	return n
}

// Binary operators by precedence, loosest first.
var precedence = map[string]int{
	"|>": 1, "<|": 1, "->": 2, "||": 3, "&&": 4, "==": 5, "!=": 5,
	"<": 6, ">": 6, "<=": 6, ">=": 6, "//": 7, "+": 9, "-": 9, "*": 10, "/": 10, "++": 11, "?": 12,
}

func (p *parser) binary(min int) *node {
	p.depth++
	defer func() { p.depth-- }()
	var left *node
	t := p.tok()
	switch {
	case p.depth > maxDepth:
		p.skipGroup()
		return &node{kind: nOther, line: t.line}
	case t.kind == tPunct && t.text == "!":
		p.next()
		left = &node{kind: nUnary, line: t.line, kids: []*node{p.binary(8)}}
	case t.kind == tPunct && t.text == "-":
		p.next()
		left = &node{kind: nUnary, line: t.line, kids: []*node{p.binary(13)}}
	default:
		left = p.apply()
	}
	for {
		t := p.tok()
		prec, ok := precedence[t.text]
		if t.kind != tPunct || !ok || prec <= min && min > 0 || prec < min {
			return left
		}
		p.next()
		if t.text == "?" { // a ? b.c: an attribute path, not an expression
			path, _ := p.attrPath()
			left = &node{kind: nBinary, line: t.line, text: "?", kids: []*node{left, {kind: nOther, path: path}}}
			continue
		}
		right := p.binary(prec)
		if left.kind == nBinary && left.text == t.text {
			left.kids = append(left.kids, right)
		} else {
			left = &node{kind: nBinary, line: t.line, text: t.text, kids: []*node{left, right}}
		}
	}
}

// startsOperand reports whether the token can begin an argument of an application.
func startsOperand(t token) bool {
	switch t.kind {
	case tIdent:
		return !keywords[t.text] || t.text == "rec"
	case tNum, tStrOpen, tPath, tPathOpen, tSPath, tURI:
		return true
	case tPunct:
		return t.text == "(" || t.text == "[" || t.text == "{"
	}
	return false
}

func (p *parser) apply() *node {
	fn := p.selectExpr()
	if !startsOperand(p.tok()) {
		return fn
	}
	n := &node{kind: nApply, line: fn.line, kids: []*node{fn}}
	for startsOperand(p.tok()) {
		start := p.i
		n.kids = append(n.kids, p.selectExpr())
		if p.i == start {
			p.next()
		}
	}
	return n
}

// selectExpr parses a primary with its `.attr.path` and `or` default.
func (p *parser) selectExpr() *node {
	base := p.primary()
	if !p.is(".") {
		return base
	}
	p.next()
	path, _ := p.attrPath()
	n := &node{kind: nSelect, line: base.line, kids: []*node{base}, path: path}
	if p.kw("or") {
		p.next()
		n.kids = append(n.kids, p.selectExpr())
	}
	return n
}

// attrPath reads `a.b."c".${d}`; dynamic parts are "" (their code is returned
// too, so paths inside them are not lost).
func (p *parser) attrPath() ([]string, []*node) {
	var path []string
	var dyn []*node
	for {
		t := p.tok()
		switch {
		case t.kind == tIdent:
			p.next()
			path = append(path, t.text)
		case t.kind == tStrOpen:
			s := p.str()
			if s.interpolate {
				path = append(path, "")
				dyn = append(dyn, s)
			} else {
				path = append(path, s.text)
			}
		case t.kind == tInterp:
			p.next()
			e := p.expr()
			p.closeTo(tInterpEnd)
			path = append(path, "")
			dyn = append(dyn, e)
		default:
			return path, dyn
		}
		if !p.is(".") {
			return path, dyn
		}
		p.next()
	}
}

// closeTo consumes up to and including the next token of kind k at this level.
func (p *parser) closeTo(k tokKind) {
	for !p.eof() {
		t := p.tok()
		if t.kind == k {
			p.next()
			return
		}
		if t.kind == tInterpEnd || t.kind == tStrClose || t.kind == tPathClose ||
			t.kind == tPunct && (t.text == "}" || t.text == ")" || t.text == "]") {
			return // a closer of an outer group: leave it
		}
		p.skipGroup()
	}
}

func (p *parser) primary() *node {
	t := p.tok()
	switch t.kind {
	case tIdent:
		switch t.text {
		case "rec":
			p.next()
			if p.is("{") {
				n := p.attrs()
				n.rec = true
				return n
			}
			return &node{kind: nOther, line: t.line}
		case "let": // let { ...; body = x; }, the old form
			p.next()
			if p.is("{") {
				n := p.attrs()
				n.rec = true
				return n
			}
			return &node{kind: nOther, line: t.line}
		}
		if keywords[t.text] {
			return &node{kind: nOther, line: t.line}
		}
		p.next()
		return &node{kind: nIdent, line: t.line, text: t.text}
	case tNum:
		p.next()
		return &node{kind: nOther, line: t.line, text: t.text}
	case tStrOpen:
		return p.str()
	case tPath:
		p.next()
		return &node{kind: nPath, line: t.line, text: t.text}
	case tPathOpen:
		return p.interpolatePath()
	case tSPath:
		p.next()
		return &node{kind: nSPath, line: t.line, text: t.text}
	case tURI:
		p.next()
		return &node{kind: nURI, line: t.line, text: t.text}
	case tPunct:
		switch t.text {
		case "(":
			open := p.i
			p.next()
			n := &node{kind: nParen, line: t.line, kids: []*node{p.expr()}}
			p.closeGroup(open, ")")
			return n
		case "[":
			open := p.i
			p.next()
			n := &node{kind: nList, line: t.line}
			for !p.eof() && !p.is("]") {
				start := p.i
				if m := p.match[open]; m > 0 && p.i >= m {
					break
				}
				if startsOperand(p.tok()) {
					n.kids = append(n.kids, p.selectExpr())
				}
				if p.i == start {
					p.skipGroup()
				}
			}
			p.closeGroup(open, "]")
			return n
		case "{":
			return p.attrs()
		}
	}
	return &node{kind: nOther, line: t.line}
}

// closeGroup moves to just after the closer matching the opener at open.
func (p *parser) closeGroup(open int, closer string) {
	if m := p.match[open]; m > 0 {
		if p.i < m {
			p.recovered += m - p.i
		}
		if p.i <= m {
			p.i = m + 1
		}
		return
	}
	if p.is(closer) {
		p.next()
	}
}

// str parses a string whose opener is at p.i.
func (p *parser) str() *node {
	open := p.i
	t := p.next()
	n := &node{kind: nString, line: t.line}
	var b strings.Builder
	for !p.eof() {
		t := p.tok()
		switch t.kind {
		case tStrText:
			b.WriteString(t.text)
			p.next()
			continue
		case tInterp:
			n.interpolate = true
			p.next()
			n.kids = append(n.kids, p.expr())
			p.closeTo(tInterpEnd)
			continue
		case tStrClose:
			p.next()
		default:
			p.closeGroup(open, "")
		}
		break
	}
	n.text = b.String()
	return n
}

// interpolatePath parses ./x/${y}.nix: its literal text is kept up to the first
// interpolation.
func (p *parser) interpolatePath() *node {
	open := p.i
	t := p.next()
	n := &node{kind: nPath, line: t.line, text: t.text, interpolate: true}
	for !p.eof() {
		t := p.tok()
		switch t.kind {
		case tPathText:
			p.next()
			continue
		case tInterp:
			p.next()
			n.kids = append(n.kids, p.expr())
			p.closeTo(tInterpEnd)
			continue
		case tPathClose:
			p.next()
		default:
			p.closeGroup(open, "")
		}
		break
	}
	return n
}

// attrs parses `{ bindings }` at a `{`.
func (p *parser) attrs() *node {
	open := p.i
	t := p.next()
	n := &node{kind: nAttrs, line: t.line}
	m := p.match[open]
	n.binds = p.binds(func() bool { return m > 0 && p.i >= m || m <= 0 && p.is("}") }, m > 0)
	p.closeGroup(open, "}")
	return n
}

// binds parses bindings until done() or a closer of an outer group. With stray
// set, done() knows the group's own closer, and any other closer is skipped.
func (p *parser) binds(done func() bool, stray bool) []*bind {
	var out []*bind
	for !p.eof() && !done() {
		start := p.i
		t := p.tok()
		switch {
		case t.kind == tIdent && t.text == "inherit":
			p.next()
			var from *node
			if p.is("(") {
				open := p.i
				p.next()
				from = p.expr()
				p.closeGroup(open, ")")
			}
			for !p.eof() && !p.is(";") {
				u := p.tok()
				if u.kind == tIdent && !keywords[u.text] || u.kind == tStrOpen {
					name := u.text
					if u.kind == tStrOpen {
						name = p.str().text
					} else {
						p.next()
					}
					out = append(out, &bind{path: []string{name}, line: u.line, value: from, inherit: true})
					continue
				}
				break
			}
		case t.kind == tIdent && !keywords[t.text] || t.kind == tStrOpen || t.kind == tInterp:
			path, dyn := p.attrPath()
			if !p.is("=") {
				break
			}
			p.next()
			b := &bind{path: path, line: t.line, value: p.expr()}
			if len(dyn) > 0 { // keep the dynamic parts' code reachable
				b.value = &node{kind: nOther, line: t.line, kids: append(dyn, b.value)}
				b.path = path
			}
			out = append(out, b)
		}
		// To the end of the binding: its ";", or the closer of the enclosing group.
		for !p.eof() && !p.is(";") && !done() && !isCloser(p.tok()) {
			p.recovered++
			p.skipGroup()
		}
		if p.is(";") {
			p.next()
		}
		if p.i == start {
			if isCloser(p.tok()) && !stray {
				break
			}
			p.recovered++
			p.next()
		}
	}
	return out
}

func isCloser(t token) bool {
	return t.kind == tInterpEnd || t.kind == tStrClose || t.kind == tPathClose ||
		t.kind == tPunct && (t.text == "}" || t.text == ")" || t.text == "]")
}
