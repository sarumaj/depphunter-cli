package swift

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// keywords are the words that cannot name a type where a type is expected, and
// that the parser reads as syntax where a statement or expression starts.
var keywords = set(
	"associatedtype", "class", "deinit", "enum", "extension", "fileprivate", "func",
	"import", "init", "inout", "internal", "let", "open", "operator", "private",
	"precedencegroup", "protocol", "public", "rethrows", "static", "struct",
	"subscript", "typealias", "var", "break", "case", "catch", "continue", "default",
	"defer", "do", "else", "fallthrough", "for", "guard", "if", "in", "repeat",
	"return", "throw", "switch", "where", "while", "as", "false", "is", "nil",
	"self", "super", "throws", "true", "try", "await", "async",
)

// operandKeywords are keywords that are values: an expression may end with them.
var operandKeywords = set("self", "super", "nil", "true", "false")

// declarationKeywords start a declaration.
var declarationKeywords = set("import", "class", "struct", "enum", "actor", "protocol",
	"extension", "func", "init", "deinit", "subscript", "var", "let", "typealias",
	"associatedtype", "operator", "precedencegroup", "macro")

// modifiers may precede a declaration keyword.
var modifiers = set("public", "private", "fileprivate", "internal", "open", "package",
	"static", "class", "final", "override", "mutating", "nonmutating", "lazy", "weak",
	"unowned", "dynamic", "optional", "required", "convenience", "indirect",
	"nonisolated", "isolated", "distributed", "prefix", "postfix", "infix",
	"consuming", "borrowing", "__consuming", "__owned", "__shared", "sending", "unsafe")

// typePrefixes may precede a type: some View, inout Int, ~Copyable, each T.
var typePrefixes = set("some", "any", "inout", "borrowing", "consuming", "sending",
	"isolated", "each", "repeat", "__owned", "__shared", "nonisolated")

// scope is what a brace opens.
type scope uint8

const (
	sTop      scope = iota // the file
	sType                  // a class, struct, actor or extension body
	sEnum                  // an enum body, where `case` declares cases
	sProtocol              // a protocol body, whose properties are requirements
	sBody                  // code: a function, accessor, closure or statement body
)

// parser reads declarations, imports and type references from a token stream. It is
// a tolerant recursive descent: every step consumes at least one token, and what it
// does not understand it reads as an expression.
type parser struct {
	source []byte
	tokens []token
	i      int

	owner []string // the types around the current declaration, outermost first
	body  int      // how many code bodies are around it; > 0 hides declarations
	depth int      // how deep the parser has recursed (maxDepth)

	imports    []lang.RawImport
	symbols    lang.SymbolSet
	declared   map[string]bool
	references []reference
}

type reference struct {
	name string
	line int
}

// parse reads source.
//
// Implements: REQ-SWIFT-014
func parse(source []byte) *parser {
	p := &parser{source: source, tokens: branches(tokenize(source)), declared: map[string]bool{}}
	for !p.atEnd() {
		p.statement(sTop)
		if p.is("}") || p.is(")") || p.is("]") {
			p.i++ // a closing bracket nothing opened
		}
	}
	return p
}

// branches keeps every branch of an #if whose branches each leave the braces as
// they found them, and only the first branch of one that does not: a declaration
// whose header differs by platform (`#if os(iOS)` / `extension X: UIView {` /
// `#else` / `extension X: NSView {` / `#endif`) would otherwise open two bodies and
// close one.
//
// Implements: REQ-SWIFT-002
func branches(tokens []token) []token {
	var out []token
	i := 0
	for i < len(tokens) {
		out = append(out, branch(tokens, &i)...)
		if i < len(tokens) {
			i++ // a directive without its #if
		}
	}
	return out
}

// branch reads tokens up to a directive that ends the current branch.
func branch(tokens []token, i *int) []token {
	var out []token
	for *i < len(tokens) {
		t := tokens[*i]
		if t.kind != tDirective {
			out = append(out, t)
			*i++
			continue
		}
		if t.text != "if" {
			return out
		}
		*i++
		alternatives := [][]token{branch(tokens, i)}
		for *i < len(tokens) && (tokens[*i].text == "elseif" || tokens[*i].text == "else") {
			*i++
			alternatives = append(alternatives, branch(tokens, i))
		}
		if *i < len(tokens) && tokens[*i].text == "endif" {
			*i++
		}
		balanced := true
		for _, a := range alternatives {
			balanced = balanced && braceDelta(a) == 0
		}
		if !balanced {
			alternatives = alternatives[:1]
		}
		for _, a := range alternatives {
			out = append(out, a...)
		}
	}
	return out
}

