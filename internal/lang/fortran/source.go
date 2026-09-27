package fortran

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindUse          = "use"           // use m, use :: m
	kindIntrinsic    = "intrinsic"     // use, intrinsic :: m
	kindNonIntrinsic = "non_intrinsic" // use, non_intrinsic :: m
	kindSubmodule    = "submodule"     // submodule (ancestor[:parent]) name
	kindInclude      = "include"       // include 'file'
	kindCpp          = "cpp"           // #include "file"
	kindCppSys       = "cpp-sys"       // #include <file>
	kindFypp         = "fypp"          // #:include "file" (fypp templates)
)

// statement is one logical Fortran statement: continuation lines joined,
// comments removed, statements separated by `;` split, string literals kept as
// written. line is where it starts.
type statement struct {
	text string
	line int
}

// directive is a preprocessor line (#include "x", #:include "x"), which is not a
// Fortran statement.
type directive struct {
	text string
	line int
}

// maxStatement bounds a statement's text: nothing read from a statement lies
// past its first few tokens, and a string or continuation run that never ends
// must not make one statement the size of the file.
const maxStatement = 4096

// reader turns source lines into statements, in free or fixed source form.
type reader struct {
	stmts []statement
	dirs  []directive
	buf   strings.Builder
	start int  // line of the statement being built, 0 for none
	quote byte // the open string's quote, 0 outside strings
}

func (r *reader) add(c byte, line int) {
	if r.start == 0 {
		if c == ' ' || c == '\t' {
			return
		}
		r.start = line
	}
	if r.buf.Len() < maxStatement {
		r.buf.WriteByte(c)
	}
}

func (r *reader) flush() {
	if r.start != 0 {
		if t := strings.TrimSpace(r.buf.String()); t != "" {
			r.stmts = append(r.stmts, statement{text: t, line: r.start})
		}
	}
	r.buf.Reset()
	r.start = 0
	r.quote = 0
}

// directiveLine reports whether a line is a preprocessor line: `#` first on the
// line (cpp, and fypp's #: directives), or fypp's `$:` and `@:` lines.
func directiveLine(line string) bool {
	t := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(t, "#") || strings.HasPrefix(t, "$:") || strings.HasPrefix(t, "@:")
}

