---
id: REQ-HUNT-031
uuid: 304d2674-0368-44d9-b4f2-76214ed22f9a
title: A catch without a gesture stops the bug
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

A catch that names no tool gesture, such as a finding taken from the side panel
in the map view, **shall** remove the bug where it stands without an animation.

## Rationale

There is no tool whose gesture could be shown.

## Acceptance criteria

1. Taking a finding from the side panel removes its bug at once.
