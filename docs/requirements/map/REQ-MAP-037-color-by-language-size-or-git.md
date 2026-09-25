---
id: REQ-MAP-037
uuid: c7365a9f-32cd-4dda-a422-90fa4ca809ba
title: Color by language, size or git history
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
  - docs/REQUIREMENTS.md M5
verification:
  - manual
  - ui
---

## Statement

The UI **shall** color buildings, symbols and districts by one selectable
quantity: language (categorical), file size (sequential, on a square-root scale)
or, once the history has loaded, one of the git history measures (sequential).

## Rationale

Different questions need different encodings; one at a time keeps each readable.

## Acceptance criteria

1. Switching Color to Size recolors buildings along the sequential ramp and the
   legend shows the size ramp.
2. History modes are disabled until the history has loaded, and fall back to
   language without it.

## Notes

The history modes themselves are REQ-HIST requirements.