// splitLines splits src into lines without their line terminators.
func splitLines(src []byte) []string {
	s := string(src)
	s = strings.TrimPrefix(s, "\xef\xbb\xbf")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

// readDirective records a preprocessor line and returns how many more lines it
// takes up: a line ending in a backslash continues.
func (r *reader) readDirective(lines []string, i int) int {
	text := strings.TrimSpace(lines[i])
	n := 0
	for strings.HasSuffix(text, "\\") && i+n+1 < len(lines) && n < 64 {
		n++
		text = strings.TrimSuffix(text, "\\") + " " + strings.TrimSpace(lines[i+n])
	}
	r.dirs = append(r.dirs, directive{text: text, line: i + 1})
	return n
}

// ompSentinel reports whether a comment line is OpenMP conditional compilation
// (`!$ use omp_lib`): code that is compiled when OpenMP is on, not a directive
// such as `!$omp parallel`.
func ompSentinel(rest string) bool {
	return rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '&'
}

// readFree reads free source form (.f90 and later): `!` comments, `&` at the end
// of a line continues the statement (and a leading `&` on the next line is
// skipped), `;` separates statements, strings in either quote with the quote
// doubled inside.
//
// Implements: REQ-FORTRAN-002
func readFree(src []byte) *reader {
	r := &reader{}
	lines := splitLines(src)
	cont := false
	for i := 0; i < len(lines); i++ {
		line, ln := lines[i], i+1
		if r.quote == 0 && directiveLine(line) {
			i += r.readDirective(lines, i)
			continue
		}
		j := 0
		t := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(t, "!$") && ompSentinel(t[2:]) {
			line = strings.Replace(line, "!$", "  ", 1)
			t = strings.TrimLeft(line, " \t")
		}
		if cont {
			if r.quote == 0 && (t == "" || t[0] == '!') {
				continue // a comment or blank line between continuation lines
			}
			j = len(line) - len(t)
			if j < len(line) && line[j] == '&' {
				j++
			}
		}
		cont = false
	scan:
		for ; j < len(line); j++ {
			c := line[j]
			if r.quote != 0 {
				if c == r.quote {
					if j+1 < len(line) && line[j+1] == r.quote {
						r.add(c, ln)
						r.add(c, ln)
						j++
						continue
					}
					r.quote = 0
				} else if c == '&' && nextCode(line, j+1) == len(line) {
					cont = true
					break scan
				}
				r.add(c, ln)
				continue
			}
			switch c {
			case '\'', '"':
				r.quote = c
			case '!':
				break scan
			case ';':
				r.flush()
				continue
			case '&':
				if k := nextCode(line, j+1); k == len(line) || line[k] == '!' {
					cont = true
					break scan
				}
			}
			r.add(c, ln)
		}
		if !cont {
			r.flush()
		} else {
			r.add(' ', ln)
		}
	}
	r.flush()
	return r
}

// nextCode is the index of the first character from i on that is not a blank.
func nextCode(line string, i int) int {
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// readFixed reads fixed source form (.f, .for, .f77): a C, c, *, ! (or D, d
// debug line) in column 1 comments the line, columns 1-5 hold a label, a
// character other than blank or zero in column 6 continues the previous
// statement, and code starts in column 7. A tab in the first columns starts the
// code at once, or continues the statement when a digit follows it. `!` starts
// a comment anywhere outside a string. Columns past 72 are read too:
// compilers are commonly told to take longer lines.
//
// Implements: REQ-FORTRAN-002
func readFixed(src []byte) *reader {
	r := &reader{}
	lines := splitLines(src)
	for i := 0; i < len(lines); i++ {
		line, ln := lines[i], i+1
		if line == "" || strings.TrimSpace(line) == "" {
			continue
		}
		switch line[0] {
		case '#':
			r.flush()
			i += r.readDirective(lines, i)
			continue
		case 'C', 'c', '*', '!', 'D', 'd':
			if len(line) > 1 && line[1] == '$' && line[0] != 'D' && line[0] != 'd' && ompSentinel(line[2:]) {
				line = "  " + line[2:] // OpenMP conditional compilation
				break
			}
			continue
		}
		code, continued := fixedCode(line)
		if !continued {
			r.flush()
		}
		for j := 0; j < len(code); j++ {
			c := code[j]
			if r.quote != 0 {
				if c == r.quote {
					if j+1 < len(code) && code[j+1] == r.quote {
						r.add(c, ln)
						r.add(c, ln)
						j++
						continue
					}
					r.quote = 0
				}
				r.add(c, ln)
				continue
			}
			if c == '!' {
				break
			}
			if c == ';' {
				r.flush()
				continue
			}
			if c == '\'' || c == '"' {
				r.quote = c
			}
			r.add(c, ln)
		}
	}
	r.flush()
	return r
}

// fixedCode is a fixed-form line's statement text and whether it continues the
// previous line.
func fixedCode(line string) (string, bool) {
	for k := 0; k < len(line) && k < 6; k++ {
		if line[k] == '\t' {
			rest := line[k+1:]
			if rest != "" && rest[0] >= '1' && rest[0] <= '9' && strings.TrimSpace(line[:k]) == "" {
				return rest[1:], true
			}
			return rest, false
		}
	}
	if len(line) < 6 {
		return "", false // a label alone
	}
	cont := line[5] != ' ' && line[5] != '0' && strings.TrimSpace(line[:5]) == ""
	return line[6:], cont
}

// looksFree reports whether a file with a fixed-form extension is written in free
// form after all: some line puts code into columns 1 to 5, where fixed form has
// only comment markers and statement labels, or continues with `&` at its end.
//
// Implements: REQ-FORTRAN-002
func looksFree(src []byte) bool {
	for n, line := range splitLines(src) {
		if n > 2000 {
			break
		}
		if line == "" || strings.TrimSpace(line) == "" {
			continue
		}
		switch line[0] {
		case 'C', 'c', '*', '!', 'D', 'd', '#', '\t':
			continue
		}
		for k := 0; k < len(line) && k < 5; k++ {
			c := line[k]
			if c == '\t' || c == '!' {
				break
			}
			if c != ' ' && (c < '0' || c > '9') {
				return true
			}
		}
		if t := strings.TrimRight(line, " \t"); strings.HasSuffix(t, "&") && len(line) > 6 && line[5] == ' ' {
			if code, _ := fixedCode(line); !strings.ContainsAny(code, "'\"!") {
				return true
			}
		}
	}
	return false
}

// token is a word, a string literal (quotes removed), a number or punctuation.
type token struct {
	text  string // as written
	lower string // words in lower case
	kind  byte   // 'w' word, 's' string, 'n' number, 'p' punctuation
}

// maxTokens bounds the tokens read from one statement; what is looked at lies
// in its first few.
const maxTokens = 192

func isLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$'
}
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// tokenize splits a statement's text into at most limit tokens.
func tokenize(s string, limit int) []token {
	out := make([]token, 0, min(limit, 16))
	for i := 0; i < len(s) && len(out) < limit; {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case isLetter(c):
			j := i + 1
			for j < len(s) && (isLetter(s[j]) || isDigit(s[j])) {
				j++
			}
			out = append(out, token{text: s[i:j], lower: strings.ToLower(s[i:j]), kind: 'w'})
			i = j
		case isDigit(c):
			j := i + 1
			for j < len(s) && (isDigit(s[j]) || s[j] == '.' || s[j] == '_' || isLetter(s[j])) {
				j++
			}
			out = append(out, token{text: s[i:j], kind: 'n'})
			i = j
		case c == '\'' || c == '"':
			var b strings.Builder
			j := i + 1
			for j < len(s) {
				if s[j] == c {
					if j+1 < len(s) && s[j+1] == c {
						b.WriteByte(c)
						j += 2
						continue
					}
					break
				}
				b.WriteByte(s[j])
				j++
			}
			out = append(out, token{text: b.String(), kind: 's'})
			i = j + 1
		default:
			n := 1
			if i+1 < len(s) {
				switch s[i : i+2] {
				case "::", "=>", "==", "/=", "<=", ">=", "**", "//":
					n = 2
				}
			}
			out = append(out, token{text: s[i : i+n], lower: s[i : i+n], kind: 'p'})
			i += n
		}
	}
	return out
}

