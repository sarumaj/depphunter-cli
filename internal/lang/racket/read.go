package racket

import (
	"bytes"
	"sort"
	"strings"
)

// Kind is what a datum is.
type Kind uint8

const (
	List    Kind = iota + 1 // (), [] and {}; vectors, hash tables and prefab structs too (Tag says which)
	Symbol                  // Text as written, |quoted| parts and \ escapes kept
	String                  // Text unescaped; byte strings too
	Keyword                 // #:kw, Text without "#:"
	Other                   // numbers, booleans, characters, regexps, here strings
)

// Node is one datum.
type Node struct {
	Kind Kind
	Text string
	Line int
	Kids []*Node
	// Tag is "vector", "hash", "hasheq", "s" (prefab) ... for the # forms
	// with parentheses, "" for plain lists.
	Tag string
}

// Head is the symbol a list starts with, or "".
func (n *Node) Head() string {
	if n == nil || n.Kind != List || n.Tag != "" || len(n.Kids) == 0 || n.Kids[0].Kind != Symbol {
		return ""
	}
	return n.Kids[0].Text
}

// Unquote strips quote and quasiquote wrappers: '(a b) is (a b).
func Unquote(n *Node) *Node {
	for n != nil && (n.Head() == "quote" || n.Head() == "quasiquote") && len(n.Kids) == 2 {
		n = n.Kids[1]
	}
	return n
}

// Module is what the reader found in a file.
type Module struct {
	// Lang is the text after #lang (or #!name) on the file's first line of
	// content, trimmed; LangLine its line.
	Lang     string
	LangLine int
	// Reader is the module path a #reader prefix names (DrRacket's saved
	// teaching-language files start with one).
	Reader *Node
	// WXME marks DrRacket's editor format (#reader(lib "read.ss" "wxme")),
	// whose content after the header is not text.
	WXME  bool
	Forms []*Node
}

// Limits that keep pathological input linear: lists nested deeper than
// maxFrames are only counted, a closer only closes a list within maxClose of
// the innermost one, and prefixes beyond maxPending are ignored.
const (
	maxFrames  = 4096
	maxClose   = 8
	maxPending = 64
)

// Modes of a body.
const (
	modeCode = iota // plain s-expressions
	modeAt          // s-expressions with @-forms (#lang at-exp ...)
	modeText        // text with @-forms (#lang scribble/...): Scribble documents
)

type frame struct {
	node    *Node
	closer  byte
	text    bool // the body is text ({...} of an @-form, or a Scribble document)
	alt     bool // |{ ... }| text: nothing inside is read
	depth   int  // plain braces open inside text
	at      bool // an @cmd[...] list: a {text} body may follow its ]
	pending []byte
}

type reader struct {
	s      string
	i      int
	starts []int
	stack  []*frame
	extra  int
	mode   int
	mod    *Module
	seen   bool // a datum (or #lang) was read: a later #lang is not the module's
}

// Read reads a Racket module (or data file). text says how a file without a
// #lang line reads: as a Scribble document (.scrbl) or as s-expressions.
// The reader never fails: truncated or unbalanced input is read as far as it
// goes, and what it cannot read is skipped.
//
// Implements: REQ-RACKET-010
func Read(src []byte, text bool) *Module {
	return read(src, text, true)
}

// read reads src, its header (#lang, #reader) first when header is set.
func read(src []byte, text, header bool) *Module {
	r := &reader{s: string(src), mod: &Module{}}
	r.starts = append(r.starts, 0)
	for i := 0; ; {
		j := bytes.IndexByte(src[i:], '\n')
		if j < 0 {
			break
		}
		i += j + 1
		r.starts = append(r.starts, i)
	}
	if header {
		r.header()
	}
	switch {
	case r.mod.WXME:
		return r.mod
	case scribbleLang(r.mod.Lang), r.mod.Lang == "" && text:
		r.mode = modeText
	case atLang(r.mod.Lang):
		r.mode = modeAt
	}
	r.stack = []*frame{{node: &Node{}, text: r.mode == modeText}}
	r.run()
	for len(r.stack) > 1 {
		r.close()
	}
	r.mod.Forms = r.stack[0].node.Kids
	return r.mod
}

