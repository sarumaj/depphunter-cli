package fsharp

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, the first line of RawImport.Name. The lines after it are the names
// the import is looked up relative to (contexts), or the script's #I directories.
const (
	kindOpen      = "open"  // open X.Y, open type X.Y, module M = X.Y
	kindReference = "ref"   // a qualified name in code: Cart.add, Shop.Cart.total
	kindLoad      = "load"  // #load "x.fsx"
	kindNuGet     = "nuget" // #r "nuget: X, 1.2.3" (second line: the version)
	kindDLL       = "dll"   // #r "x.dll", #r "System.Xml"
)

// declaration is a name a source file declares for other files: a namespace, a module or a
// type, with its full name.
type declaration struct {
	name string
	kind byte // 'n' namespace, 'm' module, 't' type
}

// info is what one F# source file says.
type info struct {
	imports      []lang.RawImport
	symbols      []lang.Symbol
	declarations []declaration
	header       bool // the file starts with a namespace or module declaration
}

// frame is a declaration scope: the file's namespace or module, a nested module or
// a type body. A line indented no further than a nested frame's header line ends it.
type frame struct {
	kind     byte   // 'n' namespace, 'm' module, 't' type
	full     string // full name (namespace-qualified)
	relative string // symbol prefix within the file: "Cart." for a nested module Cart
	column   int    // column of the header line (-1 for the file's own scope)
	body     int    // column of the body's declarations (-1 until the first body line)
	// pubs are the names the scope's declarations are known by: its full name, and
	// for an [<AutoOpen>] module also the names its parent's are known by.
	pubs []string
}

type scanner struct {
	tokens         []token
	stack          []frame
	opens          []string
	ctx            string // contexts, cached until stack or opens change
	dirty          bool
	lastKeyword    string // let or type, for `and`
	seen           map[string]bool
	symbols        lang.SymbolSet
	references     map[string]bool
	nuget          []string // #r "nuget: X" ids of a script, as extra contexts for opens
	idirs          []string // #I directories
	out            *info
	declarationSet map[string]bool
	// autoOpen says an [<AutoOpen>] attribute waits for its declaration.
	autoOpen bool
}

// maxOpens bounds the opens contexts list; maxDepth the nested scopes.
const (
	maxOpens = 32
	maxDepth = 64
)

// keywords are never names.
var keywords = map[string]bool{
	"abstract": true, "and": true, "as": true, "assert": true, "base": true, "begin": true, "class": true,
	"default": true, "delegate": true, "do": true, "done": true, "downcast": true, "downto": true, "elif": true,
	"else": true, "end": true, "exception": true, "extern": true, "false": true, "finally": true, "fixed": true,
	"for": true, "fun": true, "function": true, "global": true, "if": true, "in": true, "inherit": true,
	"inline": true, "interface": true, "internal": true, "lazy": true, "let": true, "match": true, "member": true,
	"module": true, "mutable": true, "namespace": true, "new": true, "not": true, "null": true, "of": true,
	"open": true, "or": true, "override": true, "private": true, "public": true, "rec": true, "return": true,
	"select": true, "static": true, "struct": true, "then": true, "to": true, "true": true, "try": true,
	"type": true, "upcast": true, "use": true, "val": true, "void": true, "when": true, "while": true,
	"with": true, "yield": true, "const": true,
}

