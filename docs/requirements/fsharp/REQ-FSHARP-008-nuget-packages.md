---
id: REQ-FSHARP-008
uuid: 8f5f9902-12ff-4d24-8a36-0f9cb9e893ea
title: NuGet packages shared with C#
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

F# and C# **shall** read NuGet packages from one shared reader
(internal/lang/nuget): `<PackageReference>` and `<GlobalPackageReference>`
items of `.csproj`, `.fsproj`, `.vbproj` and `Directory.Build.props`,
central versions of `Directory.Packages.props`, Paket's files, and
`packages.lock.json` (resolved versions pin, with the requested range, and
answer `--resolve-depth`). A package id **shall** be compared
case-insensitively and spelled as a lock file records it, else as first
written, so C# and F# references of one package are one node. Pin rules:
a lock pins; a PackageReference version pins when exact (C#'s rule); a
Paket constraint pins when it is `= x`, `== x` or a bare version, shown
without the operator, and floats as written otherwise, or when absent;
`#r "nuget: Id, 1.2.3"` pins an exact version, a lock's version pins it
without one, and no version floats. An open **shall** resolve, when no
project file declares it, to: a script's `#r "nuget:"` package, a
package a small table names (`Swensen.Unquote` is `Unquote`), the
declared package whose id is the longest prefix of it, unless a declared
id extends a longer prefix of it (the shortest such id: `Fake.Core` is
`Fake.Core.Target`; not for `System`, `Microsoft` and `Windows`
namespaces), FSharp.Core for `Microsoft.FSharp.*`, the `FSharp.Core`,
`FSharp.Collections`, `FSharp.Control`-style short forms and the
modules F# opens by default (`Checked`, `Printf`), the .NET base library
for `System.*`, `Microsoft.*` and `Windows.*`, else an unresolved
package named by its first two segments. FSharp.Core **shall** be a NuGet
package (versioned by what declares or locks it, else floating with the
SDK), not part of the hidden .NET base library island.

## Rationale

FSharp.Core ships as a NuGet package with its own versions and advisories;
hiding it with the base library would hide a real dependency.

## Acceptance criteria

1. A C# project and an F# project referencing Newtonsoft.Json (spelled
   `newtonsoft.json` and `Newtonsoft.Json`, versioned centrally) give one
   NuGet node, 13.0.3 pinned, with edges from the `.cs`, the `.fs` and the
   `.fsproj`.
2. `open Microsoft.FSharp.Collections` and `open FSharp.Control` resolve
   to FSharp.Core 8.0.400 (locked); `open Fake.Core.TargetOperators` to
   `Fake.Core.Target`; `open Mystery.Lib` is unresolved `Mystery.Lib`.
3. `#r "nuget: Serilog, 3.*"` floats; `#r "nuget: Expecto"` is pinned by
   the lock; `#r "packages/Argu/.../Argu.dll"` is the Argu package.
