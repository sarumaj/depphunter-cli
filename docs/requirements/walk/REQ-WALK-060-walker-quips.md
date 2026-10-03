---
id: REQ-WALK-060
title: The walker remarks on what happens, as a software engineer would
scope: walk
type: functional
priority: may
status: implemented
verification:
  - unit
  - manual
---

## Statement

Now and then, when something happens to the walker, they **shall** say a line
about it to themselves, shown as a speech bubble at the right of the view for
long enough to read and then fading away - no sound. The lines are in the voice
of a software engineer hunting the bugs of their own code: a bobby car come
into view ("What the hell? Was this feature really required?"), walking in,
jumping to a search result, putting the tools away, getting on a swing, a
roundabout, a seesaw or a rider, a slide, a pipe slide or a bobby car, picking
a ball up, a ball going in or not, a jet's thrust, skimming over water, a tool
running dry, the parachute opening, landing under it, meeting a wall under it
or cutting it away, a fall that hurts, running out of breath, a grapple line
going out or running out, something out of reach, tagging a module, putting a
fire out, being bitten - by a critical bug in its own words -, catching a bug,
catching the last one on the map, burning, sinking, standing stock still for
most of a minute and dying. Hurt with under a third of their health left, what
hurts them **shall** be remarked on as how they feel.

Each **shall** have a few lines to choose from, short enough to read at a
glance, and the walker **shall** not chatter: one line at most every ten
seconds, the same thing remarked on no more than every 45 seconds, never the
same line twice running, and a bobby car remarked on only the first time it is
seen. Dying **shall** always be remarked on.

## Rationale

The walk is a game about code, played by people who write it. A word from the
walker when a bug bites or a ride is got on makes it theirs - and a remark that
recurs every few seconds stops being a joke, so the restraint is half of it.

## Acceptance criteria

1. Walking up to a bobby car and looking at it puts up a bubble about it, once.
2. Catching a bug and then being bitten at once says only the first; ten
   seconds on, the second is said.
3. The same line never comes up twice running, and dying is always remarked on.
4. Bitten with a fifth of their health left, the walker says how they feel;
   catching the last bug on the map is remarked on as such; standing still for
   most of a minute says so, once.
