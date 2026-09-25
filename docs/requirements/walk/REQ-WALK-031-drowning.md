---
id: REQ-WALK-031
uuid: cc8ec15d-c55f-42e9-baa2-902d6b677aca
title: Drowning without floats
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M21
  - docs/REQUIREMENTS.md M26
verification:
  - manual
---

## Statement

A walker standing in deep water with nothing to float on **shall** lose health
at 45 points per second, so that the water kills in a couple of seconds, and the
HUD **shall** tell them to get to a shore.

## Rationale

Long enough to wade ashore from the shallows, nowhere near long enough to cross
the bay; stowing the skimmers over the water ends that walk.

## Acceptance criteria

1. Stowing the skimmers over the bay kills the walker within about three
   seconds.
2. A walker who steps back onto a shore within a second survives.
