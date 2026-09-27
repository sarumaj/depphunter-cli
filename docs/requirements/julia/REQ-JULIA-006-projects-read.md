---
id: REQ-JULIA-006
uuid: 256f616a-a57c-4bcd-bd61-da5dd68cdb3e
title: Project.toml read
scope: julia
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a `Project.toml` (a `JuliaProject.toml` beside
it wins): its `name` (a symbol), `[deps]`, `[weakdeps]` and `[extras]`
(name = UUID), `[compat]`, `[sources]` (path, url and rev),
`[extensions]` and `[workspace] projects`. Every dependency of each section
**shall** be an import of the file, resolved as the package's code would
resolve it; an extension an import of `ext/Name.jl` (or
`ext/Name/Name.jl`); a workspace project an import of its project. The
governing projects of a file are its directory's and those above it,
nearest first; a file under none may use any project's declarations.

## Rationale

Extensions, test extras and workspaces are how packages declare optional
and test dependencies; a test environment's `test/Project.toml` has no
name and names the package by UUID.

## Acceptance criteria

1. The fixture's `Project.toml` imports six deps, the weak dependency
   Plots (floating on `1`), the extension file and the test extra Test;
   `test/Project.toml`'s Shop and `docs/Project.toml`'s Shop (by
   `[sources]`) are `src/Shop.jl`.
