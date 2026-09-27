---
id: REQ-JULIA-002
title: using, import and include read
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read every `using` and `import` statement (each
module of `using A, B.C`; `using A: x, y`; `import A.B: c`; `import
A as B`; relative `using .Sub` and `..Parent`; in `try`, `@static
if`, `@eval` and `quote` too) as an import of the module path, with the
module the statement is in, and every `include`, `Base.include(Mod, ...)`
and Revise's `includet` as an import of the file its argument names: a
string, `joinpath(@__DIR__, ...)`, `joinpath(dirname(@__FILE__), ...)`,
`@__DIR__ * "/x.jl"`, `"$(@__DIR__)/x.jl"`, `normpath`/`abspath` of
those and an `@eval`'s `$(...)`. An include of a computed path **shall**
be recorded and dropped.

## Rationale

Julia packages are one module spread over files by `include`; `using`
names modules, packages and submodules alike.

## Acceptance criteria

1. In the fixture, `src/Shop.jl` imports JSON, HTTP, LinearAlgebra,
   Statistics (`import ... as`), Base (twice) and its two relative
   submodules, and includes `cart.jl` and `internal/pricing.jl` (by
   `joinpath(@__DIR__, ...)`); `test/runtests.jl`'s `include(f)` is
   dropped.
2. Imports inside strings, triple-quoted strings, raw strings, nested
   `#= =#` comments and line comments are not read; those in `try`,
   `@static if`, `@eval` and `quote` are.
