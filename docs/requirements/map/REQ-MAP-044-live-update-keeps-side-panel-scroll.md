---
id: REQ-MAP-044
uuid: 197ff909-fb71-4c08-b591-1bdb76e535e5
title: Live update keeps side panel scroll position
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
  - e2e
---

## Statement

When a live update redraws the side panel for the node it already shows, the UI
**shall** keep the panel's scroll position instead of scrolling back to the top.

## Rationale

A reader deep in a file's source must not lose their place on every save.

## Acceptance criteria

1. With `--watch`, scrolling a file's source down and saving another file leaves
   the panel where it was.
