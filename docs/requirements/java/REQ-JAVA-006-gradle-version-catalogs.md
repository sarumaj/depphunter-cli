---
id: REQ-JAVA-006
title: Gradle version catalogs
scope: java
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Java plugin **shall** read the `[libraries]` of a `libs.versions.toml`
version catalog, in the `"g:a:v"` form and in the table form with `module` or
`group`/`name` and a `version` given directly, as `version.ref` into
`[versions]`, or as a rich version (`strictly`, `require`, `prefer`, in that
order).

## Rationale

Version catalogs move Gradle's coordinates and versions out of the build script
(`implementation(libs.guava)`), so the build script alone does not name them.

## Acceptance criteria

1. `guava = { module = "com.google.guava:guava", version.ref = "guava" }` with
   `guava = "33.0.0-jre"` declares `com.google.guava:guava` at `33.0.0-jre`.
2. `{ group = "io.netty", name = "netty-handler", version.ref = "netty" }` with
   `netty = { strictly = "4.1.110.Final" }` declares `io.netty:netty-handler` at
   `4.1.110.Final`.
