package luarocks

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// Kind is what a token is.
type Kind uint8

const (
	EOF           Kind = iota
	Name               // an identifier or a keyword
	String             // a quoted or long-bracket string; Text is its value
	Number             // a numeral, as written
	Punctuation        // an operator or punctuation, as written
	Interpolation      // a Luau interpolated `string`; its text is not kept
)

// Token is one lexical token of Lua source.
type Token struct {
	Kind Kind
	Text string
	Line int
	// Start and End are the token's byte offsets in the source.
	Start, End int
}

// Is reports whether t is the punctuation or name s.
func (t Token) Is(s string) bool { return (t.Kind == Punctuation || t.Kind == Name) && t.Text == s }

// Lex splits Lua source - Lua 5.1 to 5.4, LuaJIT, Luau and Teal - into tokens,
// dropping comments. It never fails: whatever is cut off at the end of the input (a
// string, a long comment) ends there.
//
// Implements: REQ-LUA-012
func Lex(source []byte) []Token {
	l := &lexer{source: source, line: 1}
	if len(source) >= 3 && source[0] == 0xef && source[1] == 0xbb && source[2] == 0xbf {
		l.i = 3
	}
	if l.i < len(source) && source[l.i] == '#' { // a shebang line, which Lua skips
		for l.i < len(source) && source[l.i] != '\n' {
			l.i++
		}
	}
	var out []Token
	for {
		t := l.next()
		if t.Kind == EOF {
			return out
		}
		out = append(out, t)
	}
}

type lexer struct {
	source []byte
	i      int
	line   int
	depth  int // nesting of interpolated strings, bounded
}

// punctuations are the operators of more than one character, longest first.
var punctuations = []string{"...", "..=", "//=", "..", "==", "~=", "<=", ">=", "//", "::", "<<", ">>",
	"->", "+=", "-=", "*=", "/=", "%=", "^="}

func (l *lexer) next() Token {
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '-' && l.peek(1) == '-':
			l.i += 2
			if level, ok := l.longOpen(); ok {
				l.longBody(level)
				continue
			}
			for l.i < len(l.source) && l.source[l.i] != '\n' {
				l.i++
			}
		default:
			return l.token()
		}
	}
	return Token{Kind: EOF, Line: l.line, Start: l.i, End: l.i}
}

func (l *lexer) peek(n int) byte {
	if l.i+n < len(l.source) {
		return l.source[l.i+n]
	}
	return 0
}

func (l *lexer) token() Token {
	start, line := l.i, l.line
	c := l.source[l.i]
	token := func(k Kind, text string) Token {
		return Token{Kind: k, Text: text, Line: line, Start: start, End: l.i}
	}
	switch {
	case chars.IsIdentStartUTF8(c):
		for l.i < len(l.source) && (chars.IsIdentStartUTF8(l.source[l.i]) || chars.IsDigit(l.source[l.i])) {
			l.i++
		}
		return token(Name, string(l.source[start:l.i]))
	case chars.IsDigit(c) || (c == '.' && chars.IsDigit(l.peek(1))):
		l.number()
		return token(Number, string(l.source[start:l.i]))
	case c == '"' || c == '\'':
		l.i++
		s := l.quoted(c)
		return token(String, s)
	case c == '`':
		l.i++
		l.interpolated()
		return token(Interpolation, "")
	case c == '[':
		if level, ok := l.longOpen(); ok {
			s := l.longBody(level)
			return token(String, s)
		}
	}
	for _, p := range punctuations {
		if l.i+len(p) <= len(l.source) && string(l.source[l.i:l.i+len(p)]) == p {
			l.i += len(p)
			return token(Punctuation, p)
		}
	}
	l.i++
	return token(Punctuation, single[c])
}

// single holds every one-byte string, so punctuation costs no allocation.
var single = func() (t [256]string) {
	for i := range t {
		t[i] = string(rune(i))
		if i >= 0x80 {
			t[i] = string([]byte{byte(i)})
		}
	}
	return
}()

// number consumes a numeral: decimal or hex, with exponents, Luau's 0b and digit
// separators, LuaJIT's ULL/LL/i suffixes.
func (l *lexer) number() {
	hex := l.source[l.i] == '0' && (l.peek(1) == 'x' || l.peek(1) == 'X')
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case chars.IsDigit(c) || chars.IsIdentStartUTF8(c) && c < 0x80 || c == '.':
			l.i++
			if (!hex && (c == 'e' || c == 'E')) || (hex && (c == 'p' || c == 'P')) {
				if l.i < len(l.source) && (l.source[l.i] == '+' || l.source[l.i] == '-') {
					l.i++
				}
			}
		default:
			return
		}
	}
}

