---
id: REQ-UI-014
uuid: cdfe5067-037b-4f95-8d21-0d0dfe425746
title: Menus wait for the pointer lock release
scope: ui
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M24
verification:
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

Partial: opening a menu releases the pointer and holds the walker (`readAway` in
app.js), and `X` opens the export menu at the first press, but nothing waits for
`pointerlockchange`: the menus' click-outside handlers close the menu on any
pointer event outside it, including one delivered to the canvas while the lock
is still held.
