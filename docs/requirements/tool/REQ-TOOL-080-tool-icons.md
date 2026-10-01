---
id: REQ-TOOL-080
title: Tool icons
scope: tool
type: functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

Every tool, and the empty left hand, **shall** have an icon of its own: a line
drawing of the tool in the slot's color, with the part that names it - the
rod's bobber, the extinguisher's body, a bubble, the jets - in the tool's
accent color. The slot row along the bottom of the HUD and the tool wheel
**shall** both show the same icon for a tool.

## Rationale

The row and the wheel are read at a glance in the middle of something else.
Abstract shapes had to be learned one by one; a drawing of the thing is
recognized.

## Acceptance criteria

1. Each of the eleven tools and the empty hand has a distinct icon.
2. A tool's slot in the row and its seat on the wheel carry the same icon.
3. The icons follow the slot's color in light and dark themes, and the accent
   parts keep the tool's color.
