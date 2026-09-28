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
// and anything else it does not recognize by bracket matching.
type scanner struct {
	tokens      []token
	position    int
	cplus       bool
	definitions []definition
	namespaces  map[string]bool
	blocks      [][2]int // token ranges of function bodies and top-level blocks
	depth       int
	angles      map[int]int // angle's answers, by the index of the '<'
}

func (s *scanner) at(i int) token {
	if i < 0 || i >= len(s.tokens) {
		return token{kind: -1}
	}
	return s.tokens[i]
}

// is reports whether the token at i is the punctuation or keyword text.
func (s *scanner) is(i int, text string) bool {
	t := s.at(i)
	return (t.kind == tPunctuation || t.kind == tIdentifier) && t.text == text
}

func (s *scanner) identifier(i int) bool { return s.at(i).kind == tIdentifier }

// skipGroup returns the index after the bracket group opening at i, matching all
// three bracket kinds together so a stray closer cannot run away with the scan.
func (s *scanner) skipGroup(i int) int {
	depth := 0
	for ; i < len(s.tokens); i++ {
		t := s.tokens[i]
		if t.kind != tPunctuation || len(t.text) != 1 {
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
	return len(s.tokens)
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
	for j := i; j < len(s.tokens); j++ {
		t := s.tokens[j]
		if t.kind != tPunctuation {
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
//
// Implements: REQ-CPP-015
var attribute = map[string]bool{
	"__attribute__": true, "__attribute": true, "__declspec": true, "alignas": true, "_Alignas": true,
	"decltype": true, "typeof": true, "__typeof__": true, "__typeof": true, "_Atomic": true,
	"static_assert": true, "_Static_assert": true, "__pragma": true, "_Pragma": true, "noexcept": true,
	"throw": true, "asm": true, "__asm__": true, "__asm": true, "explicit": true, "requires": true,
	// CUDA's launch attributes: __launch_bounds__(THREADS) __global__ void k().
	"__launch_bounds__": true, "__cluster_dims__": true, "__maxnreg__": true,
}

// scanDefinitions reads the definitions of a C (cplus false) or C++ source whose
// dead lines (#if 0) preprocess found.
//
// Implements: REQ-CPP-003, REQ-CPP-014
func scanDefinitions(source []byte, dead []bool, cplus bool) []definition {
	tokens, directories := lex(string(source), dead, cplus)
	tokens, directories = selectBranches(tokens, directories)
	s := &scanner{tokens: tokens, cplus: cplus, namespaces: map[string]bool{}}
	for s.position < len(s.tokens) {
		s.declarations(nil, false)
	}
	s.macros(directories)
	return s.definitions
}

// declarations reads declarations until the '}' closing the body (inBody) or the end.
func (s *scanner) declarations(owner []string, inBody bool) {
	for s.position < len(s.tokens) {
		t := s.tokens[s.position]
		switch {
		case t.kind == tPunctuation && t.text == "}":
			s.position++
			if inBody {
				return
			}
		case t.kind == tPunctuation && t.text == ";":
			s.position++
		case t.kind == tPunctuation && t.text == "{":
			// A block where a declaration belongs: a statement at file scope, or a
			// body whose head the scanner did not recognize.
			s.position = s.skipBody(s.position)
		case s.cplus && t.text == "namespace" && t.kind == tIdentifier:
			s.namespace(owner)
		case t.text == "extern" && t.kind == tIdentifier && s.at(s.position+1).kind == tString && s.is(s.position+2, "{"):
			s.position += 3
			s.nested(func() { s.declarations(owner, true) })
		case s.cplus && t.text == "template" && t.kind == tIdentifier:
			s.position++
			if s.is(s.position, "<") {
				if end := s.angle(s.position); end > 0 {
					s.position = end
				} else {
					s.position++
				}
			}
		case s.cplus && s.access():
		case s.skipMacroLines():
		default:
			start := s.position
			s.statement(owner)
			if s.position == start {
				s.position++
			}
		}
	}
}

// nested runs body one level deeper, or steps over the body when too deep.
func (s *scanner) nested(body func()) {
	if s.depth >= maxDepth {
		s.position = s.skipGroup(s.position - 1)
		return
	}
	s.depth++
	body()
	s.depth--
}

// access steps over an access specifier (`public:`, Qt's `public slots:`,
// `signals:`).
func (s *scanner) access() bool {
	switch s.at(s.position).text {
	case "public", "private", "protected", "signals", "Q_SIGNALS", "slots", "Q_SLOTS":
	default:
		return false
	}
	for j := s.position + 1; j < s.position+3; j++ {
		if s.is(j, ":") {
			s.position = j + 1
			return true
		}
		if !s.identifier(j) {
			return false
		}
	}
	return false
}

func (s *scanner) namespace(owner []string) {
	s.position++ // namespace
	var name strings.Builder
	line := 0
	for s.position < len(s.tokens) {
		t := s.tokens[s.position]
		switch {
		case t.kind == tIdentifier && attribute[t.text] && s.is(s.position+1, "("):
			s.position = s.skipGroup(s.position + 1)
			continue
		case s.is(s.position, "[") && s.is(s.position+1, "["):
			s.position = s.skipGroup(s.position)
			continue
		case t.kind == tIdentifier || t.text == "::":
			if t.kind == tIdentifier && line == 0 {
				line = t.line
			}
			if t.text != "inline" { // namespace a::inline b
				name.WriteString(t.text)
			}
			s.position++
			continue
		case t.text == "{":
			s.position++
			if n := strings.ReplaceAll(name.String(), "::", "."); n != "" && !s.namespaces[n] {
				s.namespaces[n] = true
				s.definitions = append(s.definitions, definition{n, "namespace", line, false})
			}
			s.nested(func() { s.declarations(owner, true) })
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
	start := s.position
	typedef := false
	if s.cplus && s.is(s.position, "using") {
		if s.identifier(s.position+1) && s.is(s.position+2, "=") {
			name := s.tokens[s.position+1]
			s.definitions = append(s.definitions, definition{withOwner(owner, name.text), "type", name.line, false})
		}
		s.position = s.skipTo(s.position)
		return
	}
	declarationStart := s.position
	for s.position < len(s.tokens) {
		t := s.tokens[s.position]
		if t.kind == tPunctuation {
			switch t.text {
			case ";":
				if typedef {
					s.typedefs(owner, declarationStart, s.position)
				}
				s.position++
				return
			case "}":
				if typedef {
					s.typedefs(owner, declarationStart, s.position)
				}
				return
			case "{":
				s.position = s.skipGroup(s.position)
				if !s.is(s.position, ";") && !s.is(s.position, ",") {
					return
				}
				continue
			case "=":
				s.position = s.initializer(s.position + 1)
				continue
			case "[":
				s.position = s.skipGroup(s.position)
				continue
			case "(":
				if !typedef && s.function(owner, start) {
					if s.position > 0 && s.is(s.position-1, "}") {
						return // a definition's body ends the statement
					}
					continue
				}
				s.position = s.skipGroup(s.position)
				continue
			case "<":
				if s.position > start && (s.identifier(s.position-1) || s.is(s.position-1, ">")) {
					if end := s.angle(s.position); end > 0 {
						s.position = end
						continue
					}
				}
			}
			s.position++
			continue
		}
		if t.kind != tIdentifier {
			s.position++
			continue
		}
		switch t.text {
		case "typedef":
			typedef = true
			declarationStart = s.position + 1
		case "class", "struct", "union", "enum":
			if !s.cplus && t.text == "class" {
				break
			}
			if s.record(owner) {
				declarationStart = s.position // the declarators after the body
			}
			continue
		case "operator":
			if s.cplus {
				if s.operator(owner, start) {
					if s.position > 0 && s.is(s.position-1, "}") {
						return
					}
				}
				continue
			}
		default:
			if attribute[t.text] && s.is(s.position+1, "(") {
				s.position = s.skipGroup(s.position + 1)
				continue
			}
		}
		s.position++
	}
}

// skipMacroLines steps over macro invocations standing alone on their lines at
// the start of a declaration (ABSL_NAMESPACE_BEGIN, Q_OBJECT, Q_PROPERTY(...)):
// an all-capitals name, perhaps with arguments, after which the next token is
// on a later line.
func (s *scanner) skipMacroLines() bool {
	skipped := false
	for s.identifier(s.position) && upper.MatchString(s.tokens[s.position].text) {
		end := s.position + 1
		if s.is(end, "(") {
			end = s.skipGroup(end)
		}
		next := s.at(end)
		if next.kind < 0 || next.line <= s.tokens[end-1].line {
			return skipped
		}
		switch next.text {
		case "{", ";", ":", "::", "(", "=", ",", "<", "*", "&", "const", "noexcept", "override", "final", "->", "try":
			return skipped
		}
		s.position = end
		skipped = true
	}
	return skipped
}

// skipTo returns the index after the ';' ending the statement at i, stepping over
// bracket groups; a '}' of the enclosing body ends it too (not consumed).
func (s *scanner) skipTo(i int) int {
	for i < len(s.tokens) {
		t := s.tokens[i]
		if t.kind == tPunctuation {
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
	for i < len(s.tokens) {
		t := s.tokens[i]
		if t.kind == tPunctuation {
			switch t.text {
			case ";", ",", "}":
				return i
			case "(", "[", "{":
				i = s.skipGroup(i)
				continue
			case "<":
				if i > 0 && s.identifier(i-1) {
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
	keyword := s.tokens[s.position]
	j := s.position + 1
	if keyword.text == "enum" && (s.is(j, "class") || s.is(j, "struct")) {
		j++
	}
	var name strings.Builder
	line := 0
	for j < len(s.tokens) {
		t := s.tokens[j]
		switch {
		case s.is(j, "[") && s.is(j+1, "["):
			j = s.skipGroup(j)
			continue
		case t.kind == tIdentifier && s.is(j+1, "(") && (attribute[t.text] || upper.MatchString(t.text)):
			j = s.skipGroup(j + 1) // __declspec(dllexport), EXPORT_MACRO(x)
			name.Reset()
			continue
		case t.kind == tIdentifier && (t.text == "final" || t.text == "sealed") && name.Len() > 0:
			j++
			continue
		case t.kind == tIdentifier:
			if n := name.String(); n != "" && !strings.HasSuffix(n, "::") {
				if !upper.MatchString(n) {
					// `struct stat st`: a declaration of a variable, not a body.
					s.position = j
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
		for ; k < len(s.tokens); k++ {
			t := s.tokens[k]
			if t.kind != tPunctuation {
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
		s.position = j
		return false
	}
	n := qualified(name.String())
	if n != "" {
		kind := keyword.text
		s.definitions = append(s.definitions, definition{withOwner(owner, n), kind, line, false})
	}
	s.position = j + 1
	if keyword.text == "enum" {
		s.position = s.skipGroup(j)
		return true
	}
	s.nested(func() { s.declarations(append(append([]string{}, owner...), n), true) })
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
			s.definitions = append(s.definitions, definition{withOwner(owner, name), "type", line, false})
		}
		from = end + 1
	}
}

// declarator finds the name a declarator in [from, to) declares.
func (s *scanner) declarator(from, to int) (string, int) {
	var last token
	for i := from; i < to; i++ {
		t := s.tokens[i]
		switch {
		case t.text == "(" && t.kind == tPunctuation:
			end := s.skipGroup(i)
			// (name)(params) and (*name)(params) declare name; type(params) is a
			// function type named by the last name before it.
			single := end == i+3 && s.identifier(i+1) && (s.is(end, "(") || s.is(end, "["))
			if last.text == "" || single || s.is(i+1, "*") || s.is(i+1, "&") || s.is(i+1, "^") || callingConvention[s.at(i+1).text] {
				for k := i + 1; k < end-1; k++ {
					if s.identifier(k) && !callingConvention[s.tokens[k].text] && !qualifier[s.tokens[k].text] {
						return s.tokens[k].text, s.tokens[k].line
					}
				}
			}
			if last.text != "" {
				return last.text, last.line
			}
			i = end - 1
		case t.text == "[" && t.kind == tPunctuation:
			i = s.skipGroup(i) - 1
		case t.text == "<" && t.kind == tPunctuation:
			if e := s.angle(i); e > 0 {
				i = e - 1
			}
		case t.kind == tIdentifier && !qualifier[t.text] && !callingConvention[t.text]:
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

// function reads a function declarator whose parameter list opens at s.position,
// named by the tokens before it. It records the function and returns true, with
// s.position after the declaration (after its body, its ';' left in place, or after
// a ','); false leaves s.position alone.
func (s *scanner) function(owner []string, start int) bool {
	open := s.position
	if !s.parameters(open) {
		return false
	}
	name, line, first, ok := s.nameBefore(open, start)
	if !ok {
		return false
	}
	if upper.MatchString(name) && s.is(s.skipGroup(open), "(") {
		return false // NAME(x)(params): the name is NAME(x)
	}
	return s.afterParameters(owner, name, line, open, first == start)
}

// parameters reports whether the group opening at open can be a parameter list: not
// a parenthesized declarator ((*name)) and not a constructor call's arguments
// (x(nullptr), x(0)).
func (s *scanner) parameters(open int) bool {
	t := s.at(open + 1)
	switch t.kind {
	case tNumber, tString, tCharacter:
		return false
	case tPunctuation:
		switch t.text {
		case "*", "&", "^", "-", "!", "{", "~":
			return false
		}
	case tIdentifier:
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
	keyword := s.position
	j := keyword + 1
	var operator strings.Builder
	switch {
	case s.is(j, "(") && s.is(j+1, ")"):
		operator.WriteString("()")
		j += 2
	case s.is(j, "[") && s.is(j+1, "]"):
		operator.WriteString("[]")
		j += 2
	case s.at(j).kind == tString:
		operator.WriteString(s.tokens[j].text)
		j++
		if s.identifier(j) {
			operator.WriteString(s.tokens[j].text)
			j++
		}
	case s.is(j, "new") || s.is(j, "delete") || s.is(j, "co_await"):
		operator.WriteString(s.tokens[j].text)
		j++
		if s.is(j, "[") && s.is(j+1, "]") {
			operator.WriteString("[]")
			j += 2
		}
	case s.identifier(j):
		// A conversion operator (operator bool): its declaration declares no
		// symbol, but its body must still be stepped over.
		for j < len(s.tokens) && !s.is(j, "(") && !s.is(j, ";") && !s.is(j, "{") && !s.is(j, "}") {
			if s.is(j, "<") {
				if end := s.angle(j); end > 0 {
					j = end
					continue
				}
			}
			j++
		}
		if !s.is(j, "(") {
			s.position = j
			return false
		}
		s.position = j
		return s.afterParameters(owner, "", 0, j, false)
	default:
		for s.at(j).kind == tPunctuation && s.tokens[j].text != "(" && j < keyword+4 {
			operator.WriteString(s.tokens[j].text)
			j++
		}
	}
	if !s.is(j, "(") || operator.Len() == 0 {
		s.position = j
		return false
	}
	// The name: a qualifier before the keyword (Foo<T>::operator==).
	prefix, line, first := "", s.tokens[keyword].line, keyword
	if s.is(keyword-1, "::") {
		if q, l, f, ok := s.nameBefore(keyword-1, start); ok {
			prefix, line, first = q+"::", l, f
		}
	}
	s.position = j
	return s.afterParameters(owner, prefix+"operator"+operator.String(), line, j, first == start)
}

// nameBefore reads the declarator name ending right before the '(' at open:
// `name`, `~name`, `ns::Class<T>::name`, or a parenthesized `(*name)`.
func (s *scanner) nameBefore(open, start int) (string, int, int, bool) {
	j := open - 1
	if j < start {
		return "", 0, 0, false
	}
	t := s.tokens[j]
	if t.kind == tPunctuation && t.text == ")" {
		// (*name)(params): a pointer to a function, named as written, as a parser
		// names it; NAME(x)(params): a name a macro makes.
		k := j - 1
		if k-2 >= start && s.identifier(k) && (s.is(k-1, "*") || s.is(k-1, "&")) && s.is(k-2, "(") {
			return "(" + s.tokens[k-1].text + s.tokens[k].text + ")", s.tokens[k].line, k - 2, true
		}
		for k = j - 1; k > start && !s.is(k, "("); k-- {
			if !s.identifier(k) && !s.is(k, ",") {
				return "", 0, 0, false
			}
		}
		if k-1 >= start && s.identifier(k-1) && upper.MatchString(s.tokens[k-1].text) {
			var b strings.Builder
			for i := k - 1; i < open; i++ {
				b.WriteString(s.tokens[i].text)
			}
			return b.String(), s.tokens[k-1].line, k - 1, true
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
			if k < start || k <= j-256 || !s.identifier(k-1) {
				return "", 0, 0, false
			}
			j = k - 1
		case s.identifier(j):
		default:
			return "", 0, 0, false
		}
		if w := s.tokens[j].text; notDeclarator[w] && (s.cplus || !cplusOnly[w]) {
			return "", 0, 0, false
		}
		if s.is(j-1, "~") {
			j--
		}
		if s.is(j-1, "::") {
			j -= 2
			if j < start || !(s.identifier(j) || s.is(j, ">")) {
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
		b.WriteString(s.tokens[k].text)
		if s.identifier(k) {
			line = s.tokens[k].line
		}
	}
	return b.String(), line, j, true
}

// afterParameters reads what follows a function's parameter list (at open): trailing
// qualifiers, then a body, a constructor's initializers, `= default`, or the ';'
// of a declaration.
func (s *scanner) afterParameters(owner []string, name string, line, open int, untyped bool) bool {
	i := s.skipGroup(open)
	for n := 0; i < len(s.tokens) && n < maxTrailing; n++ {
		t := s.tokens[i]
		if t.kind == tIdentifier {
			switch {
			case t.text == "try":
				s.add(owner, name, line, false)
				i++
				if s.is(i, ":") {
					i = s.constructorInit(i + 1)
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
				s.position = i
				if !s.is(i-1, "}") {
					s.position = s.skipTo(i)
				}
				return true
			case s.is(i+1, "("):
				i = s.skipGroup(i + 1) // noexcept(x), throw(), ABSL_LOCKS_EXCLUDED(mu)
			default:
				i++ // const, override, final, a macro
			}
			continue
		}
		if t.kind != tPunctuation {
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
			s.position = s.skipBody(i)
			return true
		case ":":
			s.add(owner, name, line, false)
			i = s.constructorInit(i + 1)
			if s.is(i, "{") {
				i = s.skipBody(i)
			}
			s.position = i
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
			s.position = s.skipTo(i) - 1
			return true
		case ";", ",", "}":
			if untyped && len(owner) == 0 && !strings.Contains(name, "::") {
				return false // f(x); at file scope: a macro call
			}
			s.add(owner, name, line, true)
			s.position = i
			if t.text == "," {
				s.position++
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
	for i < len(s.tokens) {
		t := s.tokens[i]
		if t.kind == tPunctuation {
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

// constructorInit steps over a constructor's member initializers to its body.
func (s *scanner) constructorInit(i int) int {
	for i < len(s.tokens) {
		t := s.tokens[i]
		if t.kind == tPunctuation {
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
func (s *scanner) add(owner []string, text string, line int, declaration bool) {
	name := qualified(text)
	if name == "" || macroCall(name) {
		return
	}
	name = withOwner(owner, name)
	kind := "func"
	if strings.Contains(name, ".") {
		kind = "method"
	}
	s.definitions = append(s.definitions, definition{name, kind, line, declaration})
}

// macros records the #defines outside function bodies. A valueless #define of
// the name an enclosing #ifdef or #ifndef tests is a header guard, not a symbol.
func (s *scanner) macros(directories []directive) {
	// The blocks are disjoint (a body is stepped over whole) and recorded in order.
	inBlock := func(at int) bool {
		k := sort.Search(len(s.blocks), func(i int) bool { return s.blocks[i][0] >= at }) - 1
		return k >= 0 && at < s.blocks[k][1]
	}
	var tested []string
	for _, d := range directories {
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
			s.definitions = append(s.definitions, definition{name, "macro", d.line, false})
		}
	}
}

func firstWord(s string) string {
	i := 0
	for i < len(s) && identifierPart(s[i]) {
		i++
	}
	if i == 0 || !identifierStart(s[0]) {
		return ""
	}
	return s[:i]
}
