// Package javascript analyzes JavaScript and TypeScript with tree-sitter and resolves
// imports through relative paths, tsconfig/jsconfig "paths", workspace packages,
// package.json dependencies and the versions of npm-shrinkwrap.json, package-lock.json, yarn.lock,
// pnpm-lock.yaml and bun.lock (bun.go). Vue, Svelte and Astro
// components belong to the same ecosystem: their script blocks are read with the
// same grammars and resolved by the same resolver (component.go).
package javascript

import (
	"path"
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/javascript"
	"github.com/odvcencio/gotreesitter/grammars/tsx"
	"github.com/odvcencio/gotreesitter/grammars/typescript"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemNPM  = "npm"
	ecosystemNode = "node"
)

// Captures: @import is a module specifier; @def.<kind> the name of a top-level
// definition (methods are named after their class).
//
// Implements: REQ-LANG-023, REQ-JS-001, REQ-JS-011
const commonQuery = `
(import_statement source: (string (string_fragment) @import))
(export_statement source: (string (string_fragment) @import))
(call_expression
  function: (identifier) @_fn
  arguments: (arguments . (string (string_fragment) @import))
  (#eq? @_fn "require"))
(call_expression function: (import) arguments: (arguments . (string (string_fragment) @import)))

(program (function_declaration name: (_) @def.func))
(program (generator_function_declaration name: (_) @def.func))
(program (class_declaration name: (_) @def.class))
(program (lexical_declaration (variable_declarator name: (identifier) @def.var)))
(program (variable_declaration (variable_declarator name: (identifier) @def.var)))
(program (export_statement declaration: (function_declaration name: (_) @def.func)))
(program (export_statement declaration: (generator_function_declaration name: (_) @def.func)))
(program (export_statement declaration: (class_declaration name: (_) @def.class)))
(program (export_statement declaration: (lexical_declaration (variable_declarator name: (identifier) @def.var))))
(class_declaration body: (class_body (method_definition name: (_) @def.method)))
`

const typescriptQuery = commonQuery + `
(import_require_clause source: (string (string_fragment) @import))
(program (interface_declaration name: (_) @def.interface))
(program (type_alias_declaration name: (_) @def.type))
(program (enum_declaration name: (_) @def.enum))
(program (abstract_class_declaration name: (_) @def.class))
(program (export_statement declaration: (interface_declaration name: (_) @def.interface)))
(program (export_statement declaration: (type_alias_declaration name: (_) @def.type)))
(program (export_statement declaration: (enum_declaration name: (_) @def.enum)))
(program (export_statement declaration: (abstract_class_declaration name: (_) @def.class)))
(abstract_class_declaration body: (class_body (method_definition name: (_) @def.method)))
`

// Implements: REQ-LANG-007
var (
	jsGrammar  = treesitter.MustGrammar("javascript", javascript.Language(), commonQuery)
	tsGrammar  = treesitter.MustGrammar("typescript", typescript.Language(), typescriptQuery)
	tsxGrammar = treesitter.MustGrammar("tsx", tsx.Language(), typescriptQuery)
)

// Implements: REQ-JS-001
func grammarFor(p string) *treesitter.Grammar {
	switch path.Ext(p) {
	case ".ts", ".mts", ".cts":
		return tsGrammar
	case ".tsx":
		return tsxGrammar
	case ".js", ".jsx", ".mjs", ".cjs":
		return jsGrammar
	}
	return nil
}

type Plugin struct{}

func (Plugin) Name() string { return "javascript" }

// Claims leaves out a ".ts" file the scan found to be XML (a Qt Linguist
// translation): the TypeScript grammar would spend the whole parse bound on it and
// find nothing.
//
// Implements: REQ-JS-001, REQ-JS-012
func (Plugin) Claims(f *scan.File) bool {
	return (grammarFor(f.Path) != nil || component(f.Path)) && !f.Binary && f.Language != "XML"
}

// Implements: REQ-JS-005
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemNPM, Name: "npm"},
		{ID: ecosystemNode, Name: "Node.js built-ins", Std: true},
	}
}

func (Plugin) Version() int { return 2 }

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-JS-001, REQ-JS-011, REQ-JS-012, REQ-LANG-024
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	var err error
	if g := grammarFor(f.Path); g != nil {
		err = extract(g, source, extraction, &symbols)
	} else {
		err = extractComponent(f.Path, source, extraction, &symbols)
	}
	extraction.Symbols = symbols.List()
	return extraction, err
}

// extract adds the imports and top-level definitions g finds in source.
func extract(g *treesitter.Grammar, source []byte, extraction *lang.Extraction, symbols *lang.SymbolSet) error {
	return g.Matches(source, func(m treesitter.Match) {
		for _, c := range m {
			switch {
			case c.Name == "import":
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: c.Text, Module: c.Text, Line: c.Line})
			case c.Name == "def.method":
				class := c.EnclosingName("class_declaration", "abstract_class_declaration", "class")
				symbols.Add(class+"."+c.Text, "method", c.Line)
			case c.Name == "def.var":
				kind := "var"
				switch c.SiblingFieldType("value") {
				case "arrow_function", "function_expression", "function", "generator_function":
					kind = "func"
				}
				symbols.Add(c.Text, kind, c.Line)
			case strings.HasPrefix(c.Name, "def."):
				symbols.Add(c.Text, strings.TrimPrefix(c.Name, "def."), c.Line)
			}
		}
	})
}
