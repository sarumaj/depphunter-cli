---
id: REQ-OCAML-003
title: Definitions extracted
scope: ocaml
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract a file's definitions at structure level: `let`
and `let rec` bindings and their `and`s (a function when parameters or
`fun`/`function` follow, else a value; operators by their symbol), `type`
definitions (not extensions `+=`), `module` and `module type`, `exception`,
`external`, `class` with its `method`s, and `val` in signatures (a function
when its type has an arrow). Definitions inside a module's `struct` or
`sig`, `include struct`, and `[%%ext ...]` blocks **shall** be named after
their module (`Id.next`); nothing bound in an expression is a definition. A
name defined twice (shadowing, a signature and its structure) **shall** be
one symbol. dune stanzas (`library shop`, `executable main`, `test x`),
dune-project packages and dune-workspace contexts are components;
ocamllex rules and grammar rules are rules.

## Rationale

A file's definitions are what the map shows in it; dune's components are
what its build file defines.

## Acceptance criteria

1. `lib/cart.ml` defines types `item` and `t`, exception `Empty`, values,
   functions, module `Id` with `Id.t`, `Id.counter` and `Id.next` (line 34),
   class `counter` with methods `counter.incr` and `counter.get`, and
   external `raw_hash`; `lib/price.ml` defines the operator `+$`.
2. `lib/cart.mli`'s `val add : t -> item -> t` is a function and
   `val empty : t` a value; `lib/parser.mly` defines rules `items` and
   `item`, `lib/lexer.mll` rules `token` and `comment`.
