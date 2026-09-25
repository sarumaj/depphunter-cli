---
id: REQ-WALK-026
uuid: e4218929-037b-440e-96a3-89a2737c992e
title: Flight follows the view
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

While the walker is flying, `W`/`S` **shall** move along the direction looked
at, `A`/`D` **shall** strafe level, and `Space`/`C` **shall** add straight up
and down movement. On foot, movement **shall** stay level.

## Rationale

Look down and press `W` to dive; on foot the ground carries the walker.

## Acceptance criteria

1. Looking down while flying and pressing `W` descends.
2. Pressing `Space` while flying climbs; `C` descends.
3. On foot, looking down and pressing `W` moves level.

## Notes

Flight is provided by the jet backpack (scope `tool`, REQ-TOOL-023); there is no
flight key any more (REQ-WALK-048).
