---
id: REQ-JAVA-004
uuid: 37d08989-2a67-4883-9429-8fa98ce221d0
title: Maven dependencyManagement versions
scope: java
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The Java plugin **shall** take the version of a POM dependency that declares
none from the `<dependencyManagement>` entry of the same groupId.

## Rationale

Managed versions are how multi-module Maven builds and BOMs keep versions in one
place; a dependency without its managed version would have none.

## Acceptance criteria

1. A `junit-jupiter` dependency without a version and a managed
   `org.junit.jupiter` version `5.10.2` resolves to version `5.10.2`.

## Notes

Managed versions are read from the same POM only; parent POMs and imported BOMs
are not followed.
