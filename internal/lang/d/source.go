package d

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindImport     = ""         // import a.b;
	kindString     = "string"   // import("file"): a file under the string import paths
	kindDependency = "dep"      // a dependency of dub.json or dub.sdl
	kindSelected   = "selected" // an entry of dub.selections.json
	kindSubPath    = "subpkg"   // a sub-package of dub.json or dub.sdl kept in a directory
)

// frame kinds of the extraction's block stack.
const (
	fAgg        = iota // class, struct, interface, union, template: declarations, owned
	fTrans             // version, static if, attribute and else blocks: declarations, not owned
	fBody              // function bodies, unittest, enum members: only imports are read
	fExpression        // braces inside an expression: the statement goes on after them
)

type frame struct {
	kind   int
	name   string
	start  int // fExpression: the statement it interrupted
	depth  int
	equals bool
}

// maxFrames bounds the block stack; deeper braces are only counted.
const maxFrames = 256

// Implements: REQ-DLANG-002, REQ-DLANG-003
func extractSource(source []byte) *lang.Extraction {
	x := &extractor{tokens: lex(source), seen: map[string]bool{}}
	x.run()
	extraction := &lang.Extraction{Imports: x.imports, Symbols: x.symbols.List()}
	if recipe, offset := singleFile(source); recipe != nil {
		dependencies := extractRecipe(recipe).Imports
		for i := range dependencies {
			dependencies[i].Line += offset
		}
		extraction.Imports = append(dependencies, extraction.Imports...)
	}
	return extraction
}

type extractor struct {
	tokens   []token
	stack    []frame
	overflow int
	imports  []lang.RawImport
	seen     map[string]bool
	symbols  lang.SymbolSet
	// the statement being read at declaration level
	start  int
	depth  int  // ( and [ nesting since start
	equals bool // an = at depth 0 since start
}

func (x *extractor) top() *frame {
	if len(x.stack) == 0 {
		return nil
	}
	return &x.stack[len(x.stack)-1]
}

// declarationLevel reports whether declarations are read where the extractor is: at the
// top level and in aggregate, attribute and conditional blocks.
func (x *extractor) declarationLevel() bool {
	if x.overflow > 0 {
		return false
	}
	f := x.top()
	return f == nil || f.kind == fAgg || f.kind == fTrans
}

// owner is the qualified name of the enclosing aggregates.
func (x *extractor) owner() string {
	var parts []string
	for _, f := range x.stack {
		if f.kind == fAgg && f.name != "" {
			parts = append(parts, f.name)
		}
	}
	return strings.Join(parts, ".")
}

func (x *extractor) topLevel() bool {
	for _, f := range x.stack {
		if f.kind != fTrans {
			return false
		}
	}
	return x.overflow == 0
}

func (x *extractor) push(f frame) {
	if len(x.stack) >= maxFrames {
		x.overflow++
		return
	}
	x.stack = append(x.stack, f)
}

func (x *extractor) reset(i int) { x.start, x.depth, x.equals = i, 0, false }

func (x *extractor) run() {
	tokens := x.tokens
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.kind == kIdentifier && t.text == "import" {
			if i+1 < len(tokens) && tokens[i+1].text == "(" && tokens[i+1].kind == kPunctuation {
				x.stringImport(i)
				continue
			}
			end := x.importDeclaration(i)
			if x.declarationLevel() {
				x.reset(end + 1)
			}
			i = end
			continue
		}
		if t.kind != kPunctuation {
			continue
		}
		declaration := x.declarationLevel()
		switch t.text {
		case "(", "[":
			x.depth++
		case ")", "]":
			if x.depth > 0 {
				x.depth--
			}
		case "=":
			if x.depth == 0 {
				x.equals = true
			}
		case ";":
			if declaration && x.depth == 0 {
				x.declaration(x.start, i, false)
				x.reset(i + 1)
			}
		case ":":
			if declaration && x.depth == 0 && !x.equals && x.label(x.start, i) {
				x.reset(i + 1)
			}
		case "{":
			if !declaration {
				x.push(frame{kind: fBody})
				continue
			}
			if x.depth > 0 || x.equals {
				x.push(frame{kind: fExpression, start: x.start, depth: x.depth, equals: x.equals})
				x.reset(i + 1)
				continue
			}
			x.push(x.declaration(x.start, i, true))
			x.reset(i + 1)
		case "}":
			if x.overflow > 0 {
				x.overflow--
				continue
			}
			if len(x.stack) == 0 {
				x.reset(i + 1)
				continue
			}
			f := x.stack[len(x.stack)-1]
			x.stack = x.stack[:len(x.stack)-1]
			if f.kind == fExpression {
				x.start, x.depth, x.equals = f.start, f.depth, f.equals
			} else {
				x.reset(i + 1)
			}
		}
	}
}

