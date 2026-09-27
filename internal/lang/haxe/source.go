package haxe

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindImport = "import" // import a.b.C, a.b.*, a.b.C.field, a.b.C as D
	kindUsing  = "using"  // using a.b.Tools
	kindRef    = "ref"    // a qualified name in code: haxe.Json.parse(...)
	kindLib    = "lib"    // a haxelib library a manifest declares: name[:version]
	kindLix    = "lix"    // the library a lix haxe_libraries/<name>.hxml pins
	kindCP     = "cp"     // a class path directory
	kindMain   = "main"   // a main class or a root module to compile
	kindHXML   = "hxml"   // another .hxml file an .hxml includes
	kindFile   = "file"   // a resource or an included project file
)

const (
	frameType  = 1 // the body of a class, interface, enum or abstract
	frameFunc  = 2 // a function body
	frameOther = 3 // any other braces: blocks, object literals, structures
)

const (
	maxDepth      = 256 // frames tracked; deeper braces are only counted
	maxConditions = 64  // nested #if snapshots
	maxSegments   = 64  // segments of a dotted path; longer ones are not names
)

type frame struct {
	kind  byte
	name  string
	paren int // the enclosing parenthesis depth, restored when the frame closes
}

// state is what the scanner knows at a point of the file; #if saves it and each
// #elseif/#else starts again from it.
type state struct {
	stack    []frame
	overflow int
	paren    int
	pending  byte   // frameType or frameFunc: the next { opens that body
	pendName string // the type the next { opens
	angle    int    // < > depth in a pending type's header
}

func (s state) clone() state {
	s.stack = append([]frame(nil), s.stack...)
	return s
}

