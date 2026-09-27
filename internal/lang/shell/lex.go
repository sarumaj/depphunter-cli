package shell

import (
	"strings"
)

// The scanner reads a shell script as the shell does, as far as dependencies need:
// words with their quoting, expansions and command substitutions, simple commands
// split on the control operators, here-documents skipped, and just enough of the
// compound commands (braces, subshells, case, [[ ]], (( ))) to know where a command
// starts and whether it runs inside a function body. It never fails: text it does
// not understand ends up in some word, and a later command starts cleanly on the
// next line. The vendored tree-sitter bash grammar was measured and not used
// (REQ-SHELL-010).

// partKind says what a piece of a word is.
type partKind int

const (
	pLit   partKind = iota // literal text, quotes removed
	pParam                 // $name or ${...}: text is the name or the braces' content
	pSub                   // $(...) or `...`: cmds are the commands inside
	pOther                 // arithmetic, a process substitution, an array or glob group
)

type part struct {
	kind   partKind
	text   string
	quoted bool
	cmds   []*command
}

// word is one shell word: the parts it is made of and where it starts.
type word struct {
	parts []part
	raw   string // the source text, quotes and all
	line  int
}

// lit is the word's text when it is a plain unquoted literal: what reserved words and
// command names are compared against.
func (w *word) lit() (string, bool) {
	var b strings.Builder
	for _, p := range w.parts {
		if p.kind != pLit || p.quoted {
			return "", false
		}
		b.WriteString(p.text)
	}
	return b.String(), true
}

// text is the word with quotes removed when it has no expansions: `'ls -l'`,
// "x.sh" and x.sh are all plain text.
func (w *word) text() (string, bool) {
	var b strings.Builder
	for _, p := range w.parts {
		if p.kind != pLit {
			return "", false
		}
		b.WriteString(p.text)
	}
	return b.String(), true
}

// command is a simple command: its words, the assignments before them, and where it
// runs. Redirections and their targets are not words.
type command struct {
	assigns []*word
	words   []*word
	line    int
	inFunc  bool // inside a function body
	sub     bool // inside a command or process substitution
}

type frameKind int

const (
	fBrace frameKind = iota // { ... }
	fParen                  // ( ... )
	fCase                   // case ... esac
)

type frame struct {
	kind    frameKind
	fn      bool // the body of a function
	pattern bool // case: the next thing is a pattern list
}

type heredoc struct {
	delim string
	strip bool // <<-: leading tabs are removed from every line, the delimiter's too
}

// sink receives what the parser finds, in the order the shell would meet it: the
// commands of a substitution before the command using it.
type sink interface {
	command(c *command)
	function(name, kind string, line int)
}

type parser struct {
	src      []byte
	i, line  int
	heredocs []heredoc
	frames   []frame
	outerFn  bool   // the substitution being parsed sits in a function body
	sub      int    // depth of substitutions being parsed
	pending  string // a function whose body is expected next
	back     *token // a token put back
	out      sink
}

func newParser(src []byte, line int, out sink) *parser {
	return &parser{src: src, line: line, out: out}
}

type tokKind int

const (
	tWord tokKind = iota
	tOp
	tNL
	tEOF
)

type token struct {
	kind tokKind
	op   string
	w    *word
	line int
}

func (p *parser) at(off int) byte {
	if p.i+off < len(p.src) {
		return p.src[p.i+off]
	}
	return 0
}

func (p *parser) inFunc() bool {
	if p.outerFn {
		return true
	}
	for _, f := range p.frames {
		if f.fn {
			return true
		}
	}
	return false
}

// script parses the whole source.
//
// Implements: REQ-SHELL-002, REQ-SHELL-010
func (p *parser) script() {
	for p.i < len(p.src) {
		p.list(false)
		if p.i < len(p.src) { // an unmatched ")" at the top level
			p.i++
		}
	}
}

