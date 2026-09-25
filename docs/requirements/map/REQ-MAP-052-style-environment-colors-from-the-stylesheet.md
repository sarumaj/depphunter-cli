---
id: REQ-MAP-052
uuid: 30fa69af-68bc-4d45-b6e2-cd886a97741f
title: Style environment colors from the stylesheet
scope: map
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M14
verification:
  - manual
  - inspection
---

## Statement

Each style **shall** take its environment colors (ground, water, sky) from the
stylesheet, with separate values for the light and the dark theme.

## Rationale

The environment is the style's, and both themes still choose their own values.

## Acceptance criteria

1. Every style's ground, water and sky are CSS custom properties.
2. Switching the theme under each style changes its environment colors.

## Notes

Partial: the colors come from the stylesheet, but the circuit and galaxy styles
define one set of environment values that override both themes
(`:root[data-style=...]` after the theme rules), so under those styles the theme
does not change the ground, water or sky.
