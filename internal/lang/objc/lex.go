package objc

// Token kinds.
const (
	tIdentifier  = iota // identifier or keyword; "@interface" and kin keep their "@"
	tPunctuation        // one punctuation character, or "::"
	tString             // a string or character literal
	tNumber
)

type token struct {
	kind int
	text string
	line int
}

// lex splits Objective-C (and Objective-C++) source into tokens. Comments,
// preprocessor lines (with their continuations) and whitespace are dropped; string
// literals ("x", @"x", 'c', C++ raw strings R"d(...)d") become one token each. Lines
// are 1-based.
//
// Implements: REQ-OBJC-003, REQ-OBJC-013
func lex(source []byte) []token {
	var out []token
	line := 1
	lineStart := true // only whitespace since the start of the line
	n := len(source)
	for i := 0; i < n; {
		c := source[i]
		switch {
		case c == '\n':
			line++
			lineStart = true
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
			continue
		case c == '/' && i+1 < n && source[i+1] == '/':
			for i < n && source[i] != '\n' {
				i++
			}
			continue
		case c == '/' && i+1 < n && source[i+1] == '*':
			i += 2
			for i < n && !(source[i] == '*' && i+1 < n && source[i+1] == '/') {
				if source[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
			continue
		case c == '#' && lineStart:
			// A directive: to the end of the line, continuations and block comments
			// included.
			for i < n && source[i] != '\n' {
				switch {
				case source[i] == '\\' && i+1 < n && source[i+1] == '\n':
					line++
					i += 2
					continue
				case source[i] == '\\' && i+2 < n && source[i+1] == '\r' && source[i+2] == '\n':
					line++
					i += 3
					continue
				case source[i] == '/' && i+1 < n && source[i+1] == '*':
					i += 2
					for i < n && !(source[i] == '*' && i+1 < n && source[i+1] == '/') {
						if source[i] == '\n' {
							line++
						}
						i++
					}
					i += 2
					continue
				case source[i] == '/' && i+1 < n && source[i+1] == '/':
					for i < n && source[i] != '\n' {
						i++
					}
					continue
				}
				i++
			}
			continue
		}
		lineStart = false
		start, startLine := i, line
		switch {
		case c == '"' || c == '\'':
			i = skipQuoted(source, i, &line)
			out = append(out, token{tString, string(source[start:min(i, n)]), startLine})
		case c == 'R' && i+1 < n && source[i+1] == '"':
			i = skipRaw(source, i+1, &line)
			out = append(out, token{tString, "R\"\"", startLine})
		case c == '@' && i+1 < n && source[i+1] == '"':
			i = skipQuoted(source, i+1, &line)
			out = append(out, token{tString, string(source[start:min(i, n)]), startLine})
		case c == '@' && i+1 < n && identifierStart(source[i+1]):
			i++
			for i < n && identifierCharacter(source[i]) {
				i++
			}
			out = append(out, token{tIdentifier, string(source[start:i]), startLine})
		case identifierStart(c):
			for i < n && identifierCharacter(source[i]) {
				i++
			}
			out = append(out, token{tIdentifier, string(source[start:i]), startLine})
		case c >= '0' && c <= '9' || c == '.' && i+1 < n && source[i+1] >= '0' && source[i+1] <= '9':
			i++
			for i < n && (identifierCharacter(source[i]) || source[i] == '.' ||
				source[i] == '\'' && i+1 < n && identifierCharacter(source[i+1]) ||
				(source[i] == '+' || source[i] == '-') && (source[i-1] == 'e' || source[i-1] == 'E' || source[i-1] == 'p' || source[i-1] == 'P')) {
				i++
			}
			out = append(out, token{tNumber, string(source[start:i]), startLine})
		case c == ':' && i+1 < n && source[i+1] == ':':
			i += 2
			out = append(out, token{tPunctuation, "::", startLine})
		default:
			i++
			out = append(out, token{tPunctuation, string(c), startLine})
		}
	}
	return out
}

// skipQuoted steps over a string or character literal starting at i (the quote),
// returning the index after it. An unterminated one ends at the line's end.
func skipQuoted(source []byte, i int, line *int) int {
	q := source[i]
	i++
	for i < len(source) {
		switch source[i] {
		case '\\':
			if i+1 < len(source) && source[i+1] == '\n' {
				*line++
			}
			i += 2
			continue
		case q:
			return i + 1
		case '\n':
			return i // unterminated: the newline is the next token's
		}
		i++
	}
	return len(source)
}

// skipRaw steps over a C++ raw string R"delim( ... )delim" from its quote.
func skipRaw(source []byte, i int, line *int) int {
	j := i + 1
	for j < len(source) && source[j] != '(' && source[j] != '\n' && j-i < 18 {
		j++
	}
	if j >= len(source) || source[j] != '(' {
		return skipQuoted(source, i, line)
	}
	end := ")" + string(source[i+1:j]) + "\""
	for k := j + 1; k+len(end) <= len(source); k++ {
		if source[k] == '\n' {
			*line++
		}
		if string(source[k:k+len(end)]) == end {
			return k + len(end)
		}
	}
	return len(source)
}

func identifierStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func identifierCharacter(c byte) bool { return identifierStart(c) || c >= '0' && c <= '9' }
