package crystal

import "strings"

// The lexer reads what the extraction needs from Crystal: identifiers and
// constants, punctuation, and literals as single tokens so nothing inside a
// string, a heredoc, a regex, a %-literal, a char or a comment is read as code.
// Macro control ({% ... %}) and macro expressions ({{ ... }}) are opaque tokens
// too, so a `{% end %}` does not close a block and `def {{name}}` defines nothing
// the map could name.

type kind uint8

const (
	kIdent  kind = iota // identifiers and keywords, with a trailing ? or !
	kConst              // capitalized identifiers
	kString             // "..", `..`, %(..), heredocs; text is the raw content
	kChar
	kNumber
	kSymbol
	kRegex
	kVar    // @x, @@x, $x
	kMacro  // {% ... %}
	kExpand // {{ ... }}
	kPunct
)

type token struct {
	kind        kind
	text        string
	line        int
	first       bool // first token on its line
	space       bool // whitespace before it
	label       bool // an identifier written as a named argument or key: `name:`
	interpolate bool // a string with #{...} in it
}

// maxNest bounds how deep interpolations nest before the lexer stops treating
// quotes inside them as strings: the input stays linear either way.
const maxNest = 64

type lexer struct {
	s       string
	i       int
	line    int
	tokens  []token
	first   bool
	space   bool
	pending []heredoc // heredocs whose bodies start at the next line
}

type heredoc struct {
	id          string
	interpolate bool
	tok         int // index of its token
}

// lex splits src into tokens.
//
// Implements: REQ-CRYSTAL-010
func lex(src []byte) []token {
	s := string(src)
	s = strings.TrimPrefix(s, "\xef\xbb\xbf")
	l := &lexer{s: s, line: 1, first: true}
	l.run()
	return l.tokens
}

func (l *lexer) emit(k kind, text string) {
	l.tokens = append(l.tokens, token{kind: k, text: text, line: l.line, first: l.first, space: l.space})
	l.first, l.space = false, false
}

// operand reports whether the last token ends an expression, so `/` and `%` after
// it divide rather than start a literal.
func (l *lexer) operand() bool {
	if len(l.tokens) == 0 {
		return false
	}
	t := l.tokens[len(l.tokens)-1]
	switch t.kind {
	case kIdent:
		return !exprKeywords[t.text]
	case kPunct:
		return t.text == ")" || t.text == "]" || t.text == "}"
	case kMacro:
		return false
	}
	return true
}

// exprKeywords are keywords an expression follows.
var exprKeywords = map[string]bool{
	"if": true, "unless": true, "while": true, "until": true, "return": true, "break": true, "next": true,
	"when": true, "case": true, "in": true, "else": true, "elsif": true, "then": true, "do": true,
	"yield": true, "begin": true, "ensure": true, "require": true, "puts": true, "p": true, "raise": true,
}

// callArg reports whether the last token is a method name written before an
// argument without parentheses (`foo /re/`, `puts %w(a)`): an identifier
// followed by a space, with no space after the operator character at i.
func (l *lexer) callArg() bool {
	if len(l.tokens) == 0 || !l.space {
		return false
	}
	t := l.tokens[len(l.tokens)-1]
	if t.kind != kIdent || exprKeywords[t.text] {
		return false
	}
	c := byte('\n')
	if l.i+1 < len(l.s) {
		c = l.s[l.i+1]
	}
	// x // 2 and x %= 2 are operators.
	return c != ' ' && c != '\t' && c != '=' && c != '\n' && c != l.s[l.i]
}

