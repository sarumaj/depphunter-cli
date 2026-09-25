---
id: REQ-HUNT-007
uuid: 36e075da-b525-4d45-bf7a-76f7b0a7ed09
title: Opening details frees the pointer
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

When the details panel is opened from walk mode (by a second hit, `Enter` or a
catch), the system **shall** release the pointer lock so that the panel can be
operated with the mouse.

## Rationale

The panel cannot be reached with the pointer locked at the reticle.

## Acceptance criteria

1. After a second hit, the mouse cursor is free and the panel's controls can be
   clicked.
2. After `Enter` on an aimed box, the cursor is free.