// stringImport records import("file") at i.
func (x *extractor) stringImport(i int) {
	tokens := x.tokens
	if i+3 < len(tokens) && tokens[i+2].kind == kString && tokens[i+2].value && tokens[i+3].text == ")" {
		name := tokens[i+2].text
		x.add(lang.RawImport{Spec: `import("` + name + `")`, Module: name, Name: kindString, Line: tokens[i].line})
	}
}

func (x *extractor) add(rawImport lang.RawImport) {
	key := rawImport.Name + "\x00" + rawImport.Module
	if rawImport.Module == "" || x.seen[key] {
		return
	}
	x.seen[key] = true
	x.imports = append(x.imports, rawImport)
}

// importDeclaration reads `import a.b, c = d.e : f, g = h;` from the import keyword at i
// and returns the index of the token that ends it.
func (x *extractor) importDeclaration(i int) int {
	tokens := x.tokens
	j := i + 1
	bindings := false
	for ; j < len(tokens); j++ {
		t := tokens[j]
		if t.kind == kPunctuation && (t.text == ";" || t.text == "{" || t.text == "}") {
			if t.text != ";" {
				return j - 1 // a broken import: the brace is read as usual
			}
			return j
		}
		if bindings || t.kind != kIdentifier {
			if t.kind == kPunctuation && t.text == ":" {
				bindings = true
			}
			continue
		}
		if t.text == "import" {
			return j - 1
		}
		// alias = a.b
		if j+1 < len(tokens) && tokens[j+1].text == "=" {
			j++
			continue
		}
		name, next := dotted(tokens, j)
		if name != "" {
			x.add(lang.RawImport{Spec: name, Module: name, Line: t.line})
		}
		j = next - 1
	}
	return len(tokens) - 1
}

// dotted reads a.b.c from i: the name and the index after it.
func dotted(tokens []token, i int) (string, int) {
	var b strings.Builder
	j := i
	for j < len(tokens) && tokens[j].kind == kIdentifier {
		b.WriteString(tokens[j].text)
		j++
		if j+1 < len(tokens) && tokens[j].kind == kPunctuation && tokens[j].text == "." && tokens[j+1].kind == kIdentifier {
			b.WriteByte('.')
			j++
			continue
		}
		break
	}
	return b.String(), j
}

// attributeWords are storage classes, protection and other attributes that may start
// a declaration or an attribute block.
var attributeWords = lang.WordSet(`static public private protected package export extern final abstract override
synchronized deprecated align const immutable shared inout __gshared nothrow pure ref auto scope lazy
pragma`)

// keywords are D's keywords, never a function's name.
var keywords = lang.WordSet(`abstract alias align asm assert auto body bool break byte case cast catch cdouble cent
cfloat char class const continue creal dchar debug default delegate delete deprecated do double else enum
export extern false final finally float for foreach foreach_reverse function goto idouble if ifloat
immutable import in inout int interface invariant ireal is lazy long macro mixin module new nothrow null
out override package pragma private protected public pure real ref return scope shared short static
struct super switch synchronized template this throw true try typeid typeof ubyte ucent uint ulong union
unittest ushort version void wchar while with __FILE__ __FILE_FULL_PATH__ __MODULE__ __LINE__
__FUNCTION__ __PRETTY_FUNCTION__ __gshared __traits __vector __parameters`)

