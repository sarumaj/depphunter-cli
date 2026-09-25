---
id: REQ-PS-004
uuid: 939fd945-89ef-4b76-b461-f51cb2553862
title: Script paths relative to the script
scope: ps
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** resolve a script or module path relative to the
directory of the referring file, **shall** replace `$PSScriptRoot`,
`${PSScriptRoot}` and `$($PSScriptRoot)` with that directory, accept both `/`
and `\` as separators, and try the path as written, with `.psd1`, `.psm1` and
`.ps1` appended, and as a module folder (`<dir>/<dir>.psd1`,
`<dir>/<dir>.psm1`); a path using any other variable, or an absolute path,
**shall** resolve to nothing.

## Rationale

`$PSScriptRoot` is the idiomatic anchor for sibling files; other variables and
absolute paths point outside what the analysis can see.

## Acceptance criteria

1. `. "$PSScriptRoot\common.ps1"` resolves to `scripts/common.ps1`.
2. `Import-Module $PSScriptRoot/lib/Util.psm1` resolves to
   `scripts/lib/Util.psm1`.
3. `. $env:HOME/profile.ps1` resolves to nothing.
