---
id: REQ-TOOL-070
title: The parachute is thrown open from high enough
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

The parachute **shall** be a secondary tool, carried in the left hand, taken out
with `T` (its slot the first in the tool row) as well as from the wheel and
with `Q`. With it in hand, `F`, `C` or the middle
button **shall** throw the pilot chute when the walker is in the air at least
0.8 units above what is below them and the parachute is packed, and the canopy
**shall** open on the heading the walker faces, taking over the fall, flight or
line they were on. On the ground, lower, or while it is being repacked it
**shall not** open, and the walker **shall** be told which.

## Rationale

A parachute is a way down from a roof or out of the jet, not a second jump. The
row has ten digits and eleven tools, so the eleventh brings a key of its own.
`T` is a letter nothing else in walk mode answers to, within reach of the left
hand on `W` `A` `S` `D`, and the same key on QWERTY, QWERTZ and AZERTY boards;
the key left of `1` it had first is a dead key on a German board.

## Acceptance criteria

1. The tool row reads `T` and then `1` to `0` from left to right, `KeyT`
   takes the parachute out, and walk mode keeps that key from the map.
2. `F` on the ground or off the top of a jump opens nothing and says it is too
   low; from a roof it throws the pilot chute.
3. The canopy opens on the walker's heading and carries on the fall in progress.

## Notes

The key is `key` on the tool in `web/static/walk/tools.js`; `switcher.js` numbers
the row without it.
