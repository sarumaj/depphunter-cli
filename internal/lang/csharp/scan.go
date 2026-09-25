package csharp

import (
	"regexp"
	"strings"
)

// Declarations are read from statements, the text between `;`, `{` and `}` outside
// parentheses, comments, strings and preprocessor lines.

type stmt struct {
	text string
	line int
	// scope is the kind of the innermost enclosing block: "" (file), "namespace",
	// "type" (class, struct, record) or "other" (interfaces, enums, members, …).
	scope string
	owner string // enclosing type name when scope is "type"
}

type blockInfo struct{ kind, name string }

var (
	typeDecl     = regexp.MustCompile(`(?:^|\s)(class|interface|struct|enum|record(?:\s+(?:class|struct))?)\s+([A-Za-z_]\w*)`)
	delegateDecl = regexp.MustCompile(`(?:^|\s)delegate\s+[\w<>\[\],.?\s]+?\s+([A-Za-z_]\w*)\s*[<(]`)
	nsDecl       = regexp.MustCompile(`^namespace\s+([\w.]+)`)
	usingDecl    = regexp.MustCompile(`^(?:global\s+)?using\s+[^(]`)
	method       = regexp.MustCompile(`^(?:\[[^\]]*\]\s*)*(?:(?:public|private|protected|internal|static|virtual|override|abstract|sealed|async|extern|unsafe|new|partial|readonly|required)\s+)*[\w<>\[\],.?]+(?:\s*<[^>]*>)?\s+([A-Za-z_]\w*)\s*(?:<[^>]*>)?\s*\(`)
	notNames     = map[string]bool{"if": true, "while": true, "for": true, "foreach": true, "switch": true, "using": true, "lock": true, "catch": true, "return": true, "new": true, "await": true, "yield": true, "throw": true, "nameof": true, "typeof": true, "sizeof": true, "fixed": true}
)

// Implements: REQ-CS-005
func scanStatements(src string) []stmt {
	var out []stmt
	var cur strings.Builder
	curLine, line, parens := 0, 1, 0
	started := false // the current statement has a non-space character
	var blocks []blockInfo
	pending := blockInfo{kind: "other"}

	scope := func() (string, string) {
		if len(blocks) == 0 {
			return "", ""
		}
		b := blocks[len(blocks)-1]
		return b.kind, b.name
	}
	// emit ends the current statement and returns its text ("" if it was empty).
	emit := func(opensBlock bool) string {
		text := strings.Join(strings.Fields(cur.String()), " ")
		cur.Reset()
		started = false
		pending = blockInfo{kind: "other"}
		if text == "" {
			return ""
		}
		st := stmt{text: text, line: curLine}
		st.scope, st.owner = scope()
		out = append(out, st)
		if !opensBlock {
			return text
		}
		if m := nsDecl.FindStringSubmatch(text); m != nil {
			pending = blockInfo{kind: "namespace", name: m[1]}
		} else if m := typeDecl.FindStringSubmatch(text); m != nil {
			kind := "type"
			if m[1] == "interface" || m[1] == "enum" {
				kind = "other"
			}
			pending = blockInfo{kind: kind, name: m[2]}
		}
		return text
	}
	write := func(r rune) {
		if !started && r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			curLine, started = line, true
		}
		cur.WriteRune(r)
	}

	rs := []rune(src)
	at := func(i int) rune {
		if i >= 0 && i < len(rs) {
			return rs[i]
		}
		return 0
	}
	atLineStart := func(i int) bool {
		for j := i - 1; j >= 0 && rs[j] != '\n'; j-- {
			if rs[j] != ' ' && rs[j] != '\t' {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\n':
			line++
			write(' ')
		case r == '/' && at(i+1) == '/':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			i--
		case r == '/' && at(i+1) == '*':
			for i += 2; i < len(rs) && !(rs[i] == '*' && at(i+1) == '/'); i++ {
				if rs[i] == '\n' {
					line++
				}
			}
			i++
		case r == '#' && atLineStart(i): // preprocessor directive
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			i--
		case isLiteralStart(rs, i):
			// Skip the literal, keeping a placeholder so statements stay well-formed.
			i = skipLiteral(rs, i, &line)
			write('"')
			write('"')
		case r == '\'':
			i = skipChar(rs, i)
			write('\'')
			write('\'')
		case r == '(':
			parens++
			write(r)
		case r == ')':
			parens = max(0, parens-1)
			write(r)
		case r == '{' && parens == 0:
			emit(true)
			blocks = append(blocks, pending)
			pending = blockInfo{kind: "other"}
		case r == '}' && parens == 0:
			emit(false)
			if len(blocks) > 0 {
				blocks = blocks[:len(blocks)-1]
			}
		case r == ';' && parens == 0:
			// A file-scoped namespace applies to the rest of the file.
			if m := nsDecl.FindStringSubmatch(emit(false)); m != nil {
				blocks = append(blocks, blockInfo{kind: "namespace", name: m[1]})
			}
		default:
			write(r)
		}
	}
	emit(false)
	return out
}

