---
id: REQ-CITY-010
uuid: 2532109a-86ad-4b77-87ef-390618008d00
title: Parks with paths
scope: city
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M8
  - docs/REQUIREMENTS.md M10
verification:
  - manual
---

## Statement

Free space of a terrace farther than a street's width (0.42 units plus the
sidewalk) from every obstacle **shall** be drawn as a park: a mown lawn crossed
by gravel paths every 2.2 units, bordered by a sidewalk.

## Rationale

Holes left by the packing would otherwise be vast expanses of asphalt.

## Acceptance criteria

1. A large empty area on a terrace shows a lawn with a grid of paths.
