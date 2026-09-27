package cpp

import (
	"sort"
	"strings"
)

// maxTrailing bounds the qualifiers, attributes and macros read after a
// parameter list (const noexcept(x) override ABSL_LOCKS_EXCLUDED(mu) ...) before
// the tokens are taken for something other than a declaration.
const maxTrailing = 32

// maxDepth bounds the scanner's recursion into class, namespace and linkage
// bodies; deeper bodies are stepped over.
const maxDepth = 200

// scanner reads the definitions of a C or C++ file from its tokens: a tolerant
// recursive descent over the declaration contexts (the file, namespaces,
// `extern "C"` blocks, class bodies) that steps over function bodies, initializers
// and anything else it does not recognise by bracket matching.
type scanner struct {
	toks       []token
	pos        int
	cplus      bool
	defs       []def
	namespaces map[string]bool
	blocks     [][2]int // token ranges of function bodies and top-level blocks
	depth      int
	angles     map[int]int // angle's answers, by the index of the '<'
}

func (s *scanner) at(i int) token {
	if i < 0 || i >= len(s.toks) {
		return token{kind: -1}
	}
	return s.toks[i]
}

// is reports whether the token at i is the punctuation or keyword text.
func (s *scanner) is(i int, text string) bool {
	t := s.at(i)
	return (t.kind == tPunct || t.kind == tIdent) && t.text == text
}

func (s *scanner) ident(i int) bool { return s.at(i).kind == tIdent }

// skipGroup returns the index after the bracket group opening at i, matching all
// three bracket kinds together so a stray closer cannot run away with the scan.
func (s *scanner) skipGroup(i int) int {
	depth := 0
	for ; i < len(s.toks); i++ {
		t := s.toks[i]
		if t.kind != tPunct || len(t.text) != 1 {
			continue
		}
		switch t.text[0] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth <= 0 {
				return i + 1
			}
		}
	}
	return len(s.toks)
}

// skipBody steps over a function body (or a block) at i and remembers its range:
// directives inside one define nothing at file scope.
func (s *scanner) skipBody(i int) int {
	end := s.skipGroup(i)
	s.blocks = append(s.blocks, [2]int{i, end})
	return end
}

// angle returns the index after the template argument list opening at i, or -1
// when the '<' is a comparison: a ';', '{' or '}' comes before its '>'. One scan
// answers for every '<' it passes, so a run of comparisons is read once.
func (s *scanner) angle(i int) int {
	if end, ok := s.angles[i]; ok {
		return end
	}
	if s.angles == nil {
		s.angles = map[int]int{}
	}
	var open []int
	for j := i; j < len(s.toks); j++ {
		t := s.toks[j]
		if t.kind != tPunct {
			continue
		}
		switch t.text {
		case "<":
			open = append(open, j)
		case ">":
			if len(open) == 0 {
				return -1 // not called at a '<'
			}
			k := open[len(open)-1]
			open = open[:len(open)-1]
			s.angles[k] = j + 1
			if len(open) == 0 {
				return j + 1
			}
		case "(", "[":
			j = s.skipGroup(j) - 1
		case ";", "{", "}", ")", "]":
			for _, k := range open {
				s.angles[k] = -1
			}
			return -1
		}
	}
	for _, k := range open {
		s.angles[k] = -1
	}
	return -1
}

