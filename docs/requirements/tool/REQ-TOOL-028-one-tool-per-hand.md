---
id: REQ-TOOL-028
uuid: 34d87c17-a2ff-4c0d-8a97-53968c3d2b96
title: One tool of each kind, one to a hand
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M21
verification:
  - e2e
  - manual
---

## Statement

The walker **shall** carry at most one primary tool, in the right hand, and at
most one secondary tool, in the left hand, at a time; the right hand **shall**
always hold a primary tool.

## Rationale

A walker can fly with the jet backpack while netting a bug with the other hand.

## Acceptance criteria

1. A walker can fly with the jet backpack while netting a bug with the other
   hand (M21 acceptance).
2. Taking a primary tool replaces the right hand's tool and leaves the left
   hand's alone.
