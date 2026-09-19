package powershell

import "strings"

type stmt struct {
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
type scanner struct {
	statements []stmt
	comments   []comment
	clean      strings.Builder // source without comments, newlines kept

	cur     strings.Builder
	started bool // cur holds a non-space character
	curLine int
	line    int
	parens  int
	blocks  []block
	pending string // class name whose body the next `{` opens
}

func (s *scanner) emit() {
	text := strings.TrimSpace(s.cur.String())
	s.cur.Reset()
	s.started = false
	if text == "" {
		return
	}
	st := stmt{text: text, line: s.curLine}
	if n := len(s.blocks); n > 0 && s.blocks[n-1].class != "" {
		st.class = s.blocks[n-1].class
	}
	if m := classDef.FindStringSubmatch(text); m != nil {
		s.pending = m[1]
	}
	s.statements = append(s.statements, st)
}

func (s *scanner) write(r rune) {
	if !s.started && r != ' ' && r != '\t' && r != '\r' {
		s.curLine, s.started = s.line, true
	}
	s.cur.WriteRune(r)
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

func scanStatements(src string) ([]stmt, []comment) {
	s := &scanner{line: 1}
	s.run(src)
	return s.statements, s.comments
}

func stripComments(src string) string {
	s := &scanner{line: 1}
	s.run(src)
	return s.clean.String()
}

func (s *scanner) run(src string) {
	rs := []rune(src)
	at := func(i int) rune {
		if i < len(rs) {
			return rs[i]
		}
		return 0
	}
	lineStart := func(i int) bool { return i == 0 || rs[i-1] == '\n' }
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\n':
			s.clean.WriteRune(r)
			s.line++
			// A statement continues after `, |, a trailing comma or inside ( ) / @{ }.
			trimmed := strings.TrimRight(s.cur.String(), " \t\r")
			if s.parens > 0 || s.inHash() || strings.HasSuffix(trimmed, "`") || strings.HasSuffix(trimmed, "|") || strings.HasSuffix(trimmed, ",") {
				s.cur.Reset()
				s.cur.WriteString(strings.TrimSuffix(trimmed, "`") + " ")
				continue
			}
			s.emit()

		case r == '<' && at(i+1) == '#': // block comment
			start, line := i, s.line
			for i += 2; i < len(rs) && !(rs[i] == '#' && at(i+1) == '>'); i++ {
				if rs[i] == '\n' {
					s.line++
					s.clean.WriteRune('\n')
				}
			}
			i++
			s.comments = append(s.comments, comment{string(rs[start:min(i+1, len(rs))]), line})

		case r == '#':
			start := i
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			s.comments = append(s.comments, comment{strings.TrimSpace(string(rs[start:i])), s.line})
			i-- // let the newline be handled

		case r == '@' && (at(i+1) == '"' || at(i+1) == '\'') && (at(i+2) == '\n' || at(i+2) == '\r'):
			// Here-string: contents are data, never code.
			q := at(i + 1)
			s.write('@')
			s.write(q)
			for i += 2; i < len(rs); i++ {
				if rs[i] == '\n' {
					s.line++
					s.clean.WriteRune('\n')
				} else if rs[i] == q && at(i+1) == '@' && i > 0 && lineStart(i) {
					break
				}
			}
			i++
			s.write(q)
			s.write('@')

		case r == '\'' || r == '"':
			s.write(r)
			for i++; i < len(rs); i++ {
				c := rs[i]
				if c == '\n' {
					s.line++
				}
				s.write(c)
				if r == '"' && c == '`' && i+1 < len(rs) {
					i++
					s.write(rs[i])
					continue
				}
				if c == r {
					if at(i+1) == r { // doubled quote escapes itself
						i++
						s.write(rs[i])
						continue
					}
					break
				}
			}

		case r == '(':
			s.parens++
			s.write(r)
		case r == ')':
			s.parens = max(0, s.parens-1)
			s.write(r)

		case r == '{':
			if s.cur.Len() > 0 && strings.HasSuffix(s.cur.String(), "@") {
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

		case r == ';' && s.parens == 0 && !s.inHash():
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
