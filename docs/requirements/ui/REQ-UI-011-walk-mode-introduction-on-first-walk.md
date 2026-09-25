---
id: REQ-UI-011
uuid: 2913e0e1-8465-4594-89ee-5a2cb96c5a27
title: Walk-mode introduction on first walk
scope: ui
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M23
  - docs/REQUIREMENTS.md M24
verification:
  - ui
  - manual
---

## Statement

The first time somebody enters walk mode, the UI **shall** open walk mode's own
introduction, with cards on moving, on the tool in each hand, on using a tool,
on the bugs, and on staying alive.

## Rationale

Walk mode is a different thing to learn, and saying it on the way in is worth
more than on a first load, where there is nothing yet to try it on.

## Acceptance criteria

1. A first walk opens the walk introduction; a second does not.
2. The cards mention the keys for moving, changing and using tools, the backpack
   and the help.
