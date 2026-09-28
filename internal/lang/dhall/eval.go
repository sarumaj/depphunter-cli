package dhall

import (
	"path"
	"strings"
	"unicode/utf8"
)

// A small reader for Dhall, shared with the purescript plugin (spago.dhall and
// packages.dhall are Dhall). It evaluates what those files are made of - records, lists of strings,
// let bindings, field selection, `//` (record merge), `#` (list append), `with`,
// imports of other files and of remote package sets - and gives up, with an
// Unknown value, on everything else (functions, unions, arithmetic, text
// interpolation). A remote import is never fetched: it stays a location, which
// is how a package set is named.

// Kind is what a Value is known to be.
type Kind uint8

const (
	KindUnknown Kind = iota
	KindText
	KindList
	KindRecord
	KindImport
)

// Value is a Dhall value as far as it could be evaluated.
type Value struct {
	Kind   Kind
	Text   string // KindText: the string; KindUnknown: the name of an unbound variable
	Line   int
	List   []*Value
	Fields map[string]*Value
	Keys   []string // record fields in the order they were written
	// Base is what a record was merged onto when that was not a record: a remote
	// package set extended with `//` or `with`.
	Base *Value
	Loc  string // KindImport: the URL or the path as written
}

// Unknown is the value of everything the reader does not evaluate.
var Unknown = &Value{}

// Field is a record's field, or Unknown.
func (v *Value) Field(name string) *Value {
	if v != nil && v.Kind == KindRecord {
		if f, ok := v.Fields[name]; ok {
			return f
		}
	}
	return Unknown
}

// Texts are a list's known strings.
func (v *Value) Texts() []*Value {
	var out []*Value
	if v != nil && v.Kind == KindList {
		for _, e := range v.List {
			if e.Kind == KindText {
				out = append(out, e)
			}
		}
	}
	return out
}

func newRecord() *Value { return &Value{Kind: KindRecord, Fields: map[string]*Value{}} }

func (v *Value) set(name string, f *Value) {
	if _, ok := v.Fields[name]; !ok {
		v.Keys = append(v.Keys, name)
	}
	v.Fields[name] = f
}

func (v *Value) copyRecord() *Value {
	out := newRecord()
	out.Base = v.Base
	for _, k := range v.Keys {
		out.set(k, v.Fields[k])
	}
	return out
}

// merge is `a // b`: b's fields over a's. A package set that is not a record
// (a remote import) becomes the base of the result.
func merge(a, b *Value) *Value {
	switch {
	case a.Kind == KindRecord && b.Kind == KindRecord:
		out := a.copyRecord()
		for _, k := range b.Keys {
			out.set(k, b.Fields[k])
		}
		return out
	case b.Kind == KindRecord:
		out := b.copyRecord()
		if out.Base == nil {
			out.Base = a
		}
		return out
	case a.Kind == KindRecord && b.Kind == KindImport:
		return b // the right side wins wholesale when nothing of it is known
	}
	return b
}

// appendList is `a # b`. A list whose other half is Unknown keeps what is known
// of it: config.dependencies # [ "assert" ] still names assert.
func appendList(a, b *Value) *Value {
	if a.Kind != KindList && b.Kind != KindList {
		return Unknown
	}
	out := &Value{Kind: KindList}
	for _, v := range []*Value{a, b} {
		if v.Kind == KindList {
			out.List = append(out.List, v.List...)
		}
	}
	return out
}

// withPath is `v with a.b = x`.
func withPath(v *Value, p []string, x *Value) *Value {
	var out *Value
	if v.Kind == KindRecord {
		out = v.copyRecord()
	} else {
		out = newRecord()
		out.Base = v
	}
	if len(p) == 1 {
		out.set(p[0], x)
	} else {
		out.set(p[0], withPath(out.Field(p[0]), p[1:], x))
	}
	return out
}

// TokenKind tells Dhall's tokens apart.
type TokenKind uint8

