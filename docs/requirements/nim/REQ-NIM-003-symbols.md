---
id: REQ-NIM-003
title: Symbols
scope: nim
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

A module's top-level `proc`s and `func`s (kind func), `method`s, `iterator`s,
`converter`s, `template`s and `macro`s, its type definitions (an object or
`ref object` is a class, a `concept` an interface, an `enum` an enum, a
`distinct` type, tuple, procedure type or alias a type) and its `const`,
`let` and `var` names **shall** be its symbols, named without the export
marker `*` and without backquotes (`` `$` `` is `$`); a NimScript `task`
**shall** be a symbol of kind task and a `.nimble` file's package one of
kind package. Declarations inside routine bodies and object fields are not
symbols.

## Rationale

Top-level declarations are what other modules import.

## Acceptance criteria

1. The fixture's `src/shop.nim` has exactly the symbols listed in the test.
