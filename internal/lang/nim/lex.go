package nim

// Token kinds.
const (
	tIdentifier = iota // an identifier or keyword; a backquoted name without its quotes
	tString            // any string literal; text is its content
	tCharacter         // a character literal
	tNumber            // a number with its type suffix
	tOp                // a run of operator characters
	tOpen              // ( [ {
	tClose             // ) ] }
	tComma             // ,
	tSemi              // ;
)

// token is one token of Nim source. start/end delimit it in the source (for a
// string: its content, without quotes and prefix). first marks a token that is
// the first on its line, depth the bracket depth before it.
type token struct {
	kind         int
	start, end   int
	line, column int
	depth        int
	first        bool
}

// lexer holds a source and its tokens.
type lexer struct {
	source []byte
	tokens []token
}

func (l *lexer) text(t token) string { return string(l.source[t.start:t.end]) }

func (l *lexer) is(i int, kind int, text string) bool {
	if i < 0 || i >= len(l.tokens) {
		return false
	}
	t := l.tokens[i]
	return t.kind == kind && t.end-t.start == len(text) && string(l.source[t.start:t.end]) == text
}

func isIdentifierStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentifierCharacter(c byte) bool { return isIdentifierStart(c) || c >= '0' && c <= '9' }

func isOpCharacter(c byte) bool {
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
func lex(source []byte) *lexer {
	l := &lexer{source: source, tokens: make([]token, 0, len(source)/6+1)}
	line, lineStart, depth := 1, 0, 0
	first := true
	emit := func(kind, start, end, at int) {
		l.tokens = append(l.tokens, token{kind: kind, start: start, end: end, line: line, column: at - lineStart, depth: depth, first: first})
		first = false
	}
	// newlines counts the line breaks in src[from:to] into line/lineStart.
	// A token after a multi-line comment on a new line is first on it; one
	// after a multi-line string's closing quotes is not.
	newlines := func(from, to int, crossed bool) {
		for k := from; k < to; k++ {
			if source[k] == '\n' {
				line++
				lineStart = k + 1
				first = first || crossed
			}
		}
	}
	n := len(source)
	i := 0
	if n >= 3 && source[0] == 0xEF && source[1] == 0xBB && source[2] == 0xBF {
		i = 3
		lineStart = 3
	}
	for i < n {
		c := source[i]
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
			if j < n && source[j] == '#' {
				j++
			}
			if j < n && source[j] == '[' {
				end := blockComment(source, j+1)
				newlines(i, end, true)
				i = end
				continue
			}
			for i < n && source[i] != '\n' {
				i++
			}
		case c == '"':
			i = l.readString(i, i, emit, newlines, false)
		case c == '\'':
			if k := len(l.tokens) - 1; k >= 0 && l.tokens[k].kind == tNumber && l.tokens[k].end == i {
				// A type suffix: 1'i32, 1.0'f32.
				j := i + 1
				for j < n && isIdentifierCharacter(source[j]) {
					j++
				}
				l.tokens[k].end = j
				i = j
				continue
			}
			if end, ok := characterLiteral(source, i); ok {
				emit(tCharacter, i, end, i)
				i = end
				continue
			}
			emit(tOp, i, i+1, i)
			i++
		case c == '`':
			j := i + 1
			for j < n && source[j] != '`' && source[j] != '\n' {
				j++
			}
			if j < n && source[j] == '`' {
				emit(tIdentifier, i+1, j, i)
				i = j + 1
				continue
			}
			emit(tOp, i, i+1, i)
			i++
		case c >= '0' && c <= '9':
			j := i + 1
			for j < n && (isIdentifierCharacter(source[j]) || source[j] == '.' && j+1 < n && source[j+1] >= '0' && source[j+1] <= '9') {
				j++
			}
			emit(tNumber, i, j, i)
			i = j
		case isIdentifierStart(c):
			j := i + 1
			for j < n && isIdentifierCharacter(source[j]) {
				j++
			}
			if j < n && source[j] == '"' {
				// A raw string (r"…") or a generalized raw string literal (fmt"…").
				i = l.readString(i, j, emit, newlines, true)
				continue
			}
			emit(tIdentifier, i, j, i)
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
		case isOpCharacter(c):
			j := i + 1
			for j < n && isOpCharacter(source[j]) {
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
func blockComment(source []byte, i int) int {
	nest := 1
	for i < len(source) {
		switch {
		case source[i] == '#' && i+1 < len(source) && source[i+1] == '[':
			nest++
			i += 2
		case source[i] == ']' && i+1 < len(source) && source[i+1] == '#':
			nest--
			i += 2
			if nest == 0 {
				// `]##` closes a doc block comment.
				for i < len(source) && source[i] == '#' {
					i++
				}
				return i
			}
		default:
			i++
		}
	}
	return len(source)
}

// readString reads a string literal whose quote is at q (at is where the token starts,
// its prefix for a raw one) and returns the index after it. A triple-quoted
// string runs to the last of three or more quotes and has no escapes; a raw
// string doubles a quote to write one; a plain one has backslash escapes. A
// single-line string ends at the end of its line when unterminated.
func (l *lexer) readString(at, q int, emit func(kind, start, end, at int), newlines func(from, to int, crossed bool), raw bool) int {
	source := l.source
	n := len(source)
	if q+2 < n && source[q+1] == '"' && source[q+2] == '"' {
		start := q + 3
		j := start
		for j < n {
			if source[j] == '"' && j+2 < n && source[j+1] == '"' && source[j+2] == '"' {
				k := j + 3
				for k < n && source[k] == '"' {
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
	for j < n && source[j] != '\n' {
		switch {
		case source[j] == '\\' && !raw:
			if j+1 < n && source[j+1] == '\n' {
				emit(tString, start, j, at)
				return j + 1
			}
			j += 2
			continue
		case source[j] == '"':
			if raw && j+1 < n && source[j+1] == '"' {
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

// characterLiteral returns the end of the character literal at i: 'x', '\n', '\x41',
// '\255', '\” or a UTF-8 character, closed on the same line within a few bytes.
func characterLiteral(source []byte, i int) (int, bool) {
	n := len(source)
	j := i + 1
	if j >= n || source[j] == '\n' {
		return 0, false
	}
	if source[j] == '\\' {
		for k := j + 2; k < n && k <= j+12 && source[k] != '\n'; k++ {
			if source[k] == '\'' {
				return k + 1, true
			}
		}
		return 0, false
	}
	for k := j + 1; k < n && k <= j+4 && source[k] != '\n'; k++ {
		if source[k] == '\'' {
			return k + 1, true
		}
	}
	return 0, false
}
