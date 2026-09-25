---
id: REQ-HUNT-002
uuid: 0dd5db7f-ef06-4eed-907d-1a9b0b4710cb
title: Modules-tagged counter in the HUD
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

The walk-mode HUD **shall** show the number of distinct modules tagged in the
session, counting each module once however often it is hit.

## Rationale

The tally is the hunt's score; hitting the same module again is the request for
its details (REQ-HUNT-005), not a second tag.

## Acceptance criteria

1. Tagging a new module increments the counter by one.
2. Hitting an already tagged module leaves the counter unchanged.
