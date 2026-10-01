package fsharp

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// Token kinds.
const (
	tIdentifier  = 'i' // identifier or keyword; ``quoted`` identifiers without the backticks
	tTyVariable  = 'v' // type variable: 'a, ^T
	tString      = 's' // string literal (text is the decoded value for plain and verbatim strings)
	tCharacter   = 'c'
	tNumber      = 'n'
	tPunctuation = 'p' // one operator character, or [< >] ..
	tDirectory   = 'd' // #load, #r, #I: text is the name and each argument, NUL-separated
)

type token struct {
	kind   byte
	text   string
	line   int // 1-based
	column int // 0-based, in bytes from the line start
	first  bool
	// glued says no space separates the token from the one before it, which tells
	// `A.b` (a qualified name) from `a . b` and `x.Y` member access.
	glued bool
}

// maxNest bounds nested comments and interpolation holes.
const maxNest = 200

type lexer struct {
	s               string
	i               int
	line            int
	lineStart       int
	tokens          []token
	lastLine        int // line of the last token, for token.first
	lastEnd         int // offset just past the last token
	beginningOfLine bool
}

// lex splits F# source into tokens. Comments are dropped: `//` to the end of the
// line, `(* *)` nested and lexing the strings inside (so `(* "*)" *)` is one
// comment), but `(*)` is the multiplication operator. Strings of every form are one
// token: "..." with escapes, @"..." verbatim, """...""", interpolated $"..{x}.."
// (holes skipped with the strings and braces inside them), $@ and @$, byte strings
// (a trailing B). A quote is a char literal only when a character (or an escape) and
// a closing quote follow; else it starts a type variable ('a) or belongs to an
// identifier (x'). Preprocessor lines (#if, #else, #endif, #nowarn, #light, #line)
// are dropped, so the code of both branches is read; #load, #r, #I, #reference and
// #include become directive tokens with their string arguments.
//
// Implements: REQ-FSHARP-002, REQ-FSHARP-011
func lex(source string) []token {
	l := &lexer{s: strings.TrimPrefix(source, "\xef\xbb\xbf"), line: 1, lastLine: 0, beginningOfLine: true}
	l.tokens = make([]token, 0, len(l.s)/4+16)
	if strings.HasPrefix(l.s, "#!") { // a script's shebang line
		for l.i < len(l.s) && l.s[l.i] != '\n' {
			l.i++
		}
	}
	l.run()
	return l.tokens
}

func (l *lexer) emit(kind byte, text string, start int) {
	column := start - l.lineStart
	if column < 0 {
		column = 0
	}
	l.tokens = append(l.tokens, token{kind: kind, text: text, line: l.line, column: column, first: l.lastLine != l.line, glued: start == l.lastEnd && len(l.tokens) > 0})
	l.lastLine, l.lastEnd = l.line, l.i
}

// emitAt is emit for a token that began on an earlier line (a multi-line string).
func (l *lexer) emitAt(kind byte, text string, line, column int) {
	l.tokens = append(l.tokens, token{kind: kind, text: text, line: line, column: column, first: l.lastLine != line})
	l.lastLine, l.lastEnd = l.line, l.i
}

func (l *lexer) peek(k int) byte {
	if l.i+k < len(l.s) {
		return l.s[l.i+k]
	}
	return 0
}

func (l *lexer) newline() {
	l.line++
	l.lineStart = l.i + 1
}

