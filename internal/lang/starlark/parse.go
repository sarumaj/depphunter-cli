package starlark

import "strings"

// Node kinds.
const (
	Identifier = 'i' // a name; Text is the name
	Attribute  = 'a' // X.Text; Name() gives the dotted path when X is a name or path
	String     = 's' // a string literal (adjacent literals joined); Text is its value
	Number     = 'n'
	List       = 'l' // Items
	Tuple      = 't' // Items
	Dictionary = 'd' // Items: key, value, key, value...
	Call       = 'c' // Fn(Args)
	Binary     = 'b' // Items[0] Text Items[1]; Text is the operator
	Condition  = 'k' // Items: then, condition, else
	Other      = '?' // anything else: comprehensions, lambdas, subscripts, unary operators
)

// Node is an expression.
type Node struct {
	Kind      byte
	Text      string
	Line      int
	X         *Node // Attribute: the object; Call: the function
	Arguments []Argument
	Items     []*Node
}

// Argument is one argument of a call: positional (Name "") or keyword. Star is "*" or
// "**" for an unpacked argument.
type Argument struct {
	Name  string
	Star  string
	Value *Node
}

// Name is the dotted name an identifier or attribute path spells ("native.glob"),
// else "".
func (n *Node) Name() string {
	switch {
	case n == nil:
		return ""
	case n.Kind == Identifier:
		return n.Text
	case n.Kind == Attribute:
		if x := n.X.Name(); x != "" {
			return x + "." + n.Text
		}
	}
	return ""
}

// Callee is the dotted name of the function a call calls, else "".
func (n *Node) Callee() string {
	if n == nil || n.Kind != Call {
		return ""
	}
	return n.X.Name()
}

// Keyword is a call's keyword argument, or nil.
func (n *Node) Keyword(name string) *Node {
	if n == nil {
		return nil
	}
	for _, a := range n.Arguments {
		if a.Name == name && a.Star == "" {
			return a.Value
		}
	}
	return nil
}

// Position is a call's i-th positional argument, or nil.
func (n *Node) Position(i int) *Node {
	if n == nil {
		return nil
	}
	for _, a := range n.Arguments {
		if a.Name == "" && a.Star == "" {
			if i == 0 {
				return a.Value
			}
			i--
		}
	}
	return nil
}

// StringValue is a string literal's value; ok is false for anything else.
func (n *Node) StringValue() (string, bool) {
	if n == nil || n.Kind != String {
		return "", false
	}
	return n.Text, true
}

// KeywordString is a keyword argument's string value, or "".
func (n *Node) KeywordString(name string) string {
	s, _ := n.Keyword(name).StringValue()
	return s
}

