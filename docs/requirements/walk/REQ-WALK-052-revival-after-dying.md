---
id: REQ-WALK-052
uuid: ed1686e8-440a-40c1-9eed-dc6b3ff1770c
title: Getting up after dying
scope: walk
type: functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

When walk mode is entered again after a walk that ended in dying, the view
**shall** start on the ground looking up, under the red of dying, and **shall**
rise to eye height while the red clears, ending on the spot the walker is placed
on with the view they would otherwise have started with. Any key or click
**shall** cut it short, and it **shall not** play when the page is asked for
reduced motion. The tool and HUD **shall** stay hidden until the walker is up.
A walker who drowned **shall** get up on the nearest shore or street instead of
in the water, facing inland.

## Rationale

Coming back after dying is the other end of the death that REQ-WALK-033 lets the
walker watch; a cut straight to a standing walker at full health does not read
as coming back.

## Acceptance criteria

1. After dying, walking in again starts with the eye near the ground, looking up,
   with the red over the view.
2. The red only clears, and the walker ends standing on the landing spot, facing
   as that spot faces.
3. A walk that did not end in dying starts on foot at once.
4. After drowning, the walker gets up on the nearest ground that is not water and
   not a roof, a step in from its edge, facing inland; after dying anywhere else,
   where they fell.

## Notes

It lasts `REVIVAL` seconds (`walk.js`); `revivalAt` gives the path. The first
arrival (REQ-WALK-051) takes precedence on a page's first walk.
