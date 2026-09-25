---
id: REQ-FND-015
uuid: 3c20c67f-1800-49b0-b31d-316b796e9222
title: No-vulns turns reports and database off
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

`--no-vulns` **shall** disable both the reading of scanner reports and the OSV
query.

## Rationale

One switch to turn the vulnerability layer off entirely.

## Acceptance criteria

1. With `--no-vulns --findings r.json --online`, no report is read and OSV is
   not asked.
