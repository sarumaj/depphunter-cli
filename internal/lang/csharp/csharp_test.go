package csharp

import (
	"context"
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo: two projects (one with an explicit RootNamespace), central package
// management and a PackageReference with a <Version> element.
func analyse(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	const root = "testdata/repo"
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := lang.Analyze(context.Background(), Plugin{}, root, files)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestResolution(t *testing.T) {
	got := map[string]lang.Target{}
	for _, im := range analyse(t)["src/MyApp.Web/Program.cs"].Imports {
		got[im.Spec] = im.Target
	}
	want := map[string]lang.Target{
		"using System":                       {Ecosystem: "dotnet", Package: "System"},
		"using System.Collections.Generic":   {Ecosystem: "dotnet", Package: "System.Collections"},
		"using static System.Math":           {Ecosystem: "dotnet", Package: "System.Math"},
		"using Json = Newtonsoft.Json.Linq":  {Ecosystem: "nuget", Package: "Newtonsoft.Json", Version: "13.0.3"},
		"using MyApp.Core.Services":          {Local: "src/MyApp.Core/Services"},
		"using MyApp.Core.Missing":           {Local: "src/MyApp.Core"},
		"using MyApp.Web.Controllers":        {Local: "src/MyApp.Web/Controllers"},
		"global using Serilog.Sinks.Console": {Ecosystem: "nuget", Package: "Serilog.Sinks.Console", Version: "5.0.1"},
		"using Serilog":                      {Ecosystem: "nuget", Package: "Serilog", Version: "3.1.1"},
		"using Microsoft.Extensions.Hosting": {Ecosystem: "nuget", Package: "Microsoft.Extensions.Hosting", Version: "8.0.0"},
		"using Microsoft.AspNetCore.Builder": {Ecosystem: "dotnet", Package: "Microsoft.AspNetCore"},
		"using Dapper":                       {Ecosystem: "nuget", Package: "Dapper", Unresolved: true},
	}
	for spec, w := range want {
		if g, ok := got[spec]; !ok {
			t.Errorf("%s: not captured", spec)
		} else if g != w {
			t.Errorf("%s: got %+v, want %+v", spec, g, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d imports, want %d: %v", len(got), len(want), got)
	}
}

func TestSymbols(t *testing.T) {
	got := map[string]string{}
	for _, s := range analyse(t)["src/MyApp.Web/Program.cs"].Symbols {
		got[s.Name] = s.Kind
	}
	want := map[string]string{
		"Program": "class", "Program.Main": "method", "IService": "interface", "Person": "record",
		"Pt": "struct", "Mode": "enum", "Handler": "delegate",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
