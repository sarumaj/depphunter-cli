---
id: REQ-CI-006
title: Includes triggered by bridge jobs
scope: ci
type: functional
priority: must
status: implemented
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

`TestBridgeJobIncludes` covers the list, project and bare-string forms.
