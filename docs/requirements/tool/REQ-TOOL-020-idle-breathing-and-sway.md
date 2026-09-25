---
id: REQ-TOOL-020
uuid: 4a1fcdcf-cef3-4076-a9b1-20ad92b92052
title: Held tool breathes and sways
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M14
verification:
  - ui
  - manual
---

## Statement

A held tool **shall** move with a slow breathing motion while the walker stands
still and **shall** rise, fall and swing across with the walker's steps while
walking, without a jump in phase when the pace changes.

## Rationale

A tool that hangs rigidly in the frame looks pasted on.

## Acceptance criteria

1. Standing still, the tool moves slowly and slightly.
2. Breaking into a run an hour into a session does not make the tool jerk (M27
   acceptance).
