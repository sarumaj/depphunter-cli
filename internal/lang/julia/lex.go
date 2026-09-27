package julia

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token kinds of the Julia lexer. Keywords are identifiers; the reader tells them
// apart.
const (
	tIdent = iota
	tNum
	tStr   // a string literal; text is what is between the quotes
	tChar  // a character literal
	tCmd   // a command literal (backticks)
	tSym   // a quoted symbol :name
	tMacro // @name; text is the name
	tPunct
)

type tok struct {
	kind       int
	text       string
	line       int
	start, end int  // byte offsets in the source
	nl         bool // first token on its line
	sp         bool // whitespace (or a comment) before it
	prefixed   bool // a string with a prefix (raw"", r"", md""): no interpolation
}

// maxNest bounds the recursion of interpolations inside strings inside
// interpolations: a pathological file cannot run the stack out.
const maxNest = 64

type lexer struct {
	src    []byte
	i      int
	line   int
	tokens []tok
	nl     bool
	sp     bool
}

// lex splits Julia source into tokens. It never fails: what it does not know is a
// one-byte punctuation token.
//
// Implements: REQ-JULIA-011
func lex(src []byte) []tok {
	l := &lexer{src: src, line: 1, nl: true, tokens: make([]tok, 0, len(src)/5+16)}
	if len(src) >= 3 && src[0] == 0xef && src[1] == 0xbb && src[2] == 0xbf {
		l.i = 3
	}
	if strings.HasPrefix(string(src[l.i:]), "#!") { // a script's shebang line
		for l.i < len(src) && src[l.i] != '\n' {
			l.i++
		}
	}
	for l.i < len(l.src) {
		l.next()
	}
	return l.tokens
}

// operand reports whether the last token ends an operand: after one, ' is the
// adjoint operator and : is a range or ternary colon, not a quote.
func (l *lexer) operand() bool {
	if len(l.tokens) == 0 {
		return false
	}
	t := l.tokens[len(l.tokens)-1]
	switch t.kind {
	case tIdent:
		return !keywords[t.text] || t.text == "end" || t.text == "true" || t.text == "false" || t.text == "begin"
	case tNum, tStr, tChar, tCmd, tSym:
		return true
	case tPunct:
		return t.text == ")" || t.text == "]" || t.text == "}" || t.text == "'"
	}
	return false
}

func (l *lexer) emit(kind int, start int, text string) {
	l.tokens = append(l.tokens, tok{kind: kind, text: text, line: l.lineAt(start), start: start, end: l.i, nl: l.nl, sp: l.sp})
	l.nl, l.sp = false, false
}

// lineAt is the line of an offset at or after the current line's start; tokens are
// emitted after they are read, so count back the newlines they span.
func (l *lexer) lineAt(start int) int {
	return l.line - bytes.Count(l.src[start:l.i], []byte{'\n'})
}

func (l *lexer) next() {
	c := l.src[l.i]
	switch {
	case c == '\n':
		l.i++
		l.line++
		l.nl, l.sp = true, true
		return
	case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
		l.i++
		l.sp = true
		return
	case c == '\\' && l.i+1 < len(l.src) && l.src[l.i+1] == '\n':
		l.i += 2 // not Julia syntax, but harmless
		l.line++
		l.sp = true
		return
	case c == '#':
		l.comment()
		l.sp = true
		return
	}
	start := l.i
	switch {
	case c == '"':
		l.str('"', false, 0)
		l.emit(tStr, start, l.inner(start))
	case c == '`':
		l.str('`', false, 0)
		l.emit(tCmd, start, l.inner(start))
	case c == '\'':
		if l.operand() && !l.sp {
			l.i++
			l.emit(tPunct, start, "'")
		} else if l.char() {
			l.emit(tChar, start, string(l.src[start:l.i]))
		} else {
			l.i++
			l.emit(tPunct, start, "'")
		}
	case c == '@':
		l.i++
		if l.i < len(l.src) && l.src[l.i] == '.' && !l.identStart(l.i+1) { // @. broadcasting
			l.i++
			l.emit(tMacro, start, ".")
			return
		}
		s := l.i
		l.ident()
		l.emit(tMacro, start, string(l.src[s:l.i]))
	case c >= '0' && c <= '9' || c == '.' && l.i+1 < len(l.src) && l.src[l.i+1] >= '0' && l.src[l.i+1] <= '9' && !l.operand():
		l.number()
		l.emit(tNum, start, string(l.src[start:l.i]))
	case l.identStart(l.i):
		l.ident()
		text := string(l.src[start:l.i])
		// A string macro: raw"...", r"..."i, md"""...""", b"...". The prefix is
		// part of the literal, which does not interpolate.
		if l.i < len(l.src) && (l.src[l.i] == '"' || l.src[l.i] == '`') && !keywords[text] {
			q := l.src[l.i]
			s := l.i
			l.str(q, true, 0)
			kind := tStr
			if q == '`' {
				kind = tCmd
			}
			l.tokens = append(l.tokens, tok{kind: kind, text: l.inner(s), line: l.lineAt(start), start: start, end: l.i, nl: l.nl, sp: l.sp, prefixed: true})
			l.nl, l.sp = false, false
			return
		}
		l.emit(tIdent, start, text)
	case c == ':' && !l.operand() && l.i+1 < len(l.src) && l.identStart(l.i+1) && !(l.i > 0 && l.src[l.i-1] == ':'):
		l.i++
		l.ident()
		l.emit(tSym, start, string(l.src[start+1:l.i]))
	default:
		l.punct()
		l.emit(tPunct, start, string(l.src[start:l.i]))
	}
}

