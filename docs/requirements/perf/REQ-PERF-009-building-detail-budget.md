---
id: REQ-PERF-009
title: Building details within a draw and frame budget
scope: perf
type: non-functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

Building details (REQ-CITY-033) **shall** be drawn with one instanced draw per
kind of detail whatever the number of buildings, **shall** be built for at most
600 buildings at once and at most 40 new ones a frame, and the view **shall**
check what it wants only when the walker has moved a unit or the map camera has
changed.

## Rationale

A map of thousands of buildings stays smooth only if its detail is bounded by
what is near, not by what exists.

## Acceptance criteria

1. The first update builds at most 40 buildings' details and asks for another
   frame; later updates complete the set.
2. Seven draws cover every kind of detail.
