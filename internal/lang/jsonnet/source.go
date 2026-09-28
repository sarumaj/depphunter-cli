package jsonnet

import "github.com/sarumaj/depphunter-cli/internal/lang"

// Import kinds, carried in RawImport.Name.
const (
	kindImport    = "import"
	kindImportStr = "importstr"
	kindImportBin = "importbin"
	kindDep       = "dep"  // a jsonnetfile.json dependency
	kindLock      = "lock" // a jsonnetfile.lock.json entry
)

// extractSource reads a Jsonnet file: every import, importstr and importbin
// of a string literal, and as symbols the file's leading top-level locals and
// the fields of the object literals its body evaluates to at the top.
//
// Implements: REQ-JSONNET-002, REQ-JSONNET-003
func extractSource(src []byte) *lang.Extraction {
	tokens := lex(src)
	m := match(tokens)
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	for i := 0; i+1 < len(tokens); i++ {
		t := tokens[i]
		if t.kind != tIdent || tokens[i+1].kind != tString {
			continue
		}
		switch t.text {
		case kindImport, kindImportStr, kindImportBin:
		default:
			continue
		}
		mod := tokens[i+1].text
		spec := mod
		if t.text != kindImport {
			spec = t.text + " " + mod
		}
		if mod == "" || seen[spec] {
			continue
		}
		seen[spec] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: mod, Name: t.text, Line: t.line})
	}
	var syms lang.SymbolSet
	defined := map[string]bool{}
	add := func(name, kind string, line int) {
		if !defined[name] {
			defined[name] = true
			syms.Add(name, kind, line)
		}
	}
	i := 0
	// The file's leading `local x = e;` and `assert e;` statements.
leading:
	for i < len(tokens) {
		switch {
		case isWord(tokens[i], "local"):
			i = binds(tokens, m, i+1, ";", add)
		case isWord(tokens[i], "assert"):
			i = skipTo(tokens, m, i+1, len(tokens), ";") + 1
		default:
			break leading
		}
	}
	// The body: object literals at bracket depth 0 (`{...} + {...}`, a
	// function's result, the branches of an if).
	for ; i < len(tokens); i++ {
		t := tokens[i]
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "{":
			end := m[i]
			if end < 0 {
				end = len(tokens)
			}
			object(tokens, m, i+1, end, add)
			i = end
		case "(", "[":
			if m[i] < 0 {
				i = len(tokens)
			} else {
				i = m[i]
			}
		}
	}
	ex.Symbols = syms.List()
	return ex
}

func isWord(t token, w string) bool { return t.kind == tIdent && t.text == w }

func isPunct(t token, p string) bool { return t.kind == tPunct && t.text == p }

// skipTo returns the index of the first stop token at depth 0 from i, or end.
func skipTo(tokens []token, m []int, i, end int, stops ...string) int {
	for ; i < end; i++ {
		t := tokens[i]
		if t.kind != tPunct {
			continue
		}
		for _, s := range stops {
			if t.text == s {
				return i
			}
		}
		switch t.text {
		case "{", "(", "[":
			if m[i] < 0 || m[i] > end {
				return end
			}
			i = m[i]
		}
	}
	return end
}

// binds reads `a = e, f(x) = e` from i up to the terminator term (";" at the
// top, "," ends a whole object-level local) and returns the index after it.
func binds(tokens []token, m []int, i int, term string, add func(string, string, int)) int {
	for i < len(tokens) {
		if tokens[i].kind != tIdent {
			return skipTo(tokens, m, i, len(tokens), term) + 1
		}
		name, line := tokens[i].text, tokens[i].line
		kind := "var"
		j := i + 1
		if j < len(tokens) && isPunct(tokens[j], "(") {
			kind = "func"
			if m[j] < 0 {
				return len(tokens)
			}
			j = m[j] + 1
		} else if j+1 < len(tokens) && isPunct(tokens[j], "=") && isWord(tokens[j+1], "function") {
			kind = "func"
		}
		add(name, kind, line)
		k := skipTo(tokens, m, j, len(tokens), ",", term)
		if k >= len(tokens) || tokens[k].text == term {
			return k + 1
		}
		i = k + 1
	}
	return len(tokens)
}

// object records the fields of the object literal between from and end (the
// braces excluded): `a: e`, `a:: e`, `a+: e`, `'a-b': e`, `f(x): e`.
// Computed fields, object locals, asserts and comprehensions give none.
func object(tokens []token, m []int, from, end int, add func(string, string, int)) {
	for i := from; i < end; {
		t := tokens[i]
		if isWord(t, "for") {
			return // an object comprehension
		}
		if t.kind == tIdent && t.text != "local" && t.text != "assert" || t.kind == tString {
			kind := "field"
			j := i + 1
			if j < end && isPunct(tokens[j], "(") && m[j] > 0 && m[j] < end {
				kind = "func"
				j = m[j] + 1
			}
			if j < end && isPunct(tokens[j], "+") {
				j++
			}
			if j < end && isPunct(tokens[j], ":") {
				for j < end && isPunct(tokens[j], ":") {
					j++
				}
				if j < end && isWord(tokens[j], "function") {
					kind = "func"
				}
				add(t.text, kind, t.line)
			}
		}
		k := skipTo(tokens, m, i, end, ",")
		if k < end && k+1 < end && isWord(tokens[k+1], "for") {
			return
		}
		i = k + 1
	}
}
