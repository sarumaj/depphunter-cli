package fsharp

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/nuget"
)

// Import kinds of the manifests (RawImport.Name's first line).
const (
	kindCompile   = "compile"   // <Compile Include="x.fs" />
	kindProject   = "project"   // <ProjectReference Include="..\x\x.fsproj" />
	kindPackage   = "package"   // <PackageReference Include="X" /> (second line: Paket group)
	kindImplicit  = "implicit"  // the SDK's implicit FSharp.Core reference
	kindRemote    = "remote"    // a Paket github/gist/git/http dependency (kind, ref)
	kindPaketFile = "paketfile" // paket.references File: x.fs
)

// extractProject reads an .fsproj: its Compile items in order (the compile order),
// project references and package references. An SDK-style project references
// FSharp.Core implicitly unless it says DisableImplicitFSharpCoreReference or
// references FSharp.Core itself.
//
// Implements: REQ-FSHARP-005
func extractProject(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	p := nuget.ReadProject(src)
	core := false
	for _, it := range p.Items {
		if it.Include == "" {
			continue
		}
		switch it.Kind {
		case "Compile":
			if it.Update {
				continue
			}
			for _, inc := range strings.Split(it.Include, ";") {
				if inc = strings.TrimSpace(inc); inc != "" {
					ex.Imports = append(ex.Imports, lang.RawImport{Spec: inc, Module: inc, Name: kindCompile, Line: it.Line})
				}
			}
		case "ProjectReference":
			for _, inc := range strings.Split(it.Include, ";") {
				if inc = strings.TrimSpace(inc); inc != "" {
					ex.Imports = append(ex.Imports, lang.RawImport{Spec: inc, Module: inc, Name: kindProject, Line: it.Line})
				}
			}
		case "Reference": // an assembly: a file of the repository or of the framework
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: it.Include, Module: it.Include, Name: kindDLL, Line: it.Line})
		case "PackageReference":
			core = core || strings.EqualFold(it.Include, "FSharp.Core")
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: it.Include, Module: it.Include, Name: kindPackage, Line: it.Line})
		}
	}
	if p.Sdk != "" && !core && !strings.EqualFold(p.Properties["DisableImplicitFSharpCoreReference"], "true") {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "FSharp.Core (implicit)", Module: "FSharp.Core", Name: kindImplicit, Line: 1})
	}
	return ex
}

// extractDependencies makes every nuget, github, gist, git and http line of
// paket.dependencies an import.
//
// Implements: REQ-FSHARP-006
func extractDependencies(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	deps, _ := nuget.ParseDependencies(src)
	for _, d := range deps {
		spec := d.Text
		if d.Kind == "nuget" {
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: d.Name, Name: kindPackage + "\n" + d.Group, Line: d.Line})
			continue
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: d.Name, Name: kindRemote + "\n" + d.Kind + "\n" + d.Constraint, Line: d.Line})
	}
	return ex
}

// extractLock makes every entry of paket.lock an import: the packages (pinned at
// their locked versions) and the remote files and repositories.
//
// Implements: REQ-FSHARP-007
func extractLock(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	seen := map[string]bool{}
	for _, l := range nuget.ParseLock(src) {
		prefix := ""
		if !strings.EqualFold(l.Group, nuget.MainGroup) {
			prefix = l.Group + "/"
		}
		if l.Kind == "nuget" {
			spec := prefix + l.Name
			if !seen[spec] {
				seen[spec] = true
				ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: l.Name, Name: kindPackage + "\n" + l.Group, Line: l.Line})
			}
			continue
		}
		spec := prefix + l.Remote
		if l.Name != "" && !strings.HasSuffix(l.Remote, "/"+l.Name) {
			spec += "/" + l.Name
		}
		if seen[spec] {
			continue
		}
		seen[spec] = true
		remote := l.Remote
		if l.Kind == "http" && l.Name != "" && !strings.Contains(l.Remote, l.Name) {
			remote = strings.TrimRight(l.Remote, "/") + "/" + strings.TrimLeft(l.Name, "/")
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: remote, Name: kindRemote + "\n" + l.Kind + "\n", Line: l.Line})
	}
	return ex
}

// extractReferences reads a project's paket.references: the packages it uses, per
// group, and the remote files it links.
//
// Implements: REQ-FSHARP-006
func extractReferences(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	for _, r := range nuget.ParseReferences(src) {
		prefix := ""
		if !strings.EqualFold(r.Group, nuget.MainGroup) {
			prefix = r.Group + "/"
		}
		if r.File {
			ex.Imports = append(ex.Imports, lang.RawImport{Spec: prefix + "File: " + r.Name, Module: r.Name, Name: kindPaketFile, Line: r.Line})
			continue
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: prefix + r.Name, Module: r.Name, Name: kindPackage + "\n" + r.Group, Line: r.Line})
	}
	return ex
}
