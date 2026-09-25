---
id: REQ-TRC-015
uuid: c24926e0-44d5-4ee0-b5f2-ef79a16af844
title: Long lists and cells are cut
scope: trc
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
---

## Statement

The text digest **shall** list at most 50 unanswered questions and state how
many more there were; the text digest **shall** cut a cell longer than 72
characters and the Markdown document one longer than 200, ending it with an
ellipsis. The JSON **shall** keep every value whole.

## Rationale

A signed registry-redirect URL is several hundred characters and turns a table
into a ragged wall.

## Acceptance criteria

1. A Markdown cell of 250 characters is cut to 200.
2. The text digest of 60 unanswered questions lists 50 and says 10 more.

## Notes

The Markdown document keeps the full list of questions and cuts only its cells;
the design log says the written report cuts its long lists, which holds for the
text digest only.