// skipGroup returns the index after the bracket group opening at i (or i when
// no group opens there), bounded by end.
func (x *extractor) skipGroup(i, end int) int {
	if i >= end || x.tokens[i].kind != kPunctuation || x.tokens[i].text != "(" && x.tokens[i].text != "[" {
		return i
	}
	depth := 0
	for j := i; j < end; j++ {
		if x.tokens[j].kind != kPunctuation {
			continue
		}
		switch x.tokens[j].text {
		case "(", "[":
			depth++
		case ")", "]":
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return end
}

// skipAttributes returns the index of the first token from i that is not an
// attribute: storage classes (with their arguments: extern(C), align(4),
// const(T) read as an attribute too) and @attributes and UDAs.
func (x *extractor) skipAttributes(i, end int) int {
	tokens := x.tokens
	for i < end {
		t := tokens[i]
		switch {
		case t.kind == kIdentifier && attributeWords[t.text]:
			// static if, static foreach, static assert and static this are not
			// attributes of what follows.
			if t.text == "static" && i+1 < end && (tokens[i+1].text == "if" || tokens[i+1].text == "foreach" ||
				tokens[i+1].text == "foreach_reverse" || tokens[i+1].text == "assert" || tokens[i+1].text == "this" ||
				tokens[i+1].text == "~") {
				return i
			}
			i = x.skipGroup(i+1, end)
		case t.kind == kPunctuation && t.text == "@":
			i++
			if i < end && tokens[i].kind == kIdentifier {
				_, i = dotted(tokens, i)
				if i < end && tokens[i].text == "!" {
					i++
					if i < end && tokens[i].kind != kPunctuation {
						i++
					}
				}
			}
			i = x.skipGroup(i, end)
		default:
			return i
		}
	}
	return i
}

// label reports whether tokens[from:at] before a colon are only attributes -
// `private:`, `@safe nothrow:`, `extern(C):`, `version(X):` - which apply to the
// rest of the scope.
func (x *extractor) label(from, at int) bool {
	if at-from > 64 {
		return false // labels are short; this keeps a long run of attributes linear
	}
	i := x.skipAttributes(from, at)
	if i < at && (x.tokens[i].text == "version" || x.tokens[i].text == "debug") {
		i = x.skipGroup(i+1, at)
	}
	return i == at
}

// declaration reads the declaration tokens[from:end] at declaration level, ended
// by a semicolon or (body) the brace at end, records its symbol and returns the
// frame the brace opens.
//
// Implements: REQ-DLANG-003
func (x *extractor) declaration(from, end int, body bool) frame {
	tokens := x.tokens
	i := from
	// Conditions and attributes before a declaration: version (X) void f() { }.
	for {
		i = x.skipAttributes(i, end)
		if i >= end {
			break
		}
		switch w := tokens[i].text; {
		case tokens[i].kind != kIdentifier:
		case w == "version" || w == "debug":
			i = x.skipGroup(i+1, end)
			continue
		case w == "else":
			i++
			continue
		case w == "static" && i+1 < end && (tokens[i+1].text == "if" || tokens[i+1].text == "foreach" || tokens[i+1].text == "foreach_reverse"):
			i = x.skipGroup(i+2, end)
			continue
		}
		break
	}
	if i >= end {
		return frame{kind: fTrans} // private { }, extern(C) { }, version (X) { }, else { }
	}
	t := tokens[i]
	word := ""
	if t.kind == kIdentifier {
		word = t.text
	}
	switch word {
	case "static":
		return frame{kind: fBody} // static this(), static assert
	case "module":
		if name, _ := dotted(tokens, i+1); name != "" && !body {
			x.symbols.Add(name, "module", tokens[i+1].line)
		}
		return frame{kind: fBody}
	case "class", "struct", "interface", "union", "template":
		if i+1 < end && tokens[i+1].kind == kIdentifier {
			x.symbol(tokens[i+1].text, word, tokens[i+1].line)
			return frame{kind: fAgg, name: tokens[i+1].text}
		}
		return frame{kind: fTrans} // an anonymous struct or union: its members are the owner's
	case "mixin":
		if i+2 < end && tokens[i+1].text == "template" && tokens[i+2].kind == kIdentifier {
			x.symbol(tokens[i+2].text, "mixin template", tokens[i+2].line)
			return frame{kind: fAgg, name: tokens[i+2].text}
		}
		return frame{kind: fBody}
	case "enum":
		x.enum(i, end, body)
		return frame{kind: fBody}
	case "alias":
		if !body {
			x.alias(i, end)
		}
		return frame{kind: fBody}
	case "unittest", "invariant", "in", "out", "do", "body", "new", "delete", "if", "foreach":
		return frame{kind: fBody}
	case "this":
		if own := x.owner(); own != "" && i+1 < end && tokens[i+1].text == "(" {
			x.symbols.Add(own+".this", "method", t.line)
		}
		return frame{kind: fBody}
	}
	if t.kind == kPunctuation && t.text == "~" && i+1 < end && tokens[i+1].text == "this" {
		if own := x.owner(); own != "" {
			x.symbols.Add(own+".~this", "method", tokens[i+1].line)
		}
		return frame{kind: fBody}
	}
	if name, line := x.function(i, end); name != "" {
		x.symbol(name, "func", line)
	}
	return frame{kind: fBody}
}

// symbol adds a declaration, qualified by its owners; a function with an owner
// is a method.
func (x *extractor) symbol(name, kind string, line int) {
	if own := x.owner(); own != "" {
		if kind == "func" {
			kind = "method"
		}
		name = own + "." + name
	}
	x.symbols.Add(name, kind, line)
}

// function finds the name of a function declared by tokens[i:end]: the identifier
// before the first parenthesis at depth 0 that is not a type's or an
// attribute's, with no = before it (a variable's initializer).
func (x *extractor) function(i, end int) (string, int) {
	tokens := x.tokens
	for j := i; j < end; j++ {
		t := tokens[j]
		if t.kind != kPunctuation {
			continue
		}
		switch t.text {
		case "=":
			return "", 0
		case "[":
			j = x.skipGroup(j, end) - 1
		case "(":
			p := j - 1
			if p >= i && tokens[p].kind == kIdentifier && !keywords[tokens[p].text] &&
				(p == i || tokens[p-1].text != "!" && tokens[p-1].text != "@" && tokens[p-1].text != ".") {
				return tokens[p].text, tokens[p].line
			}
			j = x.skipGroup(j, end) - 1
		}
	}
	return "", 0
}

// enum reads `enum Name { .. }`, `enum Name : T { .. }` (a type) and, at the top
// level, `enum Name = value;` and `enum T name = value, other = value;`
// (manifest constants).
func (x *extractor) enum(i, end int, body bool) {
	tokens := x.tokens
	if body {
		if i+1 < end && tokens[i+1].kind == kIdentifier && (i+2 == end || tokens[i+2].text == ":") {
			x.symbol(tokens[i+1].text, "enum", tokens[i+1].line)
		}
		return
	}
	if !x.topLevel() {
		return
	}
	// Split at the commas at depth 0; each part's name is the identifier before
	// its = (or before its template parameters).
	part := i + 1
	for j := i + 1; j <= end; j++ {
		if j < end {
			if t := tokens[j]; t.kind == kPunctuation && (t.text == "(" || t.text == "[") {
				j = x.skipGroup(j, end) - 1
				continue
			} else if t.kind != kPunctuation || t.text != "," {
				continue
			}
		}
		for k := part; k < j; k++ {
			if tokens[k].kind == kPunctuation && tokens[k].text == "=" {
				n := k - 1
				if n >= part && tokens[n].text == ")" {
					for n >= part && tokens[n].text != "(" {
						n--
					}
					n--
				}
				if n >= part && tokens[n].kind == kIdentifier && !keywords[tokens[n].text] {
					x.symbols.Add(tokens[n].text, "const", tokens[n].line)
				}
				break
			}
		}
		part = j + 1
	}
}

// alias reads `alias Name = T;`, `alias Name(T) = U;` and the old
// `alias T Name;`, but not `alias this`.
func (x *extractor) alias(i, end int) {
	tokens := x.tokens
	if i+1 >= end {
		return
	}
	if n := tokens[i+1]; n.kind == kIdentifier && i+2 < end && (tokens[i+2].text == "=" || tokens[i+2].text == "(") {
		if n.text != "this" {
			x.symbol(n.text, "alias", n.line)
		}
		return
	}
	last := tokens[end-1]
	if last.kind == kIdentifier && last.text != "this" && !keywords[last.text] && end-1 > i+1 {
		x.symbol(last.text, "alias", last.line)
	}
}
