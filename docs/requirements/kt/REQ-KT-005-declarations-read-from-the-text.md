---
id: REQ-KT-005
title: Declarations read from the text
scope: kt
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

The declaration index **shall** be built from the text of Kotlin and Scala
files (REQ-KT-007), without a compiler, and **shall not** see definitions that
are generated (by annotation processors, KSP, compiler plugins or macros),
Scala 3 extension methods and given instances, or anything after a
triple-quoted string or a block comment left open. A Scala 3 body opened by a
colon is told by indentation alone.

## Rationale

The resolver is rebuilt on every run, before any file is parsed, and must stay
cheap; a file's package clauses and top-level definitions can be found from its
text once comments and literals are set aside.

## Acceptance criteria

1. A comment containing `package not.this` and a line comment `// class X` are
   not read as declarations.
2. `private[tools] final case class Box`, `sealed trait Shape`,
   `fun String.shout()`, `fun <T> List<T>.second()`, `fun interface Action` and
   `package object util` are read as `Box`, `Shape`, `shout`, `second`, `Action`
   and `util`.
3. A class nested in an object, being indented under it, is not read.
4. An unterminated raw string hides the definitions after it; one left open on
   a single line ends at its line break.

## Notes

Scripts (`.kts`, `.sc`) and files without a `package` clause are not indexed:
no import can name them.
