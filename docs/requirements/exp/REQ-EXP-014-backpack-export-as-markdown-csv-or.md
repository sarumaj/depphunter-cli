---
id: REQ-EXP-014
uuid: 3e0c74f4-9bd6-4beb-9329-7f8ae8a5e385
title: Backpack export as Markdown, CSV or JSON
scope: exp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M15
verification:
  - integration
  - extension
---

## Statement

The server **shall** write the backpack out on `GET
/api/backpack?format=md|csv|json` (JSON by default) as an attachment named
`<project>-backpack.<ext>`, worst severity first and, within a severity, most
recently caught first; another format **shall** be answered 400.

## Rationale

The catch is meant to leave the map: JSON to feed another tool, CSV for a
spreadsheet, Markdown to paste into an issue.

## Acceptance criteria

1. The Markdown export lists each item as a task-list line with the severity in
   bold, the title, the location in a code span and the id in parentheses,
   checked when the item is fixed.
2. The CSV export has the header `id,severity,title,where,line,caught,fixed`.
3. `format=xml` returns 400.
