package beam

import "strings"

// term is a literal read from the token stream of either language: the data of
// mix.exs deps, mix.lock, rebar.config, rebar.lock and .app.src. Whatever is not a
// literal - a function call, an operator expression - is kept as kind 'x' so the
// terms around it still read.
type term struct {
	kind  byte // 'a' atom, 's' string, 'n' number, 'l' list, 't' tuple, 'm' map, 'k' key: value pair, 'x' other
	s     string
	items []term
	line  int
}

func (t term) isAtom(s string) bool { return t.kind == 'a' && t.s == s }

// at is the tuple or list element i, or an empty term.
func (t term) at(i int) term {
	if i < len(t.items) {
		return t.items[i]
	}
	return term{}
}

// opt looks a key up in a keyword list or proplist: [hex: :x] or [{hex, x}]. A
// bare atom in a proplist is true.
func (t term) opt(key string) (term, bool) {
	for _, it := range t.items {
		switch {
		case it.kind == 'k' && it.s == key:
			return it.at(0), true
		case it.kind == 't' && len(it.items) >= 2 && it.items[0].isAtom(key):
			return it.items[1], true
		case it.isAtom(key):
			return term{kind: 'a', s: "true"}, true
		}
	}
	return term{}, false
}

// text is the string or atom a term holds.
func (t term) text() string {
	if t.kind == 's' || t.kind == 'a' || t.kind == 'n' {
		return t.s
	}
	return ""
}

// render writes a term back as source, for import specs.
func (t term) render(erl bool) string {
	var b strings.Builder
	t.write(&b, erl)
	return b.String()
}

func (t term) write(b *strings.Builder, erl bool) {
	list := func(open, close string) {
		b.WriteString(open)
		for i, it := range t.items {
			if i > 0 {
				b.WriteString(", ")
			}
			it.write(b, erl)
		}
		b.WriteString(close)
	}
	switch t.kind {
	case 'a':
		if !erl && t.s != "true" && t.s != "false" && t.s != "nil" {
			b.WriteByte(':')
		}
		b.WriteString(t.s)
	case 's':
		b.WriteString(`"` + t.s + `"`)
	case 'n':
		b.WriteString(t.s)
	case 'l':
		list("[", "]")
	case 't':
		list("{", "}")
	case 'm':
		list("%{", "}")
	case 'k':
		b.WriteString(t.s + ": ")
		t.at(0).write(b, erl)
	default:
		b.WriteString("…")
	}
}

type termParser struct {
	tokens []token
	i      int
	erl    bool
}

func (p *termParser) peek() token {
	if p.i < len(p.tokens) {
		return p.tokens[p.i]
	}
	return token{kind: tPunct, val: "eof"}
}

func (p *termParser) isPunct(v string) bool {
	t := p.peek()
	return t.kind == tPunct && t.val == v
}

// value reads one term, or skips an expression it cannot read up to the next comma
// or closing bracket at its own level.
func (p *termParser) value() term {
	t := p.peek()
	line := t.line
	var v term
	switch {
	case t.kind == tKey:
		p.i++
		v = term{kind: 'k', s: t.val, line: line, items: []term{p.value()}}
		return v
	case t.kind == tAtom:
		p.i++
		v = term{kind: 'a', s: t.val, line: line}
	case t.kind == tIdent && (t.val == "true" || t.val == "false" || t.val == "nil"):
		p.i++
		v = term{kind: 'a', s: t.val, line: line}
	case t.kind == tString:
		p.i++
		v = term{kind: 's', s: t.val, line: line}
	case t.kind == tNum:
		p.i++
		v = term{kind: 'n', s: t.val, line: line}
	case t.kind == tPunct && t.val == "[":
		p.i++
		v = term{kind: 'l', line: line, items: p.seq("]")}
	case t.kind == tPunct && t.val == "{":
		p.i++
		v = term{kind: 't', line: line, items: p.seq("}")}
	case t.kind == tPunct && t.val == "%" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].val == "{":
		p.i += 2
		v = term{kind: 'm', line: line, items: p.seq("}")}
	case t.kind == tPunct && t.val == "<<" && p.erl:
		// <<"name">>: a binary holding one string
		p.i++
		v = term{kind: 'x', line: line}
		if s := p.peek(); s.kind == tString {
			p.i++
			v = term{kind: 's', s: s.val, line: line}
		}
		p.skipTo(">>")
		return v
	default:
		p.skip()
		return term{kind: 'x', line: line}
	}
	// Elixir's "key" => value and Erlang's key => value in maps.
	if p.isPunct("=>") {
		p.i++
		return term{kind: 'k', s: v.s, line: line, items: []term{p.value()}}
	}
	// Anything else continuing the expression (Mix.env() == :prod, "a" <> "b") makes
	// the whole thing other.
	if t := p.peek(); !(t.kind == tPunct && (t.val == "," || t.val == "]" || t.val == "}" || t.val == ")" || t.val == "end" || t.val == ">>" || t.val == "|" || t.val == "eof")) {
		p.skip()
		return term{kind: 'x', line: line}
	}
	return v
}

// seq reads comma-separated terms up to close.
func (p *termParser) seq(close string) []term {
	var out []term
	for p.i < len(p.tokens) {
		if p.isPunct(close) {
			p.i++
			return out
		}
		if p.isPunct(",") || p.isPunct("|") {
			p.i++
			continue
		}
		if t := p.peek(); t.kind == tPunct && (t.val == "]" || t.val == "}" || t.val == ")" || t.val == "end") {
			p.i++ // a mismatched closer: give up on this sequence
			return out
		}
		start := p.i
		out = append(out, p.value())
		if p.i == start {
			p.i++
		}
	}
	return out
}

// skip steps over an expression up to a comma or a closer at depth zero.
func (p *termParser) skip() {
	depth := 0
	for p.i < len(p.tokens) {
		t := p.tokens[p.i]
		if t.kind == tPunct {
			switch t.val {
			case "(", "[", "{", "<<":
				depth++
			case ")", "]", "}", ">>":
				if depth == 0 {
					return
				}
				depth--
			case ",", "|":
				if depth == 0 {
					return
				}
			case "end":
				return
			}
		}
		if t.kind == tIdent && t.val == "do" {
			depth++
		}
		if t.kind == tIdent && t.val == "end" && depth > 0 {
			depth--
		}
		p.i++
	}
}

func (p *termParser) skipTo(v string) {
	for p.i < len(p.tokens) && !p.isPunct(v) {
		p.i++
	}
	p.i++
}

// erlForms reads the terms of a file of Erlang terms, one per form.
func erlForms(src []byte) []term {
	p := &termParser{tokens: lexErlang(src), erl: true}
	var out []term
	for p.i < len(p.tokens) {
		start := p.i
		v := p.value()
		out = append(out, v)
		for p.i < len(p.tokens) && !p.isPunct("end") {
			p.i++
		}
		p.i++
		if p.i <= start {
			p.i = start + 1
		}
	}
	return out
}