// stdModules are FSharp.Core and .NET names code qualifies all the time; a
// reference through one of them alone is not looked up (it would be dropped anyway).
var stdModules = map[string]bool{
	"List": true, "Seq": true, "Array": true, "Array2D": true, "Array3D": true, "Map": true, "Set": true,
	"Option": true, "ValueOption": true, "Result": true, "String": true, "Async": true, "Task": true,
	"Lazy": true, "Printf": true, "Operators": true, "Checked": true, "Char": true, "Math": true,
	"Console": true, "Environment": true, "DateTime": true, "DateTimeOffset": true, "TimeSpan": true,
	"Guid": true, "Int32": true, "Int64": true, "Double": true, "Decimal": true, "Convert": true,
	"Path": true, "File": true, "Directory": true, "Regex": true, "Encoding": true, "Unchecked": true,
	"LanguagePrimitives": true, "Event": true, "Observable": true, "MailboxProcessor": true,
	"Nullable": true, "StringBuilder": true, "Uri": true, "Byte": true, "Boolean": true, "Single": true,
	"Enum": true, "Activator": true, "Thread": true, "Interlocked": true, "Stopwatch": true, "Debug": true,
	"Trace": true, "Assembly": true, "BitConverter": true, "Buffer": true, "GC": true, "Type": true,
	"Microsoft": true, "System": true, "FSharp": true, "Unit": true, "ExtraTopLevelOperators": true,
}

// scanSource reads an F# source file (.fs, .fsi, .fsx): its namespace and module
// headers, nested modules, top-level let/type/exception/val declarations, type
// members, opens, module abbreviations, qualified names in code and script
// directives. Declarations are found by the offside rule without a parser: a line
// starting in the column of its scope's first declaration starts a declaration,
// deeper lines continue one, and a line no deeper than a nested module's or type's
// header ends that scope.
//
// Implements: REQ-FSHARP-002, REQ-FSHARP-003, REQ-FSHARP-011, REQ-FSHARP-012
func scanSource(source string) *info {
	s := &scanner{tokens: lex(source), seen: map[string]bool{}, references: map[string]bool{}, out: &info{}, declarationSet: map[string]bool{}, dirty: true}
	s.stack = []frame{{kind: 'm', column: -1, body: -1}}
	for i := 0; i < len(s.tokens); i++ {
		t := s.tokens[i]
		if t.kind == tDirectory {
			s.directive(t)
			continue
		}
		if t.first {
			if next := s.line(i); next > i {
				i = next - 1
				continue
			}
		}
		if t.kind != tIdentifier {
			continue
		}
		if t.text == "open" && !s.afterDot(i) {
			i = s.open(i) - 1
			continue
		}
		if next := s.reference(i); next > i {
			i = next - 1
		}
	}
	s.out.symbols = s.symbols.List()
	return s.out
}

func (s *scanner) token(i int) token {
	if i >= 0 && i < len(s.tokens) {
		return s.tokens[i]
	}
	return token{}
}

func (s *scanner) afterDot(i int) bool {
	p := s.token(i - 1)
	return p.kind == tPunctuation && p.text == "." && s.tokens[i].glued
}

func (s *scanner) top() *frame { return &s.stack[len(s.stack)-1] }

func (s *scanner) symbol(name, kind string, line int) {
	if name == "" || s.seen[name] {
		return
	}
	s.seen[name] = true
	s.symbols.Add(name, kind, line)
}

func (s *scanner) declare(name string, kind byte) {
	if name == "" {
		return
	}
	key := string(kind) + name
	if s.declarationSet[key] {
		return
	}
	s.declarationSet[key] = true
	s.out.declarations = append(s.out.declarations, declaration{name: name, kind: kind})
}

// publish declares name in scope f under every name f is known by.
func (s *scanner) publish(f *frame, name string, kind byte) {
	if len(f.pubs) == 0 {
		s.declare(join(f.full, name), kind)
	}
	for _, p := range f.pubs {
		s.declare(join(p, name), kind)
	}
}

func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

// dotted reads a dotted name (A.B.C) starting at i and returns it with the index
// after it.
func (s *scanner) dotted(i int) (string, int) {
	var parts []string
	for i < len(s.tokens) && s.tokens[i].kind == tIdentifier {
		parts = append(parts, s.tokens[i].text)
		if s.token(i+1).text == "." && s.token(i+1).kind == tPunctuation && s.token(i+2).kind == tIdentifier && s.token(i+2).line == s.tokens[i].line {
			i += 2
			continue
		}
		i++
		break
	}
	return strings.Join(parts, "."), i
}

