package r

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindPackage         = "pkg"      // library(), require(), requireNamespace(), loadNamespace(), pacman
	kindNS              = "ns"       // pkg::fun, pkg:::fun
	kindRoxygen         = "roxygen"  // #' @import / @importFrom
	kindBox             = "box"      // box::use(pkg[...])
	kindBoxLocal        = "boxlocal" // box::use(./mod, app/logic/x)
	kindSource          = "source"   // source("x.R"), sys.source(), Rcpp::sourceCpp()
	kindSourceDirectory = "srcdir"   // targets::tar_source("R")
	kindInclude         = "include"  // #' @include utils.R (collation order)
	kindChild           = "child"    // knitr child documents, Quarto includes
	kindCall            = "call"     // a function the file calls, resolved within its package
	kindDependency      = "dep"      // DESCRIPTION dependency: "dep:Imports"
	kindNSFile          = "nsfile"   // NAMESPACE import() / importFrom()
)

// code is one run of tokens with its brackets matched: an R file, or the chunks of
// an R Markdown document separated by tSeparator.
type code struct {
	tokens []token
	match  []int // index of the matching bracket, -1 when unmatched
	depth  []int // brackets open before the token
}

func newCode(tokens []token) *code {
	c := &code{tokens: tokens, match: make([]int, len(tokens)), depth: make([]int, len(tokens))}
	var stack []int
	for i, t := range tokens {
		c.match[i] = -1
		if t.k == tSeparator {
			stack = stack[:0] // a chunk that leaves a bracket open does not spill over
		}
		if t.k == tOp {
			switch t.s {
			case ")", "]", "}":
				want := map[string]string{")": "(", "]": "[", "}": "{"}[t.s]
				// Close the nearest matching opener; openers left inside are unmatched.
				for j := len(stack) - 1; j >= 0; j-- {
					if tokens[stack[j]].s == want {
						c.match[stack[j]], c.match[i] = i, stack[j]
						stack = stack[:j]
						break
					}
				}
			}
		}
		c.depth[i] = len(stack)
		if t.k == tOp && (t.s == "(" || t.s == "[" || t.s == "{") {
			stack = append(stack, i)
		}
	}
	return c
}

func (c *code) is(i int, k int, s string) bool {
	return i >= 0 && i < len(c.tokens) && c.tokens[i].k == k && c.tokens[i].s == s
}

func (c *code) operator(i int, s string) bool { return c.is(i, tOp, s) }

// end is the index of the bracket closing the opener at i, or where the chunk ends.
func (c *code) end(i int) int {
	if c.match[i] >= 0 {
		return c.match[i]
	}
	for j := i + 1; j < len(c.tokens); j++ {
		if c.tokens[j].k == tSeparator {
			return j
		}
	}
	return len(c.tokens)
}

// argument is one argument of a call: its name when it has one, and its value's tokens.
type argument struct {
	name       string
	start, end int // value tokens [start, end)
}

// arguments splits the arguments of the call whose "(" is at i.
func (c *code) arguments(i int) []argument {
	end := c.end(i)
	var out []argument
	start := i + 1
	base := c.depth[i] + 1
	for j := i + 1; j <= end; j++ {
		if j == end || c.operator(j, ",") && c.depth[j] == base {
			a := argument{start: start, end: j}
			if j-start >= 2 && (c.tokens[start].k == tIdentifier || c.tokens[start].k == tString) && c.operator(start+1, "=") {
				a.name, a.start = c.tokens[start].s, start+2
			}
			if a.start < a.end || a.name != "" {
				out = append(out, a)
			}
			start = j + 1
		}
	}
	return out
}

// single is the one token an argument's value consists of.
func (c *code) single(a argument) (token, bool) {
	if a.end-a.start == 1 {
		return c.tokens[a.start], true
	}
	return token{}, false
}

// positional returns the n-th unnamed argument.
func positional(arguments []argument, n int) (argument, bool) {
	for _, a := range arguments {
		if a.name == "" {
			if n == 0 {
				return a, true
			}
			n--
		}
	}
	return argument{}, false
}

func named(arguments []argument, name string) (argument, bool) {
	for _, a := range arguments {
		if a.name == name {
			return a, true
		}
	}
	return argument{}, false
}