// list parses commands up to the end of the source or, in a substitution, the ")"
// that closes it, which it consumes. It returns the list's own simple commands.
func (p *parser) list(nested bool) []*command {
	saved, savedFn, savedPending := p.frames, p.outerFn, p.pending
	if nested {
		p.outerFn = p.inFunc()
		p.frames, p.pending = nil, ""
		p.sub++
	}
	defer func() {
		if nested {
			p.frames, p.outerFn, p.pending = saved, savedFn, savedPending
			p.sub--
		}
	}()
	var cmds []*command
	for {
		c, end := p.command(nested)
		if c != nil {
			cmds = append(cmds, c)
			p.out.command(c)
		}
		if end {
			return cmds
		}
	}
}

func (p *parser) top() *frame {
	if len(p.frames) == 0 {
		return nil
	}
	return &p.frames[len(p.frames)-1]
}

// popTo removes frames down to and including the innermost one of kind k, when
// there is one: an unbalanced frame above it is dropped with it.
func (p *parser) popTo(k frameKind) bool {
	for j := len(p.frames) - 1; j >= 0; j-- {
		if p.frames[j].kind == k {
			p.frames = p.frames[:j]
			return true
		}
	}
	return false
}

func (p *parser) push(k frameKind) {
	p.frames = append(p.frames, frame{kind: k, fn: p.pending != "" && k != fCase})
	p.pending = ""
}

// function records a function definition; its body is the next brace group or
// subshell.
func (p *parser) function(name string, line int) {
	if name == "" || name == "{" {
		p.pending = "{anonymous}"
		return
	}
	p.out.function(name, "function", line)
	p.pending = name
}

var reservedSkip = map[string]bool{
	"if": true, "then": true, "elif": true, "else": true, "do": true, "while": true,
	"until": true, "!": true, "time": true, "fi": true, "done": true, "coproc": true,
}

// command reads one simple command, handling the compound-command syntax around it.
// end reports the end of the list (the source, or a substitution's ")").
//
// Implements: REQ-SHELL-002, REQ-SHELL-003
func (p *parser) command(nested bool) (c *command, end bool) {
	var assigns, words []*word
	line := 0
	finish := func() *command {
		if len(words) == 0 && len(assigns) == 0 {
			return nil
		}
		// A bats test is a brace group whose header is "@test NAME {".
		if len(words) > 2 {
			first, _ := words[0].lit()
			last, _ := words[len(words)-1].lit()
			if first == "@test" && last == "{" {
				name, _ := words[1].text()
				p.out.function(name, "test", words[0].line)
				p.pending = name
				p.push(fBrace)
				words = words[:len(words)-1]
			}
		}
		return &command{assigns: assigns, words: words, line: line, inFunc: p.inFunc(), sub: p.sub > 0}
	}
	for {
		start := len(words) == 0 && len(assigns) == 0
		if t := p.top(); start && t != nil && t.kind == fCase && t.pattern {
			if p.casePattern() {
				continue
			}
		}
		t := p.token()
		switch t.kind {
		case tEOF:
			return finish(), true
		case tNL:
			if start {
				continue
			}
			return finish(), false
		case tOp:
			switch t.op {
			case ";", "&", "&&", "||", "|", "|&":
				if start {
					continue
				}
				return finish(), false
			case ";;", ";&", ";;&":
				c := finish()
				for j := len(p.frames) - 1; j >= 0; j-- {
					if p.frames[j].kind == fCase {
						p.frames = p.frames[:j+1]
						p.frames[j].pattern = true
						break
					}
				}
				return c, false
			case "(":
				if start {
					p.push(fParen)
					continue
				}
				if len(words) == 1 && len(assigns) == 0 && p.peek() == ')' {
					p.token() // name ( )
					name, _ := words[0].text()
					p.function(name, words[0].line)
					words = nil
					continue
				}
			case "((":
				p.skipArith(2)
				if start {
					continue
				}
			case ")":
				if t := p.top(); t != nil && t.kind == fParen {
					p.frames = p.frames[:len(p.frames)-1]
					if start {
						continue
					}
					return finish(), false
				}
				if nested {
					return finish(), true
				}
			case "<<", "<<-":
				if w := p.token(); w.kind == tWord {
					d, _ := w.w.text()
					if d == "" {
						d = strings.Trim(w.w.raw, `'"`)
					}
					p.heredocs = append(p.heredocs, heredoc{delim: d, strip: t.op == "<<-"})
				} else {
					p.unread(w)
				}
			default: // a redirection: the next word is its target, not an argument
				if w := p.token(); w.kind != tWord {
					p.unread(w)
				}
			}
		case tWord:
			if len(words) == 0 && isAssign(t.w) {
				if line == 0 {
					line = t.line
				}
				assigns = append(assigns, t.w)
				continue
			}
			if !start {
				words = append(words, t.w)
				continue
			}
			lit, _ := t.w.lit()
			switch {
			case reservedSkip[lit]:
				continue
			case lit == "{":
				p.push(fBrace)
				continue
			case lit == "}":
				p.popTo(fBrace)
				continue
			case lit == "esac":
				p.popTo(fCase)
				continue
			case lit == "case":
				p.token() // the subject
				if w := p.token(); w.kind != tWord {
					p.unread(w)
				}
				p.frames = append(p.frames, frame{kind: fCase, pattern: true})
				continue
			case lit == "for" || lit == "select":
				p.skipHeader()
				continue
			case lit == "[[":
				p.skipTest()
				continue
			case lit == "function":
				if w := p.token(); w.kind == tWord {
					name, _ := w.w.text()
					p.function(name, w.line)
					if name == "{" {
						p.push(fBrace)
						continue
					}
					if p.peek() == '(' {
						p.token()
						if p.peek() == ')' {
							p.token()
						}
					}
				} else {
					p.unread(w)
				}
				continue
			}
			if p.pending != "" {
				p.pending = "" // a function whose body is not a group: not followed
			}
			line = t.line
			words = append(words, t.w)
		}
	}
}

