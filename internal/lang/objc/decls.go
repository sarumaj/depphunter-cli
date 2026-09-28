package objc

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// def is one definition found; decl marks a declaration (a prototype, a method an
// @interface declares) that a definition of the same name in the file replaces.
type def struct {
	name, kind string
	line       int
	decl       bool
}

// Containers: what the tokens being read belong to.
const (
	inNone = iota
	inInterface
	inImplementation
	inProtocol
)

type parser struct {
	tokens  []token
	i       int
	in      int
	owner   string // the class or protocol of the container
	frames  []bool // open braces the parser stands in: true = transparent (namespace, extern "C")
	defs    []def
	modules []lang.RawImport // @import
}

func (p *parser) tok(k int) token {
	if p.i+k < len(p.tokens) && p.i+k >= 0 {
		return p.tokens[p.i+k]
	}
	return token{kind: -1}
}

func (p *parser) is(k int, text string) bool {
	t := p.tok(k)
	return t.kind >= 0 && t.text == text && t.kind != tString
}

func (p *parser) add(name, kind string, line int, decl bool) {
	if name != "" {
		p.defs = append(p.defs, def{name, kind, line, decl})
	}
}

// parse reads the declarations of a token stream: classes, categories, protocols,
// methods and properties, and the C declarations around them (functions, typedefs,
// enums, structs, variables and constants).
//
// Implements: REQ-OBJC-003
func parse(tokens []token) *parser {
	p := &parser{tokens: tokens}
	for p.i < len(p.tokens) {
		t := p.tok(0)
		switch {
		case t.kind == tIdent && t.text == "@import":
			p.importDecl()
		case t.kind == tIdent && (t.text == "@interface" || t.text == "@implementation" || t.text == "@protocol"):
			p.header()
		case t.kind == tIdent && t.text == "@end":
			p.in, p.owner = inNone, ""
			p.i++
		case t.kind == tIdent && t.text == "@property":
			p.property()
		case t.kind == tIdent && (t.text == "@class" || t.text == "@synthesize" || t.text == "@dynamic" || t.text == "@compatibility_alias"):
			p.skipTo(";")
		case t.kind == tIdent && strings.HasPrefix(t.text, "@"):
			p.i++ // @optional, @required, @public, @private, @package...
		case t.kind == tPunct && (t.text == "-" || t.text == "+") && p.in != inNone:
			p.method()
		case t.kind == tPunct && t.text == "}":
			if n := len(p.frames); n > 0 {
				p.frames = p.frames[:n-1]
			}
			p.i++
		case t.kind == tPunct && t.text == ";":
			p.i++
		default:
			p.statement()
		}
	}
	return p
}

// stopper reports whether a token ends whatever is being read: a container keyword
// means the braces before it did not balance (an #if branch opened one), so the
// parser picks up from there.
func stopper(t token) bool {
	return t.kind == tIdent && (t.text == "@end" || t.text == "@interface" || t.text == "@implementation")
}

// skipTo steps past the next text at bracket depth 0, stopping at a container
// keyword.
func (p *parser) skipTo(text string) {
	depth := 0
	for p.i < len(p.tokens) {
		t := p.tok(0)
		if stopper(t) && t.text != text {
			return
		}
		p.i++
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
		if depth <= 0 && t.text == text {
			return
		}
	}
}

// skipGroup steps over a bracketed group starting at the current token.
func (p *parser) skipGroup() {
	open := p.tok(0).text
	closing := map[string]string{"(": ")", "[": "]", "{": "}", "<": ">"}[open]
	depth := 0
	for p.i < len(p.tokens) {
		t := p.tok(0)
		if stopper(t) {
			return
		}
		p.i++
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case open:
			depth++
		case closing:
			if depth--; depth == 0 {
				return
			}
		case ";":
			if open == "<" {
				return // a comparison, not a generic: give up
			}
		}
	}
}

// importDecl reads `@import Module.Sub;`.
func (p *parser) importDecl() {
	line := p.tok(0).line
	p.i++
	var b strings.Builder
	for p.i < len(p.tokens) && !p.is(0, ";") && !stopper(p.tok(0)) {
		t := p.tok(0)
		if t.kind != tIdent && t.text != "." {
			break
		}
		b.WriteString(t.text)
		p.i++
	}
	if m := b.String(); m != "" {
		p.modules = append(p.modules, lang.RawImport{Spec: "@import " + m + ";", Module: m, Name: kindModule, Line: line})
	}
	p.skipTo(";")
}

