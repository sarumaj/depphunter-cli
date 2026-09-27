---
id: REQ-HUNT-022
title: Bugs as severity-colored dots on the tracker
scope: hunt
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

The tracker **shall** draw every uncaught bug within range as a dot in its
severity color, with the most severe drawn last.

## Rationale

A critical bug must never be hidden under a note.

## Acceptance criteria

1. An uncaught bug near the walker appears as a dot in its severity color.
2. A caught bug disappears from the tracker.
