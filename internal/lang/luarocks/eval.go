package luarocks

// ValueKind is what a Value holds.
type ValueKind uint8

const (
	NilValue    ValueKind = iota
	StringValue           // a string, or a number kept as written
	BoolValue
	TableValue
)

// Value is what a constant Lua expression evaluates to. Anything the evaluator does
// not compute (a call, arithmetic, a function) is Nil.
type Value struct {
	Kind  ValueKind
	Text  string
	Table *Table
	Line  int // where the expression starts
}

// Field is a keyed entry of a table constructor.
type Field struct {
	Key   string
	Value Value
}

// Table is a table constructor: its positional entries and its keyed ones, in the
// order written.
type Table struct {
	List   []Value
	Fields []Field
}

// Get returns the value of a key (the last one written wins, as in Lua).
func (t *Table) Get(key string) Value {
	if t == nil {
		return Value{}
	}
	for i := len(t.Fields) - 1; i >= 0; i-- {
		if t.Fields[i].Key == key {
			return t.Fields[i].Value
		}
	}
	return Value{}
}

// Path follows keys through nested tables: Path("build", "modules").
func (v Value) Path(keys ...string) Value {
	for _, k := range keys {
		if v.Kind != TableValue {
			return Value{}
		}
		v = v.Table.Get(k)
	}
	return v
}

// Strings is a list of strings: the positional string entries of a table.
func (v Value) Strings() []Value {
	if v.Kind != TableValue {
		return nil
	}
	var out []Value
	for _, e := range v.Table.List {
		if e.Kind == StringValue {
			out = append(out, e)
		}
	}
	return out
}

// Chunk is what a file of constant assignments (a rockspec, a LuaRocks config, a
// lock) comes to: the globals it sets and what it returns.
type Chunk struct {
	Globals map[string]Value
	Return  Value
}

// Eval runs the constant part of a Lua chunk: assignments of strings, numbers,
// booleans, table constructors and concatenations of them to global and local
// variables and to fields of tables, and a top-level return. Everything else is
// skipped a token at a time, so statements it does not understand (an if around an
// assignment, a function call) cost nothing but their own values. It never fails.
//
// Implements: REQ-LUA-006
func Eval(source []byte) *Chunk {
	e := &evaluator{tokens: Lex(source), environment: map[string]Value{}}
	c := &Chunk{Globals: map[string]Value{}}
	for e.i < len(e.tokens) {
		start := e.i
		e.statement(c)
		if e.i == start {
			e.i++
		}
	}
	return c
}

type evaluator struct {
	tokens      []Token
	i           int
	environment map[string]Value
	depth       int
}

// maxDepth bounds nested expressions and tables; deeper ones are skipped.
const maxDepth = 200

func (e *evaluator) at(k int) Token {
	if e.i+k < len(e.tokens) && e.i+k >= 0 {
		return e.tokens[e.i+k]
	}
	return Token{Kind: EOF}
}

func (e *evaluator) statement(c *Chunk) {
	t := e.at(0)
	switch {
	case t.Is("local") && e.at(1).Is("function"):
		e.i++
	case t.Is("local"):
		e.i++
		var names []string
		for e.at(0).Kind == Name {
			names = append(names, e.at(0).Text)
			e.i++
			if e.at(0).Is("<") { // an attribute: <const>, <close>
				e.i += 3
			}
			if !e.at(0).Is(",") {
				break
			}
			e.i++
		}
		if !e.at(0).Is("=") {
			return
		}
		e.i++
		for k, v := range e.expressionList(len(names)) {
			e.environment[names[k]] = v
		}
	case t.Is("return"):
		e.i++
		if values := e.expressionList(1); len(values) > 0 {
			c.Return = values[0]
		}
	case t.Kind == Name && !keyword[t.Text]:
		// name {.name | [expr]} {, target} = exprlist
		type target struct {
			name string
			keys []string
		}
		var targets []target
		for {
			if e.at(0).Kind != Name || keyword[e.at(0).Text] {
				return
			}
			newTarget := target{name: e.at(0).Text}
			e.i++
			for {
				if e.at(0).Is(".") && e.at(1).Kind == Name {
					newTarget.keys = append(newTarget.keys, e.at(1).Text)
					e.i += 2
				} else if e.at(0).Is("[") {
					e.i++
					k := e.expression()
					if !e.at(0).Is("]") {
						return
					}
					e.i++
					newTarget.keys = append(newTarget.keys, k.Text)
				} else {
					break
				}
			}
			targets = append(targets, newTarget)
			if !e.at(0).Is(",") {
				break
			}
			e.i++
		}
		if !e.at(0).Is("=") {
			return
		}
		e.i++
		for k, v := range e.expressionList(len(targets)) {
			target := targets[k]
			if len(target.keys) == 0 {
				e.environment[target.name] = v
				c.Globals[target.name] = v
				continue
			}
			tab := e.environment[target.name]
			for _, key := range target.keys[:len(target.keys)-1] {
				tab = tab.Path(key)
			}
			if tab.Kind == TableValue {
				tab.Table.Fields = append(tab.Table.Fields, Field{Key: target.keys[len(target.keys)-1], Value: v})
			}
		}
	}
}

var keyword = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true,
	"false": true, "for": true, "function": true, "goto": true, "if": true, "in": true,
	"local": true, "nil": true, "not": true, "or": true, "repeat": true, "return": true,
	"then": true, "true": true, "until": true, "while": true,
}

// expressionList evaluates comma-separated expressions; at most n are kept.
func (e *evaluator) expressionList(n int) []Value {
	var out []Value
	for {
		v := e.expression()
		if len(out) < n {
			out = append(out, v)
		}
		if !e.at(0).Is(",") {
			return out
		}
		e.i++
	}
}

