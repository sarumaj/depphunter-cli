// Package javascript analyzes JavaScript and TypeScript with tree-sitter and resolves
// imports through relative paths, tsconfig/jsconfig "paths", workspace packages,
// package.json dependencies and package-lock.json versions.
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
	ecoNPM  = "npm"
	ecoNode = "node"
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

func (Plugin) Claims(f *scan.File) bool { return grammarFor(f.Path) != nil && !f.Binary }

// Implements: REQ-JS-005
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoNPM, Name: "npm"},
		{ID: ecoNode, Name: "Node.js built-ins", Std: true},
	}
}

func (Plugin) Version() int { return 1 }

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-JS-001, REQ-JS-011, REQ-LANG-024
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	err := grammarFor(f.Path).Matches(src, func(m treesitter.Match) {
		for _, c := range m {
			switch {
			case c.Name == "import":
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: c.Text, Module: c.Text, Line: c.Line})
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
	ex.Symbols = symbols.List()
	return ex, err
}
