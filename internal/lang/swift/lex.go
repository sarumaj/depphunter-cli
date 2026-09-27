package swift

import "bytes"

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

type tokKind uint8

const (
	tIdent     tokKind = iota // an identifier or keyword; bt marks a `backticked` one
	tPunct                    // a bracket, separator or operator
	tString                   // a piece of a string literal (interpolations are tokens)
	tNumber                   // a numeric literal
	tRegex                    // a regex literal
	tDirective                // a #if, #elseif, #else or #endif line; text is "if", ...
)

type token struct {
	kind tokKind
	text string
	line int
	// nl marks a token with a line break between it and the token before.
	nl bool
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
	src  []byte
	pos  int
	line int
	nl   bool
	out  []token
}

// tokenize reads all of src.
//
// Implements: REQ-SWIFT-014
func tokenize(src []byte) []token {
	l := &lexer{src: src, line: 1, nl: true}
	if bytes.HasPrefix(src, []byte("\xef\xbb\xbf")) { // a byte order mark
		l.pos = 3
	}
	if bytes.HasPrefix(src[l.pos:], []byte("#!")) { // a script's interpreter line
		l.skipLine()
	}
	for l.pos < len(l.src) {
		l.next(0)
	}
	return l.out
}

func isIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentPart(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }

// opChar reports the bytes operators are made of.
func opChar(c byte) bool {
	switch c {
	case '/', '=', '-', '+', '!', '*', '%', '<', '>', '&', '|', '^', '~', '?':
		return true
	}
	return false
}

// emit adds the token from start to l.pos.
func (l *lexer) emit(kind tokKind, start int, text string) { l.emitTo(kind, start, l.pos, text) }

// emitTo adds the token from start to end. Lines are counted as bytes are consumed,
// so its line is l.line less the line breaks since start.
func (l *lexer) emitTo(kind tokKind, start, end int, text string) {
	line := l.line - bytes.Count(l.src[start:l.pos], []byte("\n"))
	member := kind == tIdent && len(l.out) > 0 && l.out[len(l.out)-1].kind == tPunct && l.out[len(l.out)-1].text == "."
	l.out = append(l.out, token{kind: kind, text: text, line: line, nl: l.nl, start: start, end: end, member: member})
	l.nl = false
}

func (l *lexer) advance(n int) {
	end := min(l.pos+n, len(l.src))
	l.line += bytes.Count(l.src[l.pos:end], []byte("\n"))
	l.pos = end
}

func (l *lexer) skipLine() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.pos++
	}
}

