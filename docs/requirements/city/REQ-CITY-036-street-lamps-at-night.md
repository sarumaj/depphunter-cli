---
id: REQ-CITY-036
title: Street lamps lit at night
scope: city
type: functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

In the city style at night, every street lamp **shall** glow around its head
and **shall** throw a pool of its light onto the pavement under it, brightest
under the head and fading to nothing at its rim; by day **shall** neither be
drawn. The circuit board's LEDs **shall** stay lit by day and night alike.

## Rationale

The lamps line every terrace's edge; at night, dark lamps over a dark street
are lamps nobody can see, and a city whose windows are lit but whose streets
are not reads as switched off. A board's LEDs are lit whatever the time of day.

## Acceptance criteria

1. In the dark theme each lamp's head glows and a pool of light lies under it.
2. In the light theme no glow and no pool are drawn.
3. The glow is a halo about the head, not a ball larger than the lamp is tall.
