package elm

import (
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindModule = "module" // import A.B as C exposing (..)
	kindDep    = "dep"    // a package elm.json lists
	kindSrcDir = "srcdir" // a source directory elm.json lists
)

type tokKind uint8

const (
	tLower  tokKind = iota // a lower-case name or keyword, possibly qualified (Html.text)
	tUpper                 // an upper-case name, possibly qualified (Json.Decode, Maybe.Just)
	tString                // "...", """...""" or a glsl block: never a name
	tChar                  // 'x'
	tPunct                 // a bracket, a comma or a run of operator characters
	tOther                 // numbers and anything else
)

type token struct {
	kind tokKind
	text string
	line int
	// first is true for a token starting in column 0: Elm's layout rule puts every
	// top-level declaration there and everything else further right.
	first bool
}

// lex splits Elm source into tokens. `--` comments and nested `{- -}` comments are
// dropped; strings ("..." and """..."""), characters and [glsl| ... |] blocks are
// single tokens, so nothing inside them can look like a declaration. A qualified
// name (Json.Decode.field, Maybe.Just) is one token. Token texts are slices of one
// string copy of src.
//
// Implements: REQ-ELM-010
func lex(src []byte) []token {
	s := string(src)
	var out []token
	line, lineStart := 1, 0
	emit := func(kind tokKind, start, end, at int) {
		out = append(out, token{kind: kind, text: s[start:end], line: at, first: start == lineStart})
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			i++
			line, lineStart = line+1, i
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '-' && at(s, i+1) == '-':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '{' && at(s, i+1) == '-':
			// Block comments nest: {- a {- b -} c -} is one comment.
			depth := 0
			for i < len(s) {
				switch {
				case s[i] == '{' && at(s, i+1) == '-':
					depth++
					i += 2
				case s[i] == '-' && at(s, i+1) == '}':
					depth--
					i += 2
				default:
					if s[i] == '\n' {
						line, lineStart = line+1, i+1
					}
					i++
				}
				if depth == 0 {
					break
				}
			}
		case c == '"':
			start, startLine := i, line
			if at(s, i+1) == '"' && at(s, i+2) == '"' {
				i += 3
				for i < len(s) && !(s[i] == '"' && at(s, i+1) == '"' && at(s, i+2) == '"') {
					if s[i] == '\\' {
						i++
					}
					if i < len(s) && s[i] == '\n' {
						line, lineStart = line+1, i+1
					}
					i++
				}
				i = min(i+3, len(s))
			} else {
				// A single-line string ends at its quote or, unterminated, at the line.
				i++
				for i < len(s) && s[i] != '"' && s[i] != '\n' {
					if s[i] == '\\' && at(s, i+1) != '\n' {
						i++
					}
					i++
				}
				if i < len(s) && s[i] == '"' {
					i++
				}
				i = min(i, len(s))
			}
			out = append(out, token{kind: tString, text: s[start:i], line: startLine, first: start == lineStart})
		case c == '\'':
			start := i
			i++
			for i < len(s) && s[i] != '\'' && s[i] != '\n' {
				if s[i] == '\\' && at(s, i+1) != '\n' {
					i++
				}
				i++
			}
			if i < len(s) && s[i] == '\'' {
				i++
			}
			i = min(i, len(s))
			emit(tChar, start, i, line)
		case c == '[' && hasPrefix(s, i+1, "glsl|"):
			start, startLine := i, line
			for i += 6; i < len(s) && !(s[i] == '|' && at(s, i+1) == ']'); i++ {
				if s[i] == '\n' {
					line, lineStart = line+1, i+1
				}
			}
			i = min(i+2, len(s))
			out = append(out, token{kind: tString, text: s[start:i], line: startLine, first: start == lineStart})
		case isLower(c) || c == '_' || isUpper(c):
			start := i
			kind := tLower
			for {
				if isUpper(s[i]) {
					kind = tUpper
				} else {
					kind = tLower
				}
				for i < len(s) && isWord(s[i]) {
					i++
				}
				// Only an upper-case segment qualifies what follows: Html.text, not
				// model.field.
				if kind == tUpper && at(s, i) == '.' && (isLower(at(s, i+1)) || isUpper(at(s, i+1))) {
					i++
					continue
				}
				break
			}
			emit(kind, start, i, line)
		case c >= '0' && c <= '9':
			start := i
			for i < len(s) && (isWord(s[i]) || s[i] == '.' && at(s, i+1) >= '0' && at(s, i+1) <= '9' ||
				(s[i] == '-' || s[i] == '+') && (s[i-1] == 'e' || s[i-1] == 'E') && !hasPrefix(s, start, "0x")) {
				i++
			}
			emit(tOther, start, i, line)
		case c == '(' || c == ')' || c == '[' || c == ']' || c == '{' || c == '}' || c == ',':
			emit(tPunct, i, i+1, line)
			i++
		case isSymbol(c):
			start := i
			for i < len(s) && isSymbol(s[i]) && !(s[i] == '-' && at(s, i+1) == '-' && i > start) {
				i++
			}
			emit(tPunct, start, i, line)
		case c < 0x80:
			emit(tOther, i, i+1, line)
			i++
		default:
			i++ // a byte of a UTF-8 sequence outside strings and comments: not Elm
		}
	}
	return out
}

