---
id: REQ-PS-005
uuid: 9601fd70-002a-4d5f-8b2e-ca0d77a84fb4
title: #Requires -Modules declarations
scope: ps
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** record each module named in a
`#Requires -Modules` comment as a dependency, reading plain names, quoted names
and `@{ModuleName=…; ModuleVersion=…|RequiredVersion=…}` tables.

## Rationale

`#Requires -Modules` is a declared dependency that PowerShell enforces before
the script runs.

## Acceptance criteria

1. `#Requires -Modules Az.Storage` yields a PowerShell Gallery dependency on
   `Az.Storage`.
2. A `#Requires` table with `RequiredVersion = '6.1.0'` yields `Az.Resources` at
   `6.1.0`.
