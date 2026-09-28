---
id: REQ-SUP-023
title: PyPI dependencies from requires-dist
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Python distribution's dependencies from the
`requires_dist` of the index's JSON API.

## Rationale

The JSON API answers with the whole of a release's metadata in one request.
An index that serves only the Simple API is read as
[REQ-SUP-067](REQ-SUP-067-pypi-simple-api-fallback.md) says.

## Acceptance criteria

1. Against a stub index the client returns the distribution names of
   `requires_dist`, without markers or extras.
