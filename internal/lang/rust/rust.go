// Package rust analyzes Rust with tree-sitter. `use` paths resolve through the module
// tree (crate::, self::, super::, `mod x;` files), workspace and path dependencies,
// the standard crates, and crates.io dependencies from Cargo.toml pinned by Cargo.lock.
package rust

import (
	"regexp"
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/rust"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoCrates = "crates"
	ecoStd    = "rust-std"
)

// @use is a use tree; @crate an extern crate; @mod a `mod name;` declaration (its
// file is a dependency of the declaring module); @def.<kind> names a definition.
const query = `
(use_declaration argument: (_) @use)
(extern_crate_declaration name: (identifier) @crate)
(mod_item name: (identifier) @mod !body)

(source_file (function_item name: (identifier) @def.func))
(source_file (struct_item name: (type_identifier) @def.struct))
(source_file (enum_item name: (type_identifier) @def.enum))
(source_file (trait_item name: (type_identifier) @def.trait))
(source_file (type_item name: (type_identifier) @def.type))
(source_file (const_item name: (identifier) @def.const))
(source_file (static_item name: (identifier) @def.var))
(source_file (macro_definition name: (identifier) @def.macro))
(source_file (mod_item name: (identifier) @def.mod body: (_)))
(impl_item body: (declaration_list (function_item name: (identifier) @def.method)))
`

var grammar = treesitter.MustGrammar("rust", rust.Language(), query)

// Implements: REQ-RS-009
type Plugin struct{}

func (Plugin) Name() string             { return "rust" }
func (Plugin) Version() int             { return 1 }
func (Plugin) Claims(f *scan.File) bool { return strings.HasSuffix(f.Path, ".rs") && !f.Binary }
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoCrates, Name: "crates.io"},
		{ID: ecoStd, Name: "Rust standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-RS-001, REQ-RS-003
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	err := grammar.Matches(src, func(m treesitter.Match) {
		for _, c := range m {
			switch {
			case c.Name == "use":
				for _, p := range expandUse(c.Text) {
					ex.Imports = append(ex.Imports, lang.RawImport{Spec: "use " + p, Module: p, Line: c.Line})
				}
			case c.Name == "crate":
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: "extern crate " + c.Text, Module: c.Text, Line: c.Line})
			case c.Name == "mod":
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: "mod " + c.Text, Module: "self::" + c.Text, Line: c.Line})
			case c.Name == "def.method":
				typ := c.EnclosingField("type", "impl_item")
				if i := strings.IndexAny(typ, "<"); i >= 0 {
					typ = typ[:i] // Server<T> -> Server
				}
				symbols.Add(typ+"."+c.Text, "method", c.Line)
			case strings.HasPrefix(c.Name, "def."):
				symbols.Add(c.Text, strings.TrimPrefix(c.Name, "def."), c.Line)
			}
		}
	})
	ex.Symbols = symbols.List()
	return ex, err
}

var alias = regexp.MustCompile(`\s+as\s+[A-Za-z_][A-Za-z0-9_]*`)

// expandUse turns a use tree into the paths it imports:
// "crate::a::{self, b::c as d, e::*}" -> crate::a, crate::a::b::c, crate::a::e.
//
// Implements: REQ-RS-001
func expandUse(tree string) []string {
	tree = strings.Join(strings.Fields(alias.ReplaceAllString(tree, "")), "")
	var out []string
	seen := map[string]bool{}
	for _, p := range expand("", tree) {
		p = strings.TrimSuffix(strings.TrimSuffix(p, "::*"), "::self")
		if p != "" && p != "*" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func expand(prefix, s string) []string {
	join := func(a, b string) string {
		if a == "" {
			return b
		}
		if b == "" {
			return a
		}
		return a + "::" + b
	}
	i := strings.Index(s, "{")
	if i < 0 || !strings.HasSuffix(s, "}") {
		return []string{join(prefix, s)}
	}
	head := join(prefix, strings.TrimSuffix(s[:i], "::"))
	var out []string
	depth, start := 0, i+1
	for j := i + 1; j < len(s)-1; j++ {
		switch s[j] {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, expand(head, s[start:j])...)
				start = j + 1
			}
		}
	}
	if part := s[start : len(s)-1]; part != "" {
		out = append(out, expand(head, part)...)
	}
	return out
}
