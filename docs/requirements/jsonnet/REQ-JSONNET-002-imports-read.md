---
id: REQ-JSONNET-002
title: Imports read
scope: jsonnet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every `import`, `importstr` and `importbin` of a string literal
(double- or single-quoted, verbatim `@"..."` / `@'...'`) **shall** be read
as an import at its line, with `importstr x` and `importbin x` as the specs
of the latter two, and nothing inside a comment (`//`, `#`, `/* */`), a
string or a `|||` text block **shall** be.

## Rationale

Jsonnet imports are string literals; any other text that looks like one
is data.

## Acceptance criteria

1. The fixture's imports are read with their kinds, and the imports written
   in comments, strings, verbatim strings and text blocks are not.
