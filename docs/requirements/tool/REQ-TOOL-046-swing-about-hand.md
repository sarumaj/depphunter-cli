---
id: REQ-TOOL-046
uuid: 7051f523-4459-40fd-a2e7-33710a2d04c5
title: Tools swung about the hand
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M24
verification:
  - manual
---

## Statement

A swung gesture **shall** rotate the tool about the hand holding it, not about
the viewmodel's origin.

## Rationale

Rotating about a point in mid-air inboard of the fist turns a net's head into
the pivot.

## Acceptance criteria

1. A net swings about the hand (M24 acceptance).
