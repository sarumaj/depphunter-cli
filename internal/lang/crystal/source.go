package crystal

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, in RawImport.Name.
const (
	kindRequire  = "require"
	kindDep      = "dep"
	kindDevDep   = "dev"
	kindLocked   = "locked"
	kindOverride = "override"
	kindMain     = "main"
)

// frame is an open block: a type body (module, class, struct, enum, lib,
// annotation, union), a def or fun, a macro, any other block that `end` closes,
// or a bracket.
type frame struct {
	kind byte   // 't' type, 'l' lib, 'e' enum, 'd' def, 'm' macro, 'b' block, or the bracket
	name string // a type's qualified name
}

// maxFrames bounds how far a closer looks down the stack for its opener, so an
// unbalanced file costs linear time.
const maxFrames = 8

// maxStack bounds the stack itself; deeper frames are counted, not kept.
const maxStack = 256

type parser struct {
	tokens []token
	stack  []frame
	extra  int // frames beyond maxStack
	// recordDo is the index of the `do` of a `record Name ... do` block, whose
	// body is the record's (recordName).
	recordDo   int
	recordName string
	// macroIfs are the open {% if %} / {% for %} / {% begin %} tags.
	macroIfs []macroIf
	symbols  lang.SymbolSet
	ex       *lang.Extraction
}

// macroIf is an open macro block. For an {% if %} or {% unless %} it holds the
// stack as it was before the tag, so each branch starts from it, and the stack
// the first branch ended with, which is what the code after {% end %} sees:
// branches such as `{% if x %} def f(a : T) {% else %} def f(a) {% end %}`
// each open a body the one `end` below closes.
type macroIf struct {
	cond        bool
	before      []frame
	beforeExtra int
	first       []frame
	firstExtra  int
	branched    bool
}

// extractSource reads a Crystal file: its requires and what it defines.
//
// Implements: REQ-CRYSTAL-002, REQ-CRYSTAL-003, REQ-CRYSTAL-011
func extractSource(src []byte) *lang.Extraction {
	p := &parser{tokens: lex(src), ex: &lang.Extraction{}, recordDo: -1}
	p.run()
	p.ex.Symbols = p.symbols.List()
	return p.ex
}

func (p *parser) tok(i int) token {
	if i < 0 || i >= len(p.tokens) {
		return token{kind: kPunct}
	}
	return p.tokens[i]
}

func (p *parser) push(f frame) {
	if len(p.stack) >= maxStack {
		p.extra++
		return
	}
	p.stack = append(p.stack, f)
}

// closeBracket pops up to the bracket opener that c closes.
func (p *parser) closeBracket(open byte) {
	for k := len(p.stack) - 1; k >= 0 && k >= len(p.stack)-maxFrames; k-- {
		if p.stack[k].kind == open {
			p.stack = p.stack[:k]
			return
		}
	}
}

// closeEnd pops up to the innermost frame `end` closes.
func (p *parser) closeEnd() {
	if p.extra > 0 {
		p.extra--
		return
	}
	for k := len(p.stack) - 1; k >= 0 && k >= len(p.stack)-maxFrames; k-- {
		if !isBracket(p.stack[k].kind) {
			p.stack = p.stack[:k]
			return
		}
	}
}

func isBracket(k byte) bool { return k == '(' || k == '[' || k == '{' }

// owner is the qualified name of the type whose body the parser is in, and
// whether that is a place declarations are made: the top level or a type body,
// not a def, a block, a macro or brackets.
func (p *parser) owner() (string, byte, bool) {
	if p.extra > 0 {
		return "", 0, false
	}
	if len(p.stack) == 0 {
		return "", 0, true
	}
	top := p.stack[len(p.stack)-1]
	switch top.kind {
	case 't', 'l', 'e':
		for _, f := range p.stack {
			if f.kind != 't' && f.kind != 'l' && f.kind != 'e' {
				return "", 0, false
			}
		}
		return top.name, top.kind, true
	}
	return "", 0, false
}

// inMacro reports whether the parser is inside a macro definition, whose body is
// a template.
func (p *parser) inMacro() bool {
	for _, f := range p.stack {
		if f.kind == 'm' {
			return true
		}
	}
	return false
}

