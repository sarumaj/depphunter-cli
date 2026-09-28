---
id: REQ-CUE-003
title: Symbols
scope: cue
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A CUE file's package clause (`package`), top-level definitions
(`#Name`, `_#Name`, as `type`) and top-level fields (identifiers or quoted
labels, optional `?` or required `!`, aliased `X=name`, the first label of
`a: b: c`, as `field`) **shall** be its symbols, each once; pattern,
dynamic and nested fields, `let` bindings and comprehensions **shall**
not.

## Rationale

A package's top level is what other packages refer to.

## Acceptance criteria

1. The fixture's `config/main.cue` has its package, definitions and
   top-level fields as symbols.
