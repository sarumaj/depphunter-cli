package haskell

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token kinds of the Haskell lexer.
const (
	tVar    = iota + 1 // a variable identifier or keyword, possibly qualified (M.lookup)
	tCon               // a constructor or module identifier, possibly qualified (Data.Map)
	tOp                // an operator or reserved operator: = | :: -> <- => @ ~ \ ..
	tPunct             // ( ) [ ] , ; { } `
	tStr               // a string literal's content
	tChar              // a character literal
	tNum               // a numeric literal
	tPragma            // a {-# ... #-} pragma, its words joined by single spaces
)

type tok struct {
	k    int
	s    string
	line int
	col  int  // 1-based, tabs to the next multiple of 8
	bol  bool // first token of its line
	end  int  // column just past the token, when it ends on its own line
}

// lexer reads Haskell source into tokens, without applying the layout rule: the
// extraction needs only which tokens start a line at which column. It knows what could
// otherwise hide or fake a token: nested {- -} comments, {-# #-} pragmas, -- comments
// (but not operators such as --> that begin with dashes), strings with escapes and
// gaps, character literals versus the quote of a primed name (foldl'), a promoted
// constructor ('True) or a Template Haskell name quote (”Maybe), C preprocessor
// lines, and the UnicodeSyntax arrows.
type lexer struct {
	src  []byte
	i    int
	line int
	col  int
	bol  bool
	toks []tok
}

func lex(src []byte) []tok {
	l := &lexer{src: src, line: 1, col: 1, bol: true}
	l.run()
	return l.toks
}

func isSymbol(r rune) bool {
	switch r {
	case '!', '#', '$', '%', '&', '*', '+', '.', '/', '<', '=', '>', '?', '@', '\\', '^', '|', '-', '~', ':':
		return true
	}
	if r < 0x80 {
		return false
	}
	return unicode.IsSymbol(r) || unicode.IsPunct(r) && r != '_' && r != '"' && r != '\''
}

func identStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func identPart(r rune) bool {
	return r == '_' || r == '\'' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (l *lexer) rune(off int) (rune, int) {
	if l.i+off >= len(l.src) {
		return 0, 0
	}
	return utf8.DecodeRune(l.src[l.i+off:])
}

func (l *lexer) peek(off int) byte {
	if l.i+off < len(l.src) {
		return l.src[l.i+off]
	}
	return 0
}

// advance moves over n bytes, keeping line and column.
func (l *lexer) advance(n int) {
	for k := 0; k < n && l.i < len(l.src); k++ {
		c := l.src[l.i]
		l.i++
		switch {
		case c == '\n':
			l.line++
			l.col = 1
			l.bol = true
		case c == '\t':
			l.col = (l.col-1)/8*8 + 9
		case c < 0x80 || c >= 0xC0: // not a UTF-8 continuation byte
			l.col++
		}
	}
}

func (l *lexer) emit(k int, s string, line, col int) {
	l.toks = append(l.toks, tok{k: k, s: s, line: line, col: col, bol: l.bol, end: l.col})
	l.bol = false
}

func (l *lexer) run() {
	if strings.HasPrefix(string(l.src), "\ufeff") {
		l.i = 3
	}
	if l.peek(0) == '#' && l.peek(1) == '!' {
		l.skipLine()
	}
	for l.i < len(l.src) {
		c := l.src[l.i]
		line, col := l.line, l.col
		switch {
		case c == '\n' || c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.advance(1)
		case c == '#' && col == 1 && l.directive():
			l.skipLine() // CPP (#if, #include) and hsc2hs (#{enum ...}) directives
		case c == '{' && l.peek(1) == '-' && l.peek(2) == '#':
			start := l.i
			l.blockComment()
			body := string(l.src[start:l.i])
			body = strings.TrimSuffix(strings.TrimPrefix(body, "{-#"), "#-}")
			l.emit(tPragma, strings.Join(strings.Fields(body), " "), line, col)
		case c == '{' && l.peek(1) == '-':
			l.blockComment()
		case c == '-' && l.lineComment():
			l.skipLine()
		case c == '"':
			l.str(line, col)
		case c == '\'':
			l.quote(line, col)
		case c >= '0' && c <= '9':
			l.number(line, col)
		case strings.IndexByte("()[],;{}`", c) >= 0:
			l.advance(1)
			l.emit(tPunct, string(c), line, col)
		default:
			r, n := l.rune(0)
			switch {
			case identStart(r):
				l.ident(line, col)
			case isSymbol(r):
				start := l.i
				for {
					r, n := l.rune(0)
					if n == 0 || !isSymbol(r) {
						break
					}
					l.advance(n)
				}
				l.emit(tOp, unicodeOp(string(l.src[start:l.i])), line, col)
			default:
				l.advance(max(n, 1))
			}
		}
	}
}

// unicodeOp spells the UnicodeSyntax forms of the reserved operators in ASCII.
func unicodeOp(s string) string {
	switch s {
	case "∷":
		return "::"
	case "→":
		return "->"
	case "←":
		return "<-"
	case "⇒":
		return "=>"
	case "∀":
		return "forall"
	}
	return s
}

// directive reports whether a # in the first column starts a preprocessor line: a
// letter after it (#if, #include, #define) or an hsc2hs #{...}.
func (l *lexer) directive() bool {
	j := l.i + 1
	for j < len(l.src) && (l.src[j] == ' ' || l.src[j] == '\t') {
		j++
	}
	if j >= len(l.src) {
		return true
	}
	c := l.src[j]
	return c >= 'a' && c <= 'z' || c == '{'
}

// skipLine moves to the end of the line, following backslash continuations (a
// multi-line #define).
func (l *lexer) skipLine() {
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		if l.src[l.i] == '\\' && l.peek(1) == '\n' {
			l.advance(2)
			continue
		}
		l.advance(1)
	}
}

