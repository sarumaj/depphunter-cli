---
id: REQ-UI-001
uuid: 61edfb8e-baf2-4936-baa8-d8bcd484d806
title: Toolbar menus open above panel and tooltip
scope: ui
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M7
verification:
  - manual
  - inspection
---

## Statement

The UI **shall** draw the toolbar's menus (Filters, Export) above the side panel
and the tooltip.

## Rationale

A menu partly hidden behind the side panel cannot be used.

## Acceptance criteria

1. With the side panel open, the Filters and Export menus are drawn over it and
   over the tooltip.
