---
id: REQ-HUNT-050
title: A finding put in the backpack from the list is caught
scope: hunt
type: functional
priority: must
status: implemented
source:
  - README.md Findings
verification:
  - ui
  - e2e
---

## Statement

Each row of the findings list **shall** carry a button that puts its finding
in the backpack exactly as catching its bug in walk mode does: the same backpack
entry recorded against the same node, handed to the server the same way, after
which its bug no longer walks the map. Putting in a finding already in the
backpack **shall** change nothing; its row **shall** show that it is in the
backpack, and its button **shall** then take it out again.

## Rationale

Catching is how a finding is kept to come back to, and walking up to its bug is
not the only way to decide that it is worth coming back to. Two ways of catching
that recorded different entries would be two backpacks sharing one list.

## Acceptance criteria

1. The entry the button makes equals the entry catching its bug in the street
   makes, apart from the time it was caught.
2. After the button is pressed the bug is counted as caught and can no longer
   be aimed at, and the backpack is sent to the server (`PUT /api/backpack`).
3. Pressing it for a finding already in the backpack adds no second entry; the
   row reads "in the backpack" and its button takes the finding out.
