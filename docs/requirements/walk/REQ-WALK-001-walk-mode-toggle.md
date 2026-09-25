---
id: REQ-WALK-001
uuid: 3c5b55c9-d09b-4ee8-bcc9-be813e76afbb
title: Walk mode toggle
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M7
verification:
  - e2e
  - manual
---

## Statement

The UI **shall** offer a walk mode, entered with the `V` key or the Walk toolbar
button, that shows the current map in first person. While walk mode is active,
`V` and `M` **shall** leave it and return to the isometric map, as **shall**
`Esc` when the pointer is not captured.

## Rationale

The same map seen on foot gives a sense of scale and neighbourhood that the
isometric overview does not; one key in and out keeps the two views one gesture
apart.

## Acceptance criteria

1. Pressing `V` on the map enters walk mode with the walker standing on the map.
2. Pressing `V` or `M` in walk mode returns to the isometric map.
3. Clicking the Walk toolbar button toggles walk mode.

## Notes

Entering again resumes where the walker last stood unless a different building
has been selected in the meantime (`walkTarget`).
