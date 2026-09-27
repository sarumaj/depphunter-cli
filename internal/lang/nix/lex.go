package nix

// The Nix lexer. Nix's grammar is small, but three things make a plain tokenizer
// wrong: strings ("..." and ''...'') hold ${...} code, which holds strings again; a
// path is a token of its own (./x.nix, a/b, ~/x, <nixpkgs>, and ./x/${y}.nix with
// code inside); and an unquoted URI (https://x) is a string. The lexer keeps a mode
// stack so that code inside an interpolation is ordinary tokens between tInterp and
// tInterpEnd, and the parser sees one flat token stream.

type tokKind uint8

const (
	tEOF tokKind = iota
	tIdent
	tNum
	tPunct
	tStrOpen   // " or ''
	tStrText   // literal text of a string, escapes undone
	tStrClose  // " or ''
	tInterp    // ${ in a string, a path or an attribute name
	tInterpEnd // the } closing it
	tPath      // a whole path without interpolation: ./x.nix, ../lib, a/b, ~/x, /etc/x
	tPathOpen  // the first part of a path that continues with ${...}
	tPathText  // a later literal part of such a path
	tPathClose // the end of such a path
	tSPath     // <nixpkgs/lib>, text without the brackets
	tURI       // https://example.org/x.tar.gz
)

type token struct {
	kind tokKind
	text string
	line int
}

// maxModes bounds the nesting of strings and interpolations; deeper input is read on
// as code rather than growing the stack without end.
const maxModes = 256

type mode struct {
	kind  byte // 'd' "string", 'i' ''string'', 'p' path, 'c' interpolated code
	depth int  // braces open inside interpolated code
}

type lexer struct {
	src   string
	i     int
	line  int
	modes []mode
	toks  []token
	// noPath and noURI remember where a failed path or URI look-ahead ended: a
	// later start inside the same run of characters would fail the same way, and
	// retrying would be quadratic on a long a.b.c.d... chain.
	noPath, noURI int
}

// lex splits Nix source into tokens; it never fails, and unterminated strings,
// paths and interpolations end at EOF.
//
// Implements: REQ-NIX-011
func lex(src []byte) []token {
	s := string(src)
	if len(s) >= 3 && s[:3] == "\xef\xbb\xbf" {
		s = s[3:]
	}
	l := &lexer{src: s, line: 1, toks: make([]token, 0, len(s)/5+8)}
	for l.i < len(l.src) {
		if n := len(l.modes); n > 0 {
			switch l.modes[n-1].kind {
			case 'd', 'i':
				l.str(l.modes[n-1].kind)
				continue
			case 'p':
				l.pathRest()
				continue
			}
		}
		l.code()
	}
	// Unterminated strings and paths end at EOF.
	for n := len(l.modes); n > 0; n-- {
		switch l.modes[n-1].kind {
		case 'd', 'i':
			l.emit(tStrClose, "")
		case 'p':
			l.emit(tPathClose, "")
		case 'c':
			l.emit(tInterpEnd, "")
		}
	}
	l.toks = append(l.toks, token{kind: tEOF, line: l.line})
	return l.toks
}

func (l *lexer) emit(k tokKind, text string) { l.toks = append(l.toks, token{k, text, l.line}) }

func (l *lexer) push(k byte) bool {
	if len(l.modes) >= maxModes {
		return false
	}
	l.modes = append(l.modes, mode{kind: k})
	return true
}

func (l *lexer) pop() { l.modes = l.modes[:len(l.modes)-1] }

func (l *lexer) peek(k int) byte {
	if l.i+k < len(l.src) {
		return l.src[l.i+k]
	}
	return 0
}

func isIdentStart(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isIdentChar(c byte) bool {
	return isIdentStart(c) || c >= '0' && c <= '9' || c == '\'' || c == '-'
}

func isPathChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' || c == '+'
}

func isSchemeChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'
}

func isURIChar(c byte) bool {
	if isPathChar(c) {
		return true
	}
	switch c {
	case '%', '/', '?', ':', '@', '&', '=', '$', ',', '!', '~', '*', '\'':
		return true
	}
	return false
}

