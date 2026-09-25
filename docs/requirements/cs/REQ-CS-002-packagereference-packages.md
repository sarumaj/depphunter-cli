---
id: REQ-CS-002
uuid: 5e363ea9-3182-4b3f-9df6-9a136821eed3
title: NuGet packages from PackageReference
scope: cs
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The C# plugin **shall** read every `<PackageReference>` of every `.csproj`, with
its version from the `Version` attribute or a `<Version>` child element, and
**shall** resolve a namespace to the NuGet package whose id is the longest one
equal to or a prefix of the namespace; a namespace no package or project claims
**shall** go to the .NET base library when it starts with `System`, `Microsoft`
or `Windows`, and otherwise be reported unresolved.

## Rationale

NuGet package ids and the namespaces they provide usually share a prefix
(`Serilog.Sinks.Console`), so the longest matching id is the package that
provides a namespace.

## Acceptance criteria

1. `global using Serilog.Sinks.Console` resolves to package
   `Serilog.Sinks.Console` `5.0.1` from a `<Version>` element.
2. `using Microsoft.Extensions.Hosting` resolves to the referenced NuGet
   package, not the base library.
3. `using Microsoft.AspNetCore.Builder` with no such package resolves to
   ecosystem `dotnet`, package `Microsoft.AspNetCore`.
