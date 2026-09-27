package beam

import (
	"unicode/utf8"
)

// The two languages are read by lexers of their own rather than the tree-sitter
// grammars: what the plugin needs - module names, definition heads, directives,
// remote calls, manifest terms - is all visible in the token stream once strings,
// sigils, heredocs, character literals and comments are out of the way, and a lexer
// never loses a file to a parse error.

type tokKind uint8

const (
	tIdent  tokKind = iota // Elixir identifier (do, end, fn and def included)
	tAlias                 // Elixir alias segment: Foo
	tAtom                  // :atom in Elixir, atom or 'quoted' in Erlang; val without quotes
	tKey                   // Elixir keyword key: `name:` or `"name":`
	tString                // string, charlist, heredoc or sigil; val is its raw content
	tNum                   // number
	tChar                  // character literal: ?a in Elixir, $a in Erlang
	tVar                   // Erlang variable
	tPunct                 // operator or punctuation; "end" is Erlang's form-ending dot
)

type token struct {
	kind tokKind
	val  string
	line int
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

func isLower(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 0x80 }
func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v'
}

type lexer struct {
	src    []byte
	i      int
	line   int
	tokens []token
}

func (l *lexer) at(off int) byte {
	if l.i+off < len(l.src) && l.i+off >= 0 {
		return l.src[l.i+off]
	}
	return 0
}

func (l *lexer) emit(k tokKind, val string, line int) {
	l.tokens = append(l.tokens, token{kind: k, val: val, line: line})
}

// advance moves past n bytes, counting the line breaks among them.
func (l *lexer) advance(n int) {
	for ; n > 0 && l.i < len(l.src); n-- {
		if l.src[l.i] == '\n' {
			l.line++
		}
		l.i++
	}
}

func (l *lexer) skipLine() {
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		l.i++
	}
}

func (l *lexer) hasPrefix(s string) bool {
	return l.i+len(s) <= len(l.src) && string(l.src[l.i:l.i+len(s)]) == s
}

// punct emits the longest operator of ops found at the cursor, or else one byte.
func (l *lexer) punct(ops []string) {
	for _, op := range ops {
		if l.hasPrefix(op) {
			l.emit(tPunct, op, l.line)
			l.i += len(op)
			return
		}
	}
	_, n := utf8.DecodeRune(l.src[l.i:])
	l.emit(tPunct, string(l.src[l.i:l.i+n]), l.line)
	l.i += n
}

func startLexer(src []byte) *lexer {
	l := &lexer{src: src, line: 1}
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		l.i = 3
	}
	if l.hasPrefix("#!") {
		l.skipLine() // an escript or elixir script's shebang
	}
	return l
}

// ---------------------------------------------------------------- Elixir

// Longest first, so "..." wins over "..".
var exOps = []string{"...", "\\\\", "::", "..", "=>", "|>", "<>", "->", "<-", "==", "!=", "&&", "||", "++", "--", "<<", ">>", "<=", ">=", "=~"}

// lexElixir tokenizes Elixir source. Interpolations inside strings are skipped with
// the string, so a call written in one is not seen.
func lexElixir(src []byte) []token {
	l := startLexer(src)
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case isSpace(c):
			l.i++
		case c == '#':
			l.skipLine()
		case c == '"' || c == '\'':
			line := l.line
			var s string
			if l.hasPrefix(`"""`) || l.hasPrefix(`'''`) {
				s = l.exHeredoc(c, true)
			} else {
				l.i++
				s = l.exQuoted(c, true)
			}
			// "key": value - a quoted keyword key (mix.lock writes its keys so).
			if l.at(0) == ':' && (isSpace(l.at(1)) || l.at(1) == 0) {
				l.i++
				l.emit(tKey, s, line)
			} else {
				l.emit(tString, s, line)
			}
		case c == '?':
			// A character literal (?a, ?\n, ?"); identifiers ending in ? are read whole
			// below, so a lone ? is always one.
			line := l.line
			l.i++
			if l.at(0) == '\\' {
				l.i++
			}
			if l.i < len(l.src) {
				_, n := utf8.DecodeRune(l.src[l.i:])
				l.advance(n)
			}
			l.emit(tChar, "", line)
		case c == '~' && (isLower(l.at(1)) && l.at(1) != '_' && l.at(1) < 0x80 || isUpper(l.at(1))):
			l.exSigil()
		case c == ':':
			l.exColon()
		case isDigit(c):
			l.number()
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80:
			l.exIdent()
		default:
			l.punct(exOps)
		}
	}
	return l.tokens
}

