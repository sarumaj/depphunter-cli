---
id: REQ-WALK-056
title: Esc while held
scope: walk
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

While the walker is held because the pointer was let go (REQ-WALK-044), `Esc`
**shall** put away the details panel, the backpack, the photographs and the
export menu when one of them is open and leave the walker held, and with none
open **shall** return to the map. Where the pointer is never captured
(REQ-WALK-047), `Esc` **shall** walk on instead. A refusal of the pointer lock
in the moment after the user let the pointer go **shall not** count towards
deciding that the pointer cannot be captured on the page.

## Rationale

Browsers take the first `Esc` to free the pointer, and give it back neither for
an `Esc` nor in the second after a release. Walking on from a second `Esc` took
the hold off and left the walker with no mouse to look with; two such refusals
then gave up on capturing the pointer for the rest of the visit.

## Acceptance criteria

1. `Esc`, then `Esc` again with nothing open, returns to the map.
2. With a building's details open, `Esc` closes them and the walker stays held;
   a click on the street walks on.
3. Refusals within 1.5 s of letting the pointer go never turn the page into one
   where the pointer is not captured; two refusals later still do.
