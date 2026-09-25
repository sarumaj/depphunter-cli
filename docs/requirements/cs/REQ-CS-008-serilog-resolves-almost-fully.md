---
id: REQ-CS-008
uuid: b4df6a1b-ae1e-40ea-81d8-8cf30000e7e4
title: Serilog resolves with one unresolved dependency
scope: cs
type: non-functional
priority: should
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - e2e
---

## Statement

The C# plugin **should** analyze the Serilog repository with no more than 1
unresolved dependency.

## Rationale

Serilog was the C# reference project in M4.

## Acceptance criteria

1. An analysis of Serilog reports at most 1 unresolved dependency.

## Notes

Measured in the design log (M4). No automated test runs against Serilog.
