package nix

import "github.com/sarumaj/depphunter-cli/internal/lang/chars"

// The Nix lexer. Nix's grammar is small, but three things make a plain tokenizer
// wrong: strings ("..." and ''...'') hold ${...} code, which holds strings again; a
// path is a token of its own (./x.nix, a/b, ~/x, <nixpkgs>, and ./x/${y}.nix with
// code inside); and an unquoted URI (https://x) is a string. The lexer keeps a mode
// stack so that code inside an interpolation is ordinary tokens between tInterpolation and
// tInterpolationEnd, and the parser sees one flat token stream.

type tokenKind uint8

const (
	tEOF tokenKind = iota
	tIdentifier
	tNumber
	tPunctuation
	tStringOpen       // " or ''
	tStringText       // literal text of a string, escapes undone
	tStringClose      // " or ''
	tInterpolation    // ${ in a string, a path or an attribute name
	tInterpolationEnd // the } closing it
	tPath             // a whole path without interpolation: ./x.nix, ../lib, a/b, ~/x, /etc/x
	tPathOpen         // the first part of a path that continues with ${...}
	tPathText         // a later literal part of such a path
	tPathClose        // the end of such a path
	tSPath            // <nixpkgs/lib>, text without the brackets
	tURI              // https://example.org/x.tar.gz
)

type token struct {
	kind tokenKind
	text string
	line int
}

// maxModes bounds the nesting of strings and interpolations; deeper input is read on
// as code rather than growing the stack without end.
const maxModes = 256

type mode struct {
	kind  byte // 'd' "string", 'i' ''string'', 'p' path, 'c' interpolated code
	depth int  // braces open inside interpolated code
}

type lexer struct {
	source string
	i      int
	line   int
	modes  []mode
	tokens []token
	// noPath and noURI remember where a failed path or URI look-ahead ended: a
	// later start inside the same run of characters would fail the same way, and
	// retrying would be quadratic on a long a.b.c.d... chain.
	noPath, noURI int
}

// lex splits Nix source into tokens; it never fails, and unterminated strings,
// paths and interpolations end at EOF.
//
// Implements: REQ-NIX-011
func lex(source []byte) []token {
	s := string(source)
	if len(s) >= 3 && s[:3] == "\xef\xbb\xbf" {
		s = s[3:]
	}
	l := &lexer{source: s, line: 1, tokens: make([]token, 0, len(s)/5+8)}
	for l.i < len(l.source) {
		if n := len(l.modes); n > 0 {
			switch l.modes[n-1].kind {
			case 'd', 'i':
				l.readString(l.modes[n-1].kind)
				continue
			case 'p':
				l.pathRest()
				continue
			}
		}
		l.code()
	}
	// Unterminated strings and paths end at EOF.
	for n := len(l.modes); n > 0; n-- {
		switch l.modes[n-1].kind {
		case 'd', 'i':
			l.emit(tStringClose, "")
		case 'p':
			l.emit(tPathClose, "")
		case 'c':
			l.emit(tInterpolationEnd, "")
		}
	}
	l.tokens = append(l.tokens, token{kind: tEOF, line: l.line})
	return l.tokens
}

func (l *lexer) emit(k tokenKind, text string) { l.tokens = append(l.tokens, token{k, text, l.line}) }

func (l *lexer) push(k byte) bool {
	if len(l.modes) >= maxModes {
		return false
	}
	l.modes = append(l.modes, mode{kind: k})
	return true
}

func (l *lexer) pop() { l.modes = l.modes[:len(l.modes)-1] }

func (l *lexer) peek(k int) byte {
	if l.i+k < len(l.source) {
		return l.source[l.i+k]
	}
	return 0
}

func isIdentifierCharacter(c byte) bool {
	return chars.IsIdentStart(c) || c >= '0' && c <= '9' || c == '\'' || c == '-'
}

func isPathCharacter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' || c == '+'
}

func isSchemeCharacter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'
}

func isURICharacter(c byte) bool {
	if isPathCharacter(c) {
		return true
	}
	switch c {
	case '%', '/', '?', ':', '@', '&', '=', '$', ',', '!', '~', '*', '\'':
		return true
	}
	return false
}

