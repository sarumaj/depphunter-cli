---
id: REQ-PS-001
uuid: 3fef345b-7506-45ae-91c1-c123046aa6fd
title: using module statements
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

The PowerShell plugin **shall** record every `using module <x>` statement in
`.ps1`, `.psm1` and `.psd1` files as a dependency on the module or module file
`<x>`.

## Rationale

`using module` loads a module at parse time and is the way classes are shared
between modules.

## Acceptance criteria

1. `using module ../tools/Tools/Tools.psm1` in `scripts/deploy.ps1` resolves to
   `tools/Tools/Tools.psm1`.
