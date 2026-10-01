package terraform

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// This is a reader of HCL's native syntax that goes only as deep as the plugin
// looks: blocks with their labels, attributes with the tokens of their
// expressions, strings and heredocs as templates whose interpolations are lexed
// in turn. It does not evaluate anything and forgives what it does not
// understand - a token it cannot place is skipped to the end of its line - so a
// file never fails as a whole (REQ-TERRAFORM-011).

type tokenKind uint8

const (
	tIdentifier tokenKind = iota
	tNumber
	tString // a quoted string or a heredoc, both templates
	tPunctuation
	tNL
)

type token struct {
	kind tokenKind
	text string // identifier, number or punctuation; a string's template text
	line int
	// literal marks a string without interpolations or directives: text is its value.
	literal bool
	// sub holds the tokens of a string's interpolations and directives, in order.
	interpolations []token
}

func (t token) is(punctuation string) bool { return t.kind == tPunctuation && t.text == punctuation }

type lexer struct {
	source []byte
	i      int
	line   int
}

// lex reads a whole file.
func lex(source []byte) []token {
	l := &lexer{source: source, line: 1}
	if len(source) >= 3 && source[0] == 0xEF && source[1] == 0xBB && source[2] == 0xBF {
		l.i = 3
	}
	return l.tokens(false)
}

func identifierPart(c byte) bool {
	return chars.IsIdentStartUTF8(c) || c == '-' || c >= '0' && c <= '9'
}

// tokens lexes until the end of the input or, inside an interpolation, the brace
// that closes it (consumed).
func (l *lexer) tokens(interpolate bool) []token {
	var out []token
	depth := 0
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case c == '\n':
			out = append(out, token{kind: tNL, line: l.line})
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r':
			l.i++
		case c == '#' || c == '/' && l.peek(1) == '/':
			for l.i < len(l.source) && l.source[l.i] != '\n' {
				l.i++
			}
		case c == '/' && l.peek(1) == '*':
			l.i += 2
			for l.i < len(l.source) && !(l.source[l.i] == '*' && l.peek(1) == '/') {
				if l.source[l.i] == '\n' {
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
		case chars.IsIdentStartUTF8(c):
			start := l.i
			for l.i < len(l.source) && identifierPart(l.source[l.i]) {
				l.i++
			}
			out = append(out, token{kind: tIdentifier, text: string(l.source[start:l.i]), line: l.line})
		case c >= '0' && c <= '9':
			start := l.i
			for l.i < len(l.source) {
				d := l.source[l.i]
				if d >= '0' && d <= '9' || d == '.' && l.peek(1) >= '0' && l.peek(1) <= '9' ||
					(d == 'e' || d == 'E') ||
					(d == '+' || d == '-') && (l.source[l.i-1] == 'e' || l.source[l.i-1] == 'E') {
					l.i++
					continue
				}
				break
			}
			out = append(out, token{kind: tNumber, text: string(l.source[start:l.i]), line: l.line})
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
			for _, operator := range [...]string{"...", "==", "!=", "<=", ">=", "&&", "||", "=>", "::"} {
				if strings.HasPrefix(string(l.source[l.i:min(l.i+len(operator), len(l.source))]), operator) {
					p = operator
					break
				}
			}
			l.i += len(p)
			out = append(out, token{kind: tPunctuation, text: p, line: l.line})
		}
	}
	return out
}

func (l *lexer) peek(n int) byte {
	if l.i+n < len(l.source) {
		return l.source[l.i+n]
	}
	return 0
}

