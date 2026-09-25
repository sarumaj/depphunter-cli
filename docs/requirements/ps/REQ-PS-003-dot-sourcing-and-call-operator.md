---
id: REQ-PS-003
uuid: dad267c0-ba6e-4425-ad91-494a15f081dc
title: Dot-sourced and invoked scripts
scope: ps
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PowerShell plugin **shall** record a dot-sourced (`. path`) or call-operator
(`& path`) invocation of a `.ps1`, `.psm1` or `.psd1` file as a dependency on
that file.

## Rationale

Dot-sourcing and the call operator run another script, which makes it a
dependency of the caller.

## Acceptance criteria

1. `. ./helpers.ps1` in `scripts/deploy.ps1` resolves to `scripts/helpers.ps1`.
2. `& "$PSScriptRoot/tasks/build.ps1"` resolves to `scripts/tasks/build.ps1`.
