package zig

import (
	"path"
	"strings"
)

// A value of build code, as far as the build graph needs it.
type value struct {
	kind       byte   // 0 unknown, 'f' a file, 'm' a module, 'd' a dependency, 'c' a compile step, 'g' generated
	path       string // 'f': the file; 'm', 'c': the root source file ("" when generated)
	dependency string // 'd': the build.zig.zon dependency
	module     string // 'd': the module asked of it (dep.module("x")); "" for the dependency itself
}

// croot is the root source file of a compilation: @import("root") in the files it
// reaches names it.
type croot struct {
	file string
	test bool
}

// buildFacts is what the build code of one package wires up.
type buildFacts struct {
	wires   map[string][]value // import name -> what it was wired to, in source order
	exports map[string]string  // b.addModule name -> root source file
	roots   []croot
}

// maxScan bounds how far the evaluator looks for the end of something (a function's
// body, a declaration's =, a field beside another), and maxWires the values kept
// per import name, so a malformed file cannot make the evaluation quadratic.
const (
	maxScan  = 512
	maxWires = 64
)

func newFacts() *buildFacts {
	return &buildFacts{wires: map[string][]value{}, exports: map[string]string{}}
}

// compileSteps are the Build functions whose options name a compilation's root.
var compileSteps = map[string]bool{
	"addExecutable": true, "addTest": true, "addLibrary": true, "addStaticLibrary": true,
	"addSharedLibrary": true, "addObject": true,
}

// wiring are the calls that wire an import name; chained the calls whose value the
// evaluator knows. The arguments of other calls are not split.
var (
	wiring  = map[string]bool{"addImport": true, "addAnonymousImport": true, "addOptions": true, "addOptionsModule": true, "addModule": true}
	chained = map[string]bool{"dependency": true, "lazyDependency": true, "module": true, "path": true, "createModule": true, "addModule": true}
)

// evaluator follows a build file's values far enough to know which module an import
// name is wired to: modules made from a root source file, a build.zig.zon
// dependency's modules, options modules (generated), through variables, captures,
// struct fields and function returns. It never runs anything: a value it cannot
// follow is unknown.
type evaluator struct {
	*source
	root      string // the build root the file's b.path() is relative to
	open      []int  // innermost enclosing bracket of each token
	functions map[string]value
	binds     map[string]value
	facts     *buildFacts
	record    bool
	depth     int
}

// evalBuild reads one file of a package's build code (build.zig, or a file it
// imports holding build logic) into facts.
//
// Implements: REQ-ZIG-006
func evalBuild(content []byte, root string, facts *buildFacts) {
	tokens := lex(content)
	e := &evaluator{source: &source{tokens: tokens, m: match(tokens)}, root: root, functions: map[string]value{}, facts: facts}
	e.open = make([]int, len(tokens))
	var stack []int
	for i := range tokens {
		for len(stack) > 0 && e.m[stack[len(stack)-1]] >= 0 && e.m[stack[len(stack)-1]] <= i {
			stack = stack[:len(stack)-1]
		}
		e.open[i] = -1
		if len(stack) > 0 {
			e.open[i] = stack[len(stack)-1]
		}
		if e.m[i] > i {
			stack = append(stack, i)
		}
	}
	// Function returns first, so a call before the function's definition is known;
	// then the walk that records.
	e.walk()
	e.record = true
	e.walk()
}

type functionRange struct {
	name       string
	start, end int
}