const (
	TokLabel TokenKind = iota
	TokString
	TokURL
	TokPath
	TokEnv
	TokHash
	TokNumber
	TokPunct
)

// Token is one Dhall token.
type Token struct {
	Kind        TokenKind
	Text        string
	Line        int
	Interpolate bool // a string with ${...} in it: its value is not known
}

// Lex splits Dhall into tokens: comments (`--`, nested `{- -}`) are dropped,
// strings ("..." and ”...”) are single tokens, and so are URLs, paths, `env:`
// imports and `sha256:` hashes.
//
// Implements: REQ-DHALL-007, REQ-PURESCRIPT-005
func Lex(src []byte) []Token {
	s := string(src)
	var out []Token
	line := 1
	emit := func(kind TokenKind, text string, at int) { out = append(out, Token{Kind: kind, Text: text, Line: at}) }
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '-' && at(s, i+1) == '-':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '{' && at(s, i+1) == '-':
			depth := 0
			for i < len(s) {
				switch {
				case s[i] == '{' && at(s, i+1) == '-':
					depth++
					i += 2
				case s[i] == '-' && at(s, i+1) == '}':
					depth--
					i += 2
				default:
					if s[i] == '\n' {
						line++
					}
					i++
				}
				if depth == 0 {
					break
				}
			}
			i = min(i, len(s))
		case c == '"':
			start, text, interpolate := line, strings.Builder{}, false
			for i++; i < len(s) && s[i] != '"'; i++ {
				switch {
				case s[i] == '\\' && i+1 < len(s):
					i++
					switch s[i] {
					case 'n':
						text.WriteByte('\n')
					case 't':
						text.WriteByte('\t')
					default:
						text.WriteByte(s[i])
					}
				case s[i] == '$' && at(s, i+1) == '{':
					interpolate = true
					i = skipInterp(s, i+2, &line) - 1
				default:
					if s[i] == '\n' {
						line++
					}
					text.WriteByte(s[i])
				}
			}
			i = min(i+1, len(s))
			out = append(out, Token{Kind: TokString, Text: text.String(), Line: start, Interpolate: interpolate})
		case c == '\'' && at(s, i+1) == '\'':
			start, text, interpolate := line, strings.Builder{}, false
			for i += 2; i < len(s); i++ {
				switch {
				case strings.HasPrefix(s[i:], "'''"):
					text.WriteString("''")
					i += 2
				case strings.HasPrefix(s[i:], "''${"):
					text.WriteString("${")
					i += 3
				case strings.HasPrefix(s[i:], "''"):
					i += 2
					goto closed
				case s[i] == '$' && at(s, i+1) == '{':
					interpolate = true
					i = skipInterp(s, i+2, &line) - 1
				default:
					if s[i] == '\n' {
						line++
					}
					text.WriteByte(s[i])
				}
			}
		closed:
			i = min(i, len(s))
			out = append(out, Token{Kind: TokString, Text: text.String(), Line: start, Interpolate: interpolate})
		case strings.HasPrefix(s[i:], "https://") || strings.HasPrefix(s[i:], "http://"):
			j := locationEnd(s, i)
			emit(TokURL, s[i:j], line)
			i = j
		case strings.HasPrefix(s[i:], "./") || strings.HasPrefix(s[i:], "../") || strings.HasPrefix(s[i:], "~/") ||
			c == '/' && i+1 < len(s) && (isWord(s[i+1]) || s[i+1] == '.' || s[i+1] == '-'):
			j := locationEnd(s, i)
			emit(TokPath, s[i:j], line)
			i = j
		case strings.HasPrefix(s[i:], "env:"):
			j := locationEnd(s, i)
			emit(TokEnv, s[i:j], line)
			i = j
		case strings.HasPrefix(s[i:], "sha256:"):
			j := i + 7
			for j < len(s) && isWord(s[j]) {
				j++
			}
			emit(TokHash, s[i:j], line)
			i = j
		case c == '`':
			j := strings.IndexAny(s[i+1:], "`\n")
			if j < 0 {
				j = len(s) - i - 1
			}
			emit(TokLabel, s[i+1:i+1+j], line)
			i = min(i+2+j, len(s))
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_':
			j := i + 1
			for j < len(s) {
				d := s[j]
				if isWord(d) || d == '-' && at(s, j+1) != '>' || d == '/' && isWord(at(s, j+1)) {
					j++
					continue
				}
				break
			}
			emit(TokLabel, s[i:j], line)
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < len(s) && (isWord(s[j]) || s[j] == '.') {
				j++
			}
			emit(TokNumber, s[i:j], line)
			i = j
		case c < 0x80:
			op := s[i : i+1]
			for _, m := range []string{"//\\\\", "===", "//", "/\\", "->", "==", "!=", "&&", "||", "++", "::"} {
				if strings.HasPrefix(s[i:], m) {
					op = m
					break
				}
			}
			emit(TokPunct, op, line)
			i += len(op)
		default:
			r, n := utf8.DecodeRuneInString(s[i:])
			switch r {
			case 'λ':
				emit(TokPunct, "\\", line)
			case '→':
				emit(TokPunct, "->", line)
			case '∀':
				emit(TokLabel, "forall", line)
			case '⫽':
				emit(TokPunct, "//", line)
			case '≡':
				emit(TokPunct, "===", line)
			case '∧':
				emit(TokPunct, "/\\", line)
			case '⩓':
				emit(TokPunct, "//\\\\", line)
			default:
				emit(TokPunct, s[i:i+n], line)
			}
			i += n
		}
	}
	return out
}

