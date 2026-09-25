---
id: REQ-PS-010
uuid: 94a2a944-36f4-425d-86aa-da2f6420e0cf
title: RequiredVersion pins, ModuleVersion is a minimum
scope: ps
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** mark a module requirement as pinned only when it
names a `RequiredVersion`; a `ModuleVersion` **shall** be reported as the
version without pinning it, and where a table carries both, `RequiredVersion`
**shall** win. An `Import-Module` of a module that a project manifest's
`RequiredModules` declares **shall** take that declaration's version and pin
state.

## Rationale

In PowerShell `RequiredVersion` names one version, while `ModuleVersion` is only
a minimum the installed module may exceed.

## Acceptance criteria

1. `@{ModuleName='PSScriptAnalyzer'; RequiredVersion='1.21.0'}` resolves to
   `1.21.0`, pinned.
2. `@{ModuleName='Pester'; ModuleVersion='5.3.0'}` resolves to `5.3.0`, not
   pinned (floating).
3. `Import-Module Pester` in a script of a repository whose manifest requires
   Pester `5.3.0` as a minimum resolves to `5.3.0`, not pinned.

## Notes

The general floating concept belongs to scope `sup`.
