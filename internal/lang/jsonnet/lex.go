package jsonnet

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

type kind uint8

const (
	tIdentifier kind = iota
	tString
	tNumber
	tPunctuation
)

type token struct {
	kind kind
	text string // an identifier, the punctuation character, or a string's value
	line int
}

// lex splits Jsonnet source into tokens. Comments (//, # and /* */) are
// dropped; strings ("", ”, verbatim @"" and @”, and ||| text blocks) are
// one token each with their value, so nothing inside them reads as code.
// Unterminated constructs end at the end of the file.
//
// Implements: REQ-JSONNET-009
func lex(source []byte) []token {
	s := string(source)
	tokens := make([]token, 0, len(s)/6)
	line := 1
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '#' || c == '/' && i+1 < len(s) && s[i+1] == '/':
			i = chars.LineEnd(s, i)
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				end = len(s) - i - 2
			} else {
				end += 2
			}
			line += strings.Count(s[i:i+2+end], "\n")
			i += 2 + end
		case c == '"' || c == '\'':
			start := line
			v, j := quoted(s, i+1, c)
			line += strings.Count(s[i:j], "\n")
			tokens = append(tokens, token{tString, v, start})
			i = j
		case c == '@' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\''):
			start := line
			v, j := verbatim(s, i+2, s[i+1])
			line += strings.Count(s[i:j], "\n")
			tokens = append(tokens, token{tString, v, start})
			i = j
		case c == '|' && strings.HasPrefix(s[i:], "|||"):
			start := line
			v, j := textBlock(s, i+3)
			line += strings.Count(s[i:j], "\n")
			tokens = append(tokens, token{tString, v, start})
			i = j
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(s) && chars.IsWord(s[j]) {
				j++
			}
			tokens = append(tokens, token{tIdentifier, s[i:j], line})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(s) && (chars.IsWord(s[j]) || s[j] == '.' || (s[j] == '-' || s[j] == '+') && (s[j-1] == 'e' || s[j-1] == 'E')) {
				j++
			}
			tokens = append(tokens, token{tNumber, s[i:j], line})
			i = j
		default:
			tokens = append(tokens, token{tPunctuation, s[i : i+1], line})
			i++
		}
	}
	return tokens
}

// quoted reads a string from i (after its opening quote q) and returns its
// value and the offset after the closing quote.
func quoted(s string, i int, q byte) (string, int) {
	var b strings.Builder
	for i < len(s) {
		c := s[i]
		switch {
		case c == q:
			return b.String(), i + 1
		case c == '\\' && i+1 < len(s):
			switch e := s[i+1]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r', 'b', 'f':
			case 'u':
				i += 4 // the code point is not needed to find a file
			default:
				b.WriteByte(e)
			}
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), len(s)
}

// verbatim reads @"..." from i: no escapes, a doubled quote is one quote.
func verbatim(s string, i int, q byte) (string, int) {
	var b strings.Builder
	for i < len(s) {
		if s[i] == q {
			if i+1 < len(s) && s[i+1] == q {
				b.WriteByte(q)
				i += 2
				continue
			}
			return b.String(), i + 1
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), len(s)
}

// textBlock reads a ||| block from i (after the opening |||): the lines
// indented at least as far as the first one, up to the closing |||.
func textBlock(s string, i int) (string, int) {
	if i < len(s) && s[i] == '-' {
		i++
	}
	newline := strings.IndexByte(s[i:], '\n')
	if newline < 0 {
		return "", len(s)
	}
	i += newline + 1
	// The first line's leading whitespace is the block's indentation.
	j := i
	for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
		j++
	}
	indent := s[i:j]
	var b strings.Builder
	for i < len(s) {
		if s[i] == '\n' {
			b.WriteByte('\n')
			i++
			continue
		}
		if indent == "" || !strings.HasPrefix(s[i:], indent) {
			break
		}
		e := strings.IndexByte(s[i:], '\n')
		if e < 0 {
			b.WriteString(s[i+len(indent):])
			return b.String(), len(s)
		}
		b.WriteString(s[i+len(indent) : i+e+1])
		i += e + 1
	}
	// The terminating line: optional whitespace, then |||.
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if strings.HasPrefix(s[i:], "|||") {
		i += 3
	}
	return b.String(), i
}

// match pairs each opening bracket with its closer (-1 when unclosed). A
// closer only closes an opener within the 8 innermost open brackets, so a
// stray closer does not unwind the whole file.
func match(tokens []token) []int {
	m := make([]int, len(tokens))
	var stack []int
	for i, t := range tokens {
		m[i] = -1
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case "{", "[", "(":
			stack = append(stack, i)
		case "}", "]", ")":
			open := "("
			if t.text == "}" {
				open = "{"
			} else if t.text == "]" {
				open = "["
			}
			for k := len(stack) - 1; k >= 0 && k >= len(stack)-8; k-- {
				if tokens[stack[k]].text == open {
					m[stack[k]] = i
					m[i] = stack[k]
					stack = stack[:k]
					break
				}
			}
		}
	}
	return m
}
