---
id: REQ-MAP-058
uuid: 6935bc40-6568-4a6a-818f-04077d54db7a
title: Unread files drawn from their bytes
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M32
verification:
  - ui
  - manual
---

## Statement

The system **shall** draw a file that has no counted lines but a known size (a
binary file, or one over `--max-file-size`) with a drawn size of its bytes
divided by 40, an average bytes-per-line of source, rounded; a file with counted
lines **shall** be drawn from its lines.

## Rationale

Sized by lines alone every unread file came out at the floor height, which says
"there is nothing here" about a file that is mostly what is there.

## Acceptance criteria

1. A repository with a megabyte-sized binary in it draws that binary as a
   building rather than a slab.
2. Twice the bytes is twice the drawn size.