// walk visits every token once, in source order, keeping bindings current.
func (e *evaluator) walk() {
	e.binds = map[string]value{}
	var functions []functionRange
	for i := 0; i < len(e.tokens); i++ {
		for len(functions) > 0 && i >= functions[len(functions)-1].end {
			functions = functions[:len(functions)-1]
		}
		t := e.tokens[i]
		switch t.kind {
		case tIdentifier:
		case tPunctuation:
			if t.text == "|" && i+2 < len(e.tokens) && e.tokens[i+1].kind == tIdentifier && e.punctuation(i+2, "|") && e.punctuation(i-1, ")") {
				// if (b.lazyDependency("x", .{})) |dep| - bind the capture.
				if o := e.m[i-1]; o > 0 && e.at(o-1, tIdentifier, "if") || o > 0 && e.at(o-1, tIdentifier, "while") {
					if v := e.eval(o+1, i-1); v.kind != 0 {
						e.binds[e.tokens[i+1].text] = v
					}
				}
			}
			continue
		default:
			continue
		}
		switch t.text {
		case "fn":
			if i+2 < len(e.tokens) && e.tokens[i+1].kind == tIdentifier && e.punctuation(i+2, "(") {
				if body := e.functionBody(i + 2); body > 0 {
					functions = append(functions, functionRange{e.tokens[i+1].text, body, e.m[body]})
				}
			}
		case "const", "var":
			if i+1 < len(e.tokens) && e.tokens[i+1].kind == tIdentifier {
				e.bind(e.tokens[i+1].text, i+2)
			}
		case "return":
			if len(functions) > 0 {
				if v := e.eval(i+1, len(e.tokens)); v.kind != 0 {
					e.functions[functions[len(functions)-1].name] = v
				}
			}
		default:
			switch {
			case e.punctuation(i-1, ".") && e.punctuation(i+1, "=") && e.at(i-2, tPunctuation, "{") || e.punctuation(i-1, ".") && e.punctuation(i+1, "=") && e.punctuation(i-2, ","):
				e.field(i)
			case e.punctuation(i+1, "=") && (i == 0 || e.punctuation(i-1, ";") || e.punctuation(i-1, "{") || e.punctuation(i-1, "}")):
				e.bind(t.text, i+1) // x = ...;
			case e.punctuation(i-1, ".") && e.punctuation(i+1, "=") && e.at(i-2, tIdentifier, "self"):
				e.bindTo("."+t.text, i+2)
			case e.punctuation(i+1, "(") && e.record:
				e.call(i)
			}
		}
	}
}

// functionBody returns the index of a function's body brace, given its parameter list.
func (e *evaluator) functionBody(parameters int) int {
	i := e.skip(parameters)
	for end := min(len(e.tokens), i+maxScan); i < end; {
		t := e.tokens[i]
		if t.kind == tPunctuation && t.text == ";" {
			return -1
		}
		if t.kind == tPunctuation && t.text == "{" {
			if e.typeBody(i) {
				i = e.skip(i)
				continue
			}
			if e.m[i] > i {
				return i
			}
			return -1
		}
		i = e.skip(i)
	}
	return -1
}

// bind evaluates the declaration or assignment whose type or = follows at i.
func (e *evaluator) bind(name string, i int) {
	end := min(len(e.tokens), i+maxScan)
	equalsAt := e.until(i, end, "=", ";", "}", ")")
	if !e.punctuation(equalsAt, "=") {
		return
	}
	e.bindTo(name, equalsAt+1)
}

// bindTo binds name to the value from i. The value's chain ends by itself, so it is
// evaluated without finding the end of the statement first.
func (e *evaluator) bindTo(name string, i int) {
	if v := e.eval(i, len(e.tokens)); v.kind != 0 {
		e.binds[name] = v
	} else {
		delete(e.binds, name)
	}
}

// field handles .name = value inside a struct literal: .{ .name = "x", .module = m
// } in an .imports list (or 0.11's .dependencies) wires an import; any other field
// binds ".name" so deps.name reads it back.
func (e *evaluator) field(i int) {
	name := e.tokens[i].text
	group := e.open[i]
	if group < 0 || e.m[group] < group {
		return // not inside a closed struct literal
	}
	end := e.m[group]
	if name == "name" && e.isString(i+2) && (e.punctuation(i+3, ",") || e.punctuation(i+3, "}")) {
		if e.record {
			// .module = m beside it, before or after, in the same literal.
			for j := group + 1; j < end && j < group+maxScan; j = e.skip(j) {
				if e.at(j, tIdentifier, "module") && e.punctuation(j-1, ".") && e.punctuation(j+1, "=") {
					e.wire(e.tokens[i+2].text, e.eval(j+2, end))
					break
				}
			}
		}
		return
	}
	if v := e.eval(i+2, end); v.kind != 0 {
		e.binds["."+name] = v
	}
}

