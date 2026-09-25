---
id: REQ-JAVA-010
uuid: 0356d869-7465-4f64-9e86-25be4f32c4f6
title: No index answer for Maven dependencies
scope: java
type: limitation
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md Known limits
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The system **shall not** ask a package index what a Maven package depends on,
and **shall** record such a question as unanswered for want of support.

## Rationale

A POM is addressed by group and artifact, and a Maven package on the map carries
only the group; requesting a POM would mean guessing an artifact.

## Acceptance criteria

1. With `--online` and `--resolve-depth 1`, no request is sent for a Maven
   package, and the resolution report records it as unsupported.

## Notes

The index client's report test asks about a Maven package and expects it
recorded as unsupported.
