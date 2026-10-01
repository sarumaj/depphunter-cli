---
id: REQ-HUNT-036
title: Photographs live for the session only
scope: hunt
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The stash **shall** hold photographs in memory for the session only and **shall
not** persist them in browser storage. It **shall** keep at most 24, letting go
of the oldest, and its panel **shall** say so beside the count.

## Rationale

A finding id belongs in a store that outlives the tab; a megabyte of PNG does
not.

## Acceptance criteria

1. After a reload the stash is empty.
2. No photograph is written to local storage.
3. The panel says how many it keeps; a 25th photograph drops the first.
