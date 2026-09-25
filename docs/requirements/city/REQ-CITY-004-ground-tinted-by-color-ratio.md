---
id: REQ-CITY-004
uuid: 063ebabb-9ecd-4ca7-aba9-49ef465a4d97
title: Streets and lawns tinted by the box color ratio
scope: city
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

Street and lawn shading **shall** be multiplied by the ratio of the box's color
to its kind's usual color, clamped to 0.4-2.2, so nesting levels, hover and
flashes remain visible on terraces and shores.

## Rationale

Streets and lawns have colors of their own; without the tint a hovered or
flashed terrace would not show.

## Acceptance criteria

1. Hovering a terrace changes the color of its streets.
2. Nested terraces alternate in shade.
