package purescript

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindModule     = "module" // import A.B as C (x, T(..))
	kindFFI        = "ffi"    // a foreign import: the module's JavaScript companion
	kindDependency = "dep"    // a package a manifest lists as a dependency
	kindExtra      = "extra"  // a package a workspace or package set adds or overrides
	kindLock       = "lock"   // a package spago.lock records
)

type tokenKind uint8

const (
	tLower       tokenKind = iota // a lower-case name or keyword, possibly qualified (Map.lookup)
	tUpper                        // an upper-case name, possibly qualified (Data.Map, M.Just)
	tString                       // "..." or """...""": never a name
	tCharacter                    // 'x'
	tPunctuation                  // a bracket, a comma, a backtick or a run of operator characters
	tOther                        // numbers and anything else
)

type token struct {
	kind   tokenKind
	text   string
	line   int
	column int // byte offset from the start of the line
	// first is true for a token that starts its line (after indentation).
	first bool
}

// lex splits PureScript source into tokens. `--` comments and `{- -}` comments
// (which do not nest in PureScript) are dropped; strings ("..." with escapes and
// gaps, raw """...""") and characters are single tokens, so nothing inside them
// can look like a declaration or an import. A qualified name (Data.Map.lookup,
// M.Just) is one token; identifiers take primes (foldl'). `∷` is `::`. Token
// texts are slices of one string copy of source.
//
// Implements: REQ-PURESCRIPT-010
func lex(source []byte) []token {
	s := string(source)
	var out []token
	line, lineStart := 1, 0
	first := true // no token yet on this line
	emit := func(kind tokenKind, text string, start, at int) {
		out = append(out, token{kind: kind, text: text, line: at, column: start - lineStart, first: first})
		first = false
	}
	newline := func(i int) {
		line, lineStart, first = line+1, i+1, true
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			newline(i)
			i++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			i++
		case c == '-' && at(s, i+1) == '-' && lineComment(s, i):
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '{' && at(s, i+1) == '-':
			// Block comments do not nest: the first -} ends one.
			for i += 2; i < len(s) && !(s[i] == '-' && at(s, i+1) == '}'); i++ {
				if s[i] == '\n' {
					line, lineStart = line+1, i+1
				}
			}
			i = min(i+2, len(s))
		case c == '"':
			start, startLine := i, line
			startColumn := i - lineStart
			if at(s, i+1) == '"' && at(s, i+2) == '"' {
				// A raw string: no escapes, it ends at the next """.
				for i += 3; i < len(s) && !(s[i] == '"' && at(s, i+1) == '"' && at(s, i+2) == '"'); i++ {
					if s[i] == '\n' {
						line, lineStart = line+1, i+1
					}
				}
				i = min(i+3, len(s))
			} else {
				// A string ends at its quote or, unterminated, at the line; a
				// backslash escapes the next character, a line break included (a
				// string gap: "a\
				//   \b").
				for i++; i < len(s) && s[i] != '"' && s[i] != '\n'; i++ {
					if s[i] == '\\' {
						for i++; i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n'); i++ {
							if s[i] == '\n' {
								line, lineStart = line+1, i+1
							}
						}
						if i >= len(s) {
							break
						}
					}
				}
				if i < len(s) && s[i] == '"' {
					i++
				}
				i = min(i, len(s))
			}
			out = append(out, token{kind: tString, text: s[start:i], line: startLine, column: startColumn, first: first})
			first = false
		case c == '\'':
			// Identifiers absorb their primes, so a quote here starts a character:
			// 'x', '\n', '\'', '\x41'. Without a closing quote soon on the line it is
			// just punctuation.
			j := i + 1
			if at(s, j) == '\\' {
				j += 2
			} else if j < len(s) {
				_, n := utf8.DecodeRuneInString(s[j:])
				j += n
			}
			for k := 0; j < len(s) && s[j] != '\'' && s[j] != '\n' && k < 8; k++ {
				j++
			}
			if at(s, j) == '\'' {
				emit(tCharacter, s[i:j+1], i, line)
				i = j + 1
			} else {
				emit(tPunctuation, s[i:i+1], i, line)
				i++
			}
		case isIdentifierStart(s, i):
			start := i
			kind := tLower
			for {
				r, _ := utf8.DecodeRuneInString(s[i:])
				upper := unicode.IsUpper(r)
				i = identifierEnd(s, i)
				kind = tLower
				if upper {
					kind = tUpper
				}
				// Only an upper-case segment qualifies what follows: Map.lookup, not
				// record.field.
				if upper && at(s, i) == '.' && i+1 < len(s) && isIdentifierStart(s, i+1) {
					i++
					continue
				}
				break
			}
			emit(kind, s[start:i], start, line)
		case c >= '0' && c <= '9':
			start := i
			for i < len(s) && (isWord(s[i]) || s[i] == '.' && at(s, i+1) >= '0' && at(s, i+1) <= '9' ||
				(s[i] == '-' || s[i] == '+') && (s[i-1] == 'e' || s[i-1] == 'E') && !strings.HasPrefix(s[start:], "0x")) {
				i++
			}
			emit(tOther, s[start:i], start, line)
		case c == '(' || c == ')' || c == '[' || c == ']' || c == '{' || c == '}' || c == ',' || c == '`' || c == ';':
			emit(tPunctuation, s[i:i+1], i, line)
			i++
		case isSymbol(c):
			start := i
			for i < len(s) && isSymbol(s[i]) {
				i++
			}
			emit(tPunctuation, s[start:i], start, line)
		case c < 0x80:
			emit(tOther, s[i:i+1], i, line)
			i++
		default:
			r, n := utf8.DecodeRuneInString(s[i:])
			switch r {
			case '∷':
				emit(tPunctuation, "::", i, line)
			case '→', '⇒', '←', '∀':
				emit(tPunctuation, s[i:i+n], i, line)
			}
			i += n // other non-ASCII outside strings and comments: not a name
		}
	}
	return out
}