// skipInterp returns the index just past the `}` that closes an interpolation
// whose code starts at i, counting braces (a string inside it is not read).
func skipInterp(s string, i int, line *int) int {
	depth := 1
	for ; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i + 1
			}
		case '\n':
			*line++
		}
	}
	return len(s)
}

// locationEnd is the end of a URL or path starting at i: the next space or
// closing bracket.
func locationEnd(s string, i int) int {
	for i < len(s) && !strings.ContainsRune(" \t\r\n)]},", rune(s[i])) {
		i++
	}
	return i
}

// dhallMaxDepth bounds the nesting of expressions the reader follows.
const dhallMaxDepth = 200

// dhallReader evaluates one file's tokens.
type dhallReader struct {
	tokens []Token
	i      int
	depth  int
	dir    string // the file's directory, to resolve relative imports
	// load evaluates a local file (a path relative to the repository), or
	// returns nil; nil load leaves every import a location.
	load func(file string) *Value
	env  []binding
	// declare, when set, is told the file's top-level declarations: the
	// bindings of the let chain the file starts with and the fields of a
	// record at its top; top is the depth those are read at.
	declare func(name, kind string, line int)
	top     int
}

type binding struct {
	name string
	v    *Value
}

// Eval evaluates a Dhall file. dir is its directory relative to the
// repository; load, when not nil, evaluates the local files it imports.
//
// Implements: REQ-PURESCRIPT-005, REQ-DHALL-003
func Eval(src []byte, dir string, load func(string) *Value) *Value {
	r := &dhallReader{tokens: Lex(src), dir: dir, load: load}
	return r.expr(false)
}

// Declarations evaluates a file only to report its top-level declarations:
// the let bindings it starts with (kind "type" for a capitalized name,
// "function" for a lambda, else "let") and the fields of the record it
// evaluates to at its top ("field").
//
// Implements: REQ-DHALL-003
func Declarations(src []byte, declare func(name, kind string, line int)) {
	r := &dhallReader{tokens: Lex(src), dir: ".", declare: declare, top: 1}
	r.expr(false)
}

func bindingKind(name string, next Token) string {
	switch {
	case next.Kind == TokPunct && next.Text == "\\":
		return "function"
	case name[0] >= 'A' && name[0] <= 'Z':
		return "type"
	}
	return "let"
}

