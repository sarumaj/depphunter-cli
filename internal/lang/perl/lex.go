package perl

import (
	"bytes"
	"strings"
)

// Token kinds. Perl cannot be tokenized without knowing what the parser expects
// next (a "/" divides after an operand and starts a pattern elsewhere; "{" after
// "$h" subscripts), so the lexer keeps the one bit that decides most of it: whether
// the last token was an operand.
const (
	tWord  = iota // identifier or bareword, with its :: parts (Foo::Bar)
	tVar          // a variable with its sigil ($x, @{, %$h, $#a, $Foo::Bar)
	tStr          // '...', "...", `...`, q//, qq//, qx//, a here-document
	tQW           // qw// - its words in words
	tRegex        // m//, qr//, s///, tr///, y///, //
	tNum          // a number
	tPunct        // everything else: ; , => -> ( ) { } [ ] and operators
)

type token struct {
	kind        int
	text        string   // tWord/tVar/tNum/tPunct: as written; tStr: the inner text
	interpolate bool     // tStr: interpolating ("..." and qq)
	words       []string // tQW
	line        int
}

// opWords are words after which an operand, not an operator, follows: a "/" after
// them starts a pattern ("split /,/", "return /x/ ? 1 : 0").
var opWords = map[string]bool{
	"split": true, "grep": true, "map": true, "join": true, "return": true, "and": true, "or": true,
	"not": true, "xor": true, "if": true, "unless": true, "while": true, "until": true, "elsif": true,
	"push": true, "unshift": true, "when": true, "x": true, "lt": true, "gt": true, "le": true,
	"ge": true, "eq": true, "ne": true, "cmp": true, "print": true, "die": true, "warn": true,
	"foreach": true, "for": true, "else": true, "do": true, "eval": true, "defined": true, "ok": true,
	"like": true, "unlike": true, "croak": true, "confess": true, "say": true, "local": true, "my": true,
	"our": true, "state": true,
}

// quoteOps are the quote-like operators; the value is the number of delimited parts.
var quoteOps = map[string]int{"q": 1, "qq": 1, "qw": 1, "qx": 1, "m": 1, "qr": 1, "s": 2, "tr": 2, "y": 2}

type lexer struct {
	src     []byte
	i, line int
	tokens  []token
	operand bool // the last token was an operand
	// heredocs are the terminators of here-documents opened on the current line,
	// whose bodies start at the next line break.
	heredocs []heredoc
	// subHeader is set between "sub" and the body, where "(...)" may be a prototype
	// ("($;$)") that would not lex as code.
	subHeader bool
}

type heredoc struct {
	term   string
	indent bool
	tok    int // index of the token holding the body
}

// lex tokenizes Perl source. It stops at __END__ or __DATA__, skips POD and
// here-document bodies, and returns tokens for any input, however broken.
//
// Implements: REQ-PERL-010
func lex(src []byte) []token {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	l := &lexer{src: src, line: 1}
	l.run()
	return l.tokens
}

func identStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func identByte(c byte) bool { return identStart(c) || c >= '0' && c <= '9' }

func (l *lexer) at(i int) byte {
	if i >= 0 && i < len(l.src) {
		return l.src[i]
	}
	return 0
}

func (l *lexer) emit(t token) {
	l.tokens = append(l.tokens, t)
}

// lastIs reports whether the previous token is the punctuation p.
func (l *lexer) lastIs(p string) bool {
	n := len(l.tokens)
	return n > 0 && l.tokens[n-1].kind == tPunct && l.tokens[n-1].text == p
}

// stmtStart reports whether a statement may start here.
func (l *lexer) stmtStart() bool {
	n := len(l.tokens)
	return n == 0 || l.tokens[n-1].kind == tPunct && (l.tokens[n-1].text == ";" || l.tokens[n-1].text == "{" || l.tokens[n-1].text == "}")
}

// advance moves to j, counting the line breaks passed.
func (l *lexer) advance(j int) {
	j = min(j, len(l.src))
	if j > l.i {
		l.line += bytes.Count(l.src[l.i:j], []byte{'\n'})
		l.i = j
	}
}

