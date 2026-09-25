---
id: REQ-SUP-005
uuid: bc72d638-fec3-4b72-885f-0e4809abd23f
title: Floating badge in the side panel
scope: sup
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The side panel **shall** show a "floating" warning badge for a floating package
and label its version as floating.

## Rationale

The color on the map says that something moves; the panel says what.

## Acceptance criteria

1. Selecting a floating package shows the "⚠ floating" badge and the version
   stat labelled "version (floating)".
2. Selecting a pinned package shows neither.
