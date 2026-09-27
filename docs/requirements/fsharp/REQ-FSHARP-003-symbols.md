---
id: REQ-FSHARP-003
title: Declarations by the offside rule
scope: fsharp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The F# plugin **shall** report as symbols the top-level declarations of a
file's namespace or module and of its nested modules: `let` bindings
(`func` with parameters, `value` without; active patterns and operators by
their parenthesised names), `and` continuations, `val` declarations of
signature files, `type` definitions (`class` with a primary constructor
or a class body, `interface`, `delegate`, else `type`), `exception`s
and nested `module`s, named with the nested module path
(`Order.Rules.valid`), and the members of types (`member`, `static
member`, `abstract`, `override`, `default`, `member val`) as
`Type.Member` methods. Declarations **shall** be found by indentation: a
line starting in the column of its scope's first declaration starts a
declaration, deeper lines belong to it, and a line no deeper than a nested
module's or type's header ends that scope. The namespaces, modules and types a
file declares **shall** be indexed for resolution under their full names,
the contents of an `[<AutoOpen>]` module also under its parent's name.

## Rationale

F# is indentation-sensitive; the offside rule is enough to tell a module's
declarations from the code inside them without a parser.

## Acceptance criteria

1. A namespace with a `[<RequireQualifiedAccess>]` module holding a record,
   functions and a nested module, a class with let bindings, members,
   an abstract/default pair, an operator and an interface implementation,
   mutually recursive types and `let rec ... and` yields exactly the
   expected symbols and declarations.
