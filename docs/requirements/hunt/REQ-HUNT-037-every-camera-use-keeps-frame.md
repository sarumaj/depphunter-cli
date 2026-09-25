---
id: REQ-HUNT-037
uuid: 9744b727-4fb1-4ccc-9988-b4c40b0a3bc4
title: Every camera use keeps a frame
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M28
verification:
  - ui
  - e2e
---

## Statement

Every use of the camera **shall** keep a photograph of the frame, except as
provided by REQ-HUNT-038.

## Rationale

Needing two uses in quick succession was a thing to be told rather than a thing
anybody does, and left the ordinary use doing nothing visible.

## Acceptance criteria

1. One click on a building keeps a picture of it (M28 acceptance).
2. The camera is the only tool that keeps pictures.
