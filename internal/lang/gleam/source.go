package gleam

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindModule     = "module" // import a/b/c
	kindErlang     = "erlang" // @external(erlang, "mod", "fun")
	kindJS         = "js"     // @external(javascript, "./ffi.mjs", "fun")
	kindDependency = "dep"    // a gleam.toml dependency
	kindLocked     = "locked" // a manifest.toml package
)

type tokenKind uint8

const (
	tIdentifier  tokenKind = iota // lower-case name, keyword or discard
	tUpper                        // Upper-case name: a type or constructor
	tString                       // "..." with the quotes stripped, escapes kept
	tPunctuation                  // one or two punctuation bytes
	tOther                        // numbers and anything else
)

type token struct {
	kind tokenKind
	text string
	line int
}

// lex splits Gleam source into tokens. Comments (//, ///, ////) are dropped;
// strings may span lines and end at an unescaped quote or at the end of input.
// Token texts are slices of one string copy of source.
//
// Implements: REQ-GLEAM-010
func lex(source []byte) []token {
	s := string(source)
	var out []token
	line := 1
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '"':
			start, startLine := i+1, line
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				if s[i] == '\n' {
					line++
				}
				i++
			}
			out = append(out, token{tString, s[start:min(i, len(s))], startLine})
			i++
		case isLower(c) || c == '_':
			j := i + 1
			for j < len(s) && isWord(s[j]) {
				j++
			}
			out = append(out, token{tIdentifier, s[i:j], line})
			i = j
		case c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(s) && isWord(s[j]) {
				j++
			}
			out = append(out, token{tUpper, s[i:j], line})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(s) && (isWord(s[j]) || s[j] == '.' && j+1 < len(s) && s[j+1] >= '0' && s[j+1] <= '9') {
				j++
			}
			out = append(out, token{tOther, s[i:j], line})
			i = j
		case c == '-' && i+1 < len(s) && s[i+1] == '>', c == '.' && i+1 < len(s) && s[i+1] == '.':
			out = append(out, token{tPunctuation, s[i : i+2], line})
			i += 2
		case c < 0x80:
			out = append(out, token{tPunctuation, s[i : i+1], line})
			i++
		default:
			i++ // a byte of a UTF-8 sequence outside strings: not Gleam
		}
	}
	return out
}

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

func isWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// extractSource reads a module's imports, its @external targets and its
// top-level definitions in one pass over the tokens. Brackets of all kinds are
// counted together; a definition is top level when nothing is open. Closers
// never take the depth below zero, so a stray one costs nothing.
//
// Implements: REQ-GLEAM-002, REQ-GLEAM-003, REQ-GLEAM-004, REQ-GLEAM-010
func extractSource(source []byte) *lang.Extraction {
	x := &extractor{tokens: lex(source), extraction: &lang.Extraction{}, seen: map[string]bool{}, typeBody: -1}
	for x.i = 0; x.i < len(x.tokens); x.i++ {
		t := x.tokens[x.i]
		switch {
		case t.kind == tPunctuation:
			x.punctuation(t)
		case x.typeBody >= 0 && x.depth == x.typeBody && t.kind == tUpper && x.at(x.i-1).text != ".":
			x.symbols.Add(x.owner+"."+t.text, "constructor", t.line)
		case x.depth == 0 && t.kind == tIdentifier:
			x.definition(t)
		}
	}
	x.extraction.Symbols = x.symbols.List()
	return x.extraction
}

// extractor is extractSource's walk: where it is (i), how many brackets are open,
// and the custom type whose constructors it is reading, if any.
type extractor struct {
	tokens   []token
	i        int
	depth    int
	typeBody int    // depth inside the current custom type's braces, else -1
	owner    string // that type's name

	extraction *lang.Extraction
	symbols    lang.SymbolSet
	seen       map[string]bool
}

func (x *extractor) at(i int) token {
	if i < len(x.tokens) {
		return x.tokens[i]
	}
	return token{kind: tOther}
}

