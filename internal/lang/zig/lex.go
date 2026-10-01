package zig

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// Token kinds of the Zig lexer. Keywords are identifiers; the reader tells them
// apart.
const (
	tIdentifier = iota // also @"quoted identifiers", whose text is what is quoted
	tBuiltin           // @import, @embedFile...; text keeps the @
	tString            // a string literal ("..." or a \\ line); text is its value
	tCharacter
	tNumber
	tPunctuation
)

type token struct {
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
func lex(source []byte) []token {
	tokens := make([]token, 0, len(source)/5+16)
	text := string(source) // token texts are slices of one copy, not one allocation each
	i, line := 0, 1
	if len(source) >= 3 && source[0] == 0xef && source[1] == 0xbb && source[2] == 0xbf {
		i = 3
	}
	n := len(source)
	for i < n {
		c := source[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < n && source[i+1] == '/':
			for i < n && source[i] != '\n' {
				i++
			}
		case c == '\\' && i+1 < n && source[i+1] == '\\':
			j := i + 2
			for j < n && source[j] != '\n' {
				j++
			}
			tokens = append(tokens, token{tString, strings.TrimSuffix(text[i+2:j], "\r"), line})
			i = j
		case c == '"':
			s, j := quoted(source, i+1, '"')
			tokens = append(tokens, token{tString, s, line})
			i = j
		case c == '\'':
			j := skipQuoted(source, i+1, '\'')
			tokens = append(tokens, token{tCharacter, text[i:j], line})
			i = j
		case c == '@' && i+1 < n && source[i+1] == '"':
			s, j := quoted(source, i+2, '"')
			tokens = append(tokens, token{tIdentifier, s, line})
			i = j
		case c == '@' && i+1 < n && chars.IsIdentStart(source[i+1]):
			j := i + 1
			for j < n && chars.IsWord(source[j]) {
				j++
			}
			tokens = append(tokens, token{tBuiltin, text[i:j], line})
			i = j
		case chars.IsIdentStart(c):
			j := i
			for j < n && chars.IsWord(source[j]) {
				j++
			}
			tokens = append(tokens, token{tIdentifier, text[i:j], line})
			i = j
		case c >= '0' && c <= '9':
			j := number(source, i)
			tokens = append(tokens, token{tNumber, text[i:j], line})
			i = j
		default:
			j := i + 1
			if strings.IndexByte(opStart, c) >= 0 {
				for _, operator := range operators {
					if operator[0] == c && strings.HasPrefix(text[i:], operator) {
						j = i + len(operator)
						break
					}
				}
			}
			tokens = append(tokens, token{tPunctuation, text[i:j], line})
			i = j
		}
	}
	return tokens
}

// skipQuoted is quoted without the value.
func skipQuoted(source []byte, i int, q byte) int {
	for i < len(source) {
		switch c := source[i]; {
		case c == q:
			return i + 1
		case c == '\n':
			return i
		case c == '\\' && i+1 < len(source) && source[i+1] != '\n':
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
func quoted(source []byte, i int, q byte) (string, int) {
	var b strings.Builder
	for i < len(source) {
		c := source[i]
		switch {
		case c == q:
			return b.String(), i + 1
		case c == '\n':
			return b.String(), i
		case c == '\\' && i+1 < len(source) && source[i+1] != '\n':
			switch e := source[i+1]; e {
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
func number(source []byte, i int) int {
	start := i
	hex := i+1 < len(source) && source[i] == '0' && (source[i+1] == 'x' || source[i+1] == 'X')
	for i < len(source) {
		c := source[i]
		switch {
		case chars.IsWord(c):
			i++
		case c == '.' && i+1 < len(source) && source[i+1] >= '0' && source[i+1] <= '9':
			i++
		case (c == '+' || c == '-') && i > start:
			p := source[i-1]
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

// match links each opening bracket to its closing one and back (-1 when
// unbalanced). A closer that does not fit the innermost opener closes the nearest
// one of its kind within a few frames, so one stray bracket does not unbalance the
// rest of the file.
func match(tokens []token) []int {
	m := make([]int, len(tokens))
	var stack []int
	for i, t := range tokens {
		m[i] = -1
		if t.kind != tPunctuation || len(t.text) != 1 {
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