func braceDelta(tokens []token) int {
	d := 0
	for _, t := range tokens {
		if t.kind == tPunctuation {
			switch t.text {
			case "{":
				d++
			case "}":
				d--
			}
		}
	}
	return d
}

// atEnd reports whether the tokens are used up. Steps past the end are harmless: token
// and peek read the zero token there.
func (p *parser) atEnd() bool { return p.i >= len(p.tokens) }

func (p *parser) token() token {
	if p.i < len(p.tokens) {
		return p.tokens[p.i]
	}
	return token{kind: tPunctuation}
}

func (p *parser) peek(k int) token {
	if p.i+k >= 0 && p.i+k < len(p.tokens) {
		return p.tokens[p.i+k]
	}
	return token{kind: tPunctuation}
}

// is reports whether the current token is the punctuator s.
func (p *parser) is(s string) bool {
	return p.i < len(p.tokens) && p.tokens[p.i].kind == tPunctuation && p.tokens[p.i].text == s
}

// word reports whether t is the unquoted identifier or keyword w.
func word(t token, w string) bool { return t.kind == tIdentifier && !t.bt && t.text == w }

func punctuation(t token, s string) bool { return t.kind == tPunctuation && t.text == s }

// name reports whether t can name something: an identifier that is not a keyword.
func name(t token) bool { return t.kind == tIdentifier && (t.bt || !keywords[t.text]) }

func (p *parser) reference(t token) {
	if n := typeName(t.text); n != "" && !t.bt {
		p.references = append(p.references, reference{n, t.line})
	}
}

// visible reports whether a declaration here can be named from elsewhere: it is not
// inside a body.
func (p *parser) visible() bool { return p.body == 0 }

func (p *parser) qualified(n string) string { return qualify(strings.Join(p.owner, "."), n) }

// statement reads one statement or declaration in a scope of the given kind.
func (p *parser) statement(s scope) {
	start := p.i
	for p.is("@") {
		p.attribute(false)
	}
	j := p.i
	for p.i < len(p.tokens) && p.modifier() {
	}
	t := p.token()
	declaration := t.kind == tIdentifier && !t.bt && (declarationKeywords[t.text] || t.text == "case" || t.text == "default")
	switch {
	case !declaration:
	case t.text == "default":
	case punctuation(p.peek(1), ":"):
		declaration = false // a label: `for: x`
	case t.text == "actor" || t.text == "macro":
		declaration = name(p.peek(1)) && !p.peek(1).newline
	}
	if !declaration {
		p.i = j // modifiers that modify nothing were words: `open(url)`
		if !p.expression(false, false, false) && p.i == start {
			p.i++
		}
		return
	}
	switch t.text {
	case "import":
		p.importDeclaration(start)
	case "class", "struct", "enum", "actor", "protocol", "extension":
		if !p.typeDeclaration() {
			p.i = j
			p.expression(false, false, false)
		}
	case "func", "macro":
		p.functionDeclaration()
	case "init":
		p.initDeclaration()
	case "deinit", "subscript":
		p.i++
		p.genericParameters()
		p.signature()
		p.bodyBlock()
	case "var", "let":
		p.property(s)
	case "typealias":
		p.i++
		n := p.token()
		if name(n) {
			p.i++
			if p.visible() {
				p.symbols.Add(p.qualified(n.text), "type", n.line)
				p.declared[n.text] = true
			}
		}
		p.genericParameters()
		if p.is("=") {
			p.i++
			p.parseType()
		}
	case "associatedtype":
		p.i++
		if name(p.token()) {
			p.i++
		}
		p.inheritance()
		if p.is("=") {
			p.i++
			p.parseType()
		}
		p.where()
	case "case":
		p.i++
		if s == sEnum {
			p.enumCases()
		} else {
			p.expression(false, true, false)
			if p.is(":") {
				p.i++
			}
		}
	case "default":
		p.i++
		if p.is(":") {
			p.i++
		}
	case "operator":
		// infix operator <>: AdditionPrecedence - names no types.
		p.i++
		for !p.atEnd() && !p.token().newline && !p.is("{") && !p.is("}") {
			p.i++
		}
		if p.is("{") {
			p.skipGroup()
		}
	case "precedencegroup":
		p.i++
		for !p.atEnd() && !p.is("{") && !p.is("}") && !p.token().newline {
			p.i++
		}
		if p.is("{") {
			p.skipGroup()
		}
	}
}