// Scope kinds of the frames the extractor tracks.
const (
	frModule     = "module"
	frSubmodule  = "submodule"
	frProgram    = "program"
	frFunction   = "function"
	frSubroutine = "subroutine"
	frProcedure  = "procedure" // a separate module procedure: module procedure name
	frType       = "type"
	frInterface  = "interface"
	frEnum       = "enum"
	frBlockData  = "blockdata"
)

type frame struct {
	kind, name string
}

// maxDepth bounds the frame stack: a file that opens scopes it never closes
// stays linear.
const maxDepth = 64

// extractor holds the state of one file's extraction.
type extractor struct {
	ex      *lang.Extraction
	symbols lang.SymbolSet
	stack   []frame
	uses    map[string]bool
	anonBD  int
}

// Implements: REQ-FORTRAN-002, REQ-FORTRAN-003, REQ-FORTRAN-010, REQ-FORTRAN-011
func extractSource(src []byte, fixed bool) *lang.Extraction {
	var r *reader
	if fixed && !looksFree(src) {
		r = readFixed(src)
	} else {
		r = readFree(src)
	}
	x := &extractor{ex: &lang.Extraction{}, uses: map[string]bool{}}
	di := 0
	for _, st := range r.stmts {
		for di < len(r.dirs) && r.dirs[di].line <= st.line {
			x.directive(r.dirs[di])
			di++
		}
		x.statement(st)
	}
	for ; di < len(r.dirs); di++ {
		x.directive(r.dirs[di])
	}
	x.ex.Symbols = x.symbols.List()
	return x.ex
}

