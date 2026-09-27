package perl

import (
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, the first line of RawImport.Name. The lines after it are the
// file's `use lib` directories (for kindModule, kindIsa and kindFile) or a
// manifest entry's version requirement and origin (kindDep).
const (
	kindModule = "mod"  // a module loaded by use, no, require, a parent class, a role
	kindIsa    = "isa"  // a class named without loading it (use parent -norequire)
	kindFile   = "file" // require/do of a file path
	kindDep    = "dep"  // a manifest's requirement
)

// selfMarker starts a path relative to the file itself: "\x01/../lib" is the lib
// directory beside the file's directory, as `"$FindBin::Bin/../lib"` spells it.
const selfMarker = "\x01"

// pragmas are the core pragmas that switch the compiler's behavior: loading one is
// no dependency, so no import is recorded for it (use lib is read for its
// directories).
var pragmas = map[string]bool{
	"strict": true, "warnings": true, "utf8": true, "feature": true, "lib": true, "vars": true,
	"integer": true, "bytes": true, "less": true, "sort": true, "filetest": true, "locale": true,
	"open": true, "re": true, "mro": true, "overloading": true, "builtin": true, "sigtrap": true,
	"diagnostics": true, "subs": true, "vmsish": true, "attributes": true, "charnames": true,
	"deprecate": true, "stable": true, "if": true,
}

// mooseFamily are the object systems that give a package `has`, `with` and
// `extends` (the last two not for Mojo::Base, which only has `has`).
func mooseFamily(m string) bool {
	switch first, _, _ := strings.Cut(m, "::"); first {
	case "Moose", "Moo", "Mouse", "MooseX", "MooX", "MouseX", "Mo", "Moos":
		return true
	}
	return m == "Role::Tiny" || m == "Role::Tiny::With" || m == "Class::Moose" || m == "Test::Class::Moose" || m == "Test::Roo" || m == "Moops"
}

type reader struct {
	tokens  []token
	imports []lang.RawImport
	symbols lang.SymbolSet
	seen    map[string]bool
	pkg     string   // the current package ("" = main)
	frames  []string // the package to restore at each open brace
	pending string   // `package X {` / `class X {`: the package of the next block
	hasAttr bool     // has() declares attributes (Moose family, Mojo::Base)
	roles   bool     // with() and extends() load classes (Moose family)
	classes bool     // class/method/field (feature 'class', Object::Pad)
	libs    []string
}

// readSource reads a Perl file's imports and definitions.
//
// Implements: REQ-PERL-002, REQ-PERL-003
func readSource(src []byte) *lang.Extraction {
	r := &reader{tokens: lex(src), seen: map[string]bool{}}
	r.run()
	return r.extraction()
}

func (r *reader) extraction() *lang.Extraction {
	// use lib takes effect at compile time for the whole file, so every module the
	// file loads is looked up in its directories.
	if len(r.libs) > 0 {
		suffix := "\n" + strings.Join(r.libs, "\n")
		for i, im := range r.imports {
			if im.Name == kindModule || im.Name == kindIsa || im.Name == kindFile {
				r.imports[i].Name += suffix
			}
		}
	}
	sort.SliceStable(r.imports, func(i, j int) bool { return r.imports[i].Line < r.imports[j].Line })
	return &lang.Extraction{Imports: r.imports, Symbols: r.symbols.List()}
}

func (r *reader) add(spec, module, kind string, line int) {
	if module == "" || r.seen[spec] {
		return
	}
	r.seen[spec] = true
	r.imports = append(r.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

func (r *reader) tok(i int) token {
	if i >= 0 && i < len(r.tokens) {
		return r.tokens[i]
	}
	return token{kind: -1}
}

func (r *reader) punct(i int, p string) bool {
	t := r.tok(i)
	return t.kind == tPunct && t.text == p
}

// stmtStart reports whether token i starts a statement.
func (r *reader) stmtStart(i int) bool {
	return i == 0 || r.punct(i-1, ";") || r.punct(i-1, "{") || r.punct(i-1, "}")
}

// maxStatement bounds how far a statement is read: a real one is far shorter, and
// an unclosed bracket must not make every statement run to the end of the file.
const maxStatement = 1024

// end is the index of the token ending the statement starting at i: its ";" at
// bracket depth 0, or the "}" closing the enclosing block (not consumed).
func (r *reader) end(i int) int {
	depth := 0
	limit := min(len(r.tokens), i+maxStatement)
	for ; i < limit; i++ {
		t := r.tokens[i]
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			if depth == 0 {
				return i
			}
			depth--
		case ";":
			if depth == 0 {
				return i
			}
		}
	}
	return limit
}

// moduleName reports whether s is a module name: Foo, Foo::Bar, Foo::Bar::.
func moduleName(s string) bool {
	s = strings.TrimSuffix(s, "::")
	if s == "" {
		return false
	}
	for k, seg := range strings.Split(s, "::") {
		if seg == "" || k == 0 && !identStart(seg[0]) {
			return false
		}
		for k := 0; k < len(seg); k++ {
			if !identByte(seg[k]) {
				return false
			}
		}
	}
	return true
}

// versionWord reports whether a word is a v-string (v5.36, v1).
func versionWord(s string) bool {
	return len(s) > 1 && s[0] == 'v' && s[1] >= '0' && s[1] <= '9'
}

func (r *reader) run() {
	for i := 0; i < len(r.tokens); i++ {
		t := r.tokens[i]
		if t.kind == tPunct {
			switch t.text {
			case "{":
				r.frames = append(r.frames, r.pkg)
				if r.pending != "" {
					r.pkg, r.pending = r.pending, ""
				}
			case "}":
				if n := len(r.frames); n > 0 {
					r.pkg = r.frames[n-1]
					r.frames = r.frames[:n-1]
				}
			case ";":
				r.pending = ""
			}
			continue
		}
		if t.kind != tWord || r.punct(i-1, "->") || r.punct(i+1, "=>") {
			continue
		}
		start := r.stmtStart(i)
		switch t.text {
		case "package":
			if start {
				r.pkgStmt(i, "class")
			}
		case "class", "role":
			if start && r.classes {
				r.pkgStmt(i, "class")
			}
		case "use", "no":
			if start {
				r.use(i)
			}
		case "require":
			r.require(i)
		case "do":
			if s := r.tok(i + 1); s.kind == tStr && !r.punct(i+2, "(") {
				if p, ok := r.pathOf([]token{s}); ok {
					r.add("do "+quoted(s), p, kindFile, t.line)
				}
			}
		case "sub":
			r.sub(i, false)
		case "method":
			if r.classes && r.tok(i+1).kind == tWord {
				r.sub(i, true)
			}
		case "has":
			if start && r.hasAttr {
				r.has(i)
			}
		case "with", "extends":
			if start && r.roles {
				r.inherit(i, t.text)
			}
		case "use_ok", "require_ok", "use_module", "require_module", "load_class", "try_load_class", "load_optional_class":
			j := i + 1
			if r.punct(j, "(") {
				j++
			}
			if s := r.tok(j); s.kind == tStr && moduleName(s.text) {
				m := strings.TrimSuffix(s.text, "::")
				r.add(t.text+" "+m, m, kindModule, t.line)
			}
		case "eval":
			// eval "use Foo; 1" loads an optional module; its imports count, on the
			// eval's line.
			j := i + 1
			if r.punct(j, "(") {
				j++
			}
			if s := r.tok(j); s.kind == tStr && strings.Contains(s.text, "use ") || s.kind == tStr && strings.Contains(s.text, "require ") {
				inner := &reader{tokens: lex([]byte(s.text)), seen: r.seen}
				inner.run()
				for _, im := range inner.imports {
					if im.Name == kindModule {
						im.Line = s.line
						r.imports = append(r.imports, im)
					}
				}
			}
		}
	}
}

func quoted(t token) string {
	if t.interpolate {
		return `"` + t.text + `"`
	}
	return "'" + t.text + "'"
}

// pkgStmt reads `package NAME [VERSION];` or `package NAME [VERSION] {`, and the
// same for Corinna's and Object::Pad's classes and roles with their :isa and :does.
func (r *reader) pkgStmt(i int, kind string) {
	name := r.tok(i + 1)
	if name.kind != tWord || !moduleName(name.text) {
		return
	}
	r.symbols.Add(name.text, kind, r.tokens[i].line)
	end := r.end(i + 1)
	block := false
	for j := i + 2; j <= end && j < len(r.tokens); j++ {
		t := r.tokens[j]
		if t.kind == tPunct && t.text == "{" {
			block = true
			break
		}
		// :isa(Base 1.0) :does(Role, Other)
		if t.kind == tWord && (t.text == "isa" || t.text == "does") && r.punct(j-1, ":") && r.punct(j+1, "(") {
			for k := j + 2; k < min(len(r.tokens), j+maxHeader) && !r.punct(k, ")"); k++ {
				if w := r.tokens[k]; w.kind == tWord && moduleName(w.text) {
					r.add(":"+t.text+"("+w.text+")", w.text, kindModule, w.line)
				}
			}
		}
		if t.kind == tWord && t.text == "extends" && r.classes {
			if w := r.tok(j + 1); w.kind == tWord && moduleName(w.text) {
				r.add("extends "+w.text, w.text, kindModule, w.line)
			}
		}
	}
	if block {
		r.pending = name.text
	} else {
		r.pkg = name.text
	}
}

// owner qualifies a name defined in the current package.
func (r *reader) owner(name string) string {
	if i := strings.LastIndex(name, "::"); i > 0 {
		return name[:i] + "." + name[i+2:]
	}
	if r.pkg == "" || r.pkg == "main" {
		return name
	}
	return r.pkg + "." + name
}

// sub reads `sub NAME` (a forward declaration `sub NAME;` defines nothing) and
// Corinna's `method NAME`. A sub whose parameters or first lines take $self or
// $class is a method.
func (r *reader) sub(i int, method bool) {
	name := r.tok(i + 1)
	name.text = strings.TrimPrefix(name.text, "::") // sub ::f is main's
	if name.kind != tWord || !moduleName(name.text) {
		return
	}
	kind := "function"
	if method {
		kind = "method"
	}
	for j := i + 2; j < min(len(r.tokens), i+maxHeader); j++ {
		t := r.tokens[j]
		if t.kind == tPunct && t.text == ";" {
			return // a forward declaration
		}
		if t.kind == tPunct && t.text == "{" {
			for k := j + 1; k < min(j+12, len(r.tokens)); k++ {
				if v := r.tokens[k]; v.kind == tVar && selfVar(v.text) {
					kind = "method"
				}
			}
			break
		}
		if t.kind == tVar && selfVar(t.text) {
			kind = "method" // a signature
		}
	}
	if !strings.Contains(name.text, "::") && (r.pkg == "" || r.pkg == "main") && kind == "method" && !method {
		kind = "function"
	}
	r.symbols.Add(r.owner(name.text), kind, name.line)
}

// maxHeader bounds how far a sub's header (prototype or signature, attributes) is
// read for its body.
const maxHeader = 256

func selfVar(v string) bool {
	switch v {
	case "$self", "$class", "$this", "$proto", "$invocant", "$c":
		return true
	}
	return false
}

// has reads the attribute names of Moose's, Moo's and Mojo::Base's has: a word or
// string, a qw list, an array of them, '+name' overriding an inherited one.
func (r *reader) has(i int) {
	j := i + 1
	if r.punct(j, "(") {
		j++
	}
	line := r.tokens[i].line
	var names []string
	switch t := r.tok(j); {
	case t.kind == tWord && (r.punct(j+1, "=>") || r.punct(j+1, ",") || r.punct(j+1, ";")):
		names = []string{t.text}
	case t.kind == tStr:
		names = []string{t.text}
	case t.kind == tQW:
		names = t.words
	case t.kind == tPunct && t.text == "[":
		for k := j + 1; k < min(len(r.tokens), j+maxStatement) && !r.punct(k, "]"); k++ {
			switch w := r.tokens[k]; w.kind {
			case tStr, tWord:
				names = append(names, w.text)
			case tQW:
				names = append(names, w.words...)
			}
		}
	}
	for _, n := range names {
		n = strings.TrimPrefix(n, "+")
		if moduleName(n) && !strings.Contains(n, "::") {
			r.symbols.Add(r.owner(n), "property", line)
		}
	}
}

// inherit reads Moose's with and extends: the roles and classes named at the top of
// the statement, not the parameters in braces after them.
func (r *reader) inherit(i int, kw string) {
	end := r.end(i + 1)
	depth := 0
	for j := i + 1; j < end; j++ {
		t := r.tokens[j]
		switch {
		case t.kind == tPunct && (t.text == "{" || t.text == "["):
			depth++
		case t.kind == tPunct && (t.text == "}" || t.text == "]"):
			depth--
		case depth > 0:
		case t.kind == tStr && moduleName(t.text):
			r.add(kw+" "+t.text, t.text, kindModule, t.line)
		case t.kind == tQW:
			for _, w := range t.words {
				if moduleName(w) {
					r.add(kw+" "+w, w, kindModule, t.line)
				}
			}
		}
	}
}

// require reads `require Module`, `require "file.pl"` and `require("file")`;
// a version (`require 5.006`) and a computed argument are not imports.
func (r *reader) require(i int) {
	j := i + 1
	if r.punct(j, "(") {
		j++
	}
	t := r.tok(j)
	line := r.tokens[i].line
	switch {
	case t.kind == tWord && moduleName(t.text) && !versionWord(t.text) && !r.punct(j+1, "(") && !r.punct(j+1, "->"):
		if m := strings.TrimSuffix(t.text, "::"); !pragmas[m] {
			r.add("require "+m, m, kindModule, line)
		}
	case t.kind == tStr:
		if p, ok := r.pathOf([]token{t}); ok {
			r.add("require "+quoted(t), p, kindFile, line)
		}
	}
}

// use reads `use Module VERSION LIST` and `no Module`: the module, and what some
// modules are handed - use lib's directories, use parent's and use base's classes,
// Mojo::Base's parent, use constant's names, use if's module, use aliased's class.
func (r *reader) use(i int) {
	kw := r.tokens[i].text
	line := r.tokens[i].line
	name := r.tok(i + 1)
	if name.kind != tWord || versionWord(name.text) || !moduleName(name.text) {
		return // use 5.010, use v5.36
	}
	m := strings.TrimSuffix(name.text, "::")
	end := r.end(i + 2)
	j := i + 2
	// A version after the module is not one of its arguments.
	if t := r.tok(j); j < end && (t.kind == tNum || t.kind == tWord && versionWord(t.text)) && !r.punct(j+1, "=>") && !r.punct(j+1, ",") {
		j++
	}
	args := r.tokens[min(j, end):end]
	if !pragmas[m] {
		r.add(kw+" "+m, m, kindModule, line)
	}
	if kw == "no" {
		return
	}
	switch {
	case m == "lib" || m == "lib::relative":
		for _, p := range evalList(args) {
			if m == "lib::relative" && !strings.HasPrefix(p, selfMarker) {
				p = selfMarker + "/../" + p
			}
			r.libs = append(r.libs, p)
		}
	case m == "if":
		// use if COND, MODULE => ARGS: the module after the first comma.
		depth := 0
		for k, t := range args {
			if t.kind == tPunct && (t.text == "(" || t.text == "[" || t.text == "{") {
				depth++
			} else if t.kind == tPunct && (t.text == ")" || t.text == "]" || t.text == "}") {
				depth--
			} else if depth == 0 && t.kind == tPunct && (t.text == "," || t.text == "=>") {
				if s := argAt(args, k+1); s != "" && moduleName(s) {
					r.add("use "+s, s, kindModule, line)
				}
				break
			}
		}
	case m == "parent" || m == "base":
		kind := kindModule
		for k, t := range args {
			if t.kind == tWord && t.text == "norequire" && k > 0 && args[k-1].kind == tPunct && args[k-1].text == "-" {
				kind = kindIsa
			}
		}
		for _, c := range stringsOf(args) {
			if moduleName(c) {
				r.add(m+" "+c, c, kind, line)
			}
		}
	case m == "Mojo::Base":
		for k, t := range args {
			if t.kind == tWord && k > 0 && args[k-1].kind == tPunct && args[k-1].text == "-" {
				switch t.text {
				case "base":
					r.hasAttr = true
				case "role":
					r.hasAttr, r.roles = true, true
				}
			}
		}
		if s := stringsOf(args); len(s) > 0 && moduleName(s[0]) {
			r.hasAttr = true
			r.add("parent "+s[0], s[0], kindModule, line)
		}
	case m == "aliased":
		if s := stringsOf(args); len(s) > 0 && moduleName(s[0]) {
			r.add("use "+s[0], s[0], kindModule, line)
		}
	case m == "constant":
		r.constants(args, line)
	case m == "Object::Pad" || m == "Feature::Compat::Class":
		r.classes = true
	case m == "feature" || m == "experimental":
		for _, s := range stringsOf(args) {
			if s == "class" {
				r.classes = true
			}
		}
	case mooseFamily(m):
		r.hasAttr = true
		r.roles = r.roles || m != "Mo"
	}
}

// argAt is the string or word at args[k], else "".
func argAt(args []token, k int) string {
	if k < len(args) && (args[k].kind == tStr || args[k].kind == tWord) {
		return args[k].text
	}
	return ""
}

// stringsOf lists the literal strings of an argument list: quoted strings, qw words
// and barewords before "=>" or ",", not options like -norequire.
func stringsOf(args []token) []string {
	var out []string
	for k, t := range args {
		switch t.kind {
		case tStr:
			if !t.interpolate || !strings.ContainsAny(t.text, "$@") {
				out = append(out, t.text)
			}
		case tQW:
			out = append(out, t.words...)
		case tWord:
			if k > 0 && args[k-1].kind == tPunct && args[k-1].text == "-" {
				continue
			}
			if k+1 < len(args) && args[k+1].kind == tPunct && (args[k+1].text == "=>" || args[k+1].text == ",") || k+1 == len(args) {
				out = append(out, t.text)
			}
		}
	}
	return out
}

// constants reads use constant's names: `NAME => value` or `{ A => 1, B => 2 }`.
func (r *reader) constants(args []token, line int) {
	if len(args) == 0 {
		return
	}
	if args[0].kind == tPunct && args[0].text == "{" {
		depth := 0
		for k, t := range args {
			switch {
			case t.kind == tPunct && (t.text == "{" || t.text == "(" || t.text == "["):
				depth++
			case t.kind == tPunct && (t.text == "}" || t.text == ")" || t.text == "]"):
				depth--
			case depth == 1 && (t.kind == tWord || t.kind == tStr) && k+1 < len(args) && args[k+1].kind == tPunct && args[k+1].text == "=>" &&
				(args[k-1].kind == tPunct && (args[k-1].text == "{" || args[k-1].text == ",")):
				if moduleName(t.text) {
					r.symbols.Add(r.owner(t.text), "const", t.line)
				}
			}
		}
		return
	}
	if t := args[0]; (t.kind == tWord || t.kind == tStr) && moduleName(t.text) && !strings.Contains(t.text, "::") {
		r.symbols.Add(r.owner(t.text), "const", line)
	}
}

// pathOf evaluates a file path given as an expression (see evalPath).
func (r *reader) pathOf(tokens []token) (string, bool) {
	p, ok := evalPath(tokens)
	if !ok || p == "" || strings.ContainsAny(p, "\n") {
		return "", false
	}
	return p, true
}