func (e *evaluator) wire(name string, v value) {
	if e.record && name != "" && len(e.facts.wires[name]) < maxWires {
		e.facts.wires[name] = append(e.facts.wires[name], v)
	}
}

// arguments returns the argument ranges of the call whose parenthesis is at p.
func (e *evaluator) arguments(p int) [][2]int {
	end := e.m[p]
	if end < 0 {
		return nil
	}
	var out [][2]int
	for i := p + 1; i < end; {
		j := e.until(i, end, ",")
		if j > i {
			out = append(out, [2]int{i, j})
		}
		i = j + 1
	}
	return out
}

// call records what a call at i wires: addImport, addAnonymousImport, addOptions,
// addModule (b.addModule exports, 0.11's step.addModule wires) and the roots of
// compile steps.
func (e *evaluator) call(i int) {
	name := e.tokens[i].text
	if !wiring[name] && !compileSteps[name] {
		return
	}
	arguments := e.arguments(i + 1)
	named := len(arguments) >= 1 && e.isString(arguments[0][0]) && arguments[0][1] == arguments[0][0]+1
	switch {
	case name == "addImport" && named && len(arguments) >= 2:
		e.wire(e.tokens[arguments[0][0]].text, e.eval(arguments[1][0], arguments[1][1]))
	case name == "addAnonymousImport" && named && len(arguments) >= 2:
		e.wire(e.tokens[arguments[0][0]].text, e.module(arguments[1][0], arguments[1][1]))
	case (name == "addOptions" || name == "addOptionsModule") && named:
		e.wire(e.tokens[arguments[0][0]].text, value{kind: 'g'})
	case name == "addModule" && named && len(arguments) >= 2:
		if e.punctuation(arguments[1][0], ".") && e.punctuation(arguments[1][0]+1, "{") {
			if v := e.module(arguments[1][0], arguments[1][1]); v.path != "" {
				if _, ok := e.facts.exports[e.tokens[arguments[0][0]].text]; !ok {
					e.facts.exports[e.tokens[arguments[0][0]].text] = v.path
				}
			}
		} else {
			e.wire(e.tokens[arguments[0][0]].text, e.eval(arguments[1][0], arguments[1][1]))
		}
	case compileSteps[name] && len(arguments) >= 1:
		if v := e.compile(arguments[0][0], arguments[0][1]); v.path != "" {
			e.facts.roots = append(e.facts.roots, croot{file: v.path, test: name == "addTest"})
		}
	}
}

// optionField finds .name = value at the top level of the struct literal in [i, end)
// and returns the value's range.
func (e *evaluator) optionField(i, end int, names ...string) (int, int, bool) {
	for ; i < end; i++ {
		if !e.punctuation(i, "{") || e.m[i] < 0 {
			continue
		}
		close := e.m[i]
		for j := i + 1; j < close; j = e.skip(j) {
			if e.punctuation(j, ".") && j+2 < close && e.tokens[j+1].kind == tIdentifier && e.punctuation(j+2, "=") {
				for _, n := range names {
					if e.tokens[j+1].text == n {
						return j + 3, e.until(j+3, close, ","), true
					}
				}
			}
		}
		return 0, 0, false
	}
	return 0, 0, false
}

// module is the module described by options in [i, end): its root source file, or
// generated without one.
func (e *evaluator) module(i, end int) value {
	if a, b, ok := e.optionField(i, end, "root_source_file", "source_file"); ok {
		if v := e.eval(a, b); v.kind == 'f' {
			return value{kind: 'm', path: v.path}
		}
		return value{kind: 'm'} // a generated file
	}
	return value{kind: 'm'}
}