// template reads a template up to its end: the closing quote of a quoted string
// (whose escapes are decoded), or the end of the input for a heredoc's body. The
// text keeps each interpolation as written ("${path.module}/x.tpl"); its tokens
// go to sub.
func (l *lexer) template(quoted bool) token {
	t := token{kind: tString, line: l.line, literal: true}
	var b strings.Builder
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case quoted && c == '"':
			l.i++
			t.text = b.String()
			return t
		case quoted && c == '\n':
			// An unterminated string ends at its line, as HCL reports it.
			t.text = b.String()
			return t
		case quoted && c == '\\' && l.i+1 < len(l.source):
			l.i += 2
			switch e := l.source[l.i-1]; e {
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
			t.interpolations = append(t.interpolations, l.tokens(true)...)
			b.Write(l.source[start:l.i])
			t.literal = false
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
	if j < len(l.source) && l.source[j] == '-' {
		j++
	}
	if j >= len(l.source) || !chars.IsIdentStartUTF8(l.source[j]) {
		return false
	}
	for j < len(l.source) && identifierPart(l.source[j]) {
		j++
	}
	for j < len(l.source) && (l.source[j] == ' ' || l.source[j] == '\t' || l.source[j] == '\r') {
		j++
	}
	return j >= len(l.source) || l.source[j] == '\n'
}

// heredoc reads a heredoc: its body up to the line holding only the marker
// (indented or not), lexed as a template. Lines are kept.
func (l *lexer) heredoc() token {
	l.i += 2
	if l.source[l.i] == '-' {
		l.i++
	}
	start := l.i
	for l.i < len(l.source) && identifierPart(l.source[l.i]) {
		l.i++
	}
	marker := string(l.source[start:l.i])
	line := l.line
	for l.i < len(l.source) && l.source[l.i] != '\n' {
		l.i++
	}
	if l.i < len(l.source) {
		l.i++
		l.line++
	}
	body := l.i
	end, next := len(l.source), len(l.source)
	for p := body; p < len(l.source); {
		e := strings.IndexByte(string(l.source[p:]), '\n')
		lineEnd := len(l.source)
		if e >= 0 {
			lineEnd = p + e
		}
		if strings.TrimSpace(string(l.source[p:lineEnd])) == marker {
			end, next = p, lineEnd
			break
		}
		p = lineEnd + 1
	}
	inner := &lexer{source: l.source[:end], i: body, line: l.line}
	t := inner.template(false)
	t.line = line
	l.line = inner.line
	l.i = next
	return t
}

// ---------------------------------------------------------------- structure

type attribute struct {
	name       string
	line       int
	expression []token
}

type block struct {
	typeName   string
	labels     []string
	line       int
	attributes []attribute
	blocks     []*block
}

// get returns the attribute named name.
func (b *block) get(name string) (attribute, bool) {
	for _, a := range b.attributes {
		if a.name == name {
			return a, true
		}
	}
	return attribute{}, false
}

type parser struct {
	tokens []token
	i      int
}

// parse reads a file's body into a block without a type.
func parse(source []byte) *block {
	p := &parser{tokens: lex(source)}
	root := &block{line: 1}
	p.body(root)
	return root
}

func (p *parser) at(k int) token {
	if p.i+k < len(p.tokens) {
		return p.tokens[p.i+k]
	}
	return token{kind: tNL}
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
		case t.kind == tIdentifier && p.at(1).is("="):
			p.i += 2
			b.attributes = append(b.attributes, attribute{name: t.text, line: t.line, expression: p.expression()})
		case t.kind == tIdentifier:
			child := &block{typeName: t.text, line: t.line}
			p.i++
			for !p.done() && (p.tokens[p.i].kind == tString || p.tokens[p.i].kind == tIdentifier) {
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

// expression collects an attribute's expression: up to the end of its line outside
// brackets, or the brace closing the block it is in (a one-line block).
func (p *parser) expression() []token {
	start, depth := p.i, 0
	for ; !p.done(); p.i++ {
		t := p.tokens[p.i]
		switch {
		case t.kind == tNL && depth == 0:
			return p.tokens[start:p.i]
		case t.kind != tPunctuation:
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
func literal(expression []token) (string, bool) {
	if len(expression) == 1 && expression[0].kind == tString && expression[0].literal {
		return expression[0].text, true
	}
	return "", false
}

// stringExpression is the template text of an expression that is one string, literal
// or not.
func stringExpression(expression []token) (string, bool) {
	if len(expression) == 1 && expression[0].kind == tString {
		return expression[0].text, true
	}
	return "", false
}

// item is one entry of an object constructor: key = value or key: value.
type item struct {
	key   string
	line  int
	value []token
}

// object reads an object constructor's entries; nil when expression is not one.
func object(expression []token) []item {
	if len(expression) < 2 || !expression[0].is("{") || !expression[len(expression)-1].is("}") {
		return nil
	}
	inner := expression[1 : len(expression)-1]
	var out []item
	for i := 0; i < len(inner); {
		t := inner[i]
		if t.kind == tNL || t.is(",") {
			i++
			continue
		}
		if (t.kind != tIdentifier && t.kind != tString) || i+1 >= len(inner) || !(inner[i+1].is("=") || inner[i+1].is(":")) {
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
		it.value = inner[start:i]
		out = append(out, it)
	}
	return out
}

// walk calls fn for every token of expression, descending into string interpolations.
func walk(expression []token, function func(tokens []token, i int)) {
	for i, t := range expression {
		function(expression, i)
		if len(t.interpolations) > 0 {
			walk(t.interpolations, function)
		}
	}
}
