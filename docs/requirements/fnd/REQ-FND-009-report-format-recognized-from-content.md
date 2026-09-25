---
id: REQ-FND-009
uuid: 468e7e11-9eba-4ead-9a48-061d2ae586af
title: Report format recognized from content
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** recognize the format of a report from the shape of its
content, not from its file name, and **shall** reject content that is not a
report it recognizes.

## Rationale

Pipelines name reports anything.

## Acceptance criteria

1. Each supported tool's report is recognized without being named.
2. A JSON document of another shape, an empty file and a non-JSON file are
   rejected with an error.
