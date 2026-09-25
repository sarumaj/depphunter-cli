---
id: REQ-PS-006
uuid: 0c2abe76-2c86-48b2-b32d-544000a03438
title: Module manifest dependencies
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

The PowerShell plugin **shall** read a module manifest's (`.psd1`)
`RequiredModules` as module dependencies, and its `RootModule`, `NestedModules`
and `ScriptsToProcess` as file dependencies relative to the manifest, ignoring
commented-out entries.

## Rationale

The manifest is where a module declares what it loads and what it needs; it is
the module's own dependency list.

## Acceptance criteria

1. `RootModule = 'Tools.psm1'` in `tools/Tools/Tools.psd1` resolves to
   `tools/Tools/Tools.psm1`.
2. `NestedModules` `Private/Helpers.ps1` and `ScriptsToProcess` `init.ps1`
   resolve to the files beside the manifest.
3. An entry inside a `#` comment is not read.
