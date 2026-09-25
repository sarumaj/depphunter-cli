---
id: REQ-MAP-035
uuid: 920ba664-64d4-4c67-8539-24cbf1547b84
title: Packages hidden with the files importing them
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The system **shall** hide an external package when no visible file imports it,
and **shall** hide an island when none of its packages remains visible; a
directory left without visible files **shall** be hidden too.

## Rationale

A package only matters through what uses it; filtering its importers away leaves
it meaningless.

## Acceptance criteria

1. In a mixed Go/TS/Python repository, hiding TypeScript removes the npm island
   when only TypeScript files used it.
2. A package imported by both a visible and a hidden file stays.