// inner is a literal's text without its delimiters (and prefix).
func (l *lexer) inner(start int) string {
	s := string(l.src[start:l.i])
	q := s[:1]
	if strings.HasPrefix(s, q+q+q) && len(s) >= 6 {
		return strings.TrimSuffix(s[3:], q+q+q)
	}
	if len(s) >= 2 && strings.HasSuffix(s, q) {
		return s[1 : len(s)-1]
	}
	return s[1:]
}

// comment skips # to the end of the line or a #= =# block, which nests.
func (l *lexer) comment() {
	if l.i+1 < len(l.src) && l.src[l.i+1] == '=' {
		depth := 0
		for l.i < len(l.src) {
			switch {
			case l.src[l.i] == '#' && l.i+1 < len(l.src) && l.src[l.i+1] == '=':
				depth++
				l.i += 2
			case l.src[l.i] == '=' && l.i+1 < len(l.src) && l.src[l.i+1] == '#':
				depth--
				l.i += 2
				if depth == 0 {
					return
				}
			default:
				if l.src[l.i] == '\n' {
					l.line++
				}
				l.i++
			}
		}
		return
	}
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		l.i++
	}
}

// str reads a string or command literal starting at its quote: "..." or """...""".
// A plain literal interpolates $(...), which is code with its own strings; a
// prefixed one (raw"...") does not.
func (l *lexer) str(q byte, raw bool, depth int) {
	triple := l.i+2 < len(l.src) && l.src[l.i+1] == q && l.src[l.i+2] == q
	if triple {
		l.i += 3
	} else {
		l.i++
	}
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '\\':
			if l.i+1 < len(l.src) && l.src[l.i+1] == '\n' {
				l.line++
			}
			l.i += 2
			continue
		case c == '\n':
			l.line++
		case c == q:
			if !triple {
				l.i++
				return
			}
			if l.i+2 < len(l.src) && l.src[l.i+1] == q && l.src[l.i+2] == q {
				l.i += 3
				return
			}
		case c == '$' && !raw && l.i+1 < len(l.src) && l.src[l.i+1] == '(' && depth < maxNest:
			l.i += 2
			l.interpolate(depth + 1)
			continue
		}
		l.i++
	}
	if l.i > len(l.src) {
		l.i = len(l.src)
	}
}

// interpolate skips an interpolation's code up to its closing parenthesis.
func (l *lexer) interpolate(depth int) {
	n := 1
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch c {
		case '(':
			n++
		case ')':
			n--
			if n == 0 {
				l.i++
				return
			}
		case '"', '`':
			l.str(c, false, depth)
			continue
		case '\n':
			l.line++
		case '#':
			if l.i+1 < len(l.src) && l.src[l.i+1] == '=' {
				l.comment()
				continue
			}
		case '\'':
			save := l.i
			if l.i > 0 && isIdentByte(l.src[l.i-1]) || !l.char() {
				l.i = save + 1
			}
			continue
		}
		l.i++
	}
}

