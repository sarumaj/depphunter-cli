package haxe

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindImport    = "import" // import a.b.C, a.b.*, a.b.C.field, a.b.C as D
	kindUsing     = "using"  // using a.b.Tools
	kindReference = "ref"    // a qualified name in code: haxe.Json.parse(...)
	kindLibrary   = "lib"    // a haxelib library a manifest declares: name[:version]
	kindLix       = "lix"    // the library a lix haxe_libraries/<name>.hxml pins
	kindCP        = "cp"     // a class path directory
	kindMain      = "main"   // a main class or a root module to compile
	kindHXML      = "hxml"   // another .hxml file an .hxml includes
	kindFile      = "file"   // a resource or an included project file
)

// kindHx is an import every module has, which the resolver expands into the
// import.hx files that apply to it, or none.
const kindHx = "import.hx"

const (
	frameType     = 1 // the body of a class, interface, enum or abstract
	frameFunction = 2 // a function body
	frameOther    = 3 // any other braces: blocks, object literals, structures
)

const (
	maxDepth      = 256 // frames tracked; deeper braces are only counted
	maxConditions = 64  // nested #if snapshots
	maxSegments   = 64  // segments of a dotted path; longer ones are not names
)

type frame struct {
	kind        byte
	name        string
	parenthesis int // the enclosing parenthesis depth, restored when the frame closes
}

// state is what the scanner knows at a point of the file; #if saves it and each
// #elseif/#else starts again from it.
type state struct {
	stack       []frame
	overflow    int
	parenthesis int
	pending     byte   // frameType or frameFunction: the next { opens that body
	pendName    string // the type the next { opens
	angle       int    // < > depth in a pending type's header
}

func (s state) clone() state {
	s.stack = append([]frame(nil), s.stack...)
	return s
}

type condition struct {
	snap  state
	first *state // the state at the end of the first branch
}

// keywords that never start a qualified name.
var notPackage = map[string]bool{
	"this": true, "super": true, "new": true, "null": true, "true": true, "false": true, "untyped": true,
	"cast": true, "return": true, "trace": true, "macro": true, "case": true, "default": true, "in": true,
	"is": true, "throw": true, "var": true, "final": true, "if": true, "else": true, "for": true, "while": true,
	"switch": true, "catch": true, "function": true, "static": true, "inline": true, "public": true,
	"private": true, "override": true, "extern": true, "dynamic": true, "package": true, "import": true,
	"using": true, "class": true, "interface": true, "enum": true, "abstract": true, "typedef": true,
}

func upper(s string) bool { return s != "" && s[0] >= 'A' && s[0] <= 'Z' }

// constant reports whether a name is written in capitals (gl.TEXTURE_2D,
// key.ID): a field of a variable, not a type of a package.
func constant(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'a' && c <= 'z' {
			return false
		}
	}
	return len(s) > 1
}

func lower(s string) bool { return s != "" && ((s[0] >= 'a' && s[0] <= 'z') || s[0] == '_') }

// extractSource reads a Haxe module: its imports and usings, the qualified
// names its code uses (a.b.Type, not imported), and its declarations - types
// by name, their functions as Type.name and module-level functions and
// variables. Every branch of #if counts; braces follow the first branch.
//
// Implements: REQ-HAXE-002, REQ-HAXE-003, REQ-HAXE-010, REQ-HAXE-011
func extractSource(source []byte) *lang.Extraction {
	x := &extractor{extraction: &lang.Extraction{}, seenType: map[string]bool{}, seenModule: map[string]bool{}}
	l := newLexer(source)
	for {
		t, ok := l.next()
		if !ok {
			break
		}
		x.tokens = append(x.tokens, t)
	}
	for x.i = 0; x.i < len(x.tokens); x.i++ {
		switch t := x.tokens[x.i]; t.kind {
		case tDirective:
			x.directive(t.text)
		case tPunctuation:
			x.punctuation(t.text)
		case tIdentifier:
			if x.i == 0 || !x.isP(x.i-1, ".") { // not a field access
				x.identifier(t)
			}
		}
	}
	return x.finish()
}

// extractor is extractSource's walk over a module's tokens: where it is (i), the
// scopes it is in (current, and the #if snapshots), and what it has found.
type extractor struct {
	tokens        []token
	i             int
	current       state
	conditions    []condition
	conditionSkip int

	extraction *lang.Extraction
	symbols    lang.SymbolSet
	seenType   map[string]bool
	seenModule map[string]bool
	references []lang.RawImport
}

func (x *extractor) at(i int) token {
	if i < len(x.tokens) {
		return x.tokens[i]
	}
	return token{}
}

func (x *extractor) isP(i int, s string) bool {
	t := x.at(i)
	return t.kind == tPunctuation && t.text == s
}
func (x *extractor) isI(i int, s string) bool {
	t := x.at(i)
	return t.kind == tIdentifier && t.text == s
}

