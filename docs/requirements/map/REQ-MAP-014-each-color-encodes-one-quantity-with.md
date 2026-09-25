---
id: REQ-MAP-014
uuid: 81250cfa-d807-4f35-bc4e-7cda237ced59
title: Each color encodes one quantity with a legend
scope: map
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md §2
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

Partial: the legend covers languages, the size ramp, the history modes and the
two edge colors, but not the package colors for unresolved and floating packages
(the tooltip and panel state these in words). That color is never the sole
channel is REQ-A11Y-002.