// lineComment reports whether the dashes at l.i open a comment: two or more dashes
// not followed by another symbol character (--> is an operator).
func (l *lexer) lineComment() bool {
	j := l.i
	for j < len(l.src) && l.src[j] == '-' {
		j++
	}
	if j-l.i < 2 {
		return false
	}
	if j >= len(l.src) {
		return true
	}
	r, _ := utf8.DecodeRune(l.src[j:])
	return !isSymbol(r)
}

// blockComment skips a {- -} comment, nested ones included.
func (l *lexer) blockComment() {
	depth := 0
	for l.i < len(l.src) {
		switch {
		case l.src[l.i] == '{' && l.peek(1) == '-':
			depth++
			l.advance(2)
		case l.src[l.i] == '-' && l.peek(1) == '}':
			depth--
			l.advance(2)
			if depth == 0 {
				return
			}
		default:
			l.advance(1)
		}
	}
}

// str reads a string literal with its escapes (\" and \\) and gaps (a backslash,
// whitespace up to a line break, and a backslash).
func (l *lexer) str(line, col int) {
	l.advance(1)
	var b strings.Builder
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '"':
			l.advance(1)
			l.emit(tStr, b.String(), line, col)
			return
		case c == '\\':
			next := l.peek(1)
			if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
				l.advance(1)
				for l.i < len(l.src) && l.src[l.i] != '\\' {
					l.advance(1)
				}
				l.advance(1)
				continue
			}
			b.WriteByte(c)
			b.WriteByte(next)
			l.advance(2)
		case c == '\n':
			l.emit(tStr, b.String(), line, col) // unterminated: stop at the line's end
			return
		default:
			b.WriteByte(c)
			l.advance(1)
		}
	}
	l.emit(tStr, b.String(), line, col)
}

// quote reads what starts with a quote outside an identifier: a character literal
// ('a', '\n', '\”, 'λ'), or else a promotion or Template Haskell name quote, which
// is emitted as an operator and leaves the name after it to be read as usual.
func (l *lexer) quote(line, col int) {
	if l.peek(1) == '\\' {
		j := l.i + 2
		if j < len(l.src) {
			j++ // the escaped character itself, which may be a quote
		}
		for j < len(l.src) && l.src[j] != '\'' && l.src[j] != '\n' && j-l.i < 12 {
			j++
		}
		if j < len(l.src) && l.src[j] == '\'' {
			s := string(l.src[l.i : j+1])
			l.advance(j + 1 - l.i)
			l.emit(tChar, s, line, col)
			return
		}
	}
	if _, n := l.rune(1); n > 0 && l.peek(1) != '\'' && l.peek(1) != '\n' && l.peek(1+n) == '\'' {
		s := string(l.src[l.i : l.i+n+2])
		l.advance(n + 2)
		l.emit(tChar, s, line, col)
		return
	}
	n := 1
	if l.peek(1) == '\'' {
		n = 2 // ''Type
	}
	l.advance(n)
	l.emit(tOp, strings.Repeat("'", n), line, col)
}

func (l *lexer) number(line, col int) {
	start := l.i
	digits := func(ok func(byte) bool) {
		for l.i < len(l.src) && (ok(l.src[l.i]) || l.src[l.i] == '_') {
			l.advance(1)
		}
	}
	dec := func(c byte) bool { return c >= '0' && c <= '9' }
	hex := func(c byte) bool { return dec(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
	if l.peek(0) == '0' && strings.IndexByte("xXoObB", l.peek(1)) >= 0 {
		l.advance(2)
		digits(hex)
	} else {
		digits(dec)
		if l.peek(0) == '.' && dec(l.peek(1)) {
			l.advance(1)
			digits(dec)
		}
		if e := l.peek(0); e == 'e' || e == 'E' {
			off := 1
			if s := l.peek(1); s == '+' || s == '-' {
				off = 2
			}
			if dec(l.peek(off)) {
				l.advance(off)
				digits(dec)
			}
		}
	}
	l.emit(tNum, string(l.src[start:l.i]), line, col)
}

// ident reads a name, qualified by module names when it is one: Data.Map.lookup,
// M.Map, Data.Map. A qualified operator (M.!) is left as a name and an operator.
func (l *lexer) ident(line, col int) {
	start := l.i
	word := func() rune {
		first, _ := l.rune(0)
		for {
			r, n := l.rune(0)
			if n == 0 || !identPart(r) {
				return first
			}
			l.advance(n)
		}
	}
	first := word()
	// A segment starting upper case is a module name when a dot and a name follow.
	for unicode.IsUpper(first) && l.peek(0) == '.' {
		if r, n := l.rune(1); n == 0 || !identStart(r) {
			break
		}
		l.advance(1)
		first = word()
	}
	k := tVar
	if unicode.IsUpper(first) {
		k = tCon
	}
	l.emit(k, string(l.src[start:l.i]), line, col)
}

// unlit turns literate Haskell into Haskell with the same lines: the code of
// \begin{code} ... \end{code} blocks and of bird-track lines (> code) is kept, and
// every other line is emptied.
func unlit(src []byte) []byte {
	lines := strings.Split(string(src), "\n")
	inCode := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, `\begin{code}`):
			inCode = true
			lines[i] = ""
		case strings.HasPrefix(trimmed, `\end{code}`):
			inCode = false
			lines[i] = ""
		case inCode:
		case strings.HasPrefix(line, ">"):
			// "> code": the track and the space after it go, so bird-track code
			// starts in the first column like any other.
			lines[i] = strings.TrimPrefix(line[1:], " ")
		default:
			lines[i] = ""
		}
	}
	return []byte(strings.Join(lines, "\n"))
}
