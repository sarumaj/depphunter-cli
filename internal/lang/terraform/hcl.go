package terraform

import (
	"strings"
)

// This is a reader of HCL's native syntax that goes only as deep as the plugin
// looks: blocks with their labels, attributes with the tokens of their
// expressions, strings and heredocs as templates whose interpolations are lexed
// in turn. It does not evaluate anything and forgives what it does not
// understand - a token it cannot place is skipped to the end of its line - so a
// file never fails as a whole (REQ-TERRAFORM-011).

type tokKind uint8

const (
	tIdent tokKind = iota
	tNumber
	tString // a quoted string or a heredoc, both templates
	tPunct
	tNL
)

type tok struct {
	kind tokKind
	text string // identifier, number or punctuation; a string's template text
	line int
	// lit marks a string without interpolations or directives: text is its value.
	lit bool
	// sub holds the tokens of a string's interpolations and directives, in order.
	sub []tok
}

func (t tok) is(punct string) bool { return t.kind == tPunct && t.text == punct }

type lexer struct {
	src  []byte
	i    int
	line int
}

// lex reads a whole file.
func lex(src []byte) []tok {
	l := &lexer{src: src, line: 1}
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		l.i = 3
	}
	return l.tokens(false)
}

func identStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func identPart(c byte) bool { return identStart(c) || c == '-' || c >= '0' && c <= '9' }

// tokens lexes until the end of the input or, inside an interpolation, the brace
// that closes it (consumed).
func (l *lexer) tokens(interpolate bool) []tok {
	var out []tok
	depth := 0
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '\n':
			out = append(out, tok{kind: tNL, line: l.line})
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r':
			l.i++
		case c == '#' || c == '/' && l.peek(1) == '/':
			for l.i < len(l.src) && l.src[l.i] != '\n' {
				l.i++
			}
		case c == '/' && l.peek(1) == '*':
			l.i += 2
			for l.i < len(l.src) && !(l.src[l.i] == '*' && l.peek(1) == '/') {
				if l.src[l.i] == '\n' {
					l.line++
				}
				l.i++
			}
			l.i += 2
		case c == '"':
			l.i++
			out = append(out, l.template(true))
		case c == '<' && l.peek(1) == '<' && l.heredocStart():
			out = append(out, l.heredoc())
		case identStart(c):
			start := l.i
			for l.i < len(l.src) && identPart(l.src[l.i]) {
				l.i++
			}
			out = append(out, tok{kind: tIdent, text: string(l.src[start:l.i]), line: l.line})
		case c >= '0' && c <= '9':
			start := l.i
			for l.i < len(l.src) {
				d := l.src[l.i]
				if d >= '0' && d <= '9' || d == '.' && l.peek(1) >= '0' && l.peek(1) <= '9' ||
					(d == 'e' || d == 'E') ||
					(d == '+' || d == '-') && (l.src[l.i-1] == 'e' || l.src[l.i-1] == 'E') {
					l.i++
					continue
				}
				break
			}
			out = append(out, tok{kind: tNumber, text: string(l.src[start:l.i]), line: l.line})
		default:
			if c == '}' && depth == 0 && interpolate {
				l.i++
				return out
			}
			switch c {
			case '{', '[', '(':
				depth++
			case '}', ']', ')':
				depth--
			}
			p := string(c)
			for _, op := range [...]string{"...", "==", "!=", "<=", ">=", "&&", "||", "=>", "::"} {
				if strings.HasPrefix(string(l.src[l.i:min(l.i+len(op), len(l.src))]), op) {
					p = op
					break
				}
			}
			l.i += len(p)
			out = append(out, tok{kind: tPunct, text: p, line: l.line})
		}
	}
	return out
}

func (l *lexer) peek(n int) byte {
	if l.i+n < len(l.src) {
		return l.src[l.i+n]
	}
	return 0
}