// directive records #include "x", #include <x> and fypp's #:include "x".
func (x *extractor) directive(d directive) {
	t := strings.TrimSpace(strings.TrimPrefix(d.text, "#"))
	kind := kindCpp
	if strings.HasPrefix(t, ":") {
		t = strings.TrimSpace(t[1:])
		kind = kindFypp
	}
	if !strings.HasPrefix(t, "include") {
		return
	}
	t = strings.TrimSpace(t[len("include"):])
	if t == "" {
		return
	}
	var spec string
	switch t[0] {
	case '"', '\'':
		end := strings.IndexByte(t[1:], t[0])
		if end < 0 {
			return
		}
		spec = t[1 : 1+end]
	case '<':
		end := strings.IndexByte(t, '>')
		if end < 0 || kind == kindFypp {
			return
		}
		spec, kind = t[1:end], kindCppSys
	default:
		return
	}
	if spec == "" {
		return
	}
	shown := "#include \"" + spec + "\""
	switch kind {
	case kindCppSys:
		shown = "#include <" + spec + ">"
	case kindFypp:
		shown = "#:include \"" + spec + "\""
	}
	x.importOnce(lang.RawImport{Spec: shown, Module: spec, Name: kind, Line: d.line})
}

func (x *extractor) importOnce(im lang.RawImport) {
	key := im.Name + "\x00" + im.Module
	if x.uses[key] {
		return
	}
	x.uses[key] = true
	x.ex.Imports = append(x.ex.Imports, im)
}

func (x *extractor) push(kind, name string) {
	if len(x.stack) < maxDepth {
		x.stack = append(x.stack, frame{kind, name})
	}
}

func (x *extractor) top() string {
	if len(x.stack) == 0 {
		return ""
	}
	return x.stack[len(x.stack)-1].kind
}

// owner is the qualified name of the innermost enclosing program unit or
// procedure: "shop" in module shop, "shop.checkout" in its procedure checkout.
func (x *extractor) owner() string {
	var parts []string
	for _, f := range x.stack {
		switch f.kind {
		case frModule, frSubmodule, frProgram, frFunction, frSubroutine, frProcedure:
			if validName(f.name) {
				parts = append(parts, f.name)
			}
		}
	}
	return strings.Join(parts, ".")
}

func (x *extractor) inInterface() bool { return x.top() == frInterface }

// validName reports whether a Fortran name can be a symbol: not a fypp
// substitution such as ${name}$.
func validName(s string) bool {
	if s == "" || !isLetter(s[0]) || s[0] == '$' {
		return false
	}
	return !strings.Contains(s, "$")
}

func (x *extractor) symbol(name, kind string, line int, qualify bool) {
	if !validName(name) {
		return
	}
	if qualify {
		if o := x.owner(); o != "" {
			name = o + "." + name
		}
	}
	x.symbols.Add(name, kind, line)
}

// eos reports whether ts ends at i.
func eos(ts []token, i int) bool { return i >= len(ts) }

func word(ts []token, i int) string {
	if i < len(ts) && ts[i].kind == 'w' {
		return ts[i].lower
	}
	return ""
}

func punct(ts []token, i int, p string) bool {
	return i < len(ts) && ts[i].kind == 'p' && ts[i].text == p
}

// skipParens returns the index after the parenthesized group starting at i.
func skipParens(ts []token, i int) int {
	depth := 0
	for ; i < len(ts); i++ {
		if ts[i].kind != 'p' {
			continue
		}
		switch ts[i].text {
		case "(":
			depth++
		case ")":
			depth--
			if depth <= 0 {
				return i + 1
			}
		}
	}
	return i
}