func at(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func hasPrefix(s string, i int, p string) bool {
	return i <= len(s) && len(s)-i >= len(p) && s[i:i+len(p)] == p
}

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

func isWord(c byte) bool { return isLower(c) || isUpper(c) || c >= '0' && c <= '9' || c == '_' }

func isSymbol(c byte) bool {
	switch c {
	case '+', '-', '*', '/', '=', '<', '>', '|', '&', '^', '%', '!', '?', ':', '.', '~', '$', '#', '@', '\\':
		return true
	}
	return false
}

// extractSource reads a module's imports and its top-level declarations. Elm's
// layout rule does the parsing: a declaration starts with a token in column 0, and
// everything up to the next such token belongs to it.
//
// Symbols: functions and values (with or without a type annotation, which counts
// once), `type` and `type alias` (with the constructors of a custom type as
// `Type.Ctor`), `port` declarations and the operators `infix` declares.
//
// Implements: REQ-ELM-002, REQ-ELM-003, REQ-ELM-010
func extractSource(src []byte) *lang.Extraction {
	tokens := lex(src)
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	seenImport := map[string]bool{}
	defined := map[string]bool{}
	add := func(name, kind string, line int) {
		if !defined[name] {
			defined[name] = true
			symbols.Add(name, kind, line)
		}
	}
	for i := 0; i < len(tokens); {
		tk := tokens[i]
		// end is the next declaration's first token.
		end := i + 1
		for end < len(tokens) && !tokens[end].first {
			end++
		}
		decl := tokens[i:end]
		i = end
		if !tk.first {
			continue // before the first column-0 token: nothing to read
		}
		get := func(j int) token {
			if j < len(decl) {
				return decl[j]
			}
			return token{kind: tOther}
		}
		switch {
		case tk.kind == tLower && tk.text == "import":
			if m := get(1); m.kind == tUpper && !seenImport[m.text] {
				seenImport[m.text] = true
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: m.text, Module: m.text, Name: kindModule, Line: m.line})
			}
		case tk.kind == tLower && (tk.text == "module" || tk.text == "effect"):
			// The module header; `port module` starts with port, below.
		case tk.kind == tLower && tk.text == "port":
			if n := get(1); n.kind == tLower && n.text != "module" && get(2).text == ":" {
				add(n.text, "port", n.line)
			}
		case tk.kind == tLower && tk.text == "type":
			if get(1).kind == tLower && get(1).text == "alias" {
				if n := get(2); n.kind == tUpper {
					add(n.text, "type", n.line)
				}
				continue
			}
			n := get(1)
			if n.kind != tUpper {
				continue
			}
			add(n.text, "type", n.line)
			// Constructors follow `=` and each `|` outside brackets.
			depth, next := 0, false
			for _, t := range decl[2:] {
				switch {
				case t.kind == tPunct && (t.text == "(" || t.text == "[" || t.text == "{"):
					depth++
				case t.kind == tPunct && (t.text == ")" || t.text == "]" || t.text == "}"):
					depth = max(depth-1, 0)
				case depth == 0 && t.kind == tPunct && (t.text == "=" || t.text == "|"):
					next = true
					continue
				case next && t.kind == tUpper:
					add(n.text+"."+t.text, "constructor", t.line)
				}
				next = false
			}
		case tk.kind == tLower && tk.text == "infix":
			// infix left 0 (|>) = apR
			for j := 1; j+2 < len(decl) && j < 6; j++ {
				if decl[j].text == "(" && decl[j+1].kind == tPunct && decl[j+2].text == ")" {
					add(decl[j+1].text, "operator", decl[j+1].line)
					break
				}
			}
		case tk.kind == tLower && !keyword[tk.text]:
			// f : Type, or f a b = ...; a qualified name is not a definition.
			if n := get(1); n.text == ":" || n.text == "=" || n.kind == tLower || n.kind == tOther && n.text == "_" ||
				n.kind == tPunct && (n.text == "(" || n.text == "{" || n.text == "[") || n.kind == tUpper || n.kind == tString || n.kind == tChar {
				if !isQualified(tk.text) && hasEquals(decl, n) {
					add(tk.text, "func", tk.line)
				}
			}
		}
	}
	ex.Symbols = symbols.List()
	return ex
}

// hasEquals reports whether a column-0 declaration is an annotation (`:` second)
// or a definition (an `=` operator follows the name and its arguments).
func hasEquals(decl []token, second token) bool {
	if second.text == ":" {
		return true
	}
	for _, t := range decl[1:] {
		if t.kind == tPunct && t.text == "=" {
			return true
		}
	}
	return false
}

func isQualified(name string) bool {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			return true
		}
	}
	return false
}

// keyword holds the words that start a column-0 line without declaring a value.
var keyword = map[string]bool{
	"module": true, "import": true, "exposing": true, "as": true, "type": true, "alias": true, "port": true,
	"effect": true, "where": true, "infix": true, "if": true, "then": true, "else": true, "case": true,
	"of": true, "let": true, "in": true,
}