func (l *lexer) exIdent() {
	start, line := l.i, l.line
	for l.i < len(l.src) && isIdentByte(l.src[l.i]) {
		l.i++
	}
	name := string(l.src[start:l.i])
	keyword := l.at(0) == ':' && l.at(1) != ':' && (isSpace(l.at(1)) || l.at(1) == 0)
	if isUpper(name[0]) {
		if keyword { // [Plugs: [...]]: a keyword key, not a module
			l.i++
			l.emit(tKey, name, line)
			return
		}
		l.emit(tAlias, name, line)
		return
	}
	if c := l.at(0); (c == '?' || c == '!') && l.at(1) != '=' {
		l.i++
		name += string(c)
	}
	// name: value is a keyword key; name::type and name :: type are not.
	if l.at(0) == ':' && l.at(1) != ':' && (isSpace(l.at(1)) || l.at(1) == 0) {
		l.i++
		l.emit(tKey, name, line)
		return
	}
	l.emit(tIdent, name, line)
}

func (l *lexer) exColon() {
	line := l.line
	switch c := l.at(1); {
	case c == ':':
		l.emit(tPunct, "::", line)
		l.i += 2
	case c == '"' || c == '\'':
		l.i += 2
		s := l.exQuoted(c, true)
		l.emit(tAtom, s, line)
	case isIdentByte(c) && !isDigit(c):
		l.i++
		start := l.i
		for l.i < len(l.src) && (isIdentByte(l.src[l.i]) || l.src[l.i] == '@') {
			l.i++
		}
		if c := l.at(0); c == '?' || c == '!' {
			l.i++
		}
		l.emit(tAtom, string(l.src[start:l.i]), line)
	case c != 0 && !isSpace(c) && c != ',' && c != ')' && c != ']' && c != '}':
		// An operator atom: :+, :<<>>, :&&, :.
		l.i++
		start := l.i
		for l.i < len(l.src) && isOpByte(l.src[l.i]) {
			l.i++
		}
		if l.i == start {
			l.emit(tPunct, ":", line)
			return
		}
		l.emit(tAtom, string(l.src[start:l.i]), line)
	default:
		l.emit(tPunct, ":", line)
		l.i++
	}
}

func isOpByte(c byte) bool {
	switch c {
	case '+', '-', '*', '/', '<', '>', '=', '!', '&', '|', '^', '~', '.', '@', '\\', '%', '{', '}':
		return true
	}
	return false
}

// exQuoted reads a string body after its opening quote up to the closing one,
// skipping escapes and, when interpolate, #{...} interpolations.
func (l *lexer) exQuoted(q byte, interpolate bool) string {
	return l.exUntil(func() int {
		if l.src[l.i] == q {
			return 1
		}
		return 0
	}, interpolate)
}

// exUntil reads up to the terminator end reports the length of, returning the text
// before it.
func (l *lexer) exUntil(end func() int, interpolate bool) string {
	start := l.i
	for l.i < len(l.src) {
		c := l.src[l.i]
		if c == '\\' {
			l.advance(2)
			continue
		}
		if n := end(); n > 0 {
			s := string(l.src[start:l.i])
			l.i += n
			return s
		}
		if interpolate && c == '#' && l.at(1) == '{' {
			l.i += 2
			l.skipInterpolation()
			continue
		}
		l.advance(1)
	}
	return string(l.src[start:])
}

// skipInterpolation skips the code of a #{...} up to its closing brace, stepping
// over nested braces and strings.
func (l *lexer) skipInterpolation() {
	depth := 1
	for l.i < len(l.src) {
		switch c := l.src[l.i]; c {
		case '{':
			depth++
			l.i++
		case '}':
			depth--
			l.i++
			if depth == 0 {
				return
			}
		case '"', '\'':
			if l.hasPrefix(`"""`) || l.hasPrefix(`'''`) {
				l.exHeredoc(c, true)
			} else {
				l.i++
				l.exQuoted(c, true)
			}
		case '?':
			l.i++
			if l.at(0) == '\\' {
				l.i++
			}
			l.advance(1)
		default:
			l.advance(1)
		}
	}
}

// exHeredoc reads a """ or ”' heredoc from its opening delimiter.
func (l *lexer) exHeredoc(q byte, interpolate bool) string {
	l.i += 3
	delim := string([]byte{q, q, q})
	return l.exUntil(func() int {
		if l.hasPrefix(delim) {
			return 3
		}
		return 0
	}, interpolate)
}

var sigilClose = map[byte]byte{'(': ')', '[': ']', '{': '}', '<': '>', '/': '/', '|': '|', '"': '"', '\'': '\''}

