---
id: REQ-MAP-003
uuid: 92f16487-6ea0-43aa-9383-75152be10d03
title: Collapsed directory footprint follows file count
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §2
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw a collapsed directory as one district block whose
footprint area is proportional to the number of visible files beneath it, with a
minimum side length.

## Rationale

A collapsed directory stands for everything beneath it; its footprint says how
much is folded into it.

## Acceptance criteria

1. A collapsed directory is one box of kind `district`.
2. A district over four times as many files has twice the side length, above the
   minimum side of 1.4 units.
