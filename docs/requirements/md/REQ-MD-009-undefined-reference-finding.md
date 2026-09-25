---
id: REQ-MD-009
uuid: cae317ad-8e3e-49c7-9d90-959a7ea9d3dd
title: Undefined reference finding
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** report a full or collapsed reference link whose label no
reference definition in the same document defines, compared case-insensitively
and wherever in the document the definition appears, as a
`link/undefined-reference` finding of medium severity.

## Rationale

A reference link without a definition renders as literal text, which is a break
that reading the rendered page does not reveal.

## Acceptance criteria

1. `[a reference][missing]` without a `[missing]:` definition is reported.
2. `[another][spec]` with `[spec]: docs/SPEC.md` defined later in the document
   is not reported.
