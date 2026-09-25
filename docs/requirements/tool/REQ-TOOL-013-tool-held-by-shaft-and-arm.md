---
id: REQ-TOOL-013
uuid: 089023e4-9929-401a-ad54-76f3eaa342c0
title: Tool held by shaft and arm directions
scope: tool
type: functional
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

Each tool **shall** state the direction of the shaft through the fist and the
direction the arm leaves the frame, the hand **shall** be oriented from those
two directions, and the tool **shall** be a child of the hand.

## Rationale

A tool is held rather than placed beside a hand; as a child, it moves with the
hand.

## Acceptance criteria

1. Every held tool's shaft runs through the closed fist.
2. Moving the hand in a gesture moves the tool with it.
