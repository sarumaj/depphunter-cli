package haxe

import "github.com/sarumaj/depphunter-cli/internal/lang/chars"

// The lexer reads Haxe well enough to tell code from comments, strings and
// regular expressions, which is all the declaration scanner needs. It is an
// iterator, so the resolver can read a file's package line without lexing the
// rest of it.

const (
	tIdentifier = iota + 1
	tPunctuation
	tString
	tRegex
	tNumber
	tDirective // #if, #elseif, #else, #end: text is the directive's name
)

type token struct {
	kind     byte
	text     string // identifier, punctuation or directive name; "" for literals
	line     int
	position int // byte offset of the first byte
	end      int // byte offset after the last byte
}

type lexer struct {
	source []byte
	i      int
	line   int
	// interp holds the brace depth of every open ${ ... } of a single-quoted
	// string, innermost last. Tokens inside are not returned: an
	// interpolation is an expression, not a declaration.
	interpolation []int
	// start and startLine are where the outermost open string began.
	start, startLine int
}

func newLexer(source []byte) *lexer { return &lexer{source: source, line: 1} }

func isIdentifierStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentifierCharacter(c byte) bool { return isIdentifierStart(c) || (c >= '0' && c <= '9') }

// next returns the next token outside strings, comments and interpolations,
// and false at the end of the input.
func (l *lexer) next() (token, bool) {
	for {
		if !l.skipSpace() {
			return token{}, false
		}
		t, emit := l.scan()
		if emit && len(l.interpolation) == 0 {
			return t, true
		}
	}
}

// skipSpace skips blanks and comments; false at the end of the input.
func (l *lexer) skipSpace() bool {
	source := l.source
	for l.i < len(source) {
		c := source[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '/' && l.i+1 < len(source) && source[l.i+1] == '/':
			for l.i < len(source) && source[l.i] != '\n' {
				l.i++
			}
		case c == '/' && l.i+1 < len(source) && source[l.i+1] == '*':
			l.i += 2
			for l.i < len(source) && !(source[l.i] == '*' && l.i+1 < len(source) && source[l.i+1] == '/') {
				if source[l.i] == '\n' {
					l.line++
				}
				l.i++
			}
			l.i = min(l.i+2, len(source))
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
	source := l.source
	start, line := l.i, l.line
	c := source[l.i]
	makeToken := func(kind byte, text string) (token, bool) {
		return token{kind: kind, text: text, line: line, position: start, end: l.i}, true
	}
	switch {
	case isIdentifierStart(c):
		for l.i < len(source) && isIdentifierCharacter(source[l.i]) {
			l.i++
		}
		return makeToken(tIdentifier, string(source[start:l.i]))
	case chars.IsDigit(c):
		for l.i < len(source) {
			d := source[l.i]
			if isIdentifierCharacter(d) {
				l.i++
			} else if d == '.' && l.i+1 < len(source) && chars.IsDigit(source[l.i+1]) {
				l.i++
			} else if (d == '+' || d == '-') && (source[l.i-1] == 'e' || source[l.i-1] == 'E') && !hexNumber(source[start:l.i]) {
				l.i++
			} else {
				break
			}
		}
		return makeToken(tNumber, "")
	case c == '"':
		l.i++
		l.doubleQuoted()
		return makeToken(tString, "")
	case c == '\'':
		l.i++
		if len(l.interpolation) == 0 {
			l.start, l.startLine = start, line
		}
		if l.singleQuoted() && len(l.interpolation) == 0 {
			return token{kind: tString, line: l.startLine, position: l.start, end: l.i}, true
		}
		return token{}, false
	case c == '~' && l.i+1 < len(source) && source[l.i+1] == '/':
		l.i += 2
		for l.i < len(source) && source[l.i] != '/' && source[l.i] != '\n' {
			if source[l.i] == '\\' && l.i+1 < len(source) && source[l.i+1] != '\n' {
				l.i++
			}
			l.i++
		}
		if l.i < len(source) && source[l.i] == '/' {
			l.i++
			for l.i < len(source) && source[l.i] >= 'a' && source[l.i] <= 'z' {
				l.i++
			}
		}
		return makeToken(tRegex, "")
	case c == '#':
		l.i++
		n := l.i
		for l.i < len(source) && isIdentifierCharacter(source[l.i]) {
			l.i++
		}
		switch name := string(source[n:l.i]); name {
		case "if", "elseif":
			l.condition()
			return makeToken(tDirective, name)
		case "else", "end":
			return makeToken(tDirective, name)
		case "line":
			for l.i < len(source) && source[l.i] != '\n' {
				l.i++
			}
		}
		return token{}, false
	case c == '{' || c == '}':
		l.i++
		if n := len(l.interpolation); n > 0 {
			if c == '{' {
				l.interpolation[n-1]++
			} else if l.interpolation[n-1] > 0 {
				l.interpolation[n-1]--
			} else {
				// The interpolation ends: back inside its string.
				l.interpolation = l.interpolation[:n-1]
				if l.singleQuoted() && len(l.interpolation) == 0 {
					return token{kind: tString, line: l.startLine, position: l.start, end: l.i}, true
				}
				return token{}, false
			}
		}
		return makeToken(tPunctuation, string(c))
	case c == '.' && l.i+2 < len(source) && source[l.i+1] == '.' && source[l.i+2] == '.':
		l.i += 3
		return makeToken(tPunctuation, "...")
	case (c == '-' || c == '=') && l.i+1 < len(source) && source[l.i+1] == '>':
		l.i += 2
		return makeToken(tPunctuation, string(source[start:l.i]))
	case c >= 0x80:
		l.i++ // a byte of a UTF-8 sequence outside a string: not Haxe
		return token{}, false
	}
	l.i++
	return makeToken(tPunctuation, string(c))
}

func hexNumber(s []byte) bool { return len(s) > 1 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') }

// doubleQuoted skips a "..." string after its opening quote.
func (l *lexer) doubleQuoted() {
	source := l.source
	for l.i < len(source) {
		switch source[l.i] {
		case '\\':
			l.i++
			if l.i < len(source) && source[l.i] == '\n' {
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
	source := l.source
	for l.i < len(source) {
		switch source[l.i] {
		case '\\':
			l.i++
			if l.i < len(source) && source[l.i] == '\n' {
				l.line++
			}
		case '\n':
			l.line++
		case '\'':
			l.i++
			return true
		case '$':
			if l.i+1 < len(source) && source[l.i+1] == '{' {
				l.i += 2
				l.interpolation = append(l.interpolation, 0)
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
	source := l.source
	for l.i < len(source) && (source[l.i] == ' ' || source[l.i] == '\t' || source[l.i] == '!') {
		l.i++
	}
	if l.i < len(source) && source[l.i] == '(' {
		depth := 0
		for l.i < len(source) {
			switch source[l.i] {
			case '(':
				depth++
			case ')':
				depth--
			case '\n':
				l.line++
			case '"', '\'':
				q := source[l.i]
				l.i++
				for l.i < len(source) && source[l.i] != q && source[l.i] != '\n' {
					l.i++
				}
				if l.i >= len(source) || source[l.i] == '\n' {
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
	for l.i < len(source) && (isIdentifierCharacter(source[l.i]) || source[l.i] == '.') {
		l.i++
	}
}
