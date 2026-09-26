---
id: REQ-UI-015
uuid: 206c2718-8b90-4b9f-aee2-3d574b047e58
title: The details can be maximized
scope: ui
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

The details panel **shall** have a button, beside its close button, that
maximizes it over the whole of the map's room and, pressed again, puts it back
beside the map. The button **shall** say which of the two it will do. The
choice **shall** be remembered in the browser, so the next panel opens the same
way, and the map **shall not** be resized either way.

## Rationale

The panel is sized to leave the map in view, which is too narrow for reading a
long file or looking at a picture properly.

## Acceptance criteria

1. The button maximizes the panel over the map and restores it again.
2. Its label and `aria-pressed` say whether it is maximized.
3. After a reload, a panel opens maximized if it was left so, and beside the
   map otherwise.
4. Without browser storage, the choice lasts until the page is reloaded.
