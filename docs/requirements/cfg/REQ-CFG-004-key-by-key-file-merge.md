---
id: REQ-CFG-004
uuid: d59b35e5-2984-482a-bd22-a698aeba2216
title: Config files merged key by key
scope: cfg
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M9
verification:
  - unit
---

## Statement

The system **shall** merge the config files key by key, so that a key absent
from a file of higher precedence keeps the value of a file of lower precedence,
including single keys of the `ui:` section.

## Rationale

A project file that sets only `ui.theme` must not reset the `ui:` keys the user
file sets.

## Acceptance criteria

1. A user file setting `ui.color_by: size` and a project file setting only
   `ui.theme: light` yield color-by `size` and theme `light`.
