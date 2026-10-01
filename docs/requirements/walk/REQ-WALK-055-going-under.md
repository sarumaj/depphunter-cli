---
id: REQ-WALK-055
title: Drowning is seen as going under
scope: walk
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

While the walker is in deep water with nothing to float on, walk mode **shall**
show them going under, in step with the health the water is taking: the eye
**shall** sink most of the way to the surface without passing it, bobbing while
there is still fight in them; the water **shall** close over the view from the
bottom of the screen until it covers it when the health is gone; bubbles
**shall** rise through it, fewer as the walker tires; and the held tools
**shall** dip and sway. Reaching a shore, a line or the skimmers in time
**shall** drain it away again over about a second. What rises is the style's
water: the bay in the city; on a circuit board the live backplane, dark and
buzzing, with sparks climbing it; in the galaxy the void, starred and rimmed
with light, with glowing motes drifting up. What the walker is told, and what
they die of, names it the same way. Where the page asks for
reduced motion, there **shall** be no bobbing, swaying or bubbles.

## Rationale

The health bar draining was the only sign that the water was taking the
walker, and it is in a corner of the screen; the view itself stayed dry.

## Acceptance criteria

1. Half the health gone in the water is half way under; all of it is all the
   way, with the water covering the view.
2. Out of the water, the view is dry again within a second or so, not at once.
3. The eye never goes below the surface.
4. With reduced motion, the view sinks and the water rises, and nothing else
   moves.
5. Going under on a circuit board or in the galaxy looks like that style's
   water and is called by its name.
