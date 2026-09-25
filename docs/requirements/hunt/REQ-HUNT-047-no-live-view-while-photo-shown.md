---
id: REQ-HUNT-047
uuid: 3eec0af8-fd11-47e8-be29-55c2657230e8
title: No live-view pass while a photograph is up
scope: hunt
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M27
verification:
  - ui
---

## Statement

While a photograph is shown on the camera, the system **shall not** render the
second pass over the map that produces the camera's live view.

## Rationale

The live view is a whole second render of the map; a still picture needs none.

## Acceptance criteria

1. With a photograph up, the live view is not rendered.
2. Taking the photograph off restores the live view.
