package ada

import "strings"

// Token kinds.
const (
	tIdentifier  = 'i' // an identifier or reserved word; low is its lower-case form
	tString      = 's' // a string literal; text is its value ("" collapsed to ")
	tCharacter   = 'c' // a character literal 'x'
	tNumber      = 'n'
	tPunctuation = 'p'
)

type token struct {
	kind byte
	text string // as written (a string's value)
	low  string // identifiers only: lower case, as Ada compares names
	line int
}

// lexer reads Ada (and GNAT project files, which share its lexical rules) one
// token at a time, so a caller that only wants a unit's header stops early.
//
// Comments run from -- to the end of the line; string literals double their
// quotes and end at the line's end if unterminated; a quote is a character
// literal ('x', including ”') unless it follows a name or a closing
// parenthesis, where it is an attribute tick (X'First, T'(...)).
type lexer struct {
	source   string
	i        int
	line     int
	previous token // the last token returned, for the tick rule
}

func newLexer(source []byte) *lexer {
	s := string(source)
	s = strings.TrimPrefix(s, "\xef\xbb\xbf")
	return &lexer{source: s, line: 1}
}

// next returns the next token; ok is false at the end of the source.
func (l *lexer) next() (token, bool) {
	s := l.source
	for l.i < len(s) {
		c := s[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '-' && l.i+1 < len(s) && s[l.i+1] == '-':
			for l.i < len(s) && s[l.i] != '\n' {
				l.i++
			}
		case c == '#' && l.lineStart():
			l.directive()
		case c == '"':
			return l.emit(l.readString()), true
		case c == '\'':
			if l.i+2 < len(s) && s[l.i+2] == '\'' && !l.tickContext() {
				t := token{kind: tCharacter, text: s[l.i : l.i+3], line: l.line}
				if s[l.i+1] == '\n' {
					l.line++
				}
				l.i += 3
				return l.emit(t), true
			}
			l.i++
			return l.emit(token{kind: tPunctuation, text: "'", line: l.line}), true
		case isIdentifierStart(c):
			j := l.i + 1
			for j < len(s) && isIdentifierPart(s[j]) {
				j++
			}
			t := token{kind: tIdentifier, text: s[l.i:j], line: l.line}
			t.low = lower(t.text)
			l.i = j
			return l.emit(t), true
		case c >= '0' && c <= '9':
			j := l.i + 1
		number:
			for j < len(s) {
				d := s[j]
				switch {
				case d >= '0' && d <= '9', d >= 'a' && d <= 'z', d >= 'A' && d <= 'Z', d == '_', d == '#':
					j++
					continue
				case d == '.' && j+1 < len(s) && s[j+1] != '.':
					j++
					continue
				case (d == '+' || d == '-') && (s[j-1] == 'e' || s[j-1] == 'E') && !strings.Contains(s[l.i:j], "#"):
					j++
					continue
				}
				break number
			}
			t := token{kind: tNumber, text: s[l.i:j], line: l.line}
			l.i = j
			return l.emit(t), true
		default:
			n := 1
			if l.i+1 < len(s) {
				switch s[l.i : l.i+2] {
				case "=>", "..", "**", ":=", "/=", ">=", "<=", "<<", ">>", "<>":
					n = 2
				}
			}
			t := token{kind: tPunctuation, text: s[l.i : l.i+n], line: l.line}
			// Ada 2022's [ ] aggregates nest like parentheses, and are read
			// as such
			switch c {
			case '[':
				t.text = "("
			case ']':
				t.text = ")"
			}
			l.i += n
			return l.emit(t), true
		}
	}
	return token{}, false
}

// lineStart reports whether only blanks precede l.i on its line.
func (l *lexer) lineStart() bool {
	for j := l.i - 1; j >= 0; j-- {
		switch l.source[j] {
		case '\n':
			return true
		case ' ', '\t', '\r', '\f', '\v':
			continue
		}
		return false
	}
	return true
}

// directive skips a gnatprep line (#if, #elsif, #else, #end if). Only the
// first branch of a #if is read: the branches usually repeat an opening
// parenthesis or declaration, which read twice would unbalance the rest.
func (l *lexer) directive() {
	word := l.directiveWord()
	l.skipLine()
	if word != "elsif" && word != "else" {
		return
	}
	// skip the other branches up to the matching #end if
	depth := 0
	for l.i < len(l.source) {
		if l.source[l.i] == '\n' {
			l.line++
			l.i++
			continue
		}
		if c := l.source[l.i]; c == ' ' || c == '\t' || c == '\r' {
			l.i++
			continue
		}
		if l.source[l.i] == '#' && l.lineStart() {
			switch l.directiveWord() {
			case "if":
				depth++
			case "end":
				if depth == 0 {
					l.skipLine()
					return
				}
				depth--
			}
		}
		l.skipLine()
	}
}

