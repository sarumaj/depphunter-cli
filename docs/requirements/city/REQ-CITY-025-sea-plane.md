---
id: REQ-CITY-025
uuid: 2a477065-d666-45c5-9bb8-6a9636c725ff
title: Sea around the isometric map
scope: city
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The isometric map **shall** be surrounded by a water plane just above the land
boxes' base, 40 map sizes across, using walk mode's ripple shader, which fades
ripples to their mean where they are smaller than a pixel.

## Rationale

The space around the islands is sea; 40 map sizes are more than zooming out or
panning can reveal.

## Acceptance criteria

1. Panning and zooming the map to its limits never shows the edge of the sea.
2. Zoomed out, the sea shows no moiré.
