---
id: REQ-PS-012
title: Install commands and PSDepend files
scope: ps
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** record the modules `Install-Module`,
`Install-PSResource`, `Save-Module` and `Save-PSResource` fetch (`-Name` or the
first positional argument, a comma-separated list; a name held in a variable
or holding a wildcard left out) as PowerShell Gallery requirements:
`-RequiredVersion` and a bare `-Version` naming one version, `-MinimumVersion`
a minimum and a bracketed `-Version` a NuGet range, `-Repository` the
repository they come from. It **shall** read a PSDepend file
(`requirements.psd1`, `*.depend.psd1`) as PSDepend does: each key a dependency
(`Name` or `Type::Name`), its value a version (`latest` or empty for the
newest, a NuGet range, else one version) or a table of `DependencyType`,
`Name`, `Version` and `Parameters` (`Repository`); a dependency without a type
is a `PSGalleryModule` unless its name holds a `/`, and only
`PSGalleryModule` and `PSGalleryNuget` dependencies are recorded. A module a
project manifest's `RequiredModules` declares keeps that declaration.

## Rationale

Build scripts and PSDepend files install most of what a PowerShell project's
tooling runs, and name no manifest.

## Acceptance criteria

1. `Install-Module -Name X -RequiredVersion v` pins X; `Install-Module A, B
   -MinimumVersion v -Repository R` records A and B at the minimum from R;
   `Install-PSResource -Version '[5.0,6.0)'` keeps the range; a module in a
   variable is not recorded.
2. A `requirements.psd1` records `latest`, bare, typed and renamed
   dependencies, and leaves out `PSDependOptions`, GitHub and git ones.

## Notes

The repository a module comes from is asked alone with `--online`
([REQ-SUP-078](../sup/REQ-SUP-078-powershell-repositories.md)).
