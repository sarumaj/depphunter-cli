---
id: REQ-CS-004
uuid: d124a026-6e41-49fa-a7cb-c7e61693547f
title: Case-insensitive NuGet package ids
scope: cs
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The C# plugin **shall** compare NuGet package ids case-insensitively.

## Rationale

NuGet ids are case-insensitive: the `xunit` package provides the `Xunit.*`
namespaces.

## Acceptance criteria

1. A namespace `Xunit.Abstractions` resolves to a referenced package `xunit`.
2. A `<PackageReference Include="serilog" />` takes the central version of
   `<PackageVersion Include="Serilog" … />`.

## Notes

Partial: the namespace-to-package match is case-insensitive (criterion 1), but
the central version lookup (criterion 2) and the package map are keyed by the id
as written. No test covers a case difference.
