package dart

import "github.com/sarumaj/depphunter-cli/internal/lang"

// modifiers may precede a declaration without naming it.
var modifiers = map[string]bool{
	"abstract": true, "base": true, "final": true, "interface": true, "sealed": true,
	"external": true, "static": true, "late": true, "const": true, "var": true,
	"covariant": true, "augment": true, "macro": true, "factory": true,
}

// declarations reads the declarations of tokens[i:] at the top level of a library:
// classes (and mixin classes), mixins, enums, extensions, extension types and
// typedefs, top-level functions, getters, setters and variables, and one level into
// the body of each class-like declaration its constructors (Owner.new for the
// unnamed one, Owner.name for a named one), methods, operators, getters, setters and
// fields. Bodies of functions are skipped, not read.
//
// Implements: REQ-DART-003
func declarations(tokens []token, i int, symbols *lang.SymbolSet) {
	members(tokens, i, len(tokens), "", symbols)
}

// members reads the declarations of tokens[i:end]; owner is the class-like declaration
// they belong to ("" at the top level).
func members(tokens []token, i, end int, owner string, symbols *lang.SymbolSet) {
	seen := map[string]bool{} // a setter beside its getter is one property
	add := func(name, kind string, line int) {
		if owner != "" {
			name = owner + "." + name
		}
		if kind == "setter" || kind == "property" {
			if seen[name] {
				return
			}
			kind = "property"
		}
		seen[name] = true
		symbols.Add(name, kind, line)
	}
	for i < end {
		i = skipMetadata(tokens, i)
		if i >= end {
			return
		}
		if t := tokens[i]; t.kind == tPunct && (t.text == ";" || t.text == "}" || t.text == ")" || t.text == "]") {
			i++ // stray: an empty declaration or what a broken file left over
			continue
		}
		h, next, body := header(tokens, i, end)
		i = next
		if len(h) == 0 {
			continue
		}
		name, kind, line, classLike := classify(h, owner)
		if name != "" {
			add(name, kind, line)
		}
		if classLike && body >= 0 && owner == "" {
			start := body + 1
			if kind == "enum" {
				start = enumMembers(tokens, start, next-1)
			}
			members(tokens, start, next-1, name, symbols)
		}
	}
}

// header collects a declaration's tokens from i: everything up to the `;` that ends
// it or the `{` that opens its body, which is skipped. A `{` after `=` or `=>` is an
// expression (a map literal, a closure), not a body; so is one inside brackets. It
// returns the header, where the next declaration starts and the index of the body's
// `{` (-1 without one).
func header(tokens []token, i, end int) ([]token, int, int) {
	start := i
	depth := 0
	expr := false     // after `=` or `=>`: braces are part of an expression
	initList := false // a constructor's `: x = 1, super(...)` before its body
	for ; i < end; i++ {
		t := tokens[i]
		if t.kind == tIdent && t.text == "operator" {
			for i+1 < end && tokens[i+1].kind == tPunct && tokens[i+1].text != "(" {
				i++ // `operator []=`: not an assignment
			}
			continue
		}
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(", "[":
			depth++
		case ")", "]":
			depth--
		case ":":
			if depth == 0 && !expr && i > start && tokens[i-1].text == ")" {
				initList = true
			}
		case "=", "=>":
			if depth == 0 && !initList {
				expr = true
			}
		case ";":
			if depth <= 0 {
				return tokens[start:i], i + 1, -1
			}
		case "{":
			if depth > 0 || expr {
				i = matching(tokens[:end], i)
				continue
			}
			close := matching(tokens[:end], i)
			return tokens[start:i], close + 1, i
		case "}":
			if depth <= 0 { // the enclosing body ends: a declaration without its `;`
				return tokens[start:i], i, -1
			}
			depth--
		}
	}
	return tokens[start:i], i, -1
}

// enumMembers skips an enum's values, returning where its members start: after the
// first `;` at depth 0, or at the end when it has none.
func enumMembers(tokens []token, i, end int) int {
	depth := 0
	for j := i; j < end; j++ {
		switch tokens[j].text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ";":
			if depth == 0 {
				return j + 1
			}
		}
	}
	return end
}

// classify names a declaration from its header: its name, its kind, the line of its
// name, and whether its body holds members.
func classify(h []token, owner string) (string, string, int, bool) {
	// Modifiers are skipped while a name follows them: in `final base = 1`, base is
	// the variable.
	j := 0
	for j+1 < len(h) && h[j].kind == tIdent && modifiers[h[j].text] && h[j+1].kind == tIdent {
		j++
	}
	// `abstract base class`, `sealed class`, `mixin class`: a class.
	for k := 0; k < len(h) && h[k].kind == tIdent; k++ {
		if h[k].text == "class" && k+1 < len(h) && h[k+1].kind == tIdent {
			return h[k+1].text, "class", h[k+1].line, true
		}
		if !modifiers[h[k].text] && h[k].text != "mixin" {
			break
		}
	}
	if j >= len(h) {
		return "", "", 0, false
	}
	kw := h[j]
	if kw.kind == tIdent && owner == "" {
		switch kw.text {
		case "mixin":
			if j+1 < len(h) && h[j+1].kind == tIdent {
				return h[j+1].text, "mixin", h[j+1].line, true
			}
			return "", "", 0, false
		case "enum":
			if j+1 < len(h) && h[j+1].kind == tIdent {
				return h[j+1].text, "enum", h[j+1].line, true
			}
			return "", "", 0, false
		case "extension":
			k := j + 1
			if k+1 < len(h) && h[k].text == "type" && h[k+1].kind == tIdent && h[k+1].text != "on" {
				k++
				if h[k].text == "const" && k+1 < len(h) && h[k+1].kind == tIdent {
					k++
				}
				return h[k].text, "extension type", h[k].line, true
			}
			if k < len(h) && h[k].kind == tIdent && h[k].text != "on" {
				return h[k].text, "extension", h[k].line, true
			}
			return "", "", 0, false // unnamed: nothing can name its members
		case "typedef":
			return typedefName(h[j+1:])
		}
	}
	name, kind, line, _ := member(h[j:], owner)
	for _, t := range h[:j] {
		if t.text == "const" && (kind == "var" || kind == "field") {
			kind = "const"
		}
	}
	return name, kind, line, false
}

