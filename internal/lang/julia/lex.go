package julia

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// Token kinds of the Julia lexer. Keywords are identifiers; the reader tells them
// apart.
const (
	tIdentifier = iota
	tNumber
	tString    // a string literal; text is what is between the quotes
	tCharacter // a character literal
	tCommand   // a command literal (backticks)
	tSymbol    // a quoted symbol :name
	tMacro     // @name; text is the name
	tPunctuation
)

type token struct {
	kind        int
	text        string
	line        int
	start, end  int  // byte offsets in the source
	newline     bool // first token on its line
	spaceBefore bool // whitespace (or a comment) before it
	prefixed    bool // a string with a prefix (raw"", r"", md""): no interpolation
}

// maxNest bounds the recursion of interpolations inside strings inside
// interpolations: a pathological file cannot run the stack out.
const maxNest = 64

type lexer struct {
	source      []byte
	i           int
	line        int
	tokens      []token
	newline     bool
	spaceBefore bool
}

// lex splits Julia source into tokens. It never fails: what it does not know is a
// one-byte punctuation token.
//
// Implements: REQ-JULIA-011
func lex(source []byte) []token {
	l := &lexer{source: source, line: 1, newline: true, tokens: make([]token, 0, len(source)/5+16)}
	if len(source) >= 3 && source[0] == 0xef && source[1] == 0xbb && source[2] == 0xbf {
		l.i = 3
	}
	if strings.HasPrefix(string(source[l.i:]), "#!") { // a script's shebang line
		for l.i < len(source) && source[l.i] != '\n' {
			l.i++
		}
	}
	for l.i < len(l.source) {
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
	case tIdentifier:
		return !keywords[t.text] || t.text == "end" || t.text == "true" || t.text == "false" || t.text == "begin"
	case tNumber, tString, tCharacter, tCommand, tSymbol:
		return true
	case tPunctuation:
		return t.text == ")" || t.text == "]" || t.text == "}" || t.text == "'"
	}
	return false
}

func (l *lexer) emit(kind int, start int, text string) {
	l.tokens = append(l.tokens, token{kind: kind, text: text, line: l.lineAt(start), start: start, end: l.i, newline: l.newline, spaceBefore: l.spaceBefore})
	l.newline, l.spaceBefore = false, false
}

// lineAt is the line of an offset at or after the current line's start; tokens are
// emitted after they are read, so count back the newlines they span.
func (l *lexer) lineAt(start int) int {
	return l.line - bytes.Count(l.source[start:l.i], []byte{'\n'})
}

func (l *lexer) next() {
	c := l.source[l.i]
	switch {
	case c == '\n':
		l.i++
		l.line++
		l.newline, l.spaceBefore = true, true
		return
	case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
		l.i++
		l.spaceBefore = true
		return
	case c == '\\' && l.i+1 < len(l.source) && l.source[l.i+1] == '\n':
		l.i += 2 // not Julia syntax, but harmless
		l.line++
		l.spaceBefore = true
		return
	case c == '#':
		l.comment()
		l.spaceBefore = true
		return
	}
	start := l.i
	switch {
	case c == '"':
		l.readString('"', false, 0)
		l.emit(tString, start, l.inner(start))
	case c == '`':
		l.readString('`', false, 0)
		l.emit(tCommand, start, l.inner(start))
	case c == '\'':
		if l.operand() && !l.spaceBefore {
			l.i++
			l.emit(tPunctuation, start, "'")
		} else if l.character() {
			l.emit(tCharacter, start, string(l.source[start:l.i]))
		} else {
			l.i++
			l.emit(tPunctuation, start, "'")
		}
	case c == '@':
		l.i++
		if l.i < len(l.source) && l.source[l.i] == '.' && !l.identifierStart(l.i+1) { // @. broadcasting
			l.i++
			l.emit(tMacro, start, ".")
			return
		}
		s := l.i
		l.identifier()
		l.emit(tMacro, start, string(l.source[s:l.i]))
	case c >= '0' && c <= '9' || c == '.' && l.i+1 < len(l.source) && l.source[l.i+1] >= '0' && l.source[l.i+1] <= '9' && !l.operand():
		l.number()
		l.emit(tNumber, start, string(l.source[start:l.i]))
	case l.identifierStart(l.i):
		l.identifier()
		text := string(l.source[start:l.i])
		// A string macro: raw"...", r"..."i, md"""...""", b"...". The prefix is
		// part of the literal, which does not interpolate.
		if l.i < len(l.source) && (l.source[l.i] == '"' || l.source[l.i] == '`') && !keywords[text] {
			q := l.source[l.i]
			s := l.i
			l.readString(q, true, 0)
			kind := tString
			if q == '`' {
				kind = tCommand
			}
			l.tokens = append(l.tokens, token{kind: kind, text: l.inner(s), line: l.lineAt(start), start: start, end: l.i, newline: l.newline, spaceBefore: l.spaceBefore, prefixed: true})
			l.newline, l.spaceBefore = false, false
			return
		}
		l.emit(tIdentifier, start, text)
	case c == ':' && !l.operand() && l.i+1 < len(l.source) && l.identifierStart(l.i+1) && !(l.i > 0 && l.source[l.i-1] == ':'):
		l.i++
		l.identifier()
		l.emit(tSymbol, start, string(l.source[start+1:l.i]))
	default:
		l.punctuation()
		l.emit(tPunctuation, start, string(l.source[start:l.i]))
	}
}

