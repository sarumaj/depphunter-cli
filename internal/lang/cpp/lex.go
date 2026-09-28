package cpp

import "strings"

// Token kinds of the declaration scanner's lexer.
const (
	tIdentifier  = iota
	tPunctuation // one character, or "::"
	tString
	tCharacter
	tNumber
)

// token is one C or C++ token. Comments and whitespace are dropped; directives are
// kept apart (directive), each remembering the token it precedes.
type token struct {
	kind int
	text string
	line int
}

// directive is one preprocessor line (continuations joined, comments removed).
type directive struct {
	word string // define, if, ifdef, ..., without the '#'
	rest string // what follows the word
	line int
	at   int // index of the token the directive precedes
}

// lexer turns a C or C++ source into tokens and directives. Lines the preprocessor
// scan found dead (#if 0) contribute nothing.
type lexer struct {
	source      string
	i           int
	line        int
	dead        []bool
	tokens      []token
	directories []directive
	lineStart   bool // only whitespace since the last newline
	cplus       bool
}

func lex(source string, dead []bool, cplus bool) ([]token, []directive) {
	l := &lexer{source: source, line: 1, dead: dead, lineStart: true, cplus: cplus}
	l.tokens = make([]token, 0, len(source)/6) // about one token in six bytes of C++
	l.run()
	return l.tokens, l.directories
}

func (l *lexer) deadLine(line int) bool { return line < len(l.dead) && l.dead[line] }

func (l *lexer) emit(kind int, start, line int) {
	if !l.deadLine(line) {
		l.tokens = append(l.tokens, token{kind: kind, text: l.source[start:l.i], line: line})
	}
}

func identifierStart(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 || (c|0x20 >= 'a' && c|0x20 <= 'z')
}

func identifierPart(c byte) bool { return identifierStart(c) || (c >= '0' && c <= '9') }

func (l *lexer) run() {
	source := l.source
	for l.i < len(source) {
		c := source[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
			l.lineStart = true
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
			continue
		case c == '\\' && l.i+1 < len(source) && (source[l.i+1] == '\n' || source[l.i+1] == '\r'):
			l.i++ // a continued line outside a directive
			continue
		case c == '/' && l.i+1 < len(source) && source[l.i+1] == '/':
			l.lineComment()
			continue
		case c == '/' && l.i+1 < len(source) && source[l.i+1] == '*':
			l.blockComment()
			continue
		case c == '#' && l.lineStart:
			l.directive()
			continue
		}
		l.lineStart = false
		start, line := l.i, l.line
		switch {
		case identifierStart(c):
			for l.i < len(source) && identifierPart(source[l.i]) {
				l.i++
			}
			word := source[start:l.i]
			if l.i < len(source) && (source[l.i] == '"' || source[l.i] == '\'') && literalPrefix(word) {
				if source[l.i] == '"' && strings.HasSuffix(word, "R") {
					l.rawString()
				} else {
					l.quoted(source[l.i])
				}
				kind := tString
				if source[l.i-1] == '\'' {
					kind = tCharacter
				}
				l.emit(kind, start, line)
				continue
			}
			l.emit(tIdentifier, start, line)
		case c >= '0' && c <= '9' || (c == '.' && l.i+1 < len(source) && source[l.i+1] >= '0' && source[l.i+1] <= '9'):
			l.number()
			l.emit(tNumber, start, line)
		case c == '"':
			l.quoted('"')
			l.emit(tString, start, line)
		case c == '\'':
			l.quoted('\'')
			l.emit(tCharacter, start, line)
		case c == ':' && l.i+1 < len(source) && source[l.i+1] == ':' && l.cplus:
			l.i += 2
			l.emit(tPunctuation, start, line)
		case c == '-' && l.i+1 < len(source) && source[l.i+1] == '>':
			l.i += 2
			l.emit(tPunctuation, start, line)
		default:
			l.i++
			l.emit(tPunctuation, start, line)
		}
	}
}

// literalPrefix reports whether an identifier directly before a quote is a string
// or character literal's encoding prefix (L, u, U, u8) with an optional R.
func literalPrefix(word string) bool {
	switch word {
	case "L", "u", "U", "u8", "R", "LR", "uR", "UR", "u8R":
		return true
	}
	return false
}

func (l *lexer) lineComment() {
	for l.i < len(l.source) && l.source[l.i] != '\n' {
		if l.source[l.i] == '\\' && l.i+1 < len(l.source) && l.source[l.i+1] == '\n' {
			l.i++
			l.line++
		}
		l.i++
	}
}

func (l *lexer) blockComment() {
	end := strings.Index(l.source[l.i+2:], "*/")
	stop := len(l.source)
	if end >= 0 {
		stop = l.i + 2 + end + 2
	}
	l.line += strings.Count(l.source[l.i:stop], "\n")
	l.i = stop
}

// quoted steps over a string or character literal starting at the quote. A literal
// ends at its line's end when unterminated.
func (l *lexer) quoted(q byte) {
	l.i++
	for l.i < len(l.source) {
		switch l.source[l.i] {
		case '\\':
			if l.i+1 < len(l.source) && l.source[l.i+1] == '\n' {
				l.line++
			}
			l.i += 2
			continue
		case q:
			l.i++
			return
		case '\n':
			return
		}
		l.i++
	}
	l.i = min(l.i, len(l.source))
}

