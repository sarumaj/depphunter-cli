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
	typeDecl  = regexp.MustCompile(`(?:^|\s)(class|interface|struct|enum|record(?:\s+(?:class|struct))?)\s+([A-Za-z_]\w*)`)
	delegDecl = regexp.MustCompile(`(?:^|\s)delegate\s+[\w<>\[\],.?\s]+?\s+([A-Za-z_]\w*)\s*[<(]`)
	nsDecl    = regexp.MustCompile(`^namespace\s+([\w.]+)`)
	usingDecl = regexp.MustCompile(`^(?:global\s+)?using\s+[^(]`)
	method    = regexp.MustCompile(`^(?:\[[^\]]*\]\s*)*(?:(?:public|private|protected|internal|static|virtual|override|abstract|sealed|async|extern|unsafe|new|partial|readonly|required)\s+)*[\w<>\[\],.?]+(?:\s*<[^>]*>)?\s+([A-Za-z_]\w*)\s*(?:<[^>]*>)?\s*\(`)
	notNames  = map[string]bool{"if": true, "while": true, "for": true, "foreach": true, "switch": true, "using": true, "lock": true, "catch": true, "return": true, "new": true, "await": true, "yield": true, "throw": true, "nameof": true, "typeof": true, "sizeof": true, "fixed": true}
)

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
		case r == '"' || ((r == '@' || r == '$') && (at(i+1) == '"' || (at(i+1) == '@' || at(i+1) == '$') && at(i+2) == '"')):
			// Skip the literal, keeping a placeholder so statements stay well-formed.
			verbatim := false
			for rs[i] != '"' {
				verbatim = verbatim || rs[i] == '@'
				i++
			}
			quotes := 0
			for at(i+quotes) == '"' {
				quotes++
			}
			if quotes >= 3 { // raw string literal: ends with the same run of quotes
				i += quotes
				for i < len(rs) && !strings.HasPrefix(string(rs[i:min(i+quotes, len(rs))]), strings.Repeat(`"`, quotes)) {
					if rs[i] == '\n' {
						line++
					}
					i++
				}
				i += quotes - 1
			} else if quotes == 2 && !verbatim { // empty string ""
				i++
			} else {
				for i++; i < len(rs); i++ {
					c := rs[i]
					if c == '\n' {
						line++
					}
					if c == '\\' && !verbatim {
						i++
						continue
					}
					if c == '"' {
						if verbatim && at(i+1) == '"' {
							i++
							continue
						}
						break
					}
				}
			}
			write('"')
			write('"')
		case r == '\'':
			for i++; i < len(rs) && rs[i] != '\''; i++ {
				if rs[i] == '\\' {
					i++
				}
			}
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
