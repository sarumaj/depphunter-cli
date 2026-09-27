package cpp

import "strings"

// Token kinds of the declaration scanner's lexer.
const (
	tIdent = iota
	tPunct // one character, or "::"
	tString
	tChar
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
	src   string
	i     int
	line  int
	dead  []bool
	toks  []token
	dirs  []directive
	bol   bool // only whitespace since the last newline
	cplus bool
}

func lex(src string, dead []bool, cplus bool) ([]token, []directive) {
	l := &lexer{src: src, line: 1, dead: dead, bol: true, cplus: cplus}
	l.toks = make([]token, 0, len(src)/6) // about one token in six bytes of C++
	l.run()
	return l.toks, l.dirs
}

func (l *lexer) deadLine(line int) bool { return line < len(l.dead) && l.dead[line] }

func (l *lexer) emit(kind int, start, line int) {
	if !l.deadLine(line) {
		l.toks = append(l.toks, token{kind: kind, text: l.src[start:l.i], line: line})
	}
}

func identStart(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 || (c|0x20 >= 'a' && c|0x20 <= 'z')
}

func identPart(c byte) bool { return identStart(c) || (c >= '0' && c <= '9') }

func (l *lexer) run() {
	src := l.src
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == '\n':
			l.line++
			l.i++
			l.bol = true
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
			continue
		case c == '\\' && l.i+1 < len(src) && (src[l.i+1] == '\n' || src[l.i+1] == '\r'):
			l.i++ // a continued line outside a directive
			continue
		case c == '/' && l.i+1 < len(src) && src[l.i+1] == '/':
			l.lineComment()
			continue
		case c == '/' && l.i+1 < len(src) && src[l.i+1] == '*':
			l.blockComment()
			continue
		case c == '#' && l.bol:
			l.directive()
			continue
		}
		l.bol = false
		start, line := l.i, l.line
		switch {
		case identStart(c):
			for l.i < len(src) && identPart(src[l.i]) {
				l.i++
			}
			word := src[start:l.i]
			if l.i < len(src) && (src[l.i] == '"' || src[l.i] == '\'') && literalPrefix(word) {
				if src[l.i] == '"' && strings.HasSuffix(word, "R") {
					l.rawString()
				} else {
					l.quoted(src[l.i])
				}
				kind := tString
				if src[l.i-1] == '\'' {
					kind = tChar
				}
				l.emit(kind, start, line)
				continue
			}
			l.emit(tIdent, start, line)
		case c >= '0' && c <= '9' || (c == '.' && l.i+1 < len(src) && src[l.i+1] >= '0' && src[l.i+1] <= '9'):
			l.number()
			l.emit(tNumber, start, line)
		case c == '"':
			l.quoted('"')
			l.emit(tString, start, line)
		case c == '\'':
			l.quoted('\'')
			l.emit(tChar, start, line)
		case c == ':' && l.i+1 < len(src) && src[l.i+1] == ':' && l.cplus:
			l.i += 2
			l.emit(tPunct, start, line)
		case c == '-' && l.i+1 < len(src) && src[l.i+1] == '>':
			l.i += 2
			l.emit(tPunct, start, line)
		default:
			l.i++
			l.emit(tPunct, start, line)
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
	for l.i < len(l.src) && l.src[l.i] != '\n' {
		if l.src[l.i] == '\\' && l.i+1 < len(l.src) && l.src[l.i+1] == '\n' {
			l.i++
			l.line++
		}
		l.i++
	}
}

func (l *lexer) blockComment() {
	end := strings.Index(l.src[l.i+2:], "*/")
	stop := len(l.src)
	if end >= 0 {
		stop = l.i + 2 + end + 2
	}
	l.line += strings.Count(l.src[l.i:stop], "\n")
	l.i = stop
}

// quoted steps over a string or character literal starting at the quote. A literal
// ends at its line's end when unterminated.
func (l *lexer) quoted(q byte) {
	l.i++
	for l.i < len(l.src) {
		switch l.src[l.i] {
		case '\\':
			if l.i+1 < len(l.src) && l.src[l.i+1] == '\n' {
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
	l.i = min(l.i, len(l.src))
}

// rawString steps over R"delim( ... )delim" starting at the quote.
func (l *lexer) rawString() {
	open := strings.IndexByte(l.src[l.i:], '(')
	if open < 0 || open > 17 || strings.ContainsAny(l.src[l.i+1:l.i+open], " \\\n\t\"") {
		l.quoted('"')
		return
	}
	delim := ")" + l.src[l.i+1:l.i+open] + "\""
	end := strings.Index(l.src[l.i+open:], delim)
	stop := len(l.src)
	if end >= 0 {
		stop = l.i + open + end + len(delim)
	}
	l.line += strings.Count(l.src[l.i:stop], "\n")
	l.i = stop
}

// number steps over a numeric literal, digit separators (1'000) and exponents
// (1e+5, 0x1p-3) included.
func (l *lexer) number() {
	src := l.src
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case identPart(c) || c == '.':
			l.i++
			if (c|0x20 == 'e' || c|0x20 == 'p') && l.i < len(src) && (src[l.i] == '+' || src[l.i] == '-') {
				l.i++
			}
		case c == '\'' && l.i+1 < len(src) && identPart(src[l.i+1]):
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
	for l.i < len(l.src) {
		c := l.src[l.i]
		if c == '\n' {
			break
		}
		switch {
		case c == '\\' && l.i+1 < len(l.src) && l.src[l.i+1] == '\n':
			l.i += 2
			l.line++
			b.WriteByte(' ')
			continue
		case c == '\\' && l.i+2 < len(l.src) && l.src[l.i+1] == '\r' && l.src[l.i+2] == '\n':
			l.i += 3
			l.line++
			b.WriteByte(' ')
			continue
		case c == '/' && l.i+1 < len(l.src) && l.src[l.i+1] == '/':
			l.lineComment()
			continue
		case c == '/' && l.i+1 < len(l.src) && l.src[l.i+1] == '*':
			l.blockComment()
			b.WriteByte(' ')
			continue
		case c == '"' || c == '\'':
			start := l.i
			l.quoted(c)
			b.WriteString(l.src[start:l.i])
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
	l.dirs = append(l.dirs, directive{word: word, rest: strings.TrimSpace(text[len(word):]), line: line, at: len(l.toks)})
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
func selectBranches(toks []token, dirs []directive) ([]token, []directive) {
	type branch struct{ from, to int }
	type group struct{ branches []branch }
	var stack []*group
	drop := make([]bool, len(toks)+1)
	any := false
	// braces[i] is the brace balance of toks[:i].
	braces := make([]int, len(toks)+1)
	for i, t := range toks {
		braces[i+1] = braces[i]
		if t.kind == tPunct && t.text == "{" {
			braces[i+1]++
		} else if t.kind == tPunct && t.text == "}" {
			braces[i+1]--
		}
	}
	balance := func(b branch) int { return braces[b.to] - braces[b.from] }
	for _, d := range dirs {
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
		return toks, dirs
	}
	index := make([]int, len(toks)+1)
	var kept []token
	for i, t := range toks {
		index[i] = len(kept)
		if !drop[i] {
			kept = append(kept, t)
		}
	}
	index[len(toks)] = len(kept)
	for i := range dirs {
		dirs[i].at = index[dirs[i].at]
	}
	return kept, dirs
}