// Strings lists the string literals of a list, tuple or single string; other
// elements are skipped.
func (n *Node) Strings() []string {
	if n == nil {
		return nil
	}
	if s, ok := n.StringValue(); ok {
		return []string{s}
	}
	var out []string
	if n.Kind == List || n.Kind == Tuple {
		for _, it := range n.Items {
			if s, ok := it.StringValue(); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// Statement is a statement: an expression (usually a call), an assignment, a function
// definition, or a compound statement's header (if, for, ...), whose body follows
// as statements of their own.
type Statement struct {
	Kind    byte // 'e' expression, 'a' assignment, 'd' def, 'o' other
	Line    int
	Column  int
	Targets []string // assignment: the names assigned (x, and a, b in a, b = ...)
	X       *Node    // expression, or an assignment's value
	Name    string   // def: the function's name
	// Definition is the name of the function the statement is in ("" at the top level);
	// nested functions are named by their outermost one.
	Definition string
}

// File is what a Starlark file holds, statement by statement.
type File struct {
	Statements []Statement
}

// maxDepth bounds how deeply expressions nest before the reader skips a group
// unread: 1 MB of "[" must not recurse a million frames.
const maxDepth = 200

// Parse reads a Starlark file. It never fails.
//
// Implements: REQ-BAZEL-012
func Parse(source []byte) *File {
	p := &parser{tokens: lex(source)}
	p.match()
	f := &File{}
	type frame struct {
		column int
		name   string
	}
	var definitions []frame
	for p.i < len(p.tokens) {
		start := p.i
		t := p.tokens[p.i]
		for len(definitions) > 0 && t.first && t.column <= definitions[len(definitions)-1].column {
			definitions = definitions[:len(definitions)-1]
		}
		statement := p.statement()
		if len(definitions) > 0 {
			statement.Definition = definitions[0].name
		}
		if statement.Kind == 'd' && statement.Name != "" && len(definitions) < 64 {
			definitions = append(definitions, frame{column: t.column, name: statement.Name})
		}
		if statement.Kind != 0 {
			f.Statements = append(f.Statements, statement)
		}
		// Whatever the statement left unread up to the next line is skipped: that
		// is the reader's recovery from anything it did not understand.
		if p.i == start {
			p.i++
		}
		for p.i < len(p.tokens) && !p.tokens[p.i].first {
			if p.is(";") {
				p.i++
				break
			}
			p.skip()
		}
	}
	return f
}

type parser struct {
	tokens []token
	i      int
	close  []int // index of each opening bracket's closer, -1 when unclosed
	depth  int
}

// match pairs every bracket once, so a group can be skipped in one step.
func (p *parser) match() {
	p.close = make([]int, len(p.tokens))
	var stack []int
	for i, t := range p.tokens {
		p.close[i] = -1
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			open := map[string]string{")": "(", "]": "[", "}": "{"}[t.text]
			// A closer only closes an opener near the top; a stray one is ignored.
			for k := len(stack) - 1; k >= 0 && k >= len(stack)-8; k-- {
				if p.tokens[stack[k]].text == open {
					p.close[stack[k]] = i
					stack = stack[:k]
					break
				}
			}
		}
	}
}

func (p *parser) is(text string) bool {
	return p.i < len(p.tokens) && p.tokens[p.i].kind == tPunctuation && p.tokens[p.i].text == text
}

func (p *parser) isWord(w string) bool {
	return p.i < len(p.tokens) && p.tokens[p.i].kind == tIdentifier && p.tokens[p.i].text == w
}

// skip steps over one token, or a whole bracketed group.
func (p *parser) skip() {
	if p.i < len(p.tokens) && p.tokens[p.i].kind == tPunctuation {
		switch p.tokens[p.i].text {
		case "(", "[", "{":
			if c := p.close[p.i]; c > p.i {
				p.i = c + 1
				return
			}
			p.i = len(p.tokens) // unclosed: runs to the end
			return
		}
	}
	p.i++
}

var compound = map[string]bool{"if": true, "elif": true, "else": true, "for": true, "while": true, "with": true, "try": true, "except": true, "finally": true}

var simple = map[string]bool{"return": true, "pass": true, "break": true, "continue": true, "raise": true, "assert": true, "del": true, "global": true, "nonlocal": true, "yield": true}

func (p *parser) statement() Statement {
	t := p.tokens[p.i]
	statement := Statement{Line: t.line, Column: t.column}
	if t.kind == tIdentifier {
		switch {
		case t.text == "def":
			p.i++
			statement.Kind = 'd'
			if p.i < len(p.tokens) && p.tokens[p.i].kind == tIdentifier {
				statement.Name = p.tokens[p.i].text
				p.i++
			}
			p.header()
			return statement
		case compound[t.text]:
			p.i++
			statement.Kind = 'o'
			p.header()
			return statement
		case simple[t.text]:
			p.i++
			statement.Kind = 'o'
			if t.text == "return" && p.i < len(p.tokens) && !p.tokens[p.i].first {
				statement.X = p.expressionList()
			}
			return statement
		}
	}
	x := p.expressionList()
	if p.i < len(p.tokens) && p.tokens[p.i].kind == tPunctuation && !p.tokens[p.i].first {
		switch operator := p.tokens[p.i].text; {
		case operator == "=" || len(operator) >= 2 && strings.HasSuffix(operator, "=") && operator != "==" && operator != "!=" && operator != "<=" && operator != ">=":
			p.i++
			statement.Kind, statement.Targets = 'a', targets(x)
			// a = b = c: the value is the last one.
			for {
				statement.X = p.expressionList()
				if !p.is("=") || p.tokens[p.i].first {
					break
				}
				p.i++
				statement.Targets = append(statement.Targets, targets(statement.X)...)
			}
			return statement
		}
	}
	statement.Kind, statement.X = 'e', x
	return statement
}

// header skips a compound statement's or def's header up to its colon. A body on
// the same line (def f(): return 1) follows as a statement of its own.
func (p *parser) header() {
	for p.i < len(p.tokens) && !p.tokens[p.i].first {
		if p.is(":") {
			p.i++
			if p.i < len(p.tokens) {
				p.tokens[p.i].first = true
			}
			return
		}
		p.skip()
	}
}

func targets(x *Node) []string {
	switch {
	case x == nil:
	case x.Kind == Identifier:
		return []string{x.Text}
	case x.Kind == Tuple || x.Kind == List:
		var out []string
		for _, it := range x.Items {
			out = append(out, targets(it)...)
		}
		return out
	}
	return nil
}

// expressionList reads an expression, or a tuple of them written without parentheses.
func (p *parser) expressionList() *Node {
	x := p.expression()
	if !p.is(",") || p.tokens[p.i].first {
		return x
	}
	tuple := &Node{Kind: Tuple, Line: x.Line, Items: []*Node{x}}
	for p.is(",") && !p.tokens[p.i].first {
		p.i++
		if p.i >= len(p.tokens) || p.tokens[p.i].first || p.is("=") || p.is(")") || p.is("]") || p.is("}") {
			break
		}
		tuple.Items = append(tuple.Items, p.expression())
	}
	return tuple
}

func (p *parser) expression() *Node {
	if p.i >= len(p.tokens) {
		return &Node{Kind: Other}
	}
	line := p.tokens[p.i].line
	if p.depth >= maxDepth {
		p.skip()
		return &Node{Kind: Other, Line: line}
	}
	p.depth++
	defer func() { p.depth-- }()
	if p.isWord("lambda") {
		// lambda arguments: body - read the body, keep nothing.
		for p.i < len(p.tokens) && !p.tokens[p.i].first && !p.is(",") && !p.is(")") && !p.is("]") && !p.is("}") {
			if p.is(":") {
				p.i++
				p.expression()
				break
			}
			p.skip()
		}
		return &Node{Kind: Other, Line: line}
	}
	x := p.binary(0)
	if p.isWord("if") && !p.tokens[p.i].first {
		p.i++
		condition := p.binary(0)
		var els *Node
		if p.isWord("else") && !p.tokens[p.i].first {
			p.i++
			els = p.expression()
		} else {
			els = &Node{Kind: Other, Line: line}
		}
		return &Node{Kind: Condition, Line: line, Items: []*Node{x, condition, els}}
	}
	return x
}

// Binary operators by precedence, lowest first.
var levels = [][]string{
	{"or"}, {"and"}, {"in", "not in", "is", "is not", "==", "!=", "<", ">", "<=", ">="},
	{"|"}, {"^"}, {"&"}, {"<<", ">>"}, {"+", "-"}, {"*", "/", "//", "%"},
}

func (p *parser) binaryOp(level int) (string, int) {
	if p.i >= len(p.tokens) || p.tokens[p.i].first {
		return "", 0
	}
	t := p.tokens[p.i]
	for _, operator := range levels[level] {
		switch {
		case operator == "not in":
			if t.kind == tIdentifier && t.text == "not" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "in" {
				return operator, 2
			}
		case operator == "is not":
			if t.kind == tIdentifier && t.text == "is" && p.i+1 < len(p.tokens) && p.tokens[p.i+1].text == "not" {
				return operator, 2
			}
		case t.text == operator && (t.kind == tPunctuation || t.kind == tIdentifier && (operator == "or" || operator == "and" || operator == "in" || operator == "is")):
			return operator, 1
		}
	}
	return "", 0
}

func (p *parser) binary(level int) *Node {
	if level >= len(levels) {
		return p.unary()
	}
	x := p.binary(level + 1)
	for n := 0; n < 1<<20; n++ {
		operator, width := p.binaryOp(level)
		if width == 0 {
			break
		}
		p.i += width
		y := p.binary(level + 1)
		x = &Node{Kind: Binary, Text: operator, Line: x.Line, Items: []*Node{x, y}}
	}
	return x
}

func (p *parser) unary() *Node {
	var line int
	prefixed := false
	for p.i < len(p.tokens) && !p.tokens[p.i].first {
		t := p.tokens[p.i]
		if !(t.kind == tPunctuation && (t.text == "-" || t.text == "+" || t.text == "~") || t.kind == tIdentifier && t.text == "not") {
			break
		}
		if !prefixed {
			line, prefixed = t.line, true
		}
		p.i++
	}
	x := p.postfix()
	if prefixed {
		return &Node{Kind: Other, Line: line}
	}
	return x
}

func (p *parser) postfix() *Node {
	x := p.primary()
	for p.i < len(p.tokens) && !p.tokens[p.i].first && p.tokens[p.i].kind == tPunctuation {
		switch p.tokens[p.i].text {
		case ".":
			p.i++
			if p.i < len(p.tokens) && p.tokens[p.i].kind == tIdentifier {
				x = &Node{Kind: Attribute, Text: p.tokens[p.i].text, Line: x.Line, X: x}
				p.i++
			}
		case "(":
			x = &Node{Kind: Call, Line: x.Line, X: x, Arguments: p.arguments()}
		case "[":
			p.skip() // a subscript or slice: its value is not known
			x = &Node{Kind: Other, Line: x.Line}
		default:
			return x
		}
	}
	return x
}

// group runs read for the inside of the bracketed group opening at p.i, then
// leaves p.i after its closer. Past maxDepth the group is skipped unread.
func (p *parser) group(read func(end int)) bool {
	open := p.i
	end := p.close[open]
	if end < 0 {
		end = len(p.tokens)
	}
	if p.depth >= maxDepth {
		p.skip()
		return false
	}
	p.depth++
	p.i++
	// The group's line breaks do not end anything: first flags inside a group
	// come only from a def's inline body, which cannot occur here.
	read(end)
	p.depth--
	p.i = min(end+1, len(p.tokens))
	return true
}

// items reads comma-separated expressions up to end, stepping over anything it
// cannot read. A comprehension ("for" after the first item) makes the result nil.
func (p *parser) items(end int, dictionary bool) (items []*Node, comprehension bool) {
	for p.i < end {
		if p.is(",") {
			p.i++
			continue
		}
		if p.isWord("for") {
			return nil, true
		}
		start := p.i
		if p.is("*") || p.is("**") {
			p.i++
		}
		x := p.expression()
		if dictionary {
			var v *Node
			if p.is(":") {
				p.i++
				v = p.expression()
			} else {
				v = &Node{Kind: Other, Line: x.Line}
			}
			items = append(items, x, v)
		} else {
			items = append(items, x)
		}
		// Recover: anything up to the next comma is not understood.
		for p.i < end && !p.is(",") {
			if p.isWord("for") {
				return nil, true
			}
			p.skip()
		}
		if p.i == start {
			p.i++
		}
	}
	return items, false
}

func (p *parser) arguments() []Argument {
	var out []Argument
	p.group(func(end int) {
		for p.i < end {
			if p.is(",") {
				p.i++
				continue
			}
			if p.isWord("for") {
				p.i = end
				return
			}
			start := p.i
			a := Argument{}
			if p.is("*") || p.is("**") {
				a.Star = p.tokens[p.i].text
				p.i++
			} else if p.i+1 < end && p.tokens[p.i].kind == tIdentifier && p.tokens[p.i+1].kind == tPunctuation && p.tokens[p.i+1].text == "=" {
				a.Name = p.tokens[p.i].text
				p.i += 2
			}
			a.Value = p.expression()
			out = append(out, a)
			for p.i < end && !p.is(",") {
				p.skip()
			}
			if p.i == start {
				p.i++
			}
		}
	})
	return out
}

func (p *parser) primary() *Node {
	if p.i >= len(p.tokens) {
		return &Node{Kind: Other}
	}
	t := p.tokens[p.i]
	switch t.kind {
	case tIdentifier:
		p.i++
		return &Node{Kind: Identifier, Text: t.text, Line: t.line}
	case tNumber:
		p.i++
		return &Node{Kind: Number, Text: t.text, Line: t.line}
	case tString:
		p.i++
		v := t.text
		for p.i < len(p.tokens) && p.tokens[p.i].kind == tString && !p.tokens[p.i].first {
			v += p.tokens[p.i].text // adjacent literals concatenate
			p.i++
		}
		return &Node{Kind: String, Text: v, Line: t.line}
	case tPunctuation:
		switch t.text {
		case "[":
			n := &Node{Kind: List, Line: t.line}
			comprehension := false
			p.group(func(end int) { n.Items, comprehension = p.items(end, false) })
			if comprehension {
				return &Node{Kind: Other, Line: t.line}
			}
			return n
		case "{":
			n := &Node{Kind: Dictionary, Line: t.line}
			comprehension := false
			p.group(func(end int) { n.Items, comprehension = p.items(end, true) })
			if comprehension {
				return &Node{Kind: Other, Line: t.line}
			}
			return n
		case "(":
			var items []*Node
			comprehension, trailing := false, false
			p.group(func(end int) {
				items, comprehension = p.items(end, false)
				trailing = end > 0 && end <= len(p.tokens) && end-1 >= 0 && p.tokens[end-1].text == ","
			})
			switch {
			case comprehension:
				return &Node{Kind: Other, Line: t.line}
			case len(items) == 1 && !trailing:
				return items[0]
			}
			return &Node{Kind: Tuple, Line: t.line, Items: items}
		}
	}
	p.i++
	return &Node{Kind: Other, Line: t.line}
}