func (x *extractor) addImport(kind, spec, module string, line int) {
	key := kind + "\x00" + module
	if module == "" || x.seen[key] {
		return
	}
	x.seen[key] = true
	x.extraction.Imports = append(x.extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

// punctuation counts brackets, ends a custom type's body, and reads a top-level
// @external(target, "module", ...).
func (x *extractor) punctuation(t token) {
	switch t.text {
	case "{", "(", "[":
		x.depth++
	case "}", ")", "]":
		if x.depth > 0 {
			x.depth--
		}
		if x.depth < x.typeBody {
			x.typeBody, x.owner = -1, ""
		}
	case "@":
		if x.depth != 0 {
			return
		}
		if n := x.at(x.i + 1); n.kind != tIdentifier || n.text != "external" || x.at(x.i+2).text != "(" {
			return
		}
		target, module := x.at(x.i+3), x.at(x.i+5)
		if x.at(x.i+4).text != "," || module.kind != tString {
			return
		}
		switch target.text {
		case "erlang":
			x.addImport(kindErlang, "erlang:"+module.text, module.text, module.line)
		case "javascript":
			x.addImport(kindJS, module.text, module.text, module.line)
		}
	}
}

// definition reads what a top-level name starts: an import, a function, a
// constant, a type or an old-style external function.
func (x *extractor) definition(t token) {
	switch t.text {
	case "import":
		x.i = readImport(x.tokens, x.i+1, x.addImport) - 1 // the loop's i++ lands after the import
	case "fn":
		if n := x.at(x.i + 1); n.kind == tIdentifier {
			x.symbols.Add(n.text, "func", n.line)
		}
	case "const":
		if n := x.at(x.i + 1); n.kind == tIdentifier {
			x.symbols.Add(n.text, "const", n.line)
		}
	case "type":
		x.typeDefinition()
	case "external":
		x.external()
	}
}

// typeDefinition reads a type's name and, when a body follows its parameters,
// has the walk read the constructors in it; `=` is an alias and ends it.
func (x *extractor) typeDefinition() {
	n := x.at(x.i + 1)
	if n.kind != tUpper {
		return
	}
	x.symbols.Add(n.text, "type", n.line)
	j := x.i + 2
	if x.at(j).text == "(" {
		for d := 0; j < len(x.tokens); j++ {
			if x.tokens[j].kind != tPunctuation {
				continue
			}
			if x.tokens[j].text == "(" {
				d++
			} else if x.tokens[j].text == ")" {
				if d--; d == 0 {
					j++
					break
				}
			}
		}
	}
	if t := x.at(j); t.kind == tPunctuation && t.text == "{" {
		x.typeBody, x.owner = x.depth+1, n.text
	}
}

// external reads Gleam before 0.30: external fn f(a) -> b = "module" "function".
func (x *extractor) external() {
	if x.at(x.i+1).text != "fn" || x.at(x.i+2).kind != tIdentifier {
		return
	}
	x.symbols.Add(x.at(x.i+2).text, "func", x.at(x.i+2).line)
	x.i += 2 // past fn and the name, so the fn case does not count it again
	for j, d := x.i+1, 0; j < len(x.tokens) && j < x.i+254; j++ {
		t := x.tokens[j]
		if t.text == "(" {
			d++
		} else if t.text == ")" {
			d--
		} else if d == 0 && t.text == "=" && x.at(j+1).kind == tString {
			m := x.at(j + 1)
			if jsPath(m.text) {
				x.addImport(kindJS, m.text, m.text, m.line)
			} else {
				x.addImport(kindErlang, "erlang:"+m.text, m.text, m.line)
			}
			return
		} else if d == 0 && (t.text == "fn" || t.text == "pub" || t.text == "import") {
			return
		}
	}
}

// readImport reads `a/b/c`, an optional `.{type T, f as g}` list and an optional
// `as name` after an import keyword, and returns the index of the next token.
func readImport(tokens []token, i int, add func(kind, spec, module string, line int)) int {
	if i >= len(tokens) || tokens[i].kind != tIdentifier {
		return i
	}
	line := tokens[i].line
	var b strings.Builder
	b.WriteString(tokens[i].text)
	i++
	for i+1 < len(tokens) && tokens[i].text == "/" && tokens[i+1].kind == tIdentifier && tokens[i+1].line == tokens[i].line {
		b.WriteByte('/')
		b.WriteString(tokens[i+1].text)
		i += 2
	}
	module := b.String()
	add(kindModule, module, module, line)
	if i+1 < len(tokens) && tokens[i].text == "." && tokens[i+1].text == "{" {
		// The unqualified list holds names, `type`, `as` and commas and ends at the
		// first }. A keyword that starts a definition means it was never closed.
		for i += 2; i < len(tokens) && tokens[i].text != "}"; i++ {
			if t := tokens[i]; t.kind == tIdentifier && (t.text == "import" || t.text == "pub" || t.text == "fn" || t.text == "const") || t.text == "@" {
				return i
			}
		}
		i++
	}
	if i+1 < len(tokens) && tokens[i].text == "as" {
		i += 2
	}
	return i
}

// jsPath reports whether an old-style external's module string names a
// JavaScript file rather than an Erlang module.
func jsPath(s string) bool {
	return strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || strings.HasSuffix(s, ".mjs") || strings.HasSuffix(s, ".js")
}
