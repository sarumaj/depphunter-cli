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

Walk mode **shall** draw the walker's whole body - the legs, the torso, the
arms and a head with a face - a model prepared by `scripts/legs.py` in Blender
and exported to `web/static/legs.glb` with the script committed as its source,
standing at their feet, turned as they face and seen when they look down. The
chest **shall** be seen only from outside, never cut through: the eye is over
it and a little ahead, the further ahead the further down they look, as a head
bent forward over it is; and it stays upright while the legs lean down a slide.
The head is the eye's own, and **shall** be drawn only for a view from outside
it, turned the way the walker looks.

The legs **shall** be posed:

- striding as they walk, the lower foot on the ground; tucked in the air; a
  leg at a time up a ladder;
- sitting on a swing, a seesaw or a rider - on the seat, thighs along it,
  shins hanging - astride a bobby car, the thighs turned out round its body and
  the feet down beside it, and out straight down a slide, leaning back along
  its slope; sat, the body faces the way the seat or the car does, whichever
  way the walker looks;
  sitting down and getting up over a moment. Sat anywhere, no leg **shall** go
  through the seat, the chute ahead or the ground under it;
- kicking a ball, the right leg drawn back with the heel up, whipped through at
  the knee to meet the ball under the hips as it leaves, and swung on up and
  down again, eased all the way, while the standing knee gives.

There **shall** be one pair of hands at a time, and always on the shoulders:
while a tool or a ball is held before the eye, those are the hands seen and the
body's arms are not drawn; empty-handed, or looking down past what is held -
which sinks out of the view as the eye goes down - the body's own arms are the
hands. The arms **shall** hang a little forward at rest, the hands before the
thighs; swing against the legs walking, further and bent at the elbow
running; go out for balance in the air; climb a ladder hand over hand; hold on
to a ride - a swing's chains, a seesaw's or a rider's handles, a car's wheel, a
roundabout's rail, a chute's sides - as far as the walker is sat on it; and be
thrown forward and back against a kick. The shoulders **shall** turn a little
against the stride.

What the walker wears **shall** follow the map's style, on the body and on the
hands and arms alike: in a city a T-shirt, shorts and sneakers, the arms bare
to a T-shirt's sleeve; on a circuit board an electrician's coverall with a
chest pocket, knee pads and reflective bands, work boots, and insulating
gloves; in the galaxy a spacesuit with a ring at the neck and a control panel
on the chest, moon boots, and the suit's sleeves and gloves with a ring of
light at the cuff. Changing the style **shall** change the outfit at once.

Clothes **shall** look like cloth over a body and not like skin of another
color: a sleeve and a glove are layers over the arm, cut straight across where
they end with a rim down to the skin; every fabric - a T-shirt's knit, a twill,
rubber, a spacesuit's quilted panels, leather - **shall** show its weave or seams
in its shading and catch the light as matte cloth does, the weave fading out
where it would be finer than the screen can show.

## Rationale

A first-person walker who sits on a swing and sees no knees, or kicks a ball
with nothing, is a camera on a stick. And a hand in a spacesuit's glove says
where the walker is as plainly as the sky does.

## Acceptance criteria

1. Looking down while walking shows the feet stepping; on a swing, the
   thighs on the seat and the shins hanging off it.
2. A kick draws the right leg back, strikes the ball with it under the hips,
   and follows through high, with no joint jumping between frames.
3. Sitting on a seesaw's end at the ground, the feet are above it; all the
   way down a slide and along its run-out, the legs are on the chute and the
   ground, not in them.
4. A sleeve ends in a clean edge with a rim, and the shorts, the coverall and
   the suit show their weave or seams up close.
5. The city's arms are bare to the T-shirt's sleeve; the board's are gloved
   and sleeved; the galaxy's are a spacesuit's - and the legs likewise.
6. Looking straight down shows the top of the chest - a T-shirt, a coverall
   or a spacesuit - and the toes past it, and no inside of the body.
7. Holding a tool, looking down sinks the tool out of view and shows the
   body's arms instead; never two pairs of hands, never none.
8. Walking, the left arm swings forward with the right leg; on a bobby car the
   hands are on the wheel.
