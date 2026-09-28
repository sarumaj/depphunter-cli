// Package minify strips what a browser does not read from the assets the UI is made
// of: comments, indentation and blank lines.
//
// The comments in this project are the documentation, and there are a lot of them -
// close to half of some files. They belong in the repository and not on the wire, so
// they are taken out on the way there and the file on disk is left alone.
//
// Nothing here renames, reorders or rewrites anything, and nothing moves between
// lines: every newline in the source is still in the output, comments included. So
// automatic semicolon insertion cannot decide differently than it did before, and
// line 300 of what the browser runs is line 300 of the file in the repository, which
// is what makes a stack trace still worth reading. What it cannot prove is a comment,
// it copies.
package minify

import "strings"

// JS removes comments and indentation from JavaScript.
//
// The whole difficulty is the slash: it opens a comment, divides, or opens a regular
// expression, and only what came before it says which. The usual rule is used - after
// a value (an identifier, a number, a string, a closing bracket) a slash divides,
// and otherwise it opens a pattern - with one safeguard that makes a wrong guess
// harmless: a pattern may not contain a newline, so a scan that reaches one gives up
// and the slash is treated as an ordinary character after all.
func JS(source string) string {
	// Indentation is dropped as the characters are copied, outside literals only: a
	// template literal or a continued string may span lines, and the whitespace
	// inside it is part of its value. trimLines, used for CSS and HTML, cannot tell.
	out := make([]byte, 0, len(source))
	lineStart := true
	previous := byte(0) // the last significant character written, for the slash rule
	literal := func(s string) {
		out = append(out, s...)
		lineStart = false
	}
	newline := func() {
		for len(out) > 0 && isSpace(out[len(out)-1]) {
			out = out[:len(out)-1]
		}
		out = append(out, '\n')
		lineStart = true
	}
	for i := 0; i < len(source); {
		c := source[i]
		switch {
		case c == '"' || c == '\'':
			j := endOfString(source, i)
			literal(source[i:j])
			previous = '"'
			i = j
		case c == '`':
			j := endOfTemplate(source, i)
			literal(source[i:j])
			previous = '"'
			i = j
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			j := strings.IndexByte(source[i:], '\n')
			if j < 0 {
				i = len(source)
			} else {
				i += j
			}
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			j := strings.Index(source[i+2:], "*/")
			end := len(source)
			if j >= 0 {
				end = i + 2 + j + 2
			}
			// A comment separates two tokens, so a space stays behind, and every
			// newline it contained stays too: the output then has the line numbering
			// the source had, and a stack trace in the browser still points at the
			// line it came from.
			if !lineStart {
				out = append(out, ' ')
			}
			for range strings.Count(source[i:end], "\n") {
				newline()
			}
			i = end
		case c == '/' && !dividesAfter(previous, out):
			if j := endOfRegexp(source, i); j > 0 {
				literal(source[i:j])
				previous = '/'
				i = j
				continue
			}
			literal("/")
			previous = c
			i++
		case c == '\n':
			newline()
			i++
		case isSpace(c):
			if !lineStart {
				out = append(out, c)
			}
			i++
		default:
			literal(source[i : i+1])
			previous = c
			i++
		}
	}
	for len(out) > 0 && isSpace(out[len(out)-1]) {
		out = out[:len(out)-1]
	}
	return string(out)
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\v' || c == '\f' }

// CSS removes comments and indentation from a stylesheet. A stylesheet has no regular
// expressions and no line comments, so only strings have to be stepped over.
func CSS(source string) string {
	var out strings.Builder
	out.Grow(len(source))
	for i := 0; i < len(source); {
		switch c := source[i]; {
		case c == '"' || c == '\'':
			j := endOfString(source, i)
			out.WriteString(source[i:j])
			i = j
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			j := strings.Index(source[i+2:], "*/")
			if j < 0 {
				i = len(source)
			} else {
				i += 2 + j + 2
			}
			out.WriteByte(' ')
		default:
			out.WriteByte(c)
			i++
		}
	}
	return trimLines(out.String())
}

// HTML removes comments and indentation from a document. Conditional comments are
// long gone from the browsers this runs in, and there is no element here whose
// whitespace is significant, but text nodes are left alone all the same: only the
// indentation a line starts with goes.
func HTML(source string) string {
	var out strings.Builder
	out.Grow(len(source))
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "<!--") {
			j := strings.Index(source[i+4:], "-->")
			if j < 0 {
				break
			}
			i += 4 + j + 3
			continue
		}
		out.WriteByte(source[i])
		i++
	}
	return trimLines(out.String())
}

