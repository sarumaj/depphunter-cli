// Package php analyzes PHP with tree-sitter. `use` statements (grouped, `use
// function`, `use const`), fully qualified names in code (`new \X\Y`, `\X\Y::call()`,
// `extends`, `implements`, `catch`, `\f()`) and `require`/`include` of a path the
// file spells out resolve to project files through composer.json autoload rules and
// the declarations the project's own files make (resolve.go), to PHP's built-in
// classes and functions (builtin.go), and to Composer packages by the namespaces
// composer.lock says each installed package autoloads (composer.go).
package php

import (
	"cmp"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/php"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoComposer = "composer"
	ecoStd      = "php-std"
)

// exts are the extensions the plugin claims. A .inc file that is not PHP (Pascal,
// assembly, BitBake use the extension too) has no `<?php` tag, so the whole file
// parses as inline text and yields nothing.
//
// Implements: REQ-PHP-001
var exts = map[string]bool{".php": true, ".phtml": true, ".inc": true}

// @use is a use statement (text, split in Go); @inc a require/include; @ref.<form> a
// class or function named in code; @def.<kind> a definition; @ns.global a namespace
// block without a name; @fn/@dstr a call with a string first argument (define()).
// Where a definition sits (class, function body) is read from its ancestors.
const query = `
(namespace_definition name: (namespace_name) @def.namespace)
(namespace_definition !name) @ns.global
(namespace_use_declaration) @use

(require_expression) @inc
(require_once_expression) @inc
(include_expression) @inc
(include_once_expression) @inc

(object_creation_expression . [(qualified_name) (name)] @ref.new)
(scoped_call_expression scope: [(qualified_name) (name)] @ref.static)
(class_constant_access_expression . [(qualified_name) (name)] @ref.static)
(base_clause [(qualified_name) (name)] @ref.extends)
(class_interface_clause [(qualified_name) (name)] @ref.implements)
(use_declaration [(qualified_name) (name)] @ref.trait)
(catch_clause type: (type_list (named_type [(qualified_name) (name)] @ref.catch)))
(function_call_expression function: (qualified_name) @ref.call)

(class_declaration name: (name) @def.class)
(interface_declaration name: (name) @def.interface)
(trait_declaration name: (name) @def.trait)
(enum_declaration name: (name) @def.enum)
(function_definition name: (name) @def.func)
(method_declaration name: (name) @def.method)
(const_declaration (const_element (name) @def.const))
(function_call_expression function: (name) @fn arguments: (arguments . (argument (string (string_content) @dstr))))
`

var grammar = treesitter.MustGrammar("php", php.Language(), query)

// Implements: REQ-PHP-001
type Plugin struct{}

func (Plugin) Name() string { return "php" }
func (Plugin) Version() int { return 1 }
func (Plugin) Claims(f *scan.File) bool {
	return exts[strings.ToLower(path.Ext(f.Path))] && !f.Binary
}
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoComposer, Name: "Packagist"},
		{ID: ecoStd, Name: "PHP standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(root, all), nil
}

// Import kinds, carried in RawImport.Name.
const (
	kindClass    = "class"    // a fully qualified class, interface, trait or enum
	kindFunction = "function" // a fully qualified function
	kindConst    = "const"    // a fully qualified constant
	kindLocal    = "local"    // a name relative to the file's namespace: a project file or nothing
	kindInclude  = "include"  // require/include of a path
)

// ref is a name used in code, kept until the file's namespaces and aliases are known.
type ref struct {
	form, text string
	line       int
}

type namespace struct {
	line int
	name string
}

// classLike are the declarations that own methods and constants.
var classLike = map[string]bool{
	"class_declaration": true, "interface_declaration": true, "trait_declaration": true,
	"enum_declaration": true,
}

