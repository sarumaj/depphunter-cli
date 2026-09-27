package gleam

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindModule = "module" // import a/b/c
	kindErlang = "erlang" // @external(erlang, "mod", "fun")
	kindJS     = "js"     // @external(javascript, "./ffi.mjs", "fun")
	kindDep    = "dep"    // a gleam.toml dependency
	kindLocked = "locked" // a manifest.toml package
)

type tokKind uint8

const (
	tIdent  tokKind = iota // lower-case name, keyword or discard
	tUpper                 // Upper-case name: a type or constructor
	tString                // "..." with the quotes stripped, escapes kept
	tPunct                 // one or two punctuation bytes
	tOther                 // numbers and anything else
)

type token struct {
	kind tokKind
	text string
	line int
}

// lex splits Gleam source into tokens. Comments (//, ///, ////) are dropped;
// strings may span lines and end at an unescaped quote or at the end of input.
// Token texts are slices of one string copy of src.
//
// Implements: REQ-GLEAM-010
func lex(src []byte) []token {
	s := string(src)
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
			out = append(out, token{tIdent, s[i:j], line})
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
			out = append(out, token{tPunct, s[i : i+2], line})
			i += 2
		case c < 0x80:
			out = append(out, token{tPunct, s[i : i+1], line})
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
func extractSource(src []byte) *lang.Extraction {
	tokens := lex(src)
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	seen := map[string]bool{}
	addImport := func(kind, spec, module string, line int) {
		key := kind + "\x00" + module
		if module == "" || seen[key] {
			return
		}
		seen[key] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	depth := 0
	typeBody := -1 // depth inside the current custom type's braces, else -1
	owner := ""
	at := func(i int) token {
		if i < len(tokens) {
			return tokens[i]
		}
		return token{kind: tOther}
	}
	for i := 0; i < len(tokens); i++ {
		tk := tokens[i]
		if tk.kind == tPunct {
			switch tk.text {
			case "{", "(", "[":
				depth++
			case "}", ")", "]":
				if depth > 0 {
					depth--
				}
				if depth < typeBody {
					typeBody, owner = -1, ""
				}
			case "@":
				if depth == 0 {
					if n := at(i + 1); n.kind == tIdent && n.text == "external" && at(i+2).text == "(" {
						target, module := at(i+3), at(i+5)
						if at(i+4).text == "," && module.kind == tString {
							switch target.text {
							case "erlang":
								addImport(kindErlang, "erlang:"+module.text, module.text, module.line)
							case "javascript":
								addImport(kindJS, module.text, module.text, module.line)
							}
						}
					}
				}
			}
			continue
		}
		if typeBody >= 0 && depth == typeBody && tk.kind == tUpper && at(i-1).text != "." {
			symbols.Add(owner+"."+tk.text, "constructor", tk.line)
			continue
		}
		if depth != 0 || tk.kind != tIdent {
			continue
		}
		switch tk.text {
		case "import":
			i = readImport(tokens, i+1, addImport) - 1 // the loop's i++ lands after the import
		case "fn":
			if n := at(i + 1); n.kind == tIdent {
				symbols.Add(n.text, "func", n.line)
			}
		case "const":
			if n := at(i + 1); n.kind == tIdent {
				symbols.Add(n.text, "const", n.line)
			}
		case "type":
			n := at(i + 1)
			if n.kind != tUpper {
				continue
			}
			symbols.Add(n.text, "type", n.line)
			// Skip type parameters, then a body opens the constructor list; `=` is an
			// alias and ends it.
			j := i + 2
			if at(j).text == "(" {
				for d := 0; j < len(tokens); j++ {
					if tokens[j].kind != tPunct {
						continue
					}
					if tokens[j].text == "(" {
						d++
					} else if tokens[j].text == ")" {
						if d--; d == 0 {
							j++
							break
						}
					}
				}
			}
			if t := at(j); t.kind == tPunct && t.text == "{" {
				typeBody, owner = depth+1, n.text
			}
		case "external":
			// Gleam before 0.30: external fn f(a) -> b = "module" "function"
			if at(i+1).text != "fn" || at(i+2).kind != tIdent {
				continue
			}
			symbols.Add(at(i+2).text, "func", at(i+2).line)
			i += 2 // past fn and the name, so the fn case does not count it again
			for j, d := i+1, 0; j < len(tokens) && j < i+254; j++ {
				t := tokens[j]
				if t.text == "(" {
					d++
				} else if t.text == ")" {
					d--
				} else if d == 0 && t.text == "=" && at(j+1).kind == tString {
					m := at(j + 1)
					if jsPath(m.text) {
						addImport(kindJS, m.text, m.text, m.line)
					} else {
						addImport(kindErlang, "erlang:"+m.text, m.text, m.line)
					}
					break
				} else if d == 0 && (t.text == "fn" || t.text == "pub" || t.text == "import") {
					break
				}
			}
		}
	}
	ex.Symbols = symbols.List()
	return ex
}

// readImport reads `a/b/c`, an optional `.{type T, f as g}` list and an optional
// `as name` after an import keyword, and returns the index of the next token.
func readImport(tokens []token, i int, add func(kind, spec, module string, line int)) int {
	if i >= len(tokens) || tokens[i].kind != tIdent {
		return i
	}
	line := tokens[i].line
	var b strings.Builder
	b.WriteString(tokens[i].text)
	i++
	for i+1 < len(tokens) && tokens[i].text == "/" && tokens[i+1].kind == tIdent && tokens[i+1].line == tokens[i].line {
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
			if t := tokens[i]; t.kind == tIdent && (t.text == "import" || t.text == "pub" || t.text == "fn" || t.text == "const") || t.text == "@" {
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
