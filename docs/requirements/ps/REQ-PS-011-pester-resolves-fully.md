---
id: REQ-PS-011
uuid: efc62bd4-deb9-4996-a366-056d9f7d4d26
title: Pester resolves without unresolved modules
scope: ps
type: non-functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

The PowerShell plugin **shall** analyze the Pester repository with no unresolved
dependency.

## Rationale

Pester is the PowerShell reference project.

## Acceptance criteria

1. An analysis of Pester reports 0 unresolved dependencies.

## Notes

A manual measurement; no automated test runs against Pester.
