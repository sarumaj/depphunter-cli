---
id: REQ-HUNT-026
uuid: b9ed8a1d-a360-4a49-bf6f-3dba13ed34d2
title: Distance and finding of the nearest bug
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

Under the tracker the HUD **shall** show the distance to the nearest uncaught
bug and the severity and title of its finding, or that all bugs are caught.

## Rationale

The walker needs to know how far to go and what they will find.

## Acceptance criteria

1. The label reads the rounded distance, the severity and the title of the
   nearest bug.
2. Once every bug is caught the label says so.

## Notes

When fires are burning, the label reports the nearest fire instead.