// directiveWord is the lower-case word after the # at l.i.
func (l *lexer) directiveWord() string {
	j := l.i + 1
	for j < len(l.source) && (l.source[j] == ' ' || l.source[j] == '\t') {
		j++
	}
	k := j
	for k < len(l.source) && k-j < 8 && isIdentifierPart(l.source[k]) {
		k++
	}
	return lower(l.source[j:k])
}

// skipLine moves to the end of the line (its line break is not consumed).
func (l *lexer) skipLine() {
	if k := strings.IndexByte(l.source[l.i:], '\n'); k >= 0 {
		l.i += k
	} else {
		l.i = len(l.source)
	}
}

func (l *lexer) emit(t token) token {
	l.previous = t
	return t
}

// tickContext reports whether a quote here is an attribute tick: after a name
// (not a reserved word, except all: X.all'Access) or a closing parenthesis.
func (l *lexer) tickContext() bool {
	switch l.previous.kind {
	case tIdentifier:
		return !reserved[l.previous.low] || l.previous.low == "all"
	case tPunctuation:
		return l.previous.text == ")"
	}
	return false
}

// readString reads a string literal at l.i.
func (l *lexer) readString() token {
	s := l.source
	start := l.line
	j := l.i + 1
	var b strings.Builder
	simple := true
	for j < len(s) {
		c := s[j]
		if c == '\n' {
			break
		}
		if c == '"' {
			if j+1 < len(s) && s[j+1] == '"' {
				if simple {
					b.WriteString(s[l.i+1 : j])
					simple = false
				}
				b.WriteByte('"')
				j += 2
				continue
			}
			break
		}
		if !simple {
			b.WriteByte(c)
		}
		j++
	}
	text := ""
	if simple {
		text = s[l.i+1 : min(j, len(s))]
	} else {
		text = b.String()
	}
	if j < len(s) && s[j] == '"' {
		j++
	}
	l.i = j
	return token{kind: tString, text: text, line: start}
}

func isIdentifierStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentifierPart(c byte) bool {
	return isIdentifierStart(c) || c >= '0' && c <= '9' || c == '_'
}

// lower folds ASCII letters; Ada compares identifiers without case.
func lower(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// reserved are Ada 2022's reserved words.
var reserved = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`abort abs abstract accept access aliased all and array at begin body case
		constant declare delay delta digits do else elsif end entry exception exit for function generic goto
		if in interface is limited loop mod new not null of or others out overriding package parallel pragma
		private procedure protected raise range record rem renames requeue return reverse select separate some
		subtype synchronized tagged task terminate then type until use when while with xor`) {
		reserved[w] = true
	}
}

// stream is a lexer with the tokens read so far, for look-ahead. Tokens
// before base were dropped (see trim): readers look back a token or two at
// most.
type stream struct {
	lexer     *lexer
	tokens    []token
	base      int
	endOfFile bool
}

// trim drops the tokens before i - 8 once enough have accumulated, so a long
// file's tokens are not all kept.
func (s *stream) trim(i int) {
	if k := i - 8 - s.base; k > 4096 && k <= len(s.tokens) {
		n := copy(s.tokens, s.tokens[k:])
		s.tokens = s.tokens[:n]
		s.base += k
	}
}

func newStream(source []byte) *stream {
	return &stream{lexer: newLexer(source)}
}

// at returns token i, reading as far as needed; ok is false past the end.
func (s *stream) at(i int) (token, bool) {
	i -= s.base
	for i >= len(s.tokens) && !s.endOfFile {
		t, ok := s.lexer.next()
		if !ok {
			s.endOfFile = true
			break
		}
		s.tokens = append(s.tokens, t)
	}
	if i < 0 || i >= len(s.tokens) {
		return token{}, false
	}
	return s.tokens[i], true
}

// word is token i's lower-case text if it is an identifier.
func (s *stream) word(i int) string {
	if t, ok := s.at(i); ok && t.kind == tIdentifier {
		return t.low
	}
	return ""
}

// punctuation reports whether token i is the punctuation p.
func (s *stream) punctuation(i int, p string) bool {
	t, ok := s.at(i)
	return ok && t.kind == tPunctuation && t.text == p
}
