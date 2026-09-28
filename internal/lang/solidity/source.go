package solidity

import (
	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// The scanner reads a token stream as a sequence of declarations: at the
// source unit's level pragmas, imports, contracts, interfaces, libraries and
// the free functions, structs, enums, events, errors, user-defined value
// types and constants Solidity allows there; inside a contract its members.
// Function and modifier bodies - and the assembly blocks in them - are
// skipped whole by their matched braces, so nothing in them is read.

// maxCloserSearch is how many open brackets a closer looks past for its
// opener: a stray ')' in broken code is ignored instead of closing a
// contract's '{' far up the stack.
const maxCloserSearch = 8

type scanner struct {
	tokens  []token
	match   []int // index of each opener's closer (len(tokens) when unclosed); -1 otherwise
	symbols lang.SymbolSet
	imports []lang.RawImport
	seen    map[string]bool
}

// scanSource extracts a Solidity file's imports and declarations.
//
// Implements: REQ-SOLIDITY-002, REQ-SOLIDITY-003, REQ-SOLIDITY-010
func scanSource(source []byte) *lang.Extraction {
	s := &scanner{tokens: lex(source), seen: map[string]bool{}}
	s.matchBrackets()
	s.block(0, len(s.tokens), "")
	return &lang.Extraction{Imports: s.imports, Symbols: s.symbols.List()}
}

func closerOf(c string) string {
	switch c {
	case "(":
		return ")"
	case "[":
		return "]"
	case "{":
		return "}"
	}
	return ""
}

// matchBrackets pairs every opener with its closer in one pass.
func (s *scanner) matchBrackets() {
	s.match = make([]int, len(s.tokens))
	var stack []int
	for i, t := range s.tokens {
		s.match[i] = -1
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			for k := len(stack) - 1; k >= 0 && k >= len(stack)-maxCloserSearch; k-- {
				if closerOf(s.tokens[stack[k]].text) == t.text {
					for _, o := range stack[k:] {
						s.match[o] = i // openers left open close with it
					}
					stack = stack[:k]
					break
				}
			}
		}
	}
	for _, o := range stack {
		s.match[o] = len(s.tokens)
	}
}

func (s *scanner) punctuation(i int, p string) bool {
	return i < len(s.tokens) && s.tokens[i].kind == tPunctuation && s.tokens[i].text == p
}

func (s *scanner) identifier(i int) (string, bool) {
	if i < len(s.tokens) && s.tokens[i].kind == tIdentifier {
		return s.tokens[i].text, true
	}
	return "", false
}

// skip steps over token i, jumping an opener to just past its closer.
func (s *scanner) skip(i int) int {
	if m := s.match[i]; m >= 0 {
		return m + 1
	}
	return i + 1
}

// statementEnd returns the index just past the ';' ending the statement at
// i, or end - brackets jumped as a whole, stopping before a '}' or a '{'
// that is not inside brackets (a missing semicolon must not swallow the next
// declaration's body).
func (s *scanner) statementEnd(i, end int) int {
	for i < end {
		t := s.tokens[i]
		if t.kind == tPunctuation {
			switch t.text {
			case ";":
				return i + 1
			case "{", "}":
				return i
			}
		}
		i = s.skip(i)
	}
	return end
}

// bodyEnd returns the index just past a function's header and body: its
// '{' ... '}' or its ';'.
func (s *scanner) bodyEnd(i, end int) int {
	for i < end {
		t := s.tokens[i]
		if t.kind == tPunctuation {
			switch t.text {
			case ";":
				return i + 1
			case "{":
				return min(s.match[i]+1, end)
			case "}":
				return i
			}
		}
		i = s.skip(i)
	}
	return end
}

// headerOpen finds the '{' opening a contract's or struct's body, or -1 when a
// ';' or the end comes first.
func (s *scanner) headerOpen(i, end int) int {
	for i < end {
		t := s.tokens[i]
		if t.kind == tPunctuation {
			switch t.text {
			case "{":
				return i
			case ";", "}":
				return -1
			}
		}
		i = s.skip(i)
	}
	return -1
}

func member(owner, name string) string {
	if owner == "" {
		return name
	}
	return owner + "." + name
}

