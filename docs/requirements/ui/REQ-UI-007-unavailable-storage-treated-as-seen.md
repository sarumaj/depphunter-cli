---
id: REQ-UI-007
uuid: 6d69eada-32e5-4cf9-a69e-b8e7879bf136
title: Unavailable storage treated as seen
scope: ui
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M22
verification:
  - ui
---

## Statement

When browser storage cannot be read, the UI **shall** treat the introduction as
seen and **shall not** open it by itself.

## Rationale

A dialog on every load is worse than none.

## Acceptance criteria

1. With storage access throwing, a load opens no introduction and the help can
   still open it without an error.
