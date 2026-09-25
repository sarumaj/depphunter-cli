---
id: REQ-UI-013
uuid: fb63a7db-2bf2-4ed5-9f03-d98251f6fc0d
title: Walk introduction shown with the mouse free
scope: ui
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M24
  - docs/REQUIREMENTS.md M25
verification:
  - ui
  - manual
---

## Statement

On a first walk, the UI **shall** open the walk introduction before capturing
the pointer, hold the walker still while it is open, and capture the pointer at
the reticle only once it has been closed.

## Rationale

A dialog and a captured reticle at the same time is a mouse fighting itself;
taking the pointer and handing it back left it stuck.

## Acceptance criteria

1. A first walk explains itself with the mouse free, and the introduction can be
   clicked through without the mouse being taken.
2. Closing it, however it is closed, captures the pointer exactly once.

## Notes

M23 captured the pointer on entry and handed it back while the introduction was
open; M24 and M25 replaced that with the order above.
