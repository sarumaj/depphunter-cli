---
id: REQ-HUNT-033
uuid: 9e7f231f-69b2-4db6-b2d8-6781bea2bb78
title: Photographs are PNGs like the image export
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M24
verification:
  - e2e
---

## Statement

A photograph **shall** be a PNG image rendered by the same code path as the
map's own image export.

## Rationale

One way of turning the view into a file.

## Acceptance criteria

1. A saved photograph is a PNG file of the view (M24 acceptance: the camera
   saves a file).