// skipAttributes moves past [< ... >] attribute groups.
func (s *scanner) skipAttributes(i int) int {
	if i >= len(s.tokens) {
		return i
	}
	// Only groups starting on the line of the first: the attribute lines before a
	// declaration are skipped one line at a time by the caller.
	line := s.tokens[i].line
	for i < len(s.tokens) && s.tokens[i].text == "[<" && s.tokens[i].kind == tPunctuation && s.tokens[i].line == line {
		j := i + 1
		for ; j < len(s.tokens) && j-i < 400 && !(s.tokens[j].text == ">]" && s.tokens[j].kind == tPunctuation); j++ {
		}
		if j >= len(s.tokens) || j-i >= 400 {
			return i + 1
		}
		i = j + 1
	}
	return i
}

func (s *scanner) skipWords(i int, words ...string) int {
	for i < len(s.tokens) && s.tokens[i].kind == tIdentifier {
		ok := false
		for _, w := range words {
			if s.tokens[i].text == w {
				ok = true
			}
		}
		if !ok {
			break
		}
		i++
	}
	return i
}

// line handles a token that starts a line: it ends the scopes the line is not
// inside of and reads the declaration the line starts. It returns the index to
// continue from, or i when the line's tokens are to be read as code.
func (s *scanner) line(i int) int {
	t := s.tokens[i]
	for len(s.stack) > 1 && t.column <= s.top().column {
		s.stack = s.stack[:len(s.stack)-1]
		s.dirty = true
	}
	f := s.top()
	j := s.skipAttributes(i)
	for k := i; k < j; k++ {
		if s.tokens[k].kind == tIdentifier && (s.tokens[k].text == "AutoOpen" || s.tokens[k].text == "AutoOpenAttribute") {
			s.autoOpen = true
		}
	}
	if j >= len(s.tokens) || s.tokens[j].line != t.line && j > i {
		return i // an attribute line: the declaration it decorates starts the next line
	}
	autoOpen := s.autoOpen
	s.autoOpen = false
	keyword := s.tokens[j]
	if keyword.kind != tIdentifier {
		if f.body < 0 {
			f.body = t.column
		}
		return i
	}
	switch keyword.text {
	case "namespace":
		name, next := s.dotted(s.skipWords(j+1, "rec", "global"))
		s.stack = []frame{{kind: 'n', full: name, column: -1, body: -1, pubs: []string{name}}}
		s.dirty = true
		s.out.header = true
		s.declare(name, 'n')
		return next
	case "module":
		if len(s.stack) == 1 && f.full == "" && !s.nestedModule(j) {
			name, next := s.dotted(s.skipWords(s.skipAttributes(j+1), "private", "internal", "public", "rec"))
			if f.kind == 'n' { // a module header under a namespace (invalid, but read)
				name = join(f.full, name)
			}
			pubs := []string{name}
			if d := strings.LastIndexByte(name, '.'); d > 0 && autoOpen {
				pubs = append(pubs, name[:d])
			}
			s.stack = []frame{{kind: 'm', full: name, column: -1, body: -1, pubs: pubs}}
			s.dirty = true
			s.out.header = true
			s.declare(name, 'm')
			return next
		}
	}
	if f.body < 0 {
		f.body = t.column
	}
	if f.kind == 't' {
		if t.column > f.column {
			s.member(j, f)
		}
		return i
	}
	if t.column != f.body {
		return i
	}
	switch keyword.text {
	case "let", "val", "extern":
		s.lastKeyword = "let"
		s.let(j+1, f, keyword.text)
	case "and":
		if s.lastKeyword == "type" {
			s.typeDefinition(j+1, f, t.column)
		} else {
			s.let(j+1, f, "let")
		}
	case "type":
		s.lastKeyword = "type"
		s.typeDefinition(j+1, f, t.column)
	case "exception":
		k := s.skipWords(j+1, "private", "internal", "public")
		if n := s.token(k); n.kind == tIdentifier && !keywords[n.text] {
			s.symbol(f.relative+n.text, "exception", keyword.line)
			s.publish(f, n.text, 't')
		}
	case "module":
		return s.module(j, f, t.column, autoOpen)
	}
	return i
}