// dividesAfter reports whether a slash following previous is a division rather than the
// start of a pattern: it is after anything that can end a value, except a keyword an
// expression follows (return /x/, typeof /x/). out is the output so far, from which
// the word ending in previous is read.
func dividesAfter(previous byte, out []byte) bool {
	switch {
	case isWord(previous):
		end := len(out)
		for end > 0 && isSpace(out[end-1]) {
			end--
		}
		start := end
		for start > 0 && isWord(out[start-1]) {
			start--
		}
		// obj.return / 2 is a property, and divides.
		if start > 0 && out[start-1] == '.' {
			return true
		}
		return !beforeValue[string(out[start:end])]
	case previous == ')' || previous == ']' || previous == '}' || previous == '"':
		return true
	}
	return false
}

func isWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '$'
}

// beforeValue are the keywords after which an expression starts, so that a slash
// after them opens a pattern.
var beforeValue = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true, "new": true,
	"delete": true, "void": true, "throw": true, "case": true, "do": true, "else": true,
	"yield": true, "await": true,
}

// endOfString returns the index just past the quoted string starting at i.
func endOfString(source string, i int) int {
	quote := source[i]
	for j := i + 1; j < len(source); j++ {
		switch source[j] {
		case '\\':
			j++
		case quote:
			return j + 1
		case '\n':
			return j // unterminated; let the browser complain about its own file
		}
	}
	return len(source)
}

// endOfTemplate returns the index just past the template literal starting at i,
// stepping over the ${...} holes in it, which may hold anything at all.
func endOfTemplate(source string, i int) int {
	for j := i + 1; j < len(source); j++ {
		switch source[j] {
		case '\\':
			j++
		case '`':
			return j + 1
		case '$':
			if j+1 < len(source) && source[j+1] == '{' {
				j = endOfHole(source, j+1) - 1
			}
		}
	}
	return len(source)
}

// endOfHole returns the index just past the ${...} that opens at i (the brace),
// counting nested braces and stepping over the strings and templates inside it.
func endOfHole(source string, i int) int {
	depth := 0
	for j := i; j < len(source); j++ {
		switch source[j] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return j + 1
			}
		case '"', '\'':
			j = endOfString(source, j) - 1
		case '`':
			j = endOfTemplate(source, j) - 1
		}
	}
	return len(source)
}

// endOfRegexp returns the index just past the pattern starting at i, or 0 if what is
// there is not one: a pattern may not contain a newline, so anything that runs into
// one was a division after all.
func endOfRegexp(source string, i int) int {
	class := false
	for j := i + 1; j < len(source); j++ {
		switch source[j] {
		case '\\':
			j++
		case '\n':
			return 0
		case '[':
			class = true
		case ']':
			class = false
		case '/':
			if class {
				continue
			}
			// The flags after it are part of the literal.
			for j++; j < len(source) && isFlag(source[j]); j++ {
			}
			return j
		}
	}
	return 0
}

func isFlag(c byte) bool { return c >= 'a' && c <= 'z' }

// trimLines drops the indentation every line starts with and the whitespace it ends
// with. A line left empty stays: it costs one byte, which compression gives back, and
// it keeps every line where it was.
func trimLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.Join(lines, "\n")
}
