package fortran

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/chars"
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
	statements  []statement
	directories []directive
	buffer      strings.Builder
	start       int  // line of the statement being built, 0 for none
	quote       byte // the open string's quote, 0 outside strings
}

func (r *reader) add(c byte, line int) {
	if r.start == 0 {
		if c == ' ' || c == '\t' {
			return
		}
		r.start = line
	}
	if r.buffer.Len() < maxStatement {
		r.buffer.WriteByte(c)
	}
}

func (r *reader) flush() {
	if r.start != 0 {
		if t := strings.TrimSpace(r.buffer.String()); t != "" {
			r.statements = append(r.statements, statement{text: t, line: r.start})
		}
	}
	r.buffer.Reset()
	r.start = 0
	r.quote = 0
}

// directiveLine reports whether a line is a preprocessor line: `#` first on the
// line (cpp, and fypp's #: directives), or fypp's `$:` and `@:` lines.
func directiveLine(line string) bool {
	t := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(t, "#") || strings.HasPrefix(t, "$:") || strings.HasPrefix(t, "@:")
}

// splitLines splits source into lines without their line terminators.
func splitLines(source []byte) []string {
	s := string(source)
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
	r.directories = append(r.directories, directive{text: text, line: i + 1})
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
func readFree(source []byte) *reader {
	r := &reader{}
	lines := splitLines(source)
	continued := false
	for i := 0; i < len(lines); i++ {
		line, lineNumber := lines[i], i+1
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
		if continued {
			if r.quote == 0 && (t == "" || t[0] == '!') {
				continue // a comment or blank line between continuation lines
			}
			j = len(line) - len(t)
			if j < len(line) && line[j] == '&' {
				j++
			}
		}
		continued = false
	scan:
		for ; j < len(line); j++ {
			c := line[j]
			if r.quote != 0 {
				if c == r.quote {
					if j+1 < len(line) && line[j+1] == r.quote {
						r.add(c, lineNumber)
						r.add(c, lineNumber)
						j++
						continue
					}
					r.quote = 0
				} else if c == '&' && nextCode(line, j+1) == len(line) {
					continued = true
					break scan
				}
				r.add(c, lineNumber)
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
					continued = true
					break scan
				}
			}
			r.add(c, lineNumber)
		}
		if !continued {
			r.flush()
		} else {
			r.add(' ', lineNumber)
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
func readFixed(source []byte) *reader {
	r := &reader{}
	lines := splitLines(source)
	for i := 0; i < len(lines); i++ {
		line, lineNumber := lines[i], i+1
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
						r.add(c, lineNumber)
						r.add(c, lineNumber)
						j++
						continue
					}
					r.quote = 0
				}
				r.add(c, lineNumber)
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
			r.add(c, lineNumber)
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
	continued := line[5] != ' ' && line[5] != '0' && strings.TrimSpace(line[:5]) == ""
	return line[6:], continued
}

// looksFree reports whether a file with a fixed-form extension is written in free
// form after all: some line puts code into columns 1 to 5, where fixed form has
// only comment markers and statement labels, or continues with `&` at its end.
//
// Implements: REQ-FORTRAN-002
func looksFree(source []byte) bool {
	for n, line := range splitLines(source) {
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
			for j < len(s) && (isLetter(s[j]) || chars.IsDigit(s[j])) {
				j++
			}
			out = append(out, token{text: s[i:j], lower: strings.ToLower(s[i:j]), kind: 'w'})
			i = j
		case chars.IsDigit(c):
			j := i + 1
			for j < len(s) && (chars.IsDigit(s[j]) || s[j] == '.' || s[j] == '_' || isLetter(s[j])) {
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
	extraction *lang.Extraction
	symbols    lang.SymbolSet
	stack      []frame
	uses       map[string]bool
}

// Implements: REQ-FORTRAN-002, REQ-FORTRAN-003, REQ-FORTRAN-010, REQ-FORTRAN-011
func extractSource(source []byte, fixed bool) *lang.Extraction {
	var r *reader
	if fixed && !looksFree(source) {
		r = readFixed(source)
	} else {
		r = readFree(source)
	}
	x := &extractor{extraction: &lang.Extraction{}, uses: map[string]bool{}}
	directiveIndex := 0
	for _, statement := range r.statements {
		for directiveIndex < len(r.directories) && r.directories[directiveIndex].line <= statement.line {
			x.directive(r.directories[directiveIndex])
			directiveIndex++
		}
		x.statement(statement)
	}
	for ; directiveIndex < len(r.directories); directiveIndex++ {
		x.directive(r.directories[directiveIndex])
	}
	x.extraction.Symbols = x.symbols.List()
	return x.extraction
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

func (x *extractor) importOnce(rawImport lang.RawImport) {
	key := rawImport.Name + "\x00" + rawImport.Module
	if x.uses[key] {
		return
	}
	x.uses[key] = true
	x.extraction.Imports = append(x.extraction.Imports, rawImport)
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

// pastEnd reports whether tokens ends at i.
func pastEnd(tokens []token, i int) bool { return i >= len(tokens) }

func word(tokens []token, i int) string {
	if i < len(tokens) && tokens[i].kind == 'w' {
		return tokens[i].lower
	}
	return ""
}

func punctuation(tokens []token, i int, p string) bool {
	return i < len(tokens) && tokens[i].kind == 'p' && tokens[i].text == p
}

// skipParentheses returns the index after the parenthesized group starting at i.
func skipParentheses(tokens []token, i int) int {
	depth := 0
	for ; i < len(tokens); i++ {
		if tokens[i].kind != 'p' {
			continue
		}
		switch tokens[i].text {
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
func (x *extractor) statement(current statement) {
	tokens := tokenize(current.text, 4)
	if !x.relevant(tokens, current.text) {
		return
	}
	tokens = tokenize(current.text, maxTokens)
	i := 0
	if i < len(tokens) && tokens[i].kind == 'n' {
		i++ // a statement label
	}
	if word(tokens, i) != "" && punctuation(tokens, i+1, ":") {
		i += 2 // a construct name: outer: do ...
	}
	keyword := word(tokens, i)
	if keyword == "" {
		return
	}
	line := current.line
	switch keyword {
	case "module":
		next := word(tokens, i+1)
		switch {
		case next == "procedure" && word(tokens, i+2) != "" && (pastEnd(tokens, i+3) || strings.Contains(tokens[i+2].text, "$")):
			if x.inInterface() {
				return // names module procedures of a generic interface
			}
			name := tokens[i+2].text
			x.symbol(name, "function", line, true)
			x.push(frProcedure, name)
			return
		case next != "" && pastEnd(tokens, i+2) && next != "procedure":
			x.symbol(tokens[i+1].text, "module", line, false)
			x.push(frModule, tokens[i+1].text)
			return
		}
	case "submodule":
		if !punctuation(tokens, i+1, "(") {
			return
		}
		ancestor := word(tokens, i+2)
		j := i + 3
		parent := ""
		if punctuation(tokens, j, ":") {
			parent = word(tokens, j+1)
			j += 2
		}
		if ancestor == "" || !punctuation(tokens, j, ")") || word(tokens, j+1) == "" || !pastEnd(tokens, j+2) {
			return
		}
		module, spec := ancestor, ancestor
		if parent != "" {
			module = ancestor + ":" + parent
			spec = ancestor + ":" + parent
		}
		x.importOnce(lang.RawImport{Spec: "submodule (" + spec + ")", Module: module, Name: kindSubmodule, Line: line})
		name := tokens[j+1].text
		x.symbol(name, "submodule", line, false)
		x.push(frSubmodule, name)
		return
	case "program":
		if word(tokens, i+1) != "" && pastEnd(tokens, i+2) {
			x.symbol(tokens[i+1].text, "program", line, false)
			x.push(frProgram, tokens[i+1].text)
		}
		return
	case "block", "blockdata":
		j := i + 1
		if keyword == "block" {
			if word(tokens, j) != "data" {
				return
			}
			j++
		}
		if pastEnd(tokens, j) {
			x.push(frBlockData, "")
		} else if word(tokens, j) != "" && pastEnd(tokens, j+1) {
			x.symbol(tokens[j].text, "block data", line, false)
			x.push(frBlockData, tokens[j].text)
		}
		return
	case "type":
		if name, ok := typeDefinition(tokens, i+1); ok {
			x.symbol(name, "type", line, false)
			x.push(frType, name)
			return
		}
	case "interface", "abstract":
		j := i + 1
		if keyword == "abstract" {
			if word(tokens, j) != "interface" {
				return
			}
			j++
		}
		if pastEnd(tokens, j) {
			x.push(frInterface, "")
			return
		}
		name := interfaceName(tokens, j)
		if name == "" {
			return // not an interface statement: interface = 1
		}
		if keyword == "interface" {
			x.symbol(name, "interface", line, false)
		}
		x.push(frInterface, name)
		return
	case "enum":
		if punctuation(tokens, i+1, ",") {
			x.push(frEnum, "")
		}
		return
	case "end":
		x.end(tokens, i+1)
		return
	case "use":
		x.use(tokens, i+1, line)
		return
	case "include":
		if i+1 < len(tokens) && tokens[i+1].kind == 's' && pastEnd(tokens, i+2) && tokens[i+1].text != "" {
			spec := tokens[i+1].text
			x.importOnce(lang.RawImport{Spec: "include '" + spec + "'", Module: spec, Name: kindInclude, Line: line})
		}
		return
	}
	if k, ok := ends[keyword]; ok {
		x.close(k)
		return
	}
	x.procedure(tokens, i, line)
}

// relevant reports, from a statement's first tokens, whether it can be one the
// extractor reads: most statements are assignments, calls, control flow and
// declarations, and are not tokenized in full.
func (x *extractor) relevant(tokens []token, text string) bool {
	i := 0
	if i < len(tokens) && tokens[i].kind == 'n' {
		i++
	}
	if word(tokens, i) != "" && punctuation(tokens, i+1, ":") {
		i += 2
	}
	keyword := word(tokens, i)
	switch {
	case keyword == "":
		return false
	case keywords[keyword] || ends[keyword] != "" || keyword == "function" || keyword == "subroutine":
		return true
	case prefixes[keyword] || typeWords[keyword] || keyword == "double" || keyword == "class":
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
func typeDefinition(tokens []token, j int) (string, bool) {
	switch {
	case punctuation(tokens, j, "::"):
		if word(tokens, j+1) != "" {
			return tokens[j+1].text, true
		}
	case punctuation(tokens, j, ","):
		depth := 0
		for k := j; k < len(tokens); k++ {
			if tokens[k].kind != 'p' {
				continue
			}
			switch tokens[k].text {
			case "(":
				depth++
			case ")":
				depth--
			case "::":
				if depth == 0 && word(tokens, k+1) != "" {
					return tokens[k+1].text, true
				}
				return "", false
			}
		}
	case word(tokens, j) != "":
		if tokens[j].lower == "is" && punctuation(tokens, j+1, "(") {
			return "", false
		}
		if pastEnd(tokens, j+1) || punctuation(tokens, j+1, "(") && pastEnd(tokens, skipParentheses(tokens, j+1)) {
			return tokens[j].text, true
		}
	}
	return "", false
}

// interfaceName is a generic interface's name: a word, operator(.op.),
// assignment(=), or read(formatted) and its kin; "" when the tokens are no
// interface statement.
func interfaceName(tokens []token, j int) string {
	w := word(tokens, j)
	if w == "" {
		return ""
	}
	if pastEnd(tokens, j+1) {
		return tokens[j].text
	}
	if !punctuation(tokens, j+1, "(") {
		return ""
	}
	end := skipParentheses(tokens, j+1)
	if !pastEnd(tokens, end) {
		return ""
	}
	var b strings.Builder
	b.WriteString(w)
	for k := j + 1; k < end; k++ {
		b.WriteString(tokens[k].text)
	}
	return b.String()
}

// end handles END [kind [name]].
func (x *extractor) end(tokens []token, j int) {
	if pastEnd(tokens, j) {
		for k := len(x.stack) - 1; k >= 0 && k >= len(x.stack)-8; k-- {
			if units[x.stack[k].kind] {
				x.stack = x.stack[:k]
				return
			}
		}
		return
	}
	kind := word(tokens, j)
	switch kind {
	case "block":
		if word(tokens, j+1) != "data" {
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
func (x *extractor) use(tokens []token, j, line int) {
	kind := kindUse
	switch {
	case punctuation(tokens, j, ","):
		switch word(tokens, j+1) {
		case "intrinsic":
			kind = kindIntrinsic
		case "non_intrinsic":
			kind = kindNonIntrinsic
		default:
			return
		}
		if !punctuation(tokens, j+2, "::") {
			return
		}
		j += 3
	case punctuation(tokens, j, "::"):
		j++
	}
	name := word(tokens, j)
	if name == "" || !(pastEnd(tokens, j+1) || punctuation(tokens, j+1, ",")) || !validName(tokens[j].text) {
		return
	}
	spec := "use " + tokens[j].text
	if kind != kindUse {
		spec = "use, " + kind + " :: " + tokens[j].text
	}
	x.importOnce(lang.RawImport{Spec: spec, Module: name, Name: kind, Line: line})
}

// procedure reads a FUNCTION or SUBROUTINE statement after its prefixes and
// type: `pure elemental real(dp) function f(x)`, `module subroutine s`.
//
// Implements: REQ-FORTRAN-003
func (x *extractor) procedure(tokens []token, j, line int) {
	for j < len(tokens) {
		w := word(tokens, j)
		switch {
		case prefixes[w]:
			j++
			continue
		case typeWords[w] || w == "double" && (word(tokens, j+1) == "precision" || word(tokens, j+1) == "complex"):
			if w == "double" {
				j++
			}
			j++
			if punctuation(tokens, j, "(") {
				j = skipParentheses(tokens, j)
			} else if punctuation(tokens, j, "*") {
				j++
				if punctuation(tokens, j, "(") {
					j = skipParentheses(tokens, j)
				} else {
					j++
				}
			}
			continue
		case (w == "type" || w == "class") && punctuation(tokens, j+1, "("):
			j = skipParentheses(tokens, j+1)
			continue
		}
		break
	}
	w := word(tokens, j)
	if w != "function" && w != "subroutine" || word(tokens, j+1) == "" {
		return
	}
	templated := strings.Contains(tokens[j+1].text, "$") // a fypp name: f_${k}$
	if !templated && !(pastEnd(tokens, j+2) || punctuation(tokens, j+2, "(") || word(tokens, j+2) == "bind" || word(tokens, j+2) == "result") {
		return
	}
	name := tokens[j+1].text
	kind := frFunction
	if w == "subroutine" {
		kind = frSubroutine
	}
	if !x.inInterface() {
		x.symbol(name, "function", line, true)
	}
	x.push(kind, name)
}
