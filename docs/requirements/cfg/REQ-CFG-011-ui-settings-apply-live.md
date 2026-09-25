---
id: REQ-CFG-011
uuid: 90c02192-389e-434f-a638-aff1835d5716
title: View settings apply live
scope: cfg
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The UI **shall** apply a change of a view setting (color-by, height scale,
theme, style and the filters) to the map immediately, without a reload and
without re-running the command.

## Rationale

The user determines interactively what is shown (goal 2 of the product).

## Acceptance criteria

1. Changing color-by recolors the buildings and the legend at once.
2. Changing the height scale relayouts the map at once.
3. Changing the theme or style repaints the map at once.
4. Changing a filter hides or shows the affected buildings at once.

## Notes

No automated UI test covers the live application. The wiring lives in
`app.js`, which starts the whole page (WebGL scene, server connection, every
control) when it is imported, so it cannot be driven headlessly at reasonable
cost; the pieces it calls - `computeVisibility`, `layout`, `languageColors` -
are covered by the `web/uitest` tests of REQ-MAP-032 to REQ-MAP-036.