// scribbleLang reports whether a #lang reads its body as text with @-forms.
func scribbleLang(l string) bool {
	first, _, _ := strings.Cut(l, " ")
	return strings.HasPrefix(first, "scribble/") && first != "scribble/reader" || first == "pollen" || strings.HasPrefix(first, "pollen/")
}

// atLang reports whether a #lang reads s-expressions with @-forms.
func atLang(l string) bool {
	return strings.HasPrefix(l, "at-exp ") || l == "at-exp"
}

// header skips what precedes the first datum - blanks, comments, a #! line -
// and reads a #lang line (or #!lang) and a #reader prefix there.
func (r *reader) header() {
	s := r.s
	for r.i < len(s) {
		switch c := s[r.i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			r.i++
		case c == ';':
			r.skipLine()
		case strings.HasPrefix(s[r.i:], "#|"):
			r.blockComment()
		case strings.HasPrefix(s[r.i:], "#!/") || strings.HasPrefix(s[r.i:], "#! "):
			r.skipLine() // a script's interpreter line
		case strings.HasPrefix(s[r.i:], "#lang ") || strings.HasPrefix(s[r.i:], "#lang\t"):
			r.langLine(r.i + 6)
			return
		case strings.HasPrefix(s[r.i:], "#!") && r.i+2 < len(s) && isLangChar(s[r.i+2]):
			r.langLine(r.i + 2) // #!racket/base is #lang racket/base
			return
		case strings.HasPrefix(s[r.i:], "#reader"):
			start := r.i
			r.i += len("#reader")
			sub := read([]byte(r.nextDatumText()), false, false)
			if len(sub.Forms) > 0 {
				r.mod.Reader = sub.Forms[0]
				r.mod.Reader.Line = r.line(start)
				if strings.Contains(strings.ToLower(s[start:r.i]), "wxme") {
					r.mod.WXME = true
				}
			}
			return
		default:
			return
		}
	}
}

func isLangChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

func (r *reader) langLine(from int) {
	end := strings.IndexByte(r.s[from:], '\n')
	if end < 0 {
		end = len(r.s) - from
	}
	l := strings.TrimSpace(r.s[from : from+end])
	if len(l) > 256 {
		l = l[:256]
	}
	r.mod.Lang = strings.Join(strings.Fields(l), " ")
	r.mod.LangLine = r.line(r.i)
	r.seen = true
	r.i = min(from+end+1, len(r.s))
}

// nextDatumText returns the text of the next datum (a #reader's module path)
// and moves past it: a parenthesized form by counting parentheses outside
// strings, else a token.
func (r *reader) nextDatumText() string {
	s := r.s
	for r.i < len(s) && (s[r.i] == ' ' || s[r.i] == '\t') {
		r.i++
	}
	start := r.i
	if r.i < len(s) && (s[r.i] == '(' || s[r.i] == '[') {
		depth, inStr := 0, false
		for ; r.i < len(s) && r.i-start < 4096; r.i++ {
			c := s[r.i]
			switch {
			case inStr && c == '\\':
				r.i++
			case c == '"':
				inStr = !inStr
			case inStr:
			case c == '(' || c == '[':
				depth++
			case c == ')' || c == ']':
				depth--
			}
			if depth == 0 && !inStr {
				r.i++
				break
			}
		}
		r.i = min(r.i, len(s))
		return s[start:r.i]
	}
	r.token()
	return s[start:r.i]
}

func (r *reader) line(pos int) int {
	return sort.Search(len(r.starts), func(k int) bool { return r.starts[k] > pos })
}

func (r *reader) top() *frame { return r.stack[len(r.stack)-1] }

func (r *reader) push(p byte) {
	if f := r.top(); len(f.pending) < maxPending {
		f.pending = append(f.pending, p)
	}
}

// prefixHeads are the forms the quote-like prefixes read as.
var prefixHeads = map[byte]string{
	'\'': "quote", '`': "quasiquote", ',': "unquote", '@': "unquote-splicing",
	's': "syntax", 'S': "quasisyntax", 'u': "unsyntax", 'U': "unsyntax-splicing",
}

