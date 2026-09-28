// Package scala analyzes Scala 2 and 3 with tree-sitter. Its imports resolve the way
// Java's do, through the Java resolver: to project sources (Scala and Kotlin by the
// package they declare, Java by path), to the JDK, and to Maven artifacts declared
// by sbt, Gradle or Maven builds. Only the Scala standard library is its own.
package scala

import (
	"strings"

	"github.com/odvcencio/gotreesitter/grammars/scala"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
	"github.com/sarumaj/depphunter-cli/internal/lang/treesitter"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemMaven = "maven"
	ecosystemJDK   = "jdk"
	ecosystemStd   = "scala-std"
)

// Imports are taken wherever they are: Scala scopes them to a block as often as to
// the file. Definitions are those of the file and of a package block (Scala 2's
// package a.b { ... }), and the methods of every class, trait, object and enum.
const query = `
(import_declaration) @import

(compilation_unit (class_definition name: (identifier) @def.class))
(compilation_unit (trait_definition name: (identifier) @def.trait))
(compilation_unit (object_definition name: (identifier) @def.object))
(compilation_unit (enum_definition name: (identifier) @def.enum))
(compilation_unit (function_definition name: (identifier) @def.func))
(compilation_unit (val_definition pattern: (identifier) @def.var))
(compilation_unit (type_definition name: (type_identifier) @def.type))
(compilation_unit (extension_definition body: (function_definition name: (identifier) @def.func)))
(package_clause body: (template_body (class_definition name: (identifier) @def.class)))
(package_clause body: (template_body (trait_definition name: (identifier) @def.trait)))
(package_clause body: (template_body (object_definition name: (identifier) @def.object)))
(template_body (function_definition name: (identifier) @def.method))
(template_body (function_declaration name: (identifier) @def.method))
(enum_body (function_definition name: (identifier) @def.method))
`

var grammar = treesitter.MustGrammar("scala", scala.Language(), query)

// language is what sets Scala's imports apart from Java's. Its standard library is
// scala-library, less the modules once part of it and now published on their own
// (scala-xml, scala-parser-combinators, …) and the Scala.js and Scala Native
// libraries that share its root package: all Maven dependencies. Its imports are
// relative, and scala._ is imported everywhere, so "import collection.mutable" is
// scala.collection.mutable unless the project has a package collection.
//
// Implements: REQ-SCALA-002, REQ-SCALA-007
var language = java.Language{
	Std:      ecosystemStd,
	Prefixes: []string{"scala."},
	Except: []string{
		"scala.xml", "scala.util.parsing", "scala.collection.parallel", "scala.swing", "scala.async",
		"scala.scalajs", "scala.scalanative",
	},
	Relative: true,
	Implicit: []string{
		"annotation", "beans", "collection", "compat", "concurrent", "io", "jdk", "math", "ref", "reflect",
		"runtime", "sys", "util",
	},
}

// Implements: REQ-SCALA-001
type Plugin struct{}

func (Plugin) Name() string { return "scala" }
func (Plugin) Version() int { return 3 }
func (Plugin) Claims(f *scan.File) bool {
	return (strings.HasSuffix(f.Path, ".scala") || strings.HasSuffix(f.Path, ".sc")) && !f.Binary
}
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemMaven, Name: "Maven"},
		{ID: ecosystemJDK, Name: "Java standard library", Std: true},
		{ID: ecosystemStd, Name: "Scala standard library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return java.NewResolver(all, language), nil
}

// Implements: REQ-SCALA-001
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	err := grammar.Matches(source, func(m treesitter.Match) {
		for _, c := range m {
			switch {
			case c.Name == "import":
				for _, rawImport := range expandImport(c.Text) {
					rawImport.Line = c.Line
					extraction.Imports = append(extraction.Imports, rawImport)
				}
			case c.Name == "def.method":
				owner := c.EnclosingName("class_definition", "trait_definition", "object_definition", "enum_definition")
				if owner == "" { // a definition directly in a package block
					symbols.Add(c.Text, "func", c.Line)
					continue
				}
				symbols.Add(owner+"."+c.Text, "method", c.Line)
			case strings.HasPrefix(c.Name, "def."):
				symbols.Add(c.Text, strings.TrimPrefix(c.Name, "def."), c.Line)
			}
		}
	})
	extraction.Symbols = symbols.List()
	return extraction, err
}

// expandImport splits one import into what it names, the way Rust's use trees are
// split: "import a.{B, C => D}, e.F" imports a.B, a.C and e.F. A wildcard - Scala 2's
// _, Scala 3's * and given - becomes ".*"; a rename keeps the original name, and a
// selector renamed to _ hides that name rather than importing it.
//
// Implements: REQ-SCALA-001
func expandImport(text string) []lang.RawImport {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ";"))
	rest, ok := strings.CutPrefix(text, "import")
	if !ok {
		return nil
	}
	var out []lang.RawImport
	for _, clause := range splitTop(rest) {
		prefix, selectors, braced := strings.Cut(clause, "{")
		if !braced {
			path, _, _ := strings.Cut(clause, " as ") // Scala 3: import a.b as c
			path = compact(path)
			if module := wildcard(path); module != "" {
				out = append(out, lang.RawImport{Spec: "import " + strings.Join(strings.Fields(clause), " "), Module: module})
			}
			continue
		}
		prefix = strings.TrimSuffix(compact(prefix), ".")
		for _, sel := range strings.Split(strings.TrimSuffix(strings.TrimSpace(selectors), "}"), ",") {
			sel = strings.TrimSpace(sel)
			name, renamed := sel, ""
			for _, arrow := range []string{"=>", " as "} {
				if n, r, ok := strings.Cut(sel, arrow); ok {
					name, renamed = strings.TrimSpace(n), strings.TrimSpace(r)
					break
				}
			}
			if name == "" || renamed == "_" {
				continue
			}
			if module := wildcard(prefix + "." + name); module != "" {
				out = append(out, lang.RawImport{Spec: "import " + prefix + "." + sel, Module: module})
			}
		}
	}
	return out
}

// wildcard normalizes an import path's wildcard to ".*": _, * and given (Scala 3's
// "import all given instances", which come from the same package) all import from
// the package rather than name one member.
func wildcard(path string) string {
	i := strings.LastIndex(path, ".")
	if i <= 0 {
		return "" // an import of a bare name has nothing to resolve
	}
	switch last := path[i+1:]; {
	case last == "_" || last == "*" || last == "given" || strings.HasPrefix(last, "given "):
		return path[:i] + ".*"
	}
	return path
}

// splitTop splits at the commas outside braces: "a.{B, C}, d.E" is two clauses.
func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}

// compact drops the whitespace a path may be written with across lines.
func compact(s string) string { return strings.Join(strings.Fields(s), "") }