// Implements: REQ-PHP-002, REQ-PHP-003, REQ-PHP-004
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	var refs []ref
	var namespaces []namespace
	aliases := map[string]string{} // lower-case alias -> fully qualified class
	err := grammar.Matches(src, func(m treesitter.Match) {
		if fn, ok := m.Get("fn"); ok {
			if name, _ := m.Get("dstr"); strings.EqualFold(fn, "define") && validName.MatchString(name) {
				symbols.Add(name, "const", m[0].Line)
			}
			return
		}
		for _, c := range m {
			switch {
			case c.Name == "ns.global":
				namespaces = append(namespaces, namespace{c.Line, ""})
			case c.Name == "def.namespace":
				name := strings.Join(strings.Fields(c.Text), "")
				namespaces = append(namespaces, namespace{c.Line, name})
				symbols.Add(name, "namespace", c.Line)
			case c.Name == "use":
				for _, u := range parseUse(c.Text) {
					spec := "use " + u.fqn
					if u.kind != kindClass {
						spec = "use " + u.kind + " " + u.fqn
					} else {
						aliases[strings.ToLower(u.alias)] = u.fqn
					}
					ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: u.fqn, Name: u.kind, Line: c.Line})
				}
			case c.Name == "inc":
				if p, ok := includePath(c.Text); ok {
					ex.Imports = append(ex.Imports, lang.RawImport{
						Spec: strings.Join(strings.Fields(c.Text), " "), Module: p, Name: kindInclude, Line: c.Line,
					})
				}
			case strings.HasPrefix(c.Name, "ref."):
				refs = append(refs, ref{strings.TrimPrefix(c.Name, "ref."), strings.Join(strings.Fields(c.Text), ""), c.Line})
			case c.Name == "def.method", c.Name == "def.const":
				owner, inBody := ownerOf(c.Scopes())
				switch {
				case inBody:
				case owner != "":
					symbols.Add(owner+"."+c.Text, strings.TrimPrefix(c.Name, "def."), c.Line)
				case c.Name == "def.const":
					symbols.Add(c.Text, "const", c.Line)
				}
			case c.Name == "def.func":
				if _, inBody := ownerOf(c.Scopes()); !inBody {
					symbols.Add(c.Text, "func", c.Line)
				}
			case strings.HasPrefix(c.Name, "def."):
				symbols.Add(c.Text, strings.TrimPrefix(c.Name, "def."), c.Line)
			}
		}
	})
	slices.SortFunc(namespaces, func(a, b namespace) int { return cmp.Compare(a.line, b.line) })
	seen := map[string]bool{}
	for _, r := range refs {
		ns := ""
		for _, n := range namespaces {
			if n.line <= r.line {
				ns = n.name
			}
		}
		fqn, kind := qualify(r.text, r.form == "call", ns, aliases)
		if fqn == "" || seen[kind+" "+strings.ToLower(fqn)] {
			continue
		}
		seen[kind+" "+strings.ToLower(fqn)] = true
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: refSpec(r), Module: fqn, Name: kind, Line: r.line})
	}
	slices.SortStableFunc(ex.Imports, func(a, b lang.RawImport) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Spec, b.Spec))
	})
	ex.Symbols = symbols.List()
	return ex, err
}

