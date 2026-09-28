---
id: REQ-NIM-008
title: The standard library
scope: nim
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`std/x` and the standard library's module names (`os`, `strutils`,
`tables`, `asyncdispatch`, `posix`, `winlean`..., modules of older releases
included) **shall** be packages of the hidden standard library island, one
per module; a module that left the library for a nimble package in Nim 2
(`db_sqlite` for db_connector, `smtp`, `punycode`) is that package when
the project requires it. In a repository carrying Nim's own library
(`lib/system.nim` beside `lib/pure/`), std modules **shall** resolve to its
files as the compiler searches them (`lib/pure`, `lib/core`, `lib/std` for
`std/x` ...), and `$nim` and `$lib` in search paths to its directories.

## Rationale

The standard library comes with the compiler; in Nim's own repository it is
the repository.

## Acceptance criteria

1. The fixture's `std/os`, `tables` and `db_sqlite` are standard library
   packages; in the tiny Nim repository `std/os` is `lib/pure/os.nim`,
   `std/syncio` `lib/std/syncio.nim` and `compiler/ast` from `tools/`
   (`path = "$nim"`) `compiler/ast.nim`.
