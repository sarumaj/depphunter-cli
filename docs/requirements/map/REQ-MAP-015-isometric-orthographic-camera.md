---
id: REQ-MAP-015
uuid: 230a6a29-f5f1-4f49-bf73-137bc07901ae
title: Isometric orthographic camera
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
verification:
  - manual
---

## Statement

The system **shall** show the map through an orthographic camera at the true
isometric elevation (arccos(1/sqrt 3), about 35.26 degrees) and an azimuth of 45
degrees plus a multiple of 90 degrees.

## Rationale

An isometric projection keeps sizes comparable across the whole map, which a
perspective view does not.

## Acceptance criteria

1. On load, the map is seen isometrically: parallel edges stay parallel and
   equal heights look equal anywhere on screen.
