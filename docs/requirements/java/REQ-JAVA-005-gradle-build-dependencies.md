---
id: REQ-JAVA-005
uuid: e7c29456-d90d-488a-9356-3469f60bb107
title: Gradle build file dependencies
scope: java
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The Java plugin **shall** read dependencies written as
`"group:artifact[:version]"` strings from `build.gradle` and `build.gradle.kts`,
and **shall** take a top-level `group = "…"` assignment as a group of the
project.

## Rationale

Gradle is the other main Java build tool; its dependency notation is a string
that a pattern reads without evaluating the build script.

## Acceptance criteria

1. `implementation("com.squareup.okhttp3:okhttp:4.12.0")` declares group
   `com.squareup.okhttp3` at version `4.12.0`.

## Notes

The build script is not evaluated: dependencies computed in code, or whose
coordinates are built from variables, are not seen.
