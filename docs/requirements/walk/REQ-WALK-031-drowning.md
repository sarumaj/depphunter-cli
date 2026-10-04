---
id: REQ-WALK-031
title: Drowning without the swim ring
scope: walk
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

A walker standing in deep water with nothing to float on **shall** lose health
at 45 points per second, so that the water kills in a couple of seconds, and the
HUD **shall** tell them to get to a shore.

## Rationale

Long enough to wade ashore from the shallows, nowhere near long enough to cross
the bay; taking the swim ring off over the water ends that swim.

## Acceptance criteria

1. Taking the swim ring off over the bay kills the walker within about three
   seconds.
2. A walker who steps back onto a shore within a second survives.