// template reads a template up to its end: the closing quote of a quoted string
// (whose escapes are decoded), or the end of the input for a heredoc's body. The
// text keeps each interpolation as written ("${path.module}/x.tpl"); its tokens
// go to sub.
func (l *lexer) template(quoted bool) tok {
	t := tok{kind: tString, line: l.line, lit: true}
	var b strings.Builder
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case quoted && c == '"':
			l.i++
			t.text = b.String()
			return t
		case quoted && c == '\n':
			// An unterminated string ends at its line, as HCL reports it.
			t.text = b.String()
			return t
		case quoted && c == '\\' && l.i+1 < len(l.src):
			l.i += 2
			switch e := l.src[l.i-1]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(e)
			}
		case (c == '$' || c == '%') && l.peek(1) == c && l.peek(2) == '{':
			b.WriteByte(c) // "$${" is a literal "${"
			b.WriteByte('{')
			l.i += 3
		case (c == '$' || c == '%') && l.peek(1) == '{':
			start := l.i
			l.i += 2
			t.sub = append(t.sub, l.tokens(true)...)
			b.Write(l.src[start:l.i])
			t.lit = false
		default:
			if c == '\n' {
				l.line++
			}
			b.WriteByte(c)
			l.i++
		}
	}
	t.text = b.String()
	return t
}

// heredocStart reports whether "<<" starts a heredoc: "<<EOT" or "<<-EOT" followed
// by the end of the line.
func (l *lexer) heredocStart() bool {
	j := l.i + 2
	if j < len(l.src) && l.src[j] == '-' {
		j++
	}
	if j >= len(l.src) || !identStart(l.src[j]) {
		return false
	}
	for j < len(l.src) && identPart(l.src[j]) {
		j++
	}
	for j < len(l.src) && (l.src[j] == ' ' || l.src[j] == '\t' || l.src[j] == '\r') {
		j++
	}
	return j >= len(l.src) || l.src[j] == '\n'
}

// heredoc reads a heredoc: its body up to the line holding only the marker
// (indented or not), lexed as a template. Lines are kept.
func (l *lexer) heredoc() tok {
	l.i += 2
	if l.src[l.i] == '-' {
		l.i++
	}
	start := l.i
	for l.i < len(l.src) && identPart(l.src[l.i]) {
		l.i++
	}
	marker := string(l.src[start:l.i])
	line := l.line
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		l.i++
	}
	if l.i < len(l.src) {
		l.i++
		l.line++
	}
	body := l.i
	end, next := len(l.src), len(l.src)
	for p := body; p < len(l.src); {
		e := strings.IndexByte(string(l.src[p:]), '\n')
		lineEnd := len(l.src)
		if e >= 0 {
			lineEnd = p + e
		}
		if strings.TrimSpace(string(l.src[p:lineEnd])) == marker {
			end, next = p, lineEnd
			break
		}
		p = lineEnd + 1
	}
	inner := &lexer{src: l.src[:end], i: body, line: l.line}
	t := inner.template(false)
	t.line = line
	l.line = inner.line
	l.i = next
	return t
}

// ---------------------------------------------------------------- structure

type attr struct {
	name string
	line int
	expr []tok
}

type block struct {
	typ    string
	labels []string
	line   int
	attrs  []attr
	blocks []*block
}

// get returns the attribute named name.
func (b *block) get(name string) (attr, bool) {
	for _, a := range b.attrs {
		if a.name == name {
			return a, true
		}
	}
	return attr{}, false
}

type parser struct {
	tokens []tok
	i      int
}

// parse reads a file's body into a block without a type.
func parse(src []byte) *block {
	p := &parser{tokens: lex(src)}
	root := &block{line: 1}
	p.body(root)
	return root
}

func (p *parser) at(k int) tok {
	if p.i+k < len(p.tokens) {
		return p.tokens[p.i+k]
	}
	return tok{kind: tNL}
}

func (p *parser) done() bool { return p.i >= len(p.tokens) }