func (r *dhallReader) peek() Token {
	if r.i < len(r.tokens) {
		return r.tokens[r.i]
	}
	return Token{Kind: TokPunct, Text: ""}
}

func (r *dhallReader) is(text string) bool {
	t := r.peek()
	return r.i < len(r.tokens) && (t.Kind == TokPunct || t.Kind == TokLabel) && t.Text == text
}

func (r *dhallReader) accept(text string) bool {
	if r.is(text) {
		r.i++
		return true
	}
	return false
}

// stop reports whether the next token ends an expression.
func (r *dhallReader) stop() bool {
	if r.i >= len(r.tokens) {
		return true
	}
	t := r.peek()
	if t.Kind == TokPunct {
		switch t.Text {
		case ")", "]", "}", ",", "=", ":", ">", "|", "->":
			return true
		}
	}
	return t.Kind == TokLabel && dhallStop[t.Text]
}

var dhallStop = map[string]bool{"in": true, "then": true, "else": true, "with": true, "as": true, "using": true}

var dhallOps = map[string]bool{
	"//": true, "#": true, "?": true, "/\\": true, "//\\\\": true, "++": true, "+": true, "*": true,
	"&&": true, "||": true, "==": true, "!=": true, "===": true,
}

func (r *dhallReader) expr(noWith bool) *Value {
	if r.depth >= dhallMaxDepth || r.i >= len(r.tokens) {
		r.skipToEnd()
		return Unknown
	}
	r.depth++
	defer func() { r.depth-- }()
	t := r.peek()
	switch {
	case t.Kind == TokLabel && t.Text == "let":
		mark := len(r.env)
		top := r.declare != nil && r.depth == r.top
		for r.accept("let") {
			name := r.peek()
			if name.Kind != TokLabel {
				break
			}
			r.i++
			if r.accept(":") {
				r.expr(false) // the type: ∀(a : Type) → a
			}
			if !r.accept("=") {
				break
			}
			if top {
				r.declare(name.Text, bindingKind(name.Text, r.peek()), name.Line)
			}
			r.env = append(r.env, binding{name.Text, r.expr(false)})
			r.skipBinding()
		}
		r.accept("in")
		if top {
			r.top++ // the body of the file's let chain is its top too
		}
		v := r.expr(noWith)
		r.env = r.env[:mark]
		return v
	case t.Kind == TokPunct && t.Text == "\\", t.Kind == TokLabel && t.Text == "forall":
		r.i++
		if r.is("(") {
			r.skipGroup()
		}
		r.accept("->")
		r.expr(noWith)
		return Unknown
	case t.Kind == TokLabel && t.Text == "if":
		r.i++
		r.expr(false)
		r.accept("then")
		r.expr(false)
		r.accept("else")
		r.expr(noWith)
		return Unknown
	case t.Kind == TokLabel && t.Text == "assert":
		r.i++
		r.accept(":")
		r.expr(noWith)
		return Unknown
	}
	v := r.opExpr(noWith)
	if r.accept(":") {
		r.opExpr(noWith) // a type annotation
	}
	if r.accept("->") { // a function type
		r.expr(noWith)
		return Unknown
	}
	return v
}

// skipBinding moves past what is left of a let binding's value the reader
// did not understand (an operator it does not know, `≡`): to the next `let`
// or `in` outside brackets, or the bracket that closes the chain.
func (r *dhallReader) skipBinding() {
	for r.i < len(r.tokens) && !r.is("let") && !r.is("in") {
		t := r.peek()
		if t.Kind == TokPunct {
			switch t.Text {
			case "(", "[", "{":
				r.skipGroup()
				continue
			case ")", "]", "}":
				return
			}
		}
		r.i++
	}
}

// skipToEnd moves past the rest of an expression too deep to read: to the next
// token that ends one.
func (r *dhallReader) skipToEnd() {
	for r.i < len(r.tokens) && !r.stop() {
		r.i++
	}
}