func (l *lexer) run() {
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case c == '\n':
			l.newline()
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			l.i++
		case c == '/' && l.peek(1) == '/':
			for l.i < len(l.s) && l.s[l.i] != '\n' {
				l.i++
			}
		case c == '(' && l.peek(1) == '*' && l.peek(2) != ')':
			l.comment()
		case c == '#' && l.atLineStart():
			l.directive()
		case c == '"' || (c == '@' || c == '$') && l.stringAhead():
			l.stringToken()
		case c == '\'':
			l.quote()
		case c == '`' && l.peek(1) == '`':
			start := l.i
			// A ``quoted name`` ends on its line; look no further than that.
			rest := l.s[l.i+2 : min(len(l.s), l.i+2+512)]
			if newline := strings.IndexByte(rest, '\n'); newline >= 0 {
				rest = rest[:newline]
			}
			end := strings.Index(rest, "``")
			if end < 0 {
				l.i += 2
				l.emit(tPunctuation, "`", start)
				continue
			}
			l.i += 2 + end + 2
			l.emit(tIdentifier, l.s[start+2:start+2+end], start)
		case chars.IsIdentStart(c):
			start := l.i
			l.identifier()
			l.emit(tIdentifier, l.s[start:l.i], start)
		case c >= '0' && c <= '9':
			start := l.i
			l.number(start)
			l.emit(tNumber, l.s[start:l.i], start)
		case c >= 0x80:
			r, n := utf8.DecodeRuneInString(l.s[l.i:])
			start := l.i
			if unicode.IsLetter(r) {
				l.identifier()
				l.emit(tIdentifier, l.s[start:l.i], start)
				continue
			}
			l.i += max(n, 1)
			l.emit(tPunctuation, l.s[start:l.i], start)
		case c == '[' && l.peek(1) == '<':
			start := l.i
			l.i += 2
			l.emit(tPunctuation, "[<", start)
		case c == '>' && l.peek(1) == ']':
			start := l.i
			l.i += 2
			l.emit(tPunctuation, ">]", start)
		case c == '.' && l.peek(1) == '.':
			start := l.i
			l.i += 2
			l.emit(tPunctuation, "..", start)
		case c == '^' && chars.IsIdentStart(l.peek(1)) && l.i > 0 && !isIdentifierByte(l.s[l.i-1]) && l.s[l.i-1] != ')':
			// ^T, a statically resolved type parameter.
			start := l.i
			l.i++
			l.identifier()
			l.emit(tTyVariable, l.s[start:l.i], start)
		default:
			start := l.i
			l.i++
			l.emit(tPunctuation, l.s[start:l.i], start)
		}
	}
}

func isIdentifierByte(c byte) bool {
	return chars.IsIdentStart(c) || c >= '0' && c <= '9' || c == '\''
}

func (l *lexer) identifier() {
	for l.i < len(l.s) {
		c := l.s[l.i]
		if isIdentifierByte(c) {
			l.i++
			continue
		}
		if c >= 0x80 {
			r, n := utf8.DecodeRuneInString(l.s[l.i:])
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				l.i += n
				continue
			}
		}
		break
	}
}

func (l *lexer) number(start int) {
	hex := strings.HasPrefix(l.s[start:], "0x") || strings.HasPrefix(l.s[start:], "0X")
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case isIdentifierByte(c) && c != '\'':
			l.i++
		case c == '.' && l.peek(1) != '.' && l.peek(1) >= '0' && l.peek(1) <= '9':
			l.i++
		case (c == '-' || c == '+') && (l.s[l.i-1] == 'e' || l.s[l.i-1] == 'E') && !hex:
			l.i++
		default:
			return
		}
	}
}

// atLineStart reports whether only blanks precede position i on its line.
func (l *lexer) atLineStart() bool {
	for j := l.i - 1; j >= l.lineStart && j >= 0; j-- {
		if l.s[j] != ' ' && l.s[j] != '\t' {
			return false
		}
	}
	return true
}

// comment skips a (* *) comment, nested, lexing the strings inside it.
func (l *lexer) comment() {
	depth := 0
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case c == '\n':
			l.newline()
			l.i++
		case c == '(' && l.peek(1) == '*' && l.peek(2) != ')':
			depth++
			l.i += 2
		case c == '*' && l.peek(1) == ')':
			depth--
			l.i += 2
			if depth == 0 {
				return
			}
		case c == '"':
			l.skipString(false, false, 0)
		case c == '\'' && characterLength(l.s[l.i:]) > 0 && (l.i == 0 || !isIdentifierByte(l.s[l.i-1])):
			l.i += characterLength(l.s[l.i:])
		default:
			l.i++
		}
	}
}

