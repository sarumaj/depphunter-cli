---
id: REQ-EXT-008
uuid: f0e80ace-0ce8-4399-b84b-2c2643e878f5
title: Map selection reveals the tree row
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

When the server announces a selection made by another client, the Dependencies
view **shall** expand to and select that node's row without taking the keyboard
focus; a view that was hidden **shall** reveal the latest selection when it
becomes visible.

## Rationale

A building picked on the map opens the tree to its row, so the two views and the
map stay one interface.

## Acceptance criteria

1. A `selection` event from another origin reveals the row with `focus: false`.
2. No reveal is attempted while the view is hidden; on becoming visible, the row
   selected in the meantime is revealed.
3. A selected symbol reveals the row of the file that holds it.
