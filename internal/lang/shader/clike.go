package shader

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
)

// Token kinds of the scanners.
const (
	tIdentifier  = iota + 1
	tPunctuation // one character, or "::"
	tString
	tNumber
)

type token struct {
	kind int
	text string
	line int
}

func (t token) is(text string) bool { return t.kind == tPunctuation && t.text == text }

// extractC reads a GLSL or HLSL file: the includes through the cpp plugin's
// directive scanner (comments, continued lines, #if 0), the definitions through a
// scan of the tokens outside the dead lines.
func extractC(source []byte, d int) *lang.Extraction {
	includes, dead := cpp.Directives(source)
	var set lang.SymbolSet
	s := &cScanner{tokens: lexC(source, dead), hlsl: d == hlsl, set: &set}
	s.declarations(0, len(s.tokens), "", 0)
	return &lang.Extraction{Imports: includes, Symbols: set.List()}
}

// lexC splits a C-like source into tokens. Comments, preprocessor lines and the
// lines the directive scanner found dead are dropped.
func lexC(source []byte, dead []bool) []token {
	var tokens []token
	line, lineStart := 1, true
	for i := 0; i < len(source); {
		c := source[i]
		if lineStart && c != '\n' && line < len(dead) && dead[line] {
			i = chars.LineEnd(source, i)
			continue
		}
		switch {
		case c == '\n':
			line, lineStart = line+1, true
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
			continue
		case c == '#' && lineStart:
			// A directive, continued lines included.
			for i < len(source) && source[i] != '\n' {
				if source[i] == '\\' && i+1 < len(source) && source[i+1] == '\n' {
					line++
					i++
				} else if source[i] == '\\' && i+2 < len(source) && source[i+1] == '\r' && source[i+2] == '\n' {
					line++
					i += 2
				}
				i++
			}
			continue
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			i = chars.LineEnd(source, i)
			continue
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			i += 2
			for i < len(source) && !(source[i] == '*' && i+1 < len(source) && source[i+1] == '/') {
				if source[i] == '\n' {
					line++
				}
				i++
			}
			i += 2
			continue
		}
		lineStart = false
		start, at := i, line
		switch {
		case chars.IsIdentStart(c):
			for i < len(source) && identifierByte(source[i]) {
				i++
			}
			tokens = append(tokens, token{tIdentifier, string(source[start:i]), at})
		case c >= '0' && c <= '9' || c == '.' && i+1 < len(source) && source[i+1] >= '0' && source[i+1] <= '9':
			for i < len(source) && (identifierByte(source[i]) || source[i] == '.') {
				i++
			}
			tokens = append(tokens, token{tNumber, string(source[start:i]), at})
		case c == '"' || c == '\'':
			i++
			for i < len(source) && source[i] != c && source[i] != '\n' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
			i++
			tokens = append(tokens, token{tString, "", at})
		case c == ':' && i+1 < len(source) && source[i+1] == ':':
			i += 2
			tokens = append(tokens, token{tPunctuation, "::", at})
		default:
			i++
			tokens = append(tokens, token{tPunctuation, string(c), at})
		}
	}
	return tokens
}

func identifierByte(c byte) bool { return chars.IsIdentStart(c) || c >= '0' && c <= '9' }

// blockQualifiers open a GLSL interface block (`uniform Camera { ... } cam;`),
// ray tracing and mesh shading ones included.
var blockQualifiers = map[string]bool{
	"uniform": true, "buffer": true, "in": true, "out": true, "shared": true, "patch": true,
	"rayPayloadEXT": true, "rayPayloadInEXT": true, "hitAttributeEXT": true, "callableDataEXT": true,
	"callableDataInEXT": true, "shaderRecordEXT": true, "taskPayloadSharedEXT": true,
	"rayPayloadNV": true, "rayPayloadInNV": true, "hitAttributeNV": true, "callableDataNV": true,
	"callableDataInNV": true, "shaderRecordNV": true, "taskNV": true, "perprimitiveEXT": true,
}

// notNames are keywords a statement head may end in before a parenthesis.
var notNames = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true, "layout": true,
	"register": true, "packoffset": true, "sizeof": true, "do": true,
}

// maxDepth bounds nested namespaces.
const maxDepth = 32

type cScanner struct {
	tokens []token
	hlsl   bool
	set    *lang.SymbolSet
}

// declarations reads the declarations of tokens[i:end] (a file, or a namespace's body):
// statements end at a ';' or a body in braces at nesting depth 0.
//
// Implements: REQ-SHADER-003
func (s *cScanner) declarations(i, end int, owner string, depth int) {
	for i < end {
		start := i
		nest := 0 // () and [] inside the statement head
	head:
		for ; i < end; i++ {
			t := s.tokens[i]
			if t.kind != tPunctuation {
				continue
			}
			switch t.text {
			case "(", "[":
				nest++
			case ")", "]":
				nest = max(nest-1, 0)
			case ";", "{", "}":
				if nest == 0 {
					break head
				}
			}
		}
		if i >= end {
			return
		}
		if s.tokens[i].text != "{" {
			i++ // ';', or a stray '}'
			continue
		}
		head := s.tokens[start:i]
		rbrace := s.skipGroup(i, end)
		body := i + 1
		i = rbrace + 1
		name, kind := s.classify(head)
		switch kind {
		case "":
			if hasTopLevel(head, "=") {
				i = s.toSemicolon(i, end) // an initializer list: `const float w[] = {...};`
			}
			continue
		case "namespace":
			s.set.Add(owner+name, kind, head[0].line)
			if depth < maxDepth {
				s.declarations(body, rbrace, owner+name+".", depth+1)
			}
			continue
		case "struct", "block", "cbuffer", "technique":
			s.set.Add(owner+name, kind, s.lineOf(head, name))
			i = s.declarators(i, end)
		default:
			// An all-capitals name is a function too (entry points PS, VS, CS); a
			// macro invocation has no return type before it.
			s.set.Add(owner+strings.ReplaceAll(name, "::", "."), kind, s.lineOf(head, name))
		}
	}
}

