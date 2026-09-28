package cue

import "strings"

type kind uint8

const (
	tIdent kind = iota
	tString
	tNumber
	tPunct
)

type token struct {
	kind  kind
	text  string // an identifier, the punctuation character, or a string's text
	line  int
	first bool // the first token on its line
}

// maxInterp bounds how deeply string interpolations nest.
const maxInterp = 64

// lex splits CUE source into tokens. Comments (//) are dropped; strings
// ("", ”, """ """, ”' ”' and their #-delimited raw forms) are one token
// each, interpolations \( ... ) included, so nothing inside them reads as
// code. Unterminated constructs end at the end of the file.
//
// Implements: REQ-CUE-009
func lex(src []byte) []token {
	s := string(src)
	tokens := make([]token, 0, len(s)/6)
	line := 1
	first := true
	add := func(t token) {
		t.first = first
		first = false
		tokens = append(tokens, t)
	}
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\n':
			line++
			first = true
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '"' || c == '\'' || c == '#' && rawStart(s, i):
			start := i
			v, j := str(s, i, 0)
			add(token{tString, v, line, false})
			line += strings.Count(s[start:j], "\n")
			i = j
		case c == '_' || c == '$' || c == '#' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(s) && identByte(s[j]) {
				j++
			}
			add(token{tIdent, s[i:j], line, false})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(s) && (identByte(s[j]) || s[j] == '.' && j+1 < len(s) && s[j+1] >= '0' && s[j+1] <= '9') {
				j++
			}
			add(token{tNumber, s[i:j], line, false})
			i = j
		case c >= 0x80:
			j := i + 1 // a letter outside ASCII starts an identifier too
			for j < len(s) && (s[j] >= 0x80 || identByte(s[j])) {
				j++
			}
			add(token{tIdent, s[i:j], line, false})
			i = j
		default:
			add(token{tPunct, s[i : i+1], line, false})
			i++
		}
	}
	return tokens
}

func identByte(c byte) bool {
	return c == '_' || c == '$' || c == '#' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// rawStart reports whether the # at i opens a raw string (#"...", ##'...').
func rawStart(s string, i int) bool {
	for i < len(s) && s[i] == '#' {
		i++
	}
	return i < len(s) && (s[i] == '"' || s[i] == '\'')
}

// str reads the string starting at i (at its #s or quote) and returns its
// text (interpolations kept verbatim) and the offset after it.
func str(s string, i, depth int) (string, int) {
	hashes := 0
	for i < len(s) && s[i] == '#' {
		hashes++
		i++
	}
	if i >= len(s) {
		return "", len(s)
	}
	q := s[i : i+1]
	if strings.HasPrefix(s[i:], strings.Repeat(q, 3)) {
		q = strings.Repeat(q, 3)
	}
	i += len(q)
	closer := q + strings.Repeat("#", hashes)
	escape := "\\" + strings.Repeat("#", hashes)
	start := i
	for i < len(s) {
		if strings.HasPrefix(s[i:], closer) {
			return s[start:i], i + len(closer)
		}
		if strings.HasPrefix(s[i:], escape) {
			i += len(escape)
			if i < len(s) && s[i] == '(' {
				i = interpolate(s, i+1, depth+1)
			} else if i < len(s) {
				i++
			}
			continue
		}
		if len(q) == 1 && s[i] == '\n' {
			return s[start:i], i // a single-line string ends at its line
		}
		i++
	}
	return s[start:], len(s)
}

// interpolate skips an interpolation's code from i (after its "(") to the
// offset after its closing ")", reading the strings inside it.
func interpolate(s string, i, depth int) int {
	if depth > maxInterp {
		return len(s)
	}
	parens := 1
	for i < len(s) {
		switch c := s[i]; {
		case c == '(':
			parens++
		case c == ')':
			parens--
			if parens == 0 {
				return i + 1
			}
		case c == '"' || c == '\'' || c == '#' && rawStart(s, i):
			_, i = str(s, i, depth)
			continue
		case c == '\n':
			return i // CUE ends an unterminated interpolation's string at its line
		}
		i++
	}
	return len(s)
}

// match pairs each opening bracket with its closer (-1 when unclosed). A
// closer only closes an opener within the 8 innermost open brackets.
func match(tokens []token) []int {
	m := make([]int, len(tokens))
	var stack []int
	for i, t := range tokens {
		m[i] = -1
		if t.kind != tPunct {
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