func (l *lexer) run() {
	src := l.src
	for l.i < len(src) {
		c := src[l.i]
		switch {
		case c == '\n':
			l.i++
			l.line++
			if len(l.heredocs) > 0 {
				l.bodies()
			}
			if l.at(l.i) == '=' && identStart(l.at(l.i+1)) {
				l.pod()
			}
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.i++
		case c == '#':
			l.toEOL()
		case c == '=' && l.i == 0 && identStart(l.at(1)):
			l.pod()
		case c == 4 || c == 26: // ^D, ^Z end the program text
			return
		case identStart(c):
			if !l.word() {
				return
			}
		case c >= '0' && c <= '9':
			l.number()
		case c == '$':
			l.scalar()
		case c == '@':
			l.array()
		case c == '%' && !l.operand && (identStart(l.at(l.i+1)) || strings.IndexByte("{$^+-:", l.at(l.i+1)) >= 0):
			l.array()
		case c == '*' && !l.operand && (identStart(l.at(l.i+1)) || l.at(l.i+1) == '{'):
			l.array()
		case c == '\'' || c == '"' || c == '`':
			line := l.line
			inner, end := l.delimited(l.i)
			l.advance(end)
			l.emit(token{kind: tStr, text: inner, interpolate: c != '\'', line: line})
			l.operand = true
		case c == '/' && !l.operand:
			line := l.line
			_, end := l.delimited(l.i)
			l.advance(end)
			l.modifiers()
			l.emit(token{kind: tRegex, line: line})
			l.operand = true
		case c == '<' && l.at(l.i+1) == '<' && l.heredoc():
		case c == '<' && !l.operand && l.readline():
		default:
			l.punct()
		}
	}
}

func (l *lexer) toEOL() {
	if j := bytes.IndexByte(l.src[l.i:], '\n'); j >= 0 {
		l.i += j
	} else {
		l.i = len(l.src)
	}
}

// pod skips a POD block: from a line starting "=word" through the line starting
// "=cut", or to the end.
func (l *lexer) pod() {
	for l.i < len(l.src) {
		end := bytes.IndexByte(l.src[l.i:], '\n')
		cut := bytes.HasPrefix(l.src[l.i:], []byte("=cut")) && !identByte(l.at(l.i+4))
		if end < 0 {
			l.i = len(l.src)
			return
		}
		l.i += end + 1
		l.line++
		if cut {
			if l.at(l.i) == '=' && identStart(l.at(l.i+1)) {
				continue // another block right after
			}
			return
		}
	}
}

// bodies skips the here-document bodies pending at a line break, storing each body
// in its token.
func (l *lexer) bodies() {
	docs := l.heredocs
	l.heredocs = nil
	for _, h := range docs {
		start := l.i
		for l.i < len(l.src) {
			end := bytes.IndexByte(l.src[l.i:], '\n')
			line := l.src[l.i:]
			if end >= 0 {
				line = l.src[l.i : l.i+end]
			}
			line = bytes.TrimSuffix(line, []byte("\r"))
			if h.indent {
				line = bytes.TrimLeft(line, " \t")
			}
			done := string(line) == h.term
			if !done && end < 0 {
				l.i = len(l.src)
				break
			}
			body := l.i
			if end < 0 {
				l.i = len(l.src)
			} else {
				l.i += end + 1
				l.line++
			}
			if done {
				l.tokens[h.tok].text = string(l.src[start:body])
				break
			}
		}
	}
}

// heredoc reads "<<"TERM"", "<<'TERM'", "<<~TERM" or "<<TERM" (where an operand is
// expected), queueing the body for the next line break.
func (l *lexer) heredoc() bool {
	j := l.i + 2
	indent := false
	if l.at(j) == '~' {
		indent = true
		j++
	}
	k := j
	for l.at(k) == ' ' || l.at(k) == '\t' {
		k++
	}
	var term string
	interpolate := true
	switch q := l.at(k); {
	case q == '"' || q == '\'' || q == '`':
		end := bytes.IndexByte(l.src[k+1:], q)
		if end < 0 || bytes.IndexByte(l.src[k+1:k+1+end], '\n') >= 0 {
			return false
		}
		term = string(l.src[k+1 : k+1+end])
		interpolate = q != '\''
		j = k + 1 + end + 1
	case k == j && identStart(q) && (!l.operand || indent):
		e := j
		for e < len(l.src) && identByte(l.src[e]) {
			e++
		}
		term = string(l.src[j:e])
		j = e
	case k == j && q == '\\' && identStart(l.at(j+1)):
		e := j + 1
		for e < len(l.src) && identByte(l.src[e]) {
			e++
		}
		term = string(l.src[j+1 : e])
		interpolate = false
		j = e
	default:
		return false
	}
	l.emit(token{kind: tStr, interpolate: interpolate, line: l.line})
	l.heredocs = append(l.heredocs, heredoc{term: term, indent: indent, tok: len(l.tokens) - 1})
	l.i = j
	l.operand = true
	return true
}