// modifier consumes a declaration modifier (with its argument: private(set)) when a
// declaration keyword or another modifier follows it.
func (p *parser) modifier() bool {
	t := p.token()
	if t.kind != tIdentifier || t.bt || !modifiers[t.text] {
		return false
	}
	k := 1
	if punctuation(p.peek(1), "(") && !p.peek(1).newline {
		if p.peek(2).kind == tIdentifier && punctuation(p.peek(3), ")") {
			k = 4
		} else {
			return false
		}
	}
	n := p.peek(k)
	if n.newline {
		return false
	}
	if n.kind != tIdentifier || n.bt || !(declarationKeywords[n.text] || modifiers[n.text] || n.text == "case") {
		if !punctuation(n, "@") {
			return false
		}
	}
	if t.text == "class" && !(modifiers[n.text] || n.text == "func" || n.text == "var" || n.text == "let" || n.text == "subscript" || n.text == "init" || n.text == "typealias") {
		return false // class Foo: a class, not a modifier
	}
	p.i += k
	for p.is("@") {
		p.attribute(false)
	}
	return true
}

// attribute reads @Name, @Name.Sub or @Name(arguments): the name is a type the code
// uses (@MainActor, @Published). In a type (@Sendable (Int) -> Void) a parenthesis
// only holds arguments when it touches the name.
func (p *parser) attribute(inType bool) {
	at := p.token()
	p.i++
	t := p.token()
	if t.kind != tIdentifier || t.start != at.end {
		return
	}
	p.reference(t)
	p.i++
	for p.is(".") && p.peek(1).kind == tIdentifier {
		p.i += 2
	}
	if p.is("<") {
		p.genericArguments()
	}
	if p.is("(") && !p.token().newline && (!inType || p.token().start == p.peek(-1).end) {
		p.group(")")
	}
}

// importDeclaration reads an import declaration starting at token start (its attributes and
// modifiers).
func (p *parser) importDeclaration(start int) {
	p.i++
	for !p.atEnd() && !p.token().newline && (p.token().kind == tIdentifier || p.is(".")) {
		p.i++
	}
	text := string(p.source[p.tokens[start].start:p.tokens[min(p.i, len(p.tokens))-1].end])
	if rawImport, ok := parseImport(text); ok {
		rawImport.Line = p.tokens[start].line
		p.imports = append(p.imports, rawImport)
	}
}

// typeDeclaration reads a class, struct, enum, actor, protocol or extension. It reports
// false when the keyword declares nothing (`class` in `protocol P: class`).
func (p *parser) typeDeclaration() bool {
	keyword := p.token().text
	p.i++
	n := p.token()
	if !name(n) && !(keyword == "extension" && n.kind == tIdentifier) {
		return false
	}
	var typeName string
	if keyword == "extension" {
		from := p.i
		p.parseType()
		typeName = typePath(string(p.source[p.tokens[from].start:p.tokens[max(min(p.i, len(p.tokens))-1, from)].end]))
		if i := strings.IndexAny(typeName, "?!&"); i >= 0 {
			typeName = typeName[:i]
		}
	} else {
		p.i++
		typeName = typePath(n.text)
		p.genericParameters()
	}
	p.inheritance()
	p.where()
	if p.visible() {
		if keyword != "extension" {
			p.declared[typeName] = true
		}
		kind := typeKinds[keyword]
		if keyword == "protocol" {
			kind = "interface"
		}
		p.symbols.Add(p.qualified(typeName), kind, n.line)
	}
	if !p.is("{") {
		return true
	}
	p.i++
	inner := map[string]scope{"enum": sEnum, "protocol": sProtocol}[keyword]
	if inner == 0 {
		inner = sType
	}
	p.owner = append(p.owner, typeName)
	p.block(inner)
	p.owner = p.owner[:len(p.owner)-1]
	return true
}

// block reads statements up to the closing brace, which it consumes. The opening
// brace has been read.
func (p *parser) block(s scope) {
	if !p.enter() {
		p.i--
		p.skipGroup()
		return
	}
	defer p.leave()
	if s == sBody {
		p.body++
		defer func() { p.body-- }()
	}
	for !p.atEnd() {
		if p.is("}") {
			p.i++
			return
		}
		if p.is(")") || p.is("]") {
			p.i++
			continue
		}
		p.statement(s)
	}
}

// bodyBlock reads a code body if one follows.
func (p *parser) bodyBlock() {
	if p.is("{") {
		p.i++
		p.block(sBody)
	}
}