// keyword reports whether the token at i is a keyword: not a method called on
// something (x.class, range.end), not a named argument (class: "x"), not a
// typed name.
func (p *parser) keyword(i int) bool {
	t := p.tokens[i]
	if t.kind != kIdent || t.label {
		return false
	}
	// A variable or parameter may be called def or macro: `getter def : Def?`.
	if n := p.tok(i + 1); n.kind == kPunct && n.text == ":" {
		return false
	}
	prev := p.tok(i - 1)
	return !(prev.kind == kPunct && (prev.text == "." || prev.text == "&." || prev.text == "::"))
}

// opensIf reports whether an if, unless, while or until at i starts a block
// rather than modifying the statement before it (`return if x`).
func (p *parser) opensIf(i int) bool {
	t := p.tokens[i]
	if i == 0 || t.first {
		return true
	}
	prev := p.tokens[i-1]
	switch prev.kind {
	case kPunct:
		// x[i]? if y: a ? written against its operand is a method's, not a ternary.
		return prev.text != ")" && prev.text != "]" && prev.text != "}" && !(prev.text == "?" && !prev.space)
	case kMacro:
		return true
	case kIdent:
		switch prev.text {
		case "then", "else", "do", "begin", "ensure":
			return p.keyword(i - 1)
		}
	}
	return false
}

func (p *parser) run() {
	for i := 0; i < len(p.tokens); i++ {
		t := p.tokens[i]
		switch t.kind {
		case kPunct:
			switch t.text {
			case "(", "[", "{":
				p.push(frame{kind: t.text[0]})
			case ")":
				p.closeBracket('(')
			case "]":
				p.closeBracket('[')
			case "}":
				p.closeBracket('{')
			}
			continue
		case kConst:
			p.constant(i)
			continue
		case kMacro:
			p.macroTag(t.text)
			continue
		case kIdent:
		default:
			continue
		}
		if !p.keyword(i) {
			continue
		}
		switch t.text {
		case "require":
			if n := p.tok(i + 1); n.kind == kString && !n.interpolate && n.text != "" && !strings.Contains(n.text, "{{") {
				p.ex.Imports = append(p.ex.Imports, lang.RawImport{Spec: n.text, Module: n.text, Name: kindRequire, Line: t.line})
				i++
			}
		case "module", "class", "struct", "enum", "lib", "annotation", "union":
			i = p.typeDecl(i)
		case "def":
			i = p.def(i)
		case "fun":
			i = p.fun(i)
		case "macro":
			i = p.macroDef(i)
		case "if", "unless", "while", "until":
			if p.opensIf(i) {
				p.push(frame{kind: 'b'})
			}
		case "do":
			if i == p.recordDo {
				p.push(frame{kind: 't', name: p.recordName})
			} else {
				p.push(frame{kind: 'b'})
			}
		case "case", "begin":
			p.push(frame{kind: 'b'})
		case "select":
			if p.tok(i + 1).first {
				p.push(frame{kind: 'b'})
			}
		case "end":
			p.closeEnd()
		case "alias", "type", "record":
			p.alias(i)
		case "getter", "getter?", "getter!", "setter", "property", "property?", "property!",
			"class_getter", "class_getter?", "class_getter!", "class_setter", "class_property", "class_property?", "class_property!":
			p.accessors(i)
		}
	}
}

// macroTag follows macro control tags: branches of {% if %} start from the
// same stack, and the first branch's end state is kept.
//
// Implements: REQ-CRYSTAL-003
func (p *parser) macroTag(text string) {
	inner := strings.TrimSuffix(strings.TrimPrefix(text, "{%"), "%}")
	inner = strings.TrimSpace(strings.Trim(strings.TrimSpace(inner), "-"))
	word, _, _ := strings.Cut(inner, " ")
	switch word {
	case "if", "unless":
		if len(p.macroIfs) < maxStack {
			p.macroIfs = append(p.macroIfs, macroIf{cond: true, before: append([]frame(nil), p.stack...), beforeExtra: p.extra})
		}
	case "for", "begin", "verbatim":
		if len(p.macroIfs) < maxStack {
			p.macroIfs = append(p.macroIfs, macroIf{})
		}
	case "else", "elsif":
		if n := len(p.macroIfs); n > 0 && p.macroIfs[n-1].cond {
			m := &p.macroIfs[n-1]
			if !m.branched {
				m.first, m.firstExtra, m.branched = p.stack, p.extra, true
			}
			p.stack, p.extra = append([]frame(nil), m.before...), m.beforeExtra
		}
	case "end":
		if n := len(p.macroIfs); n > 0 {
			m := p.macroIfs[n-1]
			p.macroIfs = p.macroIfs[:n-1]
			if m.branched {
				p.stack, p.extra = m.first, m.firstExtra
			}
		}
	}
}

