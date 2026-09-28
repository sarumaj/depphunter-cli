package perl

import "strings"

// Paths in use lib, require and do are often computed from where the file is:
// "$FindBin::Bin/../lib", File::Spec->catdir(dirname(__FILE__), 'lib'),
// curfile->sibling('lib') (Mojo::File), path(__FILE__)->parent->child('lib')
// (Path::Tiny). evalPath evaluates that small language; a path relative to the file
// starts with selfMarker ("\x01" is the file itself, "\x01/.." its directory), and
// the resolver cleans it. Anything else it cannot know (another variable, a call it
// does not model) makes the path unknown.

// evalList evaluates a comma-separated list of paths (a qw list gives several).
//
// Implements: REQ-PERL-004
func evalList(arguments []token) []string {
	var out []string
	for _, item := range splitArguments(arguments) {
		if len(item) == 1 && item[0].kind == tQW {
			out = append(out, item[0].words...)
			continue
		}
		if p, ok := evalPath(item); ok && p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitArguments splits tokens on the commas (and fat commas) at bracket depth 0; an
// argument list in parentheses is unwrapped first.
func splitArguments(arguments []token) [][]token {
	if len(arguments) >= 2 && arguments[0].kind == tPunctuation && arguments[0].text == "(" && arguments[len(arguments)-1].kind == tPunctuation && arguments[len(arguments)-1].text == ")" {
		arguments = arguments[1 : len(arguments)-1]
	}
	var out [][]token
	depth, from := 0, 0
	for k, t := range arguments {
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case ",", "=>":
			if depth == 0 {
				out = append(out, arguments[from:k])
				from = k + 1
			}
		}
	}
	if from < len(arguments) {
		out = append(out, arguments[from:])
	}
	return out
}

// findBin are the variables holding the running script's directory.
var findBin = []string{"$FindBin::RealBin", "$FindBin::Bin", "${FindBin::RealBin}", "${FindBin::Bin}", "$RealBin", "$Bin"}

// evalPath evaluates one path expression: strings (interpolating the FindBin
// variables), `.` concatenation, __FILE__, dirname(), File::Spec's catdir and
// catfile, abs_path/realpath/rel2abs (which change nothing here), and
// Mojo::File/Path::Tiny method chains.
//
// Implements: REQ-PERL-004
func evalPath(tokens []token) (string, bool) {
	e := &evaluator{tokens: tokens}
	v, ok := e.concat()
	if !ok || e.i != len(tokens) {
		return "", false
	}
	return v, true
}

type evaluator struct {
	tokens []token
	i      int
}

func (e *evaluator) peek() token {
	if e.i < len(e.tokens) {
		return e.tokens[e.i]
	}
	return token{kind: -1}
}

func (e *evaluator) punctuation(p string) bool {
	t := e.peek()
	if t.kind == tPunctuation && t.text == p {
		e.i++
		return true
	}
	return false
}

func (e *evaluator) concat() (string, bool) {
	v, ok := e.postfix()
	for ok && e.punctuation(".") {
		var w string
		if w, ok = e.postfix(); ok {
			v += w
		}
	}
	return v, ok
}

// postfix is a primary followed by method calls: ->dirname, ->sibling('x').
func (e *evaluator) postfix() (string, bool) {
	v, ok := e.primary()
	for ok && e.punctuation("->") {
		m := e.peek()
		if m.kind != tWord {
			return "", false
		}
		e.i++
		var arguments []string
		if e.punctuation("(") {
			if arguments, ok = e.arguments(); !ok {
				return "", false
			}
		}
		switch m.text {
		case "dirname", "parent":
			v += "/.."
		case "sibling":
			v += "/.." + joinArguments(arguments)
		case "child":
			v += joinArguments(arguments)
		case "to_string", "stringify", "realpath", "absolute", "to_abs", "canonpath", "canonical":
		default:
			return "", false
		}
	}
	return v, ok
}

func joinArguments(arguments []string) string {
	var b strings.Builder
	for _, a := range arguments {
		b.WriteString("/" + a)
	}
	return b.String()
}

// arguments reads call arguments after "(" through ")".
func (e *evaluator) arguments() ([]string, bool) {
	var out []string
	if e.punctuation(")") {
		return nil, true
	}
	for {
		v, ok := e.concat()
		if !ok {
			return nil, false
		}
		out = append(out, v)
		if e.punctuation(")") {
			return out, true
		}
		if !e.punctuation(",") {
			return nil, false
		}
	}
}

func (e *evaluator) primary() (string, bool) {
	t := e.peek()
	switch t.kind {
	case tString:
		e.i++
		return interpolate(t)
	case tVariable:
		e.i++
		for _, v := range findBin {
			if t.text == v {
				return selfMarker + "/..", true
			}
		}
		return "", false
	case tPunctuation:
		if t.text == "(" {
			e.i++
			v, ok := e.concat()
			return v, ok && e.punctuation(")")
		}
		return "", false
	case tWord:
		e.i++
		name := t.text
		switch name {
		case "__FILE__":
			return selfMarker, true
		case "curfile":
			if e.punctuation("(") && !e.punctuation(")") {
				return "", false
			}
			return selfMarker, true
		case "File::Spec", "Mojo::File", "Path::Tiny", "Cwd":
			if !e.punctuation("->") {
				return "", false
			}
			m := e.peek()
			if m.kind != tWord {
				return "", false
			}
			e.i++
			name = m.text
		}
		if !e.punctuation("(") {
			return "", false
		}
		arguments, ok := e.arguments()
		if !ok {
			return "", false
		}
		switch name {
		case "catdir", "catfile", "File::Spec::Functions::catdir", "File::Spec::Functions::catfile":
			if len(arguments) == 0 {
				return "", false
			}
			return strings.Join(arguments, "/"), true
		case "dirname", "File::Basename::dirname":
			if len(arguments) != 1 {
				return "", false
			}
			return arguments[0] + "/..", true
		case "abs_path", "realpath", "rel2abs", "Cwd::abs_path", "Cwd::realpath", "path", "Path::Tiny::path", "new":
			if len(arguments) != 1 {
				return "", false
			}
			return arguments[0], true
		}
	}
	return "", false
}

// interpolate is a string's value with the FindBin variables replaced; any other
// interpolation makes it unknown.
func interpolate(t token) (string, bool) {
	s := t.text
	if !t.interpolate {
		return s, true
	}
	for _, v := range findBin {
		s = strings.ReplaceAll(s, v, selfMarker+"/..")
	}
	if strings.ContainsAny(s, "$@") {
		return "", false
	}
	return s, true
}