func (r *dhallReader) opExpr(noWith bool) *Value {
	v := r.withExpr(noWith)
	for r.i < len(r.tokens) {
		t := r.peek()
		if t.Kind != TokPunct || !dhallOps[t.Text] {
			break
		}
		r.i++
		w := r.withExpr(noWith)
		switch t.Text {
		case "//":
			v = merge(v, w)
		case "#":
			v = appendList(v, w)
		case "?":
			if v.Kind == KindUnknown || v.Kind == KindImport && w.Kind != KindImport {
				v = w
			}
		default:
			v = Unknown
		}
	}
	return v
}

func (r *dhallReader) withExpr(noWith bool) *Value {
	v := r.appExpr()
	for !noWith && r.accept("with") {
		var p []string
		for {
			t := r.peek()
			if t.Kind != TokLabel {
				break
			}
			r.i++
			p = append(p, t.Text)
			if !r.accept(".") {
				break
			}
		}
		if len(p) == 0 || !r.accept("=") {
			return Unknown
		}
		v = withPath(v, p, r.opExpr(true))
	}
	return v
}

// appExpr reads a function application. Only two are understood: `Some x` is
// x, and mkPackage deps repo version (older package sets) is the record it
// makes.
func (r *dhallReader) appExpr() *Value {
	head := r.selector()
	var args []*Value
	for r.i < len(r.tokens) && !r.stop() {
		t := r.peek()
		if t.Kind == TokPunct && (dhallOps[t.Text] || t.Text != "(" && t.Text != "[" && t.Text != "{" && t.Text != "<") {
			break
		}
		if t.Kind == TokLabel && (t.Text == "let" || t.Text == "if") {
			break
		}
		start := r.i
		args = append(args, r.selector())
		if r.i == start {
			r.i++
		}
	}
	if len(args) == 0 {
		return head
	}
	if head.Kind == KindUnknown && head.Text == "Some" {
		return args[0]
	}
	if len(args) == 3 && (head.Kind == KindUnknown && head.Text == "mkPackage" ||
		head.Kind == KindImport && strings.HasSuffix(head.Loc, "mkPackage.dhall")) {
		out := newRecord()
		out.set("dependencies", args[0])
		out.set("repo", args[1])
		out.set("version", args[2])
		return out
	}
	return Unknown
}

func (r *dhallReader) selector() *Value {
	v := r.primary()
	for r.is(".") {
		r.i++
		t := r.peek()
		switch {
		case t.Kind == TokLabel:
			r.i++
			v = v.Field(t.Text)
		case r.is("{"), r.is("("):
			r.skipGroup() // a projection: what is known of v stays
		default:
			return Unknown
		}
	}
	return v
}

func (r *dhallReader) primary() *Value {
	if r.i >= len(r.tokens) {
		return Unknown
	}
	t := r.peek()
	switch t.Kind {
	case TokString:
		r.i++
		if t.Interpolate {
			return Unknown
		}
		return &Value{Kind: KindText, Text: t.Text, Line: t.Line}
	case TokURL, TokPath, TokEnv:
		r.i++
		if r.peek().Kind == TokHash {
			r.i++
		}
		if r.accept("using") {
			r.selector()
		}
		as := ""
		if r.accept("as") {
			as = r.peek().Text
			r.i++
		}
		if t.Kind == TokEnv {
			return Unknown
		}
		if t.Kind == TokPath && as == "" && r.load != nil && !strings.HasPrefix(t.Text, "/") && !strings.HasPrefix(t.Text, "~") {
			if v := r.load(path.Join(r.dir, t.Text)); v != nil {
				return v
			}
		}
		if as == "Location" || as == "Text" || as == "Bytes" {
			return Unknown
		}
		return &Value{Kind: KindImport, Loc: t.Text, Line: t.Line}
	case TokLabel:
		r.i++
		for k := len(r.env) - 1; k >= 0; k-- {
			if r.env[k].name == t.Text {
				return r.env[k].v
			}
		}
		return &Value{Text: t.Text} // unbound: a builtin, or mkPackage
	case TokNumber, TokHash:
		r.i++
		return Unknown
	}
	switch t.Text {
	case "(":
		r.i++
		v := r.expr(false)
		r.closeGroup(")")
		return v
	case "[":
		r.i++
		out := &Value{Kind: KindList}
		for r.i < len(r.tokens) && !r.is("]") {
			start := r.i
			if !r.accept(",") {
				out.List = append(out.List, r.expr(false))
			}
			if r.i == start {
				if r.stop() && !r.is(",") {
					break
				}
				r.i++
			}
		}
		r.closeGroup("]")
		return out
	case "{":
		return r.record()
	case "<":
		r.skipUnion()
		return Unknown
	}
	return Unknown // a closer or an operator where a value was expected: not consumed
}

