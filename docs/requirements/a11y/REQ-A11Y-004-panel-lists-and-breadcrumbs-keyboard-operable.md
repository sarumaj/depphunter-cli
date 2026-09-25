---
id: REQ-A11Y-004
uuid: dfd17545-570a-47e1-be81-cfe1f9217bac
title: Panel lists and breadcrumbs keyboard operable
scope: a11y
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
  - e2e
---

## Statement

The side panel's dependency rows, symbol rows and breadcrumbs **shall** be
reachable with `Tab` and activated with `Enter` or `Space`.

## Rationale

Walking the graph from the panel is the main keyboard path through the map.

## Acceptance criteria

1. Tabbing into the panel reaches each breadcrumb and dependency row, and
   `Enter` on a row selects its node.
