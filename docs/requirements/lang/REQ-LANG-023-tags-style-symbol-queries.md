---
id: REQ-LANG-023
uuid: 24e433f9-752f-42ca-bd92-54cc3476cf1a
title: Tags-style symbol queries
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Tree-sitter plugins **shall** extract definitions through tags-style queries,
one query per grammar that captures imports and named definitions by kind.

## Rationale

Declarative queries keep each language's extraction short and reviewable and
follow the upstream tree-sitter tags convention.

## Acceptance criteria

1. The JavaScript/TypeScript and Python plugins each compile one query per
   grammar with captures for imports and definitions.
2. A capture name `def.<kind>` yields a symbol of that kind.
