---
id: REQ-LANG-007
uuid: f9f1a82b-98f3-47b6-aa8d-a03fe0356801
title: Only imported grammars linked
scope: lang
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The binary **shall** link only the tree-sitter grammars that a plugin imports.

## Rationale

Each grammar adds to the binary's size; linking all grammars the runtime ships
would grow it for languages nothing analyzes.

## Acceptance criteria

1. Grammars are imported individually per language package
   (`gotreesitter/grammars/<language>`) by the plugins that use them.
2. No package imports a bundle of all grammars.
