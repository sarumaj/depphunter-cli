package zig

import "strings"

// Token kinds of the Zig lexer. Keywords are identifiers; the reader tells them
// apart.
const (
	tIdent   = iota // also @"quoted identifiers", whose text is what is quoted
	tBuiltin        // @import, @embedFile...; text keeps the @
	tStr            // a string literal ("..." or a \\ line); text is its value
	tChar
	tNum
	tPunct
)

type tok struct {
	kind int
	text string
	line int
}

// operators are the punctuation longer than one byte, longest first where one is a
// prefix of another; the reader only needs "=" told apart from "==", "=>", "<=".
var operators = []string{
	"<<|=", "<<=", ">>=", "+%=", "-%=", "*%=", "+|=", "-|=", "*|=", "<<|", "...",
	"==", "!=", "<=", ">=", "=>", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"<<", ">>", "++", "**", "||", "+%", "-%", "*%", "+|", "-|", "*|", "..", ".*", ".?",
}

// opStart are the first bytes of the operators.
const opStart = "<>+-*=!/%&|^."

// lex splits Zig source into tokens. Zig's lexical grammar is small: line comments
// only, strings that cannot span lines, multi-line string lines starting with \\,
// character literals, @"quoted" identifiers and @builtins. It never fails: what it
// does not know is a one-byte punctuation token.
//
// Implements: REQ-ZIG-012
func lex(src []byte) []tok {
	tokens := make([]tok, 0, len(src)/5+16)
	text := string(src) // token texts are slices of one copy, not one allocation each
	i, line := 0, 1
	if len(src) >= 3 && src[0] == 0xef && src[1] == 0xbb && src[2] == 0xbf {
		i = 3
	}
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '\\' && i+1 < n && src[i+1] == '\\':
			j := i + 2
			for j < n && src[j] != '\n' {
				j++
			}
			tokens = append(tokens, tok{tStr, strings.TrimSuffix(text[i+2:j], "\r"), line})
			i = j
		case c == '"':
			s, j := quoted(src, i+1, '"')
			tokens = append(tokens, tok{tStr, s, line})
			i = j
		case c == '\'':
			j := skipQuoted(src, i+1, '\'')
			tokens = append(tokens, tok{tChar, text[i:j], line})
			i = j
		case c == '@' && i+1 < n && src[i+1] == '"':
			s, j := quoted(src, i+2, '"')
			tokens = append(tokens, tok{tIdent, s, line})
			i = j
		case c == '@' && i+1 < n && identStart(src[i+1]):
			j := i + 1
			for j < n && identPart(src[j]) {
				j++
			}
			tokens = append(tokens, tok{tBuiltin, text[i:j], line})
			i = j
		case identStart(c):
			j := i
			for j < n && identPart(src[j]) {
				j++
			}
			tokens = append(tokens, tok{tIdent, text[i:j], line})
			i = j
		case c >= '0' && c <= '9':
			j := number(src, i)
			tokens = append(tokens, tok{tNum, text[i:j], line})
			i = j
		default:
			j := i + 1
			if strings.IndexByte(opStart, c) >= 0 {
				for _, op := range operators {
					if op[0] == c && strings.HasPrefix(text[i:], op) {
						j = i + len(op)
						break
					}
				}
			}
			tokens = append(tokens, tok{tPunct, text[i:j], line})
			i = j
		}
	}
	return tokens
}

// skipQuoted is quoted without the value.
func skipQuoted(src []byte, i int, q byte) int {
	for i < len(src) {
		switch c := src[i]; {
		case c == q:
			return i + 1
		case c == '\n':
			return i
		case c == '\\' && i+1 < len(src) && src[i+1] != '\n':
			i += 2
		default:
			i++
		}
	}
	return i
}

// quoted reads a string or character literal body from i (after the opening
// quote) to the closing quote, which it steps over. A literal cannot span lines:
// one cut short ends before the line break (or at the end of the source).
func quoted(src []byte, i int, q byte) (string, int) {
	var b strings.Builder
	for i < len(src) {
		c := src[i]
		switch {
		case c == q:
			return b.String(), i + 1
		case c == '\n':
			return b.String(), i
		case c == '\\' && i+1 < len(src) && src[i+1] != '\n':
			switch e := src[i+1]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(e) // \" \\ \' and, verbatim, \x \u which paths do not use
			}
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), i
}

// number reads a numeric literal: 0x1F, 1_000, 1.5e-3, 0x1p-2, but not the range
// in 0..5.
func number(src []byte, i int) int {
	start := i
	hex := i+1 < len(src) && src[i] == '0' && (src[i+1] == 'x' || src[i+1] == 'X')
	for i < len(src) {
		c := src[i]
		switch {
		case identPart(c):
			i++
		case c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
			i++
		case (c == '+' || c == '-') && i > start:
			p := src[i-1]
			if p == 'p' || p == 'P' || (!hex && (p == 'e' || p == 'E')) {
				i++
				continue
			}
			return i
		default:
			return i
		}
	}
	return i
}

func identStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func identPart(c byte) bool { return identStart(c) || (c >= '0' && c <= '9') }

// match links each opening bracket to its closing one and back (-1 when
// unbalanced). A closer that does not fit the innermost opener closes the nearest
// one of its kind within a few frames, so one stray bracket does not unbalance the
// rest of the file.
func match(tokens []tok) []int {
	m := make([]int, len(tokens))
	var stack []int
	for i, t := range tokens {
		m[i] = -1
		if t.kind != tPunct || len(t.text) != 1 {
			continue
		}
		switch c := t.text[0]; c {
		case '(', '[', '{':
			stack = append(stack, i)
		case ')', ']', '}':
			open := "("
			if c == ']' {
				open = "["
			} else if c == '}' {
				open = "{"
			}
			for k := len(stack) - 1; k >= 0 && k >= len(stack)-8; k-- {
				if tokens[stack[k]].text == open {
					m[i], m[stack[k]] = stack[k], i
					stack = stack[:k]
					break
				}
			}
		}
	}
	return m
}
