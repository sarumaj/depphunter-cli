---
id: REQ-JULIA-010
uuid: 908818ae-3491-450c-903b-bfd60a7087c2
title: Julia read without running Julia
scope: julia
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Julia **shall** be read without running Julia or Pkg: includes of computed
paths, `LOAD_PATH` changes, `@eval`-generated modules and definitions,
and names a relative import finds through a parent's own `using` are not
followed (such a relative import reaches the parent module's file, or is
dropped); `import A.b` is read as the module path `A.b`, so a function
imported so is an edge to the package; a package's version is not chosen by
Pkg's resolver without a manifest; registries other than General are known
only when installed in a depot and hosted on GitHub.

## Rationale

The map is built from files alone.

## Acceptance criteria

1. `include(f)` with a variable is dropped; `..` from Shop.Cart reaches
   `src/Shop.jl`.
