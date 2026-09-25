---
id: REQ-MAP-009
uuid: dd7e21c6-78c8-4f37-9cd4-2b4ca9d4a284
title: Import arcs join visible representatives
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw an import reference as an arc between the visible
representatives of its two ends, where a node's representative is its own box
or, when it is not drawn, the box of its nearest drawn ancestor; an arrow head
**shall** mark the target end.

## Rationale

An edge whose end is folded into a collapsed directory is still shown, attached
to what stands for that end on the map.

## Acceptance criteria

1. Selecting a file that imports a file inside a collapsed directory draws an
   arc to that directory's district block.
2. An edge whose two ends share a representative draws no arc.
3. Each arc carries an arrow head at its target.
