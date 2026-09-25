---
id: REQ-SUP-028
uuid: e44c9d80-57e8-49aa-852d-27e174d3d899
title: Maven index cannot be asked
scope: sup
type: limitation
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
  - docs/REQUIREMENTS.md Known limits
verification:
  - unit
---

## Statement

The index client **shall not** ask a Maven repository what a package depends on,
and **shall** record the question as unanswerable for this ecosystem.

## Rationale

A POM is addressed by group and artifact, and a package on the map is a group:
there is no document to request, and guessing an artifact is worse than saying
nothing.

## Acceptance criteria

1. With `--online`, a Maven package produces no request and the report says the
   ecosystem's index cannot be asked.

## Notes

The underlying limitation, that Java imports name packages rather than
artifacts, is specified by scope [`java`](../java/).
