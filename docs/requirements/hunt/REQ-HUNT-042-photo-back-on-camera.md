---
id: REQ-HUNT-042
title: A photograph put back on the camera
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

From the street, a photograph chosen in the stash **shall** be shown on the
camera's back screen in place of the live view, with the camera brought to hand
if it is not already there and raised to the walker's face. While it is shown,
no label **shall** be drawn over the view, no reticle, and no hover card shown
for what the crosshair is on.

## Rationale

A photograph can be looked at where it was taken.

## Acceptance criteria

1. Choosing to show a photograph puts the camera in hand and raises it with the
   picture on its screen.
2. While the photograph is up, no file or directory label, no reticle and no
   hover card is on screen; they come back once it is put away.