// add hands a finished datum to the innermost list, through its pending
// prefixes: #; drops it, a quote wraps it.
func (r *reader) add(n *Node) {
	r.seen = true
	f := r.top()
	for len(f.pending) > 0 {
		p := f.pending[len(f.pending)-1]
		f.pending = f.pending[:len(f.pending)-1]
		switch p {
		case ';':
			return
		case '&': // a box: its content stands for it
		default:
			n = &Node{Kind: List, Line: n.Line, Kids: []*Node{{Kind: Symbol, Text: prefixHeads[p], Line: n.Line}, n}}
		}
	}
	f.node.Kids = append(f.node.Kids, n)
}

func (r *reader) open(n *Node, closer byte) *frame {
	if len(r.stack) >= maxFrames {
		r.extra++
		return nil
	}
	f := &frame{node: n, closer: closer}
	r.stack = append(r.stack, f)
	return f
}

// close ends the innermost list and adds it to the one around it; an @cmd[...]
// followed by { goes on with its text body instead.
func (r *reader) close() {
	f := r.top()
	if f.at && r.mode != modeCode && r.i < len(r.s) && len(r.stack) > 1 {
		if r.s[r.i] == '{' {
			r.i++
			f.at, f.text, f.closer = false, true, '}'
			return
		}
		if strings.HasPrefix(r.s[r.i:], "|{") {
			r.i += 2
			f.at, f.text, f.alt, f.closer = false, true, true, '}'
			return
		}
	}
	r.stack = r.stack[:len(r.stack)-1]
	r.add(f.node)
}

// closeAt handles a closing bracket in code: it ends the nearest list it
// closes and any left open inside it; a closer nothing near opened is ignored.
func (r *reader) closeAt(c byte) {
	if r.extra > 0 {
		r.extra--
		return
	}
	for d := 1; d <= maxClose && d < len(r.stack); d++ {
		if r.stack[len(r.stack)-d].closer == c {
			for ; d > 0; d-- {
				r.close()
			}
			return
		}
	}
}

// delimiter reports whether b ends a symbol or number.
func delimiter(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v', '(', ')', '[', ']', '{', '}', '"', ',', '\'', '`', ';':
		return true
	}
	return false
}

// token reads a symbol or number: up to a delimiter, with |...| quoting and
// \ escaping one character.
func (r *reader) token() string {
	s := r.s
	j := r.i
	for j < len(s) && !delimiter(s[j]) {
		switch s[j] {
		case '\\':
			j += 2
			continue
		case '|':
			if k := strings.IndexByte(s[j+1:], '|'); k >= 0 {
				j += k + 2
			} else {
				j = len(s)
			}
			continue
		}
		j++
	}
	j = min(j, len(s))
	t := s[r.i:j]
	r.i = j
	return t
}

func (r *reader) run() {
	for r.i < len(r.s) {
		if r.top().text {
			r.text()
		} else {
			r.code()
		}
	}
}

// text reads a stretch of text: only @-forms and braces count.
func (r *reader) text() {
	s := r.s
	f := r.top()
	if f.alt {
		// |{ ... }|: nested |{ }| pairs are counted, nothing else is read.
		for r.i < len(s) {
			k := strings.IndexByte(s[r.i:], '|')
			if k < 0 {
				r.i = len(s)
				return
			}
			r.i += k
			switch {
			case strings.HasPrefix(s[r.i:], "|{"):
				f.depth++
				r.i += 2
			case r.i > 0 && s[r.i-1] == '}':
				r.i++
				if f.depth == 0 {
					r.stack = r.stack[:len(r.stack)-1]
					r.add(f.node)
					return
				}
				f.depth--
			default:
				r.i++
			}
		}
		return
	}
	for r.i < len(s) {
		k := strings.IndexAny(s[r.i:], "@{}")
		if k < 0 {
			r.i = len(s)
			return
		}
		r.i += k
		switch s[r.i] {
		case '{':
			f.depth++
			r.i++
		case '}':
			r.i++
			if f.depth > 0 {
				f.depth--
				continue
			}
			if len(r.stack) > 1 {
				r.stack = r.stack[:len(r.stack)-1]
				r.add(f.node)
				return
			}
		case '@':
			r.at()
			if r.top() != f {
				return // a list was opened: it reads in its own mode
			}
		}
	}
}