// nestedModule reports whether the module keyword at j starts `module X =` (a
// nested module or an abbreviation) rather than a file's `module X.Y` header.
func (s *scanner) nestedModule(j int) bool {
	line := s.tokens[j].line
	for k := j + 1; k < len(s.tokens) && s.tokens[k].line == line; k++ {
		if s.tokens[k].kind == tPunctuation && s.tokens[k].text == "=" {
			return true
		}
	}
	return false
}

// module reads a nested module (`module Cart =`, whose body is the deeper lines
// after it) or a module abbreviation (`module L = Shop.Logging`).
func (s *scanner) module(j int, f *frame, column int, autoOpen bool) int {
	k := s.skipWords(s.skipAttributes(j+1), "private", "internal", "public", "rec")
	n := s.token(k)
	if n.kind != tIdentifier || keywords[n.text] {
		return j
	}
	equals := s.token(k + 1)
	if equals.kind != tPunctuation || equals.text != "=" {
		return j
	}
	if after := s.token(k + 2); after.kind == tIdentifier && after.line == equals.line && upper(after.text) && !keywords[after.text] {
		target, next := s.dotted(k + 2)
		s.out.imports = append(s.out.imports, lang.RawImport{
			Spec: "module " + n.text + " = " + target, Module: target, Name: kindOpen + s.contexts(), Line: after.line,
		})
		return next
	}
	full := join(f.full, n.text)
	s.symbol(f.relative+n.text, "module", n.line)
	s.publish(f, n.text, 'm')
	pubs := []string{full}
	if autoOpen {
		pubs = append(pubs, f.pubs...)
	}
	if len(s.stack) < maxDepth {
		s.stack = append(s.stack, frame{kind: 'm', full: full, relative: f.relative + n.text + ".", column: column, body: -1, pubs: pubs})
	}
	s.dirty = true
	return k + 2
}

// let reads the name a let (val, extern) declaration binds: an identifier, an
// active pattern (|A|B|) or an operator (+). Patterns binding several names are
// skipped.
func (s *scanner) let(k int, f *frame, keyword string) {
	k = s.skipWords(s.skipAttributes(k), "rec", "inline", "private", "internal", "public", "mutable", "static")
	n := s.token(k)
	name, next := "", k+1
	switch {
	case n.kind == tIdentifier && !keywords[n.text]:
		name = n.text
	case n.kind == tPunctuation && n.text == "(":
		var b strings.Builder
		b.WriteString("(")
		m := k + 1
		// An operator (+) or an active pattern (|Big|Small|), whose case names
		// are identifiers between bars.
		pattern := s.token(m).text == "|"
		for ; m < len(s.tokens) && m-k < 32 && s.tokens[m].text != ")" && (s.tokens[m].kind == tPunctuation || pattern && s.tokens[m].kind == tIdentifier); m++ {
			b.WriteString(s.tokens[m].text)
		}
		if s.token(m).text != ")" || m == k+1 {
			return
		}
		b.WriteString(")")
		name, next = b.String(), m+1
	default:
		return
	}
	if keyword == "extern" { // extern int f(...)
		for m := k; m < len(s.tokens) && m-k < 16 && s.tokens[m].line == n.line; m++ {
			if s.tokens[m].text == "(" && s.tokens[m-1].kind == tIdentifier {
				s.symbol(f.relative+s.tokens[m-1].text, "func", n.line)
				return
			}
		}
		return
	}
	kind := "func"
	after := s.token(next)
	if after.kind == tTyVariable || after.text == "<" { // let x<'T> = ...
		for m := next; m < len(s.tokens) && m-next < 32; m++ {
			if s.tokens[m].text == ">" {
				after = s.token(m + 1)
				break
			}
		}
	}
	if keyword == "val" || after.kind == tPunctuation && (after.text == "=" || after.text == ":") {
		kind = "value"
		if keyword == "val" {
			kind = "func"
		}
	}
	if name == "_" {
		return
	}
	s.symbol(f.relative+name, kind, n.line)
}

