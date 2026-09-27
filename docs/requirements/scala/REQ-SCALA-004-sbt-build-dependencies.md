---
id: REQ-SCALA-004
title: sbt build dependencies
scope: scala
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Scala plugin **shall** read dependencies written as
`"group" % "artifact" % version` from every `*.sbt` file outside a `project`
directory, taking a version that names a `val` of the same file from that
`val`, and taking `organization := "…"` as a group of the project. For `%%`
and `%%%` it **shall** append the Scala binary version (`_3` for Scala 3,
`_2.13` for 2.13.x) of the file's `scalaVersion` (a string or a `val`), else of
the `scalaVersion` of the build file nearest the root, to the artifact's name,
and keep the name as written when no `scalaVersion` is found.

## Rationale

sbt is the Scala build tool. `%%` publishes and resolves an artifact under its
binary-suffixed name (`cats-effect_3`), which is the name Maven Central, OSV
and the artifact's POM know. `%%%` (Scala.js, Scala Native) adds a platform
suffix that depends on the project the build is evaluated for, so it is read
as `%%`. `project/*.sbt` configures sbt's own plugins, which the code never
imports.

## Acceptance criteria

1. `"org.typelevel" %% "cats-effect" % catsVersion` with
   `val catsVersion = "3.5.4"` and `scalaVersion := "3.3.3"` declares
   `org.typelevel:cats-effect_3` at `3.5.4`, pinned.
2. `"io.circe" %% "circe-core" % "0.14.+"` declares `io.circe:circe-core_3` at
   `0.14.+`, not pinned.
3. With `ThisBuild / scalaVersion := scala213` and `val scala213 = "2.13.14"`,
   `%%` appends `_2.13`; `"com.google.guava" % "guava"` stays `guava`; without
   any `scalaVersion`, `%% "cats-effect"` stays `cats-effect`.
4. `addSbtPlugin(...)` in `project/plugins.sbt` declares nothing: an import of
   its group is unresolved.
5. An import under `ThisBuild / organization` that names no source file is
   dropped.

## Notes

The build is not evaluated: versions computed in code or held in another file
(`project/Dependencies.scala`) are not seen, and such a dependency carries no
version; `crossScalaVersions` is not consulted.
