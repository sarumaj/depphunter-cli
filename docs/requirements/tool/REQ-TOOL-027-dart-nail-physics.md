---
id: REQ-TOOL-027
uuid: 45015b57-29d4-4006-8689-875a82167a39
title: Dart and nail told apart by physics
scope: tool
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M20
verification:
  - ui
---

## Statement

The tracking dart's flight **shall** be lobbed, slow and steer towards the wall
ahead of it; the nail's **shall** be flat, fast (more than twice the dart's
speed) and scatter, and **shall not** steer.

## Rationale

The two launchers are told apart by their physics rather than by their models.

## Acceptance criteria

1. The nail is more than twice as fast as the dart.
2. The dart's lob is more than ten times the nail's.
3. Only the dart steers and only the nail scatters.
