package commonlisp

import (
	"bytes"
	"sort"
	"strings"
)

// Kind is what a datum is.
type Kind uint8

const (
	List    Kind = iota + 1 // (...), vectors #(...) and structures #S(...) too (Tag says which)
	Symbol                  // Text is the symbol's name, Package its package prefix if written
	String                  // Text unescaped; #p"..." pathnames too
	Keyword                 // :kw and #:uninterned, Text without the colon
	Other                   // numbers, characters, bit vectors, #n# references
)

// Node is one datum.
type Node struct {
	Kind Kind
	Text string
	// Package is the package prefix of a qualified symbol (alexandria in
	// alexandria:when-let or alexandria::x), as written.
	Package string
	Line    int
	Kids    []*Node
	// Tag is "vector" for #(...), "s" for #S(...), "eval" for the one-element
	// list #. wraps its form in; "" for a plain list.
	Tag string
}

// Limits that keep pathological input linear: lists nested deeper than
// maxFrames are only counted and prefixes beyond maxPending are ignored.
const (
	maxFrames  = 4096
	maxPending = 64
)

type frame struct {
	node    *Node
	pending []byte
}

type reader struct {
	s      string
	i      int
	starts []int
	stack  []*frame
	extra  int // lists opened past maxFrames, still to be closed
}

// Read reads Common Lisp source (or data written in its syntax: .asd files,
// qlfile.lock) into its top-level forms. It never fails: truncated or
// unbalanced input is read as far as it goes, and what it cannot read is
// skipped.
//
// Reader conditionals read both branches: #+feature and #-feature drop their
// feature expression and keep the form after it, since which features hold
// is known only to the implementation that loads the file. The exceptions are
// expressions that are false (or true) everywhere, the conventional way of
// commenting a form out: #+nil, #+(or) and #-(and) drop the form. #. (read
// time evaluation) keeps its form wrapped in a list tagged "eval".
//
// Implements: REQ-COMMONLISP-010
func Read(source []byte) []*Node {
	r := &reader{s: string(source)}
	r.starts = append(r.starts, 0)
	for i := 0; ; {
		j := bytes.IndexByte(source[i:], '\n')
		if j < 0 {
			break
		}
		i += j + 1
		r.starts = append(r.starts, i)
	}
	r.stack = []*frame{{node: &Node{}}}
	r.run()
	for len(r.stack) > 1 {
		r.close()
	}
	return r.stack[0].node.Kids
}

func (r *reader) line(position int) int {
	return sort.Search(len(r.starts), func(k int) bool { return r.starts[k] > position })
}

func (r *reader) top() *frame { return r.stack[len(r.stack)-1] }

func (r *reader) push(p byte) {
	if f := r.top(); len(f.pending) < maxPending {
		f.pending = append(f.pending, p)
	}
}

// prefixHeads are the forms the quote-like prefixes read as.
var prefixHeads = map[byte]string{
	'\'': "quote", '`': "quasiquote", ',': "unquote", '@': "unquote-splicing", 'f': "function",
}

// add hands a finished datum to the innermost list, through its pending
// prefixes: a discarded form is dropped, a feature expression decides whether
// the next form is, a quote wraps it.
func (r *reader) add(n *Node) {
	f := r.top()
	for len(f.pending) > 0 {
		p := f.pending[len(f.pending)-1]
		f.pending = f.pending[:len(f.pending)-1]
		switch p {
		case ';':
			return
		case '+', '-':
			// n is the feature expression; it decides about the next form.
			if v := feature(n); v != 0 && (v > 0) != (p == '+') {
				r.push(';')
			}
			return
		case '&': // #p, #c, #nA: the datum stands for itself
		case '.':
			n = &Node{Kind: List, Line: n.Line, Tag: "eval", Kids: []*Node{n}}
		default:
			n = &Node{Kind: List, Line: n.Line, Kids: []*Node{{Kind: Symbol, Text: prefixHeads[p], Line: n.Line}, n}}
		}
	}
	f.node.Kids = append(f.node.Kids, n)
}