// header reads the start of an @interface, @implementation or @protocol and enters
// it: a class, a category Class(Name) - a class extension Class() adds no symbol -
// or a protocol; a forward @protocol declaration is skipped.
func (p *parser) header() {
	kw := p.tok(0)
	p.i++
	name := p.tok(0)
	if name.kind != tIdent || strings.HasPrefix(name.text, "@") {
		return
	}
	p.i++
	if kw.text == "@protocol" && (p.is(0, ";") || p.is(0, ",")) {
		p.skipTo(";")
		return
	}
	if kw.text == "@protocol" && p.is(0, "(") {
		return // @protocol(Name) is an expression
	}
	if p.is(0, "<") && kw.text == "@interface" && !p.is(-1, ":") {
		p.skipGroup() // generic parameters: @interface Box<ObjectType>
	}
	category, isCategory := "", false
	if p.is(0, "(") {
		isCategory = true
		p.i++
		if t := p.tok(0); t.kind == tIdent {
			category = t.text
			p.i++
		}
		if p.is(0, ")") {
			p.i++
		}
	}
	switch {
	case kw.text == "@protocol":
		p.in = inProtocol
		p.add(name.text, "interface", name.line, false)
	case isCategory && category == "":
		p.in = inInterface // a class extension: more of the class
	case isCategory:
		p.in = inInterface
		p.add(name.text+"("+category+")", "extension", name.line, kw.text == "@interface")
	default:
		p.in = inInterface
		p.add(name.text, "class", name.line, kw.text == "@interface")
	}
	if kw.text == "@implementation" {
		p.in = inImplementation
	}
	p.owner = name.text
	// The rest of the header: a superclass, adopted protocols, instance variables.
	for p.i < len(p.tokens) {
		switch {
		case p.is(0, ":"):
			p.i++
			if p.tok(0).kind == tIdent {
				p.i++
			}
		case p.is(0, "<"):
			p.skipGroup()
		case p.is(0, "{"):
			p.skipGroup()
			return
		default:
			return
		}
	}
}

// method reads a method's declaration or definition: `- (T)name`, `- (T)a:(T)x
// b:(T)y`, `+ ...`; its symbol is Owner.selector. A body is skipped.
func (p *parser) method() {
	line := p.tok(0).line
	p.i++
	if p.is(0, "(") {
		p.skipGroup()
	}
	var sel strings.Builder
	parts := 0
loop:
	for p.i < len(p.tokens) {
		t := p.tok(0)
		switch {
		case t.kind == tIdent && !strings.HasPrefix(t.text, "@") && p.is(1, ":"):
			sel.WriteString(t.text)
			sel.WriteRune(':')
			parts++
			p.i += 2
		case p.is(0, ":") && parts > 0:
			sel.WriteString(":")
			p.i++
		case t.kind == tIdent && parts == 0 && sel.Len() == 0:
			sel.WriteString(t.text)
			p.i++
			break loop
		default:
			break loop
		}
		// The parameter: (T)name.
		if p.is(0, "(") {
			p.skipGroup()
		}
		if t := p.tok(0); t.kind == tIdent && !strings.HasPrefix(t.text, "@") {
			p.i++ // after a keyword and its type comes the parameter's name, always
		}
		if p.is(0, ",") { // varargs: , ...
			for p.is(0, ",") || p.is(0, ".") {
				p.i++
			}
			break
		}
	}
	// Attributes up to the end of the declaration or the start of the body.
	for p.i < len(p.tokens) && !p.is(0, ";") && !p.is(0, "{") && !stopper(p.tok(0)) &&
		!(p.in != inNone && (p.is(0, "-") || p.is(0, "+")) && p.tok(0).line != line) {
		if p.is(0, "(") {
			p.skipGroup()
			continue
		}
		p.i++
	}
	body := p.is(0, "{")
	if body {
		p.skipGroup()
	} else if p.is(0, ";") {
		p.i++
	}
	if sel.Len() > 0 && p.owner != "" {
		p.add(p.owner+"."+sel.String(), "method", line, !body)
	}
}

// property reads `@property (attrs) T name;`, a block `T (^name)(args)` included.
func (p *parser) property() {
	line := p.tok(0).line
	p.i++
	if p.is(0, "(") {
		p.skipGroup()
	}
	start := p.i
	p.skipTo(";")
	if name := declaredName(p.tokens[start:p.i]); name != "" && p.owner != "" {
		p.add(p.owner+"."+name, "property", line, false)
	}
}

