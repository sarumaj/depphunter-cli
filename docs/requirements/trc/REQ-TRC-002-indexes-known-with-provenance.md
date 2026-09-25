---
id: REQ-TRC-002
uuid: 84e0357f-d3b5-4966-8a36-acb14ef5403b
title: Indexes known, with provenance and trust
scope: trc
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
---

## Statement

The report **shall** list every index the run knew about with its ecosystem,
URL, scope, where it was learned from (this machine, the repository,
`--trust-index`, the public default or an image reference) and whether anything
vouches for it.

## Rationale

Which indexes a run knew about is the question underneath every surprising index
on the map.

## Acceptance criteria

1. A run whose repository names an npm index lists it with origin "the
   repository" and not trusted, beside the public default.
2. An index URL is listed without any credential it was written with.
