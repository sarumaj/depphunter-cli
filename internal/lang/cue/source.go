package cue

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindImport     = "import"
	kindDependency = "dep" // a module.cue dependency
)

// endsLine reports whether a line ending in t ends a declaration (CUE
// inserts a comma there, as Go inserts semicolons).
func endsLine(t token) bool {
	switch t.kind {
	case tIdentifier, tString, tNumber:
		return true
	}
	return t.text == ")" || t.text == "]" || t.text == "}"
}

// declarations calls function with the index of each declaration start between from and
// end at bracket depth 0 (what nests is jumped over).
func declarations(tokens []token, m []int, from, end int, function func(i int)) {
	start := true
	for i := from; i < end; i++ {
		t := tokens[i]
		if i > from && t.first && endsLine(tokens[i-1]) {
			start = true
		}
		if start {
			function(i)
			start = false
		}
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case ",":
			start = true
		case "{", "(", "[":
			if m[i] < 0 || m[i] >= end {
				return
			}
			i = m[i]
		}
	}
}

// label reads a field label at i - an identifier (a #definition too) or a
// plain string, an optional alias `X=` before it and `?`/`!` after it - and
// returns its name, whether a colon follows, and the index after the colon.
func label(tokens []token, i, end int) (name string, ok bool, next int) {
	if i+2 < end && tokens[i].kind == tIdentifier && isPunctuation(tokens[i+1], "=") {
		i += 2
	}
	if i >= end {
		return "", false, i
	}
	t := tokens[i]
	switch {
	case t.kind == tIdentifier:
	case t.kind == tString && !strings.Contains(t.text, "\\("):
	default:
		return "", false, i
	}
	j := i + 1
	if j < end && (isPunctuation(tokens[j], "?") || isPunctuation(tokens[j], "!")) {
		j++
	}
	if j < end && isPunctuation(tokens[j], ":") {
		return t.text, true, j + 1
	}
	return "", false, i
}

func isPunctuation(t token, p string) bool { return t.kind == tPunctuation && t.text == p }

// extractSource reads a CUE file: its package clause and imports, and as
// symbols its package, top-level definitions (#Name) and fields.
//
// Implements: REQ-CUE-002, REQ-CUE-003
func extractSource(source []byte) *lang.Extraction {
	tokens := lex(source)
	m := match(tokens)
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	defined := map[string]bool{}
	seen := map[string]bool{}
	sawPackage := false
	addImport := func(t token) {
		if t.kind == tString && t.text != "" && !seen[t.text] {
			seen[t.text] = true
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: t.text, Module: t.text, Name: kindImport, Line: t.line})
		}
	}
	declarations(tokens, m, 0, len(tokens), func(i int) {
		t := tokens[i]
		next := func(k int) token {
			if i+k < len(tokens) {
				return tokens[i+k]
			}
			return token{}
		}
		switch {
		case t.kind == tIdentifier && t.text == "package" && next(1).kind == tIdentifier && !isPunctuation(next(2), ":"):
			if !sawPackage {
				sawPackage = true
				symbols.Add(next(1).text, "package", t.line)
			}
			return
		case t.kind == tIdentifier && t.text == "import" && isPunctuation(next(1), "("):
			end := m[i+1]
			if end < 0 {
				end = len(tokens)
			}
			for k := i + 2; k < end; k++ {
				addImport(tokens[k])
			}
			return
		case t.kind == tIdentifier && t.text == "import" && next(1).kind == tString:
			addImport(next(1))
			return
		case t.kind == tIdentifier && t.text == "import" && next(1).kind == tIdentifier && next(2).kind == tString:
			addImport(next(2))
			return
		}
		name, ok, _ := label(tokens, i, len(tokens))
		if !ok || defined[name] {
			return
		}
		defined[name] = true
		kind := "field"
		if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "_#") {
			kind = "type"
		}
		symbols.Add(name, kind, t.line)
	})
	extraction.Symbols = symbols.List()
	return extraction
}

// value is a module.cue value: a string, or a struct of fields.
type value struct {
	text   string
	fields map[string]*value
	line   int
}

// structOf reads the fields between from and end: `a: "x"`, `a: {...}`, and
// label chains `a: b: "x"`; other values are left out, and a field given
// twice merges.
func structOf(tokens []token, m []int, from, end, depth int) map[string]*value {
	out := map[string]*value{}
	declarations(tokens, m, from, end, func(i int) {
		fieldAt(tokens, m, i, end, out, depth)
	})
	return out
}

func fieldAt(tokens []token, m []int, i, end int, into map[string]*value, depth int) {
	name, ok, j := label(tokens, i, end)
	if !ok || j >= end || depth > 32 {
		return
	}
	v := into[name]
	if v == nil {
		v = &value{line: tokens[i].line}
		into[name] = v
	}
	if _, chain, _ := label(tokens, j, end); chain {
		if v.fields == nil {
			v.fields = map[string]*value{}
		}
		fieldAt(tokens, m, j, end, v.fields, depth+1)
		return
	}
	switch t := tokens[j]; {
	case t.kind == tString:
		v.text = t.text
	case isPunctuation(t, "{") && m[j] > j && m[j] <= end:
		if v.fields == nil {
			v.fields = map[string]*value{}
		}
		for k, x := range structOf(tokens, m, j+1, m[j], depth+1) {
			if v.fields[k] == nil {
				v.fields[k] = x
			}
		}
	}
}

// moduleFile is what cue.mod/module.cue says.
type moduleFile struct {
	path         string // the module path, major version suffix removed
	dependencies []*moduleDependency
}

type moduleDependency struct {
	key  string // as written: github.com/x/y@v0
	path string // without the major version
	v    string
	line int
}

// readModule reads cue.mod/module.cue: `module:` and the `deps:` of the
// modules system (each with its `v:`).
//
// Implements: REQ-CUE-005
func readModule(source []byte) *moduleFile {
	tokens := lex(source)
	fields := structOf(tokens, match(tokens), 0, len(tokens), 0)
	parsed := &moduleFile{}
	if v := fields["module"]; v != nil {
		parsed.path = stripMajor(v.text)
	}
	if d := fields["deps"]; d != nil {
		for _, k := range sortedKeys(d.fields) {
			x := d.fields[k]
			md := &moduleDependency{key: k, path: stripMajor(k), line: x.line}
			if v := x.fields["v"]; v != nil {
				md.v = v.text
			}
			if md.path != "" {
				parsed.dependencies = append(parsed.dependencies, md)
			}
		}
	}
	return parsed
}

// stripMajor removes a major version suffix (@v0) from each element of an
// import or module path.
func stripMajor(p string) string {
	if !strings.Contains(p, "@") {
		return p
	}
	segments := strings.Split(p, "/")
	for i, s := range segments {
		if at := strings.IndexByte(s, '@'); at >= 0 {
			segments[i] = s[:at]
		}
	}
	return strings.Join(segments, "/")
}
