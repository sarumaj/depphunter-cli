---
id: REQ-LANG-003
uuid: 1dd5d8cd-8308-4956-863a-ae264b11c363
title: Top-level symbol extraction
scope: lang
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §4
verification:
  - unit
---

## Statement

A plugin **shall** extract from each claimed file its top-level symbols, each
with a name, a kind (for example `func`, `method`, `type`, `class`, `var`,
`const`) and the 1-based line of its definition, and symbol names **shall** be
unique within the file.

## Rationale

Symbols are what an expanded file shows; unique names are needed because symbol
node identifiers are derived from them.

## Acceptance criteria

1. Each extracted symbol has a non-empty name, a kind and a line.
2. Two definitions with the same name in one file yield two distinct symbol
   names.

## Notes

Repeats are made unique by suffixing `@<line>` (for example two Go `init`
functions yield `init` and `init@18`).
