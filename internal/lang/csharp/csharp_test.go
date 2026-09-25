package csharp

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: two projects (one with an explicit RootNamespace), central package
// management and a PackageReference with a <Version> element.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-CS-001, REQ-CS-002, REQ-CS-003, REQ-CS-007
func TestResolution(t *testing.T) {
	langtest.CheckImports(t, analyze(t)["src/MyApp.Web/Program.cs"], map[string]lang.Target{
		"using System":                       {Ecosystem: "dotnet", Package: "System"},
		"using System.Collections.Generic":   {Ecosystem: "dotnet", Package: "System.Collections"},
		"using static System.Math":           {Ecosystem: "dotnet", Package: "System.Math"},
		"using Json = Newtonsoft.Json.Linq":  {Ecosystem: "nuget", Package: "Newtonsoft.Json", Version: "13.0.3", Pinned: true},
		"using MyApp.Core.Services":          {Local: "src/MyApp.Core/Services"},
		"using MyApp.Core.Missing":           {Local: "src/MyApp.Core"},
		"using MyApp.Web.Controllers":        {Local: "src/MyApp.Web/Controllers"},
		"global using Serilog.Sinks.Console": {Ecosystem: "nuget", Package: "Serilog.Sinks.Console", Version: "5.0.1", Pinned: true},
		"using Serilog":                      {Ecosystem: "nuget", Package: "Serilog", Version: "3.1.1", Pinned: true},
		"using Microsoft.Extensions.Hosting": {Ecosystem: "nuget", Package: "Microsoft.Extensions.Hosting", Version: "8.0.0", Pinned: true},
		"using Microsoft.AspNetCore.Builder": {Ecosystem: "dotnet", Package: "Microsoft.AspNetCore"},
		// A wildcard version moves with every restore.
		"using Dapper": {Ecosystem: "nuget", Package: "Dapper", Version: "2.*"},
	})
}

// Verifies: REQ-CS-005
func TestSymbols(t *testing.T) {
	langtest.CheckSymbols(t, analyze(t)["src/MyApp.Web/Program.cs"], map[string]string{
		"Program": "class", "Program.Main": "method", "IService": "interface", "Person": "record",
		"Pt": "struct", "Mode": "enum", "Handler": "delegate",
	})
}