// at reads an @-form after its @: @cmd, @cmd[datum ...], @cmd{text},
// @cmd[...]{...}, @(datum), @"string", @; and @;{...} comments, @|id|.
func (r *reader) at() {
	s := r.s
	start := r.i
	r.i++
	if r.i >= len(s) {
		return
	}
	switch c := s[r.i]; c {
	case ';':
		r.i++
		if r.i < len(s) && s[r.i] == '{' {
			r.skipBraces()
		} else {
			r.skipLine()
		}
	case '(', '[', '"':
		if r.top().text {
			r.code1() // one datum read as code
		}
	case '{':
		r.i++
		if f := r.open(&Node{Kind: List, Line: r.line(start)}, '}'); f != nil {
			f.text = true
		}
	case '|':
		if k := strings.IndexByte(s[r.i+1:], '|'); k >= 0 && k < 256 {
			r.i += k + 2
		} else {
			r.i++
		}
	default:
		if delimiter(c) || c == '@' || c == '#' || c == '\\' {
			return // a literal @
		}
		cmd := r.command()
		if cmd == "" {
			r.i++
			return
		}
		n := &Node{Kind: List, Line: r.line(start), Kids: []*Node{{Kind: Symbol, Text: cmd, Line: r.line(start)}}}
		switch {
		case r.i < len(s) && s[r.i] == '[':
			r.i++
			if f := r.open(n, ']'); f != nil {
				f.at = true
			}
		case r.i < len(s) && s[r.i] == '{':
			r.i++
			if f := r.open(n, '}'); f != nil {
				f.text = true
			}
		case strings.HasPrefix(s[r.i:], "|{"):
			r.i += 2
			if f := r.open(n, '}'); f != nil {
				f.text, f.alt = true, true
			}
		default:
			r.add(n.Kids[0])
		}
	}
}

// command reads the name after an @: up to a delimiter, a | or an @.
func (r *reader) command() string {
	s := r.s
	j := r.i
	for j < len(s) && j-r.i < 256 && !delimiter(s[j]) && s[j] != '|' && s[j] != '@' {
		j++
	}
	t := s[r.i:j]
	r.i = j
	return t
}

// code1 reads the one datum an @ in text escapes to: an opener pushes a list
// (read as code until it closes), a string is read whole.
func (r *reader) code1() {
	switch r.s[r.i] {
	case '(':
		r.i++
		r.open(&Node{Kind: List, Line: r.line(r.i - 1)}, ')')
	case '[':
		r.i++
		r.open(&Node{Kind: List, Line: r.line(r.i - 1)}, ']')
	case '"':
		start := r.i
		r.i++
		r.add(&Node{Kind: String, Text: r.str(), Line: r.line(start)})
	}
}

// code reads s-expressions until the innermost list becomes text.
func (r *reader) code() {
	s := r.s
	for r.i < len(s) {
		if r.top().text {
			return
		}
		c := s[r.i]
		start := r.i
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			r.i++
		case ';':
			r.skipLine()
		case '(':
			r.i++
			r.open(&Node{Kind: List, Line: r.line(start)}, ')')
		case '[':
			r.i++
			r.open(&Node{Kind: List, Line: r.line(start)}, ']')
		case '{':
			r.i++
			r.open(&Node{Kind: List, Line: r.line(start)}, '}')
		case ')', ']', '}':
			r.i++
			r.closeAt(c)
		case '"':
			r.i++
			r.add(&Node{Kind: String, Text: r.str(), Line: r.line(start)})
		case '\'', '`':
			r.i++
			r.push(c)
		case ',':
			r.i++
			if r.i < len(s) && s[r.i] == '@' {
				r.i++
				r.push('@')
			} else {
				r.push(',')
			}
		case '#':
			r.dispatch()
		case '@':
			if r.mode != modeCode {
				r.at()
				continue
			}
			fallthrough
		default:
			t := r.token()
			if t == "" {
				r.i++
				continue
			}
			kind := Symbol
			if number(t) {
				kind = Other
			} else if strings.IndexByte(t, '|') >= 0 {
				t = strings.ReplaceAll(t, "|", "") // |a b| is the symbol a b
			}
			r.add(&Node{Kind: kind, Text: t, Line: r.line(start)})
		}
	}
}

