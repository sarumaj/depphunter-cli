package ocaml

import (
	"unicode/utf8"
)

// The lexer turns OCaml source into the tokens the reader needs: identifiers
// (lower-case ones and keywords apart from capitalized ones, which name modules and
// constructors), operators and brackets. Comments nest and hide what they contain
// except strings, which OCaml lexes inside comments too; strings, quoted strings
// ({|...|}, {id|...|id}, {%ext|...|}), character literals and polymorphic variants
// are single tokens, so nothing in them is read as code. A `#` in the first column
// starts a directive: cppo's #if/#else/#define lines are dropped (so both branches
// are read), the toplevel's #require/#use keep their arguments.

type kind uint8

const (
	tLower     kind = iota + 1 // lower-case identifier or keyword
	tUpper                     // capitalized identifier: a module or a constructor
	tOp                        // operator or bracket
	tString                    // string literal (content without quotes)
	tLiteral                   // character or number literal, polymorphic variant
	tDirectory                 // toplevel directive (#require): s is its name
)

type token struct {
	k    kind
	s    string
	line int
	// extension is a keyword's extension: let%lwt, match%bind; "*" etc. for binding
	// operators (let*, and+).
	extension string
}

// opCharacters are the characters OCaml builds operators from.
const opCharacters = "!$%&*+-./:<=>?@^|~#"

func isOp(c byte) bool {
	for i := 0; i < len(opCharacters); i++ {
		if opCharacters[i] == c {
			return true
		}
	}
	return false
}

func isIdentifier(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '\''
}

var extensionKeywords = map[string]bool{
	"let": true, "and": true, "match": true, "fun": true, "function": true, "try": true, "if": true,
	"begin": true, "module": true, "open": true, "include": true, "type": true, "external": true,
	"exception": true, "val": true, "class": true, "for": true, "while": true, "lazy": true, "assert": true,
}