// inner is a literal's text without its delimiters (and prefix).
func (l *lexer) inner(start int) string {
	s := string(l.source[start:l.i])
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
	if l.i+1 < len(l.source) && l.source[l.i+1] == '=' {
		depth := 0
		for l.i < len(l.source) {
			switch {
			case l.source[l.i] == '#' && l.i+1 < len(l.source) && l.source[l.i+1] == '=':
				depth++
				l.i += 2
			case l.source[l.i] == '=' && l.i+1 < len(l.source) && l.source[l.i+1] == '#':
				depth--
				l.i += 2
				if depth == 0 {
					return
				}
			default:
				if l.source[l.i] == '\n' {
					l.line++
				}
				l.i++
			}
		}
		return
	}
	for l.i < len(l.source) && l.source[l.i] != '\n' {
		l.i++
	}
}

// readString reads a string or command literal starting at its quote: "..." or """...""".
// A plain literal interpolates $(...), which is code with its own strings; a
// prefixed one (raw"...") does not.
func (l *lexer) readString(q byte, raw bool, depth int) {
	triple := l.i+2 < len(l.source) && l.source[l.i+1] == q && l.source[l.i+2] == q
	if triple {
		l.i += 3
	} else {
		l.i++
	}
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case c == '\\':
			if l.i+1 < len(l.source) && l.source[l.i+1] == '\n' {
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
			if l.i+2 < len(l.source) && l.source[l.i+1] == q && l.source[l.i+2] == q {
				l.i += 3
				return
			}
		case c == '$' && !raw && l.i+1 < len(l.source) && l.source[l.i+1] == '(' && depth < maxNest:
			l.i += 2
			l.interpolate(depth + 1)
			continue
		}
		l.i++
	}
	if l.i > len(l.source) {
		l.i = len(l.source)
	}
}

// interpolate skips an interpolation's code up to its closing parenthesis.
func (l *lexer) interpolate(depth int) {
	n := 1
	for l.i < len(l.source) {
		c := l.source[l.i]
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
			l.readString(c, false, depth)
			continue
		case '\n':
			l.line++
		case '#':
			if l.i+1 < len(l.source) && l.source[l.i+1] == '=' {
				l.comment()
				continue
			}
		case '\'':
			save := l.i
			if l.i > 0 && chars.IsIdentUTF8(l.source[l.i-1]) || !l.character() {
				l.i = save + 1
			}
			continue
		}
		l.i++
	}
}

