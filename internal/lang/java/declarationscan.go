package java

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
)

// The declaration index reads Kotlin and Scala as text, but not line by line: a
// definition counts as top-level by where it sits in the nesting, not by its
// column. The source is first blanked - comments (nested ones too), the insides of
// string and character literals, and the code of string templates become spaces,
// line breaks kept - so that what is left of each line is code whose brackets can be
// counted. A line starting outside every bracket is then a top-level definition,
// unless it is indented further than the scope's first line: a continuation, or a
// member of a Scala 3 body opened by a colon rather than a brace. Scala 2's package
// blocks (package a { ... }) and Scala 3's (package a: and indented lines) are
// scopes of their own rather than nesting, so what they hold is top-level in their
// package.
//
// No lexer of another plugin fits: Swift's interpolation is \( ), and the C-family
// ones know neither nested comments nor ${ } templates.

// maxTemplateNesting bounds the string templates followed inside one another
// ("${"${ ... }"}"); a deeper one is read as text, which can only end the enclosing
// literal early.
const maxTemplateNesting = 32

// packageBlock and packageColon are package clauses that open a scope: Scala 2's
// braces and Scala 3's colon.
var (
	packageBlock = regexp.MustCompile(`^package\s+([\w.]+)\s*\{$`)
	packageColon = regexp.MustCompile(`^package\s+([\w.]+)\s*:$`)
)

// blanker writes the blanked copy of a source.
type blanker struct {
	source, out []byte
	scala       bool
}

// blank returns source with comments and the insides of literals turned into
// spaces. The delimiters of a string stay, so `val x = ""` still reads as a
// definition.
func blank(source []byte, scala bool) []byte {
	b := &blanker{source: source, out: append([]byte(nil), source...), scala: scala}
	b.code(0, 0, false)
	return b.out
}

// wipe blanks out[i:j], keeping line breaks.
func (b *blanker) wipe(i, j int) {
	for ; i < j && i < len(b.out); i++ {
		if b.out[i] != '\n' {
			b.out[i] = ' '
		}
	}
}

// code reads code from i. Inside a template (inner) it stops after the brace that
// closes it and returns the index past it; otherwise it runs to the end.
func (b *blanker) code(i, nesting int, inner bool) int {
	source, depth := b.source, 0
	for i < len(source) {
		character := source[i]
		switch {
		case character == '/' && i+1 < len(source) && source[i+1] == '/':
			j := i
			j = chars.LineEnd(source, j)
			b.wipe(i, j)
			i = j
		case character == '/' && i+1 < len(source) && source[i+1] == '*':
			j := blockComment(source, i)
			b.wipe(i, j)
			i = j
		case character == '"':
			i = b.stringLiteral(i, nesting)
		case character == '\'':
			i = b.characterLiteral(i)
		case character == '`':
			i = b.backquoted(i)
		case inner && character == '{':
			depth++
			i++
		case inner && character == '}':
			if depth == 0 {
				return i + 1
			}
			depth--
			i++
		default:
			i++
		}
	}
	return i
}

// blockComment returns the index past the comment opening at i, nested comments
// included (both languages nest them); an unterminated one runs to the end.
func blockComment(source []byte, i int) int {
	depth := 0
	for i < len(source) {
		switch {
		case source[i] == '/' && i+1 < len(source) && source[i+1] == '*':
			depth++
			i += 2
		case source[i] == '*' && i+1 < len(source) && source[i+1] == '/':
			depth--
			i += 2
			if depth == 0 {
				return i
			}
		default:
			i++
		}
	}
	return i
}

