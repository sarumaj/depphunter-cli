---
id: REQ-JAVA-011
title: gson resolves with few unresolved imports
scope: java
type: non-functional
priority: should
status: implemented
verification:
  - e2e
---

## Statement

The Java plugin **should** analyze the gson repository with no more than about 8
unresolved imports, all of them in generated or test-only code.

## Rationale

gson is the Java reference project; the remaining unresolved imports come
from generated and test-only code the heuristics cannot attribute.

## Acceptance criteria

1. An analysis of gson reports about 8 unresolved imports, none in main sources.

## Notes

A manual measurement; no automated test runs against gson.
