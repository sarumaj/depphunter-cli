package proto

// The lexer reads the tokens of a .proto file: identifiers, numbers, strings (single
// or double quoted, escapes kept), punctuation, with // and /* */ comments dropped.
// Protocol Buffers' grammar is small and regular, so a scanner is all the plugin needs
// (REQ-PROTO-009).

type tokKind int

const (
	tIdent tokKind = iota
	tString
	tNumber
	tPunct
)

type tok struct {
	kind tokKind
	text string // an identifier, a number, a punctuation character or a string's value
	line int
}

func isLetter(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// lex splits src into tokens. It never fails: an unterminated string or comment ends
// at the end of the file.
//
// Implements: REQ-PROTO-009
func lex(src []byte) []tok {
	var toks []tok
	line := 1
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		src = src[3:] // a byte order mark
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i < len(src) && !(src[i] == '*' && i+1 < len(src) && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '"' || c == '\'':
			start := line
			var b []byte
			i++
			for i < len(src) && src[i] != c && src[i] != '\n' {
				if src[i] == '\\' && i+1 < len(src) {
					switch src[i+1] {
					case '"', '\'', '\\':
						b = append(b, src[i+1])
					default:
						b = append(b, src[i], src[i+1])
					}
					i += 2
					continue
				}
				b = append(b, src[i])
				i++
			}
			i++ // the closing quote
			toks = append(toks, tok{kind: tString, text: string(b), line: start})
		case isLetter(c):
			j := i
			for j < len(src) && (isLetter(src[j]) || isDigit(src[j])) {
				j++
			}
			toks = append(toks, tok{kind: tIdent, text: string(src[i:j]), line: line})
			i = j
		case isDigit(c):
			j := i
			for j < len(src) && (isLetter(src[j]) || isDigit(src[j]) || src[j] == '.' ||
				((src[j] == '-' || src[j] == '+') && (src[j-1] == 'e' || src[j-1] == 'E'))) {
				j++
			}
			toks = append(toks, tok{kind: tNumber, text: string(src[i:j]), line: line})
			i = j
		default:
			toks = append(toks, tok{kind: tPunct, text: string(c), line: line})
			i++
		}
	}
	return toks
}
