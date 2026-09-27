---
id: REQ-SCALA-004
uuid: 046cc599-b708-4b2e-bc7b-0efe018b96f5
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
directory, reading `%%` and `%%%` as `%`, taking a version that names a `val`
of the same file from that `val`, and taking `organization := "…"` as a group
of the project.

## Rationale

sbt is the Scala build tool. The map shows Maven groups, not artifacts, so the
Scala binary suffix `%%` would append (`_2.13`, `_3`) changes nothing it shows,
and `scalaVersion` is not consulted. `project/*.sbt` configures sbt's own
plugins, which the code never imports.

## Acceptance criteria

1. `"org.typelevel" %% "cats-effect" % catsVersion` with
   `val catsVersion = "3.5.4"` declares `org.typelevel` at `3.5.4`, pinned.
2. `"io.circe" %% "circe-core" % "0.14.+"` declares `io.circe` at `0.14.+`, not
   pinned.
3. `addSbtPlugin(...)` in `project/plugins.sbt` declares nothing: an import of
   its group is unresolved.
4. An import under `ThisBuild / organization` that names no source file is
   dropped.

## Notes

The build is not evaluated: versions computed in code or held in another file
(`project/Dependencies.scala`) are not seen, and such a dependency carries no
version.
