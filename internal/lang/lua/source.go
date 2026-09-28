package lua

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/luarocks"
)

// Import kinds, in RawImport.Name.
const (
	kindRequire    = "require" // a module name: require("a.b")
	kindPath       = "path"    // a Luau require by path: "./x", "@self/x", "@alias/x"
	kindRoblox     = "roblox"  // a Roblox instance: require(script.Parent.X), Module "script/../X"
	kindFile       = "file"    // dofile / loadfile of a literal path
	kindDependency = "dep"     // a rockspec dependency; Name "dep:<constraint>"
	kindModule     = "module"  // a rockspec build.modules entry; Module "<module>\n<file>"
	kindWally      = "wally"   // a wally.toml dependency; Name "wally:<requirement>"
	kindRojo       = "rojo"    // a Rojo project's $path
)

// scanner walks a Lua source's tokens once, keeping just enough structure - block
// nesting, bracket depth, the instance paths local variables hold - to tell a
// top-level definition from a nested one and to read what require is given.
type scanner struct {
	source    []byte
	tokens    []luarocks.Token
	teal      bool
	blocks    []byte // 'f' function body, 'b' other block, 'r' repeat, 'R' Teal record body
	functions int    // function bodies open
	br        int    // bracket depth
	typeAt    int    // index of the last type keyword, for Teal's type X = function(...)
	aliases   map[string][]string
	// defined are the names already defined at the top level: a later assignment
	// to one (a local set again in an if, M.x reassigned) is not another definition.
	defined map[string]bool
	symbols lang.SymbolSet
	imports []lang.RawImport
}

// readSource extracts a Lua, Luau or Teal file's definitions and requires.
//
// Implements: REQ-LUA-002, REQ-LUA-003, REQ-LUA-012
func readSource(source []byte, teal bool) *lang.Extraction {
	s := &scanner{source: source, tokens: luarocks.Lex(source), teal: teal, aliases: map[string][]string{},
		defined: map[string]bool{}}
	for i := 0; i < len(s.tokens); i++ {
		s.step(i)
	}
	return &lang.Extraction{Imports: s.imports, Symbols: s.symbols.List()}
}

func (s *scanner) at(i int) luarocks.Token {
	if i >= 0 && i < len(s.tokens) {
		return s.tokens[i]
	}
	return luarocks.Token{Kind: luarocks.EOF}
}

func (s *scanner) push(k byte) {
	s.blocks = append(s.blocks, k)
	if k == 'f' {
		s.functions++
	}
}

func (s *scanner) pop() {
	if n := len(s.blocks); n > 0 {
		if s.blocks[n-1] == 'f' {
			s.functions--
		}
		s.blocks = s.blocks[:n-1]
	}
}

// top reports whether definitions here are the module's own: outside any function
// body and any bracket.
func (s *scanner) top() bool { return s.functions == 0 && s.br == 0 }

func (s *scanner) step(i int) {
	t := s.at(i)
	if t.Kind == luarocks.Punctuation {
		switch t.Text {
		case "(", "{", "[":
			s.br++
		case ")", "}", "]":
			if s.br > 0 {
				s.br--
			}
		}
		return
	}
	if t.Kind != luarocks.Name {
		return
	}
	previous := s.at(i - 1)
	member := previous.Is(".") || previous.Is(":")
	switch t.Text {
	case "function":
		if s.teal && (inRecord(s.blocks) || previous.Is(":") || previous.Is("->") || previous.Is("=") && s.typeAt > 0 && i-s.typeAt < 12) {
			return // a Teal function type, which has no body
		}
		if s.functions == 0 && s.at(i+1).Kind == luarocks.Name {
			name, kind, _ := s.chain(i + 1)
			s.symbols.Add(name, kind, t.Line)
			s.defined[name] = true
		}
		s.push('f')
	case "do", "repeat":
		if !member {
			s.push(map[string]byte{"do": 'b', "repeat": 'r'}[t.Text])
		}
	case "if":
		if !member && !expressionIf(previous) {
			s.push('b')
		}
	case "end", "until":
		if !member {
			s.pop()
		}
	case "record", "interface", "enum":
		if s.teal && !member && s.tealBlock(i) {
			if s.top() && s.at(i+1).Kind == luarocks.Name && !inRecord(s.blocks) {
				s.symbols.Add(s.at(i+1).Text, map[string]string{"record": "class", "interface": "interface", "enum": "enum"}[t.Text], t.Line)
			}
			s.push(map[bool]byte{true: 'b', false: 'R'}[t.Text == "enum"])
		}
	case "type":
		// Luau's type and export type, Teal's local/global type.
		if !member {
			s.typeAt = i
		}
		if !member && s.at(i+1).Kind == luarocks.Name && (s.at(i+2).Is("=") || s.at(i+2).Is("<")) &&
			s.top() && !inRecord(s.blocks) {
			s.symbols.Add(s.at(i+1).Text, "type", t.Line)
		}
	case "local", "global":
		s.local(i)
	case "require", "dofile", "loadfile":
		if !member && !previous.Is("function") && !previous.Is("local") {
			s.call(i)
		}
	default:
		if s.top() && !member && !keywords[t.Text] && !previous.Is("for") && !previous.Is(",") && !previous.Is("<") &&
			!previous.Is("local") && !previous.Is("global") && !previous.Is("type") && !inRecord(s.blocks) {
			s.assignment(i)
		}
	}
}

var keywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true,
	"false": true, "for": true, "function": true, "goto": true, "if": true, "in": true,
	"local": true, "nil": true, "not": true, "or": true, "repeat": true, "return": true,
	"then": true, "true": true, "until": true, "while": true, "continue": true, "export": true,
}

// expressionIf reports whether an if after previous is Luau's if-expression
// (x = if a then b else c), which has no end: it follows an operator, an opening
// bracket, a comma or return.
func expressionIf(previous luarocks.Token) bool {
	switch previous.Kind {
	case luarocks.Punctuation:
		switch previous.Text {
		case ")", "]", "}", ";", "::":
			return false
		}
		return true
	case luarocks.Name:
		switch previous.Text {
		case "return", "and", "or", "not", "in":
			return true
		}
	}
	return false
}

// tealBlock reports whether a Teal record/interface/enum keyword opens a body
// closed by end: after local/global/=, or naming a nested type in a record body.
func (s *scanner) tealBlock(i int) bool {
	previous, next := s.at(i-1), s.at(i+1)
	if previous.Is("local") || previous.Is("global") || previous.Is("=") {
		return true
	}
	return inRecord(s.blocks) && next.Kind == luarocks.Name && !next.Is("end")
}

func inRecord(blocks []byte) bool { return len(blocks) > 0 && blocks[len(blocks)-1] == 'R' }

// chain reads a function name, a.b.c or a.b:c, from i and returns it as a symbol
// name (owner.name), its kind, and the index after it.
func (s *scanner) chain(i int) (string, string, int) {
	parts := []string{s.at(i).Text}
	kind := "function"
	i++
	for (s.at(i).Is(".") || s.at(i).Is(":")) && s.at(i+1).Kind == luarocks.Name {
		if s.at(i).Is(":") {
			kind = "method"
		}
		parts = append(parts, s.at(i+1).Text)
		i += 2
	}
	return strings.Join(parts, "."), kind, i
}

// local reads a local (or Teal global) declaration: the module tables it defines
// at the top level, and the Roblox instance paths it names for later requires.
func (s *scanner) local(i int) {
	next := s.at(i + 1)
	if next.Kind != luarocks.Name || next.Is("function") {
		return
	}
	if n2 := s.at(i + 2); n2.Kind == luarocks.Name && (next.Is("type") || next.Is("record") || next.Is("enum") || next.Is("interface")) {
		return // a Teal type declaration, read at its keyword
	}
	var names []string
	j := i + 1
	for s.at(j).Kind == luarocks.Name && !keywords[s.at(j).Text] {
		names = append(names, s.at(j).Text)
		j++
		if s.at(j).Is("<") && s.at(j+2).Is(">") { // <const>, <close>
			j += 3
		}
		if s.at(j).Is(":") { // a type annotation
			j = s.skipType(j + 1)
		}
		if !s.at(j).Is(",") {
			break
		}
		j++
	}
	if len(names) == 0 || !s.at(j).Is("=") {
		return
	}
	j++
	if len(names) == 1 {
		if path, end, ok := s.instancePath(j); ok && !continues(s.at(end)) {
			s.aliases[names[0]] = path
		} else {
			delete(s.aliases, names[0])
		}
	}
	if s.top() {
		if v := s.at(j); v.Is("{") || (v.Is("setmetatable") && s.at(j+1).Is("(")) {
			s.symbols.Add(names[0], "table", s.at(i).Line)
		}
		for _, n := range names {
			s.defined[n] = true
		}
	}
}

// continues reports whether a token extends the expression before it.
func continues(t luarocks.Token) bool {
	return t.Is(".") || t.Is(":") || t.Is("(") || t.Is("[") || t.Kind == luarocks.String || t.Is("{")
}

// skipType skips a type annotation to the , or = that ends it, brackets and
// generic angle brackets included.
func (s *scanner) skipType(j int) int {
	depth := 0
	for ; j < len(s.tokens); j++ {
		t := s.at(j)
		switch {
		case t.Is("(") || t.Is("{") || t.Is("[") || t.Is("<"):
			depth++
		case t.Is(")") || t.Is("}") || t.Is("]") || t.Is(">"):
			depth--
		case t.Is(">>"):
			depth -= 2
		case depth <= 0 && (t.Is(",") || t.Is("=")):
			return j
		case t.Kind == luarocks.Name && (t.Is("local") || t.Is("function") || t.Is("end") || t.Is("return")):
			return j
		}
	}
	return j
}