// readline reads "<FH>", "<$fh>", "<>" and "<<>>" where an operand is expected.
func (l *lexer) readline() bool {
	j := l.i + 1
	if l.at(j) == '<' {
		if l.at(j+1) == '>' && l.at(j+2) == '>' {
			l.i = j + 3
			l.emit(token{kind: tVar, text: "<<>>", line: l.line})
			l.operand = true
			return true
		}
		return false
	}
	for j < len(l.src) && (identByte(l.src[j]) || l.src[j] == '$' || l.src[j] == ':') {
		j++
	}
	if l.at(j) != '>' {
		return false
	}
	l.emit(token{kind: tVar, text: string(l.src[l.i : j+1]), line: l.line})
	l.i = j + 1
	l.operand = true
	return true
}

// closer is the closing delimiter of a bracketing opener, else the opener itself.
func closer(c byte) byte {
	switch c {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	case '<':
		return '>'
	}
	return c
}

// delimited reads a delimited body starting at the delimiter at j: brackets nest,
// a backslash escapes the next byte. It returns the inner text and the index after
// the closing delimiter (the end of the input when there is none).
func (l *lexer) delimited(j int) (string, int) {
	open := l.src[j]
	cl := closer(open)
	depth := 1
	for k := j + 1; k < len(l.src); k++ {
		switch c := l.src[k]; {
		case c == '\\':
			k++
		case c == cl:
			if depth--; depth == 0 {
				return string(l.src[j+1 : k]), k + 1
			}
		case c == open && cl != open:
			depth++
		}
	}
	return string(l.src[j+1:]), len(l.src)
}

func (l *lexer) modifiers() {
	for l.i < len(l.src) && (l.src[l.i] >= 'a' && l.src[l.i] <= 'z' || l.src[l.i] >= 'A' && l.src[l.i] <= 'Z') {
		l.i++
	}
}

// word reads an identifier, a quote-like operator's operand or __END__; false means
// the program text ended.
func (l *lexer) word() bool {
	start, line := l.i, l.line
	j := l.i
	for {
		for j < len(l.src) && identByte(l.src[j]) {
			j++
		}
		if l.at(j) == ':' && l.at(j+1) == ':' {
			j += 2
			continue
		}
		break
	}
	w := string(l.src[start:j])
	if w == "__END__" || w == "__DATA__" {
		return false
	}
	if parts, ok := quoteOps[w]; ok && l.quoteContext(start, j) {
		l.i = j
		l.quote(w, parts, line)
		return true
	}
	if w == "format" && l.stmtStart() && l.format(j) {
		return true
	}
	l.i = j
	l.emit(token{kind: tWord, text: w, line: line})
	switch w {
	case "sub", "method", "fun":
		l.subHeader = !l.lastWasArrow()
	}
	l.operand = !opWords[w]
	return true
}

func (l *lexer) lastWasArrow() bool {
	n := len(l.tokens)
	return n > 1 && l.tokens[n-2].kind == tPunct && l.tokens[n-2].text == "->"
}

// quoteContext reports whether a quote-like word at [start, end) is the operator
// and not a name: a method (->s), a hash key ({s} or s => 1), a file test (-s $f)
// or a word followed by something that cannot open a quote.
func (l *lexer) quoteContext(start, end int) bool {
	if l.lastIs("->") || l.at(start-1) == '-' {
		return false
	}
	if n := len(l.tokens); n > 0 && l.tokens[n-1].kind == tWord && l.tokens[n-1].text == "sub" {
		return false
	}
	k := end
	space := false
	for k < len(l.src) && (l.src[k] == ' ' || l.src[k] == '\t' || l.src[k] == '\n' || l.src[k] == '\r') {
		k++
		space = true
	}
	d := l.at(k)
	switch {
	case k >= len(l.src), identByte(d), d == '#' && space:
		return false
	case d == '=' && l.at(k+1) == '>', d == ',', d == ';', d == ')', d == ']', d == '}', d == '>' && space, d == '=' && space:
		return false
	}
	return true
}

