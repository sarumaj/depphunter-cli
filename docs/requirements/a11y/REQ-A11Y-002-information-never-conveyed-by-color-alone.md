---
id: REQ-A11Y-002
uuid: 81b45423-5d9c-412f-afb0-cb812ad52b13
title: Information never conveyed by color alone
scope: a11y
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §2
  - docs/REQUIREMENTS.md §6
verification:
  - manual
  - inspection
---

## Statement

The UI **shall not** convey any information by color alone: what a color encodes
**shall** also be available in words from the tooltip, the labels or the side
panel.

## Rationale

Color is unavailable to some readers and in some situations; the map must still
be readable.

## Acceptance criteria

1. A file's language, a package's floating or unresolved state and an edge's
   direction are each stated in the tooltip or the side panel.