// notDeclarator are the words a '(' may follow that make it no function's
// parameter list: statements, operators on types, attributes.
var notDeclarator = map[string]bool{
	"if": true, "while": true, "for": true, "switch": true, "return": true, "sizeof": true,
	"alignof": true, "_Alignof": true, "__alignof__": true, "decltype": true, "static_assert": true,
	"_Static_assert": true, "alignas": true, "_Alignas": true, "__attribute__": true, "__attribute": true,
	"__declspec": true, "noexcept": true, "throw": true, "typeof": true, "__typeof__": true,
	"__typeof": true, "asm": true, "__asm__": true, "__asm": true, "_Pragma": true, "__pragma": true,
	"requires": true, "catch": true, "defined": true, "case": true, "new": true, "delete": true,
	"void": true, "int": true, "char": true, "bool": true, "float": true, "double": true,
	"long": true, "short": true, "unsigned": true, "signed": true, "auto": true, "const": true,
	"volatile": true, "static": true, "extern": true, "inline": true, "virtual": true,
	"explicit": true, "constexpr": true, "consteval": true, "constinit": true, "typename": true,
	"struct": true, "class": true, "union": true, "enum": true, "__extension__": true,
	"_Atomic": true, "__typeof_unqual__": true, "typeof_unqual": true, "__builtin_offsetof": true,
}

// cplusOnly are the words of notDeclarator that C leaves to be names.
var cplusOnly = map[string]bool{
	"new": true, "delete": true, "class": true, "typename": true, "explicit": true, "virtual": true,
	"constexpr": true, "consteval": true, "constinit": true, "decltype": true, "noexcept": true,
	"requires": true, "catch": true, "throw": true,
}

// attribute are the words taking a parenthesized argument that never name
// anything: attributes, specifiers, assertions.
var attribute = map[string]bool{
	"__attribute__": true, "__attribute": true, "__declspec": true, "alignas": true, "_Alignas": true,
	"decltype": true, "typeof": true, "__typeof__": true, "__typeof": true, "_Atomic": true,
	"static_assert": true, "_Static_assert": true, "__pragma": true, "_Pragma": true, "noexcept": true,
	"throw": true, "asm": true, "__asm__": true, "__asm": true, "explicit": true, "requires": true,
}

// scanDefinitions reads the definitions of a C (cplus false) or C++ source whose
// dead lines (#if 0) preprocess found.
//
// Implements: REQ-CPP-003, REQ-CPP-014
func scanDefinitions(src []byte, dead []bool, cplus bool) []def {
	toks, dirs := lex(string(src), dead, cplus)
	toks, dirs = selectBranches(toks, dirs)
	s := &scanner{toks: toks, cplus: cplus, namespaces: map[string]bool{}}
	for s.pos < len(s.toks) {
		s.decls(nil, false)
	}
	s.macros(dirs)
	return s.defs
}

// decls reads declarations until the '}' closing the body (inBody) or the end.
func (s *scanner) decls(owner []string, inBody bool) {
	for s.pos < len(s.toks) {
		t := s.toks[s.pos]
		switch {
		case t.kind == tPunct && t.text == "}":
			s.pos++
			if inBody {
				return
			}
		case t.kind == tPunct && t.text == ";":
			s.pos++
		case t.kind == tPunct && t.text == "{":
			// A block where a declaration belongs: a statement at file scope, or a
			// body whose head the scanner did not recognise.
			s.pos = s.skipBody(s.pos)
		case s.cplus && t.text == "namespace" && t.kind == tIdent:
			s.namespace(owner)
		case t.text == "extern" && t.kind == tIdent && s.at(s.pos+1).kind == tString && s.is(s.pos+2, "{"):
			s.pos += 3
			s.nested(func() { s.decls(owner, true) })
		case s.cplus && t.text == "template" && t.kind == tIdent:
			s.pos++
			if s.is(s.pos, "<") {
				if end := s.angle(s.pos); end > 0 {
					s.pos = end
				} else {
					s.pos++
				}
			}
		case s.cplus && s.access():
		case s.skipMacroLines():
		default:
			start := s.pos
			s.statement(owner)
			if s.pos == start {
				s.pos++
			}
		}
	}
}

// nested runs body one level deeper, or steps over the body when too deep.
func (s *scanner) nested(body func()) {
	if s.depth >= maxDepth {
		s.pos = s.skipGroup(s.pos - 1)
		return
	}
	s.depth++
	body()
	s.depth--
}

