package puppet

import "strings"

type tokKind uint8

const (
	tName   tokKind = iota // a bare word or qualified name: include, apache::vhost, ::foo
	tRef                   // a capitalized name: File, Class, Stdlib::Port
	tVar                   // $x, $::x, $a::b
	tString                // '...' or "..." (heredoc bodies are skipped)
	tRegex                 // /.../
	tNumber
	tPunct
)

type token struct {
	kind tokKind
	text string // a string's content; interpolated code is left out
	line int
	// interpolate marks a double-quoted string with ${...} or $var in it.
	interpolate bool
	// adj marks a token written right after the previous one, without
	// white space: `f(` is a call.
	adj bool
}

// lex splits a Puppet manifest into tokens. Comments (# and /* */) are
// dropped, and so are heredoc bodies (@(END) ... END), whose text is data.
// A slash starts a regular expression where a value is expected (after an
// operator, `node`, `=~`) and is division after a value.
//
// Implements: REQ-PUPPET-009
func lex(src []byte) []token {
	s := string(src)
	var out []token
	line := 1
	adj := false
	var heredocs []string // tags whose bodies start on the next line
	emit := func(kind tokKind, text string, at int) {
		out = append(out, token{kind: kind, text: text, line: at, adj: adj})
		adj = true
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
			adj = false
			if len(heredocs) > 0 {
				i = skipHeredocs(s, i, heredocs, &line)
				heredocs = heredocs[:0]
			}
		case c == ' ' || c == '\t' || c == '\r':
			i++
			adj = false
		case c == '#':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '/' && at(s, i+1) == '*':
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				end = len(s) - i - 2
			}
			line += strings.Count(s[i:i+2+end], "\n")
			i = min(i+4+end, len(s))
			adj = false
		case c == '\'':
			var b strings.Builder
			start := line
			j := i + 1
			for ; j < len(s) && s[j] != '\''; j++ {
				if s[j] == '\\' && (at(s, j+1) == '\'' || at(s, j+1) == '\\') {
					j++
				}
				if s[j] == '\n' {
					line++
				}
				b.WriteByte(s[j])
			}
			emit(tString, b.String(), start)
			i = min(j+1, len(s))
		case c == '"':
			start := line
			text, interpolate, j := dqString(s, i+1, &line)
			out = append(out, token{kind: tString, text: text, line: start, interpolate: interpolate, adj: adj})
			adj = true
			i = j
		case c == '@' && at(s, i+1) == '(':
			end := strings.IndexAny(s[i:], ")\n")
			if end < 0 || s[i+end] != ')' {
				emit(tPunct, "@", line)
				i++
				continue
			}
			if tag := heredocTag(s[i+2 : i+end]); tag != "" {
				heredocs = append(heredocs, tag)
			}
			emit(tString, "", line) // the heredoc is a value
			i += end + 1
		case c == '$':
			j := i + 1
			if at(s, j) == '{' { // ${x} outside a string: rare, read as a variable
				j++
			}
			for j < len(s) && (isWord(s[j]) || s[j] == ':' && at(s, j+1) == ':') {
				if s[j] == ':' {
					j++
				}
				j++
			}
			emit(tVar, s[i:j], line)
			i = j
		case c == '/' && regexAllowed(out):
			j := i + 1
			for j < len(s) && s[j] != '/' && s[j] != '\n' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(s) || s[j] != '/' {
				emit(tPunct, "/", line)
				i++
				continue
			}
			emit(tRegex, s[i+1:j], line)
			i = j + 1
		case isLower(c) || c == '_' || c == ':' && at(s, i+1) == ':' && (isLower(at(s, i+2)) || at(s, i+2) == '_'):
			j := name(s, i)
			emit(tName, s[i:j], line)
			i = j
		case c >= 'A' && c <= 'Z':
			j := name(s, i)
			emit(tRef, s[i:j], line)
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < len(s) && (isWord(s[j]) || s[j] == '.') {
				j++
			}
			emit(tNumber, s[i:j], line)
			i = j
		default:
			op := s[i : i+1]
			for _, m := range []string{"<<|", "|>>", "=>", "+>", "->", "~>", "<-", "<~", "<|", "|>", "==", "!=", "=~", "!~", "<=", ">=", "@@"} {
				if strings.HasPrefix(s[i:], m) {
					op = m
					break
				}
			}
			emit(tPunct, op, line)
			i += len(op)
		}
	}
	return out
}

