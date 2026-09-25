---
id: REQ-TOOL-049
uuid: 32214936-bb06-4d6f-b1d2-1ce1de68bda3
title: Running dry in the air is a fall
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

When the jet backpack's tank runs dry in the air, the walker **shall** stop
flying and fall; when the skimmers run dry on the water, the water **shall**
stop holding the walker.

## Rationale

Running out has a consequence.

## Acceptance criteria

1. Emptying the jet tank in flight makes the walker fall.
