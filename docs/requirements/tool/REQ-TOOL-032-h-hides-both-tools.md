---
id: REQ-TOOL-032
uuid: 5abd6b3b-9ce9-41fe-9b31-01caa8237a15
title: H stows both hands
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

Pressing `H` **shall** stow both held tools from view, and pressing it again
**shall** bring them back.

## Rationale

An empty view is worth having for screenshots and for looking down.

## Acceptance criteria

1. After `H` no hand is drawn; after a second `H` both are back.

## Notes

Only the drawing is stowed: the tools keep working, and a shot leaves from the
walker's eye.
