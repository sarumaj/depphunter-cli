---
id: REQ-HUNT-045
uuid: b0018f73-06ff-4c0e-b10b-d73b2faaec47
title: Reaching for a tool ends the showing
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

Choosing another tool while a photograph is shown **shall** end the showing
without restoring the previously held tool.

## Rationale

The walker has just said what they want in their hand.

## Acceptance criteria

1. Pressing a tool key during the showing puts that tool in hand, not the one
   held before the photograph.
