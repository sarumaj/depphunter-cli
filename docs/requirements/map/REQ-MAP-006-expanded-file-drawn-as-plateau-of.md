---
id: REQ-MAP-006
uuid: 32a21045-8a29-4ff7-800d-84072e5faa8d
title: Expanded file drawn as plateau of symbols
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

The system **shall** draw an expanded file that has symbols as a low plateau
carrying one block per symbol in a near-square grid, the block's height given by
the symbol's kind (types and classes tallest, functions and methods lower,
anything else lowest).

## Rationale

Expanding a file continues the same containment metaphor one level down, so a
file opens into its declarations the way a directory opens into its files.

## Acceptance criteria

1. Expanding a file with n symbols replaces its building with a terrace and n
   boxes of kind `symbol` on it.
2. A type symbol block is 1.1 units tall, a function 0.7 and any other kind
   0.35.
3. A file without symbols cannot be expanded and stays a building.
