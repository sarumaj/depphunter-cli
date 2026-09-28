---
id: REQ-SUP-056
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
properties along that chain, each parent from the repository its child came
from. When the repository has a Clojure manifest, Clojars
(`https://repo.clojars.org`) **shall** be asked after Maven Central or the
mirror that replaces it (not after a mirror of `*`), and reported as a public
index. Maven
repositories that Clojure manifests declare (`:mvn/repos`, `:repositories`,
shadow-cljs's `:maven`) **shall** be recorded as the repository's, except
Maven Central and Clojars, as **shall** those a `pom.xml` (`<repositories>`)
a Gradle build or settings script (`maven { url … }`, `maven("…")`,
outside `pluginManagement` and `buildscript`) and an sbt build's `resolvers`
declare; each is asked beside Central, except a Clojure repository named
`"central"`, which replaces it as tools.deps and Leiningen do
([REQ-SUP-063](REQ-SUP-063-additive-sources-fall-back-to-the-public-index.md)).

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
5. The POM of an artifact only an active settings profile's repository holds
   is read from it with its server's credential, and Central answers for the
   rest.

## Notes

Live runs could reach Maven Central from the sandbox (compojure
`--online --resolve-depth 1`: `org.clojure:clojure` and
`org.clojure:tools.macro` answered), not Clojars (403 from the proxy), so
Clojars was verified with a stub server.
