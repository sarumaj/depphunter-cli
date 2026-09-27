---
id: REQ-JULIA-004
title: Modules resolved to project files
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** place every Julia file in a module through the
include graph (a file included inside `module Shop` is at Shop's top
level) and resolve: a relative module path (one dot the statement's module,
each further dot its parent, stopping at a package's top module, which is
its own parent) to the file defining that module, else to the nearest
enclosing module's file, the importer itself giving no edge; an include to
the file, relative to the including file's directory; an absolute path
whose first segment is the name of a package the file lies in to that
package's `src/Name.jl` (or the file defining the named submodule); a
dependency to a package of the repository when its UUID, a `[sources]`
path or a manifest `path` says so; `Main.X` to the file defining a
top-level module X; a package of the repository by name when no
environment declares it.

## Rationale

Packages in monorepos (lib/*, `[sources]`, `Pkg.develop`) and test and
docs environments reach the package under development, not the registry.

## Acceptance criteria

1. In the fixture, `using .Pricing` and `using ..Pricing` reach
   `src/internal/pricing.jl`, `using ..Shop` `src/Shop.jl`, `import
   Shop.Cart` `src/cart.jl`, `using Utils.Strings` the file the Utils
   package includes, and `using Shop` from `test/`, `ext/` and `docs/`
   `src/Shop.jl`.
2. `...Pricing` from Shop.Cart stops at Shop; `.Cart` inside Cart gives no
   edge.
