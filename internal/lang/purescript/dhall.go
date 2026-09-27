package purescript

import (
	"path"
	"strings"
	"unicode/utf8"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A small reader for the Dhall that spago.dhall and packages.dhall are written
// in. It evaluates what those files are made of - records, lists of strings,
// let bindings, field selection, `//` (record merge), `#` (list append), `with`,
// imports of other files and of remote package sets - and gives up, with an
// unknown value, on everything else (functions, unions, arithmetic, text
// interpolation). A remote import is never fetched: it stays a location, which
// is how a package set is named.

type dkind uint8

const (
	dUnknown dkind = iota
	dText
	dList
	dRecord
	dImport
)

// dval is a Dhall value as far as it could be evaluated.
type dval struct {
	kind   dkind
	text   string // dText: the string; dUnknown: the name of an unbound variable
	line   int
	list   []*dval
	fields map[string]*dval
	keys   []string // record fields in the order they were written
	// base is what a record was merged onto when that was not a record: a remote
	// package set extended with `//` or `with`.
	base *dval
	loc  string // dImport: the URL or the path as written
}

var unknown = &dval{}

func (v *dval) field(name string) *dval {
	if v != nil && v.kind == dRecord {
		if f, ok := v.fields[name]; ok {
			return f
		}
	}
	return unknown
}

// texts are a list's known strings.
func (v *dval) texts() []*dval {
	var out []*dval
	if v != nil && v.kind == dList {
		for _, e := range v.list {
			if e.kind == dText {
				out = append(out, e)
			}
		}
	}
	return out
}

func newRecord() *dval { return &dval{kind: dRecord, fields: map[string]*dval{}} }

func (v *dval) set(name string, f *dval) {
	if _, ok := v.fields[name]; !ok {
		v.keys = append(v.keys, name)
	}
	v.fields[name] = f
}

func (v *dval) copyRecord() *dval {
	out := newRecord()
	out.base = v.base
	for _, k := range v.keys {
		out.set(k, v.fields[k])
	}
	return out
}

// merge is `a // b`: b's fields over a's. A package set that is not a record
// (a remote import) becomes the base of the result.
func merge(a, b *dval) *dval {
	switch {
	case a.kind == dRecord && b.kind == dRecord:
		out := a.copyRecord()
		for _, k := range b.keys {
			out.set(k, b.fields[k])
		}
		return out
	case b.kind == dRecord:
		out := b.copyRecord()
		if out.base == nil {
			out.base = a
		}
		return out
	case a.kind == dRecord && b.kind == dImport:
		return b // the right side wins wholesale when nothing of it is known
	}
	return b
}

// appendList is `a # b`. A list whose other half is unknown keeps what is known
// of it: config.dependencies # [ "assert" ] still names assert.
func appendList(a, b *dval) *dval {
	if a.kind != dList && b.kind != dList {
		return unknown
	}
	out := &dval{kind: dList}
	for _, v := range []*dval{a, b} {
		if v.kind == dList {
			out.list = append(out.list, v.list...)
		}
	}
	return out
}

// withPath is `v with a.b = x`.
func withPath(v *dval, p []string, x *dval) *dval {
	var out *dval
	if v.kind == dRecord {
		out = v.copyRecord()
	} else {
		out = newRecord()
		out.base = v
	}
	if len(p) == 1 {
		out.set(p[0], x)
	} else {
		out.set(p[0], withPath(out.field(p[0]), p[1:], x))
	}
	return out
}

type dtokKind uint8

const (
	dtLabel dtokKind = iota
	dtString
	dtURL
	dtPath
	dtEnv
	dtHash
	dtNumber
	dtPunct
)

type dtok struct {
	kind        dtokKind
	text        string
	line        int
	interpolate bool // a string with ${...} in it: its value is not known
}

// dhallLex splits Dhall into tokens: comments (`--`, nested `{- -}`) are dropped,
// strings ("..." and ”...”) are single tokens, and so are URLs, paths, `env:`
// imports and `sha256:` hashes.
func dhallLex(src []byte) []dtok {
	s := string(src)
	var out []dtok
	line := 1
	emit := func(kind dtokKind, text string, at int) { out = append(out, dtok{kind: kind, text: text, line: at}) }
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
			out = append(out, dtok{kind: dtString, text: text.String(), line: start, interpolate: interpolate})
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
			out = append(out, dtok{kind: dtString, text: text.String(), line: start, interpolate: interpolate})
		case strings.HasPrefix(s[i:], "https://") || strings.HasPrefix(s[i:], "http://"):
			j := locationEnd(s, i)
			emit(dtURL, s[i:j], line)
			i = j
		case strings.HasPrefix(s[i:], "./") || strings.HasPrefix(s[i:], "../") || strings.HasPrefix(s[i:], "~/") ||
			c == '/' && i+1 < len(s) && (isWord(s[i+1]) || s[i+1] == '.' || s[i+1] == '-'):
			j := locationEnd(s, i)
			emit(dtPath, s[i:j], line)
			i = j
		case strings.HasPrefix(s[i:], "env:"):
			j := locationEnd(s, i)
			emit(dtEnv, s[i:j], line)
			i = j
		case strings.HasPrefix(s[i:], "sha256:"):
			j := i + 7
			for j < len(s) && isWord(s[j]) {
				j++
			}
			emit(dtHash, s[i:j], line)
			i = j
		case c == '`':
			j := strings.IndexAny(s[i+1:], "`\n")
			if j < 0 {
				j = len(s) - i - 1
			}
			emit(dtLabel, s[i+1:i+1+j], line)
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
			emit(dtLabel, s[i:j], line)
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < len(s) && (isWord(s[j]) || s[j] == '.') {
				j++
			}
			emit(dtNumber, s[i:j], line)
			i = j
		case c < 0x80:
			op := s[i : i+1]
			for _, m := range []string{"//\\\\", "===", "//", "/\\", "->", "==", "!=", "&&", "||", "++", "::"} {
				if strings.HasPrefix(s[i:], m) {
					op = m
					break
				}
			}
			emit(dtPunct, op, line)
			i += len(op)
		default:
			r, n := utf8.DecodeRuneInString(s[i:])
			switch r {
			case 'λ':
				emit(dtPunct, "\\", line)
			case '→':
				emit(dtPunct, "->", line)
			case '∀':
				emit(dtLabel, "forall", line)
			case '⫽':
				emit(dtPunct, "//", line)
			case '∧':
				emit(dtPunct, "/\\", line)
			case '⩓':
				emit(dtPunct, "//\\\\", line)
			default:
				emit(dtPunct, s[i:i+n], line)
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
	tokens []dtok
	i      int
	depth  int
	dir    string // the file's directory, to resolve relative imports
	// load evaluates a local file (a path relative to the repository), or
	// returns nil; nil load leaves every import a location.
	load func(file string) *dval
	env  []binding
}

type binding struct {
	name string
	v    *dval
}

// evalDhall evaluates a Dhall file. dir is its directory relative to the
// repository; load, when not nil, evaluates the local files it imports.
//
// Implements: REQ-PURESCRIPT-005
func evalDhall(src []byte, dir string, load func(string) *dval) *dval {
	r := &dhallReader{tokens: dhallLex(src), dir: dir, load: load}
	return r.expr(false)
}

func (r *dhallReader) peek() dtok {
	if r.i < len(r.tokens) {
		return r.tokens[r.i]
	}
	return dtok{kind: dtPunct, text: ""}
}

func (r *dhallReader) is(text string) bool {
	t := r.peek()
	return r.i < len(r.tokens) && (t.kind == dtPunct || t.kind == dtLabel) && t.text == text
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
	if t.kind == dtPunct {
		switch t.text {
		case ")", "]", "}", ",", "=", ":", ">", "|", "->":
			return true
		}
	}
	return t.kind == dtLabel && dhallStop[t.text]
}

var dhallStop = map[string]bool{"in": true, "then": true, "else": true, "with": true, "as": true, "using": true}

var dhallOps = map[string]bool{
	"//": true, "#": true, "?": true, "/\\": true, "//\\\\": true, "++": true, "+": true, "*": true,
	"&&": true, "||": true, "==": true, "!=": true, "===": true,
}

func (r *dhallReader) expr(noWith bool) *dval {
	if r.depth >= dhallMaxDepth || r.i >= len(r.tokens) {
		r.skipToEnd()
		return unknown
	}
	r.depth++
	defer func() { r.depth-- }()
	t := r.peek()
	switch {
	case t.kind == dtLabel && t.text == "let":
		mark := len(r.env)
		for r.accept("let") {
			name := r.peek()
			if name.kind != dtLabel {
				break
			}
			r.i++
			if r.accept(":") {
				r.opExpr(false)
			}
			if !r.accept("=") {
				break
			}
			r.env = append(r.env, binding{name.text, r.expr(false)})
		}
		r.accept("in")
		v := r.expr(noWith)
		r.env = r.env[:mark]
		return v
	case t.kind == dtPunct && t.text == "\\", t.kind == dtLabel && t.text == "forall":
		r.i++
		if r.is("(") {
			r.skipGroup()
		}
		r.accept("->")
		r.expr(noWith)
		return unknown
	case t.kind == dtLabel && t.text == "if":
		r.i++
		r.expr(false)
		r.accept("then")
		r.expr(false)
		r.accept("else")
		r.expr(noWith)
		return unknown
	case t.kind == dtLabel && t.text == "assert":
		r.i++
		r.accept(":")
		r.expr(noWith)
		return unknown
	}
	v := r.opExpr(noWith)
	if r.accept(":") {
		r.opExpr(noWith) // a type annotation
	}
	if r.accept("->") { // a function type
		r.expr(noWith)
		return unknown
	}
	return v
}

// skipToEnd moves past the rest of an expression too deep to read: to the next
// token that ends one.
func (r *dhallReader) skipToEnd() {
	for r.i < len(r.tokens) && !r.stop() {
		r.i++
	}
}

func (r *dhallReader) opExpr(noWith bool) *dval {
	v := r.withExpr(noWith)
	for r.i < len(r.tokens) {
		t := r.peek()
		if t.kind != dtPunct || !dhallOps[t.text] {
			break
		}
		r.i++
		w := r.withExpr(noWith)
		switch t.text {
		case "//":
			v = merge(v, w)
		case "#":
			v = appendList(v, w)
		case "?":
			if v.kind == dUnknown || v.kind == dImport && w.kind != dImport {
				v = w
			}
		default:
			v = unknown
		}
	}
	return v
}

func (r *dhallReader) withExpr(noWith bool) *dval {
	v := r.appExpr()
	for !noWith && r.accept("with") {
		var p []string
		for {
			t := r.peek()
			if t.kind != dtLabel {
				break
			}
			r.i++
			p = append(p, t.text)
			if !r.accept(".") {
				break
			}
		}
		if len(p) == 0 || !r.accept("=") {
			return unknown
		}
		v = withPath(v, p, r.opExpr(true))
	}
	return v
}

// appExpr reads a function application. Only two are understood: `Some x` is
// x, and mkPackage deps repo version (older package sets) is the record it
// makes.
func (r *dhallReader) appExpr() *dval {
	head := r.selector()
	var args []*dval
	for r.i < len(r.tokens) && !r.stop() {
		t := r.peek()
		if t.kind == dtPunct && (dhallOps[t.text] || t.text != "(" && t.text != "[" && t.text != "{" && t.text != "<") {
			break
		}
		if t.kind == dtLabel && (t.text == "let" || t.text == "if") {
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
	if head.kind == dUnknown && head.text == "Some" {
		return args[0]
	}
	if len(args) == 3 && (head.kind == dUnknown && head.text == "mkPackage" ||
		head.kind == dImport && strings.HasSuffix(head.loc, "mkPackage.dhall")) {
		out := newRecord()
		out.set("dependencies", args[0])
		out.set("repo", args[1])
		out.set("version", args[2])
		return out
	}
	return unknown
}

func (r *dhallReader) selector() *dval {
	v := r.primary()
	for r.is(".") {
		r.i++
		t := r.peek()
		switch {
		case t.kind == dtLabel:
			r.i++
			v = v.field(t.text)
		case r.is("{"), r.is("("):
			r.skipGroup() // a projection: what is known of v stays
		default:
			return unknown
		}
	}
	return v
}

func (r *dhallReader) primary() *dval {
	if r.i >= len(r.tokens) {
		return unknown
	}
	t := r.peek()
	switch t.kind {
	case dtString:
		r.i++
		if t.interpolate {
			return unknown
		}
		return &dval{kind: dText, text: t.text, line: t.line}
	case dtURL, dtPath, dtEnv:
		r.i++
		if r.peek().kind == dtHash {
			r.i++
		}
		if r.accept("using") {
			r.selector()
		}
		as := ""
		if r.accept("as") {
			as = r.peek().text
			r.i++
		}
		if t.kind == dtEnv {
			return unknown
		}
		if t.kind == dtPath && as == "" && r.load != nil && !strings.HasPrefix(t.text, "/") && !strings.HasPrefix(t.text, "~") {
			if v := r.load(path.Join(r.dir, t.text)); v != nil {
				return v
			}
		}
		if as == "Location" || as == "Text" || as == "Bytes" {
			return unknown
		}
		return &dval{kind: dImport, loc: t.text, line: t.line}
	case dtLabel:
		r.i++
		for k := len(r.env) - 1; k >= 0; k-- {
			if r.env[k].name == t.text {
				return r.env[k].v
			}
		}
		return &dval{text: t.text} // unbound: a builtin, or mkPackage
	case dtNumber, dtHash:
		r.i++
		return unknown
	}
	switch t.text {
	case "(":
		r.i++
		v := r.expr(false)
		r.closeGroup(")")
		return v
	case "[":
		r.i++
		out := &dval{kind: dList}
		for r.i < len(r.tokens) && !r.is("]") {
			start := r.i
			if !r.accept(",") {
				out.list = append(out.list, r.expr(false))
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
		return unknown
	}
	return unknown // a closer or an operator where a value was expected: not consumed
}

// record reads { a = 1, b.c = "x", d } or a record type { a : Text }.
func (r *dhallReader) record() *dval {
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
			if t.kind != dtLabel {
				break
			}
			r.i++
			p = append(p, t.text)
			if !r.accept(".") {
				break
			}
		}
		switch {
		case len(p) > 0 && r.accept("="):
			v := r.expr(false)
			if len(p) == 1 {
				out.set(p[0], v)
			} else {
				out.set(p[0], withPath(out.field(p[0]), p[1:], v))
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
		return unknown
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
		if t.kind != dtPunct {
			continue
		}
		switch t.text {
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
		if t.kind != dtPunct {
			continue
		}
		switch t.text {
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

// setName names a package set by the import it starts from: the release a
// package-sets URL downloads (psc-0.15.0-20220507), else the URL's repository.
func setName(v *dval) string {
	for k := 0; v != nil && k < 64; k++ {
		switch v.kind {
		case dImport:
			if !strings.Contains(v.loc, "://") {
				return ""
			}
			u := strings.TrimSuffix(v.loc, "/packages.dhall")
			if u != v.loc {
				return u[strings.LastIndexByte(u, '/')+1:]
			}
			return lang.RepoName(v.loc)
		case dRecord:
			v = v.base
		default:
			return ""
		}
	}
	return ""
}