// functionDeclaration reads a function (or macro) declaration.
func (p *parser) functionDeclaration() {
	keyword := p.token().text
	p.i++
	n := p.token()
	if n.kind == tIdentifier {
		p.i++
		if keyword == "func" && p.visible() {
			switch owner := strings.Join(p.owner, "."); {
			case owner != "":
				p.symbols.Add(owner+"."+n.text, "method", n.line)
			default:
				p.symbols.Add(n.text, "func", n.line)
			}
		}
	} else {
		// An operator: its name is the punctuation up to a space or the
		// parameters (`static func < (`, `func && <T: P>(`).
		p.i++
		for !p.atEnd() && p.token().kind == tPunctuation && p.token().start == p.peek(-1).end && !p.is("(") && !p.is("{") && !p.is("}") {
			p.i++
		}
	}
	p.genericParameters()
	p.signature()
	if keyword == "macro" && p.is("=") {
		p.i++
		p.expression(false, false, false)
	}
	p.bodyBlock()
}

func (p *parser) initDeclaration() {
	t := p.token()
	p.i++
	if p.is("?") || p.is("!") {
		p.i++
	}
	if p.visible() && len(p.owner) > 0 {
		p.symbols.Add(strings.Join(p.owner, ".")+".init", "method", t.line)
	}
	p.genericParameters()
	p.signature()
	p.bodyBlock()
}

// signature reads parameters, effects, a result type and a where clause.
func (p *parser) signature() {
	if p.is("(") {
		p.parameters(false)
	}
	p.effects()
	if p.is("->") {
		p.i++
		p.parseType()
	}
	p.where()
}

// effects reads async, throws, throws(E), rethrows.
func (p *parser) effects() {
	for {
		t := p.token()
		switch {
		case word(t, "async"), word(t, "reasync"), word(t, "rethrows"):
			p.i++
		case word(t, "throws"):
			p.i++
			if p.is("(") && !p.token().newline {
				p.i++
				p.parseType()
				if p.is(")") {
					p.i++
				}
			}
		default:
			return
		}
	}
}

// parameters reads a parameter clause: (label name: Type = default, ...). In a closure
// or an enum case a parameter may have no label: a closure's is a name, an enum
// case's a type.
func (p *parser) parameters(unlabeledType bool) {
	p.i++ // (
	for !p.atEnd() {
		if p.is(")") {
			p.i++
			return
		}
		if p.is("}") || p.is("{") {
			return
		}
		start := p.i
		for p.is("@") {
			p.attribute(true)
		}
		switch {
		case p.labels() || unlabeledType:
			p.parseType()
			if p.is("...") {
				p.i++
			}
			if p.is("=") {
				p.i++
				p.expression(true, false, false)
			}
		default:
			p.expression(true, false, false)
		}
		if p.is(",") {
			p.i++
		} else if p.i == start {
			p.i++
		}
	}
}

// labels skips the argument label and parameter name before a parameter's or tuple
// element's type (`label name:`, `_ name:`, `name:`), reporting whether there were.
func (p *parser) labels() bool {
	for k := 1; k <= 2; k++ {
		if p.peek(0).kind == tIdentifier && (k == 1 || p.peek(1).kind == tIdentifier) && punctuation(p.peek(k), ":") {
			p.i += k + 1
			return true
		}
	}
	return false
}

// genericParameters reads <T: Constraint, each U>: the parameters are names, their
// constraints types.
func (p *parser) genericParameters() {
	if !p.is("<") {
		return
	}
	p.i++
	for !p.atEnd() && !p.is(">") {
		start := p.i
		for p.is("@") {
			p.attribute(false)
		}
		if word(p.token(), "each") || word(p.token(), "let") {
			p.i++
		}
		if p.token().kind == tIdentifier {
			p.i++
		}
		if p.is(":") {
			p.i++
			p.parseType()
		}
		if p.is(",") {
			p.i++
		} else if p.i == start || !p.is(">") {
			if p.is("{") || p.is("(") || p.is("}") {
				return
			}
			p.i++
		}
	}
	if p.is(">") {
		p.i++
	}
}

// inheritance reads `: A, B` after a declared name.
func (p *parser) inheritance() {
	if !p.is(":") {
		return
	}
	p.i++
	for {
		p.parseType()
		if !p.is(",") {
			return
		}
		p.i++
	}
}

// where reads a where clause: `where T: P, T.Element == U`. The constrained side is
// a name path, not a type use.
func (p *parser) where() {
	if !word(p.token(), "where") {
		return
	}
	p.i++
	for !p.atEnd() {
		for p.is("@") {
			p.attribute(false)
		}
		start := p.i
		for p.token().kind == tIdentifier && !word(p.token(), "where") || p.is(".") {
			p.i++
		}
		if p.is("<") { // a generic constrained type: Foo<T>.Bar
			p.genericArguments()
			for p.token().kind == tIdentifier || p.is(".") {
				p.i++
			}
		}
		if p.is(":") || p.is("==") || p.is("=") {
			p.i++
			p.parseType()
		} else if p.i == start {
			return
		}
		if !p.is(",") {
			return
		}
		p.i++
	}
}

