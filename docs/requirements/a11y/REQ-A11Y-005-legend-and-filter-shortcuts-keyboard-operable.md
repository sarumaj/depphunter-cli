---
id: REQ-A11Y-005
uuid: 22c0b9dd-a99f-4a34-b8e4-de9c1c309543
title: Legend and filter shortcuts keyboard operable
scope: a11y
type: functional
priority: must
status: implemented
verification:
  - manual
  - e2e
---

## Statement

The legend's language entries and the Filters menu's shortcuts (all, none, and
the checkboxes) **shall** be reachable with `Tab` and operated with `Enter` or
`Space`, a legend entry toggling its language.

## Rationale

Filtering must not depend on a pointing device.

## Acceptance criteria

1. Focusing a legend entry and pressing `Space` hides that language; pressing it
   again shows it.
2. `Tab` reaches the Filters menu's all and none buttons.
