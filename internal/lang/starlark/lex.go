// Package starlark reads Starlark, the Python dialect Bazel's BUILD, .bzl,
// WORKSPACE and MODULE.bazel files are written in, far enough to see what they
// declare: calls with their positional and keyword arguments, assignments,
// function definitions, and the literals (strings, lists, dicts, tuples) the
// arguments are made of. It is shared by the bazel plugin and the index, which
// reads a registry's MODULE.bazel files; it is not a plugin itself.
//
// The reader is a tolerant scanner, not the vendored tree-sitter grammar
// (REQ-BAZEL-012): it never fails, and what it cannot read becomes an "other"
// value rather than an error.
package starlark

import (
	"strings"
	"unicode/utf8"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// Token kinds.
const (
	tIdentifier = iota + 1
	tString
	tNumber
	tPunctuation
)

type token struct {
	kind   int
	text   string // identifier, punctuation, number, or a string's value
	line   int
	column int  // byte column of the token on its line
	first  bool // first token of a logical line (not inside brackets)
}

// lex splits source into tokens. Comments, blank lines and line breaks inside
// brackets or after a backslash vanish; the first token of each logical line is
// marked, with its column, which is all the indentation a reader of declarations
// needs. Unterminated strings end at the line (or, for a triple-quoted one, the
// file) and unbalanced brackets are tolerated.
func lex(source []byte) []token {
	s := string(source)
	s = strings.TrimPrefix(s, "\xef\xbb\xbf")
	var tokens []token
	line, lineStart := 1, 0
	depth := 0
	first := true
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
			lineStart = i
			if depth == 0 {
				first = true
			}
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			i++
			continue
		case c == '\\' && i+1 < len(s) && (s[i+1] == '\n' || s[i+1] == '\r'):
			i++ // a line continuation: the break that follows does not end the line
			if s[i] == '\r' {
				i++
			}
			if i < len(s) && s[i] == '\n' {
				i++
				line++
				lineStart = i
			}
			continue
		case c == '#':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		t := token{line: line, column: i - lineStart, first: first}
		first = false
		switch {
		case c == '"' || c == '\'':
			var v string
			v, i, line, lineStart = lexString(s, i, false, line, lineStart)
			t.kind, t.text = tString, v
		case chars.IsIdentStartUTF8(c):
			j := i
			for j < len(s) && chars.IsIdentUTF8(s[j]) {
				j++
			}
			word := s[i:j]
			if j < len(s) && (s[j] == '"' || s[j] == '\'') && stringPrefix(word) {
				raw := strings.ContainsAny(word, "rR")
				var v string
				v, i, line, lineStart = lexString(s, j, raw, line, lineStart)
				t.kind, t.text = tString, v
				break
			}
			t.kind, t.text = tIdentifier, word
			i = j
		case c >= '0' && c <= '9' || c == '.' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9':
			j := i + 1
			for j < len(s) && (chars.IsIdentUTF8(s[j]) || s[j] == '.' ||
				(s[j] == '+' || s[j] == '-') && (s[j-1] == 'e' || s[j-1] == 'E') && !strings.HasPrefix(strings.ToLower(s[i:j]), "0x")) {
				j++
			}
			t.kind, t.text = tNumber, s[i:j]
			i = j
		default:
			n := punctuation(s[i:])
			t.kind, t.text = tPunctuation, s[i:i+n]
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				if depth > 0 {
					depth--
				}
			}
			i += n
		}
		tokens = append(tokens, t)
	}
	return tokens
}

// stringPrefix reports whether an identifier directly before a quote is a string
// prefix: r (raw), b (bytes), or both.
func stringPrefix(w string) bool {
	switch strings.ToLower(w) {
	case "r", "b", "rb", "br":
		return true
	}
	return false
}

// punctuation is the length of the operator at the start of s: the longest of the
// three-, two- and one-character operators, or one byte (or rune) of anything else.
func punctuation(s string) int {
	for _, operator := range []string{"//=", ">>=", "<<=", "**=", "...", "==", "!=", "<=", ">=", "//", "**", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>", "->"} {
		if strings.HasPrefix(s, operator) {
			return len(operator)
		}
	}
	if s[0] >= 0x80 {
		_, n := utf8.DecodeRuneInString(s)
		return max(n, 1)
	}
	return 1
}

// lexString reads the string literal whose opening quote is at s[i], returning its value
// and the position after it. Escapes are decoded where they matter for names
// (\\, \", \', \n, \t); a raw string keeps its backslashes.
func lexString(s string, i int, raw bool, line, lineStart int) (string, int, int, int) {
	q := s[i]
	triple := strings.HasPrefix(s[i:], string([]byte{q, q, q}))
	if triple {
		i += 3
	} else {
		i++
	}
	var b strings.Builder
	for i < len(s) {
		c := s[i]
		switch {
		case triple && c == q && strings.HasPrefix(s[i:], string([]byte{q, q, q})):
			return b.String(), i + 3, line, lineStart
		case !triple && c == q:
			return b.String(), i + 1, line, lineStart
		case c == '\n':
			if !triple {
				return b.String(), i, line, lineStart // unterminated: ends at the line
			}
			b.WriteByte(c)
			i++
			line++
			lineStart = i
			continue
		case c == '\\' && i+1 < len(s):
			n := s[i+1]
			if raw {
				b.WriteByte(c)
				b.WriteByte(n)
			} else {
				switch n {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case '\n':
					// an escaped line break continues the string
				default:
					b.WriteByte(n)
				}
			}
			if n == '\n' {
				line++
				lineStart = i + 2
			}
			i += 2
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String(), i, line, lineStart
}
