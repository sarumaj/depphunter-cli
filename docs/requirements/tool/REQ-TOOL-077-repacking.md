---
id: REQ-TOOL-077
title: The pack is spent whole and repacked on the ground
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

The parachute's tank **shall** be its pack: emptied at once when it is thrown,
neither draining nor filling while the canopy is open, and filling over twelve
seconds once the canopy is on the ground or gone, and it **shall** open again
only once it is full. The gauge beside the health bar **shall** show how far
along the repacking is. A landed canopy **shall** collapse and drape forward
onto the ground ahead of the walker, its lines going slack and lying on the
ground between it and where the walker came down, and fade as it is repacked.
No line of a canopy that has been let go of **shall** rise above the canopy or
leave the space between it and the harness.

## Rationale

One descent to a pack is what a parachute is, and the wait is what keeps the
jet the way up.

## Acceptance criteria

1. However long the descent, the pack is still empty at the ground.
2. It is repacked twelve seconds after landing and not before, and the walker is
   told.
3. The canopy left on the ground is gone once the pack is ready.
4. Collapsing, draped and cut away, every line stays between the canopy and the
   harness and below the canopy's highest point; draped, the lines lie on the
   ground.
