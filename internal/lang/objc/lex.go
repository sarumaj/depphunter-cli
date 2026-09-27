package objc

// Token kinds.
const (
	tIdent  = iota // identifier or keyword; "@interface" and kin keep their "@"
	tPunct         // one punctuation character, or "::"
	tString        // a string or character literal
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
func lex(src []byte) []token {
	var out []token
	line := 1
	bol := true // only whitespace since the start of the line
	n := len(src)
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '\n':
			line++
			bol = true
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
			continue
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		case c == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/') {
				if src[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
			continue
		case c == '#' && bol:
			// A directive: to the end of the line, continuations and block comments
			// included.
			for i < n && src[i] != '\n' {
				switch {
				case src[i] == '\\' && i+1 < n && src[i+1] == '\n':
					line++
					i += 2
					continue
				case src[i] == '\\' && i+2 < n && src[i+1] == '\r' && src[i+2] == '\n':
					line++
					i += 3
					continue
				case src[i] == '/' && i+1 < n && src[i+1] == '*':
					i += 2
					for i < n && !(src[i] == '*' && i+1 < n && src[i+1] == '/') {
						if src[i] == '\n' {
							line++
						}
						i++
					}
					i += 2
					continue
				case src[i] == '/' && i+1 < n && src[i+1] == '/':
					for i < n && src[i] != '\n' {
						i++
					}
					continue
				}
				i++
			}
			continue
		}
		bol = false
		start, startLine := i, line
		switch {
		case c == '"' || c == '\'':
			i = skipQuoted(src, i, &line)
			out = append(out, token{tString, string(src[start:min(i, n)]), startLine})
		case c == 'R' && i+1 < n && src[i+1] == '"':
			i = skipRaw(src, i+1, &line)
			out = append(out, token{tString, "R\"\"", startLine})
		case c == '@' && i+1 < n && src[i+1] == '"':
			i = skipQuoted(src, i+1, &line)
			out = append(out, token{tString, string(src[start:min(i, n)]), startLine})
		case c == '@' && i+1 < n && identStart(src[i+1]):
			i++
			for i < n && identChar(src[i]) {
				i++
			}
			out = append(out, token{tIdent, string(src[start:i]), startLine})
		case identStart(c):
			for i < n && identChar(src[i]) {
				i++
			}
			out = append(out, token{tIdent, string(src[start:i]), startLine})
		case c >= '0' && c <= '9' || c == '.' && i+1 < n && src[i+1] >= '0' && src[i+1] <= '9':
			i++
			for i < n && (identChar(src[i]) || src[i] == '.' ||
				src[i] == '\'' && i+1 < n && identChar(src[i+1]) ||
				(src[i] == '+' || src[i] == '-') && (src[i-1] == 'e' || src[i-1] == 'E' || src[i-1] == 'p' || src[i-1] == 'P')) {
				i++
			}
			out = append(out, token{tNumber, string(src[start:i]), startLine})
		case c == ':' && i+1 < n && src[i+1] == ':':
			i += 2
			out = append(out, token{tPunct, "::", startLine})
		default:
			i++
			out = append(out, token{tPunct, string(c), startLine})
		}
	}
	return out
}

// skipQuoted steps over a string or character literal starting at i (the quote),
// returning the index after it. An unterminated one ends at the line's end.
func skipQuoted(src []byte, i int, line *int) int {
	q := src[i]
	i++
	for i < len(src) {
		switch src[i] {
		case '\\':
			if i+1 < len(src) && src[i+1] == '\n' {
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
	return len(src)
}

// skipRaw steps over a C++ raw string R"delim( ... )delim" from its quote.
func skipRaw(src []byte, i int, line *int) int {
	j := i + 1
	for j < len(src) && src[j] != '(' && src[j] != '\n' && j-i < 18 {
		j++
	}
	if j >= len(src) || src[j] != '(' {
		return skipQuoted(src, i, line)
	}
	end := ")" + string(src[i+1:j]) + "\""
	for k := j + 1; k+len(end) <= len(src); k++ {
		if src[k] == '\n' {
			*line++
		}
		if string(src[k:k+len(end)]) == end {
			return k + len(end)
		}
	}
	return len(src)
}

func identStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func identChar(c byte) bool { return identStart(c) || c >= '0' && c <= '9' }
