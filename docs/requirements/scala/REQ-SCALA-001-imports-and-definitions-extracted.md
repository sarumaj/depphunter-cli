---
id: REQ-SCALA-001
uuid: cfea3ce7-fdbc-4568-81be-2354081c4b5a
title: Scala imports and definitions extracted
scope: scala
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Scala plugin **shall** analyze `.scala` and `.sc` files with tree-sitter,
reporting every import wherever it appears, split into one import per name:
comma-separated clauses, braced selectors (`{A, B => C}`, `{A as C}`) and
wildcards (`_`, `*`, `given`) normalized to `.*`, with a selector renamed to `_`
omitted as hidden; and reporting as symbols the classes, traits, objects,
enums, functions, values and type aliases of the file and of a package block,
extension methods, and the methods of every class, trait, object and enum
(`Owner.name`).

## Rationale

One Scala import statement often names several things from different places;
splitting it lets each resolve on its own, as Rust's use trees are split.

## Acceptance criteria

1. `import scala.concurrent.{Future, ExecutionContext => EC}` yields
   `scala.concurrent.Future` and `scala.concurrent.ExecutionContext`.
2. `import a.b.{C => _, _}` yields only `a.b.*`.
3. `import io.circe.given` and `import cats.syntax.all.*` yield `io.circe.*`
   and `cats.syntax.all.*`.
4. An `import` inside a method is reported.
5. `class App` and `object App` in one file yield `App` and `App@<line>`.
