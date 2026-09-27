package analyze

import (
	"context"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/csharp"
	"github.com/sarumaj/depphunter-cli/internal/lang/fsharp"
)

// A C# project and an F# project both referencing Newtonsoft.Json (spelled
// differently, versioned centrally) put one NuGet node on the map, with edges from
// the C# using, the F# open and the .fsproj's PackageReference; the F# project's
// ProjectReference to the C# project is a file edge.
//
// Verifies: REQ-FSHARP-008, REQ-CS-004
func TestOneNuGetNodeAcrossCSharpAndFSharp(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{
		"Directory.Packages.props": `<Project><ItemGroup><PackageVersion Include="Newtonsoft.Json" Version="13.0.3" /></ItemGroup></Project>`,
		"Core/Core.csproj":         `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="newtonsoft.json" /></ItemGroup></Project>`,
		"Core/Thing.cs":            "using Newtonsoft.Json;\n\nnamespace Core;\n\npublic class Thing { }\n",
		"App/App.fsproj": `<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <Compile Include="Program.fs" />
    <ProjectReference Include="..\Core\Core.csproj" />
    <PackageReference Include="Newtonsoft.Json" />
  </ItemGroup>
</Project>
`,
		"App/Program.fs": "module App.Program\n\nopen Newtonsoft.Json\n\nlet json x = JsonConvert.SerializeObject x\n",
	})
	g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{csharp.Plugin{}, fsharp.Plugin{}}})
	if err != nil {
		t.Fatal(err)
	}
	var nuget []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindPackage && n.Parent == graph.EcosystemID("nuget") {
			nuget = append(nuget, n.Name)
		}
	}
	// FSharp.Core is the SDK's implicit reference of the F# project.
	if len(nuget) != 2 || !contains(nuget, "Newtonsoft.Json") || !contains(nuget, "FSharp.Core") {
		t.Fatalf("nuget packages %v, want Newtonsoft.Json and FSharp.Core", nuget)
	}
	id := graph.PackageID("nuget", "Newtonsoft.Json")
	from := map[string]bool{}
	for _, e := range g.Edges {
		if e.To == id {
			from[e.From] = true
		}
	}
	for _, f := range []string{"Core/Thing.cs", "App/Program.fs", "App/App.fsproj"} {
		if !from[graph.FileID(f)] {
			t.Errorf("%s has no edge to %s", f, id)
		}
	}
	if n := byID(g)[id]; n.Version != "13.0.3" || n.Floating {
		t.Errorf("version %q floating %v, want 13.0.3 pinned", n.Version, n.Floating)
	}
	ref := false
	for _, e := range g.Edges {
		ref = ref || e.From == graph.FileID("App/App.fsproj") && e.To == graph.FileID("Core/Core.csproj")
	}
	if !ref {
		t.Error("App.fsproj has no edge to Core.csproj")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
