---
id: REQ-RACKET-003
uuid: 094c18f1-9f5a-4f02-b6a9-39829b13c5e6
title: Symbols
scope: racket
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** report the module-level definitions of a module as
symbols: `define` of a procedure header (func, curried too) or a value (var;
func for a `lambda`, `λ` or `case-lambda`, class for a `class` form, whose
`define/public` and other method definitions are `Class.method` methods,
interface for an `interface`), `define-values`, `define-syntax`,
`define-syntax-rule` and the other macro forms (macro), `struct` and
`define-struct` (struct), `define-type` (type), and a `module`, `module*` or
`module+` submodule (module, once per name). Definitions in a `module` or
`module*` submodule **shall** be named `sub.name`; those in a `module+`
submodule (tests, main) and anything a data file holds **shall** not be
symbols.

## Rationale

Top-level definitions are what other modules use; test submodules'
helpers are not.

## Acceptance criteria

1. `main.rkt` gives exactly its definitions, `cart%.total` and
   `cart%.log-line` methods, the submodules `inner`, `test` and `main`,
   `inner.x` and `main.entry`, and not `helper` of its `module+ test`.