// typeDefinition reads a type definition's name and kind and opens its body scope.
func (s *scanner) typeDefinition(k int, f *frame, column int) {
	k = s.skipWords(s.skipAttributes(k), "private", "internal", "public", "rec")
	k = s.skipAttributes(k)
	n := s.token(k)
	if n.kind != tIdentifier || keywords[n.text] {
		return
	}
	kind := "type"
	m := k + 1
	if s.token(m).text == "<" { // generic parameters
		for depth := 0; m < len(s.tokens) && m-k < 64; m++ {
			if s.tokens[m].text == "<" {
				depth++
			} else if s.tokens[m].text == ">" {
				depth--
				if depth == 0 {
					m++
					break
				}
			}
		}
	}
	if s.token(m).text == "(" && s.token(m).line == n.line {
		kind = "class" // a primary constructor
	}
	for ; m < len(s.tokens) && m-k < 128; m++ {
		if s.tokens[m].kind == tPunctuation && s.tokens[m].text == "=" {
			switch after := s.token(s.skipAttributes(m + 1)); after.text {
			case "interface":
				kind = "interface"
			case "class", "struct":
				kind = "class"
			case "delegate":
				kind = "delegate"
			case "abstract":
				kind = "interface"
			case "member", "new", "inherit", "val":
				kind = "class"
			}
			break
		}
		if s.tokens[m].first && s.tokens[m].column <= column {
			break
		}
	}
	s.symbol(f.relative+n.text, kind, n.line)
	s.publish(f, n.text, 't')
	if len(s.stack) < maxDepth {
		s.stack = append(s.stack, frame{kind: 't', full: join(f.full, n.text), relative: f.relative + n.text + ".", column: column, body: -1, pubs: []string{join(f.full, n.text)}})
	}
	s.dirty = true
}

// member reads a member of a type body: member x.Name, static member Name,
// abstract Name, override/default x.Name, member val Name.
func (s *scanner) member(j int, f *frame) {
	switch s.tokens[j].text {
	case "member", "static", "abstract", "override", "default":
	default:
		return
	}
	k := s.skipWords(j, "static", "abstract", "override", "default", "member", "private", "internal", "public", "inline", "val", "mutable")
	n := s.token(k)
	if n.kind == tPunctuation && n.text == "(" { // static member (+)
		var b strings.Builder
		b.WriteString("(")
		m := k + 1
		for ; m < len(s.tokens) && m-k < 16 && s.tokens[m].kind == tPunctuation && s.tokens[m].text != ")"; m++ {
			b.WriteString(s.tokens[m].text)
		}
		if s.token(m).text == ")" && m > k+1 {
			s.symbol(f.relative+b.String()+")", "method", n.line)
		}
		return
	}
	if n.kind != tIdentifier || keywords[n.text] && n.text != "new" {
		return
	}
	name := n.text
	if d := s.token(k + 1); d.text == "." && d.kind == tPunctuation && s.token(k+2).kind == tIdentifier {
		name = s.token(k + 2).text // the self identifier: this.Name
	}
	if name == "new" || keywords[name] {
		return
	}
	s.symbol(f.relative+name, "method", n.line)
}

// open reads `open X.Y` and `open type X.Y`.
func (s *scanner) open(i int) int {
	k := i + 1
	spec := "open "
	if s.token(k).text == "type" {
		k++
		spec = "open type "
	}
	name, next := s.dotted(k)
	if name == "" {
		return i + 1
	}
	s.out.imports = append(s.out.imports, lang.RawImport{Spec: spec + name, Module: name, Name: kindOpen + s.contexts(), Line: s.tokens[i].line})
	s.opens = append(s.opens, name)
	s.dirty = true
	return next
}