// assignment reads a top-level assignment to a global or a field: M.f = function
// defines a function, M = {} or M.sub = {} a table, M.x = anything else a variable.
func (s *scanner) assignment(i int) {
	name, _, j := s.chain(i)
	if !s.at(j).Is("=") || s.defined[name] {
		return
	}
	s.defined[name] = true
	switch v := s.at(j + 1); {
	case v.Is("function"):
		kind := "function"
		if strings.Contains(name, ".") && s.at(j+2).Is("(") && s.at(j+3).Is("self") {
			kind = "method"
		}
		s.symbols.Add(name, kind, s.at(i).Line)
	case v.Is("{"), v.Is("setmetatable") && s.at(j+2).Is("("):
		s.symbols.Add(name, "table", s.at(i).Line)
	case strings.Contains(name, "."):
		s.symbols.Add(name, "var", s.at(i).Line) // a field the module exports
	}
}

// call reads require, dofile and loadfile calls with an argument it can know: a
// string literal, or for require a Roblox instance path; and pcall(require, "x").
func (s *scanner) call(i int) {
	t := s.at(i)
	function := t.Text
	start, argument, end := i, i+1, i+1
	if s.at(i + 1).Is("(") {
		argument = i + 2
		end = i + 3
		if !s.at(end).Is(")") {
			end = -1
		}
	}
	if function == "require" && s.at(i-1).Is("(") && s.at(i-2).Is("pcall") && s.at(i+1).Is(",") && s.at(i+2).Kind == luarocks.String {
		if !s.at(i + 3).Is(")") {
			return // a name computed from the string
		}
		start, argument, end = i-2, i+2, i+3
	}
	a := s.at(argument)
	if a.Kind == luarocks.String && end >= 0 {
		spec := s.text(start, end)
		if function != "require" {
			s.imports = append(s.imports, lang.RawImport{Spec: spec, Module: a.Text, Name: kindFile, Line: t.Line})
			return
		}
		kind := kindRequire
		if strings.HasPrefix(a.Text, "./") || strings.HasPrefix(a.Text, "../") || strings.HasPrefix(a.Text, "@") {
			kind = kindPath
		}
		s.imports = append(s.imports, lang.RawImport{Spec: spec, Module: a.Text, Name: kind, Line: t.Line})
		return
	}
	if function == "require" && s.at(i+1).Is("(") {
		if path, e, ok := s.instancePath(i + 2); ok && s.at(e).Is(")") {
			s.imports = append(s.imports, lang.RawImport{Spec: s.text(i, e), Module: strings.Join(path, "/"), Name: kindRoblox, Line: t.Line})
		}
	}
}

// text is the source from token a through token b, its whitespace collapsed.
func (s *scanner) text(a, b int) string {
	from, to := s.at(a).Start, s.at(b).End
	if from > to || to > len(s.source) {
		return ""
	}
	return strings.Join(strings.Fields(string(s.source[from:to])), " ")
}

// instancePath reads a Roblox instance expression from j: script, game, workspace
// or a local holding one, followed by .Name, .Parent, ["Name"],
// :WaitForChild("Name"), :FindFirstChild("Name") and game:GetService("Name").
// Parent is "..". It returns the elements and the index after the expression.
func (s *scanner) instancePath(j int) ([]string, int, bool) {
	b := s.at(j)
	var path []string
	switch {
	case b.Is("script"):
		path = []string{"script"}
	case b.Is("game"):
		path = []string{"game"}
	case b.Is("workspace"):
		path = []string{"game", "Workspace"}
	case b.Kind == luarocks.Name && s.aliases[b.Text] != nil:
		path = append([]string(nil), s.aliases[b.Text]...)
	default:
		return nil, j, false
	}
	j++
	for {
		switch t := s.at(j); {
		case t.Is(".") && s.at(j+1).Kind == luarocks.Name:
			name := s.at(j + 1).Text
			if name == "Parent" {
				name = ".."
			}
			path = append(path, name)
			j += 2
		case t.Is("[") && s.at(j+1).Kind == luarocks.String && s.at(j+2).Is("]"):
			path = append(path, s.at(j+1).Text)
			j += 3
		case t.Is(":") && s.at(j+2).Is("(") && s.at(j+3).Kind == luarocks.String:
			switch s.at(j + 1).Text {
			case "WaitForChild", "FindFirstChild", "GetService":
			default:
				return path, j, true
			}
			k := j + 4
			for s.at(k).Is(",") { // a timeout, or FindFirstChild's recursive flag
				k += 2
			}
			if !s.at(k).Is(")") {
				return nil, j, false
			}
			path = append(path, s.at(j+3).Text)
			j = k + 1
		default:
			return path, j, true
		}
	}
}
