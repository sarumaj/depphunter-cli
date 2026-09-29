---
id: REQ-TOOL-078
title: Putting the parachute away while it is open cuts it loose
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

Taking another carried tool into the left hand, or emptying it, while the canopy
is open **shall** cut the canopy away: it **shall** fly off on its own,
emptying, drifting and fading, its lines slack below it, and the walker
**shall** fall from where they are, with the sink they had, as any fall.
Changing the tool in the right hand **shall not**.

## Rationale

Letting go of the toggles is letting go of the canopy, and a walker who switches
to the jet mid-descent should have the jet, not both.

## Acceptance criteria

1. Taking the jet out under an open canopy cuts it away and the walker falls
   from there.
2. Taking the net out under an open canopy does not.
3. A canopy cut away leaves the map within six seconds.
