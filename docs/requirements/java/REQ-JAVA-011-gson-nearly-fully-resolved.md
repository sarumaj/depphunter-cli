---
id: REQ-JAVA-011
uuid: 34bca490-b2ef-48eb-aa99-821f8eb734f1
title: gson resolves with few unresolved imports
scope: java
type: non-functional
priority: should
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - e2e
---

## Statement

The Java plugin **should** analyze the gson repository with no more than about 8
unresolved imports, all of them in generated or test-only code.

## Rationale

gson was the Java reference project in M4; the remaining unresolved imports come
from generated and test-only code the heuristics cannot attribute.

## Acceptance criteria

1. An analysis of gson reports about 8 unresolved imports, none in main sources.

## Notes

Measured in the design log (M4). No automated test runs against gson.