// next reads one token, or skips a space or comment. depth > 0 is the parenthesis
// depth inside a string interpolation, and next returns it updated: 0 when the `)`
// closing the interpolation was read.
func (l *lexer) next(depth int) int {
	src, i := l.src, l.pos
	c := src[i]
	switch {
	case c == '\n':
		l.line++
		l.nl = true
		l.pos++
	case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v' || c == 0:
		l.pos++
	case c == '/' && i+1 < len(src) && src[i+1] == '/':
		l.skipLine()
	case c == '/' && i+1 < len(src) && src[i+1] == '*':
		l.blockComment()
	case c == '#' && (l.nl || len(l.out) == 0) && directiveAt(src[i:]) != "":
		d := directiveAt(src[i:])
		l.skipLine()
		l.emit(tDirective, i, d)
		l.nl = true // the line break ending the directive is the next token's
	case c == '#' && l.hashLiteral():
	case c == '"':
		l.str(0)
	case c == '`':
		j := i + 1
		for j < len(src) && src[j] != '`' && src[j] != '\n' {
			j++
		}
		if j < len(src) && src[j] == '`' {
			l.pos = j + 1
			l.emit(tIdent, i, string(src[i:l.pos]))
			l.out[len(l.out)-1].bt = true
		} else {
			l.pos++
		}
	case isIdentStart(c) || c == '$':
		j := i + 1
		for j < len(src) && isIdentPart(src[j]) {
			j++
		}
		l.pos = j
		l.emit(tIdent, i, string(src[i:j]))
	case c >= '0' && c <= '9':
		l.number()
	case c == '/' && l.regexStart():
		l.regex(0)
	case c == '.':
		n := 1
		if i+2 < len(src) && src[i+1] == '.' && (src[i+2] == '.' || src[i+2] == '<') {
			n = 3
		}
		l.pos += n
		l.emit(tPunct, i, string(src[i:l.pos]))
	case opChar(c):
		j := i
		for j < len(src) && opChar(src[j]) && !(src[j] == '/' && j+1 < len(src) && (src[j+1] == '/' || src[j+1] == '*')) {
			j++
		}
		run := src[i:j]
		switch {
		case string(run) == "->":
			l.pos = i + 2
			l.emit(tPunct, i, "->")
		case len(bytes.Trim(run, "<>?!&")) == 0:
			// Brackets and type suffixes stand alone: Array<Set<Int>>?, T!, A & B.
			for k := range run {
				l.pos = i + k + 1
				l.emit(tPunct, i+k, string(run[k]))
			}
		default:
			l.pos = j
			l.emit(tPunct, i, string(run))
		}
	default:
		if depth > 0 && c == '(' {
			depth++
		} else if depth > 0 && c == ')' {
			depth--
		}
		l.pos++
		l.emit(tPunct, i, string(c))
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
	for j < end && isIdentPart(b[j]) {
		j++
	}
	return string(b[1:j])
}

func (l *lexer) blockComment() {
	depth := 0
	for l.pos < len(l.src) {
		switch {
		case l.src[l.pos] == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '*':
			depth++
			l.pos += 2
		case l.src[l.pos] == '*' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/':
			depth--
			l.pos += 2
			if depth == 0 {
				return
			}
		default:
			if l.src[l.pos] == '\n' {
				l.line++
				l.nl = true
			}
			l.pos++
		}
	}
}

// hashLiteral reads what starts with `#` and is not a directive: a raw string
// (#"..."#), an extended regex (#/.../#) or, when neither follows, the `#` of a
// macro or pound keyword (#selector, #available), which is emitted alone.
func (l *lexer) hashLiteral() bool {
	i := l.pos
	n := 0
	for i+n < len(l.src) && l.src[i+n] == '#' {
		n++
	}
	if i+n < len(l.src) {
		switch l.src[i+n] {
		case '"':
			l.pos = i + n
			first := len(l.out)
			l.str(n)
			l.out[first].start = i
			return true
		case '/':
			l.pos = i + n
			l.regex(n)
			return true
		}
	}
	l.pos = i + n // a run of hashes is one token
	l.emit(tPunct, i, "#")
	return true
}

// str reads a string literal at l.pos with hashes `#` around it: "...", """...""",
// #"..."#. Each interpolation \(...) (\#(...) in a raw string) is read as code:
// its tokens are emitted between the pieces of the string, after a `\(` token.
func (l *lexer) str(hashes int) {
	src := l.src
	start := l.pos
	// """ opens a multi-line string only when the line ends after it: #"""# is a
	// raw string holding a quote.
	multi := bytes.HasPrefix(src[l.pos:], []byte(`"""`)) && lineEnds(src, l.pos+3)
	if multi {
		l.pos += 3
	} else {
		l.pos++
	}
	piece := start
	closing := func(i int) bool {
		q := 1
		if multi {
			q = 3
		}
		if !bytes.HasPrefix(src[i:], bytes.Repeat([]byte{'"'}, q)) {
			return false
		}
		for k := 0; k < hashes; k++ {
			if i+q+k >= len(src) || src[i+q+k] != '#' {
				return false
			}
		}
		return true
	}
	for l.pos < len(src) {
		c := src[l.pos]
		switch {
		case c == '\n' && !multi:
			l.emit(tString, piece, "")
			return // unterminated: the line ends it
		case c == '\\' && l.escape(hashes):
			esc := l.pos
			l.pos += 1 + hashes
			if l.pos < len(src) && src[l.pos] == '(' {
				l.emitTo(tString, piece, esc, "")
				l.pos++
				l.emit(tPunct, esc, `\(`)
				for d := 1; d > 0 && l.pos < len(src); {
					d = l.next(d)
				}
				piece = l.pos
				continue
			}
			if l.pos < len(src) {
				l.advance(1)
			}
		case c == '"' && closing(l.pos):
			if multi {
				l.pos += 3
			} else {
				l.pos++
			}
			l.pos = min(l.pos+hashes, len(src))
			l.emit(tString, piece, "")
			return
		default:
			l.advance(1)
		}
	}
	l.emit(tString, piece, "")
}

// escape reports whether the backslash at l.pos starts an escape in a string with
// the given number of hashes: a raw string's escapes are \#, \##, ...
func (l *lexer) escape(hashes int) bool {
	for k := 1; k <= hashes; k++ {
		if l.pos+k >= len(l.src) || l.src[l.pos+k] != '#' {
			return false
		}
	}
	return true
}

func (l *lexer) number() {
	src, i := l.src, l.pos
	hex := bytes.HasPrefix(src[i:], []byte("0x")) || bytes.HasPrefix(src[i:], []byte("0X"))
	j := i
	for j < len(src) {
		switch c := src[j]; {
		case isIdentPart(c):
		case (c == '+' || c == '-') && exponent(src[j-1], hex):
		case c == '.' && j+1 < len(src) && (src[j+1] >= '0' && src[j+1] <= '9' || hex && isHexDigit(src[j+1])) && !bytes.Contains(src[i:j], []byte(".")):
		default:
			l.pos = j
			l.emit(tNumber, i, string(src[i:j]))
			return
		}
		j++
	}
	l.pos = j
	l.emit(tNumber, i, string(src[i:j]))
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

// regexStart reports whether the `/` at l.pos starts a regex literal rather than
// being division: it must not follow an operand, must not be followed by a space
// (`/ 2`) or `=` (`/=`), and must close with an unescaped `/` on the same line
// (outside a character class) that is not followed by an identifier character.
// Anything else is read as an operator, like the compiler does for `/`.
func (l *lexer) regexStart() bool {
	if n := len(l.out); n > 0 && !l.nl {
		if prev := l.out[n-1]; operandEnd(prev) {
			return false
		}
	}
	i := l.pos + 1
	if i >= len(l.src) || l.src[i] == ' ' || l.src[i] == '\t' || l.src[i] == '=' || l.src[i] == '\n' {
		return false
	}
	class := false
	for ; i < len(l.src) && l.src[i] != '\n'; i++ {
		switch c := l.src[i]; {
		case c == '\\':
			i++
		case c == '[':
			class = true
		case c == ']':
			class = false
		case c == '/' && !class:
			return l.src[i-1] != ' ' && (i+1 >= len(l.src) || !isIdentPart(l.src[i+1]))
		}
	}
	return false
}

// operandEnd reports whether a token can end an operand, after which `/` divides.
func operandEnd(t token) bool {
	switch t.kind {
	case tIdent:
		return t.bt || t.member || !keywords[t.text] || operandKeywords[t.text]
	case tString, tNumber, tRegex:
		return true
	case tPunct:
		return t.text == ")" || t.text == "]" || t.text == "}" || t.text == "?" || t.text == "!" || t.text == ">"
	}
	return false
}

// regex reads a regex literal at l.pos: /.../ on one line, or #/.../# (which may
// span lines) when hashes > 0.
func (l *lexer) regex(hashes int) {
	start := l.pos - hashes
	l.pos++
	class := false
	for l.pos < len(l.src) {
		c := l.src[l.pos]
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
		case c == '/' && (!class || hashes > 0) && bytes.HasPrefix(l.src[l.pos+1:], bytes.Repeat([]byte{'#'}, hashes)):
			l.pos += 1 + hashes
			l.emit(tRegex, start, "")
			return
		}
		l.advance(1)
	}
	l.emit(tRegex, start, "")
}

// lineEnds reports whether only spaces follow offset i on its line.
func lineEnds(src []byte, i int) bool {
	for ; i < len(src) && src[i] != '\n'; i++ {
		if src[i] != ' ' && src[i] != '\t' && src[i] != '\r' {
			return false
		}
	}
	return true
}
