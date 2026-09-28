package d

import "strings"

// The lexer reads what the extraction needs from D: identifiers (keywords
// included), punctuation, and every literal as one token, so nothing inside a
// comment, a string - "..", r"..", `..`, x"..", q"(..)", q"EOS ... EOS" - a
// token string q{ .. } or a character literal is read as code. Comments nest
// only as /+ +/. The lexer stops at __EOF__, as the compiler does.

type kind uint8

const (
	kIdent  kind = iota // identifiers and keywords
	kString             // any string literal; text is the value of "..", r".." and `..` only
	kChar
	kNumber
	kPunct
)

type token struct {
	kind  kind
	text  string
	line  int
	value bool // a string whose text is its value (no escapes left, not a token string)
}

type lexer struct {
	s      string
	i      int
	line   int
	tokens []token
	// tokDepth is the brace depth inside a token string q{ ... }: its tokens are
	// counted, not emitted, and the whole is one string token.
	tokDepth int
	tokLine  int
}

// lex splits src into tokens.
//
// Implements: REQ-DLANG-010
func lex(src []byte) []token {
	s := strings.TrimPrefix(string(src), "\xef\xbb\xbf")
	l := &lexer{s: s, line: 1, tokens: make([]token, 0, len(s)/6)}
	if strings.HasPrefix(s, "#!") {
		l.skipLine()
	}
	l.run()
	if l.tokDepth > 0 {
		l.tokDepth = 0
		l.emitAt(kString, "", l.tokLine, false)
	}
	return l.tokens
}

func (l *lexer) emit(k kind, text string) { l.emitAt(k, text, l.line, false) }

func (l *lexer) emitAt(k kind, text string, line int, value bool) {
	if l.tokDepth > 0 {
		if k == kPunct {
			switch text {
			case "{":
				l.tokDepth++
			case "}":
				l.tokDepth--
				if l.tokDepth == 0 {
					l.tokens = append(l.tokens, token{kind: kString, line: l.tokLine})
				}
			}
		}
		return
	}
	l.tokens = append(l.tokens, token{kind: k, text: text, line: line, value: value})
}

func (l *lexer) skipLine() {
	for l.i < len(l.s) && l.s[l.i] != '\n' {
		l.i++
	}
}

// count adds the line breaks of s[from:to] to the line number.
func (l *lexer) count(from, to int) {
	l.line += strings.Count(l.s[from:min(to, len(l.s))], "\n")
}

func identStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func identChar(c byte) bool { return identStart(c) || c >= '0' && c <= '9' }

func digit(c byte) bool { return c >= '0' && c <= '9' }

// ops are D's operators longer than one character, longest first per start.
var ops = []string{">>>=", "...", "<<=", ">>=", ">>>", "^^=", "..", "=>", "==", "!=", "<=", ">=", "+=", "-=", "*=",
	"/=", "%=", "&=", "|=", "^=", "~=", "&&", "||", "++", "--", "<<", ">>", "^^"}

func (l *lexer) run() {
	s := l.s
	for l.i < len(s) {
		c := s[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == 0 || c == 0x1a:
			return // the end of the source, as the compiler reads it
		case c == '/' && l.i+1 < len(s) && s[l.i+1] == '/':
			l.skipLine()
		case c == '/' && l.i+1 < len(s) && s[l.i+1] == '*':
			end := strings.Index(s[l.i+2:], "*/")
			to := len(s)
			if end >= 0 {
				to = l.i + 2 + end + 2
			}
			l.count(l.i, to)
			l.i = to
		case c == '/' && l.i+1 < len(s) && s[l.i+1] == '+':
			l.nestedComment()
		case c == '"':
			l.quoted(l.i + 1)
		case c == '`':
			l.wysiwyg(l.i+1, '`', true)
		case c == '\'':
			l.char()
		case c == '#':
			// #line directives; `#!` only starts a file.
			if strings.HasPrefix(s[l.i:], "#line") {
				l.skipLine()
			} else {
				l.emit(kPunct, "#")
				l.i++
			}
		case identStart(c):
			l.ident()
		case digit(c) || c == '.' && l.i+1 < len(s) && digit(s[l.i+1]):
			l.number()
		default:
			l.punct()
		}
	}
}

func (l *lexer) nestedComment() {
	s := l.s
	depth := 0
	j := l.i
	for j < len(s) {
		switch {
		case s[j] == '/' && j+1 < len(s) && s[j+1] == '+':
			depth++
			j += 2
		case s[j] == '+' && j+1 < len(s) && s[j+1] == '/':
			depth--
			j += 2
			if depth == 0 {
				l.count(l.i, j)
				l.i = j
				return
			}
		default:
			j++
		}
	}
	l.count(l.i, len(s))
	l.i = len(s)
}

// quoted reads a "..." string whose body starts at from, with escapes, and its
// optional c, w or d suffix.
func (l *lexer) quoted(from int) {
	s := l.s
	line := l.line
	var b strings.Builder
	plain := true
	j := from
	for j < len(s) && s[j] != '"' {
		if s[j] == '\\' && j+1 < len(s) {
			plain = false
			j += 2
			continue
		}
		j++
	}
	if plain {
		b.WriteString(s[from:min(j, len(s))])
	}
	l.count(l.i, j)
	l.i = min(j+1, len(s))
	l.suffix()
	l.emitAt(kString, b.String(), line, plain)
}

