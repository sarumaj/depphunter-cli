---
id: REQ-EXT-010
uuid: 4762bdcf-e4e3-47f6-a7fb-39a9a7253a06
title: Backpack view lists the catch
scope: ext
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M15
  - README.md The panel beside the code
verification:
  - extension
---

## Statement

The Backpack view **shall** list the findings of the server's session backpack,
ordered by severity, most severe first, with those absent from the most recent
scan marked as resolved and placed at the end, and **shall** update when the
server announces a backpack change.

## Rationale

A finding caught in walk mode is worked through in the editor, where the fixing
happens; the order is the order it would be worked through in.

## Acceptance criteria

1. Catching a bug in walk mode makes it appear in the panel's backpack.
2. A `PUT /api/backpack` from another client is shown without a manual refresh.
3. Each entry shows its location (`file:line`) and severity, or `fixed`.