// block reads the declarations in [i, end), owner being the contract they
// belong to ("" at the source unit's level).
func (s *scanner) block(i, end int, owner string) {
	for i < end {
		t := s.tokens[i]
		if t.kind != tIdentifier {
			i = s.skip(i)
			continue
		}
		next, _ := s.identifier(i + 1)
		switch t.text {
		case "pragma", "using":
			i = s.statementEnd(i, end)
		case "import":
			i = s.importAt(i, end)
		case "abstract", "contract", "interface", "library":
			j := i
			if t.text == "abstract" {
				if next != "contract" {
					i = s.statementEnd(i, end)
					continue
				}
				j++
			}
			name, ok := s.identifier(j + 1)
			open := s.headerOpen(j+1, end)
			if !ok || open < 0 {
				i = s.statementEnd(i, end)
				continue
			}
			kind := "class"
			if s.tokens[j].text == "interface" {
				kind = "interface"
			}
			s.symbols.Add(name, kind, s.tokens[j+1].line)
			close := min(s.match[open], end)
			s.block(open+1, close, name)
			i = close + 1
		case "function":
			name := next
			switch {
			case name != "" && s.punctuation(i+2, "("):
				kind := "func"
				if owner != "" {
					kind = "method"
				}
				s.symbols.Add(member(owner, name), kind, s.tokens[i+1].line)
				i = s.bodyEnd(i+2, end)
			case s.punctuation(i+1, "("):
				// Before 0.6 a contract's fallback was `function () external {...}`;
				// with a ';' it is a state variable of a function type.
				j := s.bodyEnd(i+1, end)
				if owner != "" && j > 0 && j <= len(s.tokens) && s.punctuation(j-1, "}") {
					s.symbols.Add(member(owner, "fallback"), "method", t.line)
				}
				i = j
			default:
				i = s.statementEnd(i+1, end)
			}
		case "constructor", "fallback", "receive":
			if !s.punctuation(i+1, "(") {
				i = s.statement(i, end, owner)
				continue
			}
			s.symbols.Add(member(owner, t.text), "method", t.line)
			i = s.bodyEnd(i+1, end)
		case "modifier":
			if next != "" {
				s.symbols.Add(member(owner, next), "modifier", s.tokens[i+1].line)
			}
			i = s.bodyEnd(i+1, end)
		case "event", "error":
			if next == "" || !s.punctuation(i+2, "(") {
				i = s.statement(i, end, owner)
				continue
			}
			s.symbols.Add(member(owner, next), t.text, s.tokens[i+1].line)
			i = s.statementEnd(i+2, end)
		case "struct", "enum":
			open := s.headerOpen(i+1, end)
			if next == "" || open != i+2 {
				i = s.statement(i, end, owner)
				continue
			}
			s.symbols.Add(member(owner, next), t.text, s.tokens[i+1].line)
			i = min(s.match[open], end) + 1
		case "type":
			if is, _ := s.identifier(i + 2); next == "" || is != "is" {
				i = s.statement(i, end, owner)
				continue
			}
			s.symbols.Add(member(owner, next), "type", s.tokens[i+1].line)
			i = s.statementEnd(i+2, end)
		default:
			i = s.statement(i, end, owner)
		}
	}
}

// statement reads a variable declaration (or anything else) up to its ';',
// adding a constant's name.
func (s *scanner) statement(i, end int, owner string) int {
	stop := s.statementEnd(i, end)
	if stop == i {
		return s.skip(i) // a '{' or '}' where a declaration was expected
	}
	constant, name, line := false, "", 0
	for j := i; j < stop; j = s.skip(j) {
		t := s.tokens[j]
		if t.kind == tPunctuation && t.text == "=" {
			break
		}
		if t.kind == tIdentifier {
			if t.text == "constant" {
				constant = true
			} else {
				name, line = t.text, t.line
			}
		}
	}
	if constant && name != "" {
		s.symbols.Add(member(owner, name), "const", line)
	}
	return stop
}

// importAt reads an import directive in any of its forms:
//
//	import "path";
//	import "path" as X;
//	import * as X from "path";
//	import {A, B as C} from "path";
//
// The path is the string after `from`, else the first string.
//
// Implements: REQ-SOLIDITY-002
func (s *scanner) importAt(i, end int) int {
	var first, from *token
	j := i + 1
	for ; j < end && j < i+1+maxImportTokens; j++ {
		t := &s.tokens[j]
		if t.kind == tPunctuation && t.text == ";" {
			j++
			break
		}
		if t.kind == tIdentifier && declarationStart[t.text] {
			break // a missing ';': the next declaration starts here
		}
		if t.kind == tString {
			switch {
			case j > i+1 && s.tokens[j-1].kind == tIdentifier && s.tokens[j-1].text == "from":
				from = t
			case first == nil:
				first = t
			}
		}
	}
	p := first
	if from != nil {
		p = from
	}
	if p != nil && p.text != "" && !s.seen[p.text] {
		s.seen[p.text] = true
		s.imports = append(s.imports, lang.RawImport{Spec: p.text, Module: p.text, Line: p.line})
	}
	return j
}

// maxImportTokens bounds an import directive: `import {A, B, ...}` lists
// symbols, never thousands of them.
const maxImportTokens = 1024

// declarationStart are the words that begin a declaration at the source unit's level.
var declarationStart = map[string]bool{
	"import": true, "pragma": true, "contract": true, "abstract": true, "interface": true,
	"library": true, "function": true, "struct": true, "enum": true, "using": true,
}
