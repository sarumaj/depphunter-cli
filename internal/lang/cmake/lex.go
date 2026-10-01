package cmake

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// argument is one argument of a command invocation as written: its text with escapes
// decoded (an escaped ";" is kept as "\;" so it does not split a list), and whether
// it was quoted or a bracket argument, which variable references and list splitting
// treat differently.
type argument struct {
	text   string
	quoted bool // "..." - variables are expanded, the value is not split on ";"
	raw    bool // [[...]] - nothing is expanded or split
	line   int
}

// command is one command invocation, `name(args)`. Parentheses nested in the
// arguments (in if() conditions) are arguments "(" and ")" of their own.
type command struct {
	name      string // lower case: command names are case-insensitive
	cased     string // as written, for the import's spec
	line      int
	arguments []argument
}

// lex reads a CMake file's command invocations: comments (`#`, `#[[ ]]`), bracket
// arguments (`[==[ ]==]`), quoted arguments with escapes and line continuations,
// and unquoted ones. Anything else - a stray character outside a command - is
// skipped, so a damaged file still yields the commands it has.
//
// Implements: REQ-CMAKE-002, REQ-CMAKE-010
func lex(source []byte) []command {
	l := lexer{s: string(source), line: 1}
	if strings.HasPrefix(l.s, "\xef\xbb\xbf") {
		l.i = 3
	}
	var out []command
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == '#':
			l.comment()
		case chars.IsIdentStart(c):
			start, line := l.i, l.line
			for l.i < len(l.s) && chars.IsWord(l.s[l.i]) {
				l.i++
			}
			name := l.s[start:l.i]
			for l.i < len(l.s) && (l.s[l.i] == ' ' || l.s[l.i] == '\t') {
				l.i++
			}
			if l.i < len(l.s) && l.s[l.i] == '(' {
				l.i++
				out = append(out, command{name: strings.ToLower(name), cased: name, line: line, arguments: l.arguments()})
			}
		default:
			l.i++
		}
	}
	return out
}

type lexer struct {
	s    string
	i    int
	line int
}

// bracketOpen reports the number of "=" of a bracket opening `[=*[` at l.i, or -1.
func (l *lexer) bracketOpen() int {
	if l.i >= len(l.s) || l.s[l.i] != '[' {
		return -1
	}
	j := l.i + 1
	for j < len(l.s) && l.s[j] == '=' {
		j++
	}
	if j < len(l.s) && l.s[j] == '[' {
		return j - l.i - 1
	}
	return -1
}

// bracket reads a bracket argument or comment opening at l.i with n "=", returning
// its content; a newline right after the opening is not part of it.
func (l *lexer) bracket(n int) string {
	l.i += n + 2
	if strings.HasPrefix(l.s[l.i:], "\r\n") {
		l.i += 2
		l.line++
	} else if l.i < len(l.s) && l.s[l.i] == '\n' {
		l.i++
		l.line++
	}
	closing := "]" + strings.Repeat("=", n) + "]"
	end := strings.Index(l.s[l.i:], closing)
	if end < 0 {
		end = len(l.s) - l.i
	}
	text := l.s[l.i : l.i+end]
	l.line += strings.Count(text, "\n")
	l.i = min(len(l.s), l.i+end+len(closing))
	return text
}

// comment skips a comment starting at the "#" at l.i.
func (l *lexer) comment() {
	l.i++
	if n := l.bracketOpen(); n >= 0 {
		l.bracket(n)
		return
	}
	for l.i < len(l.s) && l.s[l.i] != '\n' {
		l.i++
	}
}

// arguments reads the arguments after a command's "(" up to its ")".
func (l *lexer) arguments() []argument {
	var out []argument
	depth := 0
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r':
			l.i++
		case c == '#':
			l.comment()
		case c == '(':
			depth++
			out = append(out, argument{text: "(", line: l.line})
			l.i++
		case c == ')':
			l.i++
			if depth == 0 {
				return out
			}
			depth--
			out = append(out, argument{text: ")", line: l.line})
		case c == '"':
			line := l.line
			l.i++
			out = append(out, argument{text: l.quoted(), quoted: true, line: line})
		default:
			if n := l.bracketOpen(); n >= 0 {
				line := l.line
				out = append(out, argument{text: l.bracket(n), raw: true, line: line})
				continue
			}
			out = append(out, argument{text: l.unquoted(), line: l.line})
		}
	}
	return out
}

// quoted reads a quoted argument's text after its opening quote.
func (l *lexer) quoted() string {
	var b strings.Builder
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch c {
		case '"':
			l.i++
			return b.String()
		case '\\':
			l.escape(&b, true)
			continue
		case '\n':
			l.line++
		}
		b.WriteByte(c)
		l.i++
	}
	return b.String()
}

// unquoted reads an unquoted argument; a quoted part inside one (a="b c", the
// legacy form) belongs to it.
func (l *lexer) unquoted() string {
	var b strings.Builder
	for l.i < len(l.s) {
		c := l.s[l.i]
		switch c {
		case ' ', '\t', '\r', '\n', '(', ')', '#':
			return b.String()
		case '\\':
			l.escape(&b, false)
			continue
		case '"':
			l.i++
			b.WriteString(l.quoted())
			continue
		}
		b.WriteByte(c)
		l.i++
	}
	return b.String()
}

// escape decodes the escape sequence at l.i into b. `\;` stays as written, so the
// argument is not split there; in a quoted argument a backslash before a newline
// continues the line.
func (l *lexer) escape(b *strings.Builder, quoted bool) {
	l.i++
	if l.i >= len(l.s) {
		return
	}
	c := l.s[l.i]
	l.i++
	switch c {
	case 'n':
		b.WriteByte('\n')
	case 't':
		b.WriteByte('\t')
	case 'r':
		b.WriteByte('\r')
	case ';':
		b.WriteString(`\;`)
	case '\n':
		l.line++
		if !quoted {
			b.WriteByte('\n')
		}
	default:
		b.WriteByte(c)
	}
}
