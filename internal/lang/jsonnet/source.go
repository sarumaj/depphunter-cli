package jsonnet

import "github.com/sarumaj/depphunter-cli/internal/lang"

// Import kinds, carried in RawImport.Name.
const (
	kindImport       = "import"
	kindImportString = "importstr"
	kindImportBin    = "importbin"
	kindDependency   = "dep"  // a jsonnetfile.json dependency
	kindLock         = "lock" // a jsonnetfile.lock.json entry
)

// extractSource reads a Jsonnet file: every import, importstr and importbin
// of a string literal, and as symbols the file's leading top-level locals and
// the fields of the object literals its body evaluates to at the top.
//
// Implements: REQ-JSONNET-002, REQ-JSONNET-003
func extractSource(source []byte) *lang.Extraction {
	tokens := lex(source)
	m := match(tokens)
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	for i := 0; i+1 < len(tokens); i++ {
		t := tokens[i]
		if t.kind != tIdentifier || tokens[i+1].kind != tString {
			continue
		}
		switch t.text {
		case kindImport, kindImportString, kindImportBin:
		default:
			continue
		}
		module := tokens[i+1].text
		spec := module
		if t.text != kindImport {
			spec = t.text + " " + module
		}
		if module == "" || seen[spec] {
			continue
		}
		seen[spec] = true
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: t.text, Line: t.line})
	}
	var symbols lang.SymbolSet
	defined := map[string]bool{}
	add := func(name, kind string, line int) {
		if !defined[name] {
			defined[name] = true
			symbols.Add(name, kind, line)
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
		if t.kind != tPunctuation {
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
	extraction.Symbols = symbols.List()
	return extraction
}

func isWord(t token, w string) bool { return t.kind == tIdentifier && t.text == w }

func isPunctuation(t token, p string) bool { return t.kind == tPunctuation && t.text == p }

// skipTo returns the index of the first stop token at depth 0 from i, or end.
func skipTo(tokens []token, m []int, i, end int, stops ...string) int {
	for ; i < end; i++ {
		t := tokens[i]
		if t.kind != tPunctuation {
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
		if tokens[i].kind != tIdentifier {
			return skipTo(tokens, m, i, len(tokens), term) + 1
		}
		name, line := tokens[i].text, tokens[i].line
		kind := "var"
		j := i + 1
		if j < len(tokens) && isPunctuation(tokens[j], "(") {
			kind = "func"
			if m[j] < 0 {
				return len(tokens)
			}
			j = m[j] + 1
		} else if j+1 < len(tokens) && isPunctuation(tokens[j], "=") && isWord(tokens[j+1], "function") {
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
		if t.kind == tIdentifier && t.text != "local" && t.text != "assert" || t.kind == tString {
			kind := "field"
			j := i + 1
			if j < end && isPunctuation(tokens[j], "(") && m[j] > 0 && m[j] < end {
				kind = "func"
				j = m[j] + 1
			}
			if j < end && isPunctuation(tokens[j], "+") {
				j++
			}
			if j < end && isPunctuation(tokens[j], ":") {
				for j < end && isPunctuation(tokens[j], ":") {
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
