package luarocks

// VKind is what a Value holds.
type VKind uint8

const (
	Nil VKind = iota
	Str       // a string, or a number kept as written
	Bool
	Tab
)

// Value is what a constant Lua expression evaluates to. Anything the evaluator does
// not compute (a call, arithmetic, a function) is Nil.
type Value struct {
	Kind  VKind
	Str   string
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
		if v.Kind != Tab {
			return Value{}
		}
		v = v.Table.Get(k)
	}
	return v
}

// Strings is a list of strings: the positional string entries of a table.
func (v Value) Strings() []Value {
	if v.Kind != Tab {
		return nil
	}
	var out []Value
	for _, e := range v.Table.List {
		if e.Kind == Str {
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
func Eval(src []byte) *Chunk {
	e := &evaluator{tokens: Lex(src), env: map[string]Value{}}
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
	tokens []Token
	i      int
	env    map[string]Value
	depth  int
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
		for k, v := range e.exprList(len(names)) {
			e.env[names[k]] = v
		}
	case t.Is("return"):
		e.i++
		if vs := e.exprList(1); len(vs) > 0 {
			c.Return = vs[0]
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
			tg := target{name: e.at(0).Text}
			e.i++
			for {
				if e.at(0).Is(".") && e.at(1).Kind == Name {
					tg.keys = append(tg.keys, e.at(1).Text)
					e.i += 2
				} else if e.at(0).Is("[") {
					e.i++
					k := e.expr()
					if !e.at(0).Is("]") {
						return
					}
					e.i++
					tg.keys = append(tg.keys, k.Str)
				} else {
					break
				}
			}
			targets = append(targets, tg)
			if !e.at(0).Is(",") {
				break
			}
			e.i++
		}
		if !e.at(0).Is("=") {
			return
		}
		e.i++
		for k, v := range e.exprList(len(targets)) {
			tg := targets[k]
			if len(tg.keys) == 0 {
				e.env[tg.name] = v
				c.Globals[tg.name] = v
				continue
			}
			tab := e.env[tg.name]
			for _, key := range tg.keys[:len(tg.keys)-1] {
				tab = tab.Path(key)
			}
			if tab.Kind == Tab {
				tab.Table.Fields = append(tab.Table.Fields, Field{Key: tg.keys[len(tg.keys)-1], Value: v})
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

// exprList evaluates comma-separated expressions; at most n are kept.
func (e *evaluator) exprList(n int) []Value {
	var out []Value
	for {
		v := e.expr()
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

func (e *evaluator) expr() Value {
	if e.depth >= maxDepth {
		e.i++ // deeper than any real file: skipped a token at a time
		return Value{}
	}
	e.depth++
	defer func() { e.depth-- }()
	v := e.unary()
	for {
		op := e.at(0)
		if !(op.Kind == Punct || op.Kind == Name) || !binary[op.Text] {
			return v
		}
		e.i++
		w := e.unary()
		if op.Text == ".." && v.Kind == Str && w.Kind == Str {
			v = Value{Kind: Str, Str: v.Str + w.Str, Line: v.Line}
		} else if op.Text == "or" && v.Kind == Nil {
			v = w
		} else if op.Text != "or" {
			v = Value{Line: v.Line}
		}
	}
}

func (e *evaluator) unary() Value {
	op := false
	for e.at(0).Is("-") || e.at(0).Is("not") || e.at(0).Is("#") || e.at(0).Is("~") {
		e.i++
		op = true
	}
	if v := e.primary(); !op {
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
		v = Value{Kind: Str, Str: t.Text, Line: t.Line}
	case t.Is("true"), t.Is("false"):
		e.i++
		v = Value{Kind: Bool, Str: t.Text, Line: t.Line}
	case t.Is("nil"), t.Is("..."), t.Kind == Interp:
		e.i++
	case t.Is("{"):
		v = e.table()
	case t.Is("function"):
		e.skipFunction()
		return Value{}
	case t.Is("("):
		e.i++
		v = e.expr()
		if e.at(0).Is(")") {
			e.i++
		}
	case t.Kind == Name && !keyword[t.Text]:
		e.i++
		v = e.env[t.Text]
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
			k := e.expr()
			if e.at(0).Is("]") {
				e.i++
			}
			v = v.Path(k.Str)
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
			k := e.expr()
			if e.at(0).Is("]") && e.at(1).Is("=") {
				e.i += 2
				v := e.expr()
				if k.Kind == Str {
					t.Fields = append(t.Fields, Field{Key: k.Str, Value: v})
				}
			}
		case e.at(0).Kind == Name && e.at(1).Is("="):
			key := e.at(0).Text
			e.i += 2
			t.Fields = append(t.Fields, Field{Key: key, Value: e.expr()})
		default:
			t.List = append(t.List, e.expr())
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
	return Value{Kind: Tab, Table: t, Line: line}
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