// property reads a var or let declaration: patterns with types, initializers and
// accessor bodies, separated by commas.
func (p *parser) property(s scope) {
	p.i++
	for !p.atEnd() {
		n := p.token()
		switch {
		case p.is("("):
			p.group(")")
		case n.kind == tIdentifier:
			p.i++
			if p.visible() && s != sProtocol && !n.bt && isIdentifier(n.text) {
				if owner := strings.Join(p.owner, "."); owner != "" {
					p.symbols.Add(owner+"."+n.text, "property", n.line)
				} else {
					p.symbols.Add(n.text, "var", n.line)
				}
			}
		default:
			return
		}
		if p.is(":") {
			p.i++
			p.parseType()
		}
		if p.is("=") {
			p.i++
			p.expression(false, false, true)
		}
		if p.is("{") {
			p.i++
			p.block(sBody)
		}
		if !p.is(",") {
			return
		}
		p.i++
	}
}

// enumCases reads the cases after `case` in an enum: names, associated values (whose
// unlabeled elements are types) and raw values.
func (p *parser) enumCases() {
	for !p.atEnd() {
		if p.token().kind != tIdentifier {
			return
		}
		p.i++
		if p.is("(") {
			p.parameters(true)
		}
		if p.is("=") {
			p.i++
			p.expression(false, false, true)
		}
		if !p.is(",") {
			return
		}
		p.i++
	}
}

// parseType reads a type, noting the types it names.
func (p *parser) parseType() {
	if !p.enter() {
		return
	}
	defer p.leave()
	for !p.atEnd() {
		for {
			t := p.token()
			if punctuation(t, "@") {
				p.attribute(true)
			} else if punctuation(t, "~") && p.peek(1).kind == tIdentifier {
				p.i += 2 // ~Copyable suppresses a protocol; it uses nothing
				return
			} else if n := p.peek(1); t.kind == tIdentifier && !t.bt && typePrefixes[t.text] && !n.newline &&
				(n.kind == tIdentifier || punctuation(n, "(") || punctuation(n, "[") || punctuation(n, "@")) {
				p.i++
			} else {
				break
			}
		}
		t := p.token()
		switch {
		case punctuation(t, "("):
			p.i++
			for !p.atEnd() && !p.is(")") {
				start := p.i
				p.labels()
				p.parseType()
				if p.is("...") {
					p.i++
				}
				if p.is(",") {
					p.i++
				} else if p.i == start || !p.is(")") {
					if p.is("{") || p.is("}") || p.is("]") {
						return
					}
					p.i++
				}
			}
			if p.is(")") {
				p.i++
			}
		case punctuation(t, "["):
			p.i++
			p.parseType()
			if p.is(":") {
				p.i++
				p.parseType()
			}
			if p.is("]") {
				p.i++
			}
		case t.kind == tIdentifier && (t.bt || !keywords[t.text] || t.text == "Self" || t.text == "Any"):
			p.reference(t)
			p.i++
			if p.is("<") && !p.token().newline {
				p.genericArguments()
			}
			for p.is(".") && p.peek(1).kind == tIdentifier && !word(p.peek(1), "Type") && !word(p.peek(1), "Protocol") {
				p.i += 2
				if p.is("<") && !p.token().newline {
					p.genericArguments()
				}
			}
		default:
			return
		}
		for {
			t := p.token()
			switch {
			case (punctuation(t, "?") || punctuation(t, "!")) && !t.newline:
				p.i++
				continue
			case punctuation(t, "...") && !t.newline:
				p.i++
				continue
			case punctuation(t, ".") && (word(p.peek(1), "Type") || word(p.peek(1), "Protocol")):
				p.i += 2
				continue
			}
			break
		}
		p.effects()
		if p.is("->") {
			p.i++
			p.parseType()
			return
		}
		if !p.is("&") {
			return
		}
		p.i++
	}
}

// genericArguments reads <A, B> after a type name.
func (p *parser) genericArguments() {
	p.i++
	for !p.atEnd() && !p.is(">") {
		start := p.i
		p.parseType()
		if p.is(",") {
			p.i++
		} else if p.i == start || !p.is(">") {
			if p.is("{") || p.is("}") || p.is(";") || p.is(")") || p.is("]") || p.is("=") {
				return
			}
			p.i++
		}
	}
	if p.is(">") {
		p.i++
	}
}

