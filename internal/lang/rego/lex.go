package rego

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

type tokenKind uint8

const (
	tIdentifier tokenKind = iota
	tString
	tNumber
	tPunctuation
)

type token struct {
	kind  tokenKind
	text  string // a string's content
	line  int
	first bool // the first token on its line
}

// lex splits Rego into tokens: comments (#) are dropped, "..." and raw
// `...` strings are one token each.
//
// Implements: REQ-REGO-006
func lex(source []byte) []token {
	s := string(source)
	var out []token
	line, first := 1, true
	emit := func(kind tokenKind, text string, at int) {
		out = append(out, token{kind: kind, text: text, line: at, first: first})
		first = false
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			first = true
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '#':
			i = chars.LineEnd(s, i)
		case c == '"':
			var b strings.Builder
			j := i + 1
			for ; j < len(s) && s[j] != '"' && s[j] != '\n'; j++ {
				if s[j] == '\\' && j+1 < len(s) {
					j++
				}
				b.WriteByte(s[j])
			}
			emit(tString, b.String(), line)
			i = min(j+1, len(s))
		case c == '`':
			start := line
			j := i + 1
			for ; j < len(s) && s[j] != '`'; j++ {
				if s[j] == '\n' {
					line++
				}
			}
			emit(tString, s[i+1:min(j, len(s))], start)
			i = min(j+1, len(s))
		case chars.IsWord(c) && (c < '0' || c > '9'):
			j := i
			for j < len(s) && chars.IsWord(s[j]) {
				j++
			}
			emit(tIdentifier, s[i:j], line)
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < len(s) && (chars.IsWord(s[j]) || s[j] == '.' && chars.At(s, j+1) >= '0' && chars.At(s, j+1) <= '9') {
				j++
			}
			emit(tNumber, s[i:j], line)
			i = j
		default:
			operator := s[i : i+1]
			for _, m := range []string{":=", "==", "!=", "<=", ">="} {
				if strings.HasPrefix(s[i:], m) {
					operator = m
					break
				}
			}
			emit(tPunctuation, operator, line)
			i += len(operator)
		}
	}
	return out
}
