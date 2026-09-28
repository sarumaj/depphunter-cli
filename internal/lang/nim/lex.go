package nim

// Token kinds.
const (
	tIdent  = iota // an identifier or keyword; a backquoted name without its quotes
	tString        // any string literal; text is its content
	tChar          // a character literal
	tNumber        // a number with its type suffix
	tOp            // a run of operator characters
	tOpen          // ( [ {
	tClose         // ) ] }
	tComma         // ,
	tSemi          // ;
)

// token is one token of Nim source. start/end delimit it in the source (for a
// string: its content, without quotes and prefix). first marks a token that is
// the first on its line, depth the bracket depth before it.
type token struct {
	kind       int
	start, end int
	line, col  int
	depth      int
	first      bool
}

// lexer holds a source and its tokens.
type lexer struct {
	src    []byte
	tokens []token
}

func (l *lexer) text(t token) string { return string(l.src[t.start:t.end]) }

func (l *lexer) is(i int, kind int, text string) bool {
	if i < 0 || i >= len(l.tokens) {
		return false
	}
	t := l.tokens[i]
	return t.kind == kind && t.end-t.start == len(text) && string(l.src[t.start:t.end]) == text
}

func isIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentChar(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }

func isOpChar(c byte) bool {
	switch c {
	case '=', '+', '-', '*', '/', '<', '>', '@', '$', '~', '&', '%', '|', '!', '?', '^', '.', ':', '\\':
		return true
	}
	return false
}

// lex reads src into tokens: `#` comments, `##` doc comments and nested `#[ ]#`
// and `##[ ]##` block comments are skipped; "…" strings with escapes, r"…" and
// generalized raw strings (ident"…", where "" is a quote), """…""" strings with
// or without a prefix, 'x' characters, `quoted` names, numbers with 'suffixes,
// operator runs and brackets are tokens. Every construct ends at the end of the
// input, so any input is read in linear time.
//
// Implements: REQ-NIM-010
func lex(src []byte) *lexer {
	l := &lexer{src: src, tokens: make([]token, 0, len(src)/6+1)}
	line, lineStart, depth := 1, 0, 0
	first := true
	emit := func(kind, start, end, at int) {
		l.tokens = append(l.tokens, token{kind: kind, start: start, end: end, line: line, col: at - lineStart, depth: depth, first: first})
		first = false
	}
	// newlines counts the line breaks in src[from:to] into line/lineStart.
	// A token after a multi-line comment on a new line is first on it; one
	// after a multi-line string's closing quotes is not.
	newlines := func(from, to int, crossed bool) {
		for k := from; k < to; k++ {
			if src[k] == '\n' {
				line++
				lineStart = k + 1
				first = first || crossed
			}
		}
	}
	n := len(src)
	i := 0
	if n >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		i = 3
		lineStart = 3
	}
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
			lineStart = i
			first = true
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '#':
			j := i + 1
			if j < n && src[j] == '#' {
				j++
			}
			if j < n && src[j] == '[' {
				end := blockComment(src, j+1)
				newlines(i, end, true)
				i = end
				continue
			}
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '"':
			i = l.str(i, i, emit, newlines, false)
		case c == '\'':
			if k := len(l.tokens) - 1; k >= 0 && l.tokens[k].kind == tNumber && l.tokens[k].end == i {
				// A type suffix: 1'i32, 1.0'f32.
				j := i + 1
				for j < n && isIdentChar(src[j]) {
					j++
				}
				l.tokens[k].end = j
				i = j
				continue
			}
			if end, ok := charLit(src, i); ok {
				emit(tChar, i, end, i)
				i = end
				continue
			}
			emit(tOp, i, i+1, i)
			i++
		case c == '`':
			j := i + 1
			for j < n && src[j] != '`' && src[j] != '\n' {
				j++
			}
			if j < n && src[j] == '`' {
				emit(tIdent, i+1, j, i)
				i = j + 1
				continue
			}
			emit(tOp, i, i+1, i)
			i++
		case c >= '0' && c <= '9':
			j := i + 1
			for j < n && (isIdentChar(src[j]) || src[j] == '.' && j+1 < n && src[j+1] >= '0' && src[j+1] <= '9') {
				j++
			}
			emit(tNumber, i, j, i)
			i = j
		case isIdentStart(c):
			j := i + 1
			for j < n && isIdentChar(src[j]) {
				j++
			}
			if j < n && src[j] == '"' {
				// A raw string (r"…") or a generalized raw string literal (fmt"…").
				i = l.str(i, j, emit, newlines, true)
				continue
			}
			emit(tIdent, i, j, i)
			i = j
		case c == '(' || c == '[' || c == '{':
			emit(tOpen, i, i+1, i)
			depth++
			i++
		case c == ')' || c == ']' || c == '}':
			if depth > 0 {
				depth--
			}
			emit(tClose, i, i+1, i)
			i++
		case c == ',':
			emit(tComma, i, i+1, i)
			i++
		case c == ';':
			emit(tSemi, i, i+1, i)
			i++
		case isOpChar(c):
			j := i + 1
			for j < n && isOpChar(src[j]) {
				j++
			}
			emit(tOp, i, j, i)
			i = j
		default:
			i++
		}
	}
	return l
}

