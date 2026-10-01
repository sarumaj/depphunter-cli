package beam

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// extractErlang reads an Erlang module or header: its -module, functions (exported
// ones as kind function, the rest as func), records, macros and types; -include,
// -include_lib, -behaviour and -import, and every remote call mod:fun(...) to a
// module named by an atom.
//
// Implements: REQ-BEAM-004, REQ-BEAM-005
func extractErlang(source []byte) *lang.Extraction {
	x := &erlangExtractor{
		tokens: lexErlang(source), extraction: &lang.Extraction{}, seen: map[string]bool{},
		exported: map[string]bool{}, functionSeen: map[string]bool{}, defined: map[string]bool{},
	}
	formStart := true
	for i := 0; i < len(x.tokens); i++ {
		t := x.tokens[i]
		if t.kind == tPunctuation && t.value == "end" {
			formStart = true
			continue
		}
		atStart := formStart
		formStart = false
		switch {
		case atStart && t.kind == tPunctuation && t.value == "-" && x.tokenAt(i+1).kind == tAtom:
			x.attribute(i)
		case atStart && t.kind == tAtom && x.punctuation(i+1, "("):
			// A function's first clause starts a form: name(Args) ->
			key := t.value + "/" + strconv.Itoa(erlArity(x.tokens, i+1))
			if !x.functionSeen[key] {
				x.functionSeen[key] = true
				x.functions = append(x.functions, erlangFunction{key, t.line})
			}
		case t.kind == tAtom && x.punctuation(i+1, ":") && x.tokenAt(i+2).kind == tAtom && !x.punctuation(i-1, "?") && !x.punctuation(i-1, "#") && t.value != x.self && !catchClass[t.value]:
			// mod:fun - a remote call, a fun reference or a remote type.
			x.add(lang.RawImport{Spec: t.value, Module: t.value, Name: kindErlang, Line: t.line})
		}
	}
	return x.finish()
}

// erlangExtractor is extractErlang's walk over a module's forms, and what it has
// found: the module's own name, what it exports, and its functions in order.
type erlangExtractor struct {
	tokens       []token
	self         string
	exported     map[string]bool
	exportAll    bool
	functions    []erlangFunction
	functionSeen map[string]bool
	defined      map[string]bool

	extraction *lang.Extraction
	symbols    lang.SymbolSet
	seen       map[string]bool
}

// erlangFunction is a function by name/arity, and the line of its first clause.
type erlangFunction struct {
	name string
	line int
}

func (x *erlangExtractor) tokenAt(i int) token {
	if i >= 0 && i < len(x.tokens) {
		return x.tokens[i]
	}
	return token{kind: tPunctuation, value: "eof"}
}

func (x *erlangExtractor) punctuation(i int, v string) bool {
	t := x.tokenAt(i)
	return t.kind == tPunctuation && t.value == v
}

func (x *erlangExtractor) add(rawImport lang.RawImport) {
	if key := rawImport.Name + "|" + rawImport.Module; !x.seen[key] {
		x.seen[key] = true
		x.extraction.Imports = append(x.extraction.Imports, rawImport)
	}
}

func (x *erlangExtractor) define(name, kind string, line int) {
	if !x.defined[name] {
		x.defined[name] = true
		x.symbols.Add(name, kind, line)
	}
}

// attribute reads the -attribute form whose - is at i.
func (x *erlangExtractor) attribute(i int) {
	t, attribute := x.tokens[i], x.tokens[i+1]
	j := i + 2
	parenthesis := x.punctuation(j, "(")
	if parenthesis {
		j++
	}
	argument := x.tokenAt(j)
	switch attribute.value {
	case "module":
		if argument.kind == tAtom {
			x.self = argument.value
			x.define(argument.value, "module", t.line)
		}
	case "export":
		for k := j; k < len(x.tokens) && !x.punctuation(k, "end"); k++ {
			if x.tokens[k].kind == tAtom && x.punctuation(k+1, "/") && x.tokenAt(k+2).kind == tNumber {
				x.exported[x.tokens[k].value+"/"+x.tokens[k+2].value] = true
			}
		}
	case "compile":
		for k := j; k < len(x.tokens) && !x.punctuation(k, "end"); k++ {
			if x.tokens[k].kind == tAtom && x.tokens[k].value == "export_all" {
				x.exportAll = true
			}
		}
	case "record":
		if argument.kind == tAtom {
			x.define("#"+argument.value+"{}", "record", t.line)
		}
	case "define":
		if argument.kind == tAtom || argument.kind == tVariable {
			x.define("?"+argument.value, "macro", t.line)
		}
	case "type", "opaque":
		if argument.kind == tAtom {
			x.define(argument.value+"()", "type", t.line)
		}
	case "include", "include_lib":
		if argument.kind == tString && parenthesis {
			x.add(lang.RawImport{Spec: "-" + attribute.value + `("` + argument.value + `")`, Module: argument.value, Name: attribute.value, Line: t.line})
		}
	case "behaviour", "behavior":
		if argument.kind == tAtom {
			x.add(lang.RawImport{Spec: "-" + attribute.value + "(" + argument.value + ")", Module: argument.value, Name: kindErlang, Line: t.line})
		}
	case "import":
		if argument.kind == tAtom && parenthesis {
			x.add(lang.RawImport{Spec: "-import(" + argument.value + ")", Module: argument.value, Name: kindErlang, Line: t.line})
		}
	}
}

// finish declares the functions, exported or not, and puts the imports in the
// order they are written.
func (x *erlangExtractor) finish() *lang.Extraction {
	for _, f := range x.functions {
		kind := "func"
		if x.exportAll || x.exported[f.name] {
			kind = "function"
		}
		x.define(f.name, kind, f.line)
	}
	x.extraction.Symbols = x.symbols.List()
	slices.SortStableFunc(x.extraction.Imports, func(a, b lang.RawImport) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Spec, b.Spec))
	})
	return x.extraction
}

// catchClass are the exception classes of a catch clause (throw:not_found ->), not
// modules.
var catchClass = map[string]bool{"error": true, "exit": true, "throw": true}

// erlArity counts the arguments of the parenthesized list opening at i.
func erlArity(tokens []token, i int) int {
	depth, n, any := 0, 0, false
	for j := i; j < len(tokens); j++ {
		t := tokens[j]
		if t.kind != tPunctuation {
			any = true
			continue
		}
		switch t.value {
		case "(", "[", "{", "<<":
			depth++
			if depth > 1 {
				any = true
			}
		case ")", "]", "}", ">>":
			depth--
			if depth == 0 {
				if any {
					n++
				}
				return n
			}
		case ",":
			if depth == 1 {
				n++
			}
		case "end":
			return n
		default:
			any = true
		}
	}
	return n
}