// qualify names a type declared in owner.
func qualify(owner, name string) string {
	if strings.HasPrefix(name, "::") {
		return strings.TrimPrefix(name, "::")
	}
	if owner == "" {
		return name
	}
	return owner + "::" + name
}

// path reads a constant path (A::B::C) at i, returning it and the index of its
// last token; "" when there is none.
func (p *parser) path(i int) (string, int) {
	var b strings.Builder
	j := i
	if t := p.tok(j); t.kind == kPunct && t.text == "::" {
		b.WriteString("::")
		j++
	}
	for {
		t := p.tok(j)
		if t.kind != kConst {
			return "", i
		}
		b.WriteString(t.text)
		if n := p.tok(j + 1); n.kind == kPunct && n.text == "::" && p.tok(j+2).kind == kConst && !n.space {
			b.WriteString("::")
			j += 2
			continue
		}
		return b.String(), j
	}
}

// typeDecl reads `class A::B`, `module M`, `lib LibC`, `enum E : UInt8` and the
// others; the body is a new frame. A keyword followed by no name (`{{name}}` is
// one) still opens a body when a macro expression names it.
func (p *parser) typeDecl(i int) int {
	kw := p.tokens[i].text
	name, last := p.path(i + 1)
	if name == "" {
		if p.tok(i+1).kind == kExpand {
			p.push(frame{kind: 't'})
			return i + 1
		}
		return i
	}
	owner, ok := "", false
	var ownerKind byte
	if !p.inMacro() {
		owner, ownerKind, ok = p.owner()
	}
	if kw == "union" && ownerKind != 'l' {
		return i // a method called union
	}
	full := qualify(owner, name)
	if ok {
		p.symbols.Add(full, kw, p.tokens[i].line)
	}
	k := byte('t')
	switch kw {
	case "lib":
		k = 'l'
	case "enum":
		k = 'e'
	}
	p.push(frame{kind: k, name: full})
	return last
}

// def reads a method definition: its name (an operator, a setter `x=`, `self.x`)
// and, unless it is abstract, the body it opens.
func (p *parser) def(i int) int {
	abstract := p.tok(i-1).kind == kIdent && p.tok(i-1).text == "abstract"
	j := i + 1
	if t, n := p.tok(j), p.tok(j+1); (t.text == "self" || t.kind == kConst) && n.kind == kPunct && n.text == "." {
		j += 2
	}
	name, last := p.methodName(j)
	owner, _, ok := p.owner()
	if ok && name != "" && !p.inMacro() {
		if owner == "" {
			p.symbols.Add(name, "func", p.tokens[i].line)
		} else {
			p.symbols.Add(owner+"."+name, "method", p.tokens[i].line)
		}
	}
	if !abstract {
		p.push(frame{kind: 'd'})
	}
	return last
}

// methodName reads the name of a def at j: an identifier (with ? or !), a setter
// (name followed directly by =), an operator, [] / []= / []?, or a backtick.
func (p *parser) methodName(j int) (string, int) {
	t := p.tok(j)
	switch t.kind {
	case kIdent, kConst:
		if n := p.tok(j + 1); n.kind == kPunct && n.text == "=" && !n.space {
			if a := p.tok(j + 2); a.kind == kPunct && a.text == "(" && !a.space {
				return t.text + "=", j + 1
			}
		}
		return t.text, j
	case kPunct:
		if t.text == "[" {
			if n := p.tok(j + 1); n.kind == kPunct && n.text == "]" {
				name, last := "[]", j+1
				if a := p.tok(j + 2); a.kind == kPunct && (a.text == "=" || a.text == "?") && !a.space {
					name, last = "[]"+a.text, j+2
				}
				return name, last
			}
			return "", j - 1
		}
		if t.text == "(" || t.text == ";" {
			return "", j - 1
		}
		return t.text, j
	case kString:
		if t.text == "" {
			return "", j
		}
	}
	return "", j - 1
}