// quoted reads a short string after its opening quote and returns its value. An
// unescaped line break or the end of input ends an unfinished one.
func (l *lexer) quoted(q byte) string {
	var b []byte
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case c == q:
			l.i++
			return string(b)
		case c == '\n':
			return string(b)
		case c == '\\':
			l.i++
			if l.i >= len(l.source) {
				return string(b)
			}
			e := l.source[l.i]
			l.i++
			switch e {
			case 'n':
				b = append(b, '\n')
			case 't':
				b = append(b, '\t')
			case 'r':
				b = append(b, '\r')
			case 'a', 'b', 'f', 'v':
				b = append(b, "\a\b\f\v"[strings.IndexByte("abfv", e)])
			case '\n':
				l.line++
				b = append(b, '\n')
			case 'z': // skips the whitespace that follows, line breaks included
				for l.i < len(l.source) && (l.source[l.i] == ' ' || l.source[l.i] == '\t' || l.source[l.i] == '\r' || l.source[l.i] == '\n') {
					if l.source[l.i] == '\n' {
						l.line++
					}
					l.i++
				}
			case 'x':
				v, n := 0, 0
				for n < 2 && l.i < len(l.source) && hexValue(l.source[l.i]) >= 0 {
					v = v*16 + hexValue(l.source[l.i])
					l.i++
					n++
				}
				b = append(b, byte(v))
			case 'u':
				if l.i < len(l.source) && l.source[l.i] == '{' {
					l.i++
					v := 0
					for l.i < len(l.source) && hexValue(l.source[l.i]) >= 0 {
						v = (v*16 + hexValue(l.source[l.i])) & 0x7fffffff
						l.i++
					}
					if l.i < len(l.source) && l.source[l.i] == '}' {
						l.i++
					}
					b = append(b, string(rune(v))...)
				}
			default:
				if chars.IsDigit(e) {
					v := int(e - '0')
					for n := 1; n < 3 && l.i < len(l.source) && chars.IsDigit(l.source[l.i]); n++ {
						v = v*10 + int(l.source[l.i]-'0')
						l.i++
					}
					b = append(b, byte(v))
				} else {
					b = append(b, e)
				}
			}
		default:
			b = append(b, c)
			l.i++
		}
	}
	return string(b)
}

func hexValue(c byte) int {
	switch {
	case chars.IsDigit(c):
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// longOpen reads an opening long bracket ([[, [==[) at l.i and returns its level.
// Anything else leaves l.i where it was.
func (l *lexer) longOpen() (int, bool) {
	if l.i >= len(l.source) || l.source[l.i] != '[' {
		return 0, false
	}
	j := l.i + 1
	for j < len(l.source) && l.source[j] == '=' {
		j++
	}
	if j >= len(l.source) || l.source[j] != '[' {
		return 0, false
	}
	level := j - l.i - 1
	l.i = j + 1
	return level, true
}

// longBody reads up to and including the closing bracket of the given level and
// returns the text in between, without a line break right after the opening.
func (l *lexer) longBody(level int) string {
	if l.i < len(l.source) && l.source[l.i] == '\r' {
		l.i++
	}
	if l.i < len(l.source) && l.source[l.i] == '\n' {
		l.line++
		l.i++
	}
	start := l.i
	for l.i < len(l.source) {
		c := l.source[l.i]
		if c == '\n' {
			l.line++
		}
		if c == ']' {
			j := l.i + 1
			for j < len(l.source) && l.source[j] == '=' {
				j++
			}
			if j < len(l.source) && l.source[j] == ']' && j-l.i-1 == level {
				s := string(l.source[start:l.i])
				l.i = j + 1
				return s
			}
		}
		l.i++
	}
	return string(l.source[start:])
}

// interpolated skips a Luau `string` after its opening backtick; the code inside
// {...} is lexed (and dropped) so a brace or backtick in a nested string does not
// end it early.
func (l *lexer) interpolated() {
	for l.i < len(l.source) {
		switch c := l.source[l.i]; c {
		case '`':
			l.i++
			return
		case '\\':
			l.i += 2
			if l.i-1 < len(l.source) && l.source[l.i-1] == '\n' {
				l.line++
			}
		case '\n':
			l.line++
			l.i++
		case '{':
			l.i++
			if l.depth >= 50 {
				continue // deeper than any real code: read the rest as text
			}
			l.depth++
			for braces := 0; ; {
				t := l.next()
				if t.Kind == EOF || (t.Kind == Punctuation && t.Text == "}" && braces == 0) {
					break
				}
				if t.Kind == Punctuation && t.Text == "{" {
					braces++
				} else if t.Kind == Punctuation && t.Text == "}" {
					braces--
				}
			}
			l.depth--
		default:
			l.i++
		}
	}
	if l.i > len(l.source) {
		l.i = len(l.source)
	}
}