// access steps over an access specifier (`public:`, Qt's `public slots:`,
// `signals:`).
func (s *scanner) access() bool {
	switch s.at(s.pos).text {
	case "public", "private", "protected", "signals", "Q_SIGNALS", "slots", "Q_SLOTS":
	default:
		return false
	}
	for j := s.pos + 1; j < s.pos+3; j++ {
		if s.is(j, ":") {
			s.pos = j + 1
			return true
		}
		if !s.ident(j) {
			return false
		}
	}
	return false
}

func (s *scanner) namespace(owner []string) {
	s.pos++ // namespace
	var name strings.Builder
	line := 0
	for s.pos < len(s.toks) {
		t := s.toks[s.pos]
		switch {
		case t.kind == tIdent && attribute[t.text] && s.is(s.pos+1, "("):
			s.pos = s.skipGroup(s.pos + 1)
			continue
		case s.is(s.pos, "[") && s.is(s.pos+1, "["):
			s.pos = s.skipGroup(s.pos)
			continue
		case t.kind == tIdent || t.text == "::":
			if t.kind == tIdent && line == 0 {
				line = t.line
			}
			if t.text != "inline" { // namespace a::inline b
				name.WriteString(t.text)
			}
			s.pos++
			continue
		case t.text == "{":
			s.pos++
			if n := strings.ReplaceAll(name.String(), "::", "."); n != "" && !s.namespaces[n] {
				s.namespaces[n] = true
				s.defs = append(s.defs, def{n, "namespace", line, false})
			}
			s.nested(func() { s.decls(owner, true) })
			return
		}
		// An alias (namespace a = b;) or something else: the statement goes on.
		s.statement(owner)
		return
	}
}

// ownerName joins the classes around a definition, outermost first; anonymous
// ones are left out.
func ownerName(owner []string) string {
	var names []string
	for _, o := range owner {
		if o != "" {
			names = append(names, o)
		}
	}
	return strings.Join(names, ".")
}

func withOwner(owner []string, name string) string {
	if o := ownerName(owner); o != "" {
		return o + "." + name
	}
	return name
}

// statement reads one declaration up to its ';' (or a function's body), recording
// the classes, types, aliases and functions it defines or declares.
func (s *scanner) statement(owner []string) {
	start := s.pos
	typedef := false
	if s.cplus && s.is(s.pos, "using") {
		if s.ident(s.pos+1) && s.is(s.pos+2, "=") {
			name := s.toks[s.pos+1]
			s.defs = append(s.defs, def{withOwner(owner, name.text), "type", name.line, false})
		}
		s.pos = s.skipTo(s.pos)
		return
	}
	declStart := s.pos
	for s.pos < len(s.toks) {
		t := s.toks[s.pos]
		if t.kind == tPunct {
			switch t.text {
			case ";":
				if typedef {
					s.typedefs(owner, declStart, s.pos)
				}
				s.pos++
				return
			case "}":
				if typedef {
					s.typedefs(owner, declStart, s.pos)
				}
				return
			case "{":
				s.pos = s.skipGroup(s.pos)
				if !s.is(s.pos, ";") && !s.is(s.pos, ",") {
					return
				}
				continue
			case "=":
				s.pos = s.initializer(s.pos + 1)
				continue
			case "[":
				s.pos = s.skipGroup(s.pos)
				continue
			case "(":
				if !typedef && s.function(owner, start) {
					if s.pos > 0 && s.is(s.pos-1, "}") {
						return // a definition's body ends the statement
					}
					continue
				}
				s.pos = s.skipGroup(s.pos)
				continue
			case "<":
				if s.pos > start && (s.ident(s.pos-1) || s.is(s.pos-1, ">")) {
					if end := s.angle(s.pos); end > 0 {
						s.pos = end
						continue
					}
				}
			}
			s.pos++
			continue
		}
		if t.kind != tIdent {
			s.pos++
			continue
		}
		switch t.text {
		case "typedef":
			typedef = true
			declStart = s.pos + 1
		case "class", "struct", "union", "enum":
			if !s.cplus && t.text == "class" {
				break
			}
			if s.record(owner) {
				declStart = s.pos // the declarators after the body
			}
			continue
		case "operator":
			if s.cplus {
				if s.operator(owner, start) {
					if s.pos > 0 && s.is(s.pos-1, "}") {
						return
					}
				}
				continue
			}
		default:
			if attribute[t.text] && s.is(s.pos+1, "(") {
				s.pos = s.skipGroup(s.pos + 1)
				continue
			}
		}
		s.pos++
	}
}

