---
id: REQ-FND-015
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
