// Package edn reads Clojure source and EDN data (deps.edn, bb.edn, shadow-cljs.edn,
// project.clj) into forms. It is shared by the clojure plugin and by the index's
// repository discovery, which is why it is not part of the plugin.
//
// The reader is written for mapping, not evaluation: it never fails, reads truncated
// or unbalanced input as far as it goes, and simplifies what no caller needs.
// Metadata (^x) is read and dropped, #_ discards the next form, a quote, syntax quote,
// unquote, deref or var quote becomes a list headed by the symbol Clojure reads it as
// (quote, syntax-quote, unquote, unquote-splicing, deref, var), a tagged literal
// keeps its tag, and a reader conditional #?(...) or #?@(...) is replaced by the forms
// of every branch: code read for dependencies wants what any platform loads.
package edn

import (
	"bytes"
	"sort"
	"strings"
)

// Kind is what a form is.
type Kind uint8

const (
	List Kind = iota + 1
	Vector
	Map
	Set
	Symbol
	Keyword // Text without the leading ':' ("::x" keeps one: ":x")
	String  // Text unescaped
	Number
	Char
	Regex
	Other // ##Inf, ##NaN and anything unreadable
)

// Node is one form.
type Node struct {
	Kind Kind
	Text string
	Line int
	Kids []*Node
	// Tag is a tagged literal's tag (#inst "..." has Tag "inst") and "fn" for #(...).
	Tag string
}

// Head is the symbol a list form starts with, or "".
func (n *Node) Head() string {
	if n == nil || n.Kind != List || len(n.Kids) == 0 || n.Kids[0].Kind != Symbol {
		return ""
	}
	return n.Kids[0].Text
}

// Unquote strips quote and syntax-quote wrappers: '[a b] is [a b].
func Unquote(n *Node) *Node {
	for n != nil && (n.Head() == "quote" || n.Head() == "syntax-quote") && len(n.Kids) == 2 {
		n = n.Kids[1]
	}
	return n
}

// Unquoted reports whether a form is an unquote (~x) or unquote-splicing (~@x), the
// computed values of a project.clj that reading cannot know.
func Unquoted(n *Node) bool {
	h := n.Head()
	return h == "unquote" || h == "unquote-splicing"
}

// Get is a map form's value for a keyword key ("paths" for :paths), or nil.
func (n *Node) Get(key string) *Node {
	if n == nil || n.Kind != Map {
		return nil
	}
	for i := 0; i+1 < len(n.Kids); i += 2 {
		if k := n.Kids[i]; k.Kind == Keyword && k.Text == key {
			return n.Kids[i+1]
		}
	}
	return nil
}

// Strings lists the strings of a vector, list or set form (a :paths vector).
func (n *Node) Strings() []string {
	if n == nil {
		return nil
	}
	var out []string
	for _, k := range n.Kids {
		if k.Kind == String {
			out = append(out, k.Text)
		}
	}
	return out
}

// Render writes a small form back as text, for names such as a defmethod's dispatch
// value; anything longer than max runes is cut with "...".
func Render(n *Node, max int) string {
	var b strings.Builder
	render(&b, n, max, 0)
	s := b.String()
	if r := []rune(s); len(r) > max {
		s = string(r[:max]) + "..."
	}
	return s
}