func (l *lexer) run() {
	s := l.s
	for l.i < len(s) {
		c := s[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
			l.first, l.space = true, true
			if len(l.pending) > 0 {
				l.heredocBodies()
			}
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
			l.space = true
		case c == '\\' && l.i+1 < len(s) && s[l.i+1] == '\n':
			l.i += 2 // line continuation
			l.line++
			l.space = true
		case c == '#':
			for l.i < len(s) && s[l.i] != '\n' {
				l.i++
			}
		case isIdentStart(c):
			l.ident()
		case c >= '0' && c <= '9':
			start := l.i
			for l.i < len(s) && (isIdentChar(s[l.i]) || s[l.i] == '.' && l.i+1 < len(s) && s[l.i+1] >= '0' && s[l.i+1] <= '9') {
				l.i++
			}
			l.emit(kNumber, s[start:l.i])
		case (c == '`' || c == '/' || c == '%') && l.afterDef():
			l.punct() // def `(cmd), def /(other), def %(other)
		case c == '"' || c == '`':
			l.str(c, 0, true, false)
		case c == '\'':
			l.char()
		case c == ':':
			l.colon()
		case c == '@':
			start := l.i
			l.i++
			if l.i < len(s) && s[l.i] == '@' {
				l.i++
			}
			if l.i < len(s) && isIdentStart(s[l.i]) {
				for l.i < len(s) && isIdentChar(s[l.i]) {
					l.i++
				}
				l.emit(kVar, s[start:l.i])
			} else {
				l.i = start + 1
				l.emit(kPunct, "@")
			}
		case c == '$':
			start := l.i
			l.i++
			if l.i < len(s) && isIdentStart(s[l.i]) {
				for l.i < len(s) && isIdentChar(s[l.i]) {
					l.i++
				}
			} else if l.i < len(s) && s[l.i] != '\n' {
				l.i++ // $~, $?, $1
				for l.i < len(s) && s[l.i] >= '0' && s[l.i] <= '9' {
					l.i++
				}
			}
			l.emit(kVar, s[start:l.i])
		case c == '{' && l.i+1 < len(s) && (s[l.i+1] == '%' || s[l.i+1] == '{'):
			l.macro()
		case c == '<' && strings.HasPrefix(s[l.i:], "<<-") && l.i+3 < len(s) && (isIdentStart(s[l.i+3]) || s[l.i+3] == '\''):
			l.heredocStart()
		case c == '/' && (!l.operand() || l.callArg()):
			if !l.regex() {
				l.punct()
			}
		case c == '%' && (!l.operand() || l.callArg()) && l.percent():
		default:
			l.punct()
		}
	}
}

// afterDef reports whether the last token is `def` or a `.`, after which an
// operator character is a method's name.
func (l *lexer) afterDef() bool {
	if len(l.tokens) == 0 {
		return false
	}
	t := l.tokens[len(l.tokens)-1]
	return t.kind == kIdent && t.text == "def" || t.kind == kPunct && t.text == "."
}

func isIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= 0x80
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || c >= '0' && c <= '9'
}

func (l *lexer) ident() {
	s := l.s
	start := l.i
	for l.i < len(s) && isIdentChar(s[l.i]) {
		l.i++
	}
	// foo? and foo! are method names; foo!= is foo followed by !=.
	if l.i < len(s) && (s[l.i] == '?' || s[l.i] == '!') && (l.i+1 >= len(s) || s[l.i+1] != '=') {
		l.i++
	}
	k := kIdent
	if c := s[start]; c >= 'A' && c <= 'Z' {
		k = kConst
	}
	l.emit(k, s[start:l.i])
	// A label is `name:` followed by something other than a second colon.
	if l.i < len(s) && s[l.i] == ':' && (l.i+1 >= len(s) || s[l.i+1] != ':') {
		l.tokens[len(l.tokens)-1].label = true
	}
}

var puncts = []string{"<=>", "===", "...", "**=", "<<=", ">>=", "&&=", "||=", "//=", "//", "::", "->", "=>", "==", "!=", "=~", "!~",
	"<=", ">=", "&&", "||", "**", "<<", ">>", "+=", "-=", "*=", "/=", "%=", "|=", "&=", "^=", "..", "&."}

func (l *lexer) punct() {
	rest := l.s[l.i:]
	for _, p := range puncts {
		if strings.HasPrefix(rest, p) {
			l.emit(kPunct, p)
			l.i += len(p)
			return
		}
	}
	l.emit(kPunct, rest[:1])
	l.i++
}

