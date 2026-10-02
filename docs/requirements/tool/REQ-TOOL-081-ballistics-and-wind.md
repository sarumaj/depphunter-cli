---
id: REQ-TOOL-081
title: Shots, the walker and the canopy in the wind
scope: tool
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

A thrown or fired shot **shall** fall under its tool's gravity - the same for
anything solid, upwards for a bubble - and **shall** be held back by the air in
proportion to its speed through the air (for light, slow shots) and to the
square of that speed (for small, fast ones). Walk mode **shall** have a wind
that blows level, at about a walker's pace on average, wandering in direction
over minutes and in strength over seconds; drag **shall** act on a shot's speed
through that moving air. A shot **shall** be flown in steps of a fixed length,
so it lands in the same place at any frame rate.

An aimed shot **shall** be solved before it leaves: the launcher **shall** find
the lower of the arcs, and the lead into the wind, that put it on the mark at
the tool's speed, and the shot **shall** fly that path, following a bug that
walks on meanwhile. A shot the air will not carry to its mark - a bubble into
the wind - **shall** leave towards it and go where its physics take it. The
HUD **shall** show the wind's strength, and which way it blows as the walker
faces, in the first row of its counters.

The same wind **shall** carry the walker: by a tenth of its speed while they
walk, a fifth while they are off their feet, and nearly half under the jet
backpack; somebody standing still **shall not** be moved. A canopy **shall**
fly through the moving air - its lift, drag and bank answering to its speed
through it - so it drifts with the wind, makes less headway into it, and lands
slower over the ground into it than with it; a canopy cut away **shall** drift
with the wind as well.

## Rationale

An aimed shot slid from the muzzle to the mark along a curve drawn to fit, and
a miss was moved a frame at a time, so its drop and drag depended on the frame
rate; nothing blew. A dart's lob, a nail's flatness and a bubble that drifts are
what their physics make of them, and a wind is what makes a long lob a shot to
judge. A wind that moved bubbles and left a parachute hanging still would be
two winds.

## Acceptance criteria

1. An aimed dart, nail, hook or bobber lands on its mark, near and at the
   tool's reach, in still air and in a crosswind, leading into the wind.
2. A bubble blown into a gust does not reach a mark it reaches in still air.
3. Over the same distance a bubble drifts with the wind more than a dart, and a
   dart more than a nail.
4. A shot flown at 20 frames a second ends within 0.05 of the same shot at 140.
5. The HUD's wind arrow turns as the walker turns.
6. Walking, in the air and under the jet the walker is carried downwind, the
   more the less they are on their feet; standing, they are not.
7. A canopy opened in a crosswind drifts with it; landed into the wind it
   arrives slower over the ground than landed with it.