// lex tokenizes src. menhir switches on the grammar files' `/* */` and `//`
// comments (the latter outside OCaml code in braces).
func lex(source []byte, menhir bool) []token {
	var out []token
	n := len(source)
	i, line := 0, 1
	if n >= 3 && source[0] == 0xef && source[1] == 0xbb && source[2] == 0xbf {
		i = 3
	}
	braces := 0
	lineStart := true
	for i < n {
		c := source[i]
		if c == '\n' {
			line++
			i++
			lineStart = true
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\f' {
			i++
			continue
		}
		atStart := lineStart
		lineStart = false
		switch {
		case c == '#' && atStart && (i == 0 || source[i-1] == '\n'):
			j := i + 1
			for j < n && (source[j] >= 'a' && source[j] <= 'z' || source[j] == '_') {
				j++
			}
			name := string(source[i+1 : j])
			switch name {
			case "require", "use", "load", "mod_use", "directory", "thread", "install_printer":
				out = append(out, token{k: tDirectory, s: name, line: line})
				i = j
				continue
			}
			// cppo and line directives, a shebang: the line (and its backslash
			// continuations) is not code.
			for i < n && source[i] != '\n' {
				if source[i] == '\\' && i+1 < n && source[i+1] == '\n' {
					line++
					i++
				}
				i++
			}
		case c == '(' && i+1 < n && source[i+1] == '*':
			i, line = skipComment(source, i, line)
		case menhir && c == '/' && i+1 < n && source[i+1] == '*':
			j := i + 2
			for j < n && !(source[j] == '*' && j+1 < n && source[j+1] == '/') {
				if source[j] == '\n' {
					line++
				}
				j++
			}
			i = min(j+2, n)
		case menhir && braces == 0 && c == '/' && i+1 < n && source[i+1] == '/':
			for i < n && source[i] != '\n' {
				i++
			}
		case c == '"':
			start := line
			j, l := skipString(source, i, line)
			out = append(out, token{k: tString, s: string(source[i+1 : max(i+1, j-1)]), line: start})
			i, line = j, l
		case c == '{' && quoted(source, i):
			start := line
			j, l := skipQuoted(source, i, line)
			out = append(out, token{k: tString, line: start})
			i, line = j, l
		case c == '\'':
			if j := characterEnd(source, i); j > 0 {
				out = append(out, token{k: tLiteral, s: "'", line: line})
				i = j
				continue
			}
			out = append(out, token{k: tOp, s: "'", line: line})
			i++
		case c == '`' && i+1 < n && (source[i+1] >= 'A' && source[i+1] <= 'Z' || source[i+1] >= 'a' && source[i+1] <= 'z' || source[i+1] == '_'):
			j := i + 1
			for j < n && isIdentifier(source[j]) {
				j++
			}
			out = append(out, token{k: tLiteral, s: string(source[i:j]), line: line})
			i = j
		case c >= 'a' && c <= 'z' || c == '_':
			j := i
			for j < n && isIdentifier(source[j]) {
				j++
			}
			t := token{k: tLower, s: string(source[i:j]), line: line}
			if extensionKeywords[t.s] && j+1 < n && source[j] == '%' && (source[j+1] >= 'a' && source[j+1] <= 'z' || source[j+1] == '_') {
				k := j + 1
				for k < n && (isIdentifier(source[k]) || source[k] == '.') {
					k++
				}
				t.extension = string(source[j+1 : k])
				j = k
			} else if (t.s == "let" || t.s == "and") && j < n && isOp(source[j]) && source[j] != '.' && source[j] != '#' {
				k := j
				for k < n && isOp(source[k]) {
					k++
				}
				t.extension = string(source[j:k]) // a binding operator: let*, and+
				j = k
			}
			out = append(out, t)
			i = j
		case c >= 'A' && c <= 'Z':
			j := i
			for j < n && isIdentifier(source[j]) {
				j++
			}
			out = append(out, token{k: tUpper, s: string(source[i:j]), line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < n && (isIdentifier(source[j]) && source[j] != '\'' || source[j] == '.' ||
				(source[j] == '-' || source[j] == '+') && (source[j-1] == 'e' || source[j-1] == 'E' || source[j-1] == 'p' || source[j-1] == 'P')) {
				j++
			}
			out = append(out, token{k: tLiteral, s: string(source[i:j]), line: line})
			i = j
		case c == '[' && i+1 < n && source[i+1] == '|':
			out = append(out, token{k: tOp, s: "[|", line: line})
			i += 2
		case c == '|' && i+1 < n && source[i+1] == ']':
			out = append(out, token{k: tOp, s: "|]", line: line})
			i += 2
		case c == '[' && i+1 < n && (source[i+1] == '@' || source[i+1] == '%'):
			j := i + 1
			for j < n && (source[j] == '@' || source[j] == '%') {
				j++
			}
			out = append(out, token{k: tOp, s: string(source[i:j]), line: line})
			i = j
		case c == '.':
			if i+1 < n && source[i+1] == '.' {
				out = append(out, token{k: tOp, s: "..", line: line})
				i += 2
			} else {
				out = append(out, token{k: tOp, s: ".", line: line})
				i++
			}
		case c == ';' && i+1 < n && source[i+1] == ';':
			out = append(out, token{k: tOp, s: ";;", line: line})
			i += 2
		case isOp(c):
			j := i
			for j < n && isOp(source[j]) && !(source[j] == '|' && j+1 < n && source[j+1] == ']') {
				j++
			}
			out = append(out, token{k: tOp, s: string(source[i:j]), line: line})
			i = j
		default:
			if menhir {
				if c == '{' {
					braces++
				} else if c == '}' && braces > 0 {
					braces--
				}
			}
			_, size := utf8.DecodeRune(source[i:])
			if c < utf8.RuneSelf {
				out = append(out, token{k: tOp, s: string(source[i : i+1]), line: line})
			}
			i += size
		}
	}
	return out
}

// skipComment skips the comment starting at i, nested comments and the strings
// inside it included; it returns the index after it and the line reached.
func skipComment(source []byte, i, line int) (int, int) {
	n := len(source)
	depth := 0
	for i < n {
		c := source[i]
		switch {
		case c == '(' && i+1 < n && source[i+1] == '*':
			depth++
			i += 2
		case c == '*' && i+1 < n && source[i+1] == ')':
			depth--
			i += 2
			if depth == 0 {
				return i, line
			}
		case c == '"':
			i, line = skipString(source, i, line)
		case c == '{' && quoted(source, i):
			i, line = skipQuoted(source, i, line)
		case c == '\'':
			// '"' is a character, not a string opening, inside a comment as well.
			if j := characterEnd(source, i); j > 0 {
				for k := i; k < j; k++ {
					if source[k] == '\n' {
						line++
					}
				}
				i = j
			} else {
				i++
			}
		case c == '\n':
			line++
			i++
		default:
			i++
		}
	}
	return n, line
}

// skipString skips the string literal starting at i.
func skipString(source []byte, i, line int) (int, int) {
	n := len(source)
	j := i + 1
	for j < n && source[j] != '"' {
		if source[j] == '\\' && j+1 < n {
			j++
		}
		if source[j] == '\n' {
			line++
		}
		j++
	}
	return min(j+1, n), line
}

// quotedStart returns the index of the `|` opening a quoted string at i ({|, {id|,
// {%ext|, {%ext id|}) and the delimiter's id, or 0.
func quotedStart(source []byte, i int) (int, string) {
	n := len(source)
	j := i + 1
	if j < n && source[j] == '%' {
		j++
		if j < n && source[j] == '%' {
			j++
		}
		k := j
		for j < n && (isIdentifier(source[j]) && source[j] != '\'' || source[j] == '.') {
			j++
		}
		if j == k {
			return 0, ""
		}
		if j < n && source[j] == '|' {
			return j, ""
		}
		if j >= n || source[j] != ' ' {
			return 0, ""
		}
		j++
	}
	k := j
	for j < n && (source[j] >= 'a' && source[j] <= 'z' || source[j] == '_') {
		j++
	}
	if j < n && source[j] == '|' {
		return j, string(source[k:j])
	}
	return 0, ""
}

// skipQuoted skips the quoted string starting at i up to |id}.
func skipQuoted(source []byte, i, line int) (int, int) {
	n := len(source)
	bar, id := quotedStart(source, i)
	closing := "|" + id + "}"
	for j := bar + 1; j < n; j++ {
		if source[j] == '\n' {
			line++
			continue
		}
		if source[j] == '|' && j+len(closing) <= n && string(source[j:j+len(closing)]) == closing {
			return j + len(closing), line
		}
	}
	return n, line
}

// characterEnd returns the index after the character literal at i ('a', '\n', an
// escaped quote, '\123', '\xFF', 'é'), or 0 when the quote is a type variable's
// ('a) or a prime.
func characterEnd(source []byte, i int) int {
	n := len(source)
	if i+1 >= n {
		return 0
	}
	if source[i+1] == '\\' {
		for j := i + 3; j < n && j <= i+12; j++ {
			if source[j] == '\'' {
				return j + 1
			}
			if source[j] == '\n' {
				return 0
			}
		}
		return 0
	}
	if source[i+1] == '\n' {
		return 0
	}
	_, size := utf8.DecodeRune(source[i+1:])
	if i+1+size < n && source[i+1+size] == '\'' {
		return i + 2 + size
	}
	return 0
}

func quoted(source []byte, i int) bool {
	bar, _ := quotedStart(source, i)
	return bar > 0
}