// quote reads a quote-like operator's parts from after its name.
func (l *lexer) quote(op string, parts int, line int) {
	for l.i < len(l.src) && (l.src[l.i] == ' ' || l.src[l.i] == '\t' || l.src[l.i] == '\n' || l.src[l.i] == '\r') {
		l.advance(l.i + 1)
	}
	if l.i >= len(l.src) {
		return
	}
	open := l.src[l.i]
	inner, end := l.delimited(l.i)
	l.advance(end)
	if parts == 2 && l.i <= len(l.src) {
		if closer(open) != open {
			// s{...}{...}: the second part has delimiters of its own, maybe after
			// whitespace and comments.
			for l.i < len(l.src) {
				c := l.src[l.i]
				if c == '#' {
					l.toEOL()
					continue
				}
				if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
					break
				}
				l.advance(l.i + 1)
			}
			if l.i < len(l.src) {
				_, end = l.delimited(l.i)
				l.advance(end)
			}
		} else if l.i > 0 && l.i <= len(l.src) {
			_, end = l.delimited(l.i - 1)
			l.advance(end)
		}
	}
	switch op {
	case "q", "qq", "qx":
		l.emit(token{kind: tStr, text: inner, interpolate: op != "q", line: line})
	case "qw":
		l.emit(token{kind: tQW, words: strings.Fields(inner), line: line})
	default:
		l.modifiers()
		l.emit(token{kind: tRegex, line: line})
	}
	l.operand = true
}

// format skips a format definition ("format NAME =" to a line holding a single
// "."), reporting whether there was one.
func (l *lexer) format(j int) bool {
	k := j
	for k < len(l.src) && (l.src[k] == ' ' || l.src[k] == '\t' || identByte(l.src[k]) || l.src[k] == ':') {
		k++
	}
	if l.at(k) != '=' || l.at(k+1) == '>' || l.at(k+1) == '=' {
		return false
	}
	k++
	for l.at(k) == ' ' || l.at(k) == '\t' || l.at(k) == '\r' {
		k++
	}
	if l.at(k) != '\n' {
		return false
	}
	l.advance(k + 1)
	for l.i < len(l.src) {
		end := bytes.IndexByte(l.src[l.i:], '\n')
		line := l.src[l.i:]
		if end >= 0 {
			line = l.src[l.i : l.i+end]
		}
		if end < 0 {
			l.i = len(l.src)
			break
		}
		l.advance(l.i + end + 1)
		if string(bytes.TrimSuffix(line, []byte("\r"))) == "." {
			break
		}
	}
	l.emit(token{kind: tPunct, text: ";", line: l.line})
	l.operand = false
	return true
}

func (l *lexer) number() {
	j := l.i
	if l.src[j] == '0' && (l.at(j+1) == 'x' || l.at(j+1) == 'X' || l.at(j+1) == 'b' || l.at(j+1) == 'B') {
		j += 2
		for j < len(l.src) && (identByte(l.src[j])) {
			j++
		}
	} else {
		for j < len(l.src) && (l.src[j] >= '0' && l.src[j] <= '9' || l.src[j] == '_') {
			j++
		}
		if l.at(j) == '.' && l.at(j+1) >= '0' && l.at(j+1) <= '9' {
			j++
			for j < len(l.src) && (l.src[j] >= '0' && l.src[j] <= '9' || l.src[j] == '_' || l.src[j] == '.' && l.at(j+1) != '.') {
				j++
			}
		}
		if (l.at(j) == 'e' || l.at(j) == 'E') && (l.at(j+1) >= '0' && l.at(j+1) <= '9' || (l.at(j+1) == '-' || l.at(j+1) == '+') && l.at(j+2) >= '0' && l.at(j+2) <= '9') {
			j += 2
			for j < len(l.src) && l.src[j] >= '0' && l.src[j] <= '9' {
				j++
			}
		}
	}
	l.emit(token{kind: tNum, text: string(l.src[l.i:j]), line: l.line})
	l.i = j
	l.operand = true
}

