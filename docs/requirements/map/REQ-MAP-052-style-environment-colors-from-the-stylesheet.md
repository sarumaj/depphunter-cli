---
id: REQ-MAP-052
uuid: 30fa69af-68bc-4d45-b6e2-cd886a97741f
title: Style environment colors from the stylesheet
scope: map
type: functional
priority: must
status: implemented
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

Fixed: the circuit and galaxy styles have a light set of environment values and a
dark set, themed like the base tokens: the dark set applies under
`prefers-color-scheme: dark` unless `data-theme="light"`, and under
`data-theme="dark"`.
