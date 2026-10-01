package r

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// Token kinds of the R lexer.
const (
	tIdentifier = iota + 1 // a name, a backquoted name (without its quotes) or a keyword
	tString                // a string literal's content, raw strings included
	tNumber
	tOp        // an operator or bracket
	tSeparator // the end of an R Markdown chunk: nothing continues across it
)

type token struct {
	k         int
	s         string
	line      int
	lineStart bool // first token of its line
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
	source    []byte
	i         int
	line      int
	lineStart bool
	tokens    []token
	comments  []comment
}

// lex tokenizes source, numbering lines from line.
func lex(source []byte, line int) ([]token, []comment) {
	l := &lexer{source: source, line: line, lineStart: true}
	l.run()
	return l.tokens, l.comments
}

func identifierStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c >= 0x80
}

func identifierPart(c byte) bool {
	return identifierStart(c) || c >= '0' && c <= '9' || c == '_'
}

func (l *lexer) emit(k int, s string, line int) {
	l.tokens = append(l.tokens, token{k: k, s: s, line: line, lineStart: l.lineStart})
	l.lineStart = false
}

func (l *lexer) peek(off int) byte {
	if l.i+off < len(l.source) {
		return l.source[l.i+off]
	}
	return 0
}

// operators are the operators of more than one character, longest first.
var operators = []string{"<<-", "->>", ":::", "<-", "->", "::", ":=", "==", "!=", "<=", ">=", "&&", "||", "|>"}

func (l *lexer) run() {
	source := l.source
	// cSpell: ignore ufeff
	if strings.HasPrefix(string(source), "\ufeff") {
		l.i = 3
	}
	if l.i == 0 && len(source) > 1 && source[0] == '#' && source[1] == '!' {
		l.skipLine()
	}
	for l.i < len(source) {
		c := source[l.i]
		switch {
		case c == '\n':
			l.line++
			l.lineStart = true
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '#':
			start, line := l.i, l.line
			l.skipLine()
			l.comments = append(l.comments, comment{text: string(source[start:l.i]), line: line})
		case (c == 'r' || c == 'R') && (l.peek(1) == '"' || l.peek(1) == '\'') && l.rawString():
		case c == '"' || c == '\'':
			line := l.line
			l.emit(tString, l.quoted(c), line)
		case c == '`':
			line := l.line
			l.emit(tIdentifier, l.quoted('`'), line)
		case chars.IsDigit(c) || c == '.' && chars.IsDigit(l.peek(1)):
			l.number()
		case identifierStart(c):
			start := l.i
			for l.i < len(source) && identifierPart(source[l.i]) {
				l.i++
			}
			l.emit(tIdentifier, string(source[start:l.i]), l.line)
		case c == '%':
			// %in%, %>%, %||%: up to the next % on the line.
			end := l.i + 1
			for end < len(source) && source[end] != '%' && source[end] != '\n' {
				end++
			}
			if end < len(source) && source[end] == '%' {
				l.emit(tOp, string(source[l.i:end+1]), l.line)
				l.i = end + 1
			} else {
				l.emit(tOp, "%", l.line)
				l.i++
			}
		default:
			operator := string(c)
			for _, o := range operators {
				if strings.HasPrefix(string(source[l.i:min(len(source), l.i+3)]), o) {
					operator = o
					break
				}
			}
			l.emit(tOp, operator, l.line)
			l.i += len(operator)
		}
	}
}

func (l *lexer) skipLine() {
	for l.i < len(l.source) && l.source[l.i] != '\n' {
		l.i++
	}
}

// quoted reads a string or backquoted name from its opening quote, and returns its
// content with the common escapes undone.
func (l *lexer) quoted(q byte) string {
	source := l.source
	l.i++
	var b strings.Builder
	for l.i < len(source) {
		c := source[l.i]
		switch {
		case c == q:
			l.i++
			return b.String()
		case c == '\\' && l.i+1 < len(source):
			n := source[l.i+1]
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
	source := l.source
	j := l.i + 2
	dashes := 0
	for j < len(source) && source[j] == '-' {
		j++
		dashes++
	}
	if j >= len(source) {
		return false
	}
	var closer byte
	switch source[j] {
	case '(':
		closer = ')'
	case '[':
		closer = ']'
	case '{':
		closer = '}'
	default:
		return false
	}
	end := string(closer) + strings.Repeat("-", dashes) + string(source[l.i+1])
	body := j + 1
	k := strings.Index(string(source[body:]), end)
	if k < 0 {
		k = len(source) - body
	}
	text := string(source[body : body+k])
	line := l.line
	l.line += strings.Count(text, "\n")
	l.i = min(len(source), body+k+len(end))
	l.emit(tString, text, line)
	return true
}

func (l *lexer) number() {
	source := l.source
	start := l.i
	if source[l.i] == '0' && (l.peek(1) == 'x' || l.peek(1) == 'X') {
		l.i += 2
		for l.i < len(source) && (chars.IsDigit(source[l.i]) || source[l.i] >= 'a' && source[l.i] <= 'f' || source[l.i] >= 'A' && source[l.i] <= 'F') {
			l.i++
		}
	} else {
		for l.i < len(source) && (chars.IsDigit(source[l.i]) || source[l.i] == '.') {
			l.i++
		}
		if l.i < len(source) && (source[l.i] == 'e' || source[l.i] == 'E') {
			l.i++
			if l.i < len(source) && (source[l.i] == '+' || source[l.i] == '-') {
				l.i++
			}
			for l.i < len(source) && chars.IsDigit(source[l.i]) {
				l.i++
			}
		}
	}
	if l.i < len(source) && (source[l.i] == 'L' || source[l.i] == 'i') {
		l.i++
	}
	l.emit(tNumber, string(source[start:l.i]), l.line)
}