// lineComment reports whether the dashes at i start a comment: a run of two or
// more dashes not followed by another operator character (`-->` is an operator).
func lineComment(s string, i int) bool {
	j := i
	for j < len(s) && s[j] == '-' {
		j++
	}
	return j >= len(s) || !isSymbol(s[j])
}

func at(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func isWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// isIdentifierStart reports whether an identifier starts at i: a letter or `_`.
func isIdentifierStart(s string, i int) bool {
	c := s[i]
	if c < 0x80 {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return unicode.IsLetter(r)
}

// identifierEnd is the end of the identifier segment starting at i: letters, digits,
// `_` and primes.
func identifierEnd(s string, i int) int {
	for i < len(s) {
		c := s[i]
		if isWord(c) || c == '\'' {
			i++
			continue
		}
		if c >= 0x80 {
			r, n := utf8.DecodeRuneInString(s[i:])
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				i += n
				continue
			}
		}
		break
	}
	return i
}

func isSymbol(c byte) bool {
	switch c {
	case '+', '-', '*', '/', '=', '<', '>', '|', '&', '^', '%', '!', '?', ':', '.', '~', '$', '#', '@', '\\':
		return true
	}
	return false
}

// declarations slices a module's tokens into its top-level declarations. The
// module header runs to its `where` (an export list may be written anywhere);
// after it, a declaration starts at a token first on its line in the column of
// the first declaration or further left (PureScript's layout rule), and runs to
// the next one.
func declarations(tokens []token) [][]token {
	i, top := 0, 0
	if len(tokens) > 0 && tokens[0].kind == tLower && tokens[0].text == "module" {
		depth := 0
		for i = 1; i < len(tokens); i++ {
			t := tokens[i]
			if t.kind == tPunctuation {
				depth = bracket(t.text, depth)
			}
			if depth == 0 && t.kind == tLower && t.text == "where" {
				i++
				break
			}
		}
		if i < len(tokens) {
			top = tokens[i].column
		}
	}
	var out [][]token
	for i < len(tokens) {
		end := i + 1
		for end < len(tokens) && !(tokens[end].first && tokens[end].column <= top) {
			end++
		}
		out = append(out, tokens[i:end])
		i = end
	}
	return out
}

// bracket updates a bracket depth for one punctuation token; a stray closer
// leaves it at 0.
func bracket(p string, depth int) int {
	switch p {
	case "(", "[", "{":
		return depth + 1
	case ")", "]", "}":
		return max(depth-1, 0)
	}
	return depth
}

// extractSource reads a module's imports and its top-level declarations.
//
// Symbols: values and functions (a type signature and its definition count
// once), `data` and `newtype` types with their constructors as `Type.Ctor`, type
// synonyms, classes with their members as `Class.member`, named instances
// (`instance showFoo :: Show Foo`, derived ones too), `foreign import` values and
// `foreign import data` types, and the operators `infix` declarations define.
// Each `foreign import` of a value makes the module need its JavaScript
// companion, one import of kind ffi.
//
// Implements: REQ-PURESCRIPT-002, REQ-PURESCRIPT-003, REQ-PURESCRIPT-010
func extractSource(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	seenImport := map[string]bool{}
	defined := map[string]bool{}
	add := func(name, kind string, line int) {
		if name != "" && !defined[name] {
			defined[name] = true
			symbols.Add(name, kind, line)
		}
	}
	ffi := 0
	for _, declaration := range declarations(lex(source)) {
		get := func(j int) token {
			if j < len(declaration) {
				return declaration[j]
			}
			return token{kind: tOther}
		}
		token := declaration[0]
		switch {
		case token.kind == tLower && token.text == "import":
			if m := get(1); m.kind == tUpper && !seenImport[m.text] {
				seenImport[m.text] = true
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: m.text, Module: m.text, Name: kindModule, Line: m.line})
			}
		case token.kind == tLower && token.text == "foreign" && get(1).text == "import":
			switch n := get(2); {
			case n.kind == tLower && (n.text == "data" || n.text == "kind") && get(3).kind == tUpper:
				add(get(3).text, "type", get(3).line)
			case n.kind == tLower && !isQualified(n.text) && get(3).text == "::":
				add(n.text, "foreign", n.line)
				if ffi == 0 {
					ffi = token.line
				}
			}
		case token.kind == tLower && (token.text == "data" || token.text == "newtype"):
			n := get(1)
			if n.kind != tUpper || isQualified(n.text) {
				continue
			}
			add(n.text, "type", n.line)
			// Constructors follow `=` and each `|` outside brackets.
			depth, next := 0, false
			for _, t := range declaration[2:] {
				switch {
				case t.kind == tPunctuation && (t.text == "(" || t.text == "[" || t.text == "{"):
					depth++
				case t.kind == tPunctuation && (t.text == ")" || t.text == "]" || t.text == "}"):
					depth = max(depth-1, 0)
				case depth == 0 && t.kind == tPunctuation && (t.text == "=" || t.text == "|"):
					next = true
					continue
				case next && t.kind == tUpper && !isQualified(t.text):
					add(n.text+"."+t.text, "constructor", t.line)
				}
				next = false
			}
		case token.kind == tLower && token.text == "type":
			if n := get(1); n.kind == tUpper && !isQualified(n.text) {
				add(n.text, "type", n.line) // `type role T ...` has a lower name after type
			}
		case token.kind == tLower && token.text == "class":
			readClass(declaration, add)
		case token.kind == tLower && (token.text == "instance" || token.text == "derive" || token.text == "else"):
			// instance name :: C T, derive (newtype) instance name :: C T, else
			// instance name :: C T. 0.15 instances may be unnamed: no symbol.
			for j := 0; j < len(declaration) && j < 4; j++ {
				if declaration[j].kind == tLower && declaration[j].text == "instance" {
					if n := get(j + 1); n.kind == tLower && !isQualified(n.text) && get(j+2).text == "::" {
						add(n.text, "instance", n.line)
					}
					break
				}
			}
		case token.kind == tLower && (token.text == "infix" || token.text == "infixl" || token.text == "infixr"):
			// infixl 6 add as +, infixr 0 type Tuple as /\
			for j := 2; j+1 < len(declaration) && j < 6; j++ {
				if declaration[j].kind == tLower && declaration[j].text == "as" && declaration[j+1].kind == tPunctuation {
					add(declaration[j+1].text, "operator", declaration[j+1].line)
					break
				}
			}
		case token.kind == tLower && !keyword[token.text] && !isQualified(token.text):
			// f :: Type, or f a b = ... / f x | guard = ...
			if get(1).text == "::" || definition(declaration) {
				add(token.text, "func", token.line)
			}
		}
	}
	if ffi > 0 {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "foreign import", Name: kindFFI, Line: ffi})
	}
	extraction.Symbols = symbols.List()
	return extraction
}

