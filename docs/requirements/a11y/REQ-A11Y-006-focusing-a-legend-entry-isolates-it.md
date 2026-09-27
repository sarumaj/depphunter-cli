---
id: REQ-A11Y-006
title: Focusing a legend entry isolates it
scope: a11y
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

Focusing a legend language entry **shall** isolate that language on the map,
dimming every other building, exactly as hovering the entry does, and moving the
focus away **shall** end the isolation.

## Rationale

Keyboard users get the same preview that pointer users get.

## Acceptance criteria

1. Tabbing onto a legend entry dims every building of another language; tabbing
   off restores them.