// reference records a qualified name in code (Cart.add, Shop.Domain.Cart.empty): the run
// of capitalized segments before the member, which names a module or a type. The
// resolver looks it up among the project's declarations; the first mention of each
// name in a file is kept.
func (s *scanner) reference(i int) int {
	t := s.tokens[i]
	if t.text == "" || !upper(t.text) || keywords[t.text] || s.afterDot(i) {
		return i
	}
	d := s.token(i + 1)
	if d.kind != tPunctuation || d.text != "." || !d.glued || !s.token(i+2).glued || s.token(i+2).kind != tIdentifier {
		return i
	}
	parts := []string{t.text}
	k := i
	for s.token(k+1).text == "." && s.token(k+1).kind == tPunctuation && s.token(k+1).glued && s.token(k+2).kind == tIdentifier && s.token(k+2).glued {
		parts = append(parts, s.tokens[k+2].text)
		k += 2
	}
	n := 0
	for n < len(parts) && upper(parts[n]) {
		n++
	}
	path := strings.Join(parts[:n], ".")
	if n == 1 && stdModules[path] || s.references[path] {
		return k + 1
	}
	s.references[path] = true
	s.out.imports = append(s.out.imports, lang.RawImport{Spec: path, Module: path, Name: kindReference + s.contexts(), Line: t.line})
	return k + 1
}

func upper(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

// contexts lists, one per line after a leading newline, the names a partial name
// may be qualified by at this point: each enclosing namespace or module and its
// parents (innermost first), the names opened so far (latest first), and those
// opened names qualified by the enclosing namespaces.
func (s *scanner) contexts() string {
	if !s.dirty {
		return s.ctx
	}
	var list []string
	have := map[string]bool{}
	add := func(n string) {
		if n != "" && !have[n] && len(list) < 64 {
			have[n] = true
			list = append(list, n)
		}
	}
	var enclosing []string
	for k := len(s.stack) - 1; k >= max(0, len(s.stack)-8); k-- {
		for n := s.stack[k].full; n != ""; {
			enclosing = append(enclosing, n)
			add(n)
			if d := strings.LastIndexByte(n, '.'); d >= 0 {
				n = n[:d]
			} else {
				n = ""
			}
		}
	}
	// The latest opens only: a file opening hundreds of names is not code.
	recent := s.opens[max(0, len(s.opens)-maxOpens):]
	for k := len(recent) - 1; k >= 0; k-- {
		add(recent[k])
	}
	for k := len(recent) - 1; k >= 0; k-- {
		for _, e := range enclosing[:min(len(enclosing), 8)] {
			add(e + "." + recent[k])
		}
	}
	for _, p := range s.nuget {
		add("nuget:" + p) // id,version of a script's #r "nuget: ..."
	}
	var b strings.Builder
	for _, n := range list {
		b.WriteString("\n")
		b.WriteString(n)
	}
	s.ctx, s.dirty = b.String(), false
	return s.ctx
}

// directive turns #load, #r and #I into imports (#I directories are carried by the
// #load and #r imports after them).
func (s *scanner) directive(t token) {
	parts := strings.Split(t.text, "\x00")
	arguments := parts[1:]
	switch parts[0] {
	case "I", "include":
		s.idirs = append(s.idirs, arguments...)
		return
	case "load":
		for _, a := range arguments {
			s.out.imports = append(s.out.imports, lang.RawImport{Spec: `#load "` + a + `"`, Module: a, Name: kindLoad + directories(s.idirs), Line: t.line})
		}
		return
	}
	for _, a := range arguments {
		spec := `#r "` + a + `"`
		if rest, ok := cutPrefixFold(strings.TrimSpace(a), "nuget:"); ok {
			id, version, _ := strings.Cut(rest, ",")
			id, version = strings.TrimSpace(id), strings.TrimSpace(version)
			if id == "" {
				continue
			}
			s.nuget = append(s.nuget, id+","+version)
			s.dirty = true
			s.out.imports = append(s.out.imports, lang.RawImport{Spec: spec, Module: id, Name: kindNuGet + "\n" + version, Line: t.line})
			continue
		}
		s.out.imports = append(s.out.imports, lang.RawImport{Spec: spec, Module: a, Name: kindDLL + directories(s.idirs), Line: t.line})
	}
}

func directories(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return "\n" + strings.Join(list, "\n")
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return "", false
}
