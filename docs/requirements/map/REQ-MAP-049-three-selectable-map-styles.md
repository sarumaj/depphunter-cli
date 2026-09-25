---
id: REQ-MAP-049
uuid: ddab0321-0365-4985-ac74-5279a2a5d492
title: Three selectable map styles
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M14
verification:
  - integration
  - manual
---

## Statement

The system **shall** dress the map in one of three styles - `city` (default),
`circuit` (a printed circuit board) and `galaxy` - chosen by `--style` or
`ui.style` and switchable from the toolbar while the map is open.

## Rationale

A style is a look, chosen to taste, not a different map.

## Acceptance criteria

1. `--style circuit` opens the map as a circuit board.
2. Changing the Style menu restyles the open map without a reload.
3. An unknown style is rejected by the command.