// skipMacroLines steps over macro invocations standing alone on their lines at
// the start of a declaration (ABSL_NAMESPACE_BEGIN, Q_OBJECT, Q_PROPERTY(...)):
// an all-capitals name, perhaps with arguments, after which the next token is
// on a later line.
func (s *scanner) skipMacroLines() bool {
	skipped := false
	for s.ident(s.pos) && upper.MatchString(s.toks[s.pos].text) {
		end := s.pos + 1
		if s.is(end, "(") {
			end = s.skipGroup(end)
		}
		next := s.at(end)
		if next.kind < 0 || next.line <= s.toks[end-1].line {
			return skipped
		}
		switch next.text {
		case "{", ";", ":", "::", "(", "=", ",", "<", "*", "&", "const", "noexcept", "override", "final", "->", "try":
			return skipped
		}
		s.pos = end
		skipped = true
	}
	return skipped
}

// skipTo returns the index after the ';' ending the statement at i, stepping over
// bracket groups; a '}' of the enclosing body ends it too (not consumed).
func (s *scanner) skipTo(i int) int {
	for i < len(s.toks) {
		t := s.toks[i]
		if t.kind == tPunct {
			switch t.text {
			case ";":
				return i + 1
			case "}":
				return i
			case "(", "[", "{":
				i = s.skipGroup(i)
				continue
			}
		}
		i++
	}
	return i
}

// initializer steps over an initializer from i to the ',' or ';' after it (not
// consumed).
func (s *scanner) initializer(i int) int {
	for i < len(s.toks) {
		t := s.toks[i]
		if t.kind == tPunct {
			switch t.text {
			case ";", ",", "}":
				return i
			case "(", "[", "{":
				i = s.skipGroup(i)
				continue
			case "<":
				if i > 0 && s.ident(i-1) {
					if end := s.angle(i); end > 0 {
						i = end
						continue
					}
				}
			}
		}
		i++
	}
	return i
}

// record reads a class, struct, union or enum specifier at the keyword. With a
// body it records the type, reads a class body's members, and returns true;
// without one (an elaborated type, a forward declaration) the declaration goes on
// after the name.
func (s *scanner) record(owner []string) bool {
	kw := s.toks[s.pos]
	j := s.pos + 1
	if kw.text == "enum" && (s.is(j, "class") || s.is(j, "struct")) {
		j++
	}
	var name strings.Builder
	line := 0
	for j < len(s.toks) {
		t := s.toks[j]
		switch {
		case s.is(j, "[") && s.is(j+1, "["):
			j = s.skipGroup(j)
			continue
		case t.kind == tIdent && s.is(j+1, "(") && (attribute[t.text] || upper.MatchString(t.text)):
			j = s.skipGroup(j + 1) // __declspec(dllexport), EXPORT_MACRO(x)
			name.Reset()
			continue
		case t.kind == tIdent && (t.text == "final" || t.text == "sealed") && name.Len() > 0:
			j++
			continue
		case t.kind == tIdent:
			if n := name.String(); n != "" && !strings.HasSuffix(n, "::") {
				if !upper.MatchString(n) {
					// `struct stat st`: a declaration of a variable, not a body.
					s.pos = j
					return false
				}
				name.Reset() // `class EXPORT Name`: the last name counts
			}
			name.WriteString(t.text)
			line = t.line
			j++
			continue
		case t.text == "::" && s.cplus:
			name.WriteString("::")
			j++
			continue
		case t.text == "<" && name.Len() > 0:
			if end := s.angle(j); end > 0 {
				j = end
				continue
			}
		}
		break
	}
	if s.is(j, ":") {
		// Bases, or an enum's underlying type.
		k := j + 1
		for ; k < len(s.toks); k++ {
			t := s.toks[k]
			if t.kind != tPunct {
				continue
			}
			if t.text == "<" {
				if end := s.angle(k); end > 0 {
					k = end - 1
				}
				continue
			}
			if t.text == "(" {
				k = s.skipGroup(k) - 1
				continue
			}
			if t.text == "{" || t.text == ";" || t.text == "}" {
				break
			}
		}
		j = k
	}
	if !s.is(j, "{") {
		s.pos = j
		return false
	}
	n := qualified(name.String())
	if n != "" {
		kind := kw.text
		s.defs = append(s.defs, def{withOwner(owner, n), kind, line, false})
	}
	s.pos = j + 1
	if kw.text == "enum" {
		s.pos = s.skipGroup(j)
		return true
	}
	s.nested(func() { s.decls(append(append([]string{}, owner...), n), true) })
	return true
}

