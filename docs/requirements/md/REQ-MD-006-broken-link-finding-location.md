---
id: REQ-MD-006
uuid: 814e2f48-8287-41c6-97a7-48339441fbb1
title: Broken link finding carries line and column
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** report each link that leads nowhere as a finding of kind
link, source `links`, on the document and line that carry it, with the 1-based
byte column where the link starts and the link as written; two broken links on
one line **shall** be two findings.

## Rationale

Without the column, two breaks on one line would be merged as one finding
repeated.

## Acceptance criteria

1. A table row `| [a](a.md) | [b](b.md) |` with neither file present yields two
   findings after deduplication.
2. Every link finding carries a non-zero column and the link text as its detail.