// blockComment returns the end of a block comment whose opening `#[` ends at i;
// block comments nest.
func blockComment(src []byte, i int) int {
	nest := 1
	for i < len(src) {
		switch {
		case src[i] == '#' && i+1 < len(src) && src[i+1] == '[':
			nest++
			i += 2
		case src[i] == ']' && i+1 < len(src) && src[i+1] == '#':
			nest--
			i += 2
			if nest == 0 {
				// `]##` closes a doc block comment.
				for i < len(src) && src[i] == '#' {
					i++
				}
				return i
			}
		default:
			i++
		}
	}
	return len(src)
}

// str reads a string literal whose quote is at q (at is where the token starts,
// its prefix for a raw one) and returns the index after it. A triple-quoted
// string runs to the last of three or more quotes and has no escapes; a raw
// string doubles a quote to write one; a plain one has backslash escapes. A
// single-line string ends at the end of its line when unterminated.
func (l *lexer) str(at, q int, emit func(kind, start, end, at int), newlines func(from, to int, crossed bool), raw bool) int {
	src := l.src
	n := len(src)
	if q+2 < n && src[q+1] == '"' && src[q+2] == '"' {
		start := q + 3
		j := start
		for j < n {
			if src[j] == '"' && j+2 < n && src[j+1] == '"' && src[j+2] == '"' {
				k := j + 3
				for k < n && src[k] == '"' {
					k++
				}
				emit(tString, start, k-3, at)
				newlines(start, k, false)
				return k
			}
			j++
		}
		emit(tString, start, n, at)
		newlines(start, n, false)
		return n
	}
	start := q + 1
	j := start
	for j < n && src[j] != '\n' {
		switch {
		case src[j] == '\\' && !raw:
			if j+1 < n && src[j+1] == '\n' {
				emit(tString, start, j, at)
				return j + 1
			}
			j += 2
			continue
		case src[j] == '"':
			if raw && j+1 < n && src[j+1] == '"' {
				j += 2
				continue
			}
			emit(tString, start, j, at)
			return j + 1
		}
		j++
	}
	if j > n {
		j = n
	}
	emit(tString, start, j, at)
	return j
}

// charLit returns the end of the character literal at i: 'x', '\n', '\x41',
// '\255', '\” or a UTF-8 character, closed on the same line within a few bytes.
func charLit(src []byte, i int) (int, bool) {
	n := len(src)
	j := i + 1
	if j >= n || src[j] == '\n' {
		return 0, false
	}
	if src[j] == '\\' {
		for k := j + 2; k < n && k <= j+12 && src[k] != '\n'; k++ {
			if src[k] == '\'' {
				return k + 1, true
			}
		}
		return 0, false
	}
	for k := j + 1; k < n && k <= j+4 && src[k] != '\n'; k++ {
		if src[k] == '\'' {
			return k + 1, true
		}
	}
	return 0, false
}
