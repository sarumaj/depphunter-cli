// Package java analyses Java with tree-sitter. Imports resolve to project sources by
// package path (any source root: src/main/java, src, …), to the JDK, or to Maven /
// Gradle dependencies matched by groupId, since Java imports name packages, not
// artifacts.
package java

import (
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/java"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoMaven = "maven"
	ecoJDK   = "jdk"
)

const query = `
(import_declaration) @import

(program (class_declaration name: (identifier) @def.class))
(program (interface_declaration name: (identifier) @def.interface))
(program (enum_declaration name: (identifier) @def.enum))
(program (record_declaration name: (identifier) @def.record))
(program (annotation_type_declaration name: (identifier) @def.annotation))
(class_declaration body: (class_body (method_declaration name: (identifier) @def.method)))
(enum_declaration body: (enum_body (enum_body_declarations (method_declaration name: (identifier) @def.method))))
(record_declaration body: (class_body (method_declaration name: (identifier) @def.method)))
`

var grammar = treesitter.MustGrammar("java", java.Language(), query)

type Plugin struct{}

func (Plugin) Name() string             { return "java" }
func (Plugin) Version() int             { return 1 }
func (Plugin) Claims(f *scan.File) bool { return strings.HasSuffix(f.Path, ".java") && !f.Binary }
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoMaven, Name: "Maven"},
		{ID: ecoJDK, Name: "Java standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	var syms lang.SymbolSet
	err := grammar.Matches(src, func(m treesitter.Match) {
		for _, c := range m {
			switch {
			case c.Name == "import":
				// "import static a.b.C.m;" -> Module "a.b.C.m", Name "static"
				fields := strings.Fields(strings.TrimSuffix(strings.TrimSpace(c.Text), ";"))
				if len(fields) < 2 {
					continue
				}
				imp := lang.RawImport{Spec: strings.Join(fields, " "), Module: strings.Join(fields[1:], ""), Line: c.Line}
				if fields[1] == "static" {
					imp.Module, imp.Name = strings.Join(fields[2:], ""), "static"
				}
				ex.Imports = append(ex.Imports, imp)
			case c.Name == "def.method":
				owner := c.EnclosingName("class_declaration", "enum_declaration", "record_declaration")
				syms.Add(owner+"."+c.Text, "method", c.Line)
			case strings.HasPrefix(c.Name, "def."):
				syms.Add(c.Text, strings.TrimPrefix(c.Name, "def."), c.Line)
			}
		}
	})
	ex.Symbols = syms.List()
	return ex, err
}
