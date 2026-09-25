---
id: REQ-WALK-047
uuid: 1aed0935-2b94-49cc-aded-eaa1220ff868
title: Walking where pointer lock is refused
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

Where pointer lock is refused for good (a sandboxed frame, or two refusals
without a lock in between), dragging **shall** turn the view, a click **shall**
use the tool, the walker **shall not** be held for lacking the lock, and the HUD
**shall** say once that the mouse cannot be captured.

## Rationale

Walk mode is still worth having without the lock; asking again on every click
would make a click that never does anything else.

## Acceptance criteria

1. In an editor webview that withholds pointer lock, dragging turns the view and
   a click uses the tool.