// code reads one token of Nix code (or skips whitespace or a comment).
func (l *lexer) code() {
	c := l.src[l.i]
	switch {
	case c == '\n':
		l.line++
		l.i++
		return
	case c == ' ' || c == '\t' || c == '\r':
		l.i++
		return
	case c == '#':
		for l.i < len(l.src) && l.src[l.i] != '\n' {
			l.i++
		}
		return
	case c == '/' && l.peek(1) == '*':
		l.i += 2
		for l.i < len(l.src) && !(l.src[l.i] == '*' && l.peek(1) == '/') {
			if l.src[l.i] == '\n' {
				l.line++
			}
			l.i++
		}
		l.i = min(l.i+2, len(l.src))
		return
	case c == '"':
		l.emit(tStrOpen, `"`)
		l.i++
		if !l.push('d') {
			l.emit(tStrClose, "")
		}
		return
	case c == '\'' && l.peek(1) == '\'':
		l.emit(tStrOpen, "''")
		l.i += 2
		if !l.push('i') {
			l.emit(tStrClose, "")
		}
		return
	case c == '$' && l.peek(1) == '{':
		l.emit(tInterp, "${")
		l.i += 2
		if !l.push('c') {
			l.emit(tInterpEnd, "")
		}
		return
	case c == '{':
		if n := len(l.modes); n > 0 && l.modes[n-1].kind == 'c' {
			l.modes[n-1].depth++
		}
	case c == '}':
		if n := len(l.modes); n > 0 && l.modes[n-1].kind == 'c' {
			if l.modes[n-1].depth == 0 {
				l.emit(tInterpEnd, "}")
				l.i++
				l.pop()
				return
			}
			l.modes[n-1].depth--
		}
	case c == '<':
		if l.spath() {
			return
		}
	}
	if (isPathChar(c) || c == '/' || c == '~') && l.i >= l.noPath && l.path() {
		return
	}
	if isIdentStart(c) {
		if l.i >= l.noURI && l.uri() {
			return
		}
		j := l.i + 1
		for j < len(l.src) && isIdentChar(l.src[j]) {
			j++
		}
		l.emit(tIdent, l.src[l.i:j])
		l.i = j
		return
	}
	if c >= '0' && c <= '9' || c == '.' && l.peek(1) >= '0' && l.peek(1) <= '9' {
		j := l.i
		for j < len(l.src) && (l.src[j] >= '0' && l.src[j] <= '9' || l.src[j] == '.') {
			j++
		}
		if j < len(l.src) && (l.src[j] == 'e' || l.src[j] == 'E') {
			j++
			if j < len(l.src) && (l.src[j] == '+' || l.src[j] == '-') {
				j++
			}
			for j < len(l.src) && l.src[j] >= '0' && l.src[j] <= '9' {
				j++
			}
		}
		l.emit(tNum, l.src[l.i:j])
		l.i = j
		return
	}
	for _, op := range ops {
		if len(op) > 1 && l.i+len(op) <= len(l.src) && l.src[l.i:l.i+len(op)] == op {
			l.emit(tPunct, op)
			l.i += len(op)
			return
		}
	}
	if c < 0x80 {
		l.emit(tPunct, l.src[l.i:l.i+1])
	}
	l.i++ // a stray non-ASCII byte is skipped
}

var ops = []string{"...", "//", "++", "==", "!=", "<=", ">=", "&&", "||", "->", "|>", "<|"}

// path reads a path literal at l.i: path characters, then one or more /segments
// (a segment may start with ${...}). It reports false, consuming nothing, when
// there is none: "a / b" is a division, "//" an update.
func (l *lexer) path() bool {
	j := l.i
	if l.src[j] == '~' {
		if l.peek(1) != '/' {
			return false
		}
		j++
	} else {
		for j < len(l.src) && isPathChar(l.src[j]) {
			j++
		}
	}
	// At least one "/" followed by a path character or an interpolation.
	if !(j+1 < len(l.src) && l.src[j] == '/' && (isPathChar(l.src[j+1]) || l.src[j+1] == '$' && j+2 < len(l.src) && l.src[j+2] == '{')) {
		l.noPath = j
		if j == l.i {
			l.noPath = l.i + 1
		}
		return false
	}
	for j+1 < len(l.src) && l.src[j] == '/' && isPathChar(l.src[j+1]) {
		j++
		for j < len(l.src) && isPathChar(l.src[j]) {
			j++
		}
	}
	if j+2 < len(l.src) && l.src[j] == '/' && l.src[j+1] == '$' && l.src[j+2] == '{' {
		j++ // ./x/${y}: the slash belongs to the literal part
	}
	if j+1 < len(l.src) && l.src[j] == '$' && l.src[j+1] == '{' && l.push('p') {
		l.emit(tPathOpen, l.src[l.i:j])
		l.i = j
		return true
	}
	if j < len(l.src) && l.src[j] == '/' && j+1 < len(l.src) && !isPathChar(l.src[j+1]) && l.src[j+1] != '/' && l.src[j+1] != '*' {
		j++ // a trailing slash
	}
	l.emit(tPath, l.src[l.i:j])
	l.i = j
	return true
}