// feature evaluates a feature expression as far as it can be without an
// implementation: 1 true everywhere, -1 false everywhere, 0 unknown.
func feature(n *Node) int {
	switch n.Kind {
	case Symbol, Keyword:
		if strings.EqualFold(n.Text, "nil") {
			return -1
		}
		return 0
	case List:
		if n.Tag != "" || len(n.Kids) == 0 || n.Kids[0].Kind != Symbol && n.Kids[0].Kind != Keyword {
			return 0
		}
		arguments := n.Kids[1:]
		switch strings.ToLower(n.Kids[0].Text) {
		case "or":
			v := -1
			for _, a := range arguments {
				switch feature(a) {
				case 1:
					return 1
				case 0:
					v = 0
				}
			}
			return v
		case "and":
			v := 1
			for _, a := range arguments {
				switch feature(a) {
				case -1:
					return -1
				case 0:
					v = 0
				}
			}
			return v
		case "not":
			if len(arguments) == 1 {
				return -feature(arguments[0])
			}
		}
	}
	return 0
}

func (r *reader) open(n *Node) {
	if len(r.stack) >= maxFrames {
		r.extra++
		return
	}
	r.stack = append(r.stack, &frame{node: n})
}

// close ends the innermost list and adds it to the one around it.
func (r *reader) close() {
	f := r.top()
	r.stack = r.stack[:len(r.stack)-1]
	r.add(f.node)
}

// delimiter reports whether b ends a token: whitespace and the terminating
// macro characters.
func delimiter(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v', '(', ')', '"', '\'', '`', ',', ';':
		return true
	}
	return false
}

func (r *reader) run() {
	s := r.s
	for r.i < len(s) {
		c := s[r.i]
		start := r.i
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			r.i++
		case ';':
			r.skipLine()
		case '(':
			r.i++
			r.open(&Node{Kind: List, Line: r.line(start)})
		case ')':
			r.i++
			switch {
			case r.extra > 0:
				r.extra--
			case len(r.stack) > 1:
				r.close()
			} // a stray closer at the top level is ignored
		case '"':
			r.i++
			r.add(&Node{Kind: String, Text: r.readString(), Line: r.line(start)})
		case '\'', '`':
			r.i++
			r.push(c)
		case ',':
			r.i++
			if r.i < len(s) && (s[r.i] == '@' || s[r.i] == '.') {
				r.i++
				r.push('@')
			} else {
				r.push(',')
			}
		case '#':
			r.dispatch()
		default:
			r.atom(start)
		}
	}
}

// atom reads a token: a symbol (qualified or not), a keyword or a number.
func (r *reader) atom(start int) {
	raw, name, packageName, colons, escaped := r.token()
	if raw == "" {
		r.i++ // cannot happen: the caller saw a constituent
		return
	}
	line := r.line(start)
	switch {
	case !escaped && number(raw):
		r.add(&Node{Kind: Other, Text: raw, Line: line})
	case !escaped && strings.Trim(raw, ".") == "":
		r.add(&Node{Kind: Other, Text: raw, Line: line}) // the dot of a dotted pair
	case colons > 0 && packageName == "":
		r.add(&Node{Kind: Keyword, Text: name, Line: line})
	default:
		r.add(&Node{Kind: Symbol, Text: name, Package: packageName, Line: line})
	}
}

// token reads a token up to a delimiter: |...| and \ escape, and the first
// unescaped colon (or pair of colons) separates a package prefix. It returns
// the token as written, the name and prefix without escapes, how many
// colons separated them, and whether anything was escaped.
func (r *reader) token() (raw, name, packageName string, colons int, escaped bool) {
	s := r.s
	var b strings.Builder
	start := r.i
	j := r.i
	for j < len(s) && !delimiter(s[j]) {
		switch c := s[j]; c {
		case '\\':
			escaped = true
			if j+1 < len(s) {
				b.WriteByte(s[j+1])
			}
			j += 2
			continue
		case '|':
			escaped = true
			k := strings.IndexByte(s[j+1:], '|')
			if k < 0 {
				b.WriteString(s[j+1:])
				j = len(s)
				continue
			}
			b.WriteString(s[j+1 : j+1+k])
			j += k + 2
			continue
		case ':':
			if colons == 0 {
				packageName = b.String()
				b.Reset()
				colons = 1
				if j+1 < len(s) && s[j+1] == ':' {
					colons = 2
					j++
				}
				j++
				continue
			}
		}
		b.WriteByte(s[j])
		j++
	}
	j = min(j, len(s))
	r.i = j
	return s[start:j], b.String(), packageName, colons, escaped
}