// directive reads a line starting with #: the script directives become tokens,
// every other one (conditional compilation, #nowarn, #light) is dropped.
func (l *lexer) directive() {
	start := l.i
	l.i++
	for l.i < len(l.s) && (l.s[l.i] == ' ' || l.s[l.i] == '\t') {
		l.i++
	}
	whitespaceStart := l.i
	for l.i < len(l.s) && isIdentifierByte(l.s[l.i]) {
		l.i++
	}
	name := l.s[whitespaceStart:l.i]
	switch name {
	case "load", "r", "I", "reference", "include":
	default:
		for l.i < len(l.s) && l.s[l.i] != '\n' {
			l.i++
		}
		return
	}
	line, column := l.line, start-l.lineStart
	var arguments []string
	for l.i < len(l.s) && l.s[l.i] != '\n' {
		c := l.s[l.i]
		switch {
		case c == '/' && l.peek(1) == '/':
			for l.i < len(l.s) && l.s[l.i] != '\n' {
				l.i++
			}
		case c == '(' && l.peek(1) == '*':
			l.comment()
		case c == '"' || c == '@' && l.peek(1) == '"':
			verbatim := c == '@'
			if verbatim {
				l.i++
			}
			arguments = append(arguments, l.skipString(verbatim, false, 0))
		default:
			l.i++
		}
	}
	text := name
	for _, a := range arguments {
		text += "\x00" + a
	}
	l.tokens = append(l.tokens, token{kind: tDirectory, text: text, line: line, column: column, first: l.lastLine != line})
	l.lastLine, l.lastEnd = line, l.i
}

// stringAhead reports whether the @ or $ at i starts a string: @"", $"", $@"",
// @$"", $$"""...
func (l *lexer) stringAhead() bool {
	j := l.i
	for j < len(l.s) && (l.s[j] == '@' || l.s[j] == '$') && j-l.i < 8 {
		j++
	}
	return j < len(l.s) && l.s[j] == '"'
}

func (l *lexer) stringToken() {
	start, line := l.i, l.line
	column := start - l.lineStart
	verbatim, dollars := false, 0
	for l.s[l.i] != '"' {
		if l.s[l.i] == '@' {
			verbatim = true
		} else {
			dollars++
		}
		l.i++
	}
	text := l.skipString(verbatim, dollars > 0, dollars)
	if l.i < len(l.s) && l.s[l.i] == 'B' { // byte string
		l.i++
	}
	l.emitAt(tString, text, line, column)
}

// skipString moves past the string literal whose opening quote is at i and returns
// its text (escapes of plain strings kept as written; holes of interpolated ones
// left out). dollars is the number of $ of an interpolated string: $$"""..."""
// needs {{ to open a hole.
func (l *lexer) skipString(verbatim, interpolated bool, dollars int) string {
	if !verbatim && strings.HasPrefix(l.s[l.i:], `"""`) { // @""" is verbatim, starting with ""
		l.i += 3
		var b strings.Builder
		for l.i < len(l.s) {
			if strings.HasPrefix(l.s[l.i:], `"""`) {
				l.i += 3
				// """ may end in more quotes: """a"""" is a"
				for l.i < len(l.s) && l.s[l.i] == '"' {
					l.i++
				}
				return b.String()
			}
			if interpolated && l.s[l.i] == '{' {
				n := 0
				for l.i+n < len(l.s) && l.s[l.i+n] == '{' {
					n++
				}
				if n >= max(dollars, 1) {
					l.i += n - max(dollars, 1)
					l.hole()
					continue
				}
				l.i += n
				continue
			}
			if l.s[l.i] == '\n' {
				l.newline()
			}
			b.WriteByte(l.s[l.i])
			l.i++
		}
		return b.String()
	}
	l.i++ // the opening quote
	var b strings.Builder
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case c == '\n':
			l.newline()
			b.WriteByte(c)
			l.i++
		case c == '\\' && !verbatim:
			b.WriteByte(c)
			if l.i+1 < len(l.s) {
				if l.s[l.i+1] == '\n' {
					l.i++
					l.newline()
				}
				b.WriteByte(l.s[l.i+1])
			}
			l.i += 2
		case c == '"':
			if verbatim && l.peek(1) == '"' {
				b.WriteByte('"')
				l.i += 2
				continue
			}
			l.i++
			return b.String()
		case interpolated && c == '{':
			if l.peek(1) == '{' {
				l.i += 2
				continue
			}
			l.hole()
		default:
			b.WriteByte(c)
			l.i++
		}
	}
	l.i = min(l.i, len(l.s))
	return b.String()
}