// character reads a character literal at a quote ('a', '\n', '∀', '\”); false (and
// nothing read) when the quote does not start one.
func (l *lexer) character() bool {
	j := l.i + 1
	if j >= len(l.source) {
		return false
	}
	if l.source[j] == '\\' {
		for k := j + 2; k < len(l.source) && k < j+12; k++ {
			if l.source[k] == '\'' {
				l.i = k + 1
				return true
			}
			if l.source[k] == '\n' {
				return false
			}
		}
		return false
	}
	_, n := utf8.DecodeRune(l.source[j:])
	if j+n < len(l.source) && l.source[j+n] == '\'' && l.source[j] != '\n' {
		l.i = j + n + 1
		return true
	}
	return false
}

// operatorRunes are the non-ASCII operators common in Julia code; every other
// non-ASCII letter, mark or symbol may be part of a name (∇f, x̄, α₁).
var operatorRunes = map[rune]bool{
	'∈': true, '∉': true, '∋': true, '≤': true, '≥': true, '≠': true, '≈': true, '≡': true, '≢': true,
	'→': true, '←': true, '↦': true, '⊆': true, '⊇': true, '⊂': true, '⊃': true, '∘': true, '×': true,
	'÷': true, '⋅': true, '∩': true, '∪': true, '√': true, '∛': true, '⊗': true, '⊕': true, '⊻': true,
	'∧': true, '∨': true, '¬': true, '⟹': true, '⇒': true, '∀': true, '∃': true, '±': true, '∓': true,
}

func (l *lexer) identifierStart(i int) bool {
	if i >= len(l.source) {
		return false
	}
	c := l.source[i]
	if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return true
	}
	if c < 0x80 {
		return false
	}
	r, _ := utf8.DecodeRune(l.source[i:])
	return r != utf8.RuneError && !operatorRunes[r] && (unicode.IsLetter(r) || unicode.IsSymbol(r) || unicode.IsNumber(r))
}

// identifier reads a name: letters, digits, _, non-ASCII name characters and ! (unless
// it starts !=).
func (l *lexer) identifier() {
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9':
			l.i++
		case c == '!':
			if l.i+1 < len(l.source) && l.source[l.i+1] == '=' {
				return
			}
			l.i++
		case c >= 0x80:
			r, n := utf8.DecodeRune(l.source[l.i:])
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
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case c >= '0' && c <= '9' || c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			l.i++
		case c == '.':
			// 1.5 but not 1..2, 1.+x or x[1].y
			if l.i+1 < len(l.source) && (l.source[l.i+1] >= '0' && l.source[l.i+1] <= '9' || l.source[l.i+1] == 'e') {
				l.i++
			} else if l.i+1 >= len(l.source) || !strings.ContainsRune(".+-*/^=<>!&|%\\:)", rune(l.source[l.i+1])) && !chars.IsIdentUTF8(l.source[l.i+1]) {
				l.i++
			} else {
				return
			}
		case (c == '+' || c == '-') && l.i > 0 && (l.source[l.i-1] == 'e' || l.source[l.i-1] == 'E' || l.source[l.i-1] == 'p' || l.source[l.i-1] == 'f') &&
			l.i+1 < len(l.source) && l.source[l.i+1] >= '0' && l.source[l.i+1] <= '9':
			l.i++
		default:
			return
		}
	}
}

// punctuations are the multi-character operators the reader needs told apart, longest
// first.
var punctuations = []string{"...", "===", "!==", "..", "::", "==", "!=", "<=", ">=", "->", "=>", "<:", ">:",
	"+=", "-=", "*=", "/=", "^=", "|=", "&=", "%=", "&&", "||", "|>", "<|", ":=", ">>", "<<"}

func (l *lexer) punctuation() {
	rest := l.source[l.i:]
	// A dotted (broadcast) operator is one token: .=, .+, .==; `.` alone is access.
	if rest[0] == '.' && len(rest) > 1 && strings.ContainsRune("=+-*/^<>!&|%\\", rune(rest[1])) {
		l.i++
		for _, p := range punctuations {
			if strings.HasPrefix(string(l.source[l.i:min(l.i+3, len(l.source))]), p) {
				l.i += len(p)
				return
			}
		}
		l.i++
		return
	}
	for _, p := range punctuations {
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