// compile is the compile step described by options in [i, end), with its root
// module's source file.
func (e *evaluator) compile(i, end int) value {
	if a, b, ok := e.optionField(i, end, "root_module"); ok {
		v := e.eval(a, b)
		return value{kind: 'c', path: v.path}
	}
	if a, b, ok := e.optionField(i, end, "root_source_file"); ok {
		if v := e.eval(a, b); v.kind == 'f' {
			return value{kind: 'c', path: v.path}
		}
	}
	return value{kind: 'c'}
}

// file is a path of the build root.
func (e *evaluator) file(p string) value {
	if p == "" || strings.HasPrefix(p, "/") {
		return value{}
	}
	return value{kind: 'f', path: path.Clean(path.Join(e.root, p))}
}

// eval evaluates the expression in [i, end): a postfix chain from an identifier, a
// call, a struct literal or a parenthesized expression.
func (e *evaluator) eval(i, end int) value {
	if e.depth > 16 {
		return value{}
	}
	e.depth++
	defer func() { e.depth-- }()
	for i < end && e.tokens[i].kind == tPunctuation && (e.tokens[i].text == "&" || e.tokens[i].text == "(" && e.m[i] == end-1) {
		if e.tokens[i].text == "(" {
			end--
		}
		i++
	}
	for i < end && e.at(i, tIdentifier, "try") {
		i++
	}
	if i >= end {
		return value{}
	}
	var current value
	head := ""
	t := e.tokens[i]
	switch {
	case t.kind == tIdentifier && e.punctuation(i+1, "(") && e.m[i+1] > 0:
		current = e.functions[t.text]
		i = e.m[i+1] + 1
	case t.kind == tIdentifier:
		head = t.text
		current = e.binds[t.text]
		i++
	case e.punctuation(i, ".") && e.punctuation(i+1, "{"):
		if a, b, ok := e.optionField(i, end, "path"); ok && e.isString(a) && b == a+1 {
			return e.file(e.tokens[a].text)
		}
		if a, b, ok := e.optionField(i, end, "src_path"); ok {
			if c, d, ok := e.optionField(a, b, "sub_path"); ok && e.isString(c) && d == c+1 {
				return e.file(e.tokens[c].text)
			}
		}
		return value{}
	default:
		return value{}
	}
	for n := 0; i < end; n++ {
		if e.punctuation(i, ".?") {
			i++
			continue
		}
		if !e.punctuation(i, ".") || i+1 >= end || e.tokens[i+1].kind != tIdentifier {
			break
		}
		name := e.tokens[i+1].text
		if !e.punctuation(i+2, "(") || e.m[i+2] < 0 {
			// A field: a compile step's root module, or deps.x bound by .x = ...
			switch {
			case name == "root_module" && current.kind == 'c':
				current = value{kind: 'm', path: current.path}
			case current.kind == 0:
				current = e.binds["."+name]
			default:
				current = value{}
			}
			i += 2
			continue
		}
		p := i + 2
		var arguments [][2]int
		if chained[name] || compileSteps[name] {
			arguments = e.arguments(p)
		}
		literal := ""
		if len(arguments) > 0 && e.isString(arguments[0][0]) && arguments[0][1] == arguments[0][0]+1 {
			literal = e.tokens[arguments[0][0]].text
		}
		switch {
		case (name == "dependency" || name == "lazyDependency") && literal != "":
			current = value{kind: 'd', dependency: literal}
		case name == "module" && current.kind == 'd' && current.module == "" && literal != "":
			current.module = literal
		case name == "path" && n == 0 && (head == "b" || head == "builder") && literal != "" && arguments[0][1] == arguments[0][0]+1:
			current = e.file(literal)
		case name == "createModule" || name == "addModule" && len(arguments) >= 2:
			if len(arguments) == 0 {
				current = value{kind: 'g'} // options.createModule()
				break
			}
			a := arguments[len(arguments)-1]
			current = e.module(a[0], a[1])
			if current.path == "" {
				current = value{kind: 'g'}
			}
		case name == "addOptions":
			current = value{kind: 'g'}
		case compileSteps[name] && len(arguments) >= 1:
			current = e.compile(arguments[0][0], arguments[0][1])
		default:
			current = value{}
		}
		i = e.m[p] + 1
	}
	return current
}
