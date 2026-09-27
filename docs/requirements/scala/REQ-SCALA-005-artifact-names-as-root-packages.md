---
id: REQ-SCALA-005
title: Artifact names as root packages
scope: scala
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The artifact matching of the JVM plugins
([REQ-JAVA-007](../java/REQ-JAVA-007-artifact-matching-heuristics.md)) **shall**
take the dash-separated words of a declared artifact's name, read without a
Scala binary suffix, as root packages of that artifact (`cats-effect` gives
`cats.effect` and `cats`), except a single generic word (`core`, `commons`,
`java`, …) and the language's own standard library root (`scala`).

## Rationale

Scala libraries name their packages after the artifact, not the group:
`cats.effect` comes from `org.typelevel:cats-effect`, `akka.actor` from
`com.typesafe.akka:akka-actor-typed`.

## Acceptance criteria

1. `import cats.effect._` resolves to `org.typelevel:cats-effect_3`.
2. `import akka.actor.typed._` resolves to
   `com.typesafe.akka:akka-actor-typed_3`.
3. `cats-effect_2.13` is read as `cats-effect`.
4. With `scala-xml` declared and Scala.js not, `import scala.scalajs.js` is
   unresolved (`org.scala-js:scalajs-library`).
5. `import cats.syntax.all.*` with only `cats-effect_3` declared resolves to
   `org.typelevel:cats-core_3`, which comes with it, at its version.
