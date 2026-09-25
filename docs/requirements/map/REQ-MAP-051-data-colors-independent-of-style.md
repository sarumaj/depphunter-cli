---
id: REQ-MAP-051
uuid: 2f7d42e4-ab55-488b-b2f5-d072c04d284c
title: Data colors independent of style
scope: map
type: functional
priority: must
status: implemented
verification:
  - manual
  - inspection
---

## Statement

The system **shall not** change the colors that carry data - language, size and
history palettes, hover, selection and dimming - with the style.

## Rationale

A reader switching style must not have to relearn the colors.

## Acceptance criteria

1. The same repository in all three styles keeps the same language colors in the
   legend and on the buildings.
