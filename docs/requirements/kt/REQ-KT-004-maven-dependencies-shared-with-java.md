---
id: REQ-KT-004
uuid: a821d470-e190-442b-bc2b-f4c1ed407aba
title: Maven dependencies shared with Java
scope: kt
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Kotlin plugin **shall** resolve imports that name no project source to the
Maven groups declared by `pom.xml`, Gradle build scripts (Groovy and Kotlin
DSL), Gradle version catalogs and sbt builds, using the Java plugin's manifest
readers, group matching and pin rule, and **shall** place them on the same
`maven` island as Java's.

## Rationale

Kotlin builds use the same build tools and repositories as Java; reading them
twice would let the two plugins disagree about one dependency.

## Acceptance criteria

1. `import okhttp3.OkHttpClient` resolves to `com.squareup.okhttp3` at `4.12.0`,
   pinned, from `build.gradle.kts`.
2. `import io.ktor.client.HttpClient` resolves to `io.ktor` at `2.3.+`, not
   pinned.
3. `import com.google.common.collect.ImmutableList` resolves to
   `com.google.guava` through the version catalog.
4. An import under the project's own `group` that names no source file is
   dropped.
