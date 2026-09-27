package haxe

// The lexer reads Haxe well enough to tell code from comments, strings and
// regular expressions, which is all the declaration scanner needs. It is an
// iterator, so the resolver can read a file's package line without lexing the
// rest of it.

const (
	tIdent = iota + 1
	tPunct
	tString
	tRegex
	tNumber
	tDirective // #if, #elseif, #else, #end: text is the directive's name
)

type token struct {
	kind byte
	text string // identifier, punctuation or directive name; "" for literals
	line int
	pos  int // byte offset of the first byte
	end  int // byte offset after the last byte
}

type lexer struct {
	src  []byte
	i    int
	line int
	// interp holds the brace depth of every open ${ ... } of a single-quoted
	// string, innermost last. Tokens inside are not returned: an
	// interpolation is an expression, not a declaration.
	interp []int
	// start and startLine are where the outermost open string began.
	start, startLine int
}

func newLexer(src []byte) *lexer { return &lexer{src: src, line: 1} }

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentChar(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// next returns the next token outside strings, comments and interpolations,
// and false at the end of the input.
func (l *lexer) next() (token, bool) {
	for {
		if !l.skipSpace() {
			return token{}, false
		}
		t, emit := l.scan()
		if emit && len(l.interp) == 0 {
			return t, true
		}
	}
}

// skipSpace skips blanks and comments; false at the end of the input.
func (l *lexer) skipSpace() bool {
	src := l.src
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '/' && l.i+1 < len(src) && src[l.i+1] == '/':
			for l.i < len(src) && src[l.i] != '\n' {
				l.i++
			}
		case c == '/' && l.i+1 < len(src) && src[l.i+1] == '*':
			l.i += 2
			for l.i < len(src) && !(src[l.i] == '*' && l.i+1 < len(src) && src[l.i+1] == '/') {
				if src[l.i] == '\n' {
					l.line++
				}
				l.i++
			}
			l.i = min(l.i+2, len(src))
		default:
			return true
		}
	}
	return false
}

// scan reads one token at l.i; emit is false for what yields no token (an
// unknown byte, a skipped directive, a string still open around an
// interpolation).
func (l *lexer) scan() (token, bool) {
	src := l.src
	start, line := l.i, l.line
	c := src[l.i]
	tok := func(kind byte, text string) (token, bool) {
		return token{kind: kind, text: text, line: line, pos: start, end: l.i}, true
	}
	switch {
	case isIdentStart(c):
		for l.i < len(src) && isIdentChar(src[l.i]) {
			l.i++
		}
		return tok(tIdent, string(src[start:l.i]))
	case isDigit(c):
		for l.i < len(src) {
			d := src[l.i]
			if isIdentChar(d) {
				l.i++
			} else if d == '.' && l.i+1 < len(src) && isDigit(src[l.i+1]) {
				l.i++
			} else if (d == '+' || d == '-') && (src[l.i-1] == 'e' || src[l.i-1] == 'E') && !hexNumber(src[start:l.i]) {
				l.i++
			} else {
				break
			}
		}
		return tok(tNumber, "")
	case c == '"':
		l.i++
		l.doubleQuoted()
		return tok(tString, "")
	case c == '\'':
		l.i++
		if len(l.interp) == 0 {
			l.start, l.startLine = start, line
		}
		if l.singleQuoted() && len(l.interp) == 0 {
			return token{kind: tString, line: l.startLine, pos: l.start, end: l.i}, true
		}
		return token{}, false
	case c == '~' && l.i+1 < len(src) && src[l.i+1] == '/':
		l.i += 2
		for l.i < len(src) && src[l.i] != '/' && src[l.i] != '\n' {
			if src[l.i] == '\\' && l.i+1 < len(src) && src[l.i+1] != '\n' {
				l.i++
			}
			l.i++
		}
		if l.i < len(src) && src[l.i] == '/' {
			l.i++
			for l.i < len(src) && src[l.i] >= 'a' && src[l.i] <= 'z' {
				l.i++
			}
		}
		return tok(tRegex, "")
	case c == '#':
		l.i++
		n := l.i
		for l.i < len(src) && isIdentChar(src[l.i]) {
			l.i++
		}
		switch name := string(src[n:l.i]); name {
		case "if", "elseif":
			l.condition()
			return tok(tDirective, name)
		case "else", "end":
			return tok(tDirective, name)
		case "line":
			for l.i < len(src) && src[l.i] != '\n' {
				l.i++
			}
		}
		return token{}, false
	case c == '{' || c == '}':
		l.i++
		if n := len(l.interp); n > 0 {
			if c == '{' {
				l.interp[n-1]++
			} else if l.interp[n-1] > 0 {
				l.interp[n-1]--
			} else {
				// The interpolation ends: back inside its string.
				l.interp = l.interp[:n-1]
				if l.singleQuoted() && len(l.interp) == 0 {
					return token{kind: tString, line: l.startLine, pos: l.start, end: l.i}, true
				}
				return token{}, false
			}
		}
		return tok(tPunct, string(c))
	case c == '.' && l.i+2 < len(src) && src[l.i+1] == '.' && src[l.i+2] == '.':
		l.i += 3
		return tok(tPunct, "...")
	case (c == '-' || c == '=') && l.i+1 < len(src) && src[l.i+1] == '>':
		l.i += 2
		return tok(tPunct, string(src[start:l.i]))
	case c >= 0x80:
		l.i++ // a byte of a UTF-8 sequence outside a string: not Haxe
		return token{}, false
	}
	l.i++
	return tok(tPunct, string(c))
}

