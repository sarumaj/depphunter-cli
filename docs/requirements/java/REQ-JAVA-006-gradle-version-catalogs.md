---
id: REQ-JAVA-006
uuid: e83cd2d8-f8a7-49c5-81d9-9bf983498a86
title: Gradle version catalogs
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

The Java plugin **shall** read the `[libraries]` of a `libs.versions.toml`
version catalog, in the `"g:a:v"` form and in the table form with `module` or
`group`/`name` and a `version` given directly or as `version.ref` into
`[versions]`.

## Rationale

Version catalogs move Gradle's coordinates and versions out of the build script
(`implementation(libs.guava)`), so the build script alone does not name them.

## Acceptance criteria

1. `guava = { module = "com.google.guava:guava", version.ref = "guava" }` with
   `guava = "33.0.0-jre"` declares group `com.google.guava` at `33.0.0-jre`.
