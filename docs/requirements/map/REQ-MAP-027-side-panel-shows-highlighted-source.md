---
id: REQ-MAP-027
uuid: b2da7207-4871-4890-929b-0948c6963234
title: Side panel shows highlighted source
scope: map
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M1
verification:
  - manual
  - e2e
---

## Statement

The side panel **shall** show the source of a selected file (or of a selected
symbol's file), syntax-highlighted where the language is known and the file is
smaller than 300,000 bytes, as plain text otherwise, one line per row; for a
symbol it **shall** scroll to and mark the symbol's line.

## Rationale

Reading the code is the natural next step after finding it on the map.

## Acceptance criteria

1. Selecting a Go file shows its source with Go highlighting.
2. Selecting a symbol scrolls the source to the symbol's line and marks it.
3. A file the server does not serve shows the reason instead of the source.
