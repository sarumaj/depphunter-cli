---
id: REQ-JAVA-004
title: Maven dependencyManagement versions
scope: java
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Java plugin **shall** take the version of a POM dependency that declares
none from the `<dependencyManagement>` entry of the same `group:artifact`.

## Rationale

Managed versions are how multi-module Maven builds and BOMs keep versions in one
place; a dependency without its managed version would have none.

## Acceptance criteria

1. A `junit-jupiter` dependency without a version and a managed
   `org.junit.jupiter:junit-jupiter` version `5.10.2` resolves to version
   `5.10.2`.

## Notes

Managed versions are read from the same POM only; parent POMs and imported BOMs
are not followed. Before Maven packages were named by artifact, a managed entry
applied to every dependency of its groupId.
