---
id: REQ-HUNT-014
uuid: 79e5c32b-29a5-461a-a1c6-889cd47ed6d0
title: Crosshair names the bug it is on
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - e2e
---

## Statement

While the reticle is on a bug that the primary tool can catch, the HUD **shall**
show that bug's severity and finding title.

## Rationale

The walker must know what would be caught before using the tool.

## Acceptance criteria

1. With the reticle on a bug, the HUD shows a severity dot, the severity and the
   finding title.
2. Moving off the bug hides the label.
