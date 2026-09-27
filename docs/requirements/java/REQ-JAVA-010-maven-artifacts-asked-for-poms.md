---
id: REQ-JAVA-010
title: Maven artifacts asked for their POMs
scope: java
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the system **shall** ask the Maven repository for the POM of
every Maven package the Java, Kotlin and Scala plugins place on the map, as it
does for any package named `group:artifact` (REQ-SUP-056).

## Rationale

The plugins name the artifact that ships an import, so its POM, and with it
the artifact's own dependencies, can be requested.

## Acceptance criteria

1. With `--online` and `--resolve-depth 1`, `com.google.guava:guava`
   `33.0.0-jre` is answered from its POM on Maven Central, without Clojars.

## Notes

Until Maven packages were named by artifact, the Java plugins named them by
group alone, and nothing could be asked (REQ-SUP-028 keeps that rule for a
name without an artifact).
