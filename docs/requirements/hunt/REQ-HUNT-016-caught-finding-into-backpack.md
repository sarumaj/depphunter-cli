---
id: REQ-HUNT-016
uuid: e93f3d04-a55b-4b46-a37f-d8060cdb551b
title: Caught findings go into the backpack
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A caught bug's finding **shall** be added to the backpack, at most once per
finding.

## Rationale

The point of catching a finding is to come back to it.

## Acceptance criteria

1. Catching a bug in walk mode makes its finding appear in the backpack.
2. Catching the same finding again does not add a second entry.
