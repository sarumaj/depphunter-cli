---
id: REQ-SUP-004
uuid: 24244c66-965f-4a92-98a0-0c7cc9daf6e0
title: Floating packages are drawn apart
scope: sup
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

The UI **shall** draw a floating package in a color of its own, distinct from
pinned and unresolved packages, in every theme.

## Rationale

A package nothing pins is worth seeing from across the map.

## Acceptance criteria

1. In every theme a floating package building has the `--pkg-floating` color
   and a pinned one the `--pkg` color.
