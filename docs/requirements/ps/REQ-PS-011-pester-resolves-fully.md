---
id: REQ-PS-011
uuid: efc62bd4-deb9-4996-a366-056d9f7d4d26
title: Pester resolves without unresolved modules
scope: ps
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - e2e
---

## Statement

The PowerShell plugin **shall** analyze the Pester repository with no unresolved
dependency.

## Rationale

Pester was the PowerShell reference project in M4.

## Acceptance criteria

1. An analysis of Pester reports 0 unresolved dependencies.

## Notes

Measured in the design log (M4). No automated test runs against Pester.