// wysiwyg reads a string whose body starts at from and ends at the next close.
func (l *lexer) wysiwyg(from int, close byte, value bool) {
	s := l.s
	line := l.line
	end := strings.IndexByte(s[from:], close)
	to := len(s)
	if end >= 0 {
		to = from + end
	}
	text := s[from:to]
	l.count(l.i, to)
	l.i = min(to+1, len(s))
	l.suffix()
	l.emitAt(kString, text, line, value)
}

func (l *lexer) suffix() {
	if l.i < len(l.s) && (l.s[l.i] == 'c' || l.s[l.i] == 'w' || l.s[l.i] == 'd') {
		l.i++
	}
}

// delimited reads q"..." whose delimiter starts at from: a bracket pair that
// nests, an identifier ending a heredoc at a line starting with it, or any
// other character that ends the string before the closing quote.
func (l *lexer) delimited(from int) {
	s := l.s
	line := l.line
	if from >= len(s) {
		l.count(l.i, len(s))
		l.i = len(s)
		l.emitAt(kString, "", line, false)
		return
	}
	to := len(s)
	switch open := s[from]; {
	case open == '(' || open == '[' || open == '{' || open == '<':
		close := map[byte]byte{'(': ')', '[': ']', '{': '}', '<': '>'}[open]
		depth := 0
		for j := from; j < len(s); j++ {
			if s[j] == open {
				depth++
			} else if s[j] == close {
				depth--
				if depth == 0 {
					to = j + 1
					if to < len(s) && s[to] == '"' {
						to++
					}
					break
				}
			}
		}
	case identStart(open):
		j := from
		for j < len(s) && identChar(s[j]) {
			j++
		}
		id := s[from:j]
		// The body starts on the next line and ends at a line starting with id".
		k := strings.IndexByte(s[j:], '\n')
		for k >= 0 {
			start := j + k + 1
			if strings.HasPrefix(s[start:], id+`"`) {
				to = start + len(id) + 1
				break
			}
			j = start
			k = strings.IndexByte(s[j:], '\n')
		}
	default:
		if end := strings.Index(s[from+1:], string(open)+`"`); end >= 0 {
			to = from + 1 + end + 2
		}
	}
	l.count(l.i, to)
	l.i = to
	l.suffix()
	l.emitAt(kString, "", line, false)
}

// char reads a character literal, or a lone quote as punctuation.
func (l *lexer) char() {
	s := l.s
	j := l.i + 1
	if j < len(s) && s[j] == '\\' {
		// An escape: \n, \x41, ሴ, \&amp; - the closing quote is near.
		for k := j + 2; k < len(s) && k < j+16 && s[k] != '\n'; k++ {
			if s[k] == '\'' {
				l.i = k + 1
				l.emit(kChar, "")
				return
			}
		}
	} else if j < len(s) && s[j] != '\n' {
		k := j + 1
		for k < len(s) && k < j+4 && s[k]&0xc0 == 0x80 {
			k++ // the rest of a UTF-8 character
		}
		if k < len(s) && s[k] == '\'' {
			l.i = k + 1
			l.emit(kChar, "")
			return
		}
	}
	l.emit(kPunct, "'")
	l.i++
}

func (l *lexer) ident() {
	s := l.s
	j := l.i
	for j < len(s) && identChar(s[j]) {
		j++
	}
	word := s[l.i:j]
	if j < len(s) && s[j] == '"' {
		switch word {
		case "r":
			l.wysiwyg(j+1, '"', true)
			return
		case "x":
			l.wysiwyg(j+1, '"', false) // hex digits, not text
			return
		case "q":
			l.delimited(j + 1)
			return
		}
	}
	if word == "q" && j < len(s) && s[j] == '{' && l.tokDepth == 0 {
		l.tokDepth, l.tokLine = 1, l.line
		l.i = j + 1
		return
	}
	if word == "__EOF__" {
		l.i = len(s)
		return
	}
	l.i = j
	l.emit(kIdent, word)
}

func (l *lexer) number() {
	s := l.s
	j := l.i
	for j < len(s) {
		c := s[j]
		switch {
		case identChar(c) && c < 0x80:
			j++
		case c == '.' && j+1 < len(s) && digit(s[j+1]):
			j++
		case (c == '+' || c == '-') && j > l.i && strings.ContainsRune("eEpP", rune(s[j-1])) && !strings.HasPrefix(s[l.i:], "0x") && !strings.HasPrefix(s[l.i:], "0X"):
			j++
		case (c == '+' || c == '-') && j > l.i && (s[j-1] == 'p' || s[j-1] == 'P'):
			j++
		default:
			l.i = j
			l.emit(kNumber, "")
			return
		}
	}
	l.i = j
	l.emit(kNumber, "")
}

func (l *lexer) punct() {
	s := l.s[l.i:]
	if len(s) < 2 || strings.IndexByte("=.>+-&|<^", s[1]) < 0 {
		l.i++
		l.emit(kPunct, s[:1])
		return
	}
	for _, op := range ops {
		if strings.HasPrefix(s, op) {
			l.i += len(op)
			l.emit(kPunct, op)
			return
		}
	}
	l.i++
	l.emit(kPunct, s[:1])
}