// code reads one token of Nix code (or skips whitespace or a comment).
func (l *lexer) code() {
	c := l.source[l.i]
	switch {
	case c == '\n':
		l.line++
		l.i++
		return
	case c == ' ' || c == '\t' || c == '\r':
		l.i++
		return
	case c == '#':
		for l.i < len(l.source) && l.source[l.i] != '\n' {
			l.i++
		}
		return
	case c == '/' && l.peek(1) == '*':
		l.i += 2
		for l.i < len(l.source) && !(l.source[l.i] == '*' && l.peek(1) == '/') {
			if l.source[l.i] == '\n' {
				l.line++
			}
			l.i++
		}
		l.i = min(l.i+2, len(l.source))
		return
	case c == '"':
		l.emit(tStringOpen, `"`)
		l.i++
		if !l.push('d') {
			l.emit(tStringClose, "")
		}
		return
	case c == '\'' && l.peek(1) == '\'':
		l.emit(tStringOpen, "''")
		l.i += 2
		if !l.push('i') {
			l.emit(tStringClose, "")
		}
		return
	case c == '$' && l.peek(1) == '{':
		l.emit(tInterpolation, "${")
		l.i += 2
		if !l.push('c') {
			l.emit(tInterpolationEnd, "")
		}
		return
	case c == '{':
		if n := len(l.modes); n > 0 && l.modes[n-1].kind == 'c' {
			l.modes[n-1].depth++
		}
	case c == '}':
		if n := len(l.modes); n > 0 && l.modes[n-1].kind == 'c' {
			if l.modes[n-1].depth == 0 {
				l.emit(tInterpolationEnd, "}")
				l.i++
				l.pop()
				return
			}
			l.modes[n-1].depth--
		}
	case c == '<':
		if l.spath() {
			return
		}
	}
	if (isPathCharacter(c) || c == '/' || c == '~') && l.i >= l.noPath && l.path() {
		return
	}
	if chars.IsIdentStart(c) {
		if l.i >= l.noURI && l.uri() {
			return
		}
		j := l.i + 1
		for j < len(l.source) && isIdentifierCharacter(l.source[j]) {
			j++
		}
		l.emit(tIdentifier, l.source[l.i:j])
		l.i = j
		return
	}
	if c >= '0' && c <= '9' || c == '.' && l.peek(1) >= '0' && l.peek(1) <= '9' {
		j := l.i
		for j < len(l.source) && (l.source[j] >= '0' && l.source[j] <= '9' || l.source[j] == '.') {
			j++
		}
		if j < len(l.source) && (l.source[j] == 'e' || l.source[j] == 'E') {
			j++
			if j < len(l.source) && (l.source[j] == '+' || l.source[j] == '-') {
				j++
			}
			for j < len(l.source) && l.source[j] >= '0' && l.source[j] <= '9' {
				j++
			}
		}
		l.emit(tNumber, l.source[l.i:j])
		l.i = j
		return
	}
	for _, operator := range operators {
		if len(operator) > 1 && l.i+len(operator) <= len(l.source) && l.source[l.i:l.i+len(operator)] == operator {
			l.emit(tPunctuation, operator)
			l.i += len(operator)
			return
		}
	}
	if c < 0x80 {
		l.emit(tPunctuation, l.source[l.i:l.i+1])
	}
	l.i++ // a stray non-ASCII byte is skipped
}

var operators = []string{"...", "//", "++", "==", "!=", "<=", ">=", "&&", "||", "->", "|>", "<|"}

// path reads a path literal at l.i: path characters, then one or more /segments
// (a segment may start with ${...}). It reports false, consuming nothing, when
// there is none: "a / b" is a division, "//" an update.
func (l *lexer) path() bool {
	j := l.i
	if l.source[j] == '~' {
		if l.peek(1) != '/' {
			return false
		}
		j++
	} else {
		for j < len(l.source) && isPathCharacter(l.source[j]) {
			j++
		}
	}
	// At least one "/" followed by a path character or an interpolation.
	if !(j+1 < len(l.source) && l.source[j] == '/' && (isPathCharacter(l.source[j+1]) || l.source[j+1] == '$' && j+2 < len(l.source) && l.source[j+2] == '{')) {
		l.noPath = j
		if j == l.i {
			l.noPath = l.i + 1
		}
		return false
	}
	for j+1 < len(l.source) && l.source[j] == '/' && isPathCharacter(l.source[j+1]) {
		j++
		for j < len(l.source) && isPathCharacter(l.source[j]) {
			j++
		}
	}
	if j+2 < len(l.source) && l.source[j] == '/' && l.source[j+1] == '$' && l.source[j+2] == '{' {
		j++ // ./x/${y}: the slash belongs to the literal part
	}
	if j+1 < len(l.source) && l.source[j] == '$' && l.source[j+1] == '{' && l.push('p') {
		l.emit(tPathOpen, l.source[l.i:j])
		l.i = j
		return true
	}
	if j < len(l.source) && l.source[j] == '/' && j+1 < len(l.source) && !isPathCharacter(l.source[j+1]) && l.source[j+1] != '/' && l.source[j+1] != '*' {
		j++ // a trailing slash
	}
	l.emit(tPath, l.source[l.i:j])
	l.i = j
	return true
}