// ends are the fused forms of END statements that close a scope.
var ends = map[string]string{
	"endmodule": frModule, "endsubmodule": frSubmodule, "endprogram": frProgram,
	"endfunction": frFunction, "endsubroutine": frSubroutine, "endprocedure": frProcedure,
	"endtype": frType, "endinterface": frInterface, "endenum": frEnum, "endblockdata": frBlockData,
}

// units are the scopes a bare END closes.
var units = map[string]bool{
	frModule: true, frSubmodule: true, frProgram: true, frFunction: true,
	frSubroutine: true, frProcedure: true, frBlockData: true,
}

// prefixes are the words that may precede FUNCTION or SUBROUTINE.
var prefixes = map[string]bool{
	"pure": true, "impure": true, "elemental": true, "recursive": true,
	"non_recursive": true, "module": true, "simple": true,
}

// typeWords begin an intrinsic type specification.
var typeWords = map[string]bool{
	"integer": true, "real": true, "logical": true, "complex": true, "character": true, "byte": true,
	"doubleprecision": true, "doublecomplex": true,
}

// Implements: REQ-FORTRAN-002, REQ-FORTRAN-003
func (x *extractor) statement(st statement) {
	ts := tokenize(st.text, 4)
	if !x.relevant(ts, st.text) {
		return
	}
	ts = tokenize(st.text, maxTokens)
	i := 0
	if i < len(ts) && ts[i].kind == 'n' {
		i++ // a statement label
	}
	if word(ts, i) != "" && punct(ts, i+1, ":") {
		i += 2 // a construct name: outer: do ...
	}
	kw := word(ts, i)
	if kw == "" {
		return
	}
	line := st.line
	switch kw {
	case "module":
		next := word(ts, i+1)
		switch {
		case next == "procedure" && word(ts, i+2) != "" && (eos(ts, i+3) || strings.Contains(ts[i+2].text, "$")):
			if x.inInterface() {
				return // names module procedures of a generic interface
			}
			name := ts[i+2].text
			x.symbol(name, "function", line, true)
			x.push(frProcedure, name)
			return
		case next != "" && eos(ts, i+2) && next != "procedure":
			x.symbol(ts[i+1].text, "module", line, false)
			x.push(frModule, ts[i+1].text)
			return
		}
	case "submodule":
		if !punct(ts, i+1, "(") {
			return
		}
		ancestor := word(ts, i+2)
		j := i + 3
		parent := ""
		if punct(ts, j, ":") {
			parent = word(ts, j+1)
			j += 2
		}
		if ancestor == "" || !punct(ts, j, ")") || word(ts, j+1) == "" || !eos(ts, j+2) {
			return
		}
		mod, spec := ancestor, ancestor
		if parent != "" {
			mod = ancestor + ":" + parent
			spec = ancestor + ":" + parent
		}
		x.importOnce(lang.RawImport{Spec: "submodule (" + spec + ")", Module: mod, Name: kindSubmodule, Line: line})
		name := ts[j+1].text
		x.symbol(name, "submodule", line, false)
		x.push(frSubmodule, name)
		return
	case "program":
		if word(ts, i+1) != "" && eos(ts, i+2) {
			x.symbol(ts[i+1].text, "program", line, false)
			x.push(frProgram, ts[i+1].text)
		}
		return
	case "block", "blockdata":
		j := i + 1
		if kw == "block" {
			if word(ts, j) != "data" {
				return
			}
			j++
		}
		if eos(ts, j) {
			x.push(frBlockData, "")
		} else if word(ts, j) != "" && eos(ts, j+1) {
			x.symbol(ts[j].text, "block data", line, false)
			x.push(frBlockData, ts[j].text)
		}
		return
	case "type":
		if name, ok := typeDefinition(ts, i+1); ok {
			x.symbol(name, "type", line, false)
			x.push(frType, name)
			return
		}
	case "interface", "abstract":
		j := i + 1
		if kw == "abstract" {
			if word(ts, j) != "interface" {
				return
			}
			j++
		}
		if eos(ts, j) {
			x.push(frInterface, "")
			return
		}
		name := interfaceName(ts, j)
		if name == "" {
			return // not an interface statement: interface = 1
		}
		if kw == "interface" {
			x.symbol(name, "interface", line, false)
		}
		x.push(frInterface, name)
		return
	case "enum":
		if punct(ts, i+1, ",") {
			x.push(frEnum, "")
		}
		return
	case "end":
		x.end(ts, i+1)
		return
	case "use":
		x.use(ts, i+1, line)
		return
	case "include":
		if i+1 < len(ts) && ts[i+1].kind == 's' && eos(ts, i+2) && ts[i+1].text != "" {
			spec := ts[i+1].text
			x.importOnce(lang.RawImport{Spec: "include '" + spec + "'", Module: spec, Name: kindInclude, Line: line})
		}
		return
	}
	if k, ok := ends[kw]; ok {
		x.close(k)
		return
	}
	x.procedure(ts, i, line)
}

