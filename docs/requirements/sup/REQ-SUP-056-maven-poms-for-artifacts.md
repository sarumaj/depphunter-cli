---
id: REQ-SUP-056
uuid: 1ddfd2f8-7f09-4e0b-82f2-84ef9e50c201
title: Maven POMs for group:artifact packages
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a Maven package
named `group:artifact` depends on from its POM: the version pinned, else the
release its `maven-metadata.xml` names; the dependencies of the compile and
runtime scopes that are not optional, with those its parent POMs declare
(up to four levels), versions filled from `dependencyManagement` and
properties along that chain. When the index is Maven Central and the
repository has a Clojure manifest, Clojars (`https://repo.clojars.org`)
**shall** be asked after Central, and reported as a public index. Maven
repositories that Clojure manifests declare (`:mvn/repos`, `:repositories`,
shadow-cljs's `:maven`) **shall** be recorded as the repository's, except
Maven Central and Clojars, as **shall** those a `pom.xml` (`<repositories>`)
and a Gradle build or settings script (`maven { url … }`, `maven("…")`,
outside `pluginManagement` and `buildscript`) declare.

## Rationale

Maven packages on the map name their artifact (Java, Kotlin, Scala, Clojure
and Bazel alike), so a POM can be requested; Clojure's tools search Clojars
after Central without being configured to.

## Acceptance criteria

1. `org.clojure:clojure` 1.11.1 yields `spec.alpha` at the POM's property
   version, `core.specs.alpha` at the parent's managed version and the
   parent's own dependency, and neither the test-scoped nor the optional
   one.
2. `cheshire:cheshire`, absent from Central, is read from Clojars at the
   release its metadata names; without a Clojure manifest Clojars is not
   asked, and a Maven name without an artifact is never asked.
3. A Gradle script's `maven("https://kts.internal/releases")` and
   `maven { url 'https://groovy.internal/maven' }` are recorded as the
   repository's Maven sources; `mavenCentral()`, `google()`, a Central URL and
   a `pluginManagement` repository are not.
4. A POM that declares `encoding="ISO-8859-1"` (or Latin-1, Windows-1252) is
   read like a UTF-8 one.

## Notes

Live runs could reach Maven Central from the sandbox (compojure
`--online --resolve-depth 1`: `org.clojure:clojure` and
`org.clojure:tools.macro` answered), not Clojars (403 from the proxy), so
Clojars was verified with a stub server.
