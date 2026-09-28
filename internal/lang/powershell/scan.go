package powershell

import "strings"

type statement struct {
	text  string
	line  int
	class string // enclosing class when the statement sits directly in a class body
}

type comment struct {
	text string
	line int
}

type block struct {
	hash  bool   // @{ ... } hashtable, not a script block: statements do not split inside
	class string // class body
}

// scanner splits PowerShell source into statements (on newlines, `;`, `{` and `}`),
// skipping comments, string and here-string contents, and joining lines continued with
// a backtick, a trailing pipe or comma, or an open parenthesis.
//
// Implements: REQ-PS-009
type scanner struct {
	statements []statement
	comments   []comment
	clean      strings.Builder // source without comments, newlines kept

	current     strings.Builder
	started     bool // current holds a non-space character
	currentLine int
	line        int
	parentheses int
	blocks      []block
	pending     string // class name whose body the next `{` opens
}

func (s *scanner) emit() {
	text := strings.TrimSpace(s.current.String())
	s.current.Reset()
	s.started = false
	if text == "" {
		return
	}
	statement := statement{text: text, line: s.currentLine}
	if n := len(s.blocks); n > 0 && s.blocks[n-1].class != "" {
		statement.class = s.blocks[n-1].class
	}
	if m := classDefinition.FindStringSubmatch(text); m != nil {
		s.pending = m[1]
	}
	s.statements = append(s.statements, statement)
}

func (s *scanner) write(r rune) {
	if !s.started && r != ' ' && r != '\t' && r != '\r' {
		s.currentLine, s.started = s.line, true
	}
	s.current.WriteRune(r)
	s.clean.WriteRune(r)
}

func (s *scanner) inHash() bool {
	for _, b := range s.blocks {
		if b.hash {
			return true
		}
	}
	return false
}

func scanStatements(source string) ([]statement, []comment) {
	s := &scanner{line: 1}
	s.run(source)
	return s.statements, s.comments
}

func stripComments(source string) string {
	s := &scanner{line: 1}
	s.run(source)
	return s.clean.String()
}

func (s *scanner) run(source string) {
	runes := []rune(source)
	at := func(i int) rune {
		if i < len(runes) {
			return runes[i]
		}
		return 0
	}
	lineStart := func(i int) bool { return i == 0 || runes[i-1] == '\n' }
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '\n':
			s.clean.WriteRune(r)
			s.line++
			// A statement continues after `, |, a trailing comma or inside ( ) / @{ }.
			trimmed := strings.TrimRight(s.current.String(), " \t\r")
			if s.parentheses > 0 || s.inHash() || strings.HasSuffix(trimmed, "`") || strings.HasSuffix(trimmed, "|") || strings.HasSuffix(trimmed, ",") {
				s.current.Reset()
				s.current.WriteString(strings.TrimSuffix(trimmed, "`") + " ")
				continue
			}
			s.emit()

		case r == '<' && at(i+1) == '#': // block comment
			start, line := i, s.line
			for i += 2; i < len(runes) && !(runes[i] == '#' && at(i+1) == '>'); i++ {
				if runes[i] == '\n' {
					s.line++
					s.clean.WriteRune('\n')
				}
			}
			i++
			s.comments = append(s.comments, comment{string(runes[start:min(i+1, len(runes))]), line})

		case r == '#':
			start := i
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			s.comments = append(s.comments, comment{strings.TrimSpace(string(runes[start:i])), s.line})
			i-- // let the newline be handled

		case r == '@' && (at(i+1) == '"' || at(i+1) == '\'') && (at(i+2) == '\n' || at(i+2) == '\r'):
			// Here-string: contents are data, never code.
			q := at(i + 1)
			s.write('@')
			s.write(q)
			for i += 2; i < len(runes); i++ {
				if runes[i] == '\n' {
					s.line++
					s.clean.WriteRune('\n')
				} else if runes[i] == q && at(i+1) == '@' && i > 0 && lineStart(i) {
					break
				}
			}
			i++
			s.write(q)
			s.write('@')

		case r == '\'' || r == '"':
			s.write(r)
			for i++; i < len(runes); i++ {
				c := runes[i]
				if c == '\n' {
					s.line++
				}
				s.write(c)
				if r == '"' && c == '`' && i+1 < len(runes) {
					i++
					s.write(runes[i])
					continue
				}
				if c == r {
					if at(i+1) == r { // doubled quote escapes itself
						i++
						s.write(runes[i])
						continue
					}
					break
				}
			}

		case r == '(':
			s.parentheses++
			s.write(r)
		case r == ')':
			s.parentheses = max(0, s.parentheses-1)
			s.write(r)

		case r == '{':
			if s.current.Len() > 0 && strings.HasSuffix(s.current.String(), "@") {
				s.write(r)
				s.blocks = append(s.blocks, block{hash: true})
				continue
			}
			if s.inHash() {
				s.write(r)
				s.blocks = append(s.blocks, block{hash: true})
				continue
			}
			s.emit()
			s.clean.WriteRune(r)
			s.blocks = append(s.blocks, block{class: s.pending})
			s.pending = ""

		case r == '}':
			if n := len(s.blocks); n > 0 && s.blocks[n-1].hash {
				s.write(r)
				s.blocks = s.blocks[:n-1]
				continue
			}
			s.emit()
			s.clean.WriteRune(r)
			if n := len(s.blocks); n > 0 {
				s.blocks = s.blocks[:n-1]
			}

		case r == ';' && s.parentheses == 0 && !s.inHash():
			s.emit()
			s.clean.WriteRune(r)

		default:
			s.write(r)
		}
	}
	s.emit()
}

// valueAt returns the value expression at the start of text: a balanced @( ... )
// array, a quoted string, or the rest of the line.
func valueAt(text string) string {
	switch {
	case strings.HasPrefix(text, "@("):
		depth := 0
		for i, r := range text {
			switch r {
			case '(':
				depth++
			case ')':
				if depth--; depth == 0 {
					return text[:i+1]
				}
			}
		}
		return text
	case strings.HasPrefix(text, "'") || strings.HasPrefix(text, `"`):
		if end := strings.IndexByte(text[1:], text[0]); end >= 0 {
			return text[:end+2]
		}
	}
	line, _, _ := strings.Cut(text, "\n")
	return line
}