func render(b *strings.Builder, n *Node, max, depth int) {
	if b.Len() > max || depth > 8 {
		return
	}
	switch n.Kind {
	case Keyword:
		b.WriteString(":" + n.Text)
	case String:
		b.WriteString(`"` + n.Text + `"`)
	case Char:
		b.WriteString(`\` + n.Text)
	case Regex:
		b.WriteString(`#"` + n.Text + `"`)
	case List, Vector, Map, Set:
		open, closer := map[Kind]string{List: "(", Vector: "[", Map: "{", Set: "#{"}[n.Kind], map[Kind]string{List: ")", Vector: "]", Map: "}", Set: "}"}[n.Kind]
		b.WriteString(open)
		for i, k := range n.Kids {
			if i > 0 {
				b.WriteByte(' ')
			}
			render(b, k, max, depth+1)
		}
		b.WriteString(closer)
	default:
		b.WriteString(n.Text)
	}
}

// Read reads every top-level form.
//
// Implements: REQ-CLOJURE-011
func Read(src []byte) []*Node {
	var out []*Node
	ReadTop(src, func(n *Node) bool { out = append(out, n); return true })
	return out
}

// Limits that keep pathological input linear: frames nested deeper than maxFrames
// are only counted, a closer only closes a frame within maxClose of the top, and
// prefixes (quotes, metadata, #_) beyond maxPending are ignored.
const (
	maxFrames  = 4096
	maxClose   = 8
	maxPending = 64
)

type prefix struct {
	kind    byte // '\'' quote, '`' syntax-quote, '~' unquote, 'S' unquote-splicing, '@' deref, 'v' var, '^' meta, '_' discard, 't' tag
	tag     string
	gotMeta bool
}

type frame struct {
	node    *Node
	closer  byte
	cond    bool // #?( ... ): its branches replace it
	splice  bool // #?@( ... )
	pending []prefix
}

type reader struct {
	s      string
	i      int
	starts []int // offsets of line starts
	stack  []*frame
	extra  int // frames opened beyond maxFrames, only counted
	emit   func(*Node) bool
	stop   bool
}

// ReadTop calls fn with each top-level form in order until fn returns false.
//
// Implements: REQ-CLOJURE-011
func ReadTop(src []byte, fn func(*Node) bool) {
	r := &reader{s: string(src), emit: fn}
	r.starts = append(r.starts, 0)
	for i := 0; ; {
		j := bytes.IndexByte(src[i:], '\n')
		if j < 0 {
			break
		}
		i += j + 1
		r.starts = append(r.starts, i)
	}
	r.stack = []*frame{{node: &Node{}}}
	r.run()
	for len(r.stack) > 1 && !r.stop {
		r.close() // input ended inside forms: they end with it
	}
}

func (r *reader) line(pos int) int {
	return sort.Search(len(r.starts), func(k int) bool { return r.starts[k] > pos })
}

func (r *reader) top() *frame { return r.stack[len(r.stack)-1] }

// push applies a prefix to the next form.
func (r *reader) push(p prefix) {
	if f := r.top(); len(f.pending) < maxPending {
		f.pending = append(f.pending, p)
	}
}

// add hands a finished form to the innermost frame, through that frame's pending
// prefixes. It reports whether the form was kept (not discarded or taken as meta).
func (r *reader) add(n *Node) bool {
	f := r.top()
	for len(f.pending) > 0 {
		p := &f.pending[len(f.pending)-1]
		switch p.kind {
		case '^':
			if !p.gotMeta {
				p.gotMeta = true
				return false // the metadata itself: dropped
			}
		case '_':
			f.pending = f.pending[:len(f.pending)-1]
			return false
		case 't':
			n.Tag = p.tag
		default:
			head := map[byte]string{'\'': "quote", '`': "syntax-quote", '~': "unquote", 'S': "unquote-splicing", '@': "deref", 'v': "var"}[p.kind]
			n = &Node{Kind: List, Line: n.Line, Kids: []*Node{{Kind: Symbol, Text: head, Line: n.Line}, n}}
		}
		f.pending = f.pending[:len(f.pending)-1]
	}
	if len(r.stack) == 1 {
		if !r.emit(n) {
			r.stop = true
		}
		return true
	}
	f.node.Kids = append(f.node.Kids, n)
	return true
}

func (r *reader) open(kind Kind, closer byte, line int) *frame {
	if len(r.stack) >= maxFrames {
		r.extra++
		return nil
	}
	f := &frame{node: &Node{Kind: kind, Line: line}, closer: closer}
	r.stack = append(r.stack, f)
	return f
}

// close ends the innermost frame and adds its form to the one around it.
func (r *reader) close() {
	f := r.top()
	r.stack = r.stack[:len(r.stack)-1]
	n := f.node
	if !f.cond {
		if n.Kind == Map && strings.HasPrefix(n.Tag, ":") {
			// #:ns{:a 1} qualifies its unqualified keyword keys: {:ns/a 1}.
			ns := n.Tag[1:]
			for i := 0; i < len(n.Kids); i += 2 {
				if k := n.Kids[i]; k.Kind == Keyword && !strings.Contains(k.Text, "/") && ns != ":" && ns != "" {
					k.Text = ns + "/" + k.Text
				}
			}
			n.Tag = ""
		}
		r.add(n)
		return
	}
	// A reader conditional: every branch's form, spliced for #?@. The first form
	// takes the prefixes waiting for the conditional (a #_ drops all of them).
	var forms []*Node
	for i := 1; i < len(n.Kids); i += 2 {
		if f.splice {
			forms = append(forms, n.Kids[i].Kids...)
		} else {
			forms = append(forms, n.Kids[i])
		}
	}
	if len(forms) == 0 || !r.add(forms[0]) {
		return
	}
	for _, k := range forms[1:] {
		if len(r.stack) == 1 {
			if !r.emit(k) {
				r.stop = true
				return
			}
			continue
		}
		t := r.top().node
		t.Kids = append(t.Kids, k)
	}
}

// closeAt handles a closing bracket: it ends the nearest frame it closes, and any
// left open inside it; a closer nothing near the top opened is ignored.
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

// terminating reports whether b ends a symbol, number or keyword.
func terminating(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f', '\v', ',', '(', ')', '[', ']', '{', '}', '"', ';', '@', '^', '`', '~', '\\':
		return true
	}
	return false
}

func (r *reader) token() string {
	j := r.i
	for j < len(r.s) && !terminating(r.s[j]) {
		j++
	}
	t := r.s[r.i:j]
	r.i = j
	return t
}

