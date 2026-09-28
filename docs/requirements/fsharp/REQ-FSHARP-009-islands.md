---
id: REQ-FSHARP-009
title: NuGet, .NET and Paket islands
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The F# plugin **shall** declare the C# plugin's islands with the same ids
and names - `nuget` ("NuGet") and `dotnet` (".NET base library",
hidden) - and a new `paket` island ("Paket git, GitHub and HTTP
sources"); every package it emits **shall** belong to one of them. F#
packages **shall** reach OSV as NuGet, Trivy's `nuget` reports and the
NuGet index of `--online` by the `nuget` id; `paket` **shall** be a
private-pattern prefix and have no OSV ecosystem (a GitHub or git source
locked to a full commit on a public forge is asked about by that commit,
REQ-FND-026).

## Rationale

Islands merge by id: a package C# and F# share is one node.

## Acceptance criteria

1. The plugin declares exactly `nuget`, `dotnet` (hidden) and `paket`,
   and the fixture emits nothing else.