// stringLiteral blanks the string literal whose quote is at i and returns the index past it.
// Every Kotlin string is a template; a Scala one is when an identifier (s, f, raw,
// any interpolator) touches its quote, and only raw"..." takes backslashes as they
// are. Triple-quoted strings have no escapes and end at the last of a run of
// quotes. A single-quoted string left open ends at the line break.
func (b *blanker) stringLiteral(i, nesting int) int {
	source := b.source
	interpolated, escapes := !b.scala, true
	if b.scala && i > 0 && identifierByte(source[i-1]) {
		interpolated = true
		start := i
		for start > 0 && identifierByte(source[start-1]) {
			start--
		}
		escapes = string(source[start:i]) != "raw"
	}
	triple := i+2 < len(source) && source[i+1] == '"' && source[i+2] == '"'
	j := i + 1
	if triple {
		j = i + 3
	}
	for j < len(source) {
		character := source[j]
		switch {
		case triple && character == '"' && j+2 < len(source) && source[j+1] == '"' && source[j+2] == '"':
			j += 3
			for j < len(source) && source[j] == '"' {
				j++
			}
			b.wipe(i+3, j-3)
			return j
		case !triple && character == '"':
			b.wipe(i+1, j)
			return j + 1
		case !triple && character == '\n':
			b.wipe(i+1, j)
			return j
		case !triple && escapes && character == '\\':
			j += 2
		case interpolated && character == '$' && j+1 < len(source) && source[j+1] == '{' && nesting < maxTemplateNesting:
			j = b.code(j+2, nesting+1, true)
		case interpolated && b.scala && character == '$' && j+1 < len(source) && (source[j+1] == '$' || source[j+1] == '"'):
			j += 2 // Scala's $$ and $" escapes
		default:
			j++
		}
	}
	if triple {
		b.wipe(i+3, len(source))
	} else {
		b.wipe(i+1, len(source))
	}
	return len(source)
}

// characterLiteral blanks a character literal at i ('a', '\n', 'A', '"') and returns the
// index past it. A quote that opens none - a Scala 2 symbol ('sym) or a Scala 3
// quote ('{ ... }) - is passed over alone.
func (b *blanker) characterLiteral(i int) int {
	source := b.source
	if i+1 < len(source) && source[i+1] == '\\' {
		for j := i + 2; j < len(source) && j < i+12 && source[j] != '\n'; j++ {
			if source[j] == '\'' && j > i+2 {
				b.wipe(i+1, j)
				return j + 1
			}
		}
		return i + 1
	}
	if i+1 < len(source) && source[i+1] != '\n' {
		_, size := utf8.DecodeRune(source[i+1:])
		if j := i + 1 + size; j < len(source) && source[j] == '\'' {
			b.wipe(i+1, j)
			return j + 1
		}
	}
	return i + 1
}

// backquoted passes over a `quoted identifier` on its line, blanking the brackets
// and quotes it may hold so that they are not counted.
func (b *blanker) backquoted(i int) int {
	for j := i + 1; j < len(b.source) && b.source[j] != '\n'; j++ {
		switch b.source[j] {
		case '`':
			return j + 1
		case '{', '}', '(', ')', '[', ']', '"', '\'':
			b.out[j] = ' '
		}
	}
	return i + 1
}

func identifierByte(character byte) bool {
	return character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9'
}

// packageScope is a package's scope in a file: the file itself, or a package block.
type packageScope struct {
	name   string
	brace  bool // a Scala 2 package block, closed by its brace; else a Scala 3 one closed by indentation
	indent int  // the package clause's indentation
	base   int  // the indentation of the scope's first line, -1 before it
	named  bool // a definition was read in it
}

// packageDeclarations is what a file declares in one package.
type packageDeclarations struct {
	name  string
	names []string
}

