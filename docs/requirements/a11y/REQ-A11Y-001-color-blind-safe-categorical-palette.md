---
id: REQ-A11Y-001
uuid: bae6e51f-6e58-4df7-bd28-227ce0cdf0de
title: Color-blind-safe categorical palette
scope: a11y
type: non-functional
priority: must
status: implemented
verification:
  - inspection
  - manual
---

## Statement

The UI **shall** color languages from a validated color-blind-safe categorical
palette of seven slots per theme, folding further languages into a neutral
"Other".

## Rationale

Language is the default color mode, and a palette that collapses under common
color vision deficiencies hides it.

## Acceptance criteria

1. The seven series colors of each theme are distinguishable under simulated
   deuteranopia, protanopia and tritanopia.
2. An eighth language is colored as Other.
