---
id: REQ-TOOL-060
uuid: b4497d91-ff4b-4d51-b69c-310dcb940829
title: Tapped wheel stays up to be read
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

Releasing `R` with the cursor on the hub **shall** leave the wheel open; `R`
again, `Enter` or a click **shall** then take what is under the cursor. Whether
a release takes **shall** be decided by the cursor's position, not by how long
`R` was held.

## Rationale

A hold-versus-tap threshold would have to be timed, and dropped frames would
shut the wheel for no visible reason.

## Acceptance criteria

1. A tap on `R` leaves the wheel up.
2. The hub is a dead zone that points at nothing.
3. `Enter` or a left click takes the tool under the cursor.
