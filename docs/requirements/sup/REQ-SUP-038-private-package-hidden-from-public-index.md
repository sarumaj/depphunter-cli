---
id: REQ-SUP-038
uuid: 8d43a9a2-befb-402f-a56d-6581ffc12ac2
title: A private package is not named to a public index
scope: sup
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

The index client **shall not** name a private package to its ecosystem's public
index.

## Rationale

The public index does not answer for an internal package, and the request itself
says that the package exists.

## Acceptance criteria

1. A run with `--private 'corp.example/*'` names no corp.example package to a
   public proxy.
2. The report records such a question as declined for being private.
