package ada

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindWith     = "with"     // a with clause: the unit's spec
	kindBody     = "body"     // a body's own spec
	kindSeparate = "separate" // a subunit's parent body
	kindParent   = "parent"   // a child unit's parent spec

	kindProject     = "project"      // a .gpr's with, extends or Project_Files
	kindDirectory   = "dir"          // a .gpr's Source_Dirs entry
	kindMain        = "main"         // a .gpr's Main entry
	kindDependency  = "dep"          // alire.toml [[depends-on]]
	kindPin         = "pin"          // alire.toml [[pins]] of a crate depends-on does not name
	kindProjectFile = "project-file" // alire.toml project-files
)

// Frame kinds of the block scanner.
const (
	fPackage = 'P' // package spec or body, task or protected spec, protected body
	fBody    = 'S' // subprogram, task or entry body
	fBlock   = 'B' // declare or begin block
	fRecord  = 'r'
	fIf      = 'i'
	fCase    = 'c'
	fLoop    = 'l'
	fSelect  = 'x'
	fDo      = 'd' // accept ... do, extended return ... do
)

const (
	maxFrames = 256
	maxHeader = 2000 // tokens a declaration's header may take before its is or ;
)

type frame struct {
	kind       byte
	name       string // the qualified name declarations inside are owned by
	owner      bool   // declarations inside are symbols
	statements bool   // in the statement part (after begin)
}

// unit is a library unit (or subunit) a file holds.
type unit struct {
	name string // lower case, dotted: a subunit's is Parent.Name
	kind byte   // 's' spec, 'b' body, 'u' subunit
}

// scanned is what the scanner read from an Ada file.
type scanned struct {
	imports []lang.RawImport
	symbols lang.SymbolSet
	seen    map[string]bool // symbols by lower-case name: overloads and completions count once
	units   []unit
}

type parser struct {
	s          *stream
	i          int
	frames     []frame
	depth      int // parenthesis depth in a statement part
	out        *scanned
	headerOnly bool // stop after the first unit's name
	done       bool
	// subunit is the parent unit named by separate (Parent), until the
	// subunit's body is read.
	subunit, subunitLow string
	specs               map[string]bool // imports already recorded
}

// extractSource reads an Ada compilation (one or more compilation units): the
// context clauses' with clauses, what a unit's name implies (a body's spec, a
// child's parent, a subunit's parent body), and the declarations of library
// units and of the packages, tasks and protected units inside them.
//
// Implements: REQ-ADA-002, REQ-ADA-003, REQ-ADA-010, REQ-ADA-011
func extractSource(source []byte) *scanned {
	return scanSource(source, false)
}

// units reads only as far as the first unit's name, unless all is set (a
// .ada file may hold several units).
func units(source []byte, all bool) []unit {
	return scanSource(source, !all).units
}

func scanSource(source []byte, header bool) *scanned {
	out := &scanned{seen: map[string]bool{}}
	p := &parser{s: newStream(source), out: out, headerOnly: header, specs: map[string]bool{}}
	p.run()
	return out
}

func (p *parser) run() {
	for !p.done {
		if _, ok := p.s.at(p.i); !ok {
			return
		}
		start := p.i
		p.s.trim(p.i)
		switch {
		case len(p.frames) == 0:
			p.context()
		case p.frames[len(p.frames)-1].statements:
			p.statement()
		default:
			p.declaration()
		}
		if p.i <= start {
			p.i = start + 1
		}
	}
}

// context reads what comes between compilation units: context clauses, a
// generic formal part, separate (Parent) and the unit's declaration.
func (p *parser) context() {
	switch p.s.word(p.i) {
	case "with":
		p.with(p.i, "")
	case "limited":
		switch {
		case p.s.word(p.i+1) == "with":
			p.with(p.i+1, "limited ")
		case p.s.word(p.i+1) == "private" && p.s.word(p.i+2) == "with":
			p.with(p.i+2, "limited private ")
		default:
			p.i++
		}
	case "private":
		if p.s.word(p.i+1) == "with" {
			p.with(p.i+1, "private ")
		} else {
			p.i++ // a private child unit
		}
	case "use", "pragma":
		p.skipTo(p.i)
	case "generic":
		p.i++
		p.genericFormals()
	case "separate":
		p.separate()
	case "package":
		p.packageDeclaration(p.subunit, true, true)
	case "procedure", "function":
		p.subprogramDeclaration(p.subunit, true, true)
	case "overriding":
		p.i++
	case "task", "protected":
		p.taskDeclaration(p.subunit, true, true)
	default:
		p.i++
	}
}

