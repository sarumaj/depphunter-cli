---
id: REQ-SCALA-006
uuid: d093459b-e6be-48eb-aae6-2d00141cf5a7
title: Maven dependencies shared with Java
scope: scala
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Scala plugin **shall** resolve imports that name no project source to the
Maven groups declared by sbt builds, `pom.xml`, Gradle build scripts and
version catalogs, using the Java plugin's manifest readers, group matching and
pin rule, and **shall** place them on the same `maven` island as Java's.

## Rationale

Scala artifacts are published to Maven repositories, and mixed builds share
them with Java and Kotlin code.

## Acceptance criteria

1. `import com.google.common.base.Strings` resolves to `com.google.guava`
   through the known-mismatch table.
2. `import org.scalatest.flatspec.AnyFlatSpec` resolves to `org.scalatest` by
   prefix, from a test-scoped dependency.
