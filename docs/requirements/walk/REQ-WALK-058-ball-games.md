---
id: REQ-WALK-058
title: Playing ball, empty-handed
scope: walk
type: functional
priority: may
status: implemented
verification:
  - unit
  - manual
---

## Statement

Every pitch, basketball court and volleyball court within 14 units of the
walker **shall** have its own ball out, which falls, bounces off the ground, the
posts and the walls, and rolls to a stop on the ground under its middle - on a
court's surface, not sunk in it, and not on a curb it is beside. It **shall**
move only when played: a walker who walks into it **shall** stop against it,
and a click **shall** play a ball only when the walker is facing it, within
about 30 degrees. A ball lying off its court, or up on something, **shall** be
put back in the middle of its court after about a second, and one in the water
or far off at once. The walker, likewise, **shall** stand on a court's surface.
With both hands empty (REQ-WALK-057):

- a basketball **shall** be picked up with a click, held in both hands, and
  shot with another. It **shall** bounce off the rim and the board, and one
  through the ring from above **shall** be called: "Swish" when it touched
  nothing;
- a football **shall** be kicked with a click, the ball leaving as the foot
  meets it, harder running; one over the goal line between the posts and
  under the bar **shall** be called a goal, and the net **shall** hold it;
- a volleyball **shall** be picked up with a click and served with another -
  jumping first makes it a jump serve, struck from over the head on a flatter
  arc - and hit again in the air with a click. The net **shall** stop a ball
  that meets it, and a serve that lands in the far court **shall** be called
  in, a jump serve an ace, one outside it out.

Each **shall** leave the way the walker looks, lifted by the throw, the kick or
the serve, and at its own speed: nothing **shall** aim it at a hoop, a goal or
a court, and the way it leaves **shall** follow the view smoothly, not in steps.
The click that sends one **shall** send it as the button comes up: a tap at its
usual speed, and a button held down swinging how hard it goes up to half as
hard again, down to under half as hard and back, every two seconds, shown in
the line saying what a click does, so that letting go at the right moment
sends it as hard as wanted. Picking a ball up **shall** not wait for the
button.
While a ball is held, or one is at the walker's feet to kick or in the air to
hit, the guide the tools are aimed by (REQ-TOOL-082) **shall** show the path it
would take, flown by the ball's own physics, to where it would first come down;
and the ball, sent, **shall** go down that path. A shot's guide **shall** end
instead where it comes back down through the height of its court's rims, its
marker there, so that whether it drops through the ring is seen at the hoop;
a shot **shall** leave from the shooting hand, a little right of the eye, so
that its arc is seen as an arc and not as a line up the middle of the view;
and a shot from the free-throw line at the usual strength **shall** go in with
the hoop in view, the eye a little above the rim.

## Rationale

The courts were painted lines with nothing to play with. A shot, a kick and a
serve are aimed by looking, with the guide showing where the ball goes: a ball
pulled onto the hoop or the goal made the guide jump and every shot the same,
and took the game out of playing.

## Acceptance criteria

1. From the free-throw line, looking a little over the rim with the hoop in
   view, a tapped shot goes in, its guide ending in the ring.
2. A kick from the penalty spot, its guide into the goal, is a goal, and the
   net stops it.
3. A serve from behind the baseline, its guide down in the far half, lands
   there, standing and jumping.
4. Two courts side by side each have a ball; a ball beside a curb rests on
   the ground; one kicked off its court is back in the middle a second after
   it stops.
5. A shot, a kick and a serve each follow their guide exactly.
6. A tap shoots at the usual speed; held half a second the shot goes about
   half as hard again, held a second and a half under half as hard, and it
   goes as the guide showed when the button comes up.
