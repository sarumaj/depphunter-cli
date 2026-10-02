---
id: REQ-WALK-059
title: Legs, and what the walker wears in each style
scope: walk
type: functional
priority: may
status: implemented
verification:
  - unit
  - manual
---

## Statement

Walk mode **shall** draw the walker's legs from the waist down, a model
prepared by `scripts/legs.py` in Blender and exported to
`web/static/legs.glb` with the script committed as its source, standing at
their feet, turned as they face and seen when they look down:

- striding as they walk, tucked in the air, a leg at a time up a ladder;
- sitting on a swing, a seesaw or a rider - thighs along the seat, shins
  hanging - and out straight down a slide;
- the right leg drawn back and swung through when they kick a ball.

What the walker wears **shall** follow the map's style, on the legs and on the
hands and arms alike: in a city a T-shirt, shorts and sneakers, the arms bare
to a T-shirt's sleeve; on a circuit board an electrician's coverall with knee
pads and a reflective band, work boots, and insulating gloves; in the galaxy a
spacesuit's legs, moon boots, and the suit's sleeves and gloves with a ring of
light at the cuff. Changing the style **shall** change the outfit at once.

## Rationale

A first-person walker who sits on a swing and sees no knees, or kicks a ball
with nothing, is a camera on a stick. And a hand in a spacesuit's glove says
where the walker is as plainly as the sky does.

## Acceptance criteria

1. Looking down while walking shows the feet stepping; on a swing, the
   thighs on the seat and the shins hanging off it.
2. A kick swings the right leg back, through and forward.
3. The city's arms are bare to the T-shirt's sleeve; the board's are gloved
   and sleeved; the galaxy's are a spacesuit's - and the legs likewise.
