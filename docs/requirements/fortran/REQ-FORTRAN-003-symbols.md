---
id: REQ-FORTRAN-003
title: Symbols
scope: fortran
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols modules, submodules and programs
(kinds `module`, `submodule`, `program`), named `block data` units,
derived type definitions (`type :: t`, `type, extends(a) :: t`, `type t`;
kind `type`), generic interfaces (`interface name`, `interface
operator(+)`; kind `interface`) and functions and subroutines (kind
`function`) with any prefix (`pure`, `elemental`, `recursive`, `impure`,
`module`, a type such as `real(dp)`, `double precision`, `character*(*)` or
`type(t)`), and separate module procedures (`module procedure p` outside an
interface block). A procedure **shall** be named after its enclosing module,
submodule, program or procedure (`Module.proc`, `outer.inner`). Interface
bodies (declarations of procedures defined elsewhere) **shall** not be
symbols, nor names fypp substitutes (`${k}$`).

## Rationale

Module procedures are what other files call through `use`; an interface
body only declares a procedure another file or a submodule defines.

## Acceptance criteria

1. `src/shop.f90` yields `shop`, `order_t`, `total`, `shop.total_int`,
   `shop.total_real`, `shop.walk`, `shop.walk.inner` and `shop.new_order`,
   and nothing for its abstract interface.
2. `legacy/dgemm.f` yields `DGEMM`, `DDOT` (`DOUBLE PRECISION FUNCTION`),
   `LABEL` (`CHARACTER*(*) FUNCTION`), `SHOPINIT` (block data),
   `LEGACY_UTIL` and `LEGACY`.
