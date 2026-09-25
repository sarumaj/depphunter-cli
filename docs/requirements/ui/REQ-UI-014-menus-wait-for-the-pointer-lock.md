---
id: REQ-UI-014
uuid: cdfe5067-037b-4f95-8d21-0d0dfe425746
title: Menus wait for the pointer lock release
scope: ui
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M24
verification:
  - ui
  - manual
  - e2e
---

## Statement

A menu opened from walk mode that needs the pointer **shall** not be closed by
pointer events delivered to the locked canvas before the pointer-lock release
has taken effect.

## Rationale

Releasing a pointer lock is asynchronous, and a click-outside handler that sees
an event on the canvas meanwhile shuts the menu, which then opens only on the
second try.

## Acceptance criteria

1. In walk mode, `X` opens the export menu at the first press, and it stays
   open.

## Notes

Fixed: `whenUnlocked` (dom.js) releases the pointer lock and opens a menu only
once `pointerlockchange` reports the lock gone, or after a 250 ms safety
timeout, and can be called off meanwhile. The Filters and Export menus, the
backpack and the photographs open through it, and the click-outside handlers look
only at a menu that is showing, so an event still delivered to the locked canvas
cannot close one. `web/uitest/menus.test.mjs` tests the helper.
