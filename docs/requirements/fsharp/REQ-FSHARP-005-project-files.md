---
id: REQ-FSHARP-005
title: Project files
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every `<Compile Include>` item of a `.fsproj` **shall** be an import of
that file (in compile order, `;`-separated lists split, backslashes and
`$(MSBuildThisFileDirectory)` read), a file under `paket-files/` an
import of the Paket dependency it came from; every `<ProjectReference>`
an import of that project file (C# projects too); every
`<PackageReference>` (`Include` or `Update`) an import of the NuGet
package; every `<Reference>` an import of the assembly (a file of the
repository, a package's assembly under `packages/`, or a framework
assembly). An SDK-style project **shall** also import FSharp.Core, the
SDK's implicit reference, unless it references FSharp.Core itself or sets
`DisableImplicitFSharpCoreReference`. An item naming another MSBuild
property or a wildcard, or a missing file, **shall** be dropped.

## Rationale

The compile list is the project's structure; FSharp.Core is a real NuGet
package every F# project depends on, named or not.

## Acceptance criteria

1. `Shop.Domain.fsproj`'s items resolve to its five sources, the Paket-linked
   `Globbing.fs` to `github.com/fsharp/FAKE` pinned by `paket.lock`, and
   `$(GeneratedDir)\Version.fs` is dropped.
2. `Shop.Web.fsproj` imports `Shop.Domain.fsproj`, `Legacy.csproj` and
   FSharp.Core (implicit); `Shop.Tests.fsproj` with
   `DisableImplicitFSharpCoreReference` imports no FSharp.Core.