// with reads the names of a with clause whose "with" is token i.
func (p *parser) with(i int, qualifier string) {
	j := i + 1
	for {
		written, low, next := p.name(j)
		if written == "" || written[0] == '"' {
			break // a with clause names library units, never an operator
		}
		t, _ := p.s.at(j)
		p.addImport(qualifier+"with "+written, low, kindWith, t.line)
		j = next
		if !p.s.punctuation(j, ",") {
			break
		}
		j++
	}
	p.skipTo(j)
}

// separate reads separate (Parent): the subunit's parent body.
func (p *parser) separate() {
	t, _ := p.s.at(p.i)
	j := p.i + 1
	if !p.s.punctuation(j, "(") {
		p.i = j
		return
	}
	written, low, next := p.name(j + 1)
	if written == "" || !p.s.punctuation(next, ")") {
		p.i = j + 1
		return
	}
	p.addImport("separate ("+written+")", low, kindSeparate, t.line)
	p.subunit, p.subunitLow = written, low
	p.i = next + 1
}

// genericFormals skips a generic formal part up to the generic unit's
// declaration: formal types, objects, with procedure/function/package.
func (p *parser) genericFormals() {
	for {
		t, ok := p.s.at(p.i)
		if !ok {
			return
		}
		if t.kind == tIdentifier {
			switch t.low {
			case "package", "procedure", "function":
				return
			}
		}
		p.skipTo(p.i)
	}
}

// name reads a dotted name (or an operator symbol "+") at i: as written, in
// lower case, and the index after it.
func (p *parser) name(i int) (written, low string, next int) {
	t, ok := p.s.at(i)
	if !ok {
		return "", "", i
	}
	if t.kind == tString {
		return `"` + t.text + `"`, `"` + lower(t.text) + `"`, i + 1
	}
	if t.kind != tIdentifier || reserved[t.low] {
		return "", "", i
	}
	written, low = t.text, t.low
	i++
	for n := 0; n < 64 && p.s.punctuation(i, "."); n++ {
		u, ok := p.s.at(i + 1)
		if !ok || u.kind != tIdentifier || reserved[u.low] {
			break
		}
		written += "." + u.text
		low += "." + u.low
		i += 2
	}
	return written, low, i
}

// skipTo moves past the ; that ends the statement going on at i, at
// parenthesis depth 0.
func (p *parser) skipTo(i int) {
	depth := 0
	for {
		t, ok := p.s.at(i)
		if !ok {
			p.i = i
			return
		}
		i++
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case "(":
			depth++
		case ")":
			if depth > 0 {
				depth--
			}
		case ";":
			if depth == 0 {
				p.i = i
				return
			}
		}
	}
}

// header finds the is, renames or ; that ends a declaration's header starting
// at i (after parameters, a result type, discriminants and aspects), at
// parenthesis depth 0. It gives up at a word that starts another declaration.
func (p *parser) header(i int) (at int, word string) {
	depth := 0
	for n := 0; n < maxHeader; n++ {
		t, ok := p.s.at(i)
		if !ok {
			return i, ""
		}
		switch t.kind {
		case tPunctuation:
			switch t.text {
			case "(":
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			case ";":
				if depth == 0 {
					return i, ";"
				}
			}
		case tIdentifier:
			if depth > 0 {
				break
			}
			switch t.low {
			case "is", "renames":
				return i, t.low
			case "procedure", "function":
				// access procedure/function and access protected procedure are types
				if w := p.s.word(i - 1); w != "access" && w != "protected" {
					return i, ""
				}
			case "package", "begin", "end", "type", "subtype", "generic", "private":
				return i, ""
			}
		}
		i++
	}
	return i, ""
}

// qualify joins an owner's name and a declared name.
func qualify(owner, name string) string {
	if owner == "" {
		return name
	}
	return owner + "." + name
}

func (p *parser) symbol(name, kind string, line int) {
	low := lower(name)
	if p.out.seen[low] {
		return
	}
	p.out.seen[low] = true
	p.out.symbols.Add(name, kind, line)
}

