---
id: REQ-EXP-006
uuid: 679b9dbb-a6e1-410a-be89-f29d10f6731c
title: Static HTML export as one file
scope: exp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The system **shall** write the static HTML export as one self-contained file in
which every ES module is inlined as a `data:` URL behind an import map, relative
imports are rewritten to the import map names, and the stylesheet, the 3D models
and the introduction pictures are inlined.

## Rationale

A map that can be shared as a single file needs no server and no hosting.

## Acceptance criteria

1. The export contains an import map naming every module as a
   `data:text/javascript;base64` URL.
2. No inlined module keeps a relative import and the page references no external
   `style.css` or `app.js`.
