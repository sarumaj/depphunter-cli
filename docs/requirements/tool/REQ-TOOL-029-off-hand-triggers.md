---
id: REQ-TOOL-029
title: Separate triggers for each hand
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

The left mouse button **shall** use the right hand's tool, and `F`, `C` or the
middle mouse button **shall** use the left hand's tool.

## Rationale

A tool in the off hand needs a trigger of its own, and the right button is
already the scope.

## Acceptance criteria

1. The grapple can be fired at all: `F`, `C` and the middle
   button each fire it.
2. A left click uses the primary tool.
