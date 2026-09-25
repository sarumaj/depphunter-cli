---
id: REQ-WALK-021
uuid: 853dd8e7-ba33-4b8a-9338-c008575bd0cc
title: Walker held still while reading
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

While the details panel, the backpack, the photographs, a menu or a dialog has
the pointer, the walker **shall** be held: it **shall not** move, look, aim, use
a tool or change field of view, held keys **shall** be released, and the scene
**shall** keep rendering. `Enter` or a click on the map **shall** resume walking
and close what was being read.

## Rationale

A freed cursor and a reticle both steering the same scene are two controls
fighting over one view; the scene keeps rendering so what is being read about
stays on screen.

## Acceptance criteria

1. Opening a building's details while holding `W` does not keep walking.
2. The HUD shows "reading" while held.
3. `Enter` or a click on the map resumes walking and closes the panel.
