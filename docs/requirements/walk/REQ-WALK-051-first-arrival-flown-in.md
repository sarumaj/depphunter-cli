---
id: REQ-WALK-051
uuid: f09279b2-7989-4664-9cf6-949be20fc66e
title: First arrival flown in
scope: walk
type: functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

The first time walk mode is entered on a page, the view **shall** fly down from
above the city onto the spot the walker is placed on, and **shall** end on that
spot with the view the walker would otherwise have started with. Any key or
click **shall** cut the flight short, and it **shall not** play when the page is
asked for reduced motion. The tool and HUD **shall** stay hidden during the
flight. When the first walk's tour is shown, the flight **shall** wait at its
start with the pointer free until the tour is closed, and only then fly on.

## Rationale

Cutting straight from the isometric map to a street loses where on the map the
street is. Seen from above first, the street reads as a place in the city.

## Acceptance criteria

1. On the first entry, the flight starts above the tallest roof and ends exactly
   on the landing spot, facing as that spot faces.
2. Later entries on the same page start on foot at once.
3. A key press or click during the flight lands the walker immediately; `V` and
   `M` then leave walk mode as usual.
4. With `prefers-reduced-motion: reduce`, no flight is played.
5. On a first walk with its tour, the tour is read over the city at the top of
   the flight, the pointer is not captured while it is open, and the flight goes
   on when it is closed.

## Notes

The flight lasts `ARRIVAL` seconds (`walk.js`); `arrivalAt` gives the path.
