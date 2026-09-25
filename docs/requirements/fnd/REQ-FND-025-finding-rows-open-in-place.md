---
id: REQ-FND-025
uuid: b9692c8d-65d3-4a49-bbc1-63abf3f0c71e
title: Finding rows open in place
scope: fnd
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - manual
---

## Statement

A finding row **shall** open in place, on click, to show the description, the
fixed version and the advisory link.

## Rationale

A file with forty lint complaints stays readable when details are behind the
row.

## Acceptance criteria

1. Clicking a vulnerability row reveals its description, the fixed version and
   the advisory URL; clicking again hides them.
