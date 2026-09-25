---
id: REQ-SUP-022
uuid: f0be78d1-d168-4a7f-b018-f51b0b61c660
title: npm dependencies from the version document
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The index client **shall** read an npm package's dependencies from the
registry's document for that version; for a version that is not a complete exact
version it **shall** read the `latest` document.

## Rationale

The registry serves each version's manifest with its dependencies.

## Acceptance criteria

1. Against a stub registry the client returns the dependencies of the requested
   version with their requested ranges.
