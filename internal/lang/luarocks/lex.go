package luarocks

import "strings"

// Kind is what a token is.
type Kind uint8

const (
	EOF    Kind = iota
	Name        // an identifier or a keyword
	String      // a quoted or long-bracket string; Text is its value
	Number      // a numeral, as written
	Punct       // an operator or punctuation, as written
	Interp      // a Luau interpolated `string`; its text is not kept
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
func (t Token) Is(s string) bool { return (t.Kind == Punct || t.Kind == Name) && t.Text == s }

// Lex splits Lua source - Lua 5.1 to 5.4, LuaJIT, Luau and Teal - into tokens,
// dropping comments. It never fails: whatever is cut off at the end of the input (a
// string, a long comment) ends there.
//
// Implements: REQ-LUA-012
func Lex(src []byte) []Token {
	l := &lexer{src: src, line: 1}
	if len(src) >= 3 && src[0] == 0xef && src[1] == 0xbb && src[2] == 0xbf {
		l.i = 3
	}
	if l.i < len(src) && src[l.i] == '#' { // a shebang line, which Lua skips
		for l.i < len(src) && src[l.i] != '\n' {
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
	src   []byte
	i     int
	line  int
	depth int // nesting of interpolated strings, bounded
}

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// puncts are the operators of more than one character, longest first.
var puncts = []string{"...", "..=", "//=", "..", "==", "~=", "<=", ">=", "//", "::", "<<", ">>",
	"->", "+=", "-=", "*=", "/=", "%=", "^="}

func (l *lexer) next() Token {
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '-' && l.peek(1) == '-':
			l.i += 2
			if lvl, ok := l.longOpen(); ok {
				l.longBody(lvl)
				continue
			}
			for l.i < len(l.src) && l.src[l.i] != '\n' {
				l.i++
			}
		default:
			return l.token()
		}
	}
	return Token{Kind: EOF, Line: l.line, Start: l.i, End: l.i}
}

func (l *lexer) peek(n int) byte {
	if l.i+n < len(l.src) {
		return l.src[l.i+n]
	}
	return 0
}

func (l *lexer) token() Token {
	start, line := l.i, l.line
	c := l.src[l.i]
	tok := func(k Kind, text string) Token {
		return Token{Kind: k, Text: text, Line: line, Start: start, End: l.i}
	}
	switch {
	case isNameStart(c):
		for l.i < len(l.src) && (isNameStart(l.src[l.i]) || isDigit(l.src[l.i])) {
			l.i++
		}
		return tok(Name, string(l.src[start:l.i]))
	case isDigit(c) || (c == '.' && isDigit(l.peek(1))):
		l.number()
		return tok(Number, string(l.src[start:l.i]))
	case c == '"' || c == '\'':
		l.i++
		s := l.quoted(c)
		return tok(String, s)
	case c == '`':
		l.i++
		l.interpolated()
		return tok(Interp, "")
	case c == '[':
		if lvl, ok := l.longOpen(); ok {
			s := l.longBody(lvl)
			return tok(String, s)
		}
	}
	for _, p := range puncts {
		if l.i+len(p) <= len(l.src) && string(l.src[l.i:l.i+len(p)]) == p {
			l.i += len(p)
			return tok(Punct, p)
		}
	}
	l.i++
	return tok(Punct, single[c])
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
	hex := l.src[l.i] == '0' && (l.peek(1) == 'x' || l.peek(1) == 'X')
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case isDigit(c) || isNameStart(c) && c < 0x80 || c == '.':
			l.i++
			if (!hex && (c == 'e' || c == 'E')) || (hex && (c == 'p' || c == 'P')) {
				if l.i < len(l.src) && (l.src[l.i] == '+' || l.src[l.i] == '-') {
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
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == q:
			l.i++
			return string(b)
		case c == '\n':
			return string(b)
		case c == '\\':
			l.i++
			if l.i >= len(l.src) {
				return string(b)
			}
			e := l.src[l.i]
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
				for l.i < len(l.src) && (l.src[l.i] == ' ' || l.src[l.i] == '\t' || l.src[l.i] == '\r' || l.src[l.i] == '\n') {
					if l.src[l.i] == '\n' {
						l.line++
					}
					l.i++
				}
			case 'x':
				v, n := 0, 0
				for n < 2 && l.i < len(l.src) && hexVal(l.src[l.i]) >= 0 {
					v = v*16 + hexVal(l.src[l.i])
					l.i++
					n++
				}
				b = append(b, byte(v))
			case 'u':
				if l.i < len(l.src) && l.src[l.i] == '{' {
					l.i++
					v := 0
					for l.i < len(l.src) && hexVal(l.src[l.i]) >= 0 {
						v = (v*16 + hexVal(l.src[l.i])) & 0x7fffffff
						l.i++
					}
					if l.i < len(l.src) && l.src[l.i] == '}' {
						l.i++
					}
					b = append(b, string(rune(v))...)
				}
			default:
				if isDigit(e) {
					v := int(e - '0')
					for n := 1; n < 3 && l.i < len(l.src) && isDigit(l.src[l.i]); n++ {
						v = v*10 + int(l.src[l.i]-'0')
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

func hexVal(c byte) int {
	switch {
	case isDigit(c):
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
	if l.i >= len(l.src) || l.src[l.i] != '[' {
		return 0, false
	}
	j := l.i + 1
	for j < len(l.src) && l.src[j] == '=' {
		j++
	}
	if j >= len(l.src) || l.src[j] != '[' {
		return 0, false
	}
	lvl := j - l.i - 1
	l.i = j + 1
	return lvl, true
}

// longBody reads up to and including the closing bracket of the given level and
// returns the text in between, without a line break right after the opening.
func (l *lexer) longBody(lvl int) string {
	if l.i < len(l.src) && l.src[l.i] == '\r' {
		l.i++
	}
	if l.i < len(l.src) && l.src[l.i] == '\n' {
		l.line++
		l.i++
	}
	start := l.i
	for l.i < len(l.src) {
		c := l.src[l.i]
		if c == '\n' {
			l.line++
		}
		if c == ']' {
			j := l.i + 1
			for j < len(l.src) && l.src[j] == '=' {
				j++
			}
			if j < len(l.src) && l.src[j] == ']' && j-l.i-1 == lvl {
				s := string(l.src[start:l.i])
				l.i = j + 1
				return s
			}
		}
		l.i++
	}
	return string(l.src[start:])
}

// interpolated skips a Luau `string` after its opening backtick; the code inside
// {...} is lexed (and dropped) so a brace or backtick in a nested string does not
// end it early.
func (l *lexer) interpolated() {
	for l.i < len(l.src) {
		switch c := l.src[l.i]; c {
		case '`':
			l.i++
			return
		case '\\':
			l.i += 2
			if l.i-1 < len(l.src) && l.src[l.i-1] == '\n' {
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
				if t.Kind == EOF || (t.Kind == Punct && t.Text == "}" && braces == 0) {
					break
				}
				if t.Kind == Punct && t.Text == "{" {
					braces++
				} else if t.Kind == Punct && t.Text == "}" {
					braces--
				}
			}
			l.depth--
		default:
			l.i++
		}
	}
	if l.i > len(l.src) {
		l.i = len(l.src)
	}
}
