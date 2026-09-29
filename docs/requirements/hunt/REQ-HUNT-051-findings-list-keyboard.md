---
id: REQ-HUNT-051
title: The findings list works from the keyboard
scope: hunt
type: functional
priority: must
status: implemented
source:
  - README.md Keyboard & mouse
verification:
  - ui
---

## Statement

The findings list **shall** be operable without a pointing device: `L` opens it
with the focus on its first row, the arrow keys, `Home` and `End` move between
rows, `Enter` or `Space` opens the focused row, `+` or `Insert` puts its finding
in the backpack or takes it out, `Esc` closes the list, and each row's button is
reachable with `Tab`. A key the list handles **shall not** also reach the map.

## Rationale

REQ-A11Y-003: navigation must not depend on a pointing device. `+` is also the
map's key for opening a level, so it must stop at the list.

## Acceptance criteria

1. Every key above does what it says on a focused row.
2. After a row's finding is put in or taken out, the focus stays on that row or
   its button.
3. A key the list does not use, such as `B`, reaches the map.
