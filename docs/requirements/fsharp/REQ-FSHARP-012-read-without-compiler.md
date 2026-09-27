---
id: REQ-FSHARP-012
title: Read without the compiler
scope: fsharp
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

The F# plugin **shall** be understood to read F# without type checking or
MSBuild evaluation: unqualified names brought in by an `open` or an
`[<AutoOpen>]` module (`add zero 1m`) are not linked to their files;
qualified names are linked by name only (a local variable named like a
module could be taken for it); MSBuild conditions are ignored (every item
of every configuration counts) and properties other than the project's own
directory are not expanded; an F# `open` of a namespace declared by a C#
project is not linked to it (it becomes an unresolved package unless a
package provides it); what modules a Paket GitHub file declares is unknown
when `paket-files/` is not on disk; `#r` of an assembly outside the
repository is dropped; Paket's credentials and `references: strict` are
not read.

## Rationale

Linking unqualified names would need the compiler's name resolution.

## Acceptance criteria

1. `add zero 1m` in `Cart.fs` (`add` and `zero` of the `[<AutoOpen>]`
   module `Money` in `Types.fs`) is not an import.
2. `#r "bin/Debug/Shop.dll"` (not in the repository) is dropped.
