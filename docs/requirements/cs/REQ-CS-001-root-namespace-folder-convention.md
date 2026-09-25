---
id: REQ-CS-001
uuid: 82926835-d262-457b-8f5a-15f644388ddf
title: Namespaces resolve by root namespace and folder
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

The C# plugin **shall** resolve a `using` directive whose namespace equals or
lies under a project's root namespace to the deepest existing directory of C#
files along the namespace path below that project's directory, where the root
namespace is the project's `<RootNamespace>`, else its `<AssemblyName>`, else
the `.csproj` file name, and where the project with the longest matching root
namespace is tried first.

## Rationale

C# namespaces name no files. The MSBuild convention (root namespace plus folder
path) is how the IDE and most projects lay them out, so it is how a namespace is
found in the repository.

## Acceptance criteria

1. `using MyApp.Core.Services` resolves to `src/MyApp.Core/Services` where
   `MyApp.Core.csproj` sets `<RootNamespace>MyApp.Core</RootNamespace>`.
2. `using MyApp.Core.Missing` resolves to `src/MyApp.Core`, the deepest folder
   that exists.
3. `using MyApp.Web.Controllers` resolves to `src/MyApp.Web/Controllers` for a
   project without `<RootNamespace>`.
4. `global using`, `using static` and alias forms (`using Json = A.B;`) are
   read, and their namespace is resolved.