// genericCall reports whether the `<` at token k starts generic arguments of a type
// the expression uses: before a member access (Box<Int>.self), an empty call
// (Set<Int>()), or before a call or closure when the arguments cannot be read as
// comparisons: several (Dictionary<K, V>(x)) or nested ones ending in `>>`
// (Box<Set<Int>>(x)). A single argument before arguments or a closure (Box<Int>(x), Stream<T> { }) is read
// as two comparisons, the way the grammar this scanner replaced read it.
func (p *parser) genericCall(k int) bool {
	depth, complex := 0, false
	for j := p.i + k; j < len(p.tokens) && j < p.i+k+64; j++ {
		t := p.tokens[j]
		switch t.kind {
		case tIdentifier:
			continue
		case tPunctuation:
		default:
			return false
		}
		switch t.text {
		case "<":
			depth++
		case ">":
			depth--
			// Box<Set<Int>>(x): the grammar reads `>>` as one operator, which
			// makes a comparison impossible.
			complex = complex || j > 0 && punctuation(p.tokens[j-1], ">") && p.tokens[j-1].end == t.start
			if depth == 0 {
				if j+2 >= len(p.tokens) || p.tokens[j+1].newline {
					return false
				}
				switch next := p.tokens[j+1]; {
				case punctuation(next, "."):
					return true
				case punctuation(next, "("):
					return complex || punctuation(p.tokens[j+2], ")")
				case punctuation(next, "{"):
					return complex
				}
				return false
			}
		case ",":
			complex = complex || depth == 1
		case ".", "?", "!", "[", "]", "(", ")", ":", "@", "...", "->", "&":
		default:
			return false
		}
	}
	return false
}

// group reads a bracketed list of expressions up to the closing bracket, which it
// consumes. The current token is the opening bracket.
func (p *parser) group(closer string) {
	if !p.enter() {
		p.skipGroup()
		return
	}
	defer p.leave()
	p.i++
	for !p.atEnd() {
		switch {
		case p.is(closer):
			p.i++
			return
		case p.is("}"):
			return
		case p.is(")") || p.is("]"):
			p.i++
			return
		case p.is(",") || p.is(";") || p.is(":"):
			p.i++
		default:
			start := p.i
			p.expression(true, false, false)
			if p.i == start {
				p.i++
			}
		}
	}
}

// maxDepth bounds how deep brackets, bodies and types nest before the parser skips
// what is inside without reading it: no real file nests hundreds deep, and each
// level costs a stack frame.
const maxDepth = 200

func (p *parser) enter() bool {
	p.depth++
	if p.depth > maxDepth {
		p.depth--
		return false
	}
	return true
}

func (p *parser) leave() { p.depth-- }

// skipGroup skips a bracketed group, noting nothing.
func (p *parser) skipGroup() {
	depth := 0
	for !p.atEnd() {
		t := p.token()
		p.i++
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case "(", "[", "{", `\(`:
			depth++
		case ")", "]", "}":
			depth--
			if depth <= 0 {
				return
			}
		}
	}
}

// closure reads a closure at `{`: its signature ({ [weak self] (a: A) -> B in),
// whose parameter and result types are type uses, then its body.
func (p *parser) closure() {
	p.i++
	if end := p.closureSignature(); end > 0 {
		if p.is("[") {
			p.group("]")
		}
		for p.is("@") {
			p.attribute(false)
		}
		if p.is("(") {
			p.parameters(false)
		}
		for p.i < end {
			p.effects()
			if p.is("->") {
				p.i++
				p.parseType()
				continue
			}
			p.i++
		}
		p.i = end + 1
	}
	p.block(sBody)
}

// closureSignature returns the index of the `in` ending the signature of the closure
// whose body starts at the current token, or 0 when it has none.
func (p *parser) closureSignature() int {
	j := p.i
	skip := func(open, close string) bool {
		depth := 0
		for ; j < len(p.tokens); j++ {
			t := p.tokens[j]
			if t.kind != tPunctuation {
				continue
			}
			switch t.text {
			case open:
				depth++
			case close:
				depth--
				if depth == 0 {
					j++
					return true
				}
			case "{", "}":
				return false
			}
		}
		return false
	}
	at := func(s string) bool { return j < len(p.tokens) && punctuation(p.tokens[j], s) }
	if at("[") && !skip("[", "]") {
		return 0
	}
	for at("@") && j+1 < len(p.tokens) && p.tokens[j+1].kind == tIdentifier {
		j += 2
		if at("(") && !skip("(", ")") {
			return 0
		}
	}
	switch {
	case at("("):
		if !skip("(", ")") {
			return 0
		}
	case j < len(p.tokens) && p.tokens[j].kind == tIdentifier && (name(p.tokens[j]) || p.tokens[j].text == "_"):
		for j < len(p.tokens) && p.tokens[j].kind == tIdentifier && (name(p.tokens[j]) || p.tokens[j].text == "_") {
			j++
			if !at(",") {
				break
			}
			j++
		}
	}
	for n := 0; j < len(p.tokens) && n < 64; j, n = j+1, n+1 {
		t := p.tokens[j]
		switch {
		case word(t, "in"):
			return j
		case t.kind == tIdentifier && (!keywords[t.text] || t.bt || t.text == "throws" || t.text == "rethrows" || t.text == "async"):
		case t.kind == tPunctuation && strings.Contains(" -> . , ? ! < > ( ) [ ] : & ... @ ", " "+t.text+" "):
		default:
			return 0
		}
		if n == 0 && !(t.kind == tIdentifier || punctuation(t, "->")) {
			return 0
		}
		if n == 0 && t.kind == tIdentifier && !(t.text == "throws" || t.text == "rethrows" || t.text == "async") {
			return 0
		}
	}
	return 0
}

