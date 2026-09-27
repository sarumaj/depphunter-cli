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
	tLower kind = iota + 1 // lower-case identifier or keyword
	tUpper                 // capitalized identifier: a module or a constructor
	tOp                    // operator or bracket
	tStr                   // string literal (content without quotes)
	tLit                   // character or number literal, polymorphic variant
	tDir                   // toplevel directive (#require): s is its name
)

type token struct {
	k    kind
	s    string
	line int
	// ext is a keyword's extension: let%lwt, match%bind; "*" etc. for binding
	// operators (let*, and+).
	ext string
}

// opChars are the characters OCaml builds operators from.
const opChars = "!$%&*+-./:<=>?@^|~#"

func isOp(c byte) bool {
	for i := 0; i < len(opChars); i++ {
		if opChars[i] == c {
			return true
		}
	}
	return false
}

func isIdent(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '\''
}

var extKeywords = map[string]bool{
	"let": true, "and": true, "match": true, "fun": true, "function": true, "try": true, "if": true,
	"begin": true, "module": true, "open": true, "include": true, "type": true, "external": true,
	"exception": true, "val": true, "class": true, "for": true, "while": true, "lazy": true, "assert": true,
}

// lex tokenizes src. menhir switches on the grammar files' `/* */` and `//`
// comments (the latter outside OCaml code in braces).
func lex(src []byte, menhir bool) []token {
	var out []token
	n := len(src)
	i, line := 0, 1
	if n >= 3 && src[0] == 0xef && src[1] == 0xbb && src[2] == 0xbf {
		i = 3
	}
	braces := 0
	lineStart := true
	for i < n {
		c := src[i]
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
		case c == '#' && atStart && (i == 0 || src[i-1] == '\n'):
			j := i + 1
			for j < n && (src[j] >= 'a' && src[j] <= 'z' || src[j] == '_') {
				j++
			}
			name := string(src[i+1 : j])
			switch name {
			case "require", "use", "load", "mod_use", "directory", "thread", "install_printer":
				out = append(out, token{k: tDir, s: name, line: line})
				i = j
				continue
			}
			// cppo and line directives, a shebang: the line (and its backslash
			// continuations) is not code.
			for i < n && src[i] != '\n' {
				if src[i] == '\\' && i+1 < n && src[i+1] == '\n' {
					line++
					i++
				}
				i++
			}
		case c == '(' && i+1 < n && src[i+1] == '*':
			i, line = skipComment(src, i, line)
		case menhir && c == '/' && i+1 < n && src[i+1] == '*':
			j := i + 2
			for j < n && !(src[j] == '*' && j+1 < n && src[j+1] == '/') {
				if src[j] == '\n' {
					line++
				}
				j++
			}
			i = min(j+2, n)
		case menhir && braces == 0 && c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '"':
			start := line
			j, l := skipString(src, i, line)
			out = append(out, token{k: tStr, s: string(src[i+1 : max(i+1, j-1)]), line: start})
			i, line = j, l
		case c == '{' && quoted(src, i):
			start := line
			j, l := skipQuoted(src, i, line)
			out = append(out, token{k: tStr, line: start})
			i, line = j, l
		case c == '\'':
			if j := charEnd(src, i); j > 0 {
				out = append(out, token{k: tLit, s: "'", line: line})
				i = j
				continue
			}
			out = append(out, token{k: tOp, s: "'", line: line})
			i++
		case c == '`' && i+1 < n && (src[i+1] >= 'A' && src[i+1] <= 'Z' || src[i+1] >= 'a' && src[i+1] <= 'z' || src[i+1] == '_'):
			j := i + 1
			for j < n && isIdent(src[j]) {
				j++
			}
			out = append(out, token{k: tLit, s: string(src[i:j]), line: line})
			i = j
		case c >= 'a' && c <= 'z' || c == '_':
			j := i
			for j < n && isIdent(src[j]) {
				j++
			}
			t := token{k: tLower, s: string(src[i:j]), line: line}
			if extKeywords[t.s] && j+1 < n && src[j] == '%' && (src[j+1] >= 'a' && src[j+1] <= 'z' || src[j+1] == '_') {
				k := j + 1
				for k < n && (isIdent(src[k]) || src[k] == '.') {
					k++
				}
				t.ext = string(src[j+1 : k])
				j = k
			} else if (t.s == "let" || t.s == "and") && j < n && isOp(src[j]) && src[j] != '.' && src[j] != '#' {
				k := j
				for k < n && isOp(src[k]) {
					k++
				}
				t.ext = string(src[j:k]) // a binding operator: let*, and+
				j = k
			}
			out = append(out, t)
			i = j
		case c >= 'A' && c <= 'Z':
			j := i
			for j < n && isIdent(src[j]) {
				j++
			}
			out = append(out, token{k: tUpper, s: string(src[i:j]), line: line})
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < n && (isIdent(src[j]) && src[j] != '\'' || src[j] == '.' ||
				(src[j] == '-' || src[j] == '+') && (src[j-1] == 'e' || src[j-1] == 'E' || src[j-1] == 'p' || src[j-1] == 'P')) {
				j++
			}
			out = append(out, token{k: tLit, s: string(src[i:j]), line: line})
			i = j
		case c == '[' && i+1 < n && src[i+1] == '|':
			out = append(out, token{k: tOp, s: "[|", line: line})
			i += 2
		case c == '|' && i+1 < n && src[i+1] == ']':
			out = append(out, token{k: tOp, s: "|]", line: line})
			i += 2
		case c == '[' && i+1 < n && (src[i+1] == '@' || src[i+1] == '%'):
			j := i + 1
			for j < n && (src[j] == '@' || src[j] == '%') {
				j++
			}
			out = append(out, token{k: tOp, s: string(src[i:j]), line: line})
			i = j
		case c == '.':
			if i+1 < n && src[i+1] == '.' {
				out = append(out, token{k: tOp, s: "..", line: line})
				i += 2
			} else {
				out = append(out, token{k: tOp, s: ".", line: line})
				i++
			}
		case c == ';' && i+1 < n && src[i+1] == ';':
			out = append(out, token{k: tOp, s: ";;", line: line})
			i += 2
		case isOp(c):
			j := i
			for j < n && isOp(src[j]) && !(src[j] == '|' && j+1 < n && src[j+1] == ']') {
				j++
			}
			out = append(out, token{k: tOp, s: string(src[i:j]), line: line})
			i = j
		default:
			if menhir {
				if c == '{' {
					braces++
				} else if c == '}' && braces > 0 {
					braces--
				}
			}
			_, size := utf8.DecodeRune(src[i:])
			if c < utf8.RuneSelf {
				out = append(out, token{k: tOp, s: string(src[i : i+1]), line: line})
			}
			i += size
		}
	}
	return out
}

