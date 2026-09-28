---
id: REQ-SCALA-003
title: Project sources found by declared package
scope: scala
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Scala plugin **shall** resolve an import to the project Scala or Kotlin
file whose package (Scala's chained clauses joined, or the package block
holding the definition) and top-level definitions name the import's longest
prefix, wherever the file sits, and to a Java file as the Java plugin does; an
import naming a package **shall** resolve as in
[REQ-KT-003](../kt/REQ-KT-003-sources-by-declared-package.md).

## Rationale

A Scala file commonly holds several classes, and its directory need not match
its package.

## Acceptance criteria

1. `import com.example.app.model.{User, Order}` resolves both to
   `src/main/scala/domain.scala`, which declares that package and both classes.
2. `import com.example.app.util._` resolves to the directory of that package's
   only file.
3. `import org.acme.Tools` resolves to the Kotlin file declaring
   `package org.acme` and `object Tools`.
4. `package com.example` followed by `package legacy` declares
   `com.example.legacy`.
5. `import com.acme.app.A` resolves to the file declaring `package com.acme`
   and then `package app { class A }`
   ([REQ-KT-007](../kt/REQ-KT-007-declarations-found-by-nesting.md)).