var (
	upperName = regexp.MustCompile(`^[A-Z][A-Z0-9_]+$`)
	// enumMacros declare an enumeration: NS_ENUM(Type, Name).
	enumMacros = map[string]bool{
		"NS_ENUM": true, "NS_OPTIONS": true, "NS_CLOSED_ENUM": true, "NS_ERROR_ENUM": true,
		"CF_ENUM": true, "CF_OPTIONS": true, "CF_CLOSED_ENUM": true,
	}
)

// attrPrefixes start the attribute and availability macros of Apple's SDKs
// (NS_DESIGNATED_INITIALIZER, API_AVAILABLE, UIKIT_EXTERN, OBJC_EXPORT...).
var attrPrefixes = []string{"NS_", "CF_", "API_", "UIKIT_", "APPKIT_", "OBJC_", "FOUNDATION_",
	"XCT_", "SWIFT_", "OS_", "AVAILABLE_", "DEPRECATED_", "__IOS_", "__OSX_", "__TVOS_",
	"__WATCHOS_", "__API_", "IB_", "CA_", "WK_", "MP_", "AV_", "CG_", "CT_", "UNAVAILABLE_"}

// macro reports whether an identifier is an attribute or storage macro rather than
// a name: one of Apple's by its prefix, a "__x__" keyword, or one saying
// deprecated or available. A project's own all-capitals names (typedef int
// ASN1_NULL) are names.
func macro(s string) bool {
	if strings.HasPrefix(s, "__") && strings.HasSuffix(s, "__") {
		return true
	}
	if !upperName.MatchString(s) {
		return false
	}
	for _, p := range attrPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return strings.Contains(s, "DEPRECATED") || strings.Contains(s, "AVAILABLE")
}

// declaredName is the name a declaration's tokens (without the final ";") declare:
// the name inside `(^name)` or `(*name)`, else the last identifier before an "=" or
// an array bound, trailing attribute macros ignored.
func declaredName(tokens []token) string {
	for j := 0; j+2 < len(tokens); j++ {
		if tokens[j].text == "(" && (tokens[j+1].text == "^" || tokens[j+1].text == "*") && tokens[j+2].kind == tIdent {
			return tokens[j+2].text
		}
	}
	if j := indexTop(tokens, "="); j >= 0 {
		tokens = tokens[:j]
	}
	name := ""
	depth := 0
	for j := 0; j < len(tokens); j++ {
		t := tokens[j]
		switch {
		case t.text == "(" || t.text == "[":
			depth++
		case t.text == ")" || t.text == "]":
			depth--
		case depth == 0 && t.kind == tIdent && !macro(t.text) && !qualifier[t.text] &&
			!(j+1 < len(tokens) && tokens[j+1].text == "("):
			name = t.text // an identifier before "(" is a macro call: NS_SWIFT_NAME(x)
		}
	}
	return name
}

// qualifier are words that may follow a declaration's name.
var qualifier = map[string]bool{
	"const": true, "_Nullable": true, "_Nonnull": true, "_Null_unspecified": true,
	"nullable": true, "nonnull": true, "__kindof": true, "IBOutlet": true, "IBInspectable": true,
	"volatile": true, "restrict": true, "__weak": true, "__strong": true, "__block": true,
	"__unsafe_unretained": true, "__autoreleasing": true, "__nullable": true, "__nonnull": true,
	"__covariant": true, "__contravariant": true, "__unused": true, "__kindof__": true,
}