// typedefs records the names a typedef declares in tokens [from, to): the last
// name of each declarator, or the one inside a parenthesized declarator
// ((*callback_t)(int)). After a body every declarator is a name of its own.
func (s *scanner) typedefs(owner []string, from, to int) {
	for from < to {
		end := from
		for end < to && !s.is(end, ",") {
			switch {
			case s.is(end, "(") || s.is(end, "[") || s.is(end, "{"):
				end = s.skipGroup(end)
				continue
			case s.is(end, "<"):
				if e := s.angle(end); e > 0 && e <= to {
					end = e
					continue
				}
			}
			end++
		}
		if name, line := s.declarator(from, min(end, to)); name != "" {
			s.defs = append(s.defs, def{withOwner(owner, name), "type", line, false})
		}
		from = end + 1
	}
}

// declarator finds the name a declarator in [from, to) declares.
func (s *scanner) declarator(from, to int) (string, int) {
	var last token
	for i := from; i < to; i++ {
		t := s.toks[i]
		switch {
		case t.text == "(" && t.kind == tPunct:
			end := s.skipGroup(i)
			// (name)(params) and (*name)(params) declare name; type(params) is a
			// function type named by the last name before it.
			single := end == i+3 && s.ident(i+1) && (s.is(end, "(") || s.is(end, "["))
			if last.text == "" || single || s.is(i+1, "*") || s.is(i+1, "&") || s.is(i+1, "^") || callingConvention[s.at(i+1).text] {
				for k := i + 1; k < end-1; k++ {
					if s.ident(k) && !callingConvention[s.toks[k].text] && !qualifier[s.toks[k].text] {
						return s.toks[k].text, s.toks[k].line
					}
				}
			}
			if last.text != "" {
				return last.text, last.line
			}
			i = end - 1
		case t.text == "[" && t.kind == tPunct:
			i = s.skipGroup(i) - 1
		case t.text == "<" && t.kind == tPunct:
			if e := s.angle(i); e > 0 {
				i = e - 1
			}
		case t.kind == tIdent && !qualifier[t.text] && !callingConvention[t.text]:
			if attribute[t.text] && s.is(i+1, "(") {
				i = s.skipGroup(i+1) - 1
				continue
			}
			last = t
		}
	}
	return last.text, last.line
}

var callingConvention = map[string]bool{
	"__cdecl": true, "__stdcall": true, "__fastcall": true, "__thiscall": true, "__vectorcall": true,
	"WINAPI": true, "CALLBACK": true, "APIENTRY": true, "__clrcall": true,
}

var qualifier = map[string]bool{
	"const": true, "volatile": true, "restrict": true, "__restrict": true, "__restrict__": true,
	"_Nonnull": true, "_Nullable": true, "__unaligned": true,
}