// relevant reports, from a statement's first tokens, whether it can be one the
// extractor reads: most statements are assignments, calls, control flow and
// declarations, and are not tokenized in full.
func (x *extractor) relevant(ts []token, text string) bool {
	i := 0
	if i < len(ts) && ts[i].kind == 'n' {
		i++
	}
	if word(ts, i) != "" && punct(ts, i+1, ":") {
		i += 2
	}
	kw := word(ts, i)
	switch {
	case kw == "":
		return false
	case keywords[kw] || ends[kw] != "" || kw == "function" || kw == "subroutine":
		return true
	case prefixes[kw] || typeWords[kw] || kw == "double" || kw == "class":
		return containsFold(text, "function") || containsFold(text, "subroutine")
	}
	return false
}

// keywords begin the statements read besides procedure headings.
var keywords = map[string]bool{
	"module": true, "submodule": true, "program": true, "block": true, "blockdata": true, "type": true,
	"interface": true, "abstract": true, "enum": true, "end": true, "use": true, "include": true,
}

// containsFold reports whether s contains the lower-case ASCII word w in any case.
func containsFold(s, w string) bool {
	for i := 0; i+len(w) <= len(s); i++ {
		if s[i]|0x20 != w[0] {
			continue
		}
		j := 1
		for j < len(w) && s[i+j]|0x20 == w[j] {
			j++
		}
		if j == len(w) {
			return true
		}
	}
	return false
}

// typeDefinition reads a derived type definition's name after TYPE: `type name`,
// `type :: name`, `type, extends(a), public :: name` or `type name(k)` - not a
// declaration `type(t) :: x`, a guard `type is (t)` or a DEC `type *, x`.
func typeDefinition(ts []token, j int) (string, bool) {
	switch {
	case punct(ts, j, "::"):
		if word(ts, j+1) != "" {
			return ts[j+1].text, true
		}
	case punct(ts, j, ","):
		depth := 0
		for k := j; k < len(ts); k++ {
			if ts[k].kind != 'p' {
				continue
			}
			switch ts[k].text {
			case "(":
				depth++
			case ")":
				depth--
			case "::":
				if depth == 0 && word(ts, k+1) != "" {
					return ts[k+1].text, true
				}
				return "", false
			}
		}
	case word(ts, j) != "":
		if ts[j].lower == "is" && punct(ts, j+1, "(") {
			return "", false
		}
		if eos(ts, j+1) || punct(ts, j+1, "(") && eos(ts, skipParens(ts, j+1)) {
			return ts[j].text, true
		}
	}
	return "", false
}

// interfaceName is a generic interface's name: a word, operator(.op.),
// assignment(=), or read(formatted) and its kin; "" when the tokens are no
// interface statement.
func interfaceName(ts []token, j int) string {
	w := word(ts, j)
	if w == "" {
		return ""
	}
	if eos(ts, j+1) {
		return ts[j].text
	}
	if !punct(ts, j+1, "(") {
		return ""
	}
	end := skipParens(ts, j+1)
	if !eos(ts, end) {
		return ""
	}
	var b strings.Builder
	b.WriteString(w)
	for k := j + 1; k < end; k++ {
		b.WriteString(ts[k].text)
	}
	return b.String()
}

