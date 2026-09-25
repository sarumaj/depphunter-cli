---
id: REQ-HUNT-009
uuid: 5d687775-afad-493d-a6a1-30bb399e8ca7
title: Tag result reported in the HUD
scope: hunt
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - e2e
---

## Statement

The result of a tag (the module name, the running tally and how to get its
details) **shall** be reported in the walk-mode HUD message line, not in the
status corner.

## Rationale

The walker reads the HUD; the status corner is outside their attention.

## Acceptance criteria

1. After a tag, the HUD message names the module and the number tagged so far.
2. The status corner text is unchanged by a tag.
