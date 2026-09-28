// Package csharp analyzes C#. `using` directives name namespaces, which resolve to
// project folders by the MSBuild convention (root namespace + folder path), to NuGet
// packages (longest package id that prefixes the namespace, case-insensitively) as
// internal/lang/nuget reads them for C# and F# alike - PackageReference,
// Directory.Packages.props, Paket, packages.lock.json - or to the .NET base library.
//
// Parsing uses the statement scanner in scan.go: the pure-Go tree-sitter C# grammar
// took 24 s for Serilog's 216 files (8 s for one 59 KB file), while usings and
// declarations need no full parse.
package csharp

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/nuget"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Implements: REQ-CS-008
type Plugin struct{}

func (Plugin) Name() string             { return "csharp" }
func (Plugin) Version() int             { return 1 }
func (Plugin) Claims(f *scan.File) bool { return strings.HasSuffix(f.Path, ".cs") && !f.Binary }
func (Plugin) Ecosystems() []lang.Ecosystem {
	return nuget.Ecosystems()
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

// Implements: REQ-CS-005
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	for _, statement := range scanStatements(string(source)) {
		switch {
		case (statement.scope == "" || statement.scope == "namespace") && usingDeclaration.MatchString(statement.text):
			if namespace := usingNamespace(statement.text); namespace != "" {
				extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: statement.text, Module: namespace, Line: statement.line})
			}
		case typeDeclaration.MatchString(statement.text):
			m := typeDeclaration.FindStringSubmatch(statement.text)
			kind := m[1]
			if strings.HasPrefix(kind, "record") {
				kind = "record"
			}
			symbols.Add(m[2], kind, statement.line)
		case delegateDeclaration.MatchString(statement.text):
			symbols.Add(delegateDeclaration.FindStringSubmatch(statement.text)[1], "delegate", statement.line)
		case statement.scope == "type":
			// A member named like its type is a constructor, not a method.
			if m := method.FindStringSubmatch(statement.text); m != nil && !notNames[m[1]] && m[1] != statement.owner {
				symbols.Add(statement.owner+"."+m[1], "method", statement.line)
			}
		}
	}
	extraction.Symbols = symbols.List()
	return extraction, nil
}

// usingNamespace extracts the namespace from "global using static A.B;",
// "using Alias = A.B;" and the like.
//
// Implements: REQ-CS-001
func usingNamespace(text string) string {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ";"))
	s = strings.TrimSpace(strings.TrimPrefix(s, "global "))
	s = strings.TrimSpace(strings.TrimPrefix(s, "using"))
	s = strings.TrimSpace(strings.TrimPrefix(s, "static "))
	if _, rhs, ok := strings.Cut(s, "="); ok {
		s = strings.TrimSpace(rhs)
	}
	if i := strings.IndexAny(s, "<("); i >= 0 {
		s = s[:i] // generic type aliases: using X = A.B<C>;
	}
	return strings.Join(strings.Fields(s), "")
}
