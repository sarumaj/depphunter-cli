---
id: REQ-JULIA-003
title: Definitions extracted
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract a file's top-level definitions - those not
inside a function, type, `let`, loop, `do` or `quote` block, but inside
modules and `begin`, `if` and `try` blocks - as symbols: modules,
functions and macros (`function f`, `macro m`, the short form `f(x) =`
with `::T` and `where` clauses and leading macros such as `@inline`,
methods of other modules' functions as `Base.show`, operators as
`Base.+`), structs, mutable structs, abstract and primitive types (kind
`type`), `const`s and `@enum` types. A function with several methods
**shall** be one symbol, at its first definition.

## Rationale

Julia's multiple dispatch adds methods to one function; the map shows the
function once.

## Acceptance criteria

1. `src/Shop.jl` gives exactly its module, three consts (two from one
   line), four types, an enum, a macro and seven functions (`checkout`
   once; not the local `inner`, the `let` block's `lookup` or the inner
   constructor).