// rawString steps over R"delim( ... )delim" starting at the quote.
func (l *lexer) rawString() {
	open := strings.IndexByte(l.source[l.i:], '(')
	if open < 0 || open > 17 || strings.ContainsAny(l.source[l.i+1:l.i+open], " \\\n\t\"") {
		l.quoted('"')
		return
	}
	delimiter := ")" + l.source[l.i+1:l.i+open] + "\""
	end := strings.Index(l.source[l.i+open:], delimiter)
	stop := len(l.source)
	if end >= 0 {
		stop = l.i + open + end + len(delimiter)
	}
	l.line += strings.Count(l.source[l.i:stop], "\n")
	l.i = stop
}

// number steps over a numeric literal, digit separators (1'000) and exponents
// (1e+5, 0x1p-3) included.
func (l *lexer) number() {
	source := l.source
	for l.i < len(source) {
		c := source[l.i]
		switch {
		case identifierPart(c) || c == '.':
			l.i++
			if (c|0x20 == 'e' || c|0x20 == 'p') && l.i < len(source) && (source[l.i] == '+' || source[l.i] == '-') {
				l.i++
			}
		case c == '\'' && l.i+1 < len(source) && identifierPart(source[l.i+1]):
			l.i++
		default:
			return
		}
	}
}

// directive reads a preprocessor line from its '#': continuations are joined and
// comments dropped, as the preprocessor does.
func (l *lexer) directive() {
	line := l.line
	var b strings.Builder
	l.i++ // '#'
	for l.i < len(l.source) {
		c := l.source[l.i]
		if c == '\n' {
			break
		}
		switch {
		case c == '\\' && l.i+1 < len(l.source) && l.source[l.i+1] == '\n':
			l.i += 2
			l.line++
			b.WriteByte(' ')
			continue
		case c == '\\' && l.i+2 < len(l.source) && l.source[l.i+1] == '\r' && l.source[l.i+2] == '\n':
			l.i += 3
			l.line++
			b.WriteByte(' ')
			continue
		case c == '/' && l.i+1 < len(l.source) && l.source[l.i+1] == '/':
			l.lineComment()
			continue
		case c == '/' && l.i+1 < len(l.source) && l.source[l.i+1] == '*':
			l.blockComment()
			b.WriteByte(' ')
			continue
		case c == '"' || c == '\'':
			start := l.i
			l.quoted(c)
			b.WriteString(l.source[start:l.i])
			continue
		}
		b.WriteByte(c)
		l.i++
	}
	if l.deadLine(line) {
		return
	}
	text := strings.TrimSpace(b.String())
	word := text
	if j := strings.IndexAny(text, " \t<\"(!"); j >= 0 {
		word = text[:j]
	}
	l.directories = append(l.directories, directive{word: word, rest: strings.TrimSpace(text[len(word):]), line: line, at: len(l.tokens)})
}

// selectBranches picks the tokens of conditional groups the scanner reads. A
// group whose branches do not each balance their braces would leave the scanner
// in the wrong scope if all of them were read, as happens with alternative
// function headers sharing one body:
//
//	#ifdef X
//	void f(int a) {
//	#else
//	void f() {
//	#endif
//
// Only the first branch of such a group is kept; the others' tokens are dropped.
// Groups whose branches all balance keep every branch, as a parser reading all
// branches does.
//
// Implements: REQ-CPP-014
func selectBranches(tokens []token, directories []directive) ([]token, []directive) {
	type branch struct{ from, to int }
	type group struct{ branches []branch }
	var stack []*group
	drop := make([]bool, len(tokens)+1)
	any := false
	// braces[i] is the brace balance of tokens[:i].
	braces := make([]int, len(tokens)+1)
	for i, t := range tokens {
		braces[i+1] = braces[i]
		if t.kind == tPunctuation && t.text == "{" {
			braces[i+1]++
		} else if t.kind == tPunctuation && t.text == "}" {
			braces[i+1]--
		}
	}
	balance := func(b branch) int { return braces[b.to] - braces[b.from] }
	for _, d := range directories {
		switch d.word {
		case "if", "ifdef", "ifndef":
			stack = append(stack, &group{branches: []branch{{from: d.at}}})
		case "elif", "elifdef", "elifndef", "else":
			if n := len(stack); n > 0 {
				g := stack[n-1]
				g.branches[len(g.branches)-1].to = d.at
				g.branches = append(g.branches, branch{from: d.at})
			}
		case "endif":
			n := len(stack)
			if n == 0 {
				continue
			}
			g := stack[n-1]
			stack = stack[:n-1]
			g.branches[len(g.branches)-1].to = d.at
			if len(g.branches) < 2 {
				continue
			}
			neutral := true
			for _, b := range g.branches {
				if balance(b) != 0 {
					neutral = false
					break
				}
			}
			if neutral {
				continue
			}
			for _, b := range g.branches[1:] {
				for i := b.from; i < b.to; i++ {
					drop[i] = true
					any = true
				}
			}
		}
	}
	if !any {
		return tokens, directories
	}
	index := make([]int, len(tokens)+1)
	var kept []token
	for i, t := range tokens {
		index[i] = len(kept)
		if !drop[i] {
			kept = append(kept, t)
		}
	}
	index[len(tokens)] = len(kept)
	for i := range directories {
		directories[i].at = index[directories[i].at]
	}
	return kept, directories
}
