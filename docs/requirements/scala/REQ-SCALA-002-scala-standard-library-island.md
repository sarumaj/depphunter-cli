---
id: REQ-SCALA-002
uuid: 63e50456-654d-42de-a0a3-c739da98c3f5
title: Scala standard library island
scope: scala
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Scala plugin **shall** assign imports under `scala.` to the ecosystem
`scala-std` (package: the first two segments, or `scala` for a name the root
package defines, such as `scala.Option`), except the modules published apart
from the library (`scala.xml`, `scala.util.parsing`,
`scala.collection.parallel`, `scala.swing`, `scala.async`), which **shall**
resolve to the Maven group `org.scala-lang.modules` when it is declared, and
the Scala.js and Scala Native libraries (`scala.scalajs`, `scala.scalanative`),
which **shall** resolve to `org.scala-js` and `org.scala-native` when declared;
and **shall** assign imports of JDK packages to the `jdk` ecosystem.

## Rationale

scala-library ships with every Scala program; its former parts moved into
separately versioned artifacts that a build must declare.

## Acceptance criteria

1. `import scala.collection.mutable` resolves to `scala-std`, package
   `scala.collection`.
2. `import scala.xml.Elem` resolves to `org.scala-lang.modules` at the version
   the build declares.
3. `import java.time.Instant` resolves to `jdk`, package `java.time`.
4. `import scala.Option` resolves to `scala-std`, package `scala`.
