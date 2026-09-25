---
id: REQ-MAP-060
uuid: 8fe0b46e-e9e2-4449-9adf-78fdb2d7f285
title: Unread files state their size
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
  - manual
---

## Statement

For a file that has no counted lines but a known size, the tooltip and the side
panel **shall** state its size in bytes (as bytes, kB, MB, GB or TB) instead of
a line count.

## Rationale

"0 lines" would be a statement about the file rather than about the reading of
it.

## Acceptance criteria

1. The details of a binary file give a size such as `4.2 MB` where a source file
   gives lines.