// callAt reports the function called at i ("(" follows) and the namespace it is
// qualified with, if any: base::library, box::use.
func (c *code) callAt(i int) (name, namespace string, ok bool) {
	t := c.tokens[i]
	if t.k != tIdentifier || !c.operator(i+1, "(") {
		return "", "", false
	}
	if c.operator(i-1, "$") || c.operator(i-1, "@") {
		return "", "", false
	}
	if (c.operator(i-1, "::") || c.operator(i-1, ":::")) && i >= 2 && c.tokens[i-2].k == tIdentifier {
		return t.s, c.tokens[i-2].s, true
	}
	return t.s, "", true
}

// statementStart reports whether token i begins a statement: the first token of a run, a
// line that does not continue an expression left open by an operator, or after ";".
func (c *code) statementStart(i int) bool {
	if i == 0 {
		return true
	}
	p := c.tokens[i-1]
	if p.k == tSeparator || p.k == tOp && p.s == ";" {
		return true
	}
	if !c.tokens[i].lineStart {
		return false
	}
	return p.k != tOp || p.s == ")" || p.s == "]" || p.s == "}"
}

// keywords are the reserved words that are followed by "(" without being calls.
var keywords = map[string]bool{
	"function": true, "if": true, "for": true, "while": true, "repeat": true, "return": true,
	"switch": true, "else": true, "in": true, "next": true, "break": true,
}

// extractCode reads the definitions and imports of R code.
//
// Implements: REQ-R-002, REQ-R-003
func extractCode(c *code, comments []comment) *lang.Extraction {
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	defined := definitions(c, &symbols)
	extraction.Symbols = symbols.List()
	seen := map[string]bool{}
	add := func(kind, spec, module string, line int) {
		key := kind + " " + module
		if module == "" || seen[key] {
			return
		}
		seen[key] = true
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	roxygen(comments, add)
	calls := map[string]int{}
	var callOrder []string
	call := func(name string, line int) {
		if _, ok := calls[name]; !ok && !defined[name] && !keywords[name] && !common[name] {
			calls[name] = line
			callOrder = append(callOrder, name)
		}
	}
	for i, t := range c.tokens {
		if t.k != tIdentifier {
			continue
		}
		// pkg::name and pkg:::name name a package whatever follows.
		if (c.operator(i+1, "::") || c.operator(i+1, ":::")) && !c.operator(i-1, "$") && !c.operator(i-1, "@") && validPackage(t.s) {
			add(kindNS, t.s+c.tokens[i+1].s, t.s, t.line)
		}
		// Name$new() creates an R5/R6 object: a use of the class generator.
		if c.operator(i+1, "$") && c.is(i+2, tIdentifier, "new") && c.operator(i+3, "(") && !c.operator(i-1, "$") && !c.operator(i-1, "::") {
			call(t.s, t.line)
		}
		name, namespace, ok := c.callAt(i)
		if !ok {
			continue
		}
		if namespace == "" {
			call(name, t.line)
		}
		if !readArguments[name] && !applyFamily[name] {
			continue
		}
		arguments := c.arguments(i + 1)
		switch {
		case (namespace == "" || namespace == "base") && (name == "library" || name == "require"):
			a, ok := named(arguments, "package")
			if !ok {
				a, ok = positional(arguments, 0)
			}
			v, one := c.single(a)
			if !ok || !one {
				break
			}
			if v.k == tIdentifier {
				// character.only = TRUE: the name is in a variable.
				if co, ok := named(arguments, "character.only"); ok {
					if f, one := c.single(co); !one || f.s != "FALSE" && f.s != "F" {
						break
					}
				}
			}
			if (v.k == tIdentifier || v.k == tString) && validPackage(v.s) {
				add(kindPackage, name+"("+v.s+")", v.s, t.line)
			}
		case (namespace == "" || namespace == "base") && (name == "requireNamespace" || name == "loadNamespace" || name == "attachNamespace"):
			a, ok := named(arguments, "package")
			if !ok {
				a, ok = positional(arguments, 0)
			}
			if v, one := c.single(a); ok && one && v.k == tString && validPackage(v.s) {
				add(kindPackage, name+"(\""+v.s+"\")", v.s, t.line)
			}
		case (namespace == "" || namespace == "pacman") && name == "p_load":
			for _, a := range arguments {
				if v, one := c.single(a); a.name == "" && one && (v.k == tIdentifier || v.k == tString) && validPackage(v.s) {
					add(kindPackage, "p_load("+v.s+")", v.s, t.line)
				}
				if a.name == "char" {
					for _, s := range c.strings(a) {
						if validPackage(s) {
							add(kindPackage, "p_load("+s+")", s, t.line)
						}
					}
				}
			}
		case namespace == "box" && name == "use":
			for _, a := range arguments {
				module, local := c.boxModule(a)
				switch {
				case module == "":
				case local:
					if strings.HasPrefix(module, ".") && !strings.HasPrefix(module, "./") && !strings.HasPrefix(module, "../") {
						break // not a path: ".", "..x"
					}
					add(kindBoxLocal, "box::use("+module+")", module, c.tokens[a.start].line)
				case validPackage(module):
					add(kindBox, "box::use("+module+")", module, c.tokens[a.start].line)
				}
			}
		case (namespace == "" || namespace == "base") && (name == "source" || name == "sys.source"),
			(namespace == "" || namespace == "Rcpp") && name == "sourceCpp":
			a, ok := named(arguments, "file")
			if !ok {
				a, ok = positional(arguments, 0)
			}
			if !ok {
				break
			}
			if p, here, ok := c.evalPath(a.start, a.end); ok && p != "" {
				spec := name + "(\"" + p + "\")"
				if here {
					spec = name + "(here(\"" + p + "\"))"
				}
				add(kindSource, spec, p, t.line)
			}
		case (namespace == "" || namespace == "targets") && name == "tar_source":
			directory := "R"
			if a, ok := named(arguments, "files"); ok {
				directory, _, ok = c.evalPath(a.start, a.end)
				if !ok {
					break
				}
			} else if a, ok := positional(arguments, 0); ok {
				directory, _, ok = c.evalPath(a.start, a.end)
				if !ok {
					break
				}
			}
			add(kindSourceDirectory, "tar_source(\""+directory+"\")", directory, t.line)
		case (namespace == "" || namespace == "base") && name == "do.call",
			applyFamily[name] && (namespace == "" || namespace == "base" || namespace == "purrr"):
			// A function handed over by name is called as much as one called directly.
			for _, a := range arguments {
				if v, one := c.single(a); one && (v.k == tIdentifier || name == "do.call" && v.k == tString) && !c.operator(a.start-1, "$") {
					call(v.s, v.line)
				}
			}
		}
	}
	for _, name := range callOrder {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name + "()", Module: name, Name: kindCall, Line: calls[name]})
	}
	return extraction
}

