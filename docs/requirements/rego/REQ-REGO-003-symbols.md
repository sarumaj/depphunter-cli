---
id: REQ-REGO-003
title: Symbols
scope: rego
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A Rego file's package (`a.b`, bracketed segments unquoted) **shall** be a
symbol of kind `package`, and each rule (`allow`, `deny contains msg if`,
`deny[msg]`, `default x :=`, ref heads `a.b.c :=`) of kind `rule` and each
function (`f(x) :=`) of kind `function`, a name defined several times
once.

## Rationale

Incremental rule definitions are one rule; the package is what other
files import.

## Acceptance criteria

1. `deny.rego`, `lib/util.rego` and the bracketed package have the listed
   symbols.
