---
id: REQ-EXT-016
uuid: 1f6bcc41-377c-44a9-9ac0-c97d3abec32a
title: Resolution report rendered by the server
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md Use
verification:
  - extension
---

## Statement

The command **depphunter: Show the Resolution Report** **shall** open the
document the attached server renders at `GET /api/resolution?format=md` as a
Markdown document, shown in the Markdown preview where available, and **shall
not** compose the report in the extension.

## Rationale

The report is rendered once, in Go; reading it from the server means nothing in
TypeScript can drift from what the analysis did, and the document and the
`--explain` digest describe the same analysis.

## Acceptance criteria

1. The opened document has language `markdown` and contains `# Resolution
   report` and `## Indexes this run knew about`.
2. The Markdown preview is requested; if unavailable, the text document is shown
   instead.
3. Without an attached server the command says there is nothing to report on.
