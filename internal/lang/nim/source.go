package nim

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindImport  = "import"
	kindInclude = "include"
	kindRequire = "require" // a .nimble requires/taskRequires entry
	kindBin     = "bin"     // a .nimble bin entry: the program's main module
	kindPath    = "path"    // a --path of a NimScript or nim.cfg configuration
	kindLock    = "lock"    // a nimble.lock package
	kindAtlas   = "atlas"   // an atlas.lock item
)

// maxGroups bounds the nesting of import groups (std/[a, b/[c]]), maxDepth
// that of when branches written on one line.
const (
	maxGroups = 4
	maxDepth  = 16
)

// line is a statement's first line: its tokens and indentation. A statement
// starts at the first token of a line outside brackets, or after a `;`.
type line struct {
	start, end int // token indexes
	indent     int
}

func (l *lexer) lines() []line {
	var out []line
	for i, t := range l.tokens {
		switch {
		case t.first && t.depth == 0:
			out = append(out, line{start: i, indent: t.col})
		case i > 0 && l.tokens[i-1].kind == tSemi && t.depth == 0 && len(out) > 0:
			out = append(out, line{start: i, indent: out[len(out)-1].indent})
		default:
			continue
		}
		if n := len(out); n > 1 {
			out[n-2].end = i
			if l.tokens[i-1].kind == tSemi {
				out[n-2].end = i - 1
			}
		}
	}
	if n := len(out); n > 0 {
		out[n-1].end = len(l.tokens)
	}
	return out
}

// Frame kinds: what the lines indented below a statement are.
const (
	fModule = iota // the module's top level
	fWhen          // a when/elif/else branch at top level: still top level
	fType          // a type section: definitions at its first child indentation
	fConst         // a const/let/var section
	fSkip          // a body, an object's fields, code: not read
)

type frame struct {
	indent int
	kind   int
	child  int    // the indentation of a section's definitions, -1 before the first
	word   string // const, let or var
}

// source is what a Nim file (or NimScript: .nims, .nimble) says: its imports,
// its top-level symbols, and the manifest statements of NimScript.
type source struct {
	l       *lexer
	lines   []line
	frames  []frame
	imports []lang.RawImport
	seen    map[string]bool
	symbols lang.SymbolSet
	// assigns are the top-level `name = "value"` and `name = @["a", "b"]`
	// statements (and the call form `name "value"`), the first of each name.
	assigns map[string]assign
	inline  int // when branches nested on one line
}

type assign struct {
	values []string
	line   int
}

// procKinds are the routine keywords and the symbol kind each gives.
var procKinds = map[string]string{
	"proc": "func", "func": "func", "method": "method", "iterator": "iterator",
	"converter": "converter", "template": "template", "macro": "macro",
}

// scanSource reads a Nim module: import, include and from statements at its
// top level (also inside when branches, all of which are read), and its
// top-level routines, types, constants and variables.
//
// Implements: REQ-NIM-002, REQ-NIM-003, REQ-NIM-010
func scanSource(src []byte) *source {
	l := lex(src)
	s := &source{l: l, lines: l.lines(), seen: map[string]bool{}, assigns: map[string]assign{}}
	s.frames = []frame{{indent: -1, kind: fModule}}
	for k := 0; k < len(s.lines); k++ {
		ln := s.lines[k]
		var sibling *frame
		for len(s.frames) > 1 && s.frames[len(s.frames)-1].indent >= ln.indent {
			top := s.frames[len(s.frames)-1]
			if top.indent == ln.indent {
				sibling = &top
			}
			s.frames = s.frames[:len(s.frames)-1]
		}
		top := &s.frames[len(s.frames)-1]
		switch top.kind {
		case fSkip:
			continue
		case fType, fConst:
			if top.child < 0 {
				top.child = ln.indent
			}
			if ln.indent == top.child {
				if top.kind == fType {
					s.typeDef(ln.start, ln.end)
				} else {
					s.names(top.word, ln.start, ln.end)
				}
			}
			s.push(ln.indent, fSkip)
			continue
		}
		k = s.statement(k, ln.start, ln.end, ln.indent, sibling)
	}
	return s
}