type cond struct {
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
func extractSource(src []byte) *lang.Extraction {
	var tokens []token
	l := newLexer(src)
	for {
		t, ok := l.next()
		if !ok {
			break
		}
		tokens = append(tokens, t)
	}
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	seenType := map[string]bool{}
	seenMod := map[string]bool{}
	var refs []lang.RawImport
	var st state
	var conditions []cond
	condSkip := 0
	at := func(i int) token {
		if i < len(tokens) {
			return tokens[i]
		}
		return token{}
	}
	isP := func(i int, s string) bool { t := at(i); return t.kind == tPunct && t.text == s }
	isI := func(i int, s string) bool { t := at(i); return t.kind == tIdent && t.text == s }
	top := func() *frame {
		if st.overflow > 0 || len(st.stack) == 0 {
			return nil
		}
		return &st.stack[len(st.stack)-1]
	}
	moduleLevel := func() bool { return len(st.stack) == 0 && st.overflow == 0 && st.paren == 0 }
	name := func(i int) string {
		if t := at(i); t.kind == tIdent {
			return t.text
		}
		return ""
	}
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		switch t.kind {
		case tDirective:
			switch t.text {
			case "if":
				if len(conditions) < maxConditions {
					conditions = append(conditions, cond{snap: st.clone()})
				} else {
					condSkip++
				}
			case "elseif", "else":
				if condSkip == 0 && len(conditions) > 0 {
					c := &conditions[len(conditions)-1]
					if c.first == nil {
						f := st.clone()
						c.first = &f
					}
					st = c.snap.clone()
				}
			case "end":
				if condSkip > 0 {
					condSkip--
				} else if len(conditions) > 0 {
					c := conditions[len(conditions)-1]
					conditions = conditions[:len(conditions)-1]
					if c.first != nil {
						st = *c.first
					}
				}
			}
			continue
		case tPunct:
			switch t.text {
			case "{":
				f := frame{kind: frameOther, paren: st.paren}
				if st.pending != 0 && st.paren == 0 && st.angle == 0 {
					f.kind, f.name = st.pending, st.pendName
					st.pending, st.pendName = 0, ""
				}
				if len(st.stack) < maxDepth && st.overflow == 0 {
					st.stack = append(st.stack, f)
				} else {
					st.overflow++
				}
				st.paren = 0
			case "}":
				if st.overflow > 0 {
					st.overflow--
				} else if n := len(st.stack); n > 0 {
					st.paren = st.stack[n-1].paren
					st.stack = st.stack[:n-1]
				}
			case "(", "[":
				st.paren++
			case ")", "]":
				if st.paren > 0 {
					st.paren--
				}
			case "<":
				if st.pending == frameType {
					st.angle++
				}
			case ">":
				if st.pending == frameType && st.angle > 0 {
					st.angle--
				}
			case ";":
				if st.paren == 0 && st.pending == frameFunc {
					st.pending = 0 // a function without a body: interfaces, externs
				}
			case "@":
				// Metadata: @name, @:name, @:a.b - a name that is no keyword.
				j := i + 1
				if isP(j, ":") {
					j++
				}
				if at(j).kind == tIdent && at(j).pos == at(j-1).end {
					for isP(j+1, ".") && at(j+2).kind == tIdent {
						j += 2
					}
					i = j
				}
			}
			continue
		case tIdent:
		default:
			continue
		}
		if i > 0 && isP(i-1, ".") {
			continue // a field access
		}
		switch t.text {
		case "package":
			if moduleLevel() {
				for i+1 < len(tokens) && !isP(i+1, ";") && (at(i+1).kind == tIdent || isP(i+1, ".")) {
					i++
				}
			}
			continue
		case "import", "using":
			if !moduleLevel() {
				continue
			}
			var segments []string
			j := i + 1
			for {
				if at(j).kind == tIdent {
					segments = append(segments, at(j).text)
					j++
				} else if isP(j, "*") && len(segments) > 0 {
					segments = append(segments, "*")
					j++
					break
				} else {
					break
				}
				if !isP(j, ".") {
					break
				}
				j++
			}
			if len(segments) == 0 || len(segments) > maxSegments {
				i = j - 1
				continue
			}
			if isI(j, "as") || isI(j, "in") {
				j += 2
			}
			i = j - 1
			if isP(j, ";") {
				i = j
			}
			p := strings.Join(segments, ".")
			kind := kindImport
			if t.text == "using" {
				kind = kindUsing
			}
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: p, Module: p, Name: kind, Line: t.line})
			seenMod[p] = true
			continue
		case "class", "interface", "enum", "abstract", "typedef":
			if !moduleLevel() {
				break
			}
			kind := t.text
			j := i + 1
			switch {
			case kind == "enum" && isI(j, "abstract"):
				j++ // enum abstract Color(Int)
			case kind == "abstract" && (isI(j, "class") || isI(j, "interface")):
				continue // abstract class: the class keyword follows
			}
			n := name(j)
			if n == "" || notPackage[n] {
				continue
			}
			if !seenType[n] {
				seenType[n] = true
				symbols.Add(n, kind, t.line)
			}
			i = j
			if kind != "typedef" {
				st.pending, st.pendName, st.angle = frameType, n, 0
			}
			continue
		case "function":
			n := name(i + 1)
			if n == "" {
				continue // an anonymous function
			}
			if moduleLevel() {
				symbols.Add(n, "func", t.line)
			} else if f := top(); f != nil && f.kind == frameType && st.paren == 0 {
				symbols.Add(f.name+"."+n, "method", t.line)
			} else {
				continue
			}
			st.pending, st.pendName = frameFunc, ""
			i++
			continue
		case "var", "final":
			if n := name(i + 1); moduleLevel() && n != "" && !notPackage[n] {
				symbols.Add(n, "var", t.line)
				i++
			}
			continue
		}
		// A qualified name: lower-case package segments up to a type.
		if !lower(t.text) || notPackage[t.text] || !isP(i+1, ".") {
			continue
		}
		segments := []string{t.text}
		j := i
		for isP(j+1, ".") && at(j+2).kind == tIdent {
			j += 2
			segments = append(segments, at(j).text)
			if upper(at(j).text) {
				break
			}
		}
		n := len(segments)
		if !upper(segments[n-1]) || n > maxSegments || n == 2 && constant(segments[1]) {
			i = j
			continue
		}
		if isP(j+1, ".") && upper(at(j+2).text) {
			segments = append(segments, at(j+2).text) // a sub-type: haxe.macro.Expr.ExprDef
			j += 2
		}
		i = j
		p := strings.Join(segments, ".")
		if !seenMod[p] {
			seenMod[p] = true
			refs = append(refs, lang.RawImport{Spec: p, Module: p, Name: kindRef, Line: t.line})
		}
	}
	if len(refs) > 0 {
		// A qualified name of a module the file imports adds nothing.
		exact, prefixes := map[string]bool{}, map[string]bool{}
		for _, im := range ex.Imports {
			m := strings.TrimSuffix(im.Module, ".*")
			exact[m] = true
			for k := strings.LastIndexByte(m, '.'); k > 0; k = strings.LastIndexByte(m[:k], '.') {
				prefixes[m[:k]] = true
			}
		}
		for _, r := range refs {
			if !imported(exact, prefixes, r.Module) {
				ex.Imports = append(ex.Imports, r)
			}
		}
	}
	ex.Symbols = symbols.List()
	return ex
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
func readPackage(src []byte) string {
	l := newLexer(src)
	t, ok := l.next()
	if !ok || t.kind != tIdent || t.text != "package" {
		return ""
	}
	var segments []string
	for {
		t, ok = l.next()
		if !ok || t.kind != tIdent {
			break
		}
		segments = append(segments, t.text)
		if t, ok = l.next(); !ok || t.kind != tPunct || t.text != "." {
			break
		}
	}
	return strings.Join(segments, ".")
}
