package r

import "strings"

// Token kinds of the R lexer.
const (
	tIdent = iota + 1 // a name, a backquoted name (without its quotes) or a keyword
	tStr              // a string literal's content, raw strings included
	tNum
	tOp  // an operator or bracket
	tSep // the end of an R Markdown chunk: nothing continues across it
)

type tok struct {
	k    int
	s    string
	line int
	bol  bool // first token of its line
}

// comment is a comment's text from its "#", roxygen's "#'" included.
type comment struct {
	text string
	line int
}

// lexer reads R source into tokens. It knows what could otherwise hide or fake a
// token: comments, strings with escapes, raw strings (r"(...)", R"---[...]---"),
// backquoted names, numbers with exponents and suffixes (1e-3L, 0x1Fi) and the
// operators made of several characters (<<-, ->>, :::, %in%, |>).
type lexer struct {
	src      []byte
	i        int
	line     int
	bol      bool
	tokens   []tok
	comments []comment
}

// lex tokenizes src, numbering lines from line.
func lex(src []byte, line int) ([]tok, []comment) {
	l := &lexer{src: src, line: line, bol: true}
	l.run()
	return l.tokens, l.comments
}

func identStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c >= 0x80
}

func identPart(c byte) bool {
	return identStart(c) || c >= '0' && c <= '9' || c == '_'
}

func digit(c byte) bool { return c >= '0' && c <= '9' }

func (l *lexer) emit(k int, s string, line int) {
	l.tokens = append(l.tokens, tok{k: k, s: s, line: line, bol: l.bol})
	l.bol = false
}

func (l *lexer) peek(off int) byte {
	if l.i+off < len(l.src) {
		return l.src[l.i+off]
	}
	return 0
}

// ops are the operators of more than one character, longest first.
var ops = []string{"<<-", "->>", ":::", "<-", "->", "::", ":=", "==", "!=", "<=", ">=", "&&", "||", "|>"}

func (l *lexer) run() {
	src := l.src
	// cSpell: ignore ufeff
	if strings.HasPrefix(string(src), "\ufeff") {
		l.i = 3
	}
	if l.i == 0 && len(src) > 1 && src[0] == '#' && src[1] == '!' {
		l.skipLine()
	}
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.bol = true
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '#':
			start, line := l.i, l.line
			l.skipLine()
			l.comments = append(l.comments, comment{text: string(src[start:l.i]), line: line})
		case (c == 'r' || c == 'R') && (l.peek(1) == '"' || l.peek(1) == '\'') && l.rawString():
		case c == '"' || c == '\'':
			line := l.line
			l.emit(tStr, l.quoted(c), line)
		case c == '`':
			line := l.line
			l.emit(tIdent, l.quoted('`'), line)
		case digit(c) || c == '.' && digit(l.peek(1)):
			l.number()
		case identStart(c):
			start := l.i
			for l.i < len(src) && identPart(src[l.i]) {
				l.i++
			}
			l.emit(tIdent, string(src[start:l.i]), l.line)
		case c == '%':
			// %in%, %>%, %||%: up to the next % on the line.
			end := l.i + 1
			for end < len(src) && src[end] != '%' && src[end] != '\n' {
				end++
			}
			if end < len(src) && src[end] == '%' {
				l.emit(tOp, string(src[l.i:end+1]), l.line)
				l.i = end + 1
			} else {
				l.emit(tOp, "%", l.line)
				l.i++
			}
		default:
			op := string(c)
			for _, o := range ops {
				if strings.HasPrefix(string(src[l.i:min(len(src), l.i+3)]), o) {
					op = o
					break
				}
			}
			l.emit(tOp, op, l.line)
			l.i += len(op)
		}
	}
}

func (l *lexer) skipLine() {
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		l.i++
	}
}

// quoted reads a string or backquoted name from its opening quote, and returns its
// content with the common escapes undone.
func (l *lexer) quoted(q byte) string {
	src := l.src
	l.i++
	var b strings.Builder
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == q:
			l.i++
			return b.String()
		case c == '\\' && l.i+1 < len(src):
			n := src[l.i+1]
			switch n {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\n':
				l.line++
				b.WriteByte('\n')
			default:
				b.WriteByte(n)
			}
			l.i += 2
		default:
			if c == '\n' {
				l.line++
			}
			b.WriteByte(c)
			l.i++
		}
	}
	return b.String()
}

// rawString reads r"(...)" and its variants (brackets ( [ {, any number of dashes),
// reporting false when what follows the r is not one after all.
func (l *lexer) rawString() bool {
	src := l.src
	j := l.i + 2
	dashes := 0
	for j < len(src) && src[j] == '-' {
		j++
		dashes++
	}
	if j >= len(src) {
		return false
	}
	var closer byte
	switch src[j] {
	case '(':
		closer = ')'
	case '[':
		closer = ']'
	case '{':
		closer = '}'
	default:
		return false
	}
	end := string(closer) + strings.Repeat("-", dashes) + string(src[l.i+1])
	body := j + 1
	k := strings.Index(string(src[body:]), end)
	if k < 0 {
		k = len(src) - body
	}
	text := string(src[body : body+k])
	line := l.line
	l.line += strings.Count(text, "\n")
	l.i = min(len(src), body+k+len(end))
	l.emit(tStr, text, line)
	return true
}

func (l *lexer) number() {
	src := l.src
	start := l.i
	if src[l.i] == '0' && (l.peek(1) == 'x' || l.peek(1) == 'X') {
		l.i += 2
		for l.i < len(src) && (digit(src[l.i]) || src[l.i] >= 'a' && src[l.i] <= 'f' || src[l.i] >= 'A' && src[l.i] <= 'F') {
			l.i++
		}
	} else {
		for l.i < len(src) && (digit(src[l.i]) || src[l.i] == '.') {
			l.i++
		}
		if l.i < len(src) && (src[l.i] == 'e' || src[l.i] == 'E') {
			l.i++
			if l.i < len(src) && (src[l.i] == '+' || src[l.i] == '-') {
				l.i++
			}
			for l.i < len(src) && digit(src[l.i]) {
				l.i++
			}
		}
	}
	if l.i < len(src) && (src[l.i] == 'L' || src[l.i] == 'i') {
		l.i++
	}
	l.emit(tNum, string(src[start:l.i]), l.line)
}
