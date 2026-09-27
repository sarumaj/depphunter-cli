---
id: REQ-SCALA-007
uuid: b5b6914e-e4ca-4824-8711-cd4f9045baee
title: Imports relative to the enclosing packages
scope: scala
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Scala plugin **shall** resolve an import first relative to the packages
enclosing the importing file, innermost first, to a project source; then, when
its first segment is a package of the Scala library (`collection`, `util`,
`concurrent`, …) and no project package or declared Maven group has that root,
as that package of the library; and otherwise as an absolute import. An import
starting with `_root_.` **shall** be resolved as absolute only, and an import of
a single name that none of these resolves (the members of a value in scope,
`import builder._`) **shall** be dropped.

## Rationale

Scala imports are relative, and `scala._` is imported into every file:
`import collection.mutable` means `scala.collection.mutable`, and in package
`com.example.app`, `import model.Order` means `com.example.app.model.Order`. A
root package on the class path takes precedence over `scala._`, which is why
`import io.circe._` is not `scala.io.circe`.

## Acceptance criteria

1. In package `com.example.app`, `import model.Order` resolves to the file
   declaring `com.example.app.model.Order`, and `import util.Text` to the file
   declaring `com.example.app.util.Text`.
2. `import collection.immutable.ListMap` resolves to `scala-std`, package
   `scala.collection`.
3. With `io.circe` declared, `import io.circe.given` resolves to `io.circe`.
4. `import _root_.cats.effect.IO` resolves as `cats.effect.IO`.
5. `import builder._` after `val builder = …` is dropped.