// end handles END [kind [name]].
func (x *extractor) end(ts []token, j int) {
	if eos(ts, j) {
		for k := len(x.stack) - 1; k >= 0 && k >= len(x.stack)-8; k-- {
			if units[x.stack[k].kind] {
				x.stack = x.stack[:k]
				return
			}
		}
		return
	}
	kind := word(ts, j)
	switch kind {
	case "block":
		if word(ts, j+1) != "data" {
			return // END BLOCK of a BLOCK construct
		}
		kind = frBlockData
	case frModule, frSubmodule, frProgram, frFunction, frSubroutine, frProcedure, frType, frInterface, frEnum, "blockdata":
		if kind == "blockdata" {
			kind = frBlockData
		}
	default:
		return // end do, end if, end file, ...
	}
	x.close(kind)
}

// close pops frames down to the innermost one of kind, looking at most 8 deep.
func (x *extractor) close(kind string) {
	for k := len(x.stack) - 1; k >= 0 && k >= len(x.stack)-8; k-- {
		if x.stack[k].kind == kind {
			x.stack = x.stack[:k]
			return
		}
	}
}

// use reads USE [, INTRINSIC | NON_INTRINSIC] [::] name [, only: ...].
//
// Implements: REQ-FORTRAN-002
func (x *extractor) use(ts []token, j, line int) {
	kind := kindUse
	switch {
	case punct(ts, j, ","):
		switch word(ts, j+1) {
		case "intrinsic":
			kind = kindIntrinsic
		case "non_intrinsic":
			kind = kindNonIntrinsic
		default:
			return
		}
		if !punct(ts, j+2, "::") {
			return
		}
		j += 3
	case punct(ts, j, "::"):
		j++
	}
	name := word(ts, j)
	if name == "" || !(eos(ts, j+1) || punct(ts, j+1, ",")) || !validName(ts[j].text) {
		return
	}
	spec := "use " + ts[j].text
	if kind != kindUse {
		spec = "use, " + kind + " :: " + ts[j].text
	}
	x.importOnce(lang.RawImport{Spec: spec, Module: name, Name: kind, Line: line})
}

// procedure reads a FUNCTION or SUBROUTINE statement after its prefixes and
// type: `pure elemental real(dp) function f(x)`, `module subroutine s`.
//
// Implements: REQ-FORTRAN-003
func (x *extractor) procedure(ts []token, j, line int) {
	for j < len(ts) {
		w := word(ts, j)
		switch {
		case prefixes[w]:
			j++
			continue
		case typeWords[w] || w == "double" && (word(ts, j+1) == "precision" || word(ts, j+1) == "complex"):
			if w == "double" {
				j++
			}
			j++
			if punct(ts, j, "(") {
				j = skipParens(ts, j)
			} else if punct(ts, j, "*") {
				j++
				if punct(ts, j, "(") {
					j = skipParens(ts, j)
				} else {
					j++
				}
			}
			continue
		case (w == "type" || w == "class") && punct(ts, j+1, "("):
			j = skipParens(ts, j+1)
			continue
		}
		break
	}
	w := word(ts, j)
	if w != "function" && w != "subroutine" || word(ts, j+1) == "" {
		return
	}
	templated := strings.Contains(ts[j+1].text, "$") // a fypp name: f_${k}$
	if !templated && !(eos(ts, j+2) || punct(ts, j+2, "(") || word(ts, j+2) == "bind" || word(ts, j+2) == "result") {
		return
	}
	name := ts[j+1].text
	kind := frFunction
	if w == "subroutine" {
		kind = frSubroutine
	}
	if !x.inInterface() {
		x.symbol(name, "function", line, true)
	}
	x.push(kind, name)
}