// pathRest continues a path after an interpolation: more literal parts and more
// interpolations, until a character that cannot be part of it.
func (l *lexer) pathRest() {
	if l.source[l.i] == '$' && l.peek(1) == '{' {
		l.emit(tInterpolation, "${")
		l.i += 2
		if !l.push('c') {
			l.emit(tInterpolationEnd, "")
		}
		return
	}
	j := l.i
	for j < len(l.source) && (isPathCharacter(l.source[j]) || l.source[j] == '/' && j+1 < len(l.source) && (isPathCharacter(l.source[j+1]) || l.source[j+1] == '$')) {
		j++
	}
	if j > l.i {
		l.emit(tPathText, l.source[l.i:j])
		l.i = j
		return
	}
	l.emit(tPathClose, "")
	l.pop()
}

// spath reads <nixpkgs> or <nixpkgs/lib> at l.i.
func (l *lexer) spath() bool {
	j := l.i + 1
	start := j
	for j < len(l.source) && (isPathCharacter(l.source[j]) || l.source[j] == '/' && j > start && j+1 < len(l.source) && isPathCharacter(l.source[j+1])) {
		j++
	}
	if j == start || j >= len(l.source) || l.source[j] != '>' {
		return false
	}
	l.emit(tSPath, l.source[start:j])
	l.i = j + 1
	return true
}

// uri reads an unquoted URI (scheme:rest, no space after the colon) at l.i.
func (l *lexer) uri() bool {
	j := l.i
	for j < len(l.source) && isSchemeCharacter(l.source[j]) {
		j++
	}
	if j+1 >= len(l.source) || l.source[j] != ':' || !isURICharacter(l.source[j+1]) {
		l.noURI = j
		return false
	}
	k := j + 1
	for k < len(l.source) && isURICharacter(l.source[k]) {
		k++
	}
	l.emit(tURI, l.source[l.i:k])
	l.i = k
	return true
}

// readString reads the next piece of a string: literal text up to an interpolation or the
// closing quote. Escapes are undone ("\n", ”$ and ”'), and $$ is a literal "$$".
func (l *lexer) readString(kind byte) {
	var b []byte
	start := l.line
	flush := func() {
		if len(b) > 0 {
			l.tokens = append(l.tokens, token{tStringText, string(b), start})
		}
	}
	for l.i < len(l.source) {
		c := l.source[l.i]
		switch {
		case c == '\n':
			l.line++
		case c == '$' && l.peek(1) == '$':
			b = append(b, '$', '$')
			l.i += 2
			continue
		case c == '$' && l.peek(1) == '{':
			flush()
			l.emit(tInterpolation, "${")
			l.i += 2
			if !l.push('c') {
				l.emit(tInterpolationEnd, "")
			}
			return
		case kind == 'd' && c == '\\' && l.i+1 < len(l.source):
			if e := l.source[l.i+1]; e == 'n' {
				b = append(b, '\n')
			} else if e == 't' {
				b = append(b, '\t')
			} else if e == 'r' {
				b = append(b, '\r')
			} else {
				if e == '\n' {
					l.line++
				}
				b = append(b, e)
			}
			l.i += 2
			continue
		case kind == 'd' && c == '"':
			flush()
			l.emit(tStringClose, `"`)
			l.i++
			l.pop()
			return
		case kind == 'i' && c == '\'' && l.peek(1) == '\'':
			switch e := l.peek(2); {
			case e == '\'':
				b = append(b, '\'', '\'')
				l.i += 3
				continue
			case e == '$':
				b = append(b, '$')
				l.i += 3
				continue
			case e == '\\' && l.i+3 < len(l.source):
				if l.source[l.i+3] == '\n' {
					l.line++
				}
				b = append(b, l.source[l.i+3])
				l.i += 4
				continue
			}
			flush()
			l.emit(tStringClose, "''")
			l.i += 2
			l.pop()
			return
		}
		b = append(b, c)
		l.i++
	}
	flush()
}
