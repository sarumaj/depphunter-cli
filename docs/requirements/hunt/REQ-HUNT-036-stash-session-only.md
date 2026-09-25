---
id: REQ-HUNT-036
uuid: 98feb8ad-21bb-446c-89ca-530404e11f68
title: Photographs live for the session only
scope: hunt
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M25
verification:
  - inspection
---

## Statement

The stash **shall** hold photographs in memory for the session only and **shall
not** persist them in browser storage.

## Rationale

A finding id belongs in a store that outlives the tab; a megabyte of PNG does
not.

## Acceptance criteria

1. After a reload the stash is empty.
2. No photograph is written to local storage.

## Notes

The stash keeps at most 24 photographs and lets go of the oldest; the cap is not
in the design log.