func (r *reader) run() {
	s := r.s
	for r.i < len(s) && !r.stop {
		c := s[r.i]
		start := r.i
		switch c {
		case ' ', '\t', '\n', '\r', '\f', '\v', ',':
			r.i++
		case ';':
			r.skipLine()
		case '(':
			r.i++
			r.open(List, ')', r.line(start))
		case '[':
			r.i++
			r.open(Vector, ']', r.line(start))
		case '{':
			r.i++
			r.open(Map, '}', r.line(start))
		case ')', ']', '}':
			r.i++
			r.closeAt(c)
		case '"':
			r.i++
			r.add(&Node{Kind: String, Text: r.str(), Line: r.line(start)})
		case '\\':
			r.i++
			r.add(&Node{Kind: Char, Text: r.char(), Line: r.line(start)})
		case ':':
			r.i++
			r.add(&Node{Kind: Keyword, Text: r.token(), Line: r.line(start)})
		case '\'':
			r.i++
			r.push(prefix{kind: '\''})
		case '`':
			r.i++
			r.push(prefix{kind: '`'})
		case '~':
			r.i++
			if r.i < len(s) && s[r.i] == '@' {
				r.i++
				r.push(prefix{kind: 'S'})
			} else {
				r.push(prefix{kind: '~'})
			}
		case '@':
			r.i++
			r.push(prefix{kind: '@'})
		case '^':
			r.i++
			r.push(prefix{kind: '^'})
		case '#':
			r.dispatch()
		default:
			t := r.token()
			if t == "" { // cannot happen: every terminating byte is handled above
				r.i++
				continue
			}
			kind := Symbol
			if t[0] >= '0' && t[0] <= '9' || len(t) > 1 && (t[0] == '-' || t[0] == '+') && t[1] >= '0' && t[1] <= '9' {
				kind = Number
			}
			r.add(&Node{Kind: kind, Text: t, Line: r.line(start)})
		}
	}
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
	case '{':
		r.i++
		r.open(Set, '}', r.line(start))
	case '(':
		r.i++
		if f := r.open(List, ')', r.line(start)); f != nil {
			f.node.Tag = "fn"
		}
	case '"':
		r.i++
		r.add(&Node{Kind: Regex, Text: r.str(), Line: r.line(start)})
	case '_':
		r.i++
		r.push(prefix{kind: '_'})
	case '\'':
		r.i++
		r.push(prefix{kind: 'v'})
	case '^':
		r.i++
		r.push(prefix{kind: '^'})
	case '=':
		r.i++ // read-eval: the form itself is read as it is
	case '!':
		r.skipLine() // a shebang line
	case '#':
		r.i++
		r.add(&Node{Kind: Other, Text: "##" + r.token(), Line: r.line(start)})
	case '?':
		r.i++
		splice := r.i < len(s) && s[r.i] == '@'
		if splice {
			r.i++
		}
		for r.i < len(s) && (s[r.i] == ' ' || s[r.i] == '\t' || s[r.i] == '\n' || s[r.i] == '\r' || s[r.i] == ',') {
			r.i++
		}
		if r.i < len(s) && s[r.i] == '(' {
			r.i++
			if f := r.open(List, ')', r.line(start)); f != nil {
				f.cond, f.splice = true, splice
			}
		}
	case ':':
		r.i++
		ns := ":" + r.nsPrefix()
		if r.i < len(s) && s[r.i] == '{' {
			r.i++
			if f := r.open(Map, '}', r.line(start)); f != nil {
				f.node.Tag = ns
			}
		}
	default:
		if terminating(c) || c == '<' {
			return // #<...> is unreadable; whatever follows is read on its own
		}
		r.push(prefix{kind: 't', tag: r.token()})
	}
}

// nsPrefix reads the namespace of #:ns{...} (":" for #::{...}, the current one).
func (r *reader) nsPrefix() string {
	j := r.i
	for j < len(r.s) && !terminating(r.s[j]) {
		j++
	}
	t := r.s[r.i:j]
	r.i = j
	for r.i < len(r.s) && (r.s[r.i] == ' ' || r.s[r.i] == ',') {
		r.i++
	}
	return t
}

func (r *reader) skipLine() {
	if j := strings.IndexByte(r.s[r.i:], '\n'); j >= 0 {
		r.i += j + 1
	} else {
		r.i = len(r.s)
	}
}

// str reads a string's body after its opening quote, up to the unescaped closing one
// (or the end of the input), unescaping \" \\ \n \t.
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

// char reads a character literal after its backslash: one character, then the rest
// of a name (\newline, A) when it starts with a letter.
func (r *reader) char() string {
	s := r.s
	if r.i >= len(s) {
		return ""
	}
	start := r.i
	r.i++
	for r.i < len(s) && s[r.i]&0xC0 == 0x80 { // the rest of a multi-byte character
		r.i++
	}
	if c := s[start]; c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		for r.i < len(s) && !terminating(s[r.i]) {
			r.i++
		}
	}
	return s[start:r.i]
}
