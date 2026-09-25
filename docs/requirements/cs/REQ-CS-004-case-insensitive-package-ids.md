---
id: REQ-CS-004
uuid: d124a026-6e41-49fa-a7cb-c7e61693547f
title: Case-insensitive NuGet package ids
scope: cs
type: functional
priority: must
status: implemented
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

The package map and the central version lookup are keyed by the lower-case id,
so two spellings of one id are one package, named as first written, and a
reference takes the central version whatever its case. A test covers it.
