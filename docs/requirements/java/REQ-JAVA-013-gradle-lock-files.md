---
id: REQ-JAVA-013
title: Gradle lock files
scope: java
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The Java plugin, and the Kotlin and Scala plugins through it, **shall** read
Gradle's dependency locks - a project's `gradle.lockfile`
(`group:artifact:version=configurations` lines) and the per-configuration
`gradle/dependency-locks/<configuration>.lockfile` files of Gradle 6
(`group:artifact:version` lines) - and **shall** give a declared artifact the
version its locks record, pinned, keeping the declared version as the
requested one. Locks that record a module at different versions **shall**
lock none; a module only the locks name (a transitive dependency) **shall
not** be taken as declared; the locks of the build script's own classpath
(`buildscript-gradle.lockfile`, `settings-gradle.lockfile`,
`buildscript-*.lockfile`) **shall not** be read.

## Rationale

A build that locks its dependencies declares dynamic versions (`2.3.+`,
ranges, none from a platform) and records what they resolved to in plain
text; without the lock such a dependency floats on the map although the
build is reproducible.

## Acceptance criteria

1. `io.ktor:ktor-client-core:2.3.+` locked at `2.3.12` resolves to
   `2.3.12`, requested `2.3.+`, pinned; a dependency declared without a
   version gets the locked one; a range is replaced likewise.
2. `gradle.lockfile` and a subproject's per-configuration lock disagreeing on
   a module leave its declared version in place.
3. A module only `gradle.lockfile` names stays undeclared, and a build-script
   classpath lock changes nothing.

## Notes

The locks of all projects of a build are merged, as the plugin reads the
artifacts of every build file of the repository together.
