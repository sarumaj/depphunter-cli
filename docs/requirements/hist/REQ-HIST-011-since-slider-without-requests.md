---
id: REQ-HIST-011
uuid: 076b2f2d-ac1a-4275-b667-c1618c8afac5
title: Since slider without requests
scope: hist
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

In the Commits, Lines changed and Authors modes the UI **shall** offer a
**Since** slider over the history time range, in steps of one day, that
restricts the counted changes and recolors the map live while dragging, without
any request to the server.

## Rationale

Exploring different periods has to be instant to be useful.

## Acceptance criteria

1. Dragging the slider recolors the map and updates the date label; the network
   log shows no request.