func (s *source) push(indent, kind int) {
	if len(s.frames) < 256 {
		s.frames = append(s.frames, frame{indent: indent, kind: kind, child: -1})
	}
}

func (s *source) ident(i int) string {
	if i < 0 || i >= len(s.l.tokens) || s.l.tokens[i].kind != tIdent {
		return ""
	}
	return s.l.text(s.l.tokens[i])
}

// statement reads the top-level statement tokens[a:b] of line k and returns the
// last line it consumed.
func (s *source) statement(k, a, b, indent int, sibling *frame) int {
	l := s.l
	word := s.ident(a)
	switch word {
	case "import", "include":
		end, last := s.extent(k, b, indent)
		kind := kindImport
		if word == "include" {
			kind = kindInclude
		}
		s.importList(kind, a+1, end)
		return last
	case "from":
		end, last := s.extent(k, b, indent)
		for j := a + 1; j < end; j++ {
			if l.tokens[j].depth == l.tokens[a].depth && s.ident(j) == "import" {
				if mods := s.modules(a+1, j, 0); len(mods) > 0 {
					s.addImport(kindImport, mods[0], l.tokens[a].line)
				}
				break
			}
		}
		return last
	case "type":
		if a+1 >= b {
			s.frames = append(s.frames, frame{indent: indent, kind: fType, child: -1})
		} else {
			s.typeDef(a+1, b)
			s.push(indent, fSkip)
		}
		return k
	case "const", "let", "var":
		if a+1 >= b {
			s.frames = append(s.frames, frame{indent: indent, kind: fConst, child: -1, word: word})
		} else {
			s.names(word, a+1, b)
			s.push(indent, fSkip)
		}
		return k
	case "when":
		return s.branch(k, a, b, indent)
	case "elif", "else":
		if sibling != nil && sibling.kind == fWhen {
			return s.branch(k, a, b, indent)
		}
	case "task":
		// NimScript: task name, "description": body.
		if name := s.ident(a + 1); name != "" && l.is(a+2, tComma, ",") {
			s.symbols.Add(name, "task", l.tokens[a].line)
		}
	default:
		if kind, ok := procKinds[word]; ok {
			s.routine(kind, a+1, b)
		} else if word != "" && a+1 < b {
			s.assign(word, a, b)
		}
	}
	s.push(indent, fSkip)
	return k
}

// extent is where a statement starting on line k ends: its first line and every
// following line indented deeper (import lists written one module per line).
func (s *source) extent(k, b, indent int) (end, last int) {
	end, last = b, k
	for last+1 < len(s.lines) && s.lines[last+1].indent > indent {
		last++
		end = s.lines[last].end
	}
	return end, last
}

// branch reads a when/elif/else line: its body is still the top level, and a
// statement after its colon on the same line is read as one.
func (s *source) branch(k, a, b, indent int) int {
	s.frames = append(s.frames, frame{indent: indent, kind: fWhen, child: -1})
	base := s.l.tokens[a].depth
	for j := a + 1; j < b; j++ {
		t := s.l.tokens[j]
		if t.depth == base && t.kind == tOp && s.l.text(t) == ":" {
			if j+1 < b && s.inline < maxDepth {
				// when a: when b: import x - bounded.
				s.inline++
				k = s.statement(k, j+1, b, indent+1, nil)
				s.inline--
			}
			break
		}
	}
	return k
}

// importList reads `a, b/c, std/[os, strutils], "x.nim", m as n, m except x`
// from tokens[a:b].
func (s *source) importList(kind string, a, b int) {
	if a >= b {
		return
	}
	line := s.l.tokens[a-1].line
	base := s.l.tokens[a].depth
	start := a
	for j := a; j <= b; j++ {
		if j < b {
			t := s.l.tokens[j]
			if t.depth != base || t.kind != tComma {
				if t.depth == base && t.kind == tIdent && s.l.text(t) == "except" {
					// The rest names symbols, not modules.
					for _, m := range s.modules(start, j, 0) {
						s.addImport(kind, m, line)
					}
					return
				}
				continue
			}
		}
		for _, m := range s.modules(start, j, 0) {
			s.addImport(kind, m, line)
		}
		start = j + 1
	}
}