func (x *extractor) top() *frame {
	if x.current.overflow > 0 || len(x.current.stack) == 0 {
		return nil
	}
	return &x.current.stack[len(x.current.stack)-1]
}

func (x *extractor) moduleLevel() bool {
	return len(x.current.stack) == 0 && x.current.overflow == 0 && x.current.parenthesis == 0
}

func (x *extractor) name(i int) string {
	if t := x.at(i); t.kind == tIdentifier {
		return t.text
	}
	return ""
}

// directive follows #if, #elseif, #else and #end: each branch starts from the
// state before the #if, and the file goes on from the state the first left.
func (x *extractor) directive(text string) {
	switch text {
	case "if":
		if len(x.conditions) < maxConditions {
			x.conditions = append(x.conditions, condition{snap: x.current.clone()})
		} else {
			x.conditionSkip++
		}
	case "elseif", "else":
		if x.conditionSkip == 0 && len(x.conditions) > 0 {
			c := &x.conditions[len(x.conditions)-1]
			if c.first == nil {
				f := x.current.clone()
				c.first = &f
			}
			x.current = c.snap.clone()
		}
	case "end":
		if x.conditionSkip > 0 {
			x.conditionSkip--
		} else if len(x.conditions) > 0 {
			c := x.conditions[len(x.conditions)-1]
			x.conditions = x.conditions[:len(x.conditions)-1]
			if c.first != nil {
				x.current = *c.first
			}
		}
	}
}

// punctuation follows the braces, parentheses and angle brackets that say which
// body the walk is in, and steps over metadata.
func (x *extractor) punctuation(text string) {
	current := &x.current
	switch text {
	case "{":
		f := frame{kind: frameOther, parenthesis: current.parenthesis}
		if current.pending != 0 && current.parenthesis == 0 && current.angle == 0 {
			f.kind, f.name = current.pending, current.pendName
			current.pending, current.pendName = 0, ""
		}
		if len(current.stack) < maxDepth && current.overflow == 0 {
			current.stack = append(current.stack, f)
		} else {
			current.overflow++
		}
		current.parenthesis = 0
	case "}":
		if current.overflow > 0 {
			current.overflow--
		} else if n := len(current.stack); n > 0 {
			current.parenthesis = current.stack[n-1].parenthesis
			current.stack = current.stack[:n-1]
		}
	case "(", "[":
		current.parenthesis++
	case ")", "]":
		if current.parenthesis > 0 {
			current.parenthesis--
		}
	case "<":
		if current.pending == frameType {
			current.angle++
		}
	case ">":
		if current.pending == frameType && current.angle > 0 {
			current.angle--
		}
	case ";":
		if current.parenthesis == 0 && current.pending == frameFunction {
			current.pending = 0 // a function without a body: interfaces, externs
		}
	case "@":
		// Metadata: @name, @:name, @:a.b - a name that is no keyword.
		j := x.i + 1
		if x.isP(j, ":") {
			j++
		}
		if x.at(j).kind == tIdentifier && x.at(j).position == x.at(j-1).end {
			for x.isP(j+1, ".") && x.at(j+2).kind == tIdentifier {
				j += 2
			}
			x.i = j
		}
	}
}

// identifier reads what a name that is not a field access starts: a package
// line, an import, a declaration, or a qualified name.
func (x *extractor) identifier(t token) {
	switch t.text {
	case "package":
		if x.moduleLevel() {
			for x.i+1 < len(x.tokens) && !x.isP(x.i+1, ";") && (x.at(x.i+1).kind == tIdentifier || x.isP(x.i+1, ".")) {
				x.i++
			}
		}
		return
	case "import", "using":
		if x.moduleLevel() {
			x.importLine(t)
		}
		return
	case "class", "interface", "enum", "abstract", "typedef":
		if x.moduleLevel() {
			x.declaration(t)
			return
		}
	case "function":
		x.function(t)
		return
	case "var", "final":
		if n := x.name(x.i + 1); x.moduleLevel() && n != "" && !notPackage[n] {
			x.symbols.Add(n, "var", t.line)
			x.i++
		}
		return
	}
	x.qualifiedName(t)
}

// importLine reads import a.b.C, a.b.*, a.b.C as D and using a.b.Tools.
func (x *extractor) importLine(t token) {
	var segments []string
	j := x.i + 1
	for {
		if x.at(j).kind == tIdentifier {
			segments = append(segments, x.at(j).text)
			j++
		} else if x.isP(j, "*") && len(segments) > 0 {
			segments = append(segments, "*")
			j++
			break
		} else {
			break
		}
		if !x.isP(j, ".") {
			break
		}
		j++
	}
	if len(segments) == 0 || len(segments) > maxSegments {
		x.i = j - 1
		return
	}
	if x.isI(j, "as") || x.isI(j, "in") {
		j += 2
	}
	x.i = j - 1
	if x.isP(j, ";") {
		x.i = j
	}
	p := strings.Join(segments, ".")
	kind := kindImport
	if t.text == "using" {
		kind = kindUsing
	}
	x.extraction.Imports = append(x.extraction.Imports, lang.RawImport{Spec: p, Module: p, Name: kind, Line: t.line})
	x.seenModule[p] = true
}

