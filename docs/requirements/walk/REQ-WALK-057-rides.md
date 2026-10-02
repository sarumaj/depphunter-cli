---
id: REQ-WALK-057
title: Riding the playgrounds, empty-handed
scope: walk
type: functional
priority: may
status: implemented
verification:
  - unit
  - manual
---

## Statement

With both hands empty - the tools put away (REQ-TOOL-032) and nothing carried
in the left - a walker within about half a unit of a swing's seat, a
roundabout's edge, a seesaw's or spring rider's seat or a slide's ladder
**shall** be told that a click gets on, and a click **shall** get them on:

- on a swing they sit on the seat and swing with it, and W pumps it higher,
  S slows it;
- on a roundabout they stand on its deck and turn with it, and W pushes it
  round faster, S slows it;
- on a seesaw W pushes off from the bottom, and on a spring rider W rocks it;
- a slide is climbed, crossed and slid down by itself, faster down the chute,
  and left at its foot.

Space **shall** jump off, carried on at the speed the seat or the deck had: off
a swing in full flight a long way. A ride let go of **shall** come to rest by
itself. With a tool in hand nothing is ridden, and the walker **shall** be told
how to free their hands; taking a tool out on a ride gets off it.

## Rationale

A playground that could only be walked round was scenery. Riding it is a
reason to stop in a park, and a swing let go of at the top of its arc is a way
across one.

## Acceptance criteria

1. With a tool in hand, standing by a swing says how to put it away and a
   click does not get on; empty-handed, a click does.
2. Pumped for twelve seconds a swing goes over 0.7 radians out, and Space at
   the bottom of its arc throws the walker forward and up.
3. A walker pushing a roundabout turns with it.
4. A slide takes the walker up its ladder and down to its foot.
