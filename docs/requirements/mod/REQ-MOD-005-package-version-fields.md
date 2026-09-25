---
id: REQ-MOD-005
uuid: 5b807f0f-4e9b-4f00-ac12-1f9fbc893815
title: Package version fields
scope: mod
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §3
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

A package node **shall** be able to carry `version` (the version the project
resolves to), `requested` (the specifier a manifest asked for, when a lock file
resolved it to another version) and `floating` (a boolean marking a dependency
nothing fixes to one version).

## Rationale

A manifest range and the version a lock file resolved it to are both relevant to
a reader of the supply chain, and whether a dependency is pinned is a separate
fact from which version is in use.

## Acceptance criteria

1. A package pinned by a lock file carries its locked `version` and the
   manifest's `requested` range, and no `floating`.
2. A package declared only by a range carries that range as `version` and
   `floating: true`.

## Notes

When `floating` is set is specified in scope `sup`; this requirement covers the
members of the document only.
