---
id: REQ-EXP-011
uuid: e5f427e3-d8c8-4377-a2bd-b57ba5d62f56
title: PNG image of the map as shown
scope: exp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M7
verification:
  - manual
---

## Statement

The UI **shall** save the map as shown as a PNG image at the screen resolution
(device pixels), including the visible map labels, from Export → PNG image and
from the `P` key.

## Rationale

A picture of the view is the simplest thing to put into a document or a chat.

## Acceptance criteria

1. Pressing `P` downloads `<project>.png` whose pixel size equals the canvas
   size in device pixels.
2. The labels visible on screen appear in the image.
3. The PNG export matches the view.