// casePattern consumes a case item's pattern list up to its ")", or "esac". It
// reports whether it consumed anything.
func (p *parser) casePattern() bool {
	for {
		t := p.token()
		switch t.kind {
		case tEOF:
			p.unread(t)
			return false
		case tNL:
			continue
		case tWord:
			if lit, _ := t.w.lit(); lit == "esac" {
				p.popTo(fCase)
				return true
			}
		case tOp:
			if t.op == ")" {
				p.top().pattern = false
				return true
			}
		}
	}
}

// skipHeader skips a for/select header up to the separator before "do".
func (p *parser) skipHeader() {
	for {
		t := p.token()
		switch {
		case t.kind == tEOF || t.kind == tNL:
			return
		case t.kind == tOp && t.op == "((":
			p.skipArith(2)
		case t.kind == tOp && (t.op == ";" || t.op == "&"):
			return
		case t.kind == tWord:
			if lit, _ := t.w.lit(); lit == "do" {
				return
			}
		}
	}
}

// skipTest skips a [[ ... ]] conditional, whose && and || are not command separators.
func (p *parser) skipTest() {
	for {
		t := p.token()
		if t.kind == tEOF {
			return
		}
		if t.kind == tWord {
			if lit, _ := t.w.lit(); lit == "]]" {
				return
			}
		}
	}
}

// unread puts one token back, to be returned by the next call to token.
func (p *parser) unread(t token) {
	if t.kind == tEOF {
		return
	}
	p.back = &t
}

// peek returns the next non-blank byte without consuming it.
func (p *parser) peek() byte {
	j := p.i
	for j < len(p.src) && (p.src[j] == ' ' || p.src[j] == '\t') {
		j++
	}
	if j < len(p.src) {
		return p.src[j]
	}
	return 0
}

func (p *parser) skipBlanks() {
	for p.i < len(p.src) {
		switch c := p.src[p.i]; {
		case c == ' ' || c == '\t' || c == '\r':
			p.i++
		case c == '\\' && p.at(1) == '\n':
			p.i += 2
			p.line++
		default:
			return
		}
	}
}

