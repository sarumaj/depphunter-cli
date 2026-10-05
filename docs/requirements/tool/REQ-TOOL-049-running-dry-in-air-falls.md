---
id: REQ-TOOL-049
title: Running dry in the air is a fall
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

When the jet backpack's tank runs dry in the air, the walker **shall** stop
flying and fall; when the swimmer runs out of wind in the swim ring
(REQ-WALK-062), the water **shall** stop holding the walker until they have a
quarter of it back.

## Rationale

Running out has a consequence.

## Acceptance criteria

1. Emptying the jet tank in flight makes the walker fall.
2. A swimmer out of wind in the swim ring goes under.
