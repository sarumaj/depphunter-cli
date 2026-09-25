---
id: REQ-HUNT-032
uuid: 1fdd4669-33f3-484e-9217-1ccc74d9722f
title: A netted bug is carried by the net
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M26
verification:
  - ui
  - manual
---

## Statement

A netted bug **shall** home on the net's hoop rather than on the walker,
**shall** fight there (bouncing, turning over, squashed against the mesh, the
struggle fading) and **shall** be removed only once the swing has settled.

## Rationale

A bug that flew towards the walker's chest while the net went the other way did
not look caught.

## Acceptance criteria

1. A bug caught in the net stays visibly in the hoop while the hoop moves (M26
   acceptance).
2. The bug turns over at least once during the catch.
