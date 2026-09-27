---
id: REQ-EXT-012
title: Backpack entry selects its node
scope: ext
type: functional
priority: should
status: implemented
source:
  - README.md The panel beside the code
verification:
  - extension
---

## Statement

Picking an entry in the Backpack view **should** select, on the map and in the
tree, the node the finding belongs to.

## Rationale

The two views and the map form a single interface: selecting a row selects the
corresponding building.

## Acceptance criteria

1. Picking an entry with a node id sends that id as the selection and reveals
   its tree row.
