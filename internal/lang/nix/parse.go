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
	nIdentifier
	nSelect // kids[0] base, path; kids[1] the `or` default
	nString // text when literal; interpolate; kids = interpolated expressions
	nPath   // text (literal part); interpolate; kids = interpolated expressions
	nSPath  // <text>
	nURI
	nAttributes  // binds; rec
	nLet         // binds; kids[0] body
	nWith        // kids[0] environment, kids[1] body
	nAssert      // kids[0] condition, kids[1] body
	nIf          // kids[0..2]
	nLambda      // arg, formals (defaults in binds), set; kids[0] body
	nApply       // kids[0] function, kids[1:] arguments
	nList        // kids
	nBinary      // text = operator; kids = operands (a chain of one operator is flat)
	nUnary       // kids[0]
	nParenthesis // kids[0]
)

type node struct {
	kind        nodeKind
	line        int
	text        string
	interpolate bool
	recursive   bool
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
func parse(source []byte) *node {
	tokens := lex(source)
	p := &parser{tokens: tokens, match: matchBrackets(tokens)}
	root := p.expression()
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
		case tStringOpen:
			return c.kind == tStringClose
		case tPathOpen:
			return c.kind == tPathClose
		case tInterpolation:
			return c.kind == tInterpolationEnd
		}
		switch open.text {
		case "{":
			return c.kind == tPunctuation && c.text == "}"
		case "(":
			return c.kind == tPunctuation && c.text == ")"
		case "[":
			return c.kind == tPunctuation && c.text == "]"
		}
		return false
	}
	for i, t := range tokens {
		switch {
		case t.kind == tStringOpen || t.kind == tPathOpen || t.kind == tInterpolation,
			t.kind == tPunctuation && (t.text == "{" || t.text == "(" || t.text == "["):
			stack = append(stack, i)
		case t.kind == tStringClose || t.kind == tPathClose || t.kind == tInterpolationEnd,
			t.kind == tPunctuation && (t.text == "}" || t.text == ")" || t.text == "]"):
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

func (p *parser) token() token { return p.tokens[p.i] }

func (p *parser) peekToken(k int) token {
	if p.i+k < len(p.tokens) {
		return p.tokens[p.i+k]
	}
	return p.tokens[len(p.tokens)-1]
}

func (p *parser) is(text string) bool {
	t := p.tokens[p.i]
	return t.kind == tPunctuation && t.text == text
}

func (p *parser) keyword(word string) bool {
	t := p.tokens[p.i]
	return t.kind == tIdentifier && t.text == word
}

func (p *parser) next() token {
	t := p.tokens[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) atEnd() bool { return p.tokens[p.i].kind == tEOF }

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

// expression parses an expression: a lambda, let, with, assert, if or an operator
// expression.
func (p *parser) expression() *node {
	p.depth++
	defer func() { p.depth-- }()
	t := p.token()
	if p.depth > maxDepth {
		p.skipGroup()
		return &node{kind: nOther, line: t.line}
	}
	switch {
	case t.kind == tIdentifier && !keywords[t.text] && p.peekToken(1).kind == tPunctuation && p.peekToken(1).text == ":":
		p.i += 2
		return &node{kind: nLambda, line: t.line, text: t.text, kids: []*node{p.expression()}}
	case t.kind == tIdentifier && !keywords[t.text] && p.peekToken(1).kind == tPunctuation && p.peekToken(1).text == "@":
		p.i += 2
		if p.is("{") {
			n := p.formals()
			n.text = t.text
			return n
		}
		return &node{kind: nOther, line: t.line}
	case t.kind == tPunctuation && t.text == "{" && p.isFormals():
		return p.formals()
	case t.kind == tIdentifier && t.text == "let" && !(p.peekToken(1).kind == tPunctuation && p.peekToken(1).text == "{"):
		p.next()
		n := &node{kind: nLet, line: t.line}
		n.binds = p.binds(func() bool { return p.keyword("in") }, false)
		if p.keyword("in") {
			p.next()
		}
		n.kids = []*node{p.expression()}
		return n
	case t.kind == tIdentifier && (t.text == "with" || t.text == "assert"):
		p.next()
		k := nWith
		if t.text == "assert" {
			k = nAssert
		}
		environment := p.expression()
		if p.is(";") {
			p.next()
		}
		return &node{kind: k, line: t.line, kids: []*node{environment, p.expression()}}
	case t.kind == tIdentifier && t.text == "if":
		p.next()
		n := &node{kind: nIf, line: t.line}
		c := p.expression()
		var a, b *node
		if p.keyword("then") {
			p.next()
			a = p.expression()
		}
		if p.keyword("else") {
			p.next()
			b = p.expression()
		}
		n.kids = nonNil(c, a, b)
		return n
	}
	return p.binary(0)
}

func nonNil(nodes ...*node) []*node {
	out := nodes[:0]
	for _, n := range nodes {
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}

// isFormals tells `{ a, b ? 1, ... }:` (or `{ ... } @ args:`) from an attribute
// set at a `{`.
func (p *parser) isFormals() bool {
	a, b := p.peekToken(1), p.peekToken(2)
	switch {
	case a.kind == tPunctuation && a.text == "}":
		c := p.peekToken(2)
		return c.kind == tPunctuation && (c.text == ":" || c.text == "@")
	case a.kind == tPunctuation && a.text == "...":
		return true
	case a.kind == tIdentifier && !keywords[a.text]:
		if b.kind == tPunctuation && (b.text == "," || b.text == "?") {
			return true
		}
		if b.kind == tPunctuation && b.text == "}" {
			c := p.peekToken(3)
			return c.kind == tPunctuation && (c.text == ":" || c.text == "@")
		}
	}
	return false
}

// formals parses `{ a, b ? default, ... } [@ name] : body` at a `{`.
func (p *parser) formals() *node {
	open := p.i
	n := &node{kind: nLambda, line: p.token().line, set: true}
	p.next()
	for !p.atEnd() && !p.is("}") {
		start := p.i
		t := p.token()
		switch {
		case t.kind == tIdentifier:
			p.next()
			n.formals = append(n.formals, t.text)
			if p.is("?") {
				p.next()
				n.binds = append(n.binds, &bind{path: []string{t.text}, line: t.line, value: p.expression()})
			}
		case t.kind == tPunctuation && (t.text == "," || t.text == "..."):
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
		if p.token().kind == tIdentifier {
			n.text = p.next().text
		}
	}
	if p.is(":") {
		p.next()
	}
	n.kids = []*node{p.expression()}
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
	t := p.token()
	switch {
	case p.depth > maxDepth:
		p.skipGroup()
		return &node{kind: nOther, line: t.line}
	case t.kind == tPunctuation && t.text == "!":
		p.next()
		left = &node{kind: nUnary, line: t.line, kids: []*node{p.binary(8)}}
	case t.kind == tPunctuation && t.text == "-":
		p.next()
		left = &node{kind: nUnary, line: t.line, kids: []*node{p.binary(13)}}
	default:
		left = p.apply()
	}
	for {
		t := p.token()
		level, ok := precedence[t.text]
		if t.kind != tPunctuation || !ok || level <= min && min > 0 || level < min {
			return left
		}
		p.next()
		if t.text == "?" { // a ? b.c: an attribute path, not an expression
			path, _ := p.attributePath()
			left = &node{kind: nBinary, line: t.line, text: "?", kids: []*node{left, {kind: nOther, path: path}}}
			continue
		}
		right := p.binary(level)
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
	case tIdentifier:
		return !keywords[t.text] || t.text == "rec"
	case tNumber, tStringOpen, tPath, tPathOpen, tSPath, tURI:
		return true
	case tPunctuation:
		return t.text == "(" || t.text == "[" || t.text == "{"
	}
	return false
}

func (p *parser) apply() *node {
	function := p.selectExpression()
	if !startsOperand(p.token()) {
		return function
	}
	n := &node{kind: nApply, line: function.line, kids: []*node{function}}
	for startsOperand(p.token()) {
		start := p.i
		n.kids = append(n.kids, p.selectExpression())
		if p.i == start {
			p.next()
		}
	}
	return n
}

// selectExpression parses a primary with its `.attr.path` and `or` default.
func (p *parser) selectExpression() *node {
	base := p.primary()
	if !p.is(".") {
		return base
	}
	p.next()
	path, _ := p.attributePath()
	n := &node{kind: nSelect, line: base.line, kids: []*node{base}, path: path}
	if p.keyword("or") {
		p.next()
		n.kids = append(n.kids, p.selectExpression())
	}
	return n
}

// attributePath reads `a.b."c".${d}`; dynamic parts are "" (their code is returned
// too, so paths inside them are not lost).
func (p *parser) attributePath() ([]string, []*node) {
	var path []string
	var dynamic []*node
	for {
		t := p.token()
		switch {
		case t.kind == tIdentifier:
			p.next()
			path = append(path, t.text)
		case t.kind == tStringOpen:
			s := p.parseString()
			if s.interpolate {
				path = append(path, "")
				dynamic = append(dynamic, s)
			} else {
				path = append(path, s.text)
			}
		case t.kind == tInterpolation:
			p.next()
			e := p.expression()
			p.closeTo(tInterpolationEnd)
			path = append(path, "")
			dynamic = append(dynamic, e)
		default:
			return path, dynamic
		}
		if !p.is(".") {
			return path, dynamic
		}
		p.next()
	}
}

// closeTo consumes up to and including the next token of kind k at this level.
func (p *parser) closeTo(k tokenKind) {
	for !p.atEnd() {
		t := p.token()
		if t.kind == k {
			p.next()
			return
		}
		if t.kind == tInterpolationEnd || t.kind == tStringClose || t.kind == tPathClose ||
			t.kind == tPunctuation && (t.text == "}" || t.text == ")" || t.text == "]") {
			return // a closer of an outer group: leave it
		}
		p.skipGroup()
	}
}

func (p *parser) primary() *node {
	t := p.token()
	switch t.kind {
	case tIdentifier:
		switch t.text {
		case "rec":
			p.next()
			if p.is("{") {
				n := p.attributes()
				n.recursive = true
				return n
			}
			return &node{kind: nOther, line: t.line}
		case "let": // let { ...; body = x; }, the old form
			p.next()
			if p.is("{") {
				n := p.attributes()
				n.recursive = true
				return n
			}
			return &node{kind: nOther, line: t.line}
		}
		if keywords[t.text] {
			return &node{kind: nOther, line: t.line}
		}
		p.next()
		return &node{kind: nIdentifier, line: t.line, text: t.text}
	case tNumber:
		p.next()
		return &node{kind: nOther, line: t.line, text: t.text}
	case tStringOpen:
		return p.parseString()
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
	case tPunctuation:
		switch t.text {
		case "(":
			open := p.i
			p.next()
			n := &node{kind: nParenthesis, line: t.line, kids: []*node{p.expression()}}
			p.closeGroup(open, ")")
			return n
		case "[":
			open := p.i
			p.next()
			n := &node{kind: nList, line: t.line}
			for !p.atEnd() && !p.is("]") {
				start := p.i
				if m := p.match[open]; m > 0 && p.i >= m {
					break
				}
				if startsOperand(p.token()) {
					n.kids = append(n.kids, p.selectExpression())
				}
				if p.i == start {
					p.skipGroup()
				}
			}
			p.closeGroup(open, "]")
			return n
		case "{":
			return p.attributes()
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

// parseString parses a string whose opener is at p.i.
func (p *parser) parseString() *node {
	open := p.i
	t := p.next()
	n := &node{kind: nString, line: t.line}
	var b strings.Builder
	for !p.atEnd() {
		t := p.token()
		switch t.kind {
		case tStringText:
			b.WriteString(t.text)
			p.next()
			continue
		case tInterpolation:
			n.interpolate = true
			p.next()
			n.kids = append(n.kids, p.expression())
			p.closeTo(tInterpolationEnd)
			continue
		case tStringClose:
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
	for !p.atEnd() {
		t := p.token()
		switch t.kind {
		case tPathText:
			p.next()
			continue
		case tInterpolation:
			p.next()
			n.kids = append(n.kids, p.expression())
			p.closeTo(tInterpolationEnd)
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

// attributes parses `{ bindings }` at a `{`.
func (p *parser) attributes() *node {
	open := p.i
	t := p.next()
	n := &node{kind: nAttributes, line: t.line}
	m := p.match[open]
	n.binds = p.binds(func() bool { return m > 0 && p.i >= m || m <= 0 && p.is("}") }, m > 0)
	p.closeGroup(open, "}")
	return n
}

// binds parses bindings until done() or a closer of an outer group. With stray
// set, done() knows the group's own closer, and any other closer is skipped.
func (p *parser) binds(done func() bool, stray bool) []*bind {
	var out []*bind
	for !p.atEnd() && !done() {
		start := p.i
		t := p.token()
		switch {
		case t.kind == tIdentifier && t.text == "inherit":
			p.next()
			var from *node
			if p.is("(") {
				open := p.i
				p.next()
				from = p.expression()
				p.closeGroup(open, ")")
			}
			for !p.atEnd() && !p.is(";") {
				u := p.token()
				if u.kind == tIdentifier && !keywords[u.text] || u.kind == tStringOpen {
					name := u.text
					if u.kind == tStringOpen {
						name = p.parseString().text
					} else {
						p.next()
					}
					out = append(out, &bind{path: []string{name}, line: u.line, value: from, inherit: true})
					continue
				}
				break
			}
		case t.kind == tIdentifier && !keywords[t.text] || t.kind == tStringOpen || t.kind == tInterpolation:
			path, dynamic := p.attributePath()
			if !p.is("=") {
				break
			}
			p.next()
			b := &bind{path: path, line: t.line, value: p.expression()}
			if len(dynamic) > 0 { // keep the dynamic parts' code reachable
				b.value = &node{kind: nOther, line: t.line, kids: append(dynamic, b.value)}
				b.path = path
			}
			out = append(out, b)
		}
		// To the end of the binding: its ";", or the closer of the enclosing group.
		for !p.atEnd() && !p.is(";") && !done() && !isCloser(p.token()) {
			p.recovered++
			p.skipGroup()
		}
		if p.is(";") {
			p.next()
		}
		if p.i == start {
			if isCloser(p.token()) && !stray {
				break
			}
			p.recovered++
			p.next()
		}
	}
	return out
}

func isCloser(t token) bool {
	return t.kind == tInterpolationEnd || t.kind == tStringClose || t.kind == tPathClose ||
		t.kind == tPunctuation && (t.text == "}" || t.text == ")" || t.text == "]")
}
