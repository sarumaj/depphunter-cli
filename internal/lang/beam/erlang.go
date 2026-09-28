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
	tokens := lexErlang(source)
	tokenAt := func(i int) token {
		if i >= 0 && i < len(tokens) {
			return tokens[i]
		}
		return token{kind: tPunctuation, value: "eof"}
	}
	punctuation := func(i int, v string) bool { t := tokenAt(i); return t.kind == tPunctuation && t.value == v }

	var symbols lang.SymbolSet
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	add := func(rawImport lang.RawImport) {
		if key := rawImport.Name + "|" + rawImport.Module; !seen[key] {
			seen[key] = true
			extraction.Imports = append(extraction.Imports, rawImport)
		}
	}
	self := ""
	exported := map[string]bool{}
	exportAll := false
	type fun struct {
		name string
		line int
	}
	var funs []fun
	funSeen := map[string]bool{}
	defined := map[string]bool{}
	define := func(name, kind string, line int) {
		if !defined[name] {
			defined[name] = true
			symbols.Add(name, kind, line)
		}
	}

	formStart := true
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.kind == tPunctuation && t.value == "end" {
			formStart = true
			continue
		}
		atStart := formStart
		formStart = false
		if atStart && t.kind == tPunctuation && t.value == "-" && tokenAt(i+1).kind == tAtom {
			attribute := tokens[i+1]
			j := i + 2
			parenthesis := punctuation(j, "(")
			if parenthesis {
				j++
			}
			argument := tokenAt(j)
			switch attribute.value {
			case "module":
				if argument.kind == tAtom {
					self = argument.value
					define(argument.value, "module", t.line)
				}
			case "export":
				for k := j; k < len(tokens) && !punctuation(k, "end"); k++ {
					if tokens[k].kind == tAtom && punctuation(k+1, "/") && tokenAt(k+2).kind == tNumber {
						exported[tokens[k].value+"/"+tokens[k+2].value] = true
					}
				}
			case "compile":
				for k := j; k < len(tokens) && !punctuation(k, "end"); k++ {
					if tokens[k].kind == tAtom && tokens[k].value == "export_all" {
						exportAll = true
					}
				}
			case "record":
				if argument.kind == tAtom {
					define("#"+argument.value+"{}", "record", t.line)
				}
			case "define":
				if argument.kind == tAtom || argument.kind == tVariable {
					define("?"+argument.value, "macro", t.line)
				}
			case "type", "opaque":
				if argument.kind == tAtom {
					define(argument.value+"()", "type", t.line)
				}
			case "include", "include_lib":
				if argument.kind == tString && parenthesis {
					add(lang.RawImport{Spec: "-" + attribute.value + `("` + argument.value + `")`, Module: argument.value, Name: attribute.value, Line: t.line})
				}
			case "behaviour", "behavior":
				if argument.kind == tAtom {
					add(lang.RawImport{Spec: "-" + attribute.value + "(" + argument.value + ")", Module: argument.value, Name: kindErlang, Line: t.line})
				}
			case "import":
				if argument.kind == tAtom && parenthesis {
					add(lang.RawImport{Spec: "-import(" + argument.value + ")", Module: argument.value, Name: kindErlang, Line: t.line})
				}
			}
			continue
		}
		// A function's first clause starts a form: name(Args) ->
		if atStart && t.kind == tAtom && punctuation(i+1, "(") {
			key := t.value + "/" + strconv.Itoa(erlArity(tokens, i+1))
			if !funSeen[key] {
				funSeen[key] = true
				funs = append(funs, fun{key, t.line})
			}
			continue
		}
		// mod:fun - a remote call, a fun reference or a remote type.
		if t.kind == tAtom && punctuation(i+1, ":") && tokenAt(i+2).kind == tAtom && !punctuation(i-1, "?") && !punctuation(i-1, "#") && t.value != self && !catchClass[t.value] {
			add(lang.RawImport{Spec: t.value, Module: t.value, Name: kindErlang, Line: t.line})
		}
	}
	for _, f := range funs {
		kind := "func"
		if exportAll || exported[f.name] {
			kind = "function"
		}
		define(f.name, kind, f.line)
	}
	extraction.Symbols = symbols.List()
	slices.SortStableFunc(extraction.Imports, func(a, b lang.RawImport) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Spec, b.Spec))
	})
	return extraction
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
