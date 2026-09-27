---
id: REQ-PURESCRIPT-009
uuid: 98cfb3dd-a629-48ce-b92d-14e680f7788d
title: PureScript islands
scope: purescript
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PureScript plugin **shall** declare the island "PureScript packages"
(`purescript`, named as the registry names them, and by repository for git
packages; a private-pattern prefix) and "PureScript built-ins"
(`purescript-std`, a standard library island holding the compiler's Prim
modules as `prim`).

## Rationale

Registry packages are what a PureScript build downloads; the Prim modules
come with the compiler.

## Acceptance criteria

1. Every package target of the fixture is in one of the two islands.
