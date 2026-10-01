package d

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// A small reader of SDLang, the format of dub.sdl: tags, one per line (or
// separated by ;), each a name, values and key=value attributes, with children
// in braces. Strings are "..." with escapes or `...` raw; //, # and -- start
// line comments, /* */ block comments, and a \ at the end of a line continues the
// tag on the next. Values other than strings (numbers, true, dates) are kept as
// written.

type sdlTag struct {
	name       string
	values     []string
	attributes map[string]string
	children   []*sdlTag
	line       int
}

// attribute is an attribute's value, "" when absent.
func (t *sdlTag) attribute(k string) string { return t.attributes[k] }

// value is the tag's first value, "" when it has none.
func (t *sdlTag) value() string {
	if len(t.values) == 0 {
		return ""
	}
	return t.values[0]
}

type sdlToken struct {
	kind byte // 'w' word, 's' string, '=' '{' '}', 'e' end of tag
	text string
	line int
}

// maxSDLDepth bounds how deep children nest before deeper ones are dropped.
const maxSDLDepth = 64

// readSDL parses source into its top-level tags.
//
// Implements: REQ-DLANG-005
func readSDL(source []byte) []*sdlTag {
	root := &sdlTag{}
	stack := []*sdlTag{root}
	var current []sdlToken
	flush := func() {
		if len(current) == 0 {
			return
		}
		parent := stack[len(stack)-1]
		if t := sdlBuild(current); t != nil && len(stack) <= maxSDLDepth {
			parent.children = append(parent.children, t)
		}
		current = current[:0]
	}
	deep := 0 // braces opened past maxSDLDepth, or for a tag that was not built
	for _, token := range sdlLex(string(source)) {
		switch token.kind {
		case 'e':
			flush()
		case '{':
			parent := stack[len(stack)-1]
			n := len(parent.children)
			flush()
			if len(stack) <= maxSDLDepth && len(parent.children) > n {
				stack = append(stack, parent.children[len(parent.children)-1])
			} else {
				deep++
			}
		case '}':
			flush()
			if deep > 0 {
				deep--
			} else if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		default:
			current = append(current, token)
		}
	}
	flush()
	return root.children
}

// sdlBuild turns one tag's tokens into a tag; nil when it has no name.
func sdlBuild(tokens []sdlToken) *sdlTag {
	if tokens[0].kind != 'w' || len(tokens) > 1 && tokens[1].kind == '=' {
		return nil
	}
	t := &sdlTag{name: tokens[0].text, attributes: map[string]string{}, line: tokens[0].line}
	for i := 1; i < len(tokens); i++ {
		token := tokens[i]
		if token.kind == 'w' && i+2 < len(tokens) && tokens[i+1].kind == '=' {
			t.attributes[token.text] = tokens[i+2].text
			i += 2
			continue
		}
		if token.kind == 'w' || token.kind == 's' {
			t.values = append(t.values, token.text)
		}
	}
	return t
}

func sdlLex(s string) []sdlToken {
	var out []sdlToken
	line := 1
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\n':
			out = append(out, sdlToken{kind: 'e', line: line})
			line++
			i++
		case c == ';':
			out = append(out, sdlToken{kind: 'e', line: line})
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '\\':
			// A line continuation: the line break after it does not end the tag.
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\r') {
				j++
			}
			if j < len(s) && s[j] == '\n' {
				line++
				j++
			}
			i = j
		case c == '#' || c == '/' && strings.HasPrefix(s[i:], "//") || c == '-' && strings.HasPrefix(s[i:], "--"):
			i = chars.LineEnd(s, i)
		case c == '/' && strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			to := len(s)
			if end >= 0 {
				to = i + 2 + end + 2
			}
			line += strings.Count(s[i:to], "\n")
			i = to
		case c == '"':
			var b strings.Builder
			start := line
			j := i + 1
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' && j+1 < len(s) {
					j++
					switch s[j] {
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					case 'r':
						b.WriteByte('\r')
					case '\n':
						// a string continued on the next line: its indentation is dropped
						line++
						for j+1 < len(s) && (s[j+1] == ' ' || s[j+1] == '\t') {
							j++
						}
					default:
						b.WriteByte(s[j])
					}
					j++
					continue
				}
				if s[j] == '\n' {
					break // unterminated: the string ends with its line
				}
				b.WriteByte(s[j])
				j++
			}
			out = append(out, sdlToken{kind: 's', text: b.String(), line: start})
			if j < len(s) && s[j] == '"' {
				j++
			}
			i = j
		case c == '`':
			end := strings.IndexByte(s[i+1:], '`')
			to := len(s)
			if end >= 0 {
				to = i + 1 + end
			}
			out = append(out, sdlToken{kind: 's', text: s[i+1 : to], line: line})
			line += strings.Count(s[i:to], "\n")
			i = min(to+1, len(s))
		case c == '=' || c == '{' || c == '}':
			out = append(out, sdlToken{kind: c, line: line})
			i++
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t\r\n;={}\"`", rune(s[j])) {
				if s[j] == '/' && (strings.HasPrefix(s[j:], "//") || strings.HasPrefix(s[j:], "/*")) {
					break
				}
				j++
			}
			if j == i {
				j++
			}
			out = append(out, sdlToken{kind: 'w', text: s[i:j], line: line})
			i = j
		}
	}
	return out
}