// classify names what a statement head followed by a body declares: a struct,
// an interface block, a cbuffer, a technique, a namespace or a function; "" for
// anything else.
func (s *cScanner) classify(head []token) (name, kind string) {
	if len(head) == 0 {
		return "", ""
	}
	for j, t := range head {
		if t.kind != tIdentifier {
			continue
		}
		switch w := t.text; {
		case w == "struct" || w == "class" || w == "interface":
			if n := identifierAt(head, j+1); n != "" {
				return n, "struct"
			}
			return "", ""
		case s.hlsl && (w == "cbuffer" || w == "tbuffer"):
			return identifierAt(head, j+1), "cbuffer"
		case s.hlsl && strings.HasPrefix(w, "technique") && (len(w) == 9 || w == "technique10" || w == "technique11"):
			return identifierAt(head, j+1), "technique"
		case w == "namespace":
			return identifierAt(head, j+1), "namespace"
		}
	}
	// A GLSL interface block: `layout(std140) uniform Camera`.
	if last := head[len(head)-1]; !s.hlsl && last.kind == tIdentifier && len(head) >= 2 && qualified(head) {
		return last.text, "block"
	}
	// A function: a name and its parameters, then at most an HLSL semantic
	// (`: SV_Target`) or a trailing qualifier.
	parameters := -1 // the ')' ending the parameters
	nest := 0
	for j, t := range head {
		switch {
		case t.is("(") || t.is("["):
			nest++
		case t.is(")") || t.is("]"):
			nest--
			if nest == 0 && t.is(")") {
				parameters = j
			}
		case nest == 0 && t.is("="):
			return "", ""
		}
	}
	if parameters < 0 || !tail(head[parameters+1:]) {
		return "", ""
	}
	open := matching(head, parameters)
	if open < 1 || head[open-1].kind != tIdentifier || notNames[head[open-1].text] {
		return "", ""
	}
	j := open - 1
	name = head[j].text
	for j >= 2 && head[j-1].is("::") && head[j-2].kind == tIdentifier {
		j -= 2
		name = head[j].text + "::" + name
	}
	if j == 0 {
		return "", "" // a call, not a definition: a function needs a return type
	}
	return name, "func"
}

// qualified reports whether a head holds a storage qualifier of an interface
// block outside parentheses.
func qualified(head []token) bool {
	nest := 0
	for _, t := range head[:len(head)-1] {
		switch {
		case t.is("("):
			nest++
		case t.is(")"):
			nest--
		case nest == 0 && t.kind == tIdentifier && blockQualifiers[t.text]:
			return true
		}
	}
	return false
}

// tail reports whether what follows a function's parameters is only an HLSL
// semantic (`: SV_Target`, `: register(b0)`) or qualifiers.
func tail(rest []token) bool {
	for j := 0; j < len(rest); j++ {
		t := rest[j]
		switch {
		case t.is(":") || t.kind == tIdentifier:
		case t.is("("):
			for j < len(rest) && !rest[j].is(")") {
				j++
			}
		default:
			return false
		}
	}
	return true
}

// matching is the index of the '(' of the ')' at head[params].
func matching(head []token, parameters int) int {
	nest := 0
	for j := parameters; j >= 0; j-- {
		switch {
		case head[j].is(")"):
			nest++
		case head[j].is("("):
			nest--
			if nest == 0 {
				return j
			}
		}
	}
	return -1
}

func identifierAt(head []token, j int) string {
	if j < len(head) && head[j].kind == tIdentifier {
		return head[j].text
	}
	return ""
}

func hasTopLevel(head []token, text string) bool {
	nest := 0
	for _, t := range head {
		switch {
		case t.is("(") || t.is("["):
			nest++
		case t.is(")") || t.is("]"):
			nest--
		case nest == 0 && t.is(text):
			return true
		}
	}
	return false
}

// lineOf is the line of name in head (its last occurrence), else the head's first.
func (s *cScanner) lineOf(head []token, name string) int {
	last := name[strings.LastIndex(name, ":")+1:]
	for j := len(head) - 1; j >= 0; j-- {
		if head[j].kind == tIdentifier && head[j].text == last {
			return head[j].line
		}
	}
	return head[0].line
}

// skipGroup returns the index of the '}' closing the '{' at i (end-1 when none
// does).
func (s *cScanner) skipGroup(i, end int) int {
	nest := 0
	for j := i; j < end; j++ {
		switch {
		case s.tokens[j].is("{"):
			nest++
		case s.tokens[j].is("}"):
			nest--
			if nest == 0 {
				return j
			}
		}
	}
	return end - 1
}

// declarators steps over what may follow a struct's or block's body: instance
// names up to the ';' (`} camera;`, `} lights[4];`).
func (s *cScanner) declarators(i, end int) int {
	for j := i; j < end && j < i+16; j++ {
		t := s.tokens[j]
		switch {
		case t.is(";"):
			return j + 1
		case t.kind == tIdentifier || t.kind == tNumber || t.is("[") || t.is("]") || t.is(",") || t.is(":"):
		default:
			return i
		}
	}
	return i
}

// toSemicolon steps past the ';' ending a statement whose initializer was in braces.
func (s *cScanner) toSemicolon(i, end int) int {
	for j := i; j < end && j < i+64; j++ {
		switch {
		case s.tokens[j].is(";"):
			return j + 1
		case s.tokens[j].is("{") || s.tokens[j].is("}"):
			return i
		}
	}
	return i
}