// body reads attributes and blocks up to the brace closing it (consumed) or the end.
func (p *parser) body(b *block) {
	for !p.done() {
		t := p.tokens[p.i]
		switch {
		case t.kind == tNL:
			p.i++
		case t.is("}"):
			p.i++
			return
		case t.kind == tIdent && p.at(1).is("="):
			p.i += 2
			b.attrs = append(b.attrs, attr{name: t.text, line: t.line, expr: p.expr()})
		case t.kind == tIdent:
			child := &block{typ: t.text, line: t.line}
			p.i++
			for !p.done() && (p.tokens[p.i].kind == tString || p.tokens[p.i].kind == tIdent) {
				child.labels = append(child.labels, p.tokens[p.i].text)
				p.i++
			}
			if !p.done() && p.tokens[p.i].is("{") {
				p.i++
				p.body(child)
				b.blocks = append(b.blocks, child)
				continue
			}
			p.skipLine()
		default:
			p.skipLine()
		}
	}
}

// expr collects an attribute's expression: up to the end of its line outside
// brackets, or the brace closing the block it is in (a one-line block).
func (p *parser) expr() []tok {
	start, depth := p.i, 0
	for ; !p.done(); p.i++ {
		t := p.tokens[p.i]
		switch {
		case t.kind == tNL && depth == 0:
			return p.tokens[start:p.i]
		case t.kind != tPunct:
		case t.text == "{" || t.text == "[" || t.text == "(":
			depth++
		case t.text == "}" || t.text == "]" || t.text == ")":
			if depth == 0 {
				return p.tokens[start:p.i]
			}
			depth--
		}
	}
	return p.tokens[start:p.i]
}

// skipLine passes over what cannot be read, brackets and all.
func (p *parser) skipLine() {
	depth := 0
	for ; !p.done(); p.i++ {
		t := p.tokens[p.i]
		switch {
		case t.kind == tNL && depth <= 0:
			p.i++
			return
		case t.is("{") || t.is("[") || t.is("("):
			depth++
		case t.is("}") || t.is("]") || t.is(")"):
			if depth == 0 {
				return // the enclosing block's end
			}
			depth--
		}
	}
}

// literal is the value of an expression that is one string without
// interpolations.
func literal(expr []tok) (string, bool) {
	if len(expr) == 1 && expr[0].kind == tString && expr[0].lit {
		return expr[0].text, true
	}
	return "", false
}

// stringExpr is the template text of an expression that is one string, literal
// or not.
func stringExpr(expr []tok) (string, bool) {
	if len(expr) == 1 && expr[0].kind == tString {
		return expr[0].text, true
	}
	return "", false
}

// item is one entry of an object constructor: key = value or key: value.
type item struct {
	key  string
	line int
	val  []tok
}

// object reads an object constructor's entries; nil when expr is not one.
func object(expr []tok) []item {
	if len(expr) < 2 || !expr[0].is("{") || !expr[len(expr)-1].is("}") {
		return nil
	}
	inner := expr[1 : len(expr)-1]
	var out []item
	for i := 0; i < len(inner); {
		t := inner[i]
		if t.kind == tNL || t.is(",") {
			i++
			continue
		}
		if (t.kind != tIdent && t.kind != tString) || i+1 >= len(inner) || !(inner[i+1].is("=") || inner[i+1].is(":")) {
			i++
			continue
		}
		it := item{key: t.text, line: t.line}
		i += 2
		start, depth := i, 0
	value:
		for ; i < len(inner); i++ {
			v := inner[i]
			switch {
			case depth == 0 && (v.kind == tNL || v.is(",")):
				break value
			case v.is("{") || v.is("[") || v.is("("):
				depth++
			case v.is("}") || v.is("]") || v.is(")"):
				depth--
			}
		}
		it.val = inner[start:i]
		out = append(out, it)
	}
	return out
}

// walk calls fn for every token of expr, descending into string interpolations.
func walk(expr []tok, fn func(tokens []tok, i int)) {
	for i, t := range expr {
		fn(expr, i)
		if len(t.sub) > 0 {
			walk(t.sub, fn)
		}
	}
}
