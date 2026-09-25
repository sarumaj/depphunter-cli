---
id: REQ-CS-008
uuid: b4df6a1b-ae1e-40ea-81d8-8cf30000e7e4
title: Serilog resolves with one unresolved dependency
scope: cs
type: non-functional
priority: should
status: implemented
verification:
  - e2e
---

## Statement

The C# plugin **should** analyze the Serilog repository with no more than 1
unresolved dependency.

## Rationale

Serilog is the C# reference project.

## Acceptance criteria

1. An analysis of Serilog reports at most 1 unresolved dependency.

## Notes

A manual measurement; no automated test runs against Serilog.