// declarations reads the packages a Kotlin or Scala file declares and the
// top-level definitions of each, in the order they appear. Chained package clauses
// (package com.example, then package app) make one package; one after the scope's
// first definition is ignored.
//
// Implements: REQ-KT-003, REQ-KT-005, REQ-KT-007, REQ-SCALA-003
func declarations(source []byte, scala bool) []packageDeclarations {
	text := blank(source, scala)
	scopes := []*packageScope{{base: -1}}
	var out []packageDeclarations
	index := map[string]int{} // package -> its index in out
	record := func(packageName, name string) {
		at, ok := index[packageName]
		if !ok {
			at = len(out)
			index[packageName] = at
			out = append(out, packageDeclarations{name: packageName})
		}
		if name != "" {
			out[at].names = append(out[at].names, name)
		}
	}
	// stack holds the open brackets; 'P' is a package block's brace, which does not
	// nest the definitions inside it.
	var stack []byte
	blocks := 0 // the package-block braces at the bottom of the stack
	for len(text) > 0 {
		line := text
		if end := bytes.IndexByte(text, '\n'); end >= 0 {
			line, text = text[:end], text[end+1:]
		} else {
			text = nil
		}
		trimmed := string(bytes.TrimSpace(line))
		if trimmed != "" && len(stack) == blocks {
			indent := len(line) - len(bytes.TrimLeft(line, " \t"))
			for scope := scopes[len(scopes)-1]; !scope.brace && len(scopes) > 1 && indent <= scope.indent; scope = scopes[len(scopes)-1] {
				scopes = scopes[:len(scopes)-1]
			}
			scope := scopes[len(scopes)-1]
			if scope.base < 0 || indent < scope.base {
				scope.base = indent
			}
			if indent == scope.base {
				if match := packageBlock.FindStringSubmatch(trimmed); match != nil && scala {
					name := joinPackage(scope.name, match[1])
					scopes = append(scopes, &packageScope{name: name, brace: true, indent: indent, base: -1})
					record(name, "")
					stack = append(stack, 'P')
					blocks++
					continue
				}
				if match := packageColon.FindStringSubmatch(trimmed); match != nil && scala {
					name := joinPackage(scope.name, match[1])
					scopes = append(scopes, &packageScope{name: name, indent: indent, base: -1})
					record(name, "")
					continue
				}
				if name := scope.definition(trimmed); name != "" {
					scope.named = true
					record(scope.name, name)
				}
			}
		}
		for _, character := range line {
			switch character {
			case '{', '(', '[':
				stack = append(stack, character)
			case '}', ')', ']':
				if len(stack) == 0 {
					continue // unbalanced: ignored
				}
				top := stack[len(stack)-1]
				if top != opener(character) && (character != '}' || top != 'P') {
					continue
				}
				if top == 'P' {
					blocks--
					for len(scopes) > 1 {
						brace := scopes[len(scopes)-1].brace
						scopes = scopes[:len(scopes)-1]
						if brace {
							break
						}
					}
				}
				stack = stack[:len(stack)-1]
			}
		}
	}
	if scopes[0].name != "" {
		record(scopes[0].name, "")
	}
	return out
}

// definition reads a top-level line of a scope: a package object or a definition
// gives its name; a package clause extends the scope's package, when nothing is
// defined in it yet, and gives none.
func (s *packageScope) definition(line string) string {
	if rest, found := strings.CutPrefix(line, "package "); found {
		rest = strings.TrimSpace(rest)
		if object, isObject := strings.CutPrefix(rest, "object "); isObject { // Scala package object
			if fields := strings.Fields(object); len(fields) > 0 {
				return strings.Trim(fields[0], "{:")
			}
			return ""
		}
		if !s.named {
			rest = strings.TrimSpace(strings.TrimRight(rest, ";{:"))
			s.name = joinPackage(s.name, strings.Join(strings.Fields(rest), ""))
		}
		return ""
	}
	match := topLevel.FindStringSubmatch(line)
	if match == nil {
		return ""
	}
	if name := match[1][strings.LastIndex(match[1], ".")+1:]; name != "_" {
		return name
	}
	return ""
}

func joinPackage(outer, inner string) string {
	if outer == "" {
		return inner
	}
	return outer + "." + inner
}

func opener(closer byte) byte {
	switch closer {
	case '}':
		return '{'
	case ')':
		return '('
	}
	return '['
}