// colon reads `::`, a symbol (:name, :"quoted", :+) or a lone colon.
func (l *lexer) colon() {
	s := l.s
	start := l.i
	if strings.HasPrefix(s[l.i:], "::") {
		l.punct()
		return
	}
	if l.i+1 < len(s) {
		switch c := s[l.i+1]; {
		case isIdentStart(c):
			l.i++
			for l.i < len(s) && isIdentChar(s[l.i]) {
				l.i++
			}
			if l.i < len(s) && (s[l.i] == '?' || s[l.i] == '!' || s[l.i] == '=') && (l.i+1 >= len(s) || s[l.i+1] != '=' && s[l.i+1] != '>') {
				l.i++
			}
			l.emit(kSymbol, s[start:l.i])
			return
		case c == '"':
			l.i++
			l.str('"', 0, true, false)
			l.tokens[len(l.tokens)-1].kind = kSymbol
			return
		}
	}
	l.punct()
}

// char reads a char literal: 'a', '\n', '\u{1F600}', '\”. A quote that does not
// start one within a few bytes is punctuation.
func (l *lexer) char() {
	s := l.s
	start := l.i
	j := start + 1
	if j < len(s) && s[j] == '\\' {
		for k := j + 2; k < len(s) && k < start+16 && s[k] != '\n'; k++ {
			if s[k] == '\'' {
				l.i = k + 1
				l.emit(kChar, s[start:l.i])
				return
			}
		}
	} else if j < len(s) && s[j] != '\n' {
		j++
		for j < len(s) && s[j] >= 0x80 && s[j] < 0xC0 {
			j++ // the rest of a multi-byte rune
		}
		if j < len(s) && s[j] == '\'' {
			l.i = j + 1
			l.emit(kChar, s[start:l.i])
			return
		}
	}
	l.punct()
}

// str reads a string from its opening delimiter at l.i to the closing one, open
// being the bracket a %-literal nests (0 for none); a raw string (%q) has no
// escapes.
func (l *lexer) str(closing, open byte, interpolate, raw bool) {
	l.i++
	body := l.i
	has := false
	depth := 0
	s := l.s
	line := l.line
	for l.i < len(s) {
		c := s[l.i]
		switch {
		case c == '\\' && !raw:
			if l.i+1 < len(s) && s[l.i+1] == '\n' {
				l.line++
			}
			l.i += 2
			continue
		case c == '\n':
			l.line++
		case interpolate && c == '#' && l.i+1 < len(s) && s[l.i+1] == '{':
			has = true
			l.i = l.skipCode(l.i+2, 1)
			continue
		case open != 0 && c == open:
			depth++
		case c == closing:
			if depth == 0 {
				text := s[body:l.i]
				l.i++
				l.tokens = append(l.tokens, token{kind: kString, text: text, line: line, first: l.first, space: l.space, interpolate: has})
				l.first, l.space = false, false
				return
			}
			depth--
		}
		l.i++
	}
	if l.i > len(s) {
		l.i = len(s)
	}
	l.tokens = append(l.tokens, token{kind: kString, text: s[body:l.i], line: line, first: l.first, space: l.space, interpolate: has})
	l.first, l.space = false, false
}

// skipCode skips the code of an interpolation from i (after `#{`) to the index
// after its closing brace, reading nested strings so their braces do not count.
func (l *lexer) skipCode(i, nest int) int {
	s := l.s
	depth := 1
	for i < len(s) {
		switch c := s[i]; c {
		case '\n':
			l.line++
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i + 1
			}
		case '"', '`':
			if nest < maxNest {
				i = l.skipString(i+1, c, nest+1)
				continue
			}
		case '\'':
			if i+2 < len(s) && s[i+2] == '\'' {
				i += 3
				continue
			}
		}
		i++
	}
	return i
}

// skipString skips a string inside an interpolation from after its opening quote.
func (l *lexer) skipString(i int, q byte, nest int) int {
	s := l.s
	for i < len(s) {
		switch c := s[i]; {
		case c == '\\':
			i += 2
			continue
		case c == '\n':
			l.line++
		case c == '#' && i+1 < len(s) && s[i+1] == '{':
			i = l.skipCode(i+2, nest)
			continue
		case c == q:
			return i + 1
		}
		i++
	}
	return i
}

