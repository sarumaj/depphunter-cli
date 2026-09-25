---
id: REQ-PS-008
uuid: 0619baff-03b2-4ef7-b82d-0cf5ea4e1d2c
title: Built-in PowerShell modules
scope: ps
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** assign modules that ship with PowerShell or
Windows, from a fixed case-insensitive list, to the standard ecosystem
"PowerShell built-in modules" rather than to the PowerShell Gallery.

## Rationale

Modules such as `Microsoft.PowerShell.Utility`, `PSReadLine` and `PowerShellGet`
are part of the platform and are neither installed from the gallery nor
unresolved.

## Acceptance criteria

1. `Import-Module PSReadLine` and `Import-Module PowerShellGet` resolve to
   ecosystem `powershell`.
2. `RequiredModules` entry `PSReadLine` in a manifest resolves to ecosystem
   `powershell`.
