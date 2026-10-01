---
id: REQ-CITY-035
title: Facade detail up close
scope: city
type: functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

Within a few steps of a facade, the city style **shall** draw detail that is
below a pixel from further away: a fine grain in the wall's render, window
frames lit along their head and shaded along their sill, panes in the frame's
shadow along their head and one jamb, a streak of light across the glass, and
on the entrance doors a kick plate and a brass pull on each leaf. Each **shall**
fade in with the size it is drawn at, so a facade further away is drawn as
before.

## Rationale

A walker stopped at a door looks at it from closer than anything else is seen
from; a wall of one flat tone and windows cut flat into it read as crude at
that distance, however sharp their edges.

## Acceptance criteria

1. Standing within a story's height of a facade, its wall shows grain and its
   windows sit back behind lit and shaded frames.
2. From across the street the facade is drawn as it was.
