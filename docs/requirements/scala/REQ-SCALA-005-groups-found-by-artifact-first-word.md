---
id: REQ-SCALA-005
uuid: bf1db734-aff0-426c-bf26-8c82280c4a58
title: Groups found by an artifact's first word
scope: scala
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The group matching of the JVM plugins **shall**, after every other rule of
[REQ-JAVA-007](../java/REQ-JAVA-007-groupid-matching-heuristics.md), match an
import whose first or second segment equals the first dash-separated word of a
declared artifactId to that artifact's group, the alphabetically first group
winning a tie, except for an import under the language's own standard library
root (`scala.`); artifactIds are read without a Scala binary suffix.

## Rationale

Scala libraries name their packages after the artifact, not the group:
`cats.effect` comes from `org.typelevel:cats-effect`, `akka.actor` from
`com.typesafe.akka:akka-actor-typed`.

## Acceptance criteria

1. `import cats.effect._` resolves to `org.typelevel`.
2. `import akka.actor.typed._` resolves to `com.typesafe.akka`.
3. `cats-effect_2.13` is read as `cats-effect`.
4. With `scala-xml` declared and Scala.js not, `import scala.scalajs.js` is
   unresolved.
