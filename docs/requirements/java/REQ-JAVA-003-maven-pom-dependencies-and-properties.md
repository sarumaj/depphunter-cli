---
id: REQ-JAVA-003
uuid: ac87dae2-be0b-4e4f-a872-50c0c6a4e852
title: Maven POM dependencies with properties
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

The Java plugin **shall** read the dependencies of every `pom.xml`, expanding
`${…}` references from the POM's `<properties>`, `project.version` and
`project.groupId`, and **shall** take the POM's own groupId (or its parent's) as
a group of the project rather than a dependency.

## Rationale

Maven POMs commonly factor versions into properties; without expanding them the
version on the map would be a placeholder.

## Acceptance criteria

1. A dependency with `<version>${spring.version}</version>` and
   `<spring.version>6.1.0</spring.version>` resolves to version `6.1.0`.
2. An import under the project's own groupId that names no project file resolves
   to nothing rather than to an external package.
