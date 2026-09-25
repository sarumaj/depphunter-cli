---
id: REQ-FND-016
uuid: ebe671f6-3743-451f-a041-587ddd7d63fd
title: Unreadable input marks the set partial
scope: fnd
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - unit
---

## Statement

A report that cannot be read or parsed, or a database that does not answer,
**shall** mark the findings set partial and **shall not** fail the run; the
findings that were read **shall** still be shown.

## Rationale

A report is not a fatal input; the rest of the map is still worth having.

## Acceptance criteria

1. A set with one unparseable and one valid report holds the valid report's
   findings and `partial: true`.
2. An OSV endpoint that fails yields `partial: true` and no error.
3. The status bar marks a partial set "(partial)".
