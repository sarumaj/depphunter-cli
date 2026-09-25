---
id: REQ-HUNT-006
uuid: 5b3dd685-4234-4038-8c39-5710e5c22301
title: Enter shows details of the aimed module
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
  - docs/REQUIREMENTS.md M7
verification:
  - e2e
---

## Statement

In walk mode, pressing `Enter` **shall** open the details of the box the reticle
is on; with nothing aimed at, it **shall** open the current selection or,
without one, tell the walker how to get details.

## Rationale

`Enter` is the keyboard equivalent of the second shot.

## Acceptance criteria

1. With the reticle on a building, `Enter` opens that building's details.
2. With nothing aimed at and nothing selected, a HUD message explains how to get
   details.
