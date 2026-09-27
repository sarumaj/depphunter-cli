---
id: REQ-DLANG-007
uuid: 3b0f090b-6d8a-4cbf-8938-c1431ae0bd8a
title: Modules to packages
scope: dlang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module no project file provides **shall** resolve, in order: to the hidden
standard library when it is druntime's or Phobos' (`core.*`, `std.*`,
`etc.*`, `object`, and the compilers' `ldc.*` and `gcc.*`), named by its
first two segments (`std.algorithm`, `core.thread`, `etc.c`); to the
package dub fetched onto this machine that has the module (REQ-DLANG-008);
to the declared or selected package the module's leading segments spell
(`-`/`_` and case folded; a `-d` suffix, `d-` or `lib` prefix dropped:
`unit_threaded` is `unit-threaded`, `mir.random` `mir-random`) or a curated
module table names (`vibe.*`, `mir.*`, `arsd.*`, `bindbc.*`, `derelict.*`,
`dyaml`, `dparse`, ...), the longer match winning and the table on a tie;
under a package's own prefix, to the one declared package whose name starts
with the module's first segment (`mir.exception` in mir-algorithm is
mir-core); a module of the package's own name missing is dropped; else to
an unresolved package the table names or named by the first segment.

## Rationale

A D module name does not say which package publishes it; the recipe and
what dub fetched are the only authorities without asking the registry.

## Acceptance criteria

1. `vibe.http.server` and `vibe.core.log` go to `vibe-d`, `mir.ndslice` to
   `mir-algorithm` and `mir.math.common` to the selected `mir-core`,
   `arsd.dom` to `arsd-official`, `nothere.x` to an unresolved `nothere`,
   and `shop.gone` is dropped.