// hole skips an interpolation hole starting at its '{', with the strings, chars and
// braces inside it.
func (l *lexer) hole() {
	depth := 0
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case c == '\n':
			l.newline()
			l.i++
		case c == '{':
			depth++
			l.i++
			if depth > maxNest {
				return
			}
		case c == '}':
			depth--
			l.i++
			if depth <= 0 {
				return
			}
		case c == '"':
			l.skipString(false, false, 0)
		case (c == '$' || c == '@') && l.stringAhead():
			verbatim, dollars := false, 0
			for l.i < len(l.s) && l.s[l.i] != '"' {
				if l.s[l.i] == '@' {
					verbatim = true
				} else {
					dollars++
				}
				l.i++
			}
			if dollars > 0 && depth < maxNest {
				l.skipString(verbatim, true, dollars)
			} else {
				l.skipString(verbatim, false, 0)
			}
		case c == '\'':
			if n := characterLength(l.s[l.i:]); n > 0 && (l.i == 0 || !isIdentifierByte(l.s[l.i-1])) {
				l.i += n
			} else {
				l.i++
			}
		default:
			l.i++
		}
	}
}

// quote reads what a ' starts: a char literal, a type variable, or nothing (an
// apostrophe glued to an identifier was taken by identifier already).
func (l *lexer) quote() {
	start := l.i
	if n := characterLength(l.s[l.i:]); n > 0 {
		l.i += n
		if l.i < len(l.s) && l.s[l.i] == 'B' {
			l.i++
		}
		l.emit(tCharacter, l.s[start:l.i], start)
		return
	}
	l.i++
	for l.i < len(l.s) && l.s[l.i] == '\'' { // ''a
		l.i++
	}
	if l.i < len(l.s) && chars.IsIdentStart(l.s[l.i]) {
		l.identifier()
		l.emit(tTyVariable, l.s[start:l.i], start)
		return
	}
	l.emit(tPunctuation, l.s[start:l.i], start)
}

// characterLength is the length of the char literal at the start of s ('a', '\n', '\”,
// 'A', '\065', 'é'), or 0 when s does not start one.
func characterLength(s string) int {
	if len(s) < 3 || s[0] != '\'' {
		return 0
	}
	if s[1] == '\\' { // cSpell: ignore UXXXXXXXX
		// \n \t \b \r \a \f \v \\ \" \' \0, \DDD, \xHH, \uXXXX, \UXXXXXXXX
		for n := 3; n <= 11 && n < len(s); n++ {
			if s[n] == '\'' {
				return n + 1
			}
			if s[n] == '\n' {
				return 0
			}
		}
		return 0
	}
	if s[1] == '\n' {
		return 0
	}
	_, w := utf8.DecodeRuneInString(s[1:])
	if 1+w < len(s) && s[1+w] == '\'' {
		return 2 + w
	}
	return 0
}
