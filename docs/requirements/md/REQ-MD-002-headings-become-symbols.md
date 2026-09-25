---
id: REQ-MD-002
uuid: 426c024d-be35-41cf-a5e1-2d70f26a7e65
title: Headings become document symbols
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** report the ATX and setext headings of a Markdown document
outside fenced code as the document's symbols, with the formatting removed from
their names, kind `title` for level 1 and `heading <n>` for level n, and a
repeated name numbered `<name> (2)`, `<name> (3)`, ….

## Rationale

A document then expands into its sections on the map the way a source file
expands into its functions.

## Acceptance criteria

1. `# The Project` is a symbol `The Project` of kind `title`.
2. `## Install` is a symbol of kind `heading 2`; a second `## Install` is
   `Install (2)`.
3. A line underlined with `---` is a level-2 heading; a `---` line with nothing
   above it is not.
