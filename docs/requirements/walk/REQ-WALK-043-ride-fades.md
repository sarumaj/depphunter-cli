---
id: REQ-WALK-043
uuid: 1c8edb61-cca1-4593-b84d-c43856a24fbb
title: View ride fades in and out
scope: walk
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M27
verification:
  - manual
---

## Statement

The ride of the view (REQ-WALK-036) **shall** fade in and out when what carries
the walker starts or stops, rather than switching on or off.

## Rationale

Ending the swell the moment the walker steps ashore dropped the view by however
far up it had got, in one frame.

## Acceptance criteria

1. The view does not drop as the skimmers reach a shore.
