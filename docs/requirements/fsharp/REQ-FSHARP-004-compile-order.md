---
id: REQ-FSHARP-004
uuid: 21a776bf-31ee-48ed-8798-b3c909a3e7b5
title: Opens and names resolved in compile order
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An open or a qualified name **shall** resolve to the project files
declaring it - as written, else qualified by each of its contexts, the
first that any file declares; for a qualified name the longest prefix
first - an edge to each file when several declare it (a namespace spread
over files, a signature and its implementation). A file compiled by a
`.fsproj` **shall** only resolve to files earlier in the compile order of
a project both are in, or to files of a project its project references
(transitively); a file in no project (a script) may resolve to any. An
open whose declarations the compile order or the references keep away,
and that no package provides, **shall** be dropped; so **shall** a
qualified name that no project file declares.

## Rationale

F# compiles a project's files in the order the `.fsproj` lists them, and
a file can only use what earlier files declare: an edge to a later file is
impossible code.

## Acceptance criteria

1. `Cart.fs` opening `Shop.Domain` (declared by `Types.fs`, earlier)
   resolves to `Types.fs`; its `open Shop.Domain.Later` and `Later.value`
   (`Later.fs` is compiled after it) are dropped.
2. `Pricing.total` resolves to both `Pricing.fsi` and `Pricing.fs`.
3. `Program.fs` of a project referencing `Shop.Domain.fsproj` resolves
   `open Shop.Domain` to `Types.fs` and `Currency.EUR` (a type of an
   `[<AutoOpen>]` module) to `Types.fs`; `Tests.fs` of a project
   referencing neither drops `open Shop.Domain`.
