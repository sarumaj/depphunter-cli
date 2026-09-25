---
id: REQ-LANG-012
uuid: 07203f58-46d1-4c82-bde3-ffe655edcd95
title: Oversized and minified files skipped
scope: lang
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §4
  - docs/REQUIREMENTS.md M2
verification:
  - unit
---

## Statement

The system **shall not** parse a file larger than 1 MiB (1 048 576 bytes), and
**shall not** parse a file larger than 20 000 bytes whose mean line length
exceeds 250 bytes (a minified file). Such files **shall** still be listed in the
graph.

## Rationale

Such files are almost always generated or bundled: parsing them is slow and
their structure says nothing about the project.

## Acceptance criteria

1. A file measured at more than 1 MiB is not read by any plugin.
2. A 30 000-byte file with 10 lines is not parsed.
3. Both files appear as file nodes.
