---
id: REQ-SUP-039
uuid: 4744ec41-6bef-4d51-a35b-e6988248b285
title: A private package is asked of the machine's index
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** ask about a private package as usual when its index
is one this machine's configuration names rather than the public one.

## Rationale

A company registry knows about the package already.

## Acceptance criteria

1. A private package whose ecosystem resolves from a machine-configured index is
   resolved from it.
