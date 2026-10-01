package proto

import "github.com/sarumaj/depphunter-cli/internal/lang/chars"

// The lexer reads the tokens of a .proto file: identifiers, numbers, strings (single
// or double quoted, escapes kept), punctuation, with // and /* */ comments dropped.
// Protocol Buffers' grammar is small and regular, so a scanner is all the plugin needs
// (REQ-PROTO-009).

type tokenKind int

const (
	tIdentifier tokenKind = iota
	tString
	tNumber
	tPunctuation
)

type token struct {
	kind tokenKind
	text string // an identifier, a number, a punctuation character or a string's value
	line int
}

// lex splits source into tokens. It never fails: an unterminated string or comment ends
// at the end of the file.
//
// Implements: REQ-PROTO-009
func lex(source []byte) []token {
	var tokens []token
	line := 1
	if len(source) >= 3 && source[0] == 0xEF && source[1] == 0xBB && source[2] == 0xBF {
		source = source[3:] // a byte order mark
	}
	for i := 0; i < len(source); {
		c := source[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			i += 2
			for i < len(source) && !(source[i] == '*' && i+1 < len(source) && source[i+1] == '/') {
				if source[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '"' || c == '\'':
			start := line
			var b []byte
			i++
			for i < len(source) && source[i] != c && source[i] != '\n' {
				if source[i] == '\\' && i+1 < len(source) {
					switch source[i+1] {
					case '"', '\'', '\\':
						b = append(b, source[i+1])
					default:
						b = append(b, source[i], source[i+1])
					}
					i += 2
					continue
				}
				b = append(b, source[i])
				i++
			}
			i++ // the closing quote
			tokens = append(tokens, token{kind: tString, text: string(b), line: start})
		case chars.IsIdentStart(c):
			j := i
			for j < len(source) && (chars.IsIdentStart(source[j]) || chars.IsDigit(source[j])) {
				j++
			}
			tokens = append(tokens, token{kind: tIdentifier, text: string(source[i:j]), line: line})
			i = j
		case chars.IsDigit(c):
			j := i
			for j < len(source) && (chars.IsIdentStart(source[j]) || chars.IsDigit(source[j]) || source[j] == '.' ||
				((source[j] == '-' || source[j] == '+') && (source[j-1] == 'e' || source[j-1] == 'E'))) {
				j++
			}
			tokens = append(tokens, token{kind: tNumber, text: string(source[i:j]), line: line})
			i = j
		default:
			tokens = append(tokens, token{kind: tPunctuation, text: string(c), line: line})
			i++
		}
	}
	return tokens
}
