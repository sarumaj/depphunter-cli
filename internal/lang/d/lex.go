package d

import "strings"

// The lexer reads what the extraction needs from D: identifiers (keywords
// included), punctuation, and every literal as one token, so nothing inside a
// comment, a string - "..", r"..", `..`, x"..", q"(..)", q"EOS ... EOS" - a
// token string q{ .. } or a character literal is read as code. Comments nest
// only as /+ +/. The lexer stops at __EOF__, as the compiler does.

type kind uint8

const (
	kIdentifier kind = iota // identifiers and keywords
	kString                 // any string literal; text is the value of "..", r".." and `..` only
	kCharacter
	kNumber
	kPunctuation
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
	// tokenDepth is the brace depth inside a token string q{ ... }: its tokens are
	// counted, not emitted, and the whole is one string token.
	tokenDepth int
	tokenLine  int
}

// lex splits source into tokens.
//
// Implements: REQ-DLANG-010
func lex(source []byte) []token {
	s := strings.TrimPrefix(string(source), "\xef\xbb\xbf")
	l := &lexer{s: s, line: 1, tokens: make([]token, 0, len(s)/6)}
	if strings.HasPrefix(s, "#!") {
		l.skipLine()
	}
	l.run()
	if l.tokenDepth > 0 {
		l.tokenDepth = 0
		l.emitAt(kString, "", l.tokenLine, false)
	}
	return l.tokens
}

func (l *lexer) emit(k kind, text string) { l.emitAt(k, text, l.line, false) }

func (l *lexer) emitAt(k kind, text string, line int, value bool) {
	if l.tokenDepth > 0 {
		if k == kPunctuation {
			switch text {
			case "{":
				l.tokenDepth++
			case "}":
				l.tokenDepth--
				if l.tokenDepth == 0 {
					l.tokens = append(l.tokens, token{kind: kString, line: l.tokenLine})
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

func identifierStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func identifierCharacter(c byte) bool { return identifierStart(c) || c >= '0' && c <= '9' }

func digit(c byte) bool { return c >= '0' && c <= '9' }

// operators are D's operators longer than one character, longest first per start.
var operators = []string{">>>=", "...", "<<=", ">>=", ">>>", "^^=", "..", "=>", "==", "!=", "<=", ">=", "+=", "-=", "*=",
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
			l.character()
		case c == '#':
			// #line directives; `#!` only starts a file.
			if strings.HasPrefix(s[l.i:], "#line") {
				l.skipLine()
			} else {
				l.emit(kPunctuation, "#")
				l.i++
			}
		case identifierStart(c):
			l.identifier()
		case digit(c) || c == '.' && l.i+1 < len(s) && digit(s[l.i+1]):
			l.number()
		default:
			l.punctuation()
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
	case identifierStart(open):
		j := from
		for j < len(s) && identifierCharacter(s[j]) {
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

// character reads a character literal, or a lone quote as punctuation.
func (l *lexer) character() {
	s := l.s
	j := l.i + 1
	if j < len(s) && s[j] == '\\' {
		// An escape: \n, \x41, ሴ, \&amp; - the closing quote is near.
		for k := j + 2; k < len(s) && k < j+16 && s[k] != '\n'; k++ {
			if s[k] == '\'' {
				l.i = k + 1
				l.emit(kCharacter, "")
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
			l.emit(kCharacter, "")
			return
		}
	}
	l.emit(kPunctuation, "'")
	l.i++
}

func (l *lexer) identifier() {
	s := l.s
	j := l.i
	for j < len(s) && identifierCharacter(s[j]) {
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
	if word == "q" && j < len(s) && s[j] == '{' && l.tokenDepth == 0 {
		l.tokenDepth, l.tokenLine = 1, l.line
		l.i = j + 1
		return
	}
	if word == "__EOF__" {
		l.i = len(s)
		return
	}
	l.i = j
	l.emit(kIdentifier, word)
}

func (l *lexer) number() {
	s := l.s
	j := l.i
	for j < len(s) {
		c := s[j]
		switch {
		case identifierCharacter(c) && c < 0x80:
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

func (l *lexer) punctuation() {
	s := l.s[l.i:]
	if len(s) < 2 || strings.IndexByte("=.>+-&|<^", s[1]) < 0 {
		l.i++
		l.emit(kPunctuation, s[:1])
		return
	}
	for _, operator := range operators {
		if strings.HasPrefix(s, operator) {
			l.i += len(operator)
			l.emit(kPunctuation, operator)
			return
		}
	}
	l.i++
	l.emit(kPunctuation, s[:1])
}
