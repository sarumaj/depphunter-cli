---
id: REQ-JAVA-012
title: Maven packages named group:artifact
scope: java
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Java, Kotlin and Scala plugins **shall** name a Maven package
`group:artifact`, as POMs, Maven Central, OSV's Maven ecosystem and Trivy name
it, and as the Clojure and Bazel plugins name theirs, so that one artifact is
one node on the `maven` island whichever plugin reaches it, and its OSV query
and a Trivy finding about it name it the way the map does.

## Rationale

A node per group merged unrelated artifacts, could not be matched to an OSV
advisory or a Trivy finding (both name `group:artifact`), and stood beside the
Clojure and Bazel nodes for the same artifact.

## Acceptance criteria

1. A repository whose `pom.xml` declares `com.google.guava:guava` for a Java
   import, whose `deps.edn` declares `com.google.guava/guava` and whose
   `MODULE.bazel` installs `com.google.guava:guava` has one Maven package node,
   `com.google.guava:guava`, with edges from the Java file, `deps.edn` and the
   `BUILD.bazel` file.
2. The OSV query for a pinned Maven package names it `group:artifact`.

## Notes

This changed the names of every Java, Kotlin and Scala Maven node (from
`com.google.guava` to `com.google.guava:guava`); the plugins' versions were
bumped so that cached analyses are redone.