// char reads a character literal at a quote ('a', '\n', '∀', '\”); false (and
// nothing read) when the quote does not start one.
func (l *lexer) char() bool {
	j := l.i + 1
	if j >= len(l.src) {
		return false
	}
	if l.src[j] == '\\' {
		for k := j + 2; k < len(l.src) && k < j+12; k++ {
			if l.src[k] == '\'' {
				l.i = k + 1
				return true
			}
			if l.src[k] == '\n' {
				return false
			}
		}
		return false
	}
	_, n := utf8.DecodeRune(l.src[j:])
	if j+n < len(l.src) && l.src[j+n] == '\'' && l.src[j] != '\n' {
		l.i = j + n + 1
		return true
	}
	return false
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

// operatorRunes are the non-ASCII operators common in Julia code; every other
// non-ASCII letter, mark or symbol may be part of a name (∇f, x̄, α₁).
var operatorRunes = map[rune]bool{
	'∈': true, '∉': true, '∋': true, '≤': true, '≥': true, '≠': true, '≈': true, '≡': true, '≢': true,
	'→': true, '←': true, '↦': true, '⊆': true, '⊇': true, '⊂': true, '⊃': true, '∘': true, '×': true,
	'÷': true, '⋅': true, '∩': true, '∪': true, '√': true, '∛': true, '⊗': true, '⊕': true, '⊻': true,
	'∧': true, '∨': true, '¬': true, '⟹': true, '⇒': true, '∀': true, '∃': true, '±': true, '∓': true,
}

func (l *lexer) identStart(i int) bool {
	if i >= len(l.src) {
		return false
	}
	c := l.src[i]
	if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return true
	}
	if c < 0x80 {
		return false
	}
	r, _ := utf8.DecodeRune(l.src[i:])
	return r != utf8.RuneError && !operatorRunes[r] && (unicode.IsLetter(r) || unicode.IsSymbol(r) || unicode.IsNumber(r))
}

// ident reads a name: letters, digits, _, non-ASCII name characters and ! (unless
// it starts !=).
func (l *lexer) ident() {
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9':
			l.i++
		case c == '!':
			if l.i+1 < len(l.src) && l.src[l.i+1] == '=' {
				return
			}
			l.i++
		case c >= 0x80:
			r, n := utf8.DecodeRune(l.src[l.i:])
			if r == utf8.RuneError || operatorRunes[r] || !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) ||
				unicode.IsSymbol(r) || unicode.IsNumber(r) || unicode.Is(unicode.Pc, r) || r == '′' || r == '″') {
				return
			}
			l.i += n
		default:
			return
		}
	}
}

// number reads a numeric literal: 1, 1_000, 0x1F, 1.5e-3, 2im, .5.
func (l *lexer) number() {
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c >= '0' && c <= '9' || c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			l.i++
		case c == '.':
			// 1.5 but not 1..2, 1.+x or x[1].y
			if l.i+1 < len(l.src) && (l.src[l.i+1] >= '0' && l.src[l.i+1] <= '9' || l.src[l.i+1] == 'e') {
				l.i++
			} else if l.i+1 >= len(l.src) || !strings.ContainsRune(".+-*/^=<>!&|%\\:)", rune(l.src[l.i+1])) && !isIdentByte(l.src[l.i+1]) {
				l.i++
			} else {
				return
			}
		case (c == '+' || c == '-') && l.i > 0 && (l.src[l.i-1] == 'e' || l.src[l.i-1] == 'E' || l.src[l.i-1] == 'p' || l.src[l.i-1] == 'f') &&
			l.i+1 < len(l.src) && l.src[l.i+1] >= '0' && l.src[l.i+1] <= '9':
			l.i++
		default:
			return
		}
	}
}

// puncts are the multi-character operators the reader needs told apart, longest
// first.
var puncts = []string{"...", "===", "!==", "..", "::", "==", "!=", "<=", ">=", "->", "=>", "<:", ">:",
	"+=", "-=", "*=", "/=", "^=", "|=", "&=", "%=", "&&", "||", "|>", "<|", ":=", ">>", "<<"}

func (l *lexer) punct() {
	rest := l.src[l.i:]
	// A dotted (broadcast) operator is one token: .=, .+, .==; `.` alone is access.
	if rest[0] == '.' && len(rest) > 1 && strings.ContainsRune("=+-*/^<>!&|%\\", rune(rest[1])) {
		l.i++
		for _, p := range puncts {
			if strings.HasPrefix(string(l.src[l.i:min(l.i+3, len(l.src))]), p) {
				l.i += len(p)
				return
			}
		}
		l.i++
		return
	}
	for _, p := range puncts {
		if len(rest) >= len(p) && string(rest[:len(p)]) == p {
			l.i += len(p)
			return
		}
	}
	if rest[0] >= 0x80 {
		_, n := utf8.DecodeRune(rest)
		l.i += max(n, 1)
		return
	}
	l.i++
}

// keywords are Julia's reserved words and the contextual ones the reader acts on.
var keywords = map[string]bool{
	"baremodule": true, "begin": true, "break": true, "catch": true, "const": true, "continue": true,
	"do": true, "else": true, "elseif": true, "end": true, "export": true, "false": true, "finally": true,
	"for": true, "function": true, "global": true, "if": true, "import": true, "let": true, "local": true,
	"macro": true, "module": true, "quote": true, "return": true, "struct": true, "true": true, "try": true,
	"using": true, "while": true, "where": true, "in": true, "isa": true,
}
