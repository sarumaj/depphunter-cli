package lang

import (
	"bytes"
	"strconv"
	"strings"
)

// KeyLines is where a TOML document defines each key, which BurntSushi/toml
// does not report. A table is named by its key parts joined with dots, the top
// level by "".
type KeyLines struct {
	lines       map[[2]string]int       // {table, key} -> line
	definitions map[string][]definition // key -> its tables, in file order
	elements    map[string][]int        // array of tables -> each [[header]]'s line
}

type definition struct {
	table string
	line  int
}

// TOMLKeyLines scans source, which should be TOML the decoder accepted. A
// key's line is where its table first defines it: its own `key = ...`, the
// first dotted key through it (`key.git = ...`) or the first header through it
// (`[table.key]`, `[table.key.sub]`). A key of an inline table belongs to the
// table that value is (`x = { a = 1 }` defines a in x), and what a string
// holds is never a key.
//
// Implements: REQ-LANG-033
func TOMLKeyLines(source []byte) *KeyLines {
	k := &KeyLines{lines: map[[2]string]int{}, definitions: map[string][]definition{}, elements: map[string][]int{}}
	s := &tomlScanner{keys: k, source: bytes.TrimPrefix(source, []byte("\ufeff")), line: 1}
	table := ""
	for s.position < len(s.source) {
		s.skipBlank()
		line := s.line
		switch s.peek() {
		case '\n':
			s.next()
		case '#':
			s.skipLine()
		case '[':
			array := s.has("[[")
			closing := "]"
			if array {
				closing = "]]"
			}
			s.position += len(closing)
			path, ok := s.key()
			if s.skipBlank(); ok && s.has(closing) {
				table = k.define("", path, line)
				if array {
					k.elements[table] = append(k.elements[table], line)
				}
			}
			s.skipLine()
		default:
			path, ok := s.key()
			if s.skipBlank(); !ok || s.peek() != '=' {
				s.skipLine()
				continue
			}
			s.position++
			s.value(k.define(table, path, line))
		}
	}
	return k
}

// define records each part of a dotted key or header path in the table the
// parts before it name: a.b.c in t defines a in t, b in t.a and c in t.a.b.
// It returns the table the whole path names.
func (k *KeyLines) define(table string, path []string, line int) string {
	for _, part := range path {
		if _, ok := k.lines[[2]string{table, part}]; !ok {
			k.lines[[2]string{table, part}] = line
			k.definitions[part] = append(k.definitions[part], definition{table, line})
		}
		if table != "" {
			table += "."
		}
		table += part
	}
	return table
}

// Line is the line that first defines key in table, 0 when none does.
func (k *KeyLines) Line(table, key string) int {
	return k.lines[[2]string{table, key}]
}

// Within is the first line that defines key in table or in any table below
// it, so "" looks in every table; 0 when none does.
func (k *KeyLines) Within(table, key string) int {
	for _, d := range k.definitions[key] {
		if table == "" || d.table == table || strings.HasPrefix(d.table, table+".") {
			return d.line
		}
	}
	return 0
}

// Elements is the header line of each [[table]] element, in order.
func (k *KeyLines) Elements(table string) []int {
	return k.elements[table]
}

type tomlScanner struct {
	keys     *KeyLines
	source   []byte
	position int
	line     int
}

func (s *tomlScanner) peek() byte {
	if s.position < len(s.source) {
		return s.source[s.position]
	}
	return 0
}

func (s *tomlScanner) has(prefix string) bool {
	return bytes.HasPrefix(s.source[s.position:], []byte(prefix))
}

func (s *tomlScanner) next() {
	if s.position < len(s.source) {
		if s.source[s.position] == '\n' {
			s.line++
		}
		s.position++
	}
}

// skipBlank skips spaces and tabs, and the \r of a CRLF line ending.
func (s *tomlScanner) skipBlank() {
	for c := s.peek(); c == ' ' || c == '\t' || c == '\r'; c = s.peek() {
		s.position++
	}
}

// skipLine skips to the end of the line, leaving its newline.
func (s *tomlScanner) skipLine() {
	if i := bytes.IndexByte(s.source[s.position:], '\n'); i >= 0 {
		s.position += i
	} else {
		s.position = len(s.source)
	}
}

// key reads a dotted key, whitespace allowed around its dots.
func (s *tomlScanner) key() ([]string, bool) {
	var path []string
	for {
		s.skipBlank()
		part, ok := s.keyPart()
		if !ok {
			return nil, false
		}
		path = append(path, part)
		if s.skipBlank(); s.peek() != '.' {
			return path, true
		}
		s.position++
	}
}

func (s *tomlScanner) keyPart() (string, bool) {
	start := s.position
	if quote := s.peek(); quote == '"' || quote == '\'' {
		s.skipString()
		raw := string(s.source[start:s.position])
		if len(raw) < 2 || raw[len(raw)-1] != quote {
			return "", false
		}
		if unquoted, err := strconv.Unquote(raw); err == nil && quote == '"' {
			return unquoted, true
		}
		return raw[1 : len(raw)-1], true
	}
	for c := s.peek(); c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'; c = s.peek() {
		s.position++
	}
	return string(s.source[start:s.position]), s.position > start
}

// value skips the value after "=" to the end of its last line, defining the
// keys of its inline tables in table. Arrays may span lines and hold comments,
// and a bracket, brace or "#" inside a string is text, so the scanner tracks
// nesting and strings.
func (s *tomlScanner) value(table string) {
	depth := 0
	for s.position < len(s.source) {
		switch s.peek() {
		case '"', '\'':
			s.skipString()
			continue
		case '#':
			s.skipLine()
			continue
		case '{':
			s.position++
			s.inlineTable(table)
			continue
		case '[':
			depth++
		case ']':
			depth--
		case ',', '}', '\n':
			// The end of this value, unless an array holds what ends it.
			if depth <= 0 {
				return
			}
		}
		s.next()
	}
}

// inlineTable reads the key/value pairs after "{" through its "}".
func (s *tomlScanner) inlineTable(table string) {
	for s.position < len(s.source) {
		s.skipBlank()
		line := s.line
		switch s.peek() {
		case '}':
			s.position++
			return
		case ',', '\n':
			s.next()
			continue
		case '#':
			s.skipLine()
			continue
		}
		path, ok := s.key()
		if s.skipBlank(); !ok || s.peek() != '=' {
			s.next()
			continue
		}
		s.position++
		s.value(s.keys.define(table, path, line))
	}
}

// skipString skips a basic or literal string, one-line or multi-line.
func (s *tomlScanner) skipString() {
	quote := s.peek()
	if triple := strings.Repeat(string(quote), 3); s.has(triple) {
		s.position += 3
		for s.position < len(s.source) {
			if quote == '"' && s.peek() == '\\' {
				s.position++
				s.next() // an escaped character, or a line-ending backslash's newline
				continue
			}
			if s.has(triple) {
				s.position += 3
				// Up to two more quotes are content before the closing three.
				for range 2 {
					if s.peek() == quote {
						s.position++
					}
				}
				return
			}
			s.next()
		}
		return
	}
	s.position++
	for s.position < len(s.source) && s.peek() != '\n' {
		c := s.peek()
		s.position++
		if c == quote {
			return
		}
		if c == '\\' && quote == '"' && s.peek() != '\n' {
			s.next()
		}
	}
}