// function reads a function declarator whose parameter list opens at s.pos,
// named by the tokens before it. It records the function and returns true, with
// s.pos after the declaration (after its body, its ';' left in place, or after
// a ','); false leaves s.pos alone.
func (s *scanner) function(owner []string, start int) bool {
	open := s.pos
	if !s.params(open) {
		return false
	}
	name, line, first, ok := s.nameBefore(open, start)
	if !ok {
		return false
	}
	if upper.MatchString(name) && s.is(s.skipGroup(open), "(") {
		return false // NAME(x)(params): the name is NAME(x)
	}
	return s.afterParams(owner, name, line, open, first == start)
}

// params reports whether the group opening at open can be a parameter list: not
// a parenthesized declarator ((*name)) and not a constructor call's arguments
// (x(nullptr), x(0)).
func (s *scanner) params(open int) bool {
	t := s.at(open + 1)
	switch t.kind {
	case tNumber, tString, tChar:
		return false
	case tPunct:
		switch t.text {
		case "*", "&", "^", "-", "!", "{", "~":
			return false
		}
	case tIdent:
		switch t.text {
		case "nullptr", "true", "false", "this", "NULL", "sizeof":
			return false
		}
	}
	return true
}

// operator reads an operator function's name from the keyword on and its
// declaration; conversion operators (operator bool) declare no symbol.
func (s *scanner) operator(owner []string, start int) bool {
	kw := s.pos
	j := kw + 1
	var op strings.Builder
	switch {
	case s.is(j, "(") && s.is(j+1, ")"):
		op.WriteString("()")
		j += 2
	case s.is(j, "[") && s.is(j+1, "]"):
		op.WriteString("[]")
		j += 2
	case s.at(j).kind == tString:
		op.WriteString(s.toks[j].text)
		j++
		if s.ident(j) {
			op.WriteString(s.toks[j].text)
			j++
		}
	case s.is(j, "new") || s.is(j, "delete") || s.is(j, "co_await"):
		op.WriteString(s.toks[j].text)
		j++
		if s.is(j, "[") && s.is(j+1, "]") {
			op.WriteString("[]")
			j += 2
		}
	case s.ident(j):
		// A conversion operator (operator bool): its declaration declares no
		// symbol, but its body must still be stepped over.
		for j < len(s.toks) && !s.is(j, "(") && !s.is(j, ";") && !s.is(j, "{") && !s.is(j, "}") {
			if s.is(j, "<") {
				if end := s.angle(j); end > 0 {
					j = end
					continue
				}
			}
			j++
		}
		if !s.is(j, "(") {
			s.pos = j
			return false
		}
		s.pos = j
		return s.afterParams(owner, "", 0, j, false)
	default:
		for s.at(j).kind == tPunct && s.toks[j].text != "(" && j < kw+4 {
			op.WriteString(s.toks[j].text)
			j++
		}
	}
	if !s.is(j, "(") || op.Len() == 0 {
		s.pos = j
		return false
	}
	// The name: a qualifier before the keyword (Foo<T>::operator==).
	prefix, line, first := "", s.toks[kw].line, kw
	if s.is(kw-1, "::") {
		if q, l, f, ok := s.nameBefore(kw-1, start); ok {
			prefix, line, first = q+"::", l, f
		}
	}
	s.pos = j
	return s.afterParams(owner, prefix+"operator"+op.String(), line, j, first == start)
}