// name is the end of the (qualified) name starting at i.
func name(s string, i int) int {
	j := i
	for j < len(s) {
		switch {
		case isWord(s[j]):
			j++
		case s[j] == ':' && at(s, j+1) == ':' && isWord(at(s, j+2)):
			j += 2
		default:
			return j
		}
	}
	return j
}

// dqString reads a double-quoted string whose content starts at i: its
// literal text, whether it interpolates, and the index past its closing
// quote. Interpolated code (${...}, with strings of its own) is skipped.
func dqString(s string, i int, line *int) (string, bool, int) {
	var b strings.Builder
	interpolate := false
	for i < len(s) && s[i] != '"' {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			if s[i+1] == '\n' {
				*line++
			}
			b.WriteByte(s[i+1])
			i += 2
		case s[i] == '$' && at(s, i+1) == '{':
			interpolate = true
			i = skipInterpolate(s, i+2, line, 0)
		case s[i] == '$' && (isWord(at(s, i+1)) || at(s, i+1) == ':'):
			interpolate = true
			i++
		default:
			if s[i] == '\n' {
				*line++
			}
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String(), interpolate, min(i+1, len(s))
}

// skipInterpolate returns the index past the `}` closing an interpolation whose
// code starts at i, skipping the strings inside it; depth bounds nesting.
func skipInterpolate(s string, i int, line *int, depth int) int {
	braces := 1
	for i < len(s) {
		switch c := s[i]; {
		case c == '{':
			braces++
		case c == '}':
			if braces--; braces == 0 {
				return i + 1
			}
		case c == '\n':
			*line++
		case c == '\'':
			for i++; i < len(s) && s[i] != '\''; i++ {
				switch s[i] {
				case '\\':
					i++
				case '\n':
					*line++
				}
			}
		case c == '"' && depth < 16:
			_, _, j := dqString(s, i+1, line)
			i = j
			continue
		}
		i++
	}
	return len(s)
}

// heredocTag is the end tag of a heredoc from what is inside @( ): "END",
// `"END"`, `END:json/L`.
func heredocTag(spec string) string {
	spec = strings.TrimSpace(spec)
	if i := strings.IndexAny(spec, ":/"); i >= 0 {
		spec = spec[:i]
	}
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(spec), `"`))
}

// skipHeredocs moves past the bodies of the heredocs opened on the line that
// just ended, one after the other: each ends at a line holding its tag,
// optionally after `|` and `-`.
func skipHeredocs(s string, i int, tags []string, line *int) int {
	for _, tag := range tags {
		for i < len(s) {
			end := strings.IndexByte(s[i:], '\n')
			if end < 0 {
				end = len(s) - i
			}
			l := strings.TrimSpace(s[i : i+end])
			l = strings.TrimSpace(strings.TrimPrefix(l, "|"))
			l = strings.TrimSpace(strings.TrimPrefix(l, "-"))
			i += end
			if i < len(s) {
				i++
				*line++
			}
			if l == tag {
				break
			}
		}
	}
	return i
}

// regexAllowed reports whether a slash after these tokens starts a regular
// expression: where a value is expected, not after one.
func regexAllowed(out []token) bool {
	if len(out) == 0 {
		return true
	}
	t := out[len(out)-1]
	switch t.kind {
	case tVar, tString, tRegex, tNumber, tRef:
		return false
	case tName:
		return keyword[t.text]
	}
	return t.text != ")" && t.text != "]" && t.text != "}"
}

func at(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

func isWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}