func hexNumber(s []byte) bool { return len(s) > 1 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') }

// doubleQuoted skips a "..." string after its opening quote.
func (l *lexer) doubleQuoted() {
	src := l.src
	for l.i < len(src) {
		switch src[l.i] {
		case '\\':
			l.i++
			if l.i < len(src) && src[l.i] == '\n' {
				l.line++
			}
		case '\n':
			l.line++
		case '"':
			l.i++
			return
		}
		l.i++
	}
}

// singleQuoted skips a '...' string from l.i up to its closing quote (true) or
// the ${ of an interpolation, which it opens (false).
func (l *lexer) singleQuoted() bool {
	src := l.src
	for l.i < len(src) {
		switch src[l.i] {
		case '\\':
			l.i++
			if l.i < len(src) && src[l.i] == '\n' {
				l.line++
			}
		case '\n':
			l.line++
		case '\'':
			l.i++
			return true
		case '$':
			if l.i+1 < len(src) && src[l.i+1] == '{' {
				l.i += 2
				l.interp = append(l.interp, 0)
				return false
			}
		}
		l.i++
	}
	return true // unterminated: ends with the input
}

// condition skips the condition of #if or #elseif: negations, then a
// parenthesized expression (strings inside skipped) or a (dotted) name.
func (l *lexer) condition() {
	src := l.src
	for l.i < len(src) && (src[l.i] == ' ' || src[l.i] == '\t' || src[l.i] == '!') {
		l.i++
	}
	if l.i < len(src) && src[l.i] == '(' {
		depth := 0
		for l.i < len(src) {
			switch src[l.i] {
			case '(':
				depth++
			case ')':
				depth--
			case '\n':
				l.line++
			case '"', '\'':
				q := src[l.i]
				l.i++
				for l.i < len(src) && src[l.i] != q && src[l.i] != '\n' {
					l.i++
				}
				if l.i >= len(src) || src[l.i] == '\n' {
					continue
				}
			}
			l.i++
			if depth == 0 {
				return
			}
		}
		return
	}
	for l.i < len(src) && (isIdentChar(src[l.i]) || src[l.i] == '.') {
		l.i++
	}
}
