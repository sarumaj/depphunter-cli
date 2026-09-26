---
id: REQ-WALK-027
uuid: 659f0c46-5985-43e4-b977-0b0137f0676b
title: Fall damage
scope: walk
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

A fall **shall** cost health as it would in life, measured against the walker,
who is half a map unit tall: nothing for a drop of up to 0.9 units (about three
metres); for a longer one, a share of the walker's current maximum health equal
to the part of the drop beyond 0.9 units over the 4.1 units between 0.9 and 5;
and all of it for a drop of 5 units (about seventeen metres) or more, whatever
the backpack holds. A fall **shall** be measured from its top, a jump included.
Being reeled down a line **shall** count as falling from where the walker was
when the line bit, or from the top of a fall already in progress; being reeled
up **shall** end a fall in progress.

## Rationale

Roofs are within reach of the grapple and the jet; arriving on one is only worth
something if leaving it the wrong way costs what it would. A jump off a terrace
wall costs nothing. A share rather than a number of points keeps a full backpack
from making a walker survive what nobody would, and a line is a way up, not a
brake on the way down.

## Acceptance criteria

1. A jump off a terrace wall (0.28 units plus the 0.39 of the jump) costs
   nothing.
2. A drop of 2 units costs something and is survived at full health.
3. A higher drop costs a larger share, and the share does not depend on how
   many bugs the backpack holds.
4. A drop of 5 units kills, with an empty backpack and with a full one, and so
   does a drop of 40.
5. A walker who steps off a tower dies and is returned to the map, with the
   height of the fall given in metres.