// skipComment skips the comment starting at i, nested comments and the strings
// inside it included; it returns the index after it and the line reached.
func skipComment(src []byte, i, line int) (int, int) {
	n := len(src)
	depth := 0
	for i < n {
		c := src[i]
		switch {
		case c == '(' && i+1 < n && src[i+1] == '*':
			depth++
			i += 2
		case c == '*' && i+1 < n && src[i+1] == ')':
			depth--
			i += 2
			if depth == 0 {
				return i, line
			}
		case c == '"':
			i, line = skipString(src, i, line)
		case c == '{' && quoted(src, i):
			i, line = skipQuoted(src, i, line)
		case c == '\'':
			// '"' is a character, not a string opening, inside a comment as well.
			if j := charEnd(src, i); j > 0 {
				for k := i; k < j; k++ {
					if src[k] == '\n' {
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
func skipString(src []byte, i, line int) (int, int) {
	n := len(src)
	j := i + 1
	for j < n && src[j] != '"' {
		if src[j] == '\\' && j+1 < n {
			j++
		}
		if src[j] == '\n' {
			line++
		}
		j++
	}
	return min(j+1, n), line
}

// quotedStart returns the index of the `|` opening a quoted string at i ({|, {id|,
// {%ext|, {%ext id|}) and the delimiter's id, or 0.
func quotedStart(src []byte, i int) (int, string) {
	n := len(src)
	j := i + 1
	if j < n && src[j] == '%' {
		j++
		if j < n && src[j] == '%' {
			j++
		}
		k := j
		for j < n && (isIdent(src[j]) && src[j] != '\'' || src[j] == '.') {
			j++
		}
		if j == k {
			return 0, ""
		}
		if j < n && src[j] == '|' {
			return j, ""
		}
		if j >= n || src[j] != ' ' {
			return 0, ""
		}
		j++
	}
	k := j
	for j < n && (src[j] >= 'a' && src[j] <= 'z' || src[j] == '_') {
		j++
	}
	if j < n && src[j] == '|' {
		return j, string(src[k:j])
	}
	return 0, ""
}

// skipQuoted skips the quoted string starting at i up to |id}.
func skipQuoted(src []byte, i, line int) (int, int) {
	n := len(src)
	bar, id := quotedStart(src, i)
	closing := "|" + id + "}"
	for j := bar + 1; j < n; j++ {
		if src[j] == '\n' {
			line++
			continue
		}
		if src[j] == '|' && j+len(closing) <= n && string(src[j:j+len(closing)]) == closing {
			return j + len(closing), line
		}
	}
	return n, line
}

// charEnd returns the index after the character literal at i ('a', '\n', an
// escaped quote, '\123', '\xFF', 'é'), or 0 when the quote is a type variable's
// ('a) or a prime.
func charEnd(src []byte, i int) int {
	n := len(src)
	if i+1 >= n {
		return 0
	}
	if src[i+1] == '\\' {
		for j := i + 3; j < n && j <= i+12; j++ {
			if src[j] == '\'' {
				return j + 1
			}
			if src[j] == '\n' {
				return 0
			}
		}
		return 0
	}
	if src[i+1] == '\n' {
		return 0
	}
	_, size := utf8.DecodeRune(src[i+1:])
	if i+1+size < n && src[i+1+size] == '\'' {
		return i + 2 + size
	}
	return 0
}

func quoted(src []byte, i int) bool {
	bar, _ := quotedStart(src, i)
	return bar > 0
}