// regex reads /.../flags; a slash with no closing one on its line is not a regex.
func (l *lexer) regex() bool {
	s := l.s
	j := l.i + 1
	if j < len(s) && (s[j] == ' ' || s[j] == '=') && l.operand() {
		return false
	}
	for j < len(s) && s[j] != '\n' {
		switch s[j] {
		case '\\':
			j++
		case '/':
			j++
			for j < len(s) && s[j] >= 'a' && s[j] <= 'z' {
				j++
			}
			l.emit(kRegex, s[l.i:j])
			l.i = j
			return true
		}
		j++
	}
	return false
}

// percent reads a %-literal: %(..), %[..], %{..}, %<..>, %|..|, and the q, Q, w,
// i, r and x forms. It reports false for a % that starts none.
func (l *lexer) percent() bool {
	s := l.s
	j := l.i + 1
	interpolate, raw := true, false
	if j < len(s) && strings.IndexByte("qQwirx", s[j]) >= 0 {
		interpolate = s[j] != 'q' && s[j] != 'w' && s[j] != 'i'
		raw = s[j] == 'q' // %q(C:\) has no escapes
		j++
	}
	if j >= len(s) {
		return false
	}
	var open, closing byte
	switch s[j] {
	case '(':
		open, closing = '(', ')'
	case '[':
		open, closing = '[', ']'
	case '{':
		open, closing = '{', '}'
	case '<':
		open, closing = '<', '>'
	case '|':
		closing = '|'
	default:
		return false
	}
	l.i = j
	l.str(closing, open, interpolate, raw)
	return true
}

// macro reads {% ... %} or {{ ... }} as one token, braces and strings inside
// counted so a nested block or a "}}" string does not end it.
func (l *lexer) macro() {
	s := l.s
	start := l.i
	line := l.line
	expand := s[l.i+1] == '{'
	i := l.i + 2
	depth := 0
	for i < len(s) {
		c := s[i]
		if c == '\n' {
			l.line++
		}
		if expand {
			if c == '}' && depth == 0 && i+1 < len(s) && s[i+1] == '}' {
				i += 2
				break
			}
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
			}
		} else if c == '%' && i+1 < len(s) && s[i+1] == '}' {
			i += 2
			break
		}
		if c == '"' {
			i = l.skipString(i+1, '"', 1)
			continue
		}
		i++
	}
	if i > len(s) {
		i = len(s)
	}
	k := kMacro
	if expand {
		k = kExpand
	}
	l.i = i
	l.tokens = append(l.tokens, token{kind: k, text: s[start:i], line: line, first: l.first, space: l.space})
	l.first, l.space = false, false
}

// heredocStart reads <<-ID or <<-'ID'; the body is skipped at the next line.
func (l *lexer) heredocStart() {
	s := l.s
	j := l.i + 3
	interpolate := true
	if s[j] == '\'' {
		interpolate = false
		j++
	}
	idStart := j
	for j < len(s) && isIdentChar(s[j]) {
		j++
	}
	id := s[idStart:j]
	if !interpolate && j < len(s) && s[j] == '\'' {
		j++
	}
	if id == "" {
		l.punct()
		return
	}
	l.pending = append(l.pending, heredoc{id: id, interpolate: interpolate, tok: len(l.tokens)})
	l.emit(kString, "")
	l.i = j
}

// heredocBodies skips the bodies of the pending heredocs, which start at l.i (a
// line start), each up to the line holding only its identifier.
func (l *lexer) heredocBodies() {
	s := l.s
	for _, h := range l.pending {
		body := l.i
		for l.i < len(s) {
			end := strings.IndexByte(s[l.i:], '\n')
			lineEnd := len(s)
			if end >= 0 {
				lineEnd = l.i + end
			}
			line := strings.TrimSpace(s[l.i:lineEnd])
			if line == h.id {
				t := &l.tokens[h.tok]
				t.text = s[body:l.i]
				t.interpolate = h.interpolate && strings.Contains(t.text, "#{")
				l.i = lineEnd
				break
			}
			l.i = lineEnd
			if end >= 0 {
				l.i++
				l.line++
			}
		}
	}
	l.pending = l.pending[:0]
}
