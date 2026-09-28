---
id: REQ-JAVA-005
title: Gradle build file dependencies
scope: java
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Java plugin **shall** read dependencies written as
`"group:artifact[:version]"` strings and in the map notation
(`group: 'g', name: 'a', version: 'v'`, or `group = "g", name = "a"` in the
Kotlin DSL) from `build.gradle` and `build.gradle.kts`, Kotlin Multiplatform
source sets included, and **shall** take a top-level `group = "…"` assignment as
a group of the project.

## Rationale

Gradle is the other main Java build tool; its dependency notations are strings
that a pattern reads without evaluating the build script.

## Acceptance criteria

1. `implementation("com.squareup.okhttp3:okhttp:4.12.0")` declares the artifact
   `com.squareup.okhttp3:okhttp` at version `4.12.0`.
2. `implementation("io.ktor:ktor-client-core:2.3.+")` declares
   `io.ktor:ktor-client-core` at the dynamic version `2.3.+`.
3. `implementation group: 'io.ktor', name: 'ktor-client-core', version:
   '2.3.12'` declares `io.ktor:ktor-client-core` at `2.3.12`.

## Notes

The build script is not evaluated: dependencies computed in code, or whose
coordinates are built from variables, are not seen. Gradle lock files pin
the declared dependencies (REQ-JAVA-013).