func (p *parser) addImport(spec, module, kind string, line int) {
	key := kind + "\x00" + spec
	if p.specs[key] {
		return
	}
	p.specs[key] = true
	p.out.imports = append(p.out.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

func (p *parser) push(f frame) {
	if len(p.frames) < maxFrames {
		p.frames = append(p.frames, f)
	}
}

// libraryUnit records a library unit (or subunit) named low and what its name
// implies: a child's parent spec, a body's own spec.
func (p *parser) libraryUnit(written, low string, kind byte, line int) {
	if p.subunit != "" {
		p.out.units = append(p.out.units, unit{name: p.subunitLow + "." + low, kind: 'u'})
		p.subunit, p.subunitLow = "", ""
		if p.headerOnly {
			p.done = true
		}
		return
	}
	p.out.units = append(p.out.units, unit{name: low, kind: kind})
	if kind == 'b' || kind == 'p' {
		p.addImport("spec "+written, low, kindBody, line)
	}
	if kind == 's' || kind == 'p' {
		if k := strings.LastIndexByte(written, '.'); k > 0 {
			p.addImport("parent "+written[:k], low[:strings.LastIndexByte(low, '.')], kindParent, line)
		}
	}
	if kind == 'p' {
		// a subprogram body without a spec declares the unit itself
		p.out.units[len(p.out.units)-1].kind = 'b'
	}
	if p.headerOnly {
		p.done = true
	}
}

// packageDeclaration reads a package declaration, body, renaming or instantiation at
// p.i ("package").
func (p *parser) packageDeclaration(owner string, library, symbols bool) {
	start, _ := p.s.at(p.i)
	i := p.i + 1
	body := p.s.word(i) == "body"
	if body {
		i++
	}
	written, low, j := p.name(i)
	if written == "" {
		p.i = i
		return
	}
	qualifier := qualify(owner, written)
	at, w := p.header(j)
	switch {
	case w == "is" && p.s.word(at+1) == "separate":
		p.skipTo(at)
	case w == "is" && !body && p.s.word(at+1) == "new":
		if symbols {
			p.symbol(qualifier, "package", start.line)
		}
		if library {
			p.libraryUnit(written, low, 's', start.line)
		}
		p.skipTo(at)
	case w == "is":
		if library {
			kind := byte('s')
			if body {
				kind = 'b'
			}
			p.libraryUnit(written, low, kind, start.line)
		}
		if symbols && !body {
			p.symbol(qualifier, "package", start.line)
		}
		p.push(frame{kind: fPackage, name: qualifier, owner: symbols})
		p.i = at + 1
	case w == "renames":
		if symbols && !body {
			p.symbol(qualifier, "package", start.line)
		}
		if library && !body {
			p.libraryUnit(written, low, 's', start.line)
		}
		p.skipTo(at)
	case w == ";":
		p.i = at + 1
	default:
		p.i = at
	}
}

// subprogramDeclaration reads a procedure or function declaration, body, renaming,
// instantiation, stub or expression function at p.i.
func (p *parser) subprogramDeclaration(owner string, library, symbols bool) {
	start, _ := p.s.at(p.i)
	written, low, j := p.name(p.i + 1)
	if written == "" {
		p.i++
		return
	}
	qualifier := qualify(owner, written)
	kind := start.low
	at, w := p.header(j)
	switch w {
	case ";", "renames":
		if symbols {
			p.symbol(qualifier, kind, start.line)
		}
		if library {
			p.libraryUnit(written, low, 's', start.line)
		}
		p.skipTo(at)
	case "is":
		next := p.s.word(at + 1)
		if next == "new" || next == "separate" || next == "abstract" || next == "null" || p.s.punctuation(at+1, "<>") || p.s.punctuation(at+1, "(") {
			if symbols {
				p.symbol(qualifier, kind, start.line)
			}
			if library {
				p.libraryUnit(written, low, 's', start.line)
			}
			p.skipTo(at)
			return
		}
		if symbols {
			p.symbol(qualifier, kind, start.line)
		}
		if library {
			p.libraryUnit(written, low, 'p', start.line)
			if p.done {
				return
			}
		}
		p.push(frame{kind: fBody})
		p.i = at + 1
	default:
		p.i = at
	}
}

// taskDeclaration reads a task or protected declaration or body at p.i.
func (p *parser) taskDeclaration(owner string, library, symbols bool) {
	start, _ := p.s.at(p.i)
	protected := start.low == "protected"
	i := p.i + 1
	body := false
	switch p.s.word(i) {
	case "body":
		body = true
		i++
	case "type":
		i++
	}
	written, low, j := p.name(i)
	if written == "" {
		p.i = i
		return
	}
	qualifier := qualify(owner, written)
	at, w := p.header(j)
	if !body && symbols {
		kind := "task"
		if protected {
			kind = "protected"
		}
		p.symbol(qualifier, kind, start.line)
	}
	switch {
	case w == "is" && p.s.word(at+1) == "separate":
		p.skipTo(at)
	case w == "is":
		if library && body {
			p.libraryUnit(written, low, 'b', start.line)
			if p.done {
				return
			}
		}
		k := at + 1
		if p.s.word(k) == "new" {
			// task T is new Iface with ... end T: skip to the with
			for n := 0; n < maxHeader; n++ {
				if p.s.word(k) == "with" {
					k++
					break
				}
				if _, ok := p.s.at(k); !ok || p.s.punctuation(k, ";") {
					break
				}
				k++
			}
		}
		switch {
		case body && !protected:
			p.push(frame{kind: fBody})
		default:
			p.push(frame{kind: fPackage, name: qualifier, owner: symbols && protected})
		}
		p.i = k
	case w == ";":
		p.i = at + 1
	default:
		p.i = at
	}
}

// entryDeclaration reads an entry declaration or (in a protected body) entry body.
func (p *parser) entryDeclaration() {
	_, low, j := p.name(p.i + 1)
	if low == "" {
		p.i++
		return
	}
	at, w := p.header(j)
	switch w {
	case "is":
		p.push(frame{kind: fBody})
		p.i = at + 1
	case ";":
		p.i = at + 1
	default:
		p.i = at
	}
}

// typeDeclaration reads a type declaration at p.i: a record opens a frame that ends at
// end record.
func (p *parser) typeDeclaration(owner string, symbols bool) {
	start, _ := p.s.at(p.i)
	t, ok := p.s.at(p.i + 1)
	if !ok || t.kind != tIdentifier || reserved[t.low] {
		p.i++
		return
	}
	qualifier := qualify(owner, t.text)
	kind := "type"
	tagged, derived, record := false, false, false
	i := p.i + 2
	depth := 0
	afterIs := false
loop:
	for n := 0; n < maxHeader; n++ {
		u, ok := p.s.at(i)
		if !ok {
			break
		}
		if u.kind == tPunctuation {
			switch u.text {
			case "(":
				if depth == 0 && afterIs && p.s.word(i-1) == "is" {
					kind = "enum"
				}
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			case ";":
				if depth == 0 {
					i++
					break loop
				}
			}
			i++
			continue
		}
		if u.kind != tIdentifier || depth > 0 {
			i++
			continue
		}
		switch u.low {
		case "is":
			afterIs = true
		case "tagged":
			tagged = true
		case "new":
			derived = true
		case "interface":
			if afterIs {
				kind = "interface"
			}
		case "with":
			if derived {
				if w := p.s.word(i + 1); w == "record" || w == "null" || w == "private" {
					tagged = true
				}
			}
		case "record":
			if p.s.word(i-1) != "null" {
				record = true
				i++
				break loop
			}
		case "procedure", "function":
			if w := p.s.word(i - 1); w != "access" && w != "protected" {
				break loop // access procedure is a type; else a new declaration
			}
		case "private":
			switch p.s.word(i - 1) {
			case "is", "with", "limited", "tagged", "abstract", "synchronized":
			default:
				break loop
			}
		case "package", "begin", "end", "subtype", "generic", "type":
			break loop
		}
		i++
	}
	switch {
	case kind == "interface":
	case tagged:
		kind = "class"
	case record:
		kind = "struct"
	}
	if symbols {
		p.symbol(qualifier, kind, start.line)
	}
	if record {
		p.push(frame{kind: fRecord})
	}
	p.i = i
}

// declaration reads one declaration of a declarative part.
func (p *parser) declaration() {
	top := p.frames[len(p.frames)-1]
	symbols := top.owner
	owner := top.name
	switch p.s.word(p.i) {
	case "procedure", "function":
		p.subprogramDeclaration(owner, false, symbols)
	case "overriding":
		p.i++
	case "not":
		if p.s.word(p.i+1) == "overriding" {
			p.i += 2
		} else {
			p.skipTo(p.i)
		}
	case "package":
		p.packageDeclaration(owner, false, symbols)
	case "generic":
		p.i++
		p.genericFormals()
	case "type":
		p.typeDeclaration(owner, symbols)
	case "subtype":
		if t, ok := p.s.at(p.i + 1); ok && t.kind == tIdentifier && symbols {
			p.symbol(qualify(owner, t.text), "type", t.line)
		}
		p.skipTo(p.i)
	case "task", "protected":
		p.taskDeclaration(owner, false, symbols)
	case "entry":
		p.entryDeclaration()
	case "begin":
		p.frames[len(p.frames)-1].statements = true
		p.depth = 0
		p.i++
	case "private":
		p.i++
	case "end":
		p.end()
	case "use", "pragma", "with":
		p.skipTo(p.i)
	case "case":
		// a record's variant part: its components are read as declarations
		// until end case closes the frame
		p.push(frame{kind: fCase})
		p.skipAlternative()
	case "when":
		p.skipAlternative()
	default:
		p.skipDeclaration()
	}
}

// skipAlternative skips case D is / when A | B => in a variant part.
func (p *parser) skipAlternative() {
	for n := 0; n < maxHeader; n++ {
		t, ok := p.s.at(p.i)
		if !ok {
			return
		}
		p.i++
		if t.kind == tIdentifier && t.low == "is" || t.kind == tPunctuation && (t.text == "=>" || t.text == ";") {
			return
		}
	}
}

// skipDeclaration skips an object, exception or representation declaration;
// for ... use record opens a record frame.
func (p *parser) skipDeclaration() {
	depth := 0
	i := p.i
	for n := 0; ; n++ {
		t, ok := p.s.at(i)
		if !ok {
			p.i = i
			return
		}
		if t.kind == tPunctuation {
			switch t.text {
			case "(":
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			case ";":
				if depth == 0 {
					p.i = i + 1
					return
				}
			}
		} else if t.kind == tIdentifier && depth == 0 && n > 0 {
			switch t.low {
			case "record":
				if p.s.word(i-1) != "null" {
					p.push(frame{kind: fRecord})
					p.i = i + 1
					return
				}
			case "end", "begin", "package", "type", "subtype", "generic", "private", "task", "entry":
				p.i = i
				return
			case "procedure", "function":
				if w := p.s.word(i - 1); w != "access" && w != "protected" {
					p.i = i
					return
				}
			}
		}
		i++
	}
}

// statement reads one token of a statement part: only what opens or closes a
// block matters.
func (p *parser) statement() {
	t, _ := p.s.at(p.i)
	if t.kind == tPunctuation {
		switch t.text {
		case "(":
			p.depth++
		case ")":
			if p.depth > 0 {
				p.depth--
			}
		case ";":
			p.depth = 0
		}
		p.i++
		return
	}
	if t.kind != tIdentifier || p.depth > 0 {
		p.i++
		return
	}
	switch t.low {
	case "if":
		p.push(frame{kind: fIf, statements: true})
	case "case":
		p.push(frame{kind: fCase, statements: true})
	case "loop":
		p.push(frame{kind: fLoop, statements: true})
	case "select":
		p.push(frame{kind: fSelect, statements: true})
	case "declare":
		p.push(frame{kind: fBlock})
	case "begin":
		p.push(frame{kind: fBlock, statements: true})
	case "do":
		p.push(frame{kind: fDo, statements: true})
	case "end":
		p.end()
		return
	}
	p.i++
}

// end closes a frame: end if/loop/case/select/record/return the innermost of
// that kind, a plain end the innermost unit, body, block or accept.
func (p *parser) end() {
	var want string
	switch p.s.word(p.i + 1) {
	case "if":
		want = string(fIf)
	case "loop":
		want = string(fLoop)
	case "case":
		want = string(fCase)
	case "select":
		want = string(fSelect)
	case "record":
		want = string(fRecord)
	case "return":
		want = string(fDo)
	default:
		want = string([]byte{fPackage, fBody, fBlock, fDo})
	}
	for k, n := len(p.frames)-1, 0; k >= 0 && n < 8; k, n = k-1, n+1 {
		if strings.IndexByte(want, p.frames[k].kind) >= 0 {
			p.frames = p.frames[:k]
			break
		}
	}
	p.depth = 0
	p.skipTo(p.i)
}
