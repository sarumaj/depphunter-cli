package zig

import (
	"path"
	"strings"
)

// A value of build code, as far as the build graph needs it.
type val struct {
	kind byte   // 0 unknown, 'f' a file, 'm' a module, 'd' a dependency, 'c' a compile step, 'g' generated
	path string // 'f': the file; 'm', 'c': the root source file ("" when generated)
	dep  string // 'd': the build.zig.zon dependency
	mod  string // 'd': the module asked of it (dep.module("x")); "" for the dependency itself
}

// croot is the root source file of a compilation: @import("root") in the files it
// reaches names it.
type croot struct {
	file string
	test bool
}

// buildFacts is what the build code of one package wires up.
type buildFacts struct {
	wires   map[string][]val  // import name -> what it was wired to, in source order
	exports map[string]string // b.addModule name -> root source file
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
	return &buildFacts{wires: map[string][]val{}, exports: map[string]string{}}
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
	root   string // the build root the file's b.path() is relative to
	open   []int  // innermost enclosing bracket of each token
	fns    map[string]val
	binds  map[string]val
	facts  *buildFacts
	record bool
	depth  int
}

// evalBuild reads one file of a package's build code (build.zig, or a file it
// imports holding build logic) into facts.
//
// Implements: REQ-ZIG-006
func evalBuild(src []byte, root string, facts *buildFacts) {
	tokens := lex(src)
	e := &evaluator{source: &source{tokens: tokens, m: match(tokens)}, root: root, fns: map[string]val{}, facts: facts}
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

type fnRange struct {
	name       string
	start, end int
}

// walk visits every token once, in source order, keeping bindings current.
func (e *evaluator) walk() {
	e.binds = map[string]val{}
	var fns []fnRange
	for i := 0; i < len(e.tokens); i++ {
		for len(fns) > 0 && i >= fns[len(fns)-1].end {
			fns = fns[:len(fns)-1]
		}
		t := e.tokens[i]
		switch t.kind {
		case tIdent:
		case tPunct:
			if t.text == "|" && i+2 < len(e.tokens) && e.tokens[i+1].kind == tIdent && e.punct(i+2, "|") && e.punct(i-1, ")") {
				// if (b.lazyDependency("x", .{})) |dep| - bind the capture.
				if o := e.m[i-1]; o > 0 && e.at(o-1, tIdent, "if") || o > 0 && e.at(o-1, tIdent, "while") {
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
			if i+2 < len(e.tokens) && e.tokens[i+1].kind == tIdent && e.punct(i+2, "(") {
				if body := e.fnBody(i + 2); body > 0 {
					fns = append(fns, fnRange{e.tokens[i+1].text, body, e.m[body]})
				}
			}
		case "const", "var":
			if i+1 < len(e.tokens) && e.tokens[i+1].kind == tIdent {
				e.bind(e.tokens[i+1].text, i+2)
			}
		case "return":
			if len(fns) > 0 {
				if v := e.eval(i+1, len(e.tokens)); v.kind != 0 {
					e.fns[fns[len(fns)-1].name] = v
				}
			}
		default:
			switch {
			case e.punct(i-1, ".") && e.punct(i+1, "=") && e.at(i-2, tPunct, "{") || e.punct(i-1, ".") && e.punct(i+1, "=") && e.punct(i-2, ","):
				e.field(i)
			case e.punct(i+1, "=") && (i == 0 || e.punct(i-1, ";") || e.punct(i-1, "{") || e.punct(i-1, "}")):
				e.bind(t.text, i+1) // x = ...;
			case e.punct(i-1, ".") && e.punct(i+1, "=") && e.at(i-2, tIdent, "self"):
				e.bindTo("."+t.text, i+2)
			case e.punct(i+1, "(") && e.record:
				e.call(i)
			}
		}
	}
}

// fnBody returns the index of a function's body brace, given its parameter list.
func (e *evaluator) fnBody(params int) int {
	i := e.skip(params)
	for end := min(len(e.tokens), i+maxScan); i < end; {
		t := e.tokens[i]
		if t.kind == tPunct && t.text == ";" {
			return -1
		}
		if t.kind == tPunct && t.text == "{" {
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
	eq := e.until(i, end, "=", ";", "}", ")")
	if !e.punct(eq, "=") {
		return
	}
	e.bindTo(name, eq+1)
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
	grp := e.open[i]
	if grp < 0 || e.m[grp] < grp {
		return // not inside a closed struct literal
	}
	end := e.m[grp]
	if name == "name" && e.str(i+2) && (e.punct(i+3, ",") || e.punct(i+3, "}")) {
		if e.record {
			// .module = m beside it, before or after, in the same literal.
			for j := grp + 1; j < end && j < grp+maxScan; j = e.skip(j) {
				if e.at(j, tIdent, "module") && e.punct(j-1, ".") && e.punct(j+1, "=") {
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

func (e *evaluator) wire(name string, v val) {
	if e.record && name != "" && len(e.facts.wires[name]) < maxWires {
		e.facts.wires[name] = append(e.facts.wires[name], v)
	}
}

// args returns the argument ranges of the call whose parenthesis is at p.
func (e *evaluator) args(p int) [][2]int {
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
	args := e.args(i + 1)
	named := len(args) >= 1 && e.str(args[0][0]) && args[0][1] == args[0][0]+1
	switch {
	case name == "addImport" && named && len(args) >= 2:
		e.wire(e.tokens[args[0][0]].text, e.eval(args[1][0], args[1][1]))
	case name == "addAnonymousImport" && named && len(args) >= 2:
		e.wire(e.tokens[args[0][0]].text, e.module(args[1][0], args[1][1]))
	case (name == "addOptions" || name == "addOptionsModule") && named:
		e.wire(e.tokens[args[0][0]].text, val{kind: 'g'})
	case name == "addModule" && named && len(args) >= 2:
		if e.punct(args[1][0], ".") && e.punct(args[1][0]+1, "{") {
			if v := e.module(args[1][0], args[1][1]); v.path != "" {
				if _, ok := e.facts.exports[e.tokens[args[0][0]].text]; !ok {
					e.facts.exports[e.tokens[args[0][0]].text] = v.path
				}
			}
		} else {
			e.wire(e.tokens[args[0][0]].text, e.eval(args[1][0], args[1][1]))
		}
	case compileSteps[name] && len(args) >= 1:
		if v := e.compile(args[0][0], args[0][1]); v.path != "" {
			e.facts.roots = append(e.facts.roots, croot{file: v.path, test: name == "addTest"})
		}
	}
}

// optField finds .name = value at the top level of the struct literal in [i, end)
// and returns the value's range.
func (e *evaluator) optField(i, end int, names ...string) (int, int, bool) {
	for ; i < end; i++ {
		if !e.punct(i, "{") || e.m[i] < 0 {
			continue
		}
		close := e.m[i]
		for j := i + 1; j < close; j = e.skip(j) {
			if e.punct(j, ".") && j+2 < close && e.tokens[j+1].kind == tIdent && e.punct(j+2, "=") {
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
func (e *evaluator) module(i, end int) val {
	if a, b, ok := e.optField(i, end, "root_source_file", "source_file"); ok {
		if v := e.eval(a, b); v.kind == 'f' {
			return val{kind: 'm', path: v.path}
		}
		return val{kind: 'm'} // a generated file
	}
	return val{kind: 'm'}
}

// compile is the compile step described by options in [i, end), with its root
// module's source file.
func (e *evaluator) compile(i, end int) val {
	if a, b, ok := e.optField(i, end, "root_module"); ok {
		v := e.eval(a, b)
		return val{kind: 'c', path: v.path}
	}
	if a, b, ok := e.optField(i, end, "root_source_file"); ok {
		if v := e.eval(a, b); v.kind == 'f' {
			return val{kind: 'c', path: v.path}
		}
	}
	return val{kind: 'c'}
}

// file is a path of the build root.
func (e *evaluator) file(p string) val {
	if p == "" || strings.HasPrefix(p, "/") {
		return val{}
	}
	return val{kind: 'f', path: path.Clean(path.Join(e.root, p))}
}

// eval evaluates the expression in [i, end): a postfix chain from an identifier, a
// call, a struct literal or a parenthesized expression.
func (e *evaluator) eval(i, end int) val {
	if e.depth > 16 {
		return val{}
	}
	e.depth++
	defer func() { e.depth-- }()
	for i < end && e.tokens[i].kind == tPunct && (e.tokens[i].text == "&" || e.tokens[i].text == "(" && e.m[i] == end-1) {
		if e.tokens[i].text == "(" {
			end--
		}
		i++
	}
	for i < end && e.at(i, tIdent, "try") {
		i++
	}
	if i >= end {
		return val{}
	}
	var cur val
	head := ""
	t := e.tokens[i]
	switch {
	case t.kind == tIdent && e.punct(i+1, "(") && e.m[i+1] > 0:
		cur = e.fns[t.text]
		i = e.m[i+1] + 1
	case t.kind == tIdent:
		head = t.text
		cur = e.binds[t.text]
		i++
	case e.punct(i, ".") && e.punct(i+1, "{"):
		if a, b, ok := e.optField(i, end, "path"); ok && e.str(a) && b == a+1 {
			return e.file(e.tokens[a].text)
		}
		if a, b, ok := e.optField(i, end, "src_path"); ok {
			if c, d, ok := e.optField(a, b, "sub_path"); ok && e.str(c) && d == c+1 {
				return e.file(e.tokens[c].text)
			}
		}
		return val{}
	default:
		return val{}
	}
	for n := 0; i < end; n++ {
		if e.punct(i, ".?") {
			i++
			continue
		}
		if !e.punct(i, ".") || i+1 >= end || e.tokens[i+1].kind != tIdent {
			break
		}
		name := e.tokens[i+1].text
		if !e.punct(i+2, "(") || e.m[i+2] < 0 {
			// A field: a compile step's root module, or deps.x bound by .x = ...
			switch {
			case name == "root_module" && cur.kind == 'c':
				cur = val{kind: 'm', path: cur.path}
			case cur.kind == 0:
				cur = e.binds["."+name]
			default:
				cur = val{}
			}
			i += 2
			continue
		}
		p := i + 2
		var args [][2]int
		if chained[name] || compileSteps[name] {
			args = e.args(p)
		}
		str := ""
		if len(args) > 0 && e.str(args[0][0]) && args[0][1] == args[0][0]+1 {
			str = e.tokens[args[0][0]].text
		}
		switch {
		case (name == "dependency" || name == "lazyDependency") && str != "":
			cur = val{kind: 'd', dep: str}
		case name == "module" && cur.kind == 'd' && cur.mod == "" && str != "":
			cur.mod = str
		case name == "path" && n == 0 && (head == "b" || head == "builder") && str != "" && args[0][1] == args[0][0]+1:
			cur = e.file(str)
		case name == "createModule" || name == "addModule" && len(args) >= 2:
			if len(args) == 0 {
				cur = val{kind: 'g'} // options.createModule()
				break
			}
			a := args[len(args)-1]
			cur = e.module(a[0], a[1])
			if cur.path == "" {
				cur = val{kind: 'g'}
			}
		case name == "addOptions":
			cur = val{kind: 'g'}
		case compileSteps[name] && len(args) >= 1:
			cur = e.compile(args[0][0], args[0][1])
		default:
			cur = val{}
		}
		i = e.m[p] + 1
	}
	return cur
}