// nameBefore reads the declarator name ending right before the '(' at open:
// `name`, `~name`, `ns::Class<T>::name`, or a parenthesized `(*name)`.
func (s *scanner) nameBefore(open, start int) (string, int, int, bool) {
	j := open - 1
	if j < start {
		return "", 0, 0, false
	}
	t := s.toks[j]
	if t.kind == tPunct && t.text == ")" {
		// (*name)(params): a pointer to a function, named as written, as a parser
		// names it; NAME(x)(params): a name a macro makes.
		k := j - 1
		if k-2 >= start && s.ident(k) && (s.is(k-1, "*") || s.is(k-1, "&")) && s.is(k-2, "(") {
			return "(" + s.toks[k-1].text + s.toks[k].text + ")", s.toks[k].line, k - 2, true
		}
		for k = j - 1; k > start && !s.is(k, "("); k-- {
			if !s.ident(k) && !s.is(k, ",") {
				return "", 0, 0, false
			}
		}
		if k-1 >= start && s.ident(k-1) && upper.MatchString(s.toks[k-1].text) {
			var b strings.Builder
			for i := k - 1; i < open; i++ {
				b.WriteString(s.toks[i].text)
			}
			return b.String(), s.toks[k-1].line, k - 1, true
		}
		return "", 0, 0, false
	}
	end := j + 1
	for {
		switch {
		case s.is(j, ">"):
			// Template arguments: walk back to their '<'.
			depth := 0
			k := j
			for ; k >= start && k > j-256; k-- {
				if s.is(k, ">") {
					depth++
				} else if s.is(k, "<") {
					depth--
					if depth == 0 {
						break
					}
				} else if s.is(k, ";") || s.is(k, "{") || s.is(k, "}") {
					return "", 0, 0, false
				}
			}
			if k < start || k <= j-256 || !s.ident(k-1) {
				return "", 0, 0, false
			}
			j = k - 1
		case s.ident(j):
		default:
			return "", 0, 0, false
		}
		if w := s.toks[j].text; notDeclarator[w] && (s.cplus || !cplusOnly[w]) {
			return "", 0, 0, false
		}
		if s.is(j-1, "~") {
			j--
		}
		if s.is(j-1, "::") {
			j -= 2
			if j < start || !(s.ident(j) || s.is(j, ">")) {
				j++ // ::name
				break
			}
			continue
		}
		break
	}
	if s.is(j-1, ".") || s.is(j-1, "->") {
		return "", 0, 0, false // a member call
	}
	var b strings.Builder
	line := 0
	for k := j; k < end; k++ {
		b.WriteString(s.toks[k].text)
		if s.ident(k) {
			line = s.toks[k].line
		}
	}
	return b.String(), line, j, true
}

// afterParams reads what follows a function's parameter list (at open): trailing
// qualifiers, then a body, a constructor's initializers, `= default`, or the ';'
// of a declaration.
func (s *scanner) afterParams(owner []string, name string, line, open int, untyped bool) bool {
	i := s.skipGroup(open)
	for n := 0; i < len(s.toks) && n < maxTrailing; n++ {
		t := s.toks[i]
		if t.kind == tIdent {
			switch {
			case t.text == "try":
				s.add(owner, name, line, false)
				i++
				if s.is(i, ":") {
					i = s.ctorInit(i + 1)
				}
				if s.is(i, "{") {
					i = s.skipBody(i)
				}
				for s.is(i, "catch") && s.is(i+1, "(") {
					i = s.skipGroup(i + 1)
					if s.is(i, "{") {
						i = s.skipBody(i)
					}
				}
				s.pos = i
				if !s.is(i-1, "}") {
					s.pos = s.skipTo(i)
				}
				return true
			case s.is(i+1, "("):
				i = s.skipGroup(i + 1) // noexcept(x), throw(), ABSL_LOCKS_EXCLUDED(mu)
			default:
				i++ // const, override, final, a macro
			}
			continue
		}
		if t.kind != tPunct {
			return false // a literal: a call or an initializer, not a declarator
		}
		switch t.text {
		case "&", "*":
			i++
		case "[":
			i = s.skipGroup(i)
		case "(":
			i = s.skipGroup(i)
		case "->":
			i = s.trailingReturn(i + 1)
		case "{":
			s.add(owner, name, line, false)
			s.pos = s.skipBody(i)
			return true
		case ":":
			s.add(owner, name, line, false)
			i = s.ctorInit(i + 1)
			if s.is(i, "{") {
				i = s.skipBody(i)
			}
			s.pos = i
			return true
		case "=":
			switch s.at(i + 1).text {
			case "default", "delete":
				s.add(owner, name, line, false)
			case "0":
				s.add(owner, name, line, true)
			default:
				if !strings.HasPrefix(name, "(") || len(owner) == 0 {
					return false // an initializer
				}
				s.add(owner, name, line, true) // a member pointer to a function
			}
			s.pos = s.skipTo(i) - 1
			return true
		case ";", ",", "}":
			if untyped && len(owner) == 0 && !strings.Contains(name, "::") {
				return false // f(x); at file scope: a macro call
			}
			s.add(owner, name, line, true)
			s.pos = i
			if t.text == "," {
				s.pos++
			}
			return true
		default:
			return false
		}
	}
	return false
}