var validName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ownerOf reads a method's or constant's class from the ancestors of its name (the
// first is its own declaration), and whether it sits in a function body (a local
// function, a method of an anonymous class) and is not a definition anybody can name.
func ownerOf(scopes []treesitter.Scope) (owner string, inBody bool) {
	for i, s := range scopes {
		if i == 0 && (s.Type == "function_definition" || s.Type == "method_declaration") {
			continue
		}
		switch {
		case classLike[s.Type]:
			return s.Name, false
		case s.Type == "anonymous_class", s.Type == "function_definition", s.Type == "method_declaration",
			s.Type == "anonymous_function", s.Type == "arrow_function":
			return "", true
		}
	}
	return "", false
}

// refSpec is how a name used in code is shown: as the construct that uses it.
func refSpec(r ref) string {
	switch r.form {
	case "new":
		return "new " + r.text
	case "static":
		return r.text + "::"
	case "call":
		return r.text + "()"
	}
	return r.form + " " + r.text // extends, implements, catch
}

// qualify resolves a name used in code the way PHP does: `\X` is fully qualified,
// `namespace\X` and an unqualified or qualified name are relative to the file's
// namespace unless their first segment is an alias a use statement made. A name that
// is exactly an alias is already an import (its use statement) and yields "". A
// function call is kept only when fully qualified or aliased: PHP falls back to the
// global function for any other, so its target is not knowable from the text.
//
// Implements: REQ-PHP-002
func qualify(name string, call bool, ns string, aliases map[string]string) (fqn, kind string) {
	kind = kindClass
	if call {
		kind = kindFunction
	}
	switch low := strings.ToLower(name); {
	case low == "self" || low == "static" || low == "parent" || name == "":
		return "", ""
	case strings.HasPrefix(name, `\`):
		return strings.TrimPrefix(name, `\`), kind
	case strings.HasPrefix(low, `namespace\`):
		return join(ns, name[len(`namespace\`):]), kindLocal
	}
	first, rest, qualified := strings.Cut(name, `\`)
	if target, ok := aliases[strings.ToLower(first)]; ok {
		if !qualified {
			return "", ""
		}
		return target + `\` + rest, kind
	}
	if call {
		return "", ""
	}
	if ns == "" {
		return name, kindClass
	}
	return join(ns, name), kindLocal
}

func join(ns, name string) string {
	if ns == "" {
		return name
	}
	return ns + `\` + name
}

// use is one name a use statement imports.
type use struct {
	kind, fqn, alias string
}

var useAlias = regexp.MustCompile(`(?i)\s+as\s+([A-Za-z_][A-Za-z0-9_]*)\s*$`)

// parseUse expands a use statement: "use A\{B, C as D, function e}" imports A\B, A\C
// (alias D) and the function A\e; "use function a\b, a\c" two functions.
//
// Implements: REQ-PHP-002
func parseUse(text string) []use {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(stripComments(text)), ";"))
	text = strings.TrimSpace(strings.TrimPrefix(text, "use"))
	kind, text := useKind(text, kindClass)
	var out []use
	add := func(kind, prefix, item string) {
		kind, item = useKind(strings.TrimSpace(item), kind)
		alias := ""
		if m := useAlias.FindStringSubmatchIndex(item); m != nil {
			alias, item = item[m[2]:m[3]], item[:m[0]]
		}
		item = strings.Join(strings.Fields(item), "")
		fqn := strings.TrimPrefix(join(strings.Trim(prefix, `\`), strings.TrimPrefix(item, `\`)), `\`)
		if item == "" || fqn == "" {
			return
		}
		if alias == "" {
			alias = fqn[strings.LastIndex(fqn, `\`)+1:]
		}
		out = append(out, use{kind, fqn, alias})
	}
	if i := strings.Index(text, "{"); i >= 0 {
		prefix := strings.Join(strings.Fields(text[:i]), "")
		body := strings.TrimSuffix(strings.TrimSpace(text[i+1:]), "}")
		for _, item := range strings.Split(body, ",") {
			add(kind, prefix, item)
		}
		return out
	}
	for _, item := range strings.Split(text, ",") {
		add(kind, "", item)
	}
	return out
}

// useKind takes a leading `function` or `const` off a use clause.
func useKind(s, def string) (string, string) {
	for _, k := range []string{kindFunction, kindConst} {
		if len(s) > len(k) && strings.EqualFold(s[:len(k)], k) && (s[len(k)] == ' ' || s[len(k)] == '\t' || s[len(k)] == '\n') {
			return k, strings.TrimSpace(s[len(k):])
		}
	}
	return def, s
}

var comments = regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*|#[^\n\[][^\n]*`)

func stripComments(s string) string { return comments.ReplaceAllString(s, " ") }

// includePath evaluates the path a require or include names, when the file spells
// it out: string literals, __DIR__, dirname(__FILE__) and dirname(__DIR__[, n]),
// DIRECTORY_SEPARATOR, joined with `.`. A path relative to the including file comes
// back as "__DIR__/..."; one PHP looks up on its include path as written. Anything
// else (a variable, a constant, a function call) is not evaluated.
//
// Implements: REQ-PHP-004
func includePath(text string) (string, bool) {
	expr := strings.TrimSpace(stripComments(text))
	for _, kw := range []string{"require_once", "include_once", "require", "include"} {
		if len(expr) >= len(kw) && strings.EqualFold(expr[:len(kw)], kw) {
			expr = strings.TrimSpace(expr[len(kw):])
			break
		}
	}
	for strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") && balanced(expr[1:len(expr)-1]) {
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	var out strings.Builder
	for i, part := range splitConcat(expr) {
		part = strings.TrimSpace(part)
		if i == 0 {
			if dir, ok := dirExpr(part); ok {
				out.WriteString(dir)
				continue
			}
		}
		switch {
		case part == "DIRECTORY_SEPARATOR":
			out.WriteString("/")
		case len(part) >= 2 && part[0] == '\'' && part[len(part)-1] == '\'':
			out.WriteString(part[1 : len(part)-1])
		case len(part) >= 2 && part[0] == '"' && part[len(part)-1] == '"' && !strings.ContainsAny(part, "${"):
			out.WriteString(part[1 : len(part)-1])
		default:
			return "", false
		}
	}
	if p := out.String(); p != "" && p != "__DIR__" {
		return p, true
	}
	return "", false
}

var dirname = regexp.MustCompile(`^(?i:dirname)\s*\(\s*(__DIR__|__FILE__)\s*(?:,\s*(\d+)\s*)?\)$`)

// dirExpr reads an expression naming the including file's directory or one above it.
func dirExpr(s string) (string, bool) {
	if s == "__DIR__" {
		return "__DIR__", true
	}
	m := dirname.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	levels := 1
	if m[2] != "" {
		levels = int(m[2][0] - '0')
		if len(m[2]) > 1 || levels < 1 {
			return "", false
		}
	}
	if m[1] == "__FILE__" {
		levels-- // dirname(__FILE__) is __DIR__
	}
	return "__DIR__" + strings.Repeat("/..", levels), true
}

// splitConcat splits an expression at the `.` operators outside strings and brackets.
func splitConcat(s string) []string {
	var parts []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == '.' && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// balanced reports whether s closes every bracket it opens, so "(a) . (b)" is not
// mistaken for one parenthesized expression.
func balanced(s string) bool {
	depth := 0
	for _, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			if depth--; depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}