// record reads { a = 1, b.c = "x", d } or a record type { a : Text }.
func (r *dhallReader) record() *Value {
	r.i++ // {
	out := newRecord()
	if r.accept("=") {
		r.closeGroup("}")
		return out
	}
	typ := false
	for r.i < len(r.tokens) && !r.is("}") {
		start := r.i
		if r.accept(",") {
			continue
		}
		var p []string
		for {
			t := r.peek()
			if t.Kind != TokLabel {
				break
			}
			r.i++
			p = append(p, t.Text)
			if !r.accept(".") {
				break
			}
		}
		if len(p) > 0 && r.declare != nil && r.depth == r.top {
			r.declare(p[0], "field", r.tokens[start].Line)
		}
		switch {
		case len(p) > 0 && r.accept("="):
			v := r.expr(false)
			if len(p) == 1 {
				out.set(p[0], v)
			} else {
				out.set(p[0], withPath(out.Field(p[0]), p[1:], v))
			}
		case len(p) > 0 && r.accept(":"):
			typ = true
			r.expr(false)
		case len(p) == 1 && (r.is(",") || r.is("}")):
			// A punned field: { x } is { x = x }.
			for k := len(r.env) - 1; k >= 0; k-- {
				if r.env[k].name == p[0] {
					out.set(p[0], r.env[k].v)
					break
				}
			}
		}
		if r.i == start {
			r.i++
		}
		// Anything else up to the next field is not understood.
		for r.i < len(r.tokens) && !r.is(",") && !r.is("}") {
			if r.is("(") || r.is("[") || r.is("{") {
				r.skipGroup()
				continue
			}
			r.i++
		}
	}
	r.closeGroup("}")
	if typ {
		return Unknown
	}
	return out
}

func (r *dhallReader) closeGroup(closer string) {
	r.accept(closer)
}

// skipGroup moves past a bracketed group, brackets counted without a stack.
func (r *dhallReader) skipGroup() {
	depth := 0
	for r.i < len(r.tokens) {
		t := r.tokens[r.i]
		r.i++
		if t.Kind != TokPunct {
			continue
		}
		switch t.Text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
		if depth <= 0 {
			return
		}
	}
}

// skipUnion moves past a union type or literal < A | B : T >.
func (r *dhallReader) skipUnion() {
	depth := 0
	for r.i < len(r.tokens) {
		t := r.tokens[r.i]
		r.i++
		if t.Kind != TokPunct {
			continue
		}
		switch t.Text {
		case "<":
			depth++
		case ">":
			depth--
		}
		if depth <= 0 {
			return
		}
	}
}

func at(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func isWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// Spago reports whether a Dhall file is spago's: the package set
// packages.dhall, spago.dhall and other files whose name says spago
// (spago-test.dhall), and a test/ configuration test.dhall. The purescript
// plugin reads these; the dhall plugin leaves them to it.
//
// Implements: REQ-PURESCRIPT-001, REQ-DHALL-001
func Spago(p string) bool {
	base := path.Base(p)
	return path.Ext(base) == ".dhall" && (base == "packages.dhall" || base == "test.dhall" || strings.Contains(base, "spago"))
}
