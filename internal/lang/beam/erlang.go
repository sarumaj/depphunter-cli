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
func extractErlang(src []byte) *lang.Extraction {
	toks := lexErlang(src)
	tok := func(i int) token {
		if i >= 0 && i < len(toks) {
			return toks[i]
		}
		return token{kind: tPunct, val: "eof"}
	}
	punct := func(i int, v string) bool { t := tok(i); return t.kind == tPunct && t.val == v }

	var symbols lang.SymbolSet
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	add := func(im lang.RawImport) {
		if key := im.Name + "|" + im.Module; !seen[key] {
			seen[key] = true
			ex.Imports = append(ex.Imports, im)
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
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == tPunct && t.val == "end" {
			formStart = true
			continue
		}
		atStart := formStart
		formStart = false
		if atStart && t.kind == tPunct && t.val == "-" && tok(i+1).kind == tAtom {
			attr := toks[i+1]
			j := i + 2
			paren := punct(j, "(")
			if paren {
				j++
			}
			arg := tok(j)
			switch attr.val {
			case "module":
				if arg.kind == tAtom {
					self = arg.val
					define(arg.val, "module", t.line)
				}
			case "export":
				for k := j; k < len(toks) && !punct(k, "end"); k++ {
					if toks[k].kind == tAtom && punct(k+1, "/") && tok(k+2).kind == tNum {
						exported[toks[k].val+"/"+toks[k+2].val] = true
					}
				}
			case "compile":
				for k := j; k < len(toks) && !punct(k, "end"); k++ {
					if toks[k].kind == tAtom && toks[k].val == "export_all" {
						exportAll = true
					}
				}
			case "record":
				if arg.kind == tAtom {
					define("#"+arg.val+"{}", "record", t.line)
				}
			case "define":
				if arg.kind == tAtom || arg.kind == tVar {
					define("?"+arg.val, "macro", t.line)
				}
			case "type", "opaque":
				if arg.kind == tAtom {
					define(arg.val+"()", "type", t.line)
				}
			case "include", "include_lib":
				if arg.kind == tString && paren {
					add(lang.RawImport{Spec: "-" + attr.val + `("` + arg.val + `")`, Module: arg.val, Name: attr.val, Line: t.line})
				}
			case "behaviour", "behavior":
				if arg.kind == tAtom {
					add(lang.RawImport{Spec: "-" + attr.val + "(" + arg.val + ")", Module: arg.val, Name: kindErlang, Line: t.line})
				}
			case "import":
				if arg.kind == tAtom && paren {
					add(lang.RawImport{Spec: "-import(" + arg.val + ")", Module: arg.val, Name: kindErlang, Line: t.line})
				}
			}
			continue
		}
		// A function's first clause starts a form: name(Args) ->
		if atStart && t.kind == tAtom && punct(i+1, "(") {
			key := t.val + "/" + strconv.Itoa(erlArity(toks, i+1))
			if !funSeen[key] {
				funSeen[key] = true
				funs = append(funs, fun{key, t.line})
			}
			continue
		}
		// mod:fun - a remote call, a fun reference or a remote type.
		if t.kind == tAtom && punct(i+1, ":") && tok(i+2).kind == tAtom && !punct(i-1, "?") && !punct(i-1, "#") && t.val != self && !catchClass[t.val] {
			add(lang.RawImport{Spec: t.val, Module: t.val, Name: kindErlang, Line: t.line})
		}
	}
	for _, f := range funs {
		kind := "func"
		if exportAll || exported[f.name] {
			kind = "function"
		}
		define(f.name, kind, f.line)
	}
	ex.Symbols = symbols.List()
	slices.SortStableFunc(ex.Imports, func(a, b lang.RawImport) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Spec, b.Spec))
	})
	return ex
}

// catchClass are the exception classes of a catch clause (throw:not_found ->), not
// modules.
var catchClass = map[string]bool{"error": true, "exit": true, "throw": true}

// erlArity counts the arguments of the parenthesized list opening at i.
func erlArity(toks []token, i int) int {
	depth, n, any := 0, 0, false
	for j := i; j < len(toks); j++ {
		t := toks[j]
		if t.kind != tPunct {
			any = true
			continue
		}
		switch t.val {
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
