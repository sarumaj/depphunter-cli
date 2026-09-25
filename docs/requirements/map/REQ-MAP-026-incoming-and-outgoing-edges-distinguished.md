---
id: REQ-MAP-026
uuid: 2f98b571-a2b0-480b-9eed-309fa373cb10
title: Incoming and outgoing edges distinguished
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
verification:
  - manual
---

## Statement

The system **shall** draw a selection's outgoing edges ("depends on") and
incoming edges ("used by") in two distinct colors, and **shall** show each
aggregated arc's edge count, as a tube thickness growing with the count and as a
count in the side panel's dependency lists.

## Rationale

Direction is the first question about a dependency, and the count says how
strong it is.

## Acceptance criteria

1. Arcs from the selection and arcs to it have different colors, as the legend's
   edge key states.
2. An arc standing for more edges is thicker.
3. A dependency row in the side panel standing for several edges shows `xN`.
