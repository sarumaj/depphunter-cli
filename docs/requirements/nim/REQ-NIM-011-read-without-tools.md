---
id: REQ-NIM-011
title: Read without the compiler, nimble or Atlas
scope: nim
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Nim projects **shall** be read without running the compiler, nimble or
Atlas: NimScript is not executed (a requirement or search path computed from
variables is not known), `when` conditions are not evaluated (every branch
counts), macros and templates are not expanded, a configuration applies to
the modules of its directory and below (not per compiled project), `$home`,
`~`, `$nimblepath` and other variables in search paths are not known, a
`nimble.develop` path outside the repository names nothing, a generated
`nimble.paths` missing from the repository names nothing, and nothing is
asked of the package list.

## Rationale

What NimScript computes is known only when it runs.

## Acceptance criteria

1. `include "nimble.paths"` without the file and `--path:"$home/winlibs"`
   are dropped, and a requirement in a `when defined(windows)` branch is
   read.