// token reads the next word or operator.
func (p *parser) token() token {
	if p.back != nil {
		t := *p.back
		p.back = nil
		return t
	}
	for {
		p.skipBlanks()
		if p.i >= len(p.src) {
			return token{kind: tEOF, line: p.line}
		}
		if p.src[p.i] == '#' {
			for p.i < len(p.src) && p.src[p.i] != '\n' {
				p.i++
			}
			continue
		}
		break
	}
	line := p.line
	c := p.src[p.i]
	op := func(s string) token {
		p.i += len(s)
		return token{kind: tOp, op: s, line: line}
	}
	has := func(s string) bool { return strings.HasPrefix(string(p.src[p.i:min(p.i+len(s), len(p.src))]), s) }
	switch c {
	case '\n':
		p.i++
		p.line++
		p.readHeredocs()
		return token{kind: tNL, line: line}
	case ';':
		for _, s := range []string{";;&", ";;", ";&", ";"} {
			if has(s) {
				return op(s)
			}
		}
	case '&':
		for _, s := range []string{"&&", "&>>", "&>", "&"} {
			if has(s) {
				return op(s)
			}
		}
	case '|':
		for _, s := range []string{"||", "|&", "|"} {
			if has(s) {
				return op(s)
			}
		}
	case '(':
		if has("((") {
			return op("((")
		}
		return op("(")
	case ')':
		return op(")")
	case '<', '>':
		if p.at(1) == '(' { // a process substitution is a word
			break
		}
		return op(p.redirection())
	}
	if c >= '0' && c <= '9' { // 2>&1: a descriptor number starts the redirection
		j := p.i
		for j < len(p.src) && p.src[j] >= '0' && p.src[j] <= '9' {
			j++
		}
		if j < len(p.src) && (p.src[j] == '<' || p.src[j] == '>') && (j+1 >= len(p.src) || p.src[j+1] != '(') {
			p.i = j
			return op(p.redirection())
		}
	}
	return token{kind: tWord, w: p.word(), line: line}
}

func (p *parser) redirection() string {
	for _, s := range []string{"<<<", "<<-", "<<", "<>", "<&", "<", ">>", ">&", ">|", ">"} {
		if strings.HasPrefix(string(p.src[p.i:min(p.i+len(s), len(p.src))]), s) {
			return s
		}
	}
	return string(p.src[p.i])
}

// readHeredocs skips the bodies of the here-documents the line just ended opened.
func (p *parser) readHeredocs() {
	for _, h := range p.heredocs {
		for p.i < len(p.src) {
			end := p.i
			for end < len(p.src) && p.src[end] != '\n' {
				end++
			}
			l := strings.TrimSuffix(string(p.src[p.i:end]), "\r")
			if h.strip {
				l = strings.TrimLeft(l, "\t")
			}
			p.i = min(end+1, len(p.src))
			p.line++
			if l == h.delim {
				break
			}
		}
	}
	p.heredocs = nil
}

func isMeta(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', ';', '&', '|', '<', '>', '(', ')':
		return true
	}
	return false
}

// word reads one word.
func (p *parser) word() *word {
	w := &word{line: p.line}
	start := p.i
	var lit strings.Builder
	litQuoted := false
	flush := func() {
		if lit.Len() > 0 {
			w.parts = append(w.parts, part{kind: pLit, text: lit.String(), quoted: litQuoted})
			lit.Reset()
		}
	}
	addLit := func(s string, quoted bool) {
		if lit.Len() > 0 && litQuoted != quoted {
			flush()
		}
		litQuoted = quoted
		lit.WriteString(s)
	}
	add := func(pt part) {
		flush()
		w.parts = append(w.parts, pt)
	}
	for p.i < len(p.src) {
		c := p.src[p.i]
		if (c == '<' || c == '>') && p.at(1) == '(' { // <(cmd): a process substitution
			if p.i > start {
				break
			}
			p.i += 2
			add(part{kind: pOther, cmds: p.list(true)})
			continue
		}
		if c == '(' {
			empty := p.i == start
			if empty || p.peekAfter(p.i+1) == ')' && p.src[p.i-1] != '=' {
				break // "(" starts a subshell, or "name()" a function
			}
			// name=( array ), zsh glob qualifiers *(.), extglob @(a|b): one word.
			p.skipGroup()
			add(part{kind: pOther})
			continue
		}
		if isMeta(c) {
			break
		}
		switch c {
		case '\\':
			if p.at(1) == '\n' {
				p.i += 2
				p.line++
				continue
			}
			if p.i+1 < len(p.src) {
				addLit(string(p.src[p.i+1]), true)
			}
			p.i += 2
		case '\'':
			p.i++
			s := p.i
			for p.i < len(p.src) && p.src[p.i] != '\'' {
				if p.src[p.i] == '\n' {
					p.line++
				}
				p.i++
			}
			addLit(string(p.src[s:p.i]), true)
			p.i++
		case '"':
			p.i++
			p.double(addLit, add)
		case '`':
			add(p.backtick())
		case '$':
			if p.at(1) == '\'' { // $'...': ANSI-C quoting
				p.i += 2
				var b strings.Builder
				for p.i < len(p.src) && p.src[p.i] != '\'' {
					if p.src[p.i] == '\\' && p.i+1 < len(p.src) {
						p.i++
					}
					if p.src[p.i] == '\n' {
						p.line++
					}
					b.WriteByte(p.src[p.i])
					p.i++
				}
				p.i++
				addLit(b.String(), true)
				continue
			}
			if p.at(1) == '"' {
				p.i += 2
				p.double(addLit, add)
				continue
			}
			if pt, ok := p.dollar(); ok {
				add(pt)
			} else {
				addLit("$", false)
				p.i++
			}
		default:
			addLit(string(c), false)
			p.i++
		}
	}
	flush()
	p.i = min(p.i, len(p.src)) // an unterminated quote or escape at the end
	w.raw = string(p.src[start:p.i])
	return w
}

