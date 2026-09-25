---
id: REQ-CI-006
uuid: 43a7036b-8029-4758-b91f-20e484e703c0
title: Includes triggered by bridge jobs
scope: ci
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The CI plugin **shall** record the entries of a job's `trigger: include:` (a
bridge job starting a child pipeline) as dependencies, in the same forms as a
top-level `include:`.

## Rationale

A bridge job runs another pipeline, which is a dependency like any include.

## Acceptance criteria

1. A job with `trigger: { include: [{ local: child.yml }] }` records a
   dependency on `child.yml`.

## Notes

No automated test covers a bridge job.
