---
id: REQ-EXT-035
title: Findings view lists the scanners' findings
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

A Findings view **shall** list the findings of the map shown most recently, as
the server serves them (`GET /api/findings`), ordered by severity, most severe
first, and then by the package or file each one is against, marking those
already in the backpack. It **shall** update when the server announces new
findings or a backpack change, and picking an entry **shall** select on the map
the node the map places the finding on.

## Rationale

The backpack view lists what was caught; the rest of what the scanners said is
worth seeing beside the code as well, where the fixing happens.

## Acceptance criteria

1. The view lists the findings in the order stated once the server announces
   `findings`.
2. An entry in the backpack shows as such; one caught on the map is marked
   without a manual refresh.
3. Picking an entry sends the node the map places it on as the selection.