// trailingReturn steps over a trailing return type from i to the body, ';' or '='.
func (s *scanner) trailingReturn(i int) int {
	for i < len(s.toks) {
		t := s.toks[i]
		if t.kind == tPunct {
			switch t.text {
			case "{", ";", "=", "}":
				return i
			case "(", "[":
				i = s.skipGroup(i)
				continue
			case "<":
				if end := s.angle(i); end > 0 {
					i = end
					continue
				}
			}
		}
		i++
	}
	return i
}

// ctorInit steps over a constructor's member initializers to its body.
func (s *scanner) ctorInit(i int) int {
	for i < len(s.toks) {
		t := s.toks[i]
		if t.kind == tPunct {
			switch t.text {
			case "{":
				if s.is(i-1, ")") || s.is(i-1, "}") || s.is(i-1, ":") {
					return i
				}
				i = s.skipGroup(i)
				continue
			case "(", "[":
				i = s.skipGroup(i)
				continue
			case ";", "}":
				return i
			case "<":
				if end := s.angle(i); end > 0 {
					i = end
					continue
				}
			}
		}
		i++
	}
	return i
}

// add records a function or method; test and benchmark macros (all capitals) are
// not functions.
func (s *scanner) add(owner []string, text string, line int, decl bool) {
	name := qualified(text)
	if name == "" || macroCall(name) {
		return
	}
	name = withOwner(owner, name)
	kind := "func"
	if strings.Contains(name, ".") {
		kind = "method"
	}
	s.defs = append(s.defs, def{name, kind, line, decl})
}

// macros records the #defines outside function bodies. A valueless #define of
// the name an enclosing #ifdef or #ifndef tests is a header guard, not a symbol.
func (s *scanner) macros(dirs []directive) {
	// The blocks are disjoint (a body is stepped over whole) and recorded in order.
	inBlock := func(at int) bool {
		k := sort.Search(len(s.blocks), func(i int) bool { return s.blocks[i][0] >= at }) - 1
		return k >= 0 && at < s.blocks[k][1]
	}
	var tested []string
	for _, d := range dirs {
		switch d.word {
		case "if":
			tested = append(tested, "")
		case "ifdef", "ifndef":
			tested = append(tested, firstWord(d.rest))
		case "endif":
			if n := len(tested); n > 0 {
				tested = tested[:n-1]
			}
		case "define":
			name := firstWord(d.rest)
			if name == "" || inBlock(d.at) {
				continue
			}
			value := strings.TrimSpace(d.rest[len(name):])
			if value == "" && !strings.HasPrefix(d.rest[len(name):], "(") {
				guard := false
				for _, n := range tested {
					if n == name {
						guard = true
					}
				}
				if guard {
					continue
				}
			}
			s.defs = append(s.defs, def{name, "macro", d.line, false})
		}
	}
}

func firstWord(s string) string {
	i := 0
	for i < len(s) && identPart(s[i]) {
		i++
	}
	if i == 0 || !identStart(s[0]) {
		return ""
	}
	return s[:i]
}
