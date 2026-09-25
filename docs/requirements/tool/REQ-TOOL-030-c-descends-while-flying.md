---
id: REQ-TOOL-030
uuid: 49e1e7a6-ad67-4dea-8433-d5d3b4f80081
title: C keeps its meaning while flying
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M22
verification:
  - e2e
---

## Statement

While flying, `C` **shall** move the walker down and **shall not** use the
off-hand tool.

## Rationale

`C` is how a flying walker goes down.

## Acceptance criteria

1. Holding `C` while flying descends and does not fire a burst.