// declaration reads a module-level class, interface, enum, abstract or typedef,
// and has the next { open its body.
func (x *extractor) declaration(t token) {
	kind := t.text
	j := x.i + 1
	switch {
	case kind == "enum" && x.isI(j, "abstract"):
		j++ // enum abstract Color(Int)
	case kind == "abstract" && (x.isI(j, "class") || x.isI(j, "interface")):
		return // abstract class: the class keyword follows
	}
	n := x.name(j)
	if n == "" || notPackage[n] {
		return
	}
	if !x.seenType[n] {
		x.seenType[n] = true
		x.symbols.Add(n, kind, t.line)
	}
	x.i = j
	if kind != "typedef" {
		x.current.pending, x.current.pendName, x.current.angle = frameType, n, 0
	}
}

// function reads a named function: module-level, or a method of the type whose
// body the walk is in.
func (x *extractor) function(t token) {
	n := x.name(x.i + 1)
	if n == "" {
		return // an anonymous function
	}
	if x.moduleLevel() {
		x.symbols.Add(n, "func", t.line)
	} else if f := x.top(); f != nil && f.kind == frameType && x.current.parenthesis == 0 {
		x.symbols.Add(f.name+"."+n, "method", t.line)
	} else {
		return
	}
	x.current.pending, x.current.pendName = frameFunction, ""
	x.i++
}

// qualifiedName reads a.b.Type in code: lower-case package segments up to a type.
func (x *extractor) qualifiedName(t token) {
	if !lower(t.text) || notPackage[t.text] || !x.isP(x.i+1, ".") {
		return
	}
	segments := []string{t.text}
	j := x.i
	for x.isP(j+1, ".") && x.at(j+2).kind == tIdentifier {
		j += 2
		segments = append(segments, x.at(j).text)
		if upper(x.at(j).text) {
			break
		}
	}
	n := len(segments)
	if !upper(segments[n-1]) || n > maxSegments || n == 2 && constant(segments[1]) {
		x.i = j
		return
	}
	if x.isP(j+1, ".") && upper(x.at(j+2).text) {
		segments = append(segments, x.at(j+2).text) // a sub-type: haxe.macro.Expr.ExprDef
		j += 2
	}
	x.i = j
	p := strings.Join(segments, ".")
	if !x.seenModule[p] {
		x.seenModule[p] = true
		x.references = append(x.references, lang.RawImport{Spec: p, Module: p, Name: kindReference, Line: t.line})
	}
}

// finish adds the qualified names of modules the file does not import, and the
// import every module has.
func (x *extractor) finish() *lang.Extraction {
	if len(x.references) > 0 {
		// A qualified name of a module the file imports adds nothing.
		exact, prefixes := map[string]bool{}, map[string]bool{}
		for _, rawImport := range x.extraction.Imports {
			m := strings.TrimSuffix(rawImport.Module, ".*")
			exact[m] = true
			for k := strings.LastIndexByte(m, '.'); k > 0; k = strings.LastIndexByte(m[:k], '.') {
				prefixes[m[:k]] = true
			}
		}
		for _, r := range x.references {
			if !imported(exact, prefixes, r.Module) {
				x.extraction.Imports = append(x.extraction.Imports, r)
			}
		}
	}
	x.extraction.Imports = append(x.extraction.Imports, lang.RawImport{Spec: "import.hx", Name: kindHx, Line: 1})
	x.extraction.Symbols = x.symbols.List()
	return x.extraction
}

// imported reports whether a qualified name is one a module imports: the
// import of its module or package, or of a type in it.
func imported(exact, prefixes map[string]bool, p string) bool {
	if exact[p] || prefixes[p] {
		return true
	}
	for k := strings.LastIndexByte(p, '.'); k > 0; k = strings.LastIndexByte(p[:k], '.') {
		if exact[p[:k]] {
			return true
		}
	}
	return false
}

// readPackage is the package a module declares (package a.b;), "" for the
// top-level package. Only the first token and what follows are read.
func readPackage(source []byte) string {
	l := newLexer(source)
	t, ok := l.next()
	if !ok || t.kind != tIdentifier || t.text != "package" {
		return ""
	}
	var segments []string
	for {
		t, ok = l.next()
		if !ok || t.kind != tIdentifier {
			break
		}
		segments = append(segments, t.text)
		if t, ok = l.next(); !ok || t.kind != tPunctuation || t.text != "." {
			break
		}
	}
	return strings.Join(segments, ".")
}