// peekAfter returns the first non-blank byte at or after j.
func (p *parser) peekAfter(j int) byte {
	for j < len(p.src) && (p.src[j] == ' ' || p.src[j] == '\t') {
		j++
	}
	if j < len(p.src) {
		return p.src[j]
	}
	return 0
}

// double reads a double-quoted string after its opening quote.
func (p *parser) double(addLit func(string, bool), add func(part)) {
	for p.i < len(p.src) {
		c := p.src[p.i]
		switch c {
		case '"':
			p.i++
			return
		case '\\':
			if n := p.at(1); n == '$' || n == '`' || n == '"' || n == '\\' {
				addLit(string(n), true)
				p.i += 2
				continue
			} else if n == '\n' {
				p.i += 2
				p.line++
				continue
			}
			addLit("\\", true)
			p.i++
		case '`':
			add(p.backtick())
		case '$':
			if pt, ok := p.dollar(); ok {
				add(pt)
			} else {
				addLit("$", true)
				p.i++
			}
		default:
			if c == '\n' {
				p.line++
			}
			addLit(string(c), true)
			p.i++
		}
	}
}

// dollar reads an expansion at a "$": a parameter, a command substitution or an
// arithmetic expansion. ok is false for a "$" that is just a dollar sign.
func (p *parser) dollar() (part, bool) {
	switch n := p.at(1); {
	case n == '(' && p.at(2) == '(':
		p.i++
		p.skipArith(2)
		return part{kind: pOther}, true
	case n == '(':
		p.i += 2
		return part{kind: pSub, cmds: p.list(true)}, true
	case n == '[': // $[ ]: old arithmetic
		p.i += 2
		for p.i < len(p.src) && p.src[p.i] != ']' {
			p.i++
		}
		p.i++
		return part{kind: pOther}, true
	case n == '{':
		p.i += 2
		s := p.i
		p.skipBraces()
		p.i = min(p.i, len(p.src))
		return part{kind: pParam, text: string(p.src[s:max(s, p.i-1)])}, true
	case n == '_' || n >= 'a' && n <= 'z' || n >= 'A' && n <= 'Z':
		p.i++
		s := p.i
		for p.i < len(p.src) && (p.src[p.i] == '_' || isAlnum(p.src[p.i])) {
			p.i++
		}
		return part{kind: pParam, text: string(p.src[s:p.i])}, true
	case n >= '0' && n <= '9' || strings.IndexByte("@*#?$!-", n) >= 0 && n != 0:
		p.i += 2
		return part{kind: pParam, text: string(n)}, true
	}
	return part{}, false
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// skipBraces skips a ${...} body after its "{", through the matching "}".
func (p *parser) skipBraces() {
	depth := 1
	for p.i < len(p.src) && depth > 0 {
		switch c := p.src[p.i]; c {
		case '\\':
			p.i++
		case '\n':
			p.line++
		case '"':
			p.i++
			for p.i < len(p.src) && p.src[p.i] != '"' {
				if p.src[p.i] == '\\' {
					p.i++
				} else if p.src[p.i] == '\n' {
					p.line++
				}
				p.i++
			}
		case '{':
			depth++
		case '}':
			depth--
		}
		p.i++
	}
}

// skipArith skips an arithmetic expression opened by n "(" at the current position.
func (p *parser) skipArith(n int) {
	if p.i+n <= len(p.src) && strings.Repeat("(", n) == string(p.src[p.i:p.i+n]) {
		p.i += n
	}
	depth := n
	for p.i < len(p.src) && depth > 0 {
		switch p.src[p.i] {
		case '(':
			depth++
		case ')':
			depth--
		case '\n':
			p.line++
		}
		p.i++
	}
}

// skipGroup skips a parenthesised group inside a word (an array's elements, a glob
// qualifier), quotes and comments included.
func (p *parser) skipGroup() {
	depth := 0
	prev := byte(' ')
	for p.i < len(p.src) {
		c := p.src[p.i]
		switch {
		case c == '\\':
			p.i++
		case c == '\n':
			p.line++
		case c == '\'' || c == '"':
			p.i++
			for p.i < len(p.src) && p.src[p.i] != c {
				if c == '"' && p.src[p.i] == '\\' {
					p.i++
				} else if p.src[p.i] == '\n' {
					p.line++
				}
				p.i++
			}
		case c == '#' && (prev == ' ' || prev == '\t' || prev == '\n'):
			for p.i < len(p.src) && p.src[p.i] != '\n' {
				p.i++
			}
			continue
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				p.i++
				return
			}
		}
		prev = c
		p.i++
	}
}

