// Package kotlin analyzes Kotlin with tree-sitter. Its imports resolve the way Java's
// do, through the Java resolver: to project sources (Kotlin and Scala by the package
// they declare, Java by path), to the JDK, and to Maven groups declared by Gradle,
// Maven or sbt builds. Only the Kotlin standard library is its own.
package kotlin

import (
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/kotlin"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoMaven = "maven"
	ecoJDK   = "jdk"
	ecoStd   = "kotlin-std"
)

// The grammar names no fields, so a method's pattern captures its owner (@owner)
// alongside it. Interfaces and classes are both class_declaration, told apart by
// their keyword; an enum class is a class whose body is an enum body (@enum).
const query = `
(import_header) @import

(source_file (class_declaration "class" (type_identifier) @def.class))
(source_file (class_declaration "interface" (type_identifier) @def.interface))
(source_file (class_declaration (type_identifier) @enum (enum_class_body)))
(source_file (object_declaration (type_identifier) @def.object))
(source_file (function_declaration (simple_identifier) @def.func))
(source_file (property_declaration (variable_declaration (simple_identifier) @def.var)))
(source_file (type_alias (type_identifier) @def.type))
(class_declaration (type_identifier) @owner (class_body (function_declaration (simple_identifier) @def.method)))
(class_declaration (type_identifier) @owner (enum_class_body (function_declaration (simple_identifier) @def.method)))
(class_declaration (type_identifier) @owner (class_body (companion_object (class_body (function_declaration (simple_identifier) @def.method)))))
(object_declaration (type_identifier) @owner (class_body (function_declaration (simple_identifier) @def.method)))
`

var grammar = treesitter.MustGrammar("kotlin", kotlin.Language(), query)

// language is what sets Kotlin's imports apart from Java's: its standard library,
// which the compiler ships and every Kotlin file imports implicitly. kotlinx.* is not
// in it: coroutines and serialization are Maven artifacts.
//
// Implements: REQ-KT-002
var language = java.Language{Std: ecoStd, Prefixes: []string{"kotlin."}}

// Implements: REQ-KT-001
type Plugin struct{}

func (Plugin) Name() string { return "kotlin" }
func (Plugin) Version() int { return 1 }
func (Plugin) Claims(f *scan.File) bool {
	return (strings.HasSuffix(f.Path, ".kt") || strings.HasSuffix(f.Path, ".kts")) && !f.Binary
}
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoMaven, Name: "Maven"},
		{ID: ecoJDK, Name: "Java standard library", Std: true},
		{ID: ecoStd, Name: "Kotlin standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return java.NewResolver(all, language), nil
}

// Implements: REQ-KT-001
func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	type def struct {
		name, kind string
		line       int
	}
	var defs []def
	enums := map[def]bool{}
	err := grammar.Matches(src, func(m treesitter.Match) {
		for _, c := range m {
			switch {
			case c.Name == "import":
				if imp, ok := parseImport(c.Text); ok {
					imp.Line = c.Line
					ex.Imports = append(ex.Imports, imp)
				}
			case c.Name == "enum":
				enums[def{c.Text, "", c.Line}] = true
			case c.Name == "def.method":
				owner, _ := m.Get("owner")
				defs = append(defs, def{owner + "." + c.Text, "method", c.Line})
			case strings.HasPrefix(c.Name, "def."):
				defs = append(defs, def{c.Text, strings.TrimPrefix(c.Name, "def."), c.Line})
			}
		}
	})
	var symbols lang.SymbolSet
	for _, d := range defs {
		if d.kind == "class" && enums[def{d.name, "", d.line}] {
			d.kind = "enum"
		}
		symbols.Add(d.name, d.kind, d.line)
	}
	ex.Symbols = symbols.List()
	return ex, err
}

// parseImport reads "import a.b.C", "import a.b.*" and "import a.b.C as D". The
// alias stays in the spec, where it tells the reader what the file calls the import;
// resolution only needs the path.
//
// Implements: REQ-KT-001
func parseImport(text string) (lang.RawImport, bool) {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ";"))
	rest, ok := strings.CutPrefix(text, "import")
	if !ok {
		return lang.RawImport{}, false
	}
	target, alias, _ := strings.Cut(rest, " as ")
	module := strings.Join(strings.Fields(target), "")
	if module == "" {
		return lang.RawImport{}, false
	}
	spec := "import " + module
	if alias = strings.TrimSpace(alias); alias != "" {
		spec += " as " + alias
	}
	return lang.RawImport{Spec: spec, Module: module}, true
}
