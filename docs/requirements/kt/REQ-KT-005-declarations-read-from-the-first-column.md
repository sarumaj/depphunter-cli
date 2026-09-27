---
id: REQ-KT-005
title: Declarations read from the first column
scope: kt
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

The declaration index **shall** be built from the text of Kotlin and Scala
files, taking the leading `package` clauses and the definitions that begin in
the first column, and **shall not** see definitions that are indented,
generated, or declared after a Scala package block opens.

## Rationale

The resolver is rebuilt on every run, before any file is parsed, and must stay
cheap; the package clause and top-level definitions are, by convention, at the
start of a line.

## Acceptance criteria

1. A comment containing `package not.this` and a line comment `// class X` are
   not read as declarations.
2. `private[tools] final case class Box`, `sealed trait Shape`,
   `fun String.shout()`, `fun <T> List<T>.second()`, `fun interface Action` and
   `package object util` are read as `Box`, `Shape`, `shout`, `second`, `Action`
   and `util`.
3. A class nested in an object, being indented, is not read.

## Notes

Scripts (`.kts`, `.sc`) and files without a `package` clause are not indexed:
no import can name them.