// typedefName names `typedef Name<T> = Type;` and the older `typedef R Name(args);`.
func typedefName(h []token) (string, string, int, bool) {
	if len(h) > 0 && h[0].kind == tIdent {
		for k := 1; k < len(h); k++ {
			if h[k].text == "=" {
				return h[0].text, "typedef", h[0].line, false
			}
			if h[k].text == "<" {
				k = skipAngles(h, k) - 1
				continue
			}
			break
		}
	}
	if k := paramList(h); k > 0 {
		return h[k-1].text, "typedef", h[k-1].line, false
	}
	if n := len(h); n > 0 && h[n-1].kind == tIdent {
		return h[n-1].text, "typedef", h[n-1].line, false
	}
	return "", "", 0, false
}

// bodyStart are the tokens that may follow a getter's name.
var bodyStart = map[string]bool{"=>": true, "async": true, "sync": true}

// member names a function, getter, setter, operator, constructor or variable
// declaration (modifiers already taken off): at the top level with owner "", else in
// a class-like body.
func member(h []token, owner string) (string, string, int, bool) {
	fn, field := "func", "var"
	if owner != "" {
		fn, field = "method", "field"
	}
	for k, t := range h {
		if t.kind != tIdent {
			if t.text == "=" || t.text == "(" && k == 0 {
				break
			}
			continue
		}
		switch t.text {
		case "get":
			if k+1 < len(h) && h[k+1].kind == tIdent && (k+2 == len(h) || bodyStart[h[k+2].text]) {
				return h[k+1].text, "property", h[k+1].line, false
			}
		case "set":
			if k+2 < len(h) && h[k+1].kind == tIdent && h[k+2].text == "(" {
				return h[k+1].text, "setter", h[k+1].line, false
			}
		case "operator":
			if owner != "" && k+1 < len(h) && h[k+1].kind == tPunct {
				op := ""
				for m := k + 1; m < len(h) && h[m].text != "("; m++ {
					op += h[m].text
				}
				return "operator" + op, "method", t.line, false
			}
		}
	}
	// A constructor: the owner's name, maybe dotted, then its parameters.
	if owner != "" && len(h) > 1 && h[0].kind == tIdent && h[0].text == owner {
		switch {
		case h[1].text == "(":
			return "new", "constructor", h[0].line, false
		case h[1].text == "." && len(h) > 3 && h[2].kind == tIdent && h[3].text == "(":
			return h[2].text, "constructor", h[2].line, false
		}
	}
	if k := paramList(h); k > 0 {
		return h[k-1].text, fn, h[k-1].line, false
	}
	// A variable: `Type a = 1, b;` is named by its first variable, the last
	// identifier before its `=`, its comma or the end.
	kind := field
	name, line := "", 0
	depth, angles := 0, 0
	inInit := false
	for k := 0; k < len(h); k++ {
		t := h[k]
		if t.kind == tPunct {
			switch t.text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				depth--
			case "<":
				if !inInit {
					angles++
				}
			case ">":
				if !inInit {
					angles--
				}
			case ">>":
				if !inInit {
					angles -= 2
				}
			case "=":
				if depth == 0 && angles <= 0 {
					inInit = true
				}
			case ",":
				if depth == 0 && angles <= 0 {
					if name != "" {
						return name, kind, line, false // the first of a list names the declaration
					}
				}
			}
			continue
		}
		if !inInit && depth == 0 && angles <= 0 && t.kind == tIdent && !modifiers[t.text] {
			name, line = t.text, t.line
		}
	}
	if name == "" {
		return "", "", 0, false
	}
	return name, kind, line, false
}

// paramList finds a declaration's parameter list: the first `(` at depth 0 before any
// `=` that follows the declaration's name (an identifier, or the `>` ending its type
// parameters). A function type's `Function(...)` and a record type's `(...)` are
// types, not parameters. It returns the index of the name + 1, or 0.
func paramList(h []token) int {
	depth := 0
	for k := 0; k < len(h); k++ {
		t := h[k]
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "=", "=>":
			if depth == 0 {
				return 0
			}
		case "[", "{":
			depth++
		case "]", "}":
			depth--
		case ")":
			depth--
		case "(":
			if depth == 0 && k > 0 {
				n := k - 1
				if h[n].text == ">" || h[n].text == ">>" {
					n = openingAngle(h, n) - 1
				}
				if n >= 0 && h[n].kind == tIdent && h[n].text != "Function" && !modifiers[h[n].text] {
					return n + 1
				}
			}
			depth++
		}
	}
	return 0
}

// openingAngle returns the index of the `<` matching the `>` at k.
func openingAngle(h []token, k int) int {
	depth := 0
	for j := k; j >= 0; j-- {
		switch h[j].text {
		case ">":
			depth++
		case ">>":
			depth += 2
		case "<":
			if depth--; depth == 0 {
				return j
			}
		}
	}
	return 0
}
