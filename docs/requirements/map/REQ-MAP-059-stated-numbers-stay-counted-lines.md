---
id: REQ-MAP-059
title: Stated numbers stay counted lines
scope: map
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

The system **shall** state only counted lines in every number it puts in words -
the status bar's line count, the Lines of a file or directory in the tooltip and
side panel, and the totals a filter computes - and **shall not** include the
byte-derived stand-in in any of them.

## Rationale

A stand-in inside a stated number is a claim about the file rather than about
the reading of it.

## Acceptance criteria

1. The line count in the status bar is unchanged by the presence of a
   megabyte-sized binary.
2. A directory's stated lines exclude its binaries while its drawn size includes
   them.