// fun reads a C binding (`fun name = real(...) : T` in a lib, no body) or a
// function with a body outside one.
func (p *parser) fun(i int) int {
	owner, k, ok := p.owner()
	t := p.tok(i + 1)
	if t.kind != kIdent && t.kind != kConst {
		return i
	}
	if ok && !p.inMacro() {
		if owner == "" {
			p.symbols.Add(t.text, "func", p.tokens[i].line)
		} else {
			p.symbols.Add(owner+"."+t.text, "func", p.tokens[i].line)
		}
	}
	if k != 'l' || !ok {
		p.push(frame{kind: 'd'})
	}
	return i + 1
}

// macroDef reads a macro definition; its body is a template, not code of the file.
func (p *parser) macroDef(i int) int {
	t := p.tok(i + 1)
	owner, _, ok := p.owner()
	if ok && (t.kind == kIdent || t.kind == kConst) && !p.inMacro() {
		name := t.text
		if owner != "" {
			name = owner + "." + name
		}
		p.symbols.Add(name, "macro", p.tokens[i].line)
	}
	p.push(frame{kind: 'm'})
	if t.kind == kIdent || t.kind == kConst {
		return i + 1
	}
	return i
}

// constant reads `NAME = value` at the start of a statement in a type body or at
// the top level. An enum's members are not constants of their own.
func (p *parser) constant(i int) {
	t := p.tokens[i]
	if !p.statementStart(i) {
		return
	}
	if n := p.tok(i + 1); n.kind != kPunct || n.text != "=" {
		return
	}
	owner, k, ok := p.owner()
	if !ok || k == 'e' || p.inMacro() {
		return
	}
	p.symbols.Add(qualify(owner, t.text), "const", t.line)
}

// statementStart reports whether the token at i begins a statement.
func (p *parser) statementStart(i int) bool {
	if i == 0 || p.tokens[i].first {
		return true
	}
	prev := p.tokens[i-1]
	return prev.kind == kMacro || prev.kind == kPunct && prev.text == ";" ||
		prev.kind == kIdent && (prev.text == "private" || prev.text == "protected")
}

// alias reads `alias Name = T`, a lib's `type Name = T` and `record Name, ...`
// (a struct the record macro writes).
func (p *parser) alias(i int) {
	t := p.tokens[i]
	if !p.statementStart(i) {
		return
	}
	n := p.tok(i + 1)
	if n.kind != kConst {
		return
	}
	owner, k, ok := p.owner()
	if !ok || p.inMacro() {
		return
	}
	switch t.text {
	case "type":
		if k != 'l' {
			return
		}
		p.symbols.Add(qualify(owner, n.text), "type", t.line)
	case "alias":
		p.symbols.Add(qualify(owner, n.text), "type", t.line)
	case "record":
		p.symbols.Add(qualify(owner, n.text), "struct", t.line)
		depth := 0
		for j := i + 2; j < len(p.tokens) && j < i+512 && !p.tokens[j].first; j++ {
			switch c := p.tokens[j]; {
			case c.kind == kPunct && (c.text == "(" || c.text == "[" || c.text == "{"):
				depth++
			case c.kind == kPunct && (c.text == ")" || c.text == "]" || c.text == "}"):
				depth--
			case depth == 0 && c.kind == kIdent && c.text == "do":
				p.recordDo, p.recordName = j, qualify(owner, n.text)
			}
		}
	}
}

// accessors reads `getter name : T`, `property a, b = 1` and their kin in a type
// body: each name is an attribute of the type.
func (p *parser) accessors(i int) {
	t := p.tokens[i]
	if !p.statementStart(i) {
		return
	}
	owner, _, ok := p.owner()
	if !ok || owner == "" || p.inMacro() {
		return
	}
	j := i + 1
	if n := p.tok(j); n.kind == kPunct && n.text == "(" && !n.space {
		j++
	}
	depth := 0
	want := true
	for ; j < len(p.tokens) && j < i+256; j++ {
		n := p.tokens[j]
		if n.first && j > i+1 && depth == 0 {
			return
		}
		switch {
		case want && depth == 0 && (n.kind == kIdent || n.kind == kConst):
			p.symbols.Add(owner+"."+n.text, "attr", t.line)
			want = false
		case want && depth == 0 && n.kind == kVar && strings.HasPrefix(n.text, "@") && !strings.HasPrefix(n.text, "@@"):
			p.symbols.Add(owner+"."+n.text[1:], "attr", t.line)
			want = false
		case n.kind == kPunct:
			switch n.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				if depth--; depth < 0 {
					return
				}
			case ",":
				want = depth == 0
			case ";":
				if depth == 0 {
					return
				}
			}
		default:
			want = false
		}
	}
}
