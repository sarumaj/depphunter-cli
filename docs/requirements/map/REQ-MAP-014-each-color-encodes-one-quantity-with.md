---
id: REQ-MAP-014
uuid: 81250cfa-d807-4f35-bc4e-7cda237ced59
title: Each color encodes one quantity with a legend
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

The system **shall** use every color on the map to encode exactly one quantity
at a time (language, a size or history measure, edge direction, or package
state), and **shall** explain every such encoding in the legend.

## Rationale

A color that means two things, or whose meaning is not stated, cannot be read.

## Acceptance criteria

1. In each color mode the legend names the quantity and shows its colors or
   ramp.
2. The legend shows the two edge colors and what they mean.
3. Every color used for package state (resolved, unresolved, floating) is
   explained in the legend.

## Notes

Fixed: the legend has a Packages entry, shown when there are islands, with the
three package colors (resolved, floating, unresolved) and what each means. The
tooltip and panel also state these in words; that color is never the sole
channel is REQ-A11Y-002.