// readArguments are the calls whose arguments name a package or a file.
var readArguments = map[string]bool{
	"library": true, "require": true, "requireNamespace": true, "loadNamespace": true,
	"attachNamespace": true, "p_load": true, "use": true, "source": true, "sys.source": true,
	"sourceCpp": true, "tar_source": true, "do.call": true,
}

// applyFamily are the functions that take a function to call as an argument.
var applyFamily = map[string]bool{
	"lapply": true, "sapply": true, "vapply": true, "mapply": true, "Map": true, "Reduce": true,
	"Filter": true, "Position": true, "Find": true, "apply": true, "tapply": true, "outer": true,
	"map": true, "map_chr": true, "map_lgl": true, "map_int": true, "map_dbl": true, "map_dfr": true,
	"map2": true, "pmap": true, "walk": true, "imap": true, "match.fun": true,
}

var packageName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9.]*[A-Za-z0-9]$`)

// validPackage reports whether s can be an R package name (letters, digits and dots,
// starting with a letter, at least two characters).
func validPackage(s string) bool { return packageName.MatchString(s) }

// strings collects the string literals of c("a", "b") or of a single "a".
func (c *code) strings(a argument) []string {
	if v, one := c.single(a); one && v.k == tString {
		return []string{v.s}
	}
	if a.end-a.start >= 3 && c.is(a.start, tIdentifier, "c") && c.operator(a.start+1, "(") {
		var out []string
		for _, b := range c.arguments(a.start + 1) {
			if v, one := c.single(b); one && v.k == tString {
				out = append(out, v.s)
			}
		}
		return out
	}
	return nil
}

// boxModule reads one argument of box::use: "alias = " is ignored, then a module
// path of names and slashes (./x, ../x, app/logic/x, pkg) up to an attach list
// "[...]". A path with a slash or a leading dot is a local module, anything else a
// package.
func (c *code) boxModule(a argument) (string, bool) {
	var b strings.Builder
	for j := a.start; j < a.end; j++ {
		t := c.tokens[j]
		if t.k == tOp && t.s == "[" {
			break
		}
		if t.k != tIdentifier && !(t.k == tOp && t.s == "/") {
			return "", false
		}
		b.WriteString(t.s)
	}
	module := b.String()
	return module, strings.Contains(module, "/") || strings.HasPrefix(module, ".")
}

// evalPath evaluates the few path expressions source() is usually given: a string,
// file.path(...) and paste0(...) of evaluable parts, and here::here(...) /
// here(...), which is relative to the project root rather than to the working
// directory. Anything else (a variable, a computed name) is not a path.
func (c *code) evalPath(start, end int) (p string, here bool, ok bool) {
	if end-start == 1 {
		if t := c.tokens[start]; t.k == tString {
			return t.s, false, true
		}
		return "", false, false
	}
	i := start
	namespace := ""
	if end-start > 3 && c.tokens[i].k == tIdentifier && c.operator(i+1, "::") {
		namespace = c.tokens[i].s
		i += 2
	}
	if c.tokens[i].k != tIdentifier || !c.operator(i+1, "(") || c.end(i+1) != end-1 {
		return "", false, false
	}
	function := c.tokens[i].s
	var parts []string
	for _, a := range c.arguments(i + 1) {
		if a.name != "" {
			if function == "file.path" && a.name == "fsep" {
				continue
			}
			return "", false, false
		}
		s, h, ok := c.evalPath(a.start, a.end)
		if !ok {
			return "", false, false
		}
		here = here || h
		parts = append(parts, s)
	}
	switch {
	case function == "here" && (namespace == "" || namespace == "here"):
		return strings.Join(parts, "/"), true, true
	case function == "file.path" && (namespace == "" || namespace == "base"):
		parts = slices.DeleteFunc(parts, func(s string) bool { return s == "" })
		return strings.Join(parts, "/"), here, true
	case function == "paste0" && (namespace == "" || namespace == "base"):
		return strings.Join(parts, ""), here, true
	}
	return "", false, false
}

// definitions records what the code defines at the top level - functions (f <-
// function, f = \(x), assign("f", function)), R5, R6 and S7 classes with their
// methods, S4 classes, generics and methods, and variables, a variable only where it
// is first assigned - and returns the names of functions and classes.
//
// Implements: REQ-R-003
func definitions(c *code, symbols *lang.SymbolSet) map[string]bool {
	defined := map[string]bool{}
	variables := map[string]bool{}
	for i := 0; i < len(c.tokens); i++ {
		t := c.tokens[i]
		if c.depth[i] != 0 || !c.statementStart(i) || t.k != tIdentifier && t.k != tString {
			continue
		}
		// name <- value, name = value, name <<- value; a chain a <- b <- value names both.
		j := i
		var names []token
		for (c.tokens[j].k == tIdentifier || c.tokens[j].k == tString) && (c.operator(j+1, "<-") || c.operator(j+1, "=") || c.operator(j+1, "<<-")) {
			names = append(names, c.tokens[j])
			j += 2
		}
		if len(names) > 0 {
			if j >= len(c.tokens) {
				break
			}
			kind, class := c.valueKind(j)
			for _, n := range names {
				switch kind {
				case "function":
					symbols.Add(n.s, "function", n.line)
					defined[n.s] = true
				case "class":
					name := n.s
					if class != "" {
						name = class
					}
					symbols.Add(name, "class", n.line)
					defined[name], defined[n.s] = true, true
					c.methods(j, name, symbols)
				case "generic":
					symbols.Add(n.s, "generic", n.line)
					defined[n.s] = true
				default:
					if !variables[n.s] && !defined[n.s] {
						variables[n.s] = true
						symbols.Add(n.s, "var", n.line)
					}
				}
			}
			continue
		}
		k := i
		if t.k == tIdentifier && c.operator(i+1, "::") {
			k = i + 2 // methods::setClass(...)
		}
		name, namespace, ok := c.callAt(k)
		if !ok || namespace != "" && namespace != "methods" && namespace != "base" {
			// Name$methods(...) adds methods to an R5 class.
			if t.k == tIdentifier && c.operator(i+1, "$") && c.is(i+2, tIdentifier, "methods") && c.operator(i+3, "(") {
				c.methodList(i+3, t.s, symbols)
			}
			continue
		}
		arguments := c.arguments(k + 1)
		first := ""
		if a, ok := positional(arguments, 0); ok {
			if v, one := c.single(a); one && v.k == tString {
				first = v.s
			}
		}
		switch name {
		case "setClass", "setRefClass":
			if first != "" {
				symbols.Add(first, "class", t.line)
				defined[first] = true
				c.methods(k, first, symbols)
			}
		case "setGeneric":
			if first != "" {
				symbols.Add(first, "generic", t.line)
				defined[first] = true
			}
		case "setMethod":
			if first == "" {
				break
			}
			signature := ""
			if a, ok := named(arguments, "signature"); ok {
				signature = c.firstString(a)
			} else if a, ok := positional(arguments, 1); ok {
				signature = c.firstString(a)
			}
			if signature != "" {
				symbols.Add(signature+"."+first, "method", t.line)
			} else {
				symbols.Add(first, "method", t.line)
			}
		case "assign":
			if a, ok := positional(arguments, 1); ok && first != "" && c.is(a.start, tIdentifier, "function") {
				symbols.Add(first, "function", t.line)
				defined[first] = true
			}
		}
	}
	return defined
}

// firstString is the first string literal among an argument's tokens:
// "Person", signature("Person", "numeric"), c(x = "Person").
func (c *code) firstString(a argument) string {
	for j := a.start; j < a.end; j++ {
		if c.tokens[j].k == tString {
			return c.tokens[j].s
		}
	}
	return ""
}

// valueKind says what the value starting at j defines: a function, a class
// generator (with the class name its first argument gives, if a string) or a
// generic.
func (c *code) valueKind(j int) (kind, class string) {
	t := c.tokens[j]
	if t.k == tIdentifier && t.s == "function" || c.operator(j, "\\") {
		return "function", ""
	}
	k := j
	if t.k == tIdentifier && (c.operator(j+1, "::") || c.operator(j+1, ":::")) {
		k = j + 2
	}
	name, _, ok := c.callAt(k)
	if !ok {
		return "", ""
	}
	first := ""
	if a, ok := positional(c.arguments(k+1), 0); ok {
		if v, one := c.single(a); one && v.k == tString {
			first = v.s
		}
	}
	switch name {
	case "R6Class", "setRefClass", "setClass", "new_class":
		return "class", first
	case "setGeneric", "new_generic":
		return "generic", ""
	}
	return "", ""
}

// methods records the methods of a class generator call starting at j (the call's
// name, or its namespace): R6's public, private and active lists and a reference
// class's methods list, as Class.method.
func (c *code) methods(j int, class string, symbols *lang.SymbolSet) {
	for k := j; k < len(c.tokens) && k <= j+3; k++ {
		if !c.operator(k, "(") {
			continue
		}
		for _, a := range c.arguments(k) {
			switch a.name {
			case "public", "private", "active", "methods":
				if c.is(a.start, tIdentifier, "list") && c.operator(a.start+1, "(") {
					c.methodList(a.start+1, class, symbols)
				}
			}
		}
		return
	}
}

// methodList records the functions of list(name = function...) or of a call's named
// arguments, whose "(" is at k.
func (c *code) methodList(k int, class string, symbols *lang.SymbolSet) {
	for _, m := range c.arguments(k) {
		if m.name != "" && m.start < m.end && (c.is(m.start, tIdentifier, "function") || c.operator(m.start, "\\")) {
			symbols.Add(class+"."+m.name, "method", c.tokens[m.start-2].line)
		}
	}
}

var (
	roxImport     = regexp.MustCompile(`^#'\s*@import\s+(.+)$`)
	roxImportFrom = regexp.MustCompile(`^#'\s*@importFrom\s+(\S+)`)
	roxInclude    = regexp.MustCompile(`^#'\s*@include\s+(.+)$`)
)

// roxygen reads the roxygen tags that name what a file depends on: @import and
// @importFrom (what roxygen2 writes into NAMESPACE, here attributed to the file that
// asks for it) and @include (the files that must be collated first).
//
// Implements: REQ-R-002
func roxygen(comments []comment, add func(kind, spec, module string, line int)) {
	for _, comment := range comments {
		if !strings.HasPrefix(comment.text, "#'") {
			continue
		}
		text := strings.TrimSpace(comment.text)
		if m := roxImportFrom.FindStringSubmatch(text); m != nil {
			if validPackage(m[1]) {
				add(kindRoxygen, "@importFrom "+m[1], m[1], comment.line)
			}
		} else if m := roxImport.FindStringSubmatch(text); m != nil {
			for _, p := range strings.Fields(m[1]) {
				if validPackage(p) {
					add(kindRoxygen, "@import "+p, p, comment.line)
				}
			}
		} else if m := roxInclude.FindStringSubmatch(text); m != nil {
			for _, f := range strings.Fields(m[1]) {
				add(kindInclude, "@include "+f, path.Clean(f), comment.line)
			}
		}
	}
}