// indexTop finds text at bracket depth 0.
func indexTop(tokens []token, text string) int {
	depth := 0
	for j, t := range tokens {
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case text:
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// statement reads a C declaration or definition at the top level: up to a ";", or a
// "{" opening a function body, a type's body, an initializer, a namespace.
func (p *parser) statement() {
	start := p.i
	depth := 0
	for p.i < len(p.tokens) {
		t := p.tok(0)
		if (t.kind == tIdent && strings.HasPrefix(t.text, "@") && t.text != "@selector" && t.text != "@encode" && !(t.text == "@protocol" && p.is(1, "("))) ||
			stopper(t) || (depth == 0 && t.kind == tPunct && t.text == "}") {
			if p.i == start {
				p.i++
			}
			return // not a statement: drop what was read
		}
		if t.kind == tPunct {
			switch t.text {
			case "(", "[":
				depth++
			case ")", "]":
				depth--
			}
		}
		if depth == 0 && t.kind == tPunct && (t.text == ";" || t.text == "{") {
			break
		}
		p.i++
	}
	head := trimLeading(p.tokens[start:p.i])
	if p.i >= len(p.tokens) {
		return
	}
	if p.is(0, ";") {
		p.i++
		p.declaration(head, nil, false)
		return
	}
	// "{"
	if len(head) > 0 && (head[0].text == "namespace" || head[0].text == "extern" && len(head) == 2 && head[1].kind == tString) {
		p.frames = append(p.frames, true)
		p.i++
		return
	}
	if len(head) == 0 || head[0].text == "@" { // a stray block, an @{...} literal
		p.skipGroup()
		return
	}
	p.skipGroup()
	if isFunction(head) {
		p.declaration(head, nil, true)
		return
	}
	// A type's body or an initializer: the declaration goes on to its ";".
	tail := p.i
	p.skipTo(";")
	end := p.i
	if end > tail && p.tokens[end-1].text == ";" {
		end--
	}
	p.declaration(head, p.tokens[tail:end], true)
}

// trimLeading drops the region macros a declaration may start with
// (NS_ASSUME_NONNULL_BEGIN, CF_EXTERN_C_BEGIN), which end no statement.
func trimLeading(tokens []token) []token {
	for len(tokens) > 0 && tokens[0].kind == tIdent && (strings.HasSuffix(tokens[0].text, "_BEGIN") || strings.HasSuffix(tokens[0].text, "_END")) && upperName.MatchString(tokens[0].text) {
		tokens = tokens[1:]
	}
	return tokens
}

// isFunction reports whether a declaration head declares a function: a parameter
// list at depth 0 after a name, no "=" before it, and no type keyword first.
func isFunction(head []token) bool {
	if len(head) == 0 {
		return false
	}
	switch head[0].text {
	case "typedef", "struct", "enum", "union", "class", "template", "using", "static_assert":
		if head[0].text != "struct" || indexTop(head, "(") < 0 {
			return false
		}
	}
	eq, paren := indexTop(head, "="), funcParen(head)
	return paren > 0 && (eq < 0 || eq > paren) && !pointerDeclarator(head, paren)
}

// funcParen finds the "(" of a function's parameter list: the first at depth 0 after
// an identifier that is not an attribute macro, __attribute__ or a keyword.
func funcParen(head []token) int {
	depth := 0
	for j, t := range head {
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "(":
			if depth == 0 && j > 0 && head[j-1].kind == tIdent && !skipCall[head[j-1].text] && !macro(head[j-1].text) &&
				!(upperName.MatchString(head[j-1].text) && indexTop(head[j+1:], "(") >= 0) {
				return j
			}
			depth++
		case "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		}
	}
	return -1
}

var skipCall = map[string]bool{
	"__attribute__": true, "__declspec": true, "alignas": true, "_Alignas": true, "decltype": true,
	"sizeof": true, "if": true, "while": true, "for": true, "switch": true, "return": true,
}

// pointerDeclarator reports whether the parenthesis at paren starts `(*name)` or
// `(^name)`: a function pointer or block variable, not a function.
func pointerDeclarator(head []token, paren int) bool {
	return paren+1 < len(head) && (head[paren+1].text == "*" || head[paren+1].text == "^")
}

// declaration records what a statement declares. body says a "{...}" followed the
// head; tail is what came between it and the ";".
func (p *parser) declaration(head, tail []token, body bool) {
	if len(head) == 0 {
		return
	}
	line := head[0].line
	first := head[0].text
	switch first {
	case "template", "using", "static_assert", "return", "goto", "friend":
		return
	case "typedef":
		p.typedef(head, tail, body)
		return
	}
	if j := enumMacro(head); j >= 0 {
		p.add(declaredName(groupAfter(head, j)), "enum", head[j].line, false)
		return
	}
	if paren := funcParen(head); paren > 0 && !pointerDeclarator(head, paren) && (indexTop(head, "=") < 0 || indexTop(head, "=") > paren) {
		name := qualifiedBefore(head, paren)
		if name == "" || upperName.MatchString(name) || skipCall[name] {
			return // a macro invocation: TEST(Suite, Case), SPEC_BEGIN(X)
		}
		kind := "func"
		if strings.Contains(name, ".") {
			kind = "method"
		}
		p.add(name, kind, head[paren-1].line, !body)
		return
	}
	if kind := typeKeyword(first); kind != "" && (body || len(head) == 2) {
		if body && len(head) >= 2 && head[1].kind == tIdent && !macro(head[1].text) {
			name := head[1].text
			if name == "class" || name == "struct" { // enum class Name
				if len(head) < 3 {
					return
				}
				name = head[2].text
			}
			p.add(name, kind, head[1].line, false)
		}
		if body && len(tail) > 0 { // struct S {...} s;
			if name := declaredName(tail); name != "" {
				p.add(name, "var", line, false)
			}
		}
		return
	}
	if len(head) < 2 {
		return
	}
	switch first {
	case "@", "if", "else", "do", "while", "for", "switch", "case", "default", "break", "continue":
		return
	}
	name := declaredName(head)
	if name == "" || name == first {
		return // a lone word: a macro call without a semicolon of its own
	}
	kind := "var"
	if slices.ContainsFunc(head, func(t token) bool { return t.text == "const" }) {
		kind = "const"
	}
	p.add(name, kind, line, first == "extern" || first == "FOUNDATION_EXPORT" || first == "FOUNDATION_EXTERN" ||
		first == "UIKIT_EXTERN" || first == "APPKIT_EXTERN")
}

// typedef records a typedef's name: an NS_ENUM's second argument, the name after a
// struct or enum body, a block or function pointer type's name, else its last name.
func (p *parser) typedef(head, tail []token, body bool) {
	if j := enumMacro(head); j >= 0 {
		p.add(declaredName(groupAfter(head, j)), "enum", head[j].line, false)
		return
	}
	kind := "type"
	if len(head) > 1 {
		if k := typeKeyword(head[1].text); k != "" && body {
			kind = k
		}
	}
	decl := head[1:]
	if body {
		decl = tail
	}
	if paren := funcParen(decl); paren > 0 && !pointerDeclarator(decl, paren) {
		p.add(decl[paren-1].text, kind, head[0].line, false) // a function type
		return
	}
	if name := declaredName(decl); name != "" {
		p.add(name, kind, head[0].line, false)
	}
}

func typeKeyword(s string) string {
	switch s {
	case "struct", "union", "enum":
		return s
	case "class":
		return "class"
	}
	return ""
}

// enumMacro finds NS_ENUM( and its kin in a head.
func enumMacro(head []token) int {
	for j := 0; j+1 < len(head); j++ {
		if enumMacros[head[j].text] && head[j+1].text == "(" {
			return j
		}
	}
	return -1
}

// groupAfter is the inside of the parenthesis group following index j.
func groupAfter(head []token, j int) []token {
	start := j + 2
	depth := 1
	for k := start; k < len(head); k++ {
		switch head[k].text {
		case "(":
			depth++
		case ")":
			if depth--; depth == 0 {
				return lastArg(head[start:k])
			}
		}
	}
	return nil
}

// lastArg is the last comma-separated argument.
func lastArg(tokens []token) []token {
	for j := len(tokens) - 1; j >= 0; j-- {
		if tokens[j].text == "," {
			return tokens[j+1:]
		}
	}
	return tokens
}

// qualifiedBefore is the (C++-qualified) name before a parameter list: "Foo::bar"
// becomes "Foo.bar".
func qualifiedBefore(head []token, paren int) string {
	j := paren - 1
	if j < 0 || head[j].kind != tIdent {
		return ""
	}
	name := head[j].text
	for j >= 2 && head[j-1].text == "::" && head[j-2].kind == tIdent {
		name = head[j-2].text + "." + name
		j -= 2
	}
	return name
}

// symbols orders the definitions by line and drops a declaration whose name the
// file also defines (a prototype, an @interface of the class implemented below).
func symbols(defs []def) []lang.Symbol {
	slices.SortStableFunc(defs, func(a, b def) int {
		return cmp.Or(cmp.Compare(a.line, b.line), strings.Compare(a.name, b.name))
	})
	defined := map[string]bool{}
	for _, d := range defs {
		if !d.decl {
			defined[d.name] = true
		}
	}
	seen := map[string]bool{}
	var set lang.SymbolSet
	for _, d := range defs {
		if (d.decl && defined[d.name]) || seen[d.name+"\x00"+d.kind] {
			continue
		}
		seen[d.name+"\x00"+d.kind] = true
		set.Add(d.name, d.kind, d.line)
	}
	return set.List()
}