// modules turns one import item tokens[a:b] into module paths: the tokens joined
// (`std / os` is std/os, `../x` is ../x, "x.nim" is x), a bracket group
// expanding to one path per member, `as alias` dropped.
func (s *source) modules(a, b, depth int) []string {
	var prefix strings.Builder
	name := false // the last token was a name: another name needs a `/` first
	for j := a; j < b; j++ {
		t := s.l.tokens[j]
		switch t.kind {
		case tIdent, tString, tNumber:
			if t.kind == tIdent && s.l.text(t) == "as" && prefix.Len() > 0 {
				return clean([]string{prefix.String()})
			}
			if name {
				return nil
			}
			name = true
			prefix.WriteString(s.l.text(t))
		case tOp:
			if strings.Trim(s.l.text(t), "/.$") != "" {
				return nil
			}
			name = false
			prefix.WriteString(s.l.text(t))
		case tOpen:
			if s.l.src[t.start] != '[' || depth >= maxGroups {
				return nil
			}
			close := s.closer(j, b)
			var out []string
			start := j + 1
			for m := j + 1; m <= close; m++ {
				if m < close && (s.l.tokens[m].kind != tComma || s.l.tokens[m].depth != t.depth+1) {
					continue
				}
				for _, sub := range s.modules(start, m, depth+1) {
					out = append(out, prefix.String()+sub)
				}
				start = m + 1
			}
			return clean(out)
		default:
			return nil
		}
	}
	return clean([]string{prefix.String()})
}

// closer is the index of the bracket closing the one at j, or b.
func (s *source) closer(j, b int) int {
	d := s.l.tokens[j].depth
	for m := j + 1; m < b; m++ {
		if t := s.l.tokens[m]; t.kind == tClose && t.depth == d {
			return m
		}
	}
	return b
}

func clean(mods []string) []string {
	out := mods[:0]
	for _, m := range mods {
		m = strings.TrimSuffix(strings.TrimSpace(m), ".nim")
		if m != "" && !strings.ContainsAny(m, " \t\n\"") {
			out = append(out, m)
		}
	}
	return out
}