// number reports whether a token reads as a number rather than a symbol:
// digits with a sign, point, ratio slash or exponent marker (1+ and 3d-view
// are symbols).
func number(t string) bool {
	digit := false
	for i := 0; i < len(t); i++ {
		switch c := t[i]; {
		case c >= '0' && c <= '9':
			digit = true
		case c == '.' || c == '/' || c == '+' || c == '-':
		case c == 'e' || c == 'E' || c == 'd' || c == 'D' || c == 'f' || c == 'F' || c == 's' || c == 'S' || c == 'l' || c == 'L':
			if !digit {
				return false
			}
		default:
			return false
		}
	}
	return digit && t[len(t)-1] != '+' && t[len(t)-1] != '-'
}

// dispatch reads what follows a '#': an optional decimal argument, then the
// sub-character.
func (r *reader) dispatch() {
	s := r.s
	start := r.i
	r.i++
	for r.i < len(s) && r.i-start < 16 && s[r.i] >= '0' && s[r.i] <= '9' {
		r.i++
	}
	if r.i >= len(s) {
		return
	}
	c := s[r.i]
	line := r.line(start)
	switch c {
	case '(':
		r.i++
		r.open(&Node{Kind: List, Line: line, Tag: "vector"})
	case '\'':
		r.i++
		r.push('f')
	case '\\':
		r.i++
		r.add(&Node{Kind: Other, Text: `#\` + r.character(), Line: line})
	case ':':
		r.i++
		_, name, _, _, _ := r.token()
		r.add(&Node{Kind: Keyword, Text: name, Line: line})
	case '.':
		r.i++
		r.push('.')
	case '+', '-':
		r.i++
		r.push(c)
	case '|':
		r.i-- // blockComment steps over the two characters #|
		r.blockComment()
	case 'p', 'P', 'c', 'C', 'a', 'A':
		r.i++
		r.push('&')
	case 's', 'S':
		r.i++
		if r.i < len(s) && s[r.i] == '(' {
			r.i++
			r.open(&Node{Kind: List, Line: line, Tag: "s"})
		}
	case '=':
		r.i++ // #1=datum: the label is dropped
	case '#', '*', 'b', 'B', 'o', 'O', 'x', 'X', 'r', 'R':
		// #1# references, #*1010 bit vectors, #b101 #o17 #xFF #36rZ rationals
		r.i = start
		for r.i < len(s) && !delimiter(s[r.i]) && r.i-start < 4096 {
			r.i++
		}
		if r.i == start {
			r.i++
		}
		r.add(&Node{Kind: Other, Text: s[start:r.i], Line: line})
	default:
		// #<...> cannot be read back, and a readtable's own dispatch
		// characters (#?"..." of cl-interpol, #/.../) are unknown: skip the two
		// characters and read on.
		r.i++
	}
}

func (r *reader) skipLine() {
	if j := strings.IndexByte(r.s[r.i:], '\n'); j >= 0 {
		r.i += j + 1
	} else {
		r.i = len(r.s)
	}
}

// blockComment skips a #| |# comment, nested ones included; r.i is at '#'.
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

// readString reads a string's body after its opening quote, up to the unescaped
// closing one (or the end of the input); a backslash escapes any character.
func (r *reader) readString() string {
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
		if body[k] == '\\' && k+1 < len(body) {
			k++
		}
		b.WriteByte(body[k])
	}
	return b.String()
}

// character reads a character after #\: one character (#\( #\;), then, when it
// was a letter or digit, the rest of a name (#\Space, #\Newline, #\U+2603).
func (r *reader) character() string {
	s := r.s
	if r.i >= len(s) {
		return ""
	}
	start := r.i
	r.i++
	for r.i < len(s) && s[r.i]&0xC0 == 0x80 {
		r.i++
	}
	for r.i < len(s) && r.i-start < 64 && !delimiter(s[r.i]) {
		r.i++
	}
	return s[start:r.i]
}
