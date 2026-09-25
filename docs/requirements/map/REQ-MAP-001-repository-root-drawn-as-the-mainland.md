---
id: REQ-MAP-001
uuid: 8dd49e14-57b9-4e95-bafa-6ceb50780e35
title: Repository root drawn as the mainland
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §2
  - docs/REQUIREMENTS.md M1
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw the analyzed repository's root directory as a
mainland: a land box under the root's terrace, extending beyond it by a fixed
margin on every side, from which every file and directory of the repository is
reached by nesting.

## Rationale

The archipelago metaphor separates what the project is (one land mass) from what
it draws in from outside (islands), so the two read apart at a glance.

## Acceptance criteria

1. The layout of any repository contains exactly one land box whose node is the
   root directory, and every building, district and terrace of the repository
   lies within its footprint.
2. External packages never stand on the mainland.
