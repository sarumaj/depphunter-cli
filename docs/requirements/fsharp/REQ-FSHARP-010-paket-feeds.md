---
id: REQ-FSHARP-010
title: Paket feeds discovered
scope: fsharp
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

Index discovery **shall** record the NuGet feeds `paket.dependencies`
names in its `source` lines (every group's) and `paket.lock` names in
its NUGET `remote:` lines as repository NuGet sources (not trusted, like
`nuget.config`'s); a directory source and nuget.org itself (Paket
projects often still name its retired v2 API) **shall not** be recorded.

## Rationale

A Paket project names its private feeds in its Paket files, not in
`nuget.config`.

## Acceptance criteria

1. `source https://www.nuget.org/api/v2`, `source ./local-packages`, a
   Build group's `source https://nuget.pkg.example.com/acme/index.json`
   with credentials, and a lock's `remote: https://feed.internal/v3/index.json`
   record exactly the last two.

## Notes

Paket's credentials (`username:`/`password:` on a source line, `paket
config add-credentials`) are not read.