// isLiteralStart reports whether a string literal starts at i: "…", @"…", $"…",
// $@"…", @$"…", or a raw literal with $ prefixes ($$"""…""").
func isLiteralStart(rs []rune, i int) bool {
	for ; i < len(rs) && (rs[i] == '@' || rs[i] == '$'); i++ {
	}
	return i < len(rs) && rs[i] == '"'
}

// skipLiteral returns the index of the last rune of the string literal starting at i,
// counting newlines into line. Interpolation holes are walked, so strings, chars and
// braces inside them ($"{(ok ? "}" : "{")}") cannot end the literal early.
//
// Implements: REQ-CS-005, REQ-CS-006
func skipLiteral(rs []rune, i int, line *int) int {
	at := func(j int) rune {
		if j < len(rs) {
			return rs[j]
		}
		return 0
	}
	verbatim, interpolated := false, false
	for ; rs[i] != '"'; i++ {
		verbatim = verbatim || rs[i] == '@'
		interpolated = interpolated || rs[i] == '$'
	}
	quotes := 0
	for at(i+quotes) == '"' {
		quotes++
	}
	if quotes >= 3 { // raw literal: ends at the same run of quotes
		run := strings.Repeat(`"`, quotes)
		for i += quotes; i < len(rs) && !strings.HasPrefix(string(rs[i:min(i+quotes, len(rs))]), run); i++ {
			if rs[i] == '\n' {
				*line++
			}
		}
		return min(i+quotes-1, len(rs)-1)
	}
	if quotes == 2 && !verbatim { // ""
		return i + 1
	}
	for i++; i < len(rs); i++ {
		switch c := rs[i]; {
		case c == '\n':
			*line++
		case c == '\\' && !verbatim:
			i++
		case c == '"':
			if verbatim && at(i+1) == '"' { // "" escapes a quote in verbatim strings
				i++
				continue
			}
			return i
		case interpolated && c == '{':
			if at(i+1) == '{' { // {{ is a literal brace
				i++
				continue
			}
			for depth := 1; depth > 0 && i+1 < len(rs); {
				i++
				switch d := rs[i]; {
				case isLiteralStart(rs, i):
					i = skipLiteral(rs, i, line)
				case d == '\'':
					i = skipChar(rs, i)
				case d == '{':
					depth++
				case d == '}':
					depth--
				case d == '\n':
					*line++
				}
			}
		}
	}
	return len(rs) - 1
}

// skipChar returns the index of the closing quote of the char literal at i.
func skipChar(rs []rune, i int) int {
	for i++; i < len(rs) && rs[i] != '\''; i++ {
		if rs[i] == '\\' {
			i++
		}
	}
	return min(i, len(rs)-1)
}