// name reads a variable name at j: an identifier with :: parts, "::name", "^W",
// digits. It returns the end, or j when there is none.
func (l *lexer) name(j int) int {
	switch c := l.at(j); {
	case identStart(c) || c == ':' && l.at(j+1) == ':':
		for {
			for j < len(l.src) && identByte(l.src[j]) {
				j++
			}
			if l.at(j) == ':' && l.at(j+1) == ':' {
				j += 2
				continue
			}
			return j
		}
	case c == '^' && (l.at(j+1) >= 'A' && l.at(j+1) <= 'Z' || l.at(j+1) == '_' || l.at(j+1) == '['):
		return j + 2
	case c >= '0' && c <= '9':
		for j < len(l.src) && l.src[j] >= '0' && l.src[j] <= '9' {
			j++
		}
		return j
	}
	return j
}

// scalar reads "$name", "$#array", "$$ref", "${", and the punctuation variables
// ("$;", "$/", "$'"), which must not start a comment, a pattern or a string.
func (l *lexer) scalar() {
	j := l.i + 1
	if l.at(j) == '#' {
		if n := l.at(j + 1); n == '{' || n == '$' || identStart(n) {
			j++
		} else {
			l.emitVar(j + 1)
			return
		}
	}
	for l.at(j) == '$' && (identStart(l.at(j+1)) || l.at(j+1) == '{' || l.at(j+1) == '$' || l.at(j+1) == ':' && l.at(j+2) == ':') {
		j++
	}
	if e := l.name(j); e > j {
		l.emitVar(e)
		return
	}
	switch c := l.at(j); {
	case c == '{', c == 0, c == ' ', c == '\t', c == '\n', c == '\r', c == ')', c == '(', c == ',':
		l.emitVar(j) // "${ ... }" or a bare "$" (a signature's placeholder)
	default:
		l.emitVar(j + 1) // a punctuation variable
	}
}

// array reads "@name", "@{", "@$ref", "%name", "%$ref", "*glob" and the like.
func (l *lexer) array() {
	j := l.i + 1
	for l.at(j) == '$' && (identStart(l.at(j+1)) || l.at(j+1) == '{' || l.at(j+1) == '$' || l.at(j+1) == ':') {
		j++
	}
	if e := l.name(j); e > j {
		l.emitVar(e)
		return
	}
	switch c := l.at(j); {
	case c == '$':
		l.emitVar(j + 1)
	case (c == '-' || c == '+') && l.src[l.i] != '*':
		l.emitVar(j + 1)
	default:
		l.emitVar(j)
	}
}

func (l *lexer) emitVar(end int) {
	end = min(end, len(l.src))
	l.emit(token{kind: tVar, text: string(l.src[l.i:end]), line: l.line})
	l.i = end
	l.operand = true
}

// multi are the punctuation tokens longer than one byte that matter to the reader
// ("=>" quotes the word before it, "->" makes the next word a method), by their
// first byte.
var multi = map[byte][]string{
	'=': {"=>", "=~", "=="}, '-': {"->", "--"}, '!': {"!~", "!="}, ':': {"::"}, '+': {"++"},
	'&': {"&&"}, '|': {"||"}, '/': {"//"}, '<': {"<="}, '>': {">="}, '.': {"..."},
}

func (l *lexer) punct() {
	rest := l.src[l.i:]
	text := string(rest[:1])
	for _, m := range multi[rest[0]] {
		if len(rest) >= len(m) && string(rest[:len(m)]) == m {
			text = m
			break
		}
	}
	if l.subHeader && text == "(" && l.prototype() {
		return
	}
	l.emit(token{kind: tPunct, text: text, line: l.line})
	l.i += len(text)
	switch text {
	case ")", "]", "}":
		l.operand = true
	case "++", "--":
		// postfix keeps an operand an operand, prefix leaves none
	default:
		l.operand = false
	}
	if text == "{" || text == ";" || text == "=" {
		l.subHeader = false
	}
}

// prototype skips a sub's prototype "($;\@)" as one token pair, since "$;" or "$)"
// would otherwise read as variables.
func (l *lexer) prototype() bool {
	j := l.i + 1
	for j < len(l.src) && strings.IndexByte("$@%&*;\\[]+_ \t", l.src[j]) >= 0 {
		j++
	}
	if l.at(j) != ')' {
		return false
	}
	l.emit(token{kind: tPunct, text: "(", line: l.line})
	l.emit(token{kind: tPunct, text: ")", line: l.line})
	l.i = j + 1
	l.operand = true
	return true
}
