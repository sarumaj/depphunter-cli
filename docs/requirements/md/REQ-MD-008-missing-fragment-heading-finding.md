---
id: REQ-MD-008
uuid: de1fe7f3-02f2-4fa6-9da6-832a33776c02
title: Missing fragment heading finding
scope: md
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M19
verification:
  - unit
---

## Statement

The system **shall** report a link whose fragment names no anchor of the
Markdown document it points at, or of the same document for a bare `#fragment`,
as a `link/missing-anchor` finding of low severity; the anchors of a document
**shall** be its heading slugs (lower-cased, formatting removed, characters
other than letters, digits, spaces, `-` and `_` dropped, spaces turned to `-`,
repeats suffixed `-1`, `-2`, …), `{#id}` heading anchors, and HTML `name` and
`id` attributes, compared case-insensitively.

## Rationale

A renamed heading breaks every link to its section without any file going
missing.

## Acceptance criteria

1. A fragment naming a heading that exists is silent.
2. A fragment naming a heading that does not exist is reported.
3. `[nothing](#nothing)` in a document without such a heading is reported;
   `[Home](#home)` under `# Home` is not.
4. A fragment into a directory or a non-Markdown file is not checked.
