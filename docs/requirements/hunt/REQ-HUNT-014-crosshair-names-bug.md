---
id: REQ-HUNT-014
title: Crosshair names the bug it is on
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

While the reticle - or, for a tool that throws, the marker where its shot would
land (REQ-TOOL-082) - is on a bug that the primary tool can catch, the HUD
**shall** show that bug's severity and finding title.

## Rationale

The walker must know what would be caught before using the tool.

## Acceptance criteria

1. With the reticle on a bug, the HUD shows a severity dot, the severity and the
   finding title.
2. Moving off the bug hides the label.