// exSigil reads ~r/.../i, ~s(...), ~H"""...""" and their kin. Lower-case sigils
// interpolate; upper-case ones do not.
func (l *lexer) exSigil() {
	line := l.line
	l.i++ // ~
	start := l.i
	lower := isLower(l.src[l.i])
	if lower {
		l.i++
	} else {
		for l.i < len(l.src) && (isUpper(l.src[l.i]) || isDigit(l.src[l.i])) {
			l.i++
		}
	}
	if l.i == start {
		l.emit(tPunct, "~", line)
		return
	}
	var s string
	switch d := l.at(0); {
	case (d == '"' || d == '\'') && (l.hasPrefix(`"""`) || l.hasPrefix(`'''`)):
		s = l.exHeredoc(d, lower)
	case sigilClose[d] != 0:
		closer := sigilClose[d]
		l.i++
		s = l.exUntil(func() int {
			if l.src[l.i] == closer {
				return 1
			}
			return 0
		}, lower)
	default:
		l.emit(tPunct, "~", line)
		l.i = start
		return
	}
	for l.i < len(l.src) && (l.src[l.i] >= 'a' && l.src[l.i] <= 'z' || l.src[l.i] >= 'A' && l.src[l.i] <= 'Z') {
		l.i++ // modifiers
	}
	l.emit(tString, s, line)
}

// number reads digits, underscores, a base prefix, a fraction and an exponent: all
// that matters is that none of it is taken for something else.
func (l *lexer) number() {
	start, line := l.i, l.line
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case isIdentByte(c) && c < 0x80, c == '#':
		case c == '.' && isDigit(l.at(1)):
		case (c == '+' || c == '-') && (l.src[l.i-1] == 'e' || l.src[l.i-1] == 'E') && !hexNumber(l.src[start:l.i]):
		default:
			l.emit(tNum, string(l.src[start:l.i]), line)
			return
		}
		l.i++
	}
	l.emit(tNum, string(l.src[start:]), line)
}

func hexNumber(b []byte) bool {
	return len(b) > 1 && b[0] == '0' && (b[1] == 'x' || b[1] == 'X') || containsByte(b, '#')
}

func containsByte(b []byte, c byte) bool {
	for _, x := range b {
		if x == c {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- Erlang

var erlOps = []string{"=:=", "=/=", "...", "->", "<-", "<=", "=>", ":=", "::", "||", "<<", ">>", "==", "/=", "=<", ">=", "++", "--", "?=", "??", ".."}

// lexErlang tokenizes Erlang source (and the Erlang terms of rebar.config,
// rebar.lock and .app.src). A dot that ends a form becomes the punct "end".
func lexErlang(src []byte) []token {
	l := startLexer(src)
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case isSpace(c):
			l.i++
		case c == '%':
			l.skipLine()
		case c == '"':
			line := l.line
			if l.hasPrefix(`"""`) {
				l.emit(tString, l.exHeredoc('"', false), line)
			} else {
				l.i++
				l.emit(tString, l.exQuoted('"', false), line)
			}
		case c == '\'':
			line := l.line
			l.i++
			l.emit(tAtom, l.exQuoted('\'', false), line)
		case c == '$':
			line := l.line
			l.i++
			if l.at(0) == '\\' {
				l.i++
				if l.at(0) == 'x' && l.at(1) == '{' {
					for l.i < len(l.src) && l.src[l.i] != '}' {
						l.i++
					}
				} else if l.at(0) == '^' {
					l.i++
				}
			}
			if l.i < len(l.src) {
				_, n := utf8.DecodeRune(l.src[l.i:])
				l.advance(n)
			}
			l.emit(tChar, "", line)
		case c == '~' && (l.at(1) == '"' || l.at(1) >= 'a' && l.at(1) <= 'z' || l.at(1) >= 'A' && l.at(1) <= 'Z'):
			l.exSigil() // OTP 27 sigils
		case c == '.' && (l.i+1 >= len(l.src) || isSpace(l.at(1)) || l.at(1) == '%'):
			l.emit(tPunct, "end", l.line)
			l.i++
		case isDigit(c):
			l.number()
		case c >= 'a' && c <= 'z' || c >= 0x80:
			start, line := l.i, l.line
			for l.i < len(l.src) && (isIdentByte(l.src[l.i]) || l.src[l.i] == '@') {
				l.i++
			}
			l.emit(tAtom, string(l.src[start:l.i]), line)
		case c == '_' || isUpper(c):
			start, line := l.i, l.line
			for l.i < len(l.src) && (isIdentByte(l.src[l.i]) || l.src[l.i] == '@') {
				l.i++
			}
			l.emit(tVar, string(l.src[start:l.i]), line)
		default:
			l.punct(erlOps)
		}
	}
	return l.tokens
}