// pathRest continues a path after an interpolation: more literal parts and more
// interpolations, until a character that cannot be part of it.
func (l *lexer) pathRest() {
	if l.src[l.i] == '$' && l.peek(1) == '{' {
		l.emit(tInterp, "${")
		l.i += 2
		if !l.push('c') {
			l.emit(tInterpEnd, "")
		}
		return
	}
	j := l.i
	for j < len(l.src) && (isPathChar(l.src[j]) || l.src[j] == '/' && j+1 < len(l.src) && (isPathChar(l.src[j+1]) || l.src[j+1] == '$')) {
		j++
	}
	if j > l.i {
		l.emit(tPathText, l.src[l.i:j])
		l.i = j
		return
	}
	l.emit(tPathClose, "")
	l.pop()
}

// spath reads <nixpkgs> or <nixpkgs/lib> at l.i.
func (l *lexer) spath() bool {
	j := l.i + 1
	start := j
	for j < len(l.src) && (isPathChar(l.src[j]) || l.src[j] == '/' && j > start && j+1 < len(l.src) && isPathChar(l.src[j+1])) {
		j++
	}
	if j == start || j >= len(l.src) || l.src[j] != '>' {
		return false
	}
	l.emit(tSPath, l.src[start:j])
	l.i = j + 1
	return true
}

// uri reads an unquoted URI (scheme:rest, no space after the colon) at l.i.
func (l *lexer) uri() bool {
	j := l.i
	for j < len(l.src) && isSchemeChar(l.src[j]) {
		j++
	}
	if j+1 >= len(l.src) || l.src[j] != ':' || !isURIChar(l.src[j+1]) {
		l.noURI = j
		return false
	}
	k := j + 1
	for k < len(l.src) && isURIChar(l.src[k]) {
		k++
	}
	l.emit(tURI, l.src[l.i:k])
	l.i = k
	return true
}

// str reads the next piece of a string: literal text up to an interpolation or the
// closing quote. Escapes are undone ("\n", ”$ and ”'), and $$ is a literal "$$".
func (l *lexer) str(kind byte) {
	var b []byte
	start := l.line
	flush := func() {
		if len(b) > 0 {
			l.toks = append(l.toks, token{tStrText, string(b), start})
		}
	}
	for l.i < len(l.src) {
		c := l.src[l.i]
		switch {
		case c == '\n':
			l.line++
		case c == '$' && l.peek(1) == '$':
			b = append(b, '$', '$')
			l.i += 2
			continue
		case c == '$' && l.peek(1) == '{':
			flush()
			l.emit(tInterp, "${")
			l.i += 2
			if !l.push('c') {
				l.emit(tInterpEnd, "")
			}
			return
		case kind == 'd' && c == '\\' && l.i+1 < len(l.src):
			if e := l.src[l.i+1]; e == 'n' {
				b = append(b, '\n')
			} else if e == 't' {
				b = append(b, '\t')
			} else if e == 'r' {
				b = append(b, '\r')
			} else {
				if e == '\n' {
					l.line++
				}
				b = append(b, e)
			}
			l.i += 2
			continue
		case kind == 'd' && c == '"':
			flush()
			l.emit(tStrClose, `"`)
			l.i++
			l.pop()
			return
		case kind == 'i' && c == '\'' && l.peek(1) == '\'':
			switch e := l.peek(2); {
			case e == '\'':
				b = append(b, '\'', '\'')
				l.i += 3
				continue
			case e == '$':
				b = append(b, '$')
				l.i += 3
				continue
			case e == '\\' && l.i+3 < len(l.src):
				if l.src[l.i+3] == '\n' {
					l.line++
				}
				b = append(b, l.src[l.i+3])
				l.i += 4
				continue
			}
			flush()
			l.emit(tStrClose, "''")
			l.i += 2
			l.pop()
			return
		}
		b = append(b, c)
		l.i++
	}
	flush()
}
