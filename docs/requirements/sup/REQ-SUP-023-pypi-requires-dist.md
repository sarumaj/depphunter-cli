---
id: REQ-SUP-023
uuid: 12c6e336-6b36-46a7-9c20-96625b124865
title: PyPI dependencies from requires-dist
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

The index client **shall** read a Python distribution's dependencies from the
`requires_dist` of the index's JSON API.

## Rationale

A simple index (PEP 503) serves file listings and no metadata.

## Acceptance criteria

1. Against a stub index the client returns the distribution names of
   `requires_dist`, without markers or extras.
