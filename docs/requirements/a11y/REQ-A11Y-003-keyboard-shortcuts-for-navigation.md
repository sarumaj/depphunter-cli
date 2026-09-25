---
id: REQ-A11Y-003
uuid: aab167ce-c02f-4fe4-9f28-9d6e6d5611c1
title: Keyboard shortcuts for navigation
scope: a11y
type: functional
priority: must
status: implemented
verification:
  - manual
  - e2e
---

## Statement

The UI **shall** provide keyboard shortcuts for navigating the map: `Q`/`E`
rotate, `Home` fits, `+`/`-` change the depth, `Enter` expands or collapses the
selection, `Backspace` selects its parent, `/` searches, `Esc` clears the
selection, and `?` opens the help that lists them.

## Rationale

Navigation must not depend on a pointing device.

## Acceptance criteria

1. Every shortcut listed in the help's map section works while the map has the
   focus.
2. Shortcuts are not triggered while typing in a text field.
