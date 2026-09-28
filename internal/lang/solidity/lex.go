package solidity

// The lexer turns Solidity (and the Yul of assembly blocks, which shares its
// comments, strings and brackets) into identifiers, punctuation, strings and
// numbers. Comments - NatSpec's /// and /** */ included - are dropped, and
// so is everything inside a string, which is how an import written in a
// comment, a string or an assembly block never reaches the scanner.

type kind uint8

const (
	tIdentifier kind = iota + 1
	tPunctuation
	tString
	tNumber
)

type token struct {
	kind kind
	text string // an identifier, a punctuation character, or a string's content
	line int
}

func identifierStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func identifierPart(c byte) bool { return identifierStart(c) || c >= '0' && c <= '9' }

// lex reads source into tokens. It never fails: an unterminated comment runs to
// the end of the file, an unterminated string to the end of its line.
//
// Implements: REQ-SOLIDITY-002, REQ-SOLIDITY-010
func lex(source []byte) []token {
	s := string(source) // one copy; every token's text is a slice of it
	tokens := make([]token, 0, len(s)/5+16)
	line := 1
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			i += 2
			for i < len(s) && !(s[i] == '*' && i+1 < len(s) && s[i+1] == '/') {
				if s[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
		case c == '"' || c == '\'':
			start := line
			j := i + 1
			for j < len(s) && s[j] != c && s[j] != '\n' {
				if s[j] == '\\' && j+1 < len(s) {
					if s[j+1] == '\n' {
						line++ // an escaped line break continues the string
					}
					j++
				}
				j++
			}
			tokens = append(tokens, token{kind: tString, text: s[i+1 : min(j, len(s))], line: start})
			i = j
			if i < len(s) && s[i] == c {
				i++
			}
		case identifierStart(c):
			j := i + 1
			for j < len(s) && identifierPart(s[j]) {
				j++
			}
			// unicode"..." and hex"..." are string literals with a prefix: the
			// prefix is dropped and the literal lexed as any other string.
			if w := s[i:j]; (w == "unicode" || w == "hex") && j < len(s) && (s[j] == '"' || s[j] == '\'') {
				i = j
				continue
			}
			tokens = append(tokens, token{kind: tIdentifier, text: s[i:j], line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(s) && (identifierPart(s[j]) || s[j] == '.' && j+1 < len(s) && s[j+1] >= '0' && s[j+1] <= '9') {
				j++
			}
			tokens = append(tokens, token{kind: tNumber, text: s[i:j], line: line})
			i = j
		default:
			tokens = append(tokens, token{kind: tPunctuation, text: s[i : i+1], line: line})
			i++
		}
	}
	return tokens
}