// readClass records a class and its members. The class is the name after `<=`
// when it has superclasses (`class (Eq a) <= Ord a where`), else the first
// upper-case name; members are the names first on their lines after `where` that
// a `::` follows.
func readClass(declaration []token, add func(name, kind string, line int)) {
	name, where := token{}, len(declaration)
	depth := 0
	for j := 1; j < len(declaration); j++ {
		t := declaration[j]
		if t.kind == tPunctuation {
			depth = bracket(t.text, depth)
			if depth == 0 && (t.text == "<=" || t.text == "⇐") {
				name = token{}
			}
			continue
		}
		if depth == 0 && t.kind == tLower && t.text == "where" {
			where = j
			break
		}
		if depth == 0 && name.text == "" && t.kind == tUpper && !isQualified(t.text) {
			name = t
		}
	}
	if name.text == "" {
		return
	}
	add(name.text, "class", name.line)
	for j := where + 1; j+1 < len(declaration); j++ {
		if t := declaration[j]; t.first && t.kind == tLower && !isQualified(t.text) && declaration[j+1].text == "::" {
			add(name.text+"."+t.text, "method", t.line)
		}
	}
}

// definition reports whether a declaration starting with a name defines it: an
// `=` or a guard `|` follows at bracket depth 0.
func definition(declaration []token) bool {
	depth := 0
	for _, t := range declaration[1:] {
		if t.kind != tPunctuation {
			continue
		}
		if depth == 0 && (t.text == "=" || t.text == "|") {
			return true
		}
		depth = bracket(t.text, depth)
	}
	return false
}

func isQualified(name string) bool { return strings.IndexByte(name, '.') >= 0 }

// keyword holds the words that start a top-level line without declaring a value.
var keyword = map[string]bool{
	"module": true, "import": true, "foreign": true, "data": true, "newtype": true, "type": true, "class": true,
	"instance": true, "derive": true, "else": true, "infix": true, "infixl": true, "infixr": true, "where": true,
	"let": true, "in": true, "if": true, "then": true, "case": true, "of": true, "do": true, "ado": true,
	"as": true, "hiding": true, "forall": true, "true": true, "false": true,
}