// addImport records an import once per file; an include is told apart from an
// import of the same module by its spec.
func (s *source) addImport(kind, module string, line int) {
	spec := module
	if kind == kindInclude {
		spec = "include " + module
	}
	if s.seen[spec] || len(s.imports) >= 4096 {
		return
	}
	s.seen[spec] = true
	s.imports = append(s.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

// defName reads a definition's name at i: an identifier or a `quoted` name, and
// whether an export marker `*` follows it. It returns the index after them.
func (s *source) defName(i, b int) (name string, next int) {
	if i >= b || s.l.tokens[i].kind != tIdent {
		return "", i
	}
	name, next = s.ident(i), i+1
	if next < b && s.l.tokens[next].kind == tOp && strings.HasPrefix(s.l.text(s.l.tokens[next]), "*") {
		if s.l.text(s.l.tokens[next]) == "*" {
			next++
		}
	}
	return name, next
}

// routine reads `proc name*[T](params): R {.pragmas.} = body`.
func (s *source) routine(kind string, a, b int) {
	if name, _ := s.defName(a, b); name != "" {
		s.symbols.Add(name, kind, s.l.tokens[a].line)
	}
}

// typeDef reads one definition of a type section, `Name*[T] {.p.} = object`,
// and names its kind by what follows `=`: an object is a class, a concept an
// interface, an enum an enum, anything else (distinct, tuple, ref T, proc
// types, aliases) a type.
func (s *source) typeDef(a, b int) {
	name, j := s.defName(a, b)
	if name == "" {
		return
	}
	base := s.l.tokens[a].depth
	kind := "type"
	for ; j < b; j++ {
		t := s.l.tokens[j]
		if t.depth == base && t.kind == tOp && s.l.text(t) == "=" {
			kind = typeKind(s, j+1, b)
			break
		}
	}
	s.symbols.Add(name, kind, s.l.tokens[a].line)
}

// typeKind names what the tokens after a type definition's `=` define.
func typeKind(s *source, j, b int) string {
	for m := j; m < b && m <= j+4; m++ {
		switch s.ident(m) {
		case "ref", "ptr", "sink", "lent":
			continue
		case "object":
			return "class"
		case "concept":
			return "interface"
		case "enum":
			return "enum"
		}
		break
	}
	return "type"
}

// names reads the names a const/let/var definition declares: `a*, b: T = v`,
// `a {.threadvar.}: T`, `(x, y) = t`.
func (s *source) names(word string, a, b int) {
	if a >= b {
		return
	}
	kind := word
	if word == "var" || word == "let" {
		kind = "var"
	}
	base := s.l.tokens[a].depth
	for j := a; j < b; j++ {
		t := s.l.tokens[j]
		if t.kind == tOp && t.depth == base {
			txt := s.l.text(t)
			if strings.ContainsAny(txt, ":=") {
				return
			}
			continue
		}
		if t.kind == tOpen && s.l.src[t.start] == '{' {
			j = s.closer(j, b)
			continue
		}
		if t.kind == tIdent && (t.depth == base || t.depth == base+1 && s.l.src[s.l.tokens[a].start] == '(') {
			s.symbols.Add(s.l.text(t), kind, t.line)
		}
	}
}

// assign records `name = "value"`, `name = @["a", "b"]` and `name "value"`
// statements of NimScript (version, srcDir, bin ...), the first of each name.
func (s *source) assign(word string, a, b int) {
	if _, ok := s.assigns[word]; ok {
		return
	}
	j := a + 1
	if j < b && s.l.tokens[j].kind == tOp {
		switch s.l.text(s.l.tokens[j]) {
		case "=", "=@", "@":
			j++
		default:
			return
		}
	}
	if s.l.is(j, tOp, "@") {
		j++
	}
	var values []string
	switch {
	case j < b && s.l.tokens[j].kind == tString:
		values = []string{s.l.text(s.l.tokens[j])}
	case j < b && s.l.tokens[j].kind == tOpen && s.l.src[s.l.tokens[j].start] == '[':
		for m := j + 1; m < b && s.l.tokens[m].depth > s.l.tokens[j].depth; m++ {
			switch s.l.tokens[m].kind {
			case tString:
				values = append(values, s.l.text(s.l.tokens[m]))
			case tComma:
			default:
				return
			}
		}
	default:
		return
	}
	s.assigns[word] = assign{values: values, line: s.l.tokens[a].line}
}

// requirement is one string of a requires or taskRequires statement.
type requirement struct {
	text string
	task string
	line int
}

// requirements finds the requires and taskRequires statements of a .nimble
// file wherever they are - top level, when branches, feature and dev blocks,
// task bodies - and returns their strings: `requires "a >= 1", "b"`,
// `requires("a")`, `requires @["a"]`, a list continued on the next lines.
//
// Implements: REQ-NIM-005
func (s *source) requirements() []requirement {
	l := s.l
	var out []requirement
	for i, t := range l.tokens {
		if t.kind != tIdent {
			continue
		}
		word := l.text(t)
		if word != "requires" && word != "taskRequires" {
			continue
		}
		if !t.first && !l.is(i-1, tOp, ":") {
			continue
		}
		col := t.col
		if !t.first {
			col = s.lineIndent(i)
		}
		task := ""
		for j := i + 1; j < len(l.tokens); j++ {
			u := l.tokens[j]
			if u.first && u.depth <= t.depth && u.col <= col && !continued(l, j) {
				break
			}
			if u.kind == tSemi && u.depth == t.depth {
				break
			}
			if u.kind != tString {
				continue
			}
			if word == "taskRequires" && task == "" {
				task = l.text(u)
				continue
			}
			out = append(out, requirement{text: strings.TrimSpace(l.text(u)), task: task, line: u.line})
			if len(out) >= 4096 {
				return out
			}
		}
	}
	return out
}

// continued reports whether token j continues the previous line's expression:
// that line ended with a comma or an operator.
func continued(l *lexer, j int) bool {
	if j == 0 {
		return false
	}
	p := l.tokens[j-1]
	return p.kind == tComma || p.kind == tOp && l.text(p) != ":"
}

// lineIndent is the column of the first token of token i's line.
func (s *source) lineIndent(i int) int {
	for j := i; j >= 0 && j > i-256; j-- {
		if s.l.tokens[j].first {
			return s.l.tokens[j].col
		}
	}
	return 0
}

// pathSwitch is a search path a NimScript configuration adds.
type pathSwitch struct {
	value string
	line  int
}

// paths finds the search paths a NimScript file (config.nims, x.nims, a
// .nimble) adds: `--path:"x"`, `--path:x`, `switch("path", "x")` (`p` for
// path too), thisDir() and projectDir() read as $projectDir.
//
// Implements: REQ-NIM-004
func (s *source) paths() []pathSwitch {
	l := s.l
	var out []pathSwitch
	for i, t := range l.tokens {
		switch {
		case t.kind == tOp && l.text(t) == "--" && (t.first || l.is(i-1, tOp, ":")):
			key := s.ident(i + 1)
			if (key != "path" && key != "p") || i+2 >= len(l.tokens) || l.tokens[i+2].kind != tOp {
				continue
			}
			// `:` and a bare path's leading `../` or `$` lex as one operator run.
			colon := l.text(l.tokens[i+2])
			rest, ok := strings.CutPrefix(colon, ":")
			if !ok {
				continue
			}
			var b strings.Builder
			b.WriteString(rest)
			for j := i + 3; j < len(l.tokens) && l.tokens[j].line == t.line && l.tokens[j].kind != tSemi; j++ {
				if l.tokens[j].kind == tString {
					if b.Len() == 0 {
						b.WriteString(l.text(l.tokens[j]))
					}
					break
				}
				b.WriteString(l.text(l.tokens[j]))
			}
			if v := b.String(); v != "" {
				out = append(out, pathSwitch{value: v, line: t.line})
			}
		case t.kind == tIdent && l.text(t) == "switch" && l.is(i+1, tOpen, "(") && i+3 < len(l.tokens):
			key := l.tokens[i+2]
			if key.kind != tString || (l.text(key) != "path" && l.text(key) != "p") || !l.is(i+3, tComma, ",") {
				continue
			}
			if v, ok := s.pathExpr(i+4, l.tokens[i+1].depth+1); ok {
				out = append(out, pathSwitch{value: v, line: t.line})
			}
		}
		if len(out) >= 1024 {
			break
		}
	}
	return out
}

// pathExpr evaluates the second argument of switch("path", ...) from token i:
// strings joined by `/` or `&`, thisDir() and projectDir() as $projectDir.
func (s *source) pathExpr(i, depth int) (string, bool) {
	l := s.l
	var b strings.Builder
	for j := i; j < len(l.tokens) && j < i+64; j++ {
		t := l.tokens[j]
		switch {
		case t.kind == tClose && t.depth == depth-1:
			return b.String(), b.Len() > 0
		case t.kind == tString:
			b.WriteString(l.text(t))
		case t.kind == tOp && l.text(t) == "/":
			b.WriteString("/")
		case t.kind == tOp && l.text(t) == "&":
		case t.kind == tIdent && (l.text(t) == "thisDir" || l.text(t) == "projectDir" || l.text(t) == "getCurrentDir") &&
			l.is(j+1, tOpen, "(") && l.is(j+2, tClose, ")"):
			b.WriteString("$projectDir")
			j += 2
		default:
			return "", false
		}
	}
	return "", false
}