// expression reads an expression, or a run of them, up to the end of the statement: a `;`,
// a closing bracket, or a line that starts a new statement. In a group (inside
// brackets) line breaks mean nothing and a `,` ends it; stopColon ends it at a `:`
// (a case pattern), stopComma at a `,` (a property's initializer). It reports
// whether it read anything.
func (p *parser) expression(inGroup, stopColon, stopComma bool) bool {
	start := p.i
	condition := false // after if/while/guard/for/switch: a `{` is the statement's body
	for !p.atEnd() {
		t := p.token()
		if p.i > start && !inGroup && t.newline && operandEnd(p.peek(-1)) && !continues(t) {
			return true
		}
		if t.kind == tPunctuation {
			switch t.text {
			case ";", "}", ")", "]":
				return p.i > start
			case ",":
				if inGroup || stopComma {
					return p.i > start
				}
				p.i++
			case ":":
				if stopColon {
					return p.i > start
				}
				p.i++
			case "(", `\(`:
				p.group(")")
			case "[":
				if p.arrayType() {
					p.parseType()
				} else {
					p.group("]")
				}
			case "{":
				if condition {
					condition = false
					p.i++
					p.block(sBody)
				} else {
					p.closure()
				}
			case "->":
				p.i++
				p.parseType()
			case "@":
				p.attribute(false)
			case "#":
				p.i++
				if p.token().kind == tIdentifier && p.token().start == t.end {
					p.i++
				}
			case `\`:
				p.i++
				if p.token().kind == tIdentifier && p.token().start == t.end {
					p.i++
					if p.is("<") {
						p.skipGenerics()
					}
				}
			case ".":
				// .Foo.bar is an implicit member whose base is a type; x.Foo a member.
				implicit := p.i == start || !p.operandBefore()
				p.i++
				if p.token().kind == tIdentifier && !p.token().newline {
					if implicit && punctuation(p.peek(1), ".") {
						p.reference(p.token())
					}
					p.i++
				}
			default:
				p.i++
			}
			continue
		}
		if t.kind != tIdentifier {
			p.i++
			continue
		}
		if !t.bt && keywords[t.text] || word(t, "actor") || word(t, "macro") {
			if punctuation(p.peek(1), ":") && !inGroup && !stopColon || inGroup && punctuation(p.peek(1), ":") {
				p.i += 2 // a label
				continue
			}
			switch t.text {
			case "as", "is":
				p.i++
				if (p.is("?") || p.is("!")) && !p.token().newline {
					p.i++
				}
				p.parseType()
				continue
			case "if", "guard", "while", "switch", "catch":
				condition = true
			case "for":
				condition = true
				p.i++
				if p.token().kind == tIdentifier && punctuation(p.peek(1), ":") { // for x: T in
					p.i += 2
					p.parseType()
				}
				continue
			case "throws": // do throws(E) {
				p.i++
				if p.is("(") && !p.token().newline {
					p.i++
					p.parseType()
				}
				continue
			case "let", "var":
				p.i++
				if p.token().kind == tIdentifier && punctuation(p.peek(1), ":") {
					p.i += 2
					p.parseType()
				}
				continue
			case "func", "class", "struct", "enum", "protocol", "extension", "import",
				"typealias", "associatedtype", "init", "deinit", "subscript", "case",
				"default", "static", "private", "public", "fileprivate", "internal", "operator",
				"precedencegroup", "actor", "macro":
				if t.text == "case" && (condition || inGroup) {
					break // if case .a = x, for case let
				}
				if (t.text == "actor" || t.text == "macro") && !(p.i == start || t.newline) {
					break
				}
				if t.text == "init" && punctuation(p.peek(-1), ".") {
					break
				}
				if p.i > start {
					return true
				}
				if t.text == "actor" || t.text == "macro" {
					break
				}
			}
			p.i++
			continue
		}
		if (word(t, "any") || word(t, "some")) && name(p.peek(1)) && !p.peek(1).newline {
			p.parseType() // (any Error).self
			continue
		}
		p.identifier(condition)
	}
	return p.i > start
}

// arrayType reports whether the `[` at the current token starts an array or
// dictionary type whose member is accessed ([UInt8].random(), [String: Any].self)
// or, when it has generic arguments or a function type, that is called
// ([Box<Int>](), [(Int) -> Void]()): [Int]() read as an array literal called.
func (p *parser) arrayType() bool {
	depth, nested, generic := 0, 0, false
	for j := p.i; j < len(p.tokens) && j < p.i+64; j++ {
		t := p.tokens[j]
		switch {
		case t.kind == tIdentifier && (t.bt || !keywords[t.text] || t.text == "async" || t.text == "throws"):
		case t.text == "->":
			generic = true // a function type cannot be a literal
		case t.kind != tPunctuation:
			return false
		case t.text == "[":
			depth++
		case t.text == "]":
			depth--
			if depth == 0 {
				if j == p.i+1 || j+1 >= len(p.tokens) || p.tokens[j+1].start != t.end {
					return false
				}
				return punctuation(p.tokens[j+1], ".") || generic && punctuation(p.tokens[j+1], "(")
			}
		case t.text == "(" || t.text == "<":
			nested++
			generic = generic || t.text == "<"
		case t.text == ")" || t.text == ">":
			nested--
		case t.text == ",":
			if nested <= 0 {
				return false
			}
		case t.text == "." && j > p.i+1, t.text == ":", t.text == "?", t.text == "!":
		default:
			return false
		}
	}
	return false
}

// operandBefore reports whether the token before the current one ends an operand,
// so that what follows is a member access or a binary operator. A `?` or `!` only
// does when it is a postfix: `a? .b` is not, `c ? .b : .d` is a conditional.
func (p *parser) operandBefore() bool {
	if p.i == 0 {
		return false
	}
	previous := p.peek(-1)
	if punctuation(previous, "?") || punctuation(previous, "!") {
		return p.i > 1 && p.peek(-2).end == previous.start
	}
	return operandEnd(previous)
}

// continues reports whether a token at the start of a line continues the expression
// on the line before: a member access, a binary operator, else, catch.
func continues(t token) bool {
	switch t.kind {
	case tPunctuation:
		switch t.text {
		case ".", "->", "?", "&", "<", ">", "...", "..<", "=":
			return true
		case "(", "[", "{", "}", ")", "]", ",", ";", ":", "@", "#", `\`, `\(`, "!":
			return t.text == "{" // a trailing closure
		}
		return true // an operator
	case tIdentifier:
		return !t.bt && (t.text == "else" || t.text == "catch" || t.text == "where" || t.text == "as" || t.text == "is" || t.text == "in")
	}
	return false
}

// identifier reads an identifier in an expression. A capitalized one is a type the
// code uses when a member access, call, subscript, trailing closure or generic
// arguments follow it: Foo.shared, Foo(), Foo<Int>(). Not in a condition, where `{`
// is the statement's body: `if x == Foo {`.
func (p *parser) identifier(condition bool) {
	t := p.token()
	n := p.peek(1)
	// After an additive or multiplicative operator the grammar applied a call to
	// the whole operation (`x + Foo(y)` as `(x + Foo)(y)`), and after a prefix
	// operator to the operand (`!Foo()`, `try! Foo()`): no type use.
	previous := p.peek(-1)
	arith := p.i > 0 && !t.newline && previous.kind == tPunctuation && strings.Contains(" + - * / % ! ~ ", " "+previous.text+" ")
	switch {
	case punctuation(n, "."):
		p.reference(t)
	case (punctuation(n, "(") || punctuation(n, "[")) && !n.newline && !arith:
		p.reference(t)
	case punctuation(n, "{") && !condition && !arith:
		p.reference(t)
	case punctuation(n, "<") && !n.newline && p.genericCall(1):
		p.reference(t)
		p.i++
		p.genericArguments()
		return
	}
	p.i++
}

// skipGenerics skips balanced <...>.
func (p *parser) skipGenerics() {
	depth := 0
	for !p.atEnd() {
		t := p.token()
		if t.kind == tPunctuation {
			switch t.text {
			case "<":
				depth++
			case ">":
				depth--
			case "{", "}", ";", "(", ")":
				return
			}
		}
		p.i++
		if depth == 0 {
			return
		}
	}
}