// binary are the binary operators; only .. is computed.
var binary = map[string]bool{
	"..": true, "+": true, "-": true, "*": true, "/": true, "//": true, "%": true, "^": true,
	"==": true, "~=": true, "<": true, "<=": true, ">": true, ">=": true, "and": true, "or": true,
	"&": true, "|": true, "~": true, "<<": true, ">>": true,
}

func (e *evaluator) expression() Value {
	if e.depth >= maxDepth {
		e.i++ // deeper than any real file: skipped a token at a time
		return Value{}
	}
	e.depth++
	defer func() { e.depth-- }()
	v := e.unary()
	for {
		operator := e.at(0)
		if !(operator.Kind == Punctuation || operator.Kind == Name) || !binary[operator.Text] {
			return v
		}
		e.i++
		w := e.unary()
		if operator.Text == ".." && v.Kind == StringValue && w.Kind == StringValue {
			v = Value{Kind: StringValue, Text: v.Text + w.Text, Line: v.Line}
		} else if operator.Text == "or" && v.Kind == NilValue {
			v = w
		} else if operator.Text != "or" {
			v = Value{Line: v.Line}
		}
	}
}

func (e *evaluator) unary() Value {
	sawOperator := false
	for e.at(0).Is("-") || e.at(0).Is("not") || e.at(0).Is("#") || e.at(0).Is("~") {
		e.i++
		sawOperator = true
	}
	if v := e.primary(); !sawOperator {
		return v
	}
	return Value{}
}

func (e *evaluator) primary() Value {
	t := e.at(0)
	var v Value
	switch {
	case t.Kind == String, t.Kind == Number:
		e.i++
		v = Value{Kind: StringValue, Text: t.Text, Line: t.Line}
	case t.Is("true"), t.Is("false"):
		e.i++
		v = Value{Kind: BoolValue, Text: t.Text, Line: t.Line}
	case t.Is("nil"), t.Is("..."), t.Kind == Interpolation:
		e.i++
	case t.Is("{"):
		v = e.table()
	case t.Is("function"):
		e.skipFunction()
		return Value{}
	case t.Is("("):
		e.i++
		v = e.expression()
		if e.at(0).Is(")") {
			e.i++
		}
	case t.Kind == Name && !keyword[t.Text]:
		e.i++
		v = e.environment[t.Text]
		v.Line = t.Line
	default:
		return Value{}
	}
	// Suffixes: fields are followed, calls give nil.
	for {
		switch s := e.at(0); {
		case s.Is(".") && e.at(1).Kind == Name:
			v = v.Path(e.at(1).Text)
			e.i += 2
		case s.Is("["):
			e.i++
			k := e.expression()
			if e.at(0).Is("]") {
				e.i++
			}
			v = v.Path(k.Text)
		case s.Is(":") && e.at(1).Kind == Name:
			e.i += 2
		case s.Is("("):
			e.skipGroup("(", ")")
			v = Value{}
		case s.Kind == String:
			e.i++
			v = Value{}
		case s.Is("{"):
			e.table()
			v = Value{}
		default:
			return v
		}
	}
}

// table evaluates a table constructor at e.i.
func (e *evaluator) table() Value {
	line := e.at(0).Line
	if e.depth >= maxDepth {
		e.skipGroup("{", "}")
		return Value{}
	}
	e.depth++
	defer func() { e.depth-- }()
	e.i++
	t := &Table{}
	for e.i < len(e.tokens) && !e.at(0).Is("}") {
		start := e.i
		switch {
		case e.at(0).Is("["):
			e.i++
			k := e.expression()
			if e.at(0).Is("]") && e.at(1).Is("=") {
				e.i += 2
				v := e.expression()
				if k.Kind == StringValue {
					t.Fields = append(t.Fields, Field{Key: k.Text, Value: v})
				}
			}
		case e.at(0).Kind == Name && e.at(1).Is("="):
			key := e.at(0).Text
			e.i += 2
			t.Fields = append(t.Fields, Field{Key: key, Value: e.expression()})
		default:
			t.List = append(t.List, e.expression())
		}
		// Anything up to the next separator is what the evaluator did not follow.
		for e.i < len(e.tokens) && !e.at(0).Is(",") && !e.at(0).Is(";") && !e.at(0).Is("}") {
			if e.at(0).Is("{") {
				e.skipGroup("{", "}")
			} else if e.at(0).Is("(") {
				e.skipGroup("(", ")")
			} else {
				e.i++
			}
		}
		if e.at(0).Is(",") || e.at(0).Is(";") {
			e.i++
		}
		if e.i == start {
			e.i++
		}
	}
	if e.at(0).Is("}") {
		e.i++
	}
	return Value{Kind: TableValue, Table: t, Line: line}
}

// skipGroup skips a bracketed group starting at e.i, nested ones included.
func (e *evaluator) skipGroup(open, close string) {
	depth := 0
	for e.i < len(e.tokens) {
		t := e.at(0)
		e.i++
		if t.Is(open) {
			depth++
		} else if t.Is(close) {
			depth--
			if depth <= 0 {
				return
			}
		}
	}
}

// skipFunction skips a function expression through its matching end.
func (e *evaluator) skipFunction() {
	depth := 0
	for e.i < len(e.tokens) {
		t := e.at(0)
		e.i++
		if t.Kind != Name {
			continue
		}
		switch t.Text {
		case "function", "do", "if", "repeat":
			depth++
		case "end", "until":
			depth--
			if depth <= 0 {
				return
			}
		}
	}
}
