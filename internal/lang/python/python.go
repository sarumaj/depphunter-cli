// Package python analyzes Python with tree-sitter. Imports resolve to project modules
// (relative imports, the importing file's directory, the project root, src/ layouts
// and every directory holding a pyproject.toml, setup.py or setup.cfg), to the
// standard library, or to distributions declared in requirements files, pyproject.toml
// or Pipfile, with versions pinned by poetry.lock, uv.lock, pdm.lock or Pipfile.lock.
package python

import (
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/python"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoPyPI = "pypi"
	ecoStd  = "python-std"
)

// @import is a dotted module; @from.module / @from.name come from "from m import n"
// (one match per imported name); @future is a __future__ import; @def.<kind> names a
// definition. The runtime may elide expression_statement wrappers, hence the alternation.
//
// Implements: REQ-LANG-023, REQ-PY-001, REQ-PY-014
const query = `
(import_statement name: (dotted_name) @import)
(import_statement name: (aliased_import name: (dotted_name) @import))
(import_from_statement module_name: (_) @from.module name: (dotted_name) @from.name)
(import_from_statement module_name: (_) @from.module name: (aliased_import name: (dotted_name) @from.name))
(import_from_statement module_name: (_) @from.module (wildcard_import))
(future_import_statement) @future

(module (function_definition name: (identifier) @def.func))
(module (decorated_definition definition: (function_definition name: (identifier) @def.func)))
(module (class_definition name: (identifier) @def.class))
(module (decorated_definition definition: (class_definition name: (identifier) @def.class)))
(module [
  (expression_statement (assignment left: (identifier) @def.var))
  (assignment left: (identifier) @def.var)
])
(class_definition body: (block (function_definition name: (identifier) @def.method)))
(class_definition body: (block (decorated_definition definition: (function_definition name: (identifier) @def.method))))
`

// Implements: REQ-LANG-007
var grammar = treesitter.MustGrammar("python", python.Language(), query)

type Plugin struct{}

func (Plugin) Name() string { return "python" }

func (Plugin) Claims(f *scan.File) bool {
	return (strings.HasSuffix(f.Path, ".py") || strings.HasSuffix(f.Path, ".pyi")) && !f.Binary
}

// Implements: REQ-PY-005
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoPyPI, Name: "PyPI"},
		{ID: ecoStd, Name: "Python standard library", Std: true},
	}
}

func (Plugin) Version() int { return 1 }

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all, lang.Claimed(Plugin{}, all)), nil
}

// Implements: REQ-PY-001, REQ-PY-014, REQ-LANG-024
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	err := grammar.Matches(src, func(m treesitter.Match) {
		if mod, ok := m.Get("from.module"); ok {
			name, _ := m.Get("from.name")
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: fromSpec(mod, name), Module: mod, Name: name, Line: m[0].Line})
			return
		}
		for _, c := range m {
			switch {
			case c.Name == "future":
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: c.Text, Module: "__future__", Line: c.Line})
			case c.Name == "import":
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: c.Text, Module: c.Text, Line: c.Line})
			case c.Name == "def.method":
				symbols.Add(c.EnclosingName("class_definition")+"."+c.Text, "method", c.Line)
			case strings.HasPrefix(c.Name, "def."):
				symbols.Add(c.Text, strings.TrimPrefix(c.Name, "def."), c.Line)
			}
		}
	})
	ex.Symbols = symbols.List()
	return ex, err
}

func fromSpec(mod, name string) string {
	if name == "" {
		return "from " + mod + " import *"
	}
	return "from " + mod + " import " + name
}
