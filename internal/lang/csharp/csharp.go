// Package csharp analyses C#. `using` directives name namespaces, which resolve to
// project folders by the MSBuild convention (root namespace + folder path), to NuGet
// packages from PackageReference / Directory.Packages.props (longest package id that
// prefixes the namespace, case-insensitively), or to the .NET base library.
//
// Parsing uses the statement scanner in scan.go: the pure-Go tree-sitter C# grammar
// took 24 s for Serilog's 216 files (8 s for one 59 KB file), while usings and
// declarations need no full parse.
package csharp

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecoNuGet  = "nuget"
	ecoDotnet = "dotnet"
)

type Plugin struct{}

func (Plugin) Name() string             { return "csharp" }
func (Plugin) Version() int             { return 1 }
func (Plugin) Claims(f *scan.File) bool { return strings.HasSuffix(f.Path, ".cs") && !f.Binary }
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecoNuGet, Name: "NuGet"},
		{ID: ecoDotnet, Name: ".NET base library", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

func (Plugin) Extract(f *scan.File, src []byte) (*lang.Extraction, error) {
	ex := &lang.Extraction{}
	var syms lang.SymbolSet
	for _, st := range scanStatements(string(src)) {
		switch {
		case (st.scope == "" || st.scope == "namespace") && usingDecl.MatchString(st.text):
			if ns := usingNamespace(st.text); ns != "" {
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: st.text, Module: ns, Line: st.line})
			}
		case typeDecl.MatchString(st.text):
			m := typeDecl.FindStringSubmatch(st.text)
			kind := m[1]
			if strings.HasPrefix(kind, "record") {
				kind = "record"
			}
			syms.Add(m[2], kind, st.line)
		case delegDecl.MatchString(st.text):
			syms.Add(delegDecl.FindStringSubmatch(st.text)[1], "delegate", st.line)
		case st.scope == "type":
			// A member named like its type is a constructor, not a method.
			if m := method.FindStringSubmatch(st.text); m != nil && !notNames[m[1]] && m[1] != st.owner {
				syms.Add(st.owner+"."+m[1], "method", st.line)
			}
		}
	}
	ex.Symbols = syms.List()
	return ex, nil
}

// usingNamespace extracts the namespace from "global using static A.B;",
// "using Alias = A.B;" and the like.
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
