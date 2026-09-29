---
id: REQ-EXT-036
title: A finding is added to the backpack from the editor
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md The panel beside the code
verification:
  - extension
---

## Statement

The command **depphunter: Add to the Backpack** **shall** put a finding in the
backpack with the entry the map's own catch makes (REQ-HUNT-050), first in the
list, by sending the backpack to the server (`PUT /api/backpack`). It **shall**
be offered inline beside each Findings entry not yet in the backpack, while an
entry already in it offers **Take Out of the Backpack** instead. From the
command palette it **shall** ask which of the findings not yet caught to add.
Adding a finding already in the backpack **shall** send nothing.

## Rationale

The map is one way to decide that a finding is worth coming back to; the
editor, where the code is, is another. Whichever client catches it, the entry
must be the same, or the one backpack is two.

## Acceptance criteria

1. The entry sent equals the one the map's `Backpack.add` makes for the node
   the map places the finding on, apart from the time it was caught.
2. The entry is marked as caught in the Findings view and listed in the
   Backpack view; the map takes it from the server's announcement.
3. The inline actions follow the entry's context value: `uncaught` offers
   adding, `caught` offers taking out.
4. From the palette only findings not yet caught are offered.
