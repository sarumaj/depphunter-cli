---
id: REQ-WALK-045
uuid: 2c2c513e-404c-4bca-96b6-3ffde808883a
title: A click on the map walks on
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

While the walker is held, a left click on the map **shall** request pointer lock
and resume walking, and **shall not** scope, fire or hold the trigger.

## Rationale

The HUD and the help had always said so; the click was dropped before anything
recorded it, so a held walker could only be started again with a key.

## Acceptance criteria

1. One click on the map after `Esc` walks on again.