// number reports whether a token reads as a number rather than a symbol:
// digits with a sign, point, exponent, fraction or complex part (3arg and
// 1+ are symbols).
func number(t string) bool {
	digit := false
	for i := 0; i < len(t); i++ {
		switch c := t[i]; {
		case c >= '0' && c <= '9':
			digit = true
		case c == '.' || c == '/' || c == '+' || c == '-' || c == 'e' || c == 'E' || c == 'i':
		default:
			return false
		}
	}
	return digit && t[len(t)-1] != '+' && t[len(t)-1] != '-'
}

// dispatch reads what follows a '#'.
func (r *reader) dispatch() {
	s := r.s
	start := r.i
	r.i++
	if r.i >= len(s) {
		return
	}
	switch c := s[r.i]; c {
	case '(', '[', '{':
		r.i++
		r.open(&Node{Kind: List, Line: r.line(start), Tag: "vector"}, closerOf(c))
	case '"':
		r.i++
		r.add(&Node{Kind: String, Text: r.str(), Line: r.line(start)})
	case '\\':
		r.i++
		r.add(&Node{Kind: Other, Text: `#\` + r.char(), Line: r.line(start)})
	case ':':
		r.i++
		r.add(&Node{Kind: Keyword, Text: r.token(), Line: r.line(start)})
	case '\'':
		r.i++
		r.push('s')
	case '`':
		r.i++
		r.push('S')
	case ',':
		r.i++
		if r.i < len(s) && s[r.i] == '@' {
			r.i++
			r.push('U')
		} else {
			r.push('u')
		}
	case '&':
		r.i++
		r.push('&')
	case ';':
		r.i++
		r.push(';')
	case '|':
		r.i = start
		r.blockComment()
	case '!':
		if strings.HasPrefix(s[r.i:], "!/") || strings.HasPrefix(s[r.i:], "! ") {
			r.skipLine()
			return
		}
		r.i++
		r.add(&Node{Kind: Other, Text: "#!" + r.token(), Line: r.line(start)})
	case '<':
		if strings.HasPrefix(s[r.i:], "<<") {
			r.i += 2
			r.add(&Node{Kind: Other, Text: r.hereString(), Line: r.line(start)})
			return
		}
		r.i++ // #<...> cannot be read back
	case '%':
		r.i = start
		r.add(&Node{Kind: Symbol, Text: r.token(), Line: r.line(start)})
	default:
		switch {
		case strings.HasPrefix(s[r.i:], "rx\"") || strings.HasPrefix(s[r.i:], "px\""):
			r.i += 3
			r.add(&Node{Kind: Other, Text: "#rx" + r.str(), Line: r.line(start)})
			return
		case strings.HasPrefix(s[r.i:], "rx#\"") || strings.HasPrefix(s[r.i:], "px#\""):
			r.i += 4
			r.add(&Node{Kind: Other, Text: "#rx" + r.str(), Line: r.line(start)})
			return
		case strings.HasPrefix(s[r.i:], "lang ") && !r.seen:
			r.langLine(r.i + 5)
			return
		case strings.HasPrefix(s[r.i:], "reader"):
			r.i += len("reader")
			r.push(';') // the module path of a reader for what follows
			return
		case strings.HasPrefix(s[r.i:], "ci") || strings.HasPrefix(s[r.i:], "cs"):
			if r.i+2 >= len(s) || delimiter(s[r.i+2]) || s[r.i+2] == '#' {
				r.i += 2
				return
			}
		}
		if delimiter(c) {
			return
		}
		// #hash( #hasheq( #s( #fl( #3( ..., graph labels #0= and #0#, and
		// tokens: #t #f #true #e1.5 #x1F.
		j := r.i
		for j < len(s) && j-r.i < 16 && (s[j] >= 'a' && s[j] <= 'z' || s[j] >= '0' && s[j] <= '9') {
			j++
		}
		if j < len(s) && (s[j] == '(' || s[j] == '[' || s[j] == '{') {
			tag := s[r.i:j]
			if tag == "" || tag[0] >= '0' && tag[0] <= '9' {
				tag = "vector"
			}
			r.i = j + 1
			r.open(&Node{Kind: List, Line: r.line(start), Tag: tag}, closerOf(s[j]))
			return
		}
		if j > r.i && j < len(s) && s[r.i] >= '0' && s[r.i] <= '9' && s[j] == '=' {
			r.i = j + 1 // #0=datum: the label is dropped
			return
		}
		r.i = start
		r.add(&Node{Kind: Other, Text: r.token(), Line: r.line(start)})
		if r.i == start {
			r.i++
		}
	}
}

func closerOf(c byte) byte {
	switch c {
	case '[':
		return ']'
	case '{':
		return '}'
	}
	return ')'
}

func (r *reader) skipLine() {
	if j := strings.IndexByte(r.s[r.i:], '\n'); j >= 0 {
		r.i += j + 1
	} else {
		r.i = len(r.s)
	}
}

// blockComment skips a #| |# comment, nested ones included.
func (r *reader) blockComment() {
	s := r.s
	r.i += 2
	depth := 1
	for r.i < len(s) {
		k := strings.IndexAny(s[r.i:], "|#")
		if k < 0 {
			r.i = len(s)
			return
		}
		r.i += k
		switch {
		case strings.HasPrefix(s[r.i:], "|#"):
			r.i += 2
			if depth--; depth == 0 {
				return
			}
		case strings.HasPrefix(s[r.i:], "#|"):
			r.i += 2
			depth++
		default:
			r.i++
		}
	}
}

// skipBraces skips an @;{...} comment: balanced braces.
func (r *reader) skipBraces() {
	s := r.s
	depth := 0
	for r.i < len(s) {
		k := strings.IndexAny(s[r.i:], "{}")
		if k < 0 {
			r.i = len(s)
			return
		}
		r.i += k
		if s[r.i] == '{' {
			depth++
		} else {
			depth--
		}
		r.i++
		if depth <= 0 {
			return
		}
	}
}

// hereString reads #<<EOS: the rest of the line is the terminator, and the
// string ends at a line that is exactly the terminator.
func (r *reader) hereString() string {
	s := r.s
	eol := strings.IndexByte(s[r.i:], '\n')
	if eol < 0 {
		r.i = len(s)
		return ""
	}
	term := strings.TrimSuffix(s[r.i:r.i+eol], "\r")
	r.i += eol + 1
	bodyStart := r.i
	for r.i < len(s) {
		end := strings.IndexByte(s[r.i:], '\n')
		line := s[r.i:]
		if end >= 0 {
			line = s[r.i : r.i+end]
		}
		if strings.TrimSuffix(line, "\r") == term {
			body := s[bodyStart:r.i]
			if end >= 0 {
				r.i += end + 1
			} else {
				r.i = len(s)
			}
			return body
		}
		if end < 0 {
			r.i = len(s)
			break
		}
		r.i += end + 1
	}
	return s[bodyStart:]
}

// str reads a string's body after its opening quote, up to the unescaped
// closing one (or the end of the input), unescaping \" \\ \n \t.
func (r *reader) str() string {
	s := r.s
	j := r.i
	escaped := false
	for j < len(s) && s[j] != '"' {
		if s[j] == '\\' {
			escaped = true
			j++
		}
		j++
	}
	j = min(j, len(s))
	body := s[r.i:j]
	r.i = min(j+1, len(s))
	if !escaped {
		return body
	}
	var b strings.Builder
	for k := 0; k < len(body); k++ {
		if body[k] != '\\' || k+1 >= len(body) {
			b.WriteByte(body[k])
			continue
		}
		k++
		switch body[k] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		default:
			b.WriteByte(body[k])
		}
	}
	return b.String()
}

// char reads a character after #\: one character, then more letters and
// digits when it was one (#\space, #λ, #\nul).
func (r *reader) char() string {
	s := r.s
	if r.i >= len(s) {
		return ""
	}
	start := r.i
	c := s[r.i]
	r.i++
	for r.i < len(s) && s[r.i]&0xC0 == 0x80 {
		r.i++
	}
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
		for r.i < len(s) && r.i-start < 32 && (s[r.i] >= 'a' && s[r.i] <= 'z' || s[r.i] >= 'A' && s[r.i] <= 'Z' || s[r.i] >= '0' && s[r.i] <= '9') {
			r.i++
		}
	}
	return s[start:r.i]
}
