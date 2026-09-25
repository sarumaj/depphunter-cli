---
id: REQ-TOOL-016
uuid: 662a8f3d-6c71-4e33-aecb-98a60ba0b581
title: Camera built to standard
scope: tool
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M14
verification:
  - manual
---

## Statement

The camera **shall** have a prism hump, a focus ring, a lens hood and a shutter
button, and **shall** be held in both hands.

## Rationale

All tools are built to the same standard of detail.

## Acceptance criteria

1. The camera shows a prism hump, a focus ring, a hood and a shutter button.
2. Both hands hold the camera.

## Notes

Partial: the camera is held in the right hand only, by the body's own grip, the
way one is held to fire it one-handed (tools.js `grasps` is called once). It
also carries a live-view screen on its back, which the design log mentions only
under M27.
