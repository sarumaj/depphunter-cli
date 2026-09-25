---
id: REQ-TOOL-043
uuid: f8141e6c-d413-447a-b3c9-61bbbad80cdd
title: Held trigger only for nail gun and extinguisher
scope: tool
type: functional
priority: must
status: implemented
verification:
  - ui
  - e2e
---

## Statement

The nail gun and the fire extinguisher **shall** keep firing while the button is
held; every other tool **shall** fire once per click.

## Rationale

A strip nailer and an extinguisher are hoses; everything else is one act.

## Acceptance criteria

1. The nail gun empties its magazine into a wall while the button is held and
   the dart does not.
2. Only the nail gun and the extinguisher have a cadence.