// backtick reads a `...` command substitution and parses its commands, with the
// backslashes that quote inside it removed.
func (p *parser) backtick() part {
	p.i++
	line := p.line
	var b strings.Builder
	for p.i < len(p.src) && p.src[p.i] != '`' {
		c := p.src[p.i]
		if c == '\\' && p.i+1 < len(p.src) {
			if n := p.src[p.i+1]; n == '`' || n == '\\' || n == '$' {
				b.WriteByte(n)
				p.i += 2
				continue
			}
		}
		if c == '\n' {
			p.line++
		}
		b.WriteByte(c)
		p.i++
	}
	p.i++
	q := newParser([]byte(b.String()), line, p.out)
	q.outerFn, q.sub = p.inFunc(), p.sub+1
	cmds := q.list(false)
	return part{kind: pSub, cmds: cmds}
}

// isAssign reports whether w is NAME=value, NAME+=value or NAME[i]=value.
func isAssign(w *word) bool {
	name, _, ok := splitAssign(w)
	return ok && name != ""
}

// splitAssign splits an assignment word into the variable's name and its value.
func splitAssign(w *word) (string, *word, bool) {
	if len(w.parts) == 0 || w.parts[0].kind != pLit || w.parts[0].quoted {
		return "", nil, false
	}
	first := w.parts[0].text
	eq := strings.IndexByte(first, '=')
	if eq <= 0 {
		return "", nil, false
	}
	name := strings.TrimSuffix(first[:eq], "+")
	if i := strings.IndexByte(name, '['); i > 0 && strings.HasSuffix(name, "]") {
		name = name[:i]
	}
	if !validName(name) {
		return "", nil, false
	}
	val := &word{line: w.line, raw: w.raw[min(len(w.raw), strings.IndexByte(w.raw, '=')+1):]}
	if rest := first[eq+1:]; rest != "" {
		val.parts = append(val.parts, part{kind: pLit, text: rest})
	}
	val.parts = append(val.parts, w.parts[1:]...)
	return name, val, true
}

func validName(s string) bool {
	if s == "" || s[0] >= '0' && s[0] <= '9' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '_' && !isAlnum(s[i]) {
			return false
		}
	}
	return true
}
