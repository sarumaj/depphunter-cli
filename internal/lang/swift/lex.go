package swift

import (
	"bytes"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// Swift is read with a small scanner rather than the tree-sitter grammar. Measured on
// real projects (apple/swift-argument-parser, vapor/vapor, CodeEditApp/CodeEdit,
// signalapp/Signal-iOS), the grammar failed on about one file in ten - generic
// operator declarations, `#if` inside declarations, large closures, newer syntax -
// and a file with errors sent the parser into a retry ladder that ran to the 3 s
// parse bound, while its memory-budget checks stopped every other worker: minutes
// for Signal-iOS. What the map needs from a Swift file is regular: the imports, the
// declarations outside function bodies, and the type names the code uses. The
// scanner reads them from a token stream that knows nested comments, every string
// form (with interpolated code read as code), regex literals and `#if` lines.

type tokenKind uint8

const (
	tIdentifier  tokenKind = iota // an identifier or keyword; bt marks a `backticked` one
	tPunctuation                  // a bracket, separator or operator
	tString                       // a piece of a string literal (interpolations are tokens)
	tNumber                       // a numeric literal
	tRegex                        // a regex literal
	tDirective                    // a #if, #elseif, #else or #endif line; text is "if", ...
)

type token struct {
	kind tokenKind
	text string
	line int
	// newline marks a token with a line break between it and the token before.
	newline bool
	// bt marks a backticked identifier, whose text keeps the backticks.
	bt bool
	// member marks an identifier after a `.`: a member name, even a keyword's
	// (.default, .init).
	member bool
	// start and end are the token's byte offsets in the source.
	start, end int
}

// lexer turns Swift source into tokens. It never fails: an unterminated string,
// comment or regex ends at the end of the file (a single-line string at the end of
// its line), and bytes that start nothing are skipped.
type lexer struct {
	source   []byte
	position int
	line     int
	newline  bool
	out      []token
}

// tokenize reads all of source.
//
// Implements: REQ-SWIFT-014
func tokenize(source []byte) []token {
	l := &lexer{source: source, line: 1, newline: true}
	if bytes.HasPrefix(source, []byte("\xef\xbb\xbf")) { // a byte order mark
		l.position = 3
	}
	if bytes.HasPrefix(source[l.position:], []byte("#!")) { // a script's interpreter line
		l.skipLine()
	}
	for l.position < len(l.source) {
		l.next(0)
	}
	return l.out
}

// opCharacter reports the bytes operators are made of.
func opCharacter(c byte) bool {
	switch c {
	case '/', '=', '-', '+', '!', '*', '%', '<', '>', '&', '|', '^', '~', '?':
		return true
	}
	return false
}

// emit adds the token from start to l.position.
func (l *lexer) emit(kind tokenKind, start int, text string) { l.emitTo(kind, start, l.position, text) }

// emitTo adds the token from start to end. Lines are counted as bytes are consumed,
// so its line is l.line less the line breaks since start.
func (l *lexer) emitTo(kind tokenKind, start, end int, text string) {
	line := l.line - bytes.Count(l.source[start:l.position], []byte("\n"))
	member := kind == tIdentifier && len(l.out) > 0 && l.out[len(l.out)-1].kind == tPunctuation && l.out[len(l.out)-1].text == "."
	l.out = append(l.out, token{kind: kind, text: text, line: line, newline: l.newline, start: start, end: end, member: member})
	l.newline = false
}

func (l *lexer) advance(n int) {
	end := min(l.position+n, len(l.source))
	l.line += bytes.Count(l.source[l.position:end], []byte("\n"))
	l.position = end
}

func (l *lexer) skipLine() {
	for l.position < len(l.source) && l.source[l.position] != '\n' {
		l.position++
	}
}

// next reads one token, or skips a space or comment. depth > 0 is the parenthesis
// depth inside a string interpolation, and next returns it updated: 0 when the `)`
// closing the interpolation was read.
func (l *lexer) next(depth int) int {
	source, i := l.source, l.position
	c := source[i]
	switch {
	case c == '\n':
		l.line++
		l.newline = true
		l.position++
	case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v' || c == 0:
		l.position++
	case c == '/' && i+1 < len(source) && source[i+1] == '/':
		l.skipLine()
	case c == '/' && i+1 < len(source) && source[i+1] == '*':
		l.blockComment()
	case c == '#' && (l.newline || len(l.out) == 0) && directiveAt(source[i:]) != "":
		d := directiveAt(source[i:])
		l.skipLine()
		l.emit(tDirective, i, d)
		l.newline = true // the line break ending the directive is the next token's
	case c == '#' && l.hashLiteral():
	case c == '"':
		l.readString(0)
	case c == '`':
		j := i + 1
		for j < len(source) && source[j] != '`' && source[j] != '\n' {
			j++
		}
		if j < len(source) && source[j] == '`' {
			l.position = j + 1
			l.emit(tIdentifier, i, string(source[i:l.position]))
			l.out[len(l.out)-1].bt = true
		} else {
			l.position++
		}
	case chars.IsIdentStartUTF8(c) || c == '$':
		j := i + 1
		for j < len(source) && chars.IsIdentUTF8(source[j]) {
			j++
		}
		l.position = j
		l.emit(tIdentifier, i, string(source[i:j]))
	case c >= '0' && c <= '9':
		l.number()
	case c == '/' && l.regexStart():
		l.regex(0)
	case c == '.':
		n := 1
		if i+2 < len(source) && source[i+1] == '.' && (source[i+2] == '.' || source[i+2] == '<') {
			n = 3
		}
		l.position += n
		l.emit(tPunctuation, i, string(source[i:l.position]))
	case opCharacter(c):
		j := i
		for j < len(source) && opCharacter(source[j]) && !(source[j] == '/' && j+1 < len(source) && (source[j+1] == '/' || source[j+1] == '*')) {
			j++
		}
		run := source[i:j]
		switch {
		case string(run) == "->":
			l.position = i + 2
			l.emit(tPunctuation, i, "->")
		case len(bytes.Trim(run, "<>?!&")) == 0:
			// Brackets and type suffixes stand alone: Array<Set<Int>>?, T!, A & B.
			for k := range run {
				l.position = i + k + 1
				l.emit(tPunctuation, i+k, string(run[k]))
			}
		default:
			l.position = j
			l.emit(tPunctuation, i, string(run))
		}
	default:
		if depth > 0 && c == '(' {
			depth++
		} else if depth > 0 && c == ')' {
			depth--
		}
		l.position++
		l.emit(tPunctuation, i, string(c))
	}
	return depth
}

// directiveAt names the conditional-compilation directive a line starts with.
func directiveAt(b []byte) string {
	end := bytes.IndexByte(b, '\n')
	if end < 0 {
		end = len(b)
	}
	if !directive(b[:end]) {
		return ""
	}
	j := 1
	for j < end && chars.IsIdentUTF8(b[j]) {
		j++
	}
	return string(b[1:j])
}

func (l *lexer) blockComment() {
	depth := 0
	for l.position < len(l.source) {
		switch {
		case l.source[l.position] == '/' && l.position+1 < len(l.source) && l.source[l.position+1] == '*':
			depth++
			l.position += 2
		case l.source[l.position] == '*' && l.position+1 < len(l.source) && l.source[l.position+1] == '/':
			depth--
			l.position += 2
			if depth == 0 {
				return
			}
		default:
			if l.source[l.position] == '\n' {
				l.line++
				l.newline = true
			}
			l.position++
		}
	}
}

// hashLiteral reads what starts with `#` and is not a directive: a raw string
// (#"..."#), an extended regex (#/.../#) or, when neither follows, the `#` of a
// macro or pound keyword (#selector, #available), which is emitted alone.
func (l *lexer) hashLiteral() bool {
	i := l.position
	n := 0
	for i+n < len(l.source) && l.source[i+n] == '#' {
		n++
	}
	if i+n < len(l.source) {
		switch l.source[i+n] {
		case '"':
			l.position = i + n
			first := len(l.out)
			l.readString(n)
			l.out[first].start = i
			return true
		case '/':
			l.position = i + n
			l.regex(n)
			return true
		}
	}
	l.position = i + n // a run of hashes is one token
	l.emit(tPunctuation, i, "#")
	return true
}

// readString reads a string literal at l.position with hashes `#` around it: "...", """...""",
// #"..."#. Each interpolation \(...) (\#(...) in a raw string) is read as code:
// its tokens are emitted between the pieces of the string, after a `\(` token.
func (l *lexer) readString(hashes int) {
	source := l.source
	start := l.position
	// """ opens a multi-line string only when the line ends after it: #"""# is a
	// raw string holding a quote.
	multi := bytes.HasPrefix(source[l.position:], []byte(`"""`)) && lineEnds(source, l.position+3)
	if multi {
		l.position += 3
	} else {
		l.position++
	}
	piece := start
	closing := func(i int) bool {
		q := 1
		if multi {
			q = 3
		}
		if !bytes.HasPrefix(source[i:], bytes.Repeat([]byte{'"'}, q)) {
			return false
		}
		for k := 0; k < hashes; k++ {
			if i+q+k >= len(source) || source[i+q+k] != '#' {
				return false
			}
		}
		return true
	}
	for l.position < len(source) {
		c := source[l.position]
		switch {
		case c == '\n' && !multi:
			l.emit(tString, piece, "")
			return // unterminated: the line ends it
		case c == '\\' && l.escape(hashes):
			escape := l.position
			l.position += 1 + hashes
			if l.position < len(source) && source[l.position] == '(' {
				l.emitTo(tString, piece, escape, "")
				l.position++
				l.emit(tPunctuation, escape, `\(`)
				for d := 1; d > 0 && l.position < len(source); {
					d = l.next(d)
				}
				piece = l.position
				continue
			}
			if l.position < len(source) {
				l.advance(1)
			}
		case c == '"' && closing(l.position):
			if multi {
				l.position += 3
			} else {
				l.position++
			}
			l.position = min(l.position+hashes, len(source))
			l.emit(tString, piece, "")
			return
		default:
			l.advance(1)
		}
	}
	l.emit(tString, piece, "")
}

// escape reports whether the backslash at l.position starts an escape in a string with
// the given number of hashes: a raw string's escapes are \#, \##, ...
func (l *lexer) escape(hashes int) bool {
	for k := 1; k <= hashes; k++ {
		if l.position+k >= len(l.source) || l.source[l.position+k] != '#' {
			return false
		}
	}
	return true
}

func (l *lexer) number() {
	source, i := l.source, l.position
	hex := bytes.HasPrefix(source[i:], []byte("0x")) || bytes.HasPrefix(source[i:], []byte("0X"))
	j := i
	for j < len(source) {
		switch c := source[j]; {
		case chars.IsIdentUTF8(c):
		case (c == '+' || c == '-') && exponent(source[j-1], hex):
		case c == '.' && j+1 < len(source) && (source[j+1] >= '0' && source[j+1] <= '9' || hex && isHexDigit(source[j+1])) && !bytes.Contains(source[i:j], []byte(".")):
		default:
			l.position = j
			l.emit(tNumber, i, string(source[i:j]))
			return
		}
		j++
	}
	l.position = j
	l.emit(tNumber, i, string(source[i:j]))
}

// exponent reports whether c marks an exponent, after which a sign belongs to the
// number: e in decimal, p in hexadecimal literals.
func exponent(c byte, hex bool) bool {
	if hex {
		return c == 'p' || c == 'P'
	}
	return c == 'e' || c == 'E'
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// regexStart reports whether the `/` at l.position starts a regex literal rather than
// being division: it must not follow an operand, must not be followed by a space
// (`/ 2`) or `=` (`/=`), and must close with an unescaped `/` on the same line
// (outside a character class) that is not followed by an identifier character.
// Anything else is read as an operator, like the compiler does for `/`.
func (l *lexer) regexStart() bool {
	if n := len(l.out); n > 0 && !l.newline {
		if previous := l.out[n-1]; operandEnd(previous) {
			return false
		}
	}
	i := l.position + 1
	if i >= len(l.source) || l.source[i] == ' ' || l.source[i] == '\t' || l.source[i] == '=' || l.source[i] == '\n' {
		return false
	}
	class := false
	for ; i < len(l.source) && l.source[i] != '\n'; i++ {
		switch c := l.source[i]; {
		case c == '\\':
			i++
		case c == '[':
			class = true
		case c == ']':
			class = false
		case c == '/' && !class:
			return l.source[i-1] != ' ' && (i+1 >= len(l.source) || !chars.IsIdentUTF8(l.source[i+1]))
		}
	}
	return false
}

// operandEnd reports whether a token can end an operand, after which `/` divides.
func operandEnd(t token) bool {
	switch t.kind {
	case tIdentifier:
		return t.bt || t.member || !keywords[t.text] || operandKeywords[t.text]
	case tString, tNumber, tRegex:
		return true
	case tPunctuation:
		return t.text == ")" || t.text == "]" || t.text == "}" || t.text == "?" || t.text == "!" || t.text == ">"
	}
	return false
}

// regex reads a regex literal at l.position: /.../ on one line, or #/.../# (which may
// span lines) when hashes > 0.
func (l *lexer) regex(hashes int) {
	start := l.position - hashes
	l.position++
	class := false
	for l.position < len(l.source) {
		c := l.source[l.position]
		switch {
		case c == '\n' && hashes == 0:
			l.emit(tRegex, start, "")
			return
		case c == '\\':
			l.advance(2)
			continue
		case c == '[':
			class = true
		case c == ']':
			class = false
		case c == '/' && (!class || hashes > 0) && bytes.HasPrefix(l.source[l.position+1:], bytes.Repeat([]byte{'#'}, hashes)):
			l.position += 1 + hashes
			l.emit(tRegex, start, "")
			return
		}
		l.advance(1)
	}
	l.emit(tRegex, start, "")
}

// lineEnds reports whether only spaces follow offset i on its line.
func lineEnds(source []byte, i int) bool {
	for ; i < len(source) && source[i] != '\n'; i++ {
		if source[i] != ' ' && source[i] != '\t' && source[i] != '\r' {
			return false
		}
	}
	return true
}
