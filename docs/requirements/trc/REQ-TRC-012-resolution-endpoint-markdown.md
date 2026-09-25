---
id: REQ-TRC-012
uuid: 489e6071-ae0e-4fb9-a6e9-dfda06cc0919
title: Resolution report as a document
scope: trc
type: interface
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

`GET /api/resolution?format=md` **shall** render the report as a Markdown
document with tables, including every question kept.

## Rationale

The editor opens this document, so nothing in TypeScript can drift from what the
analysis actually did.

## Acceptance criteria

1. The response is `text/markdown` beginning `# Resolution report` and contains
   the section "Every question asked".
2. A `|` in a value is escaped so the table keeps its columns.

## Notes

The editor command that opens this document is specified by scope
[`ext`](../ext/).
