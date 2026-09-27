---
id: REQ-FND-001
title: Scanner reports named with a repeatable flag
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read the scanner reports named by the repeatable
`--findings` flag (configuration key `findings`), each a file or a glob,
relative to the repository unless absolute; a pattern that matches nothing
**shall** be reported in the log without stopping the other reports.

## Rationale

CI pipelines write reports under arbitrary names and in arbitrary numbers.

## Acceptance criteria

1. `--findings "reports/*.json"` reads every matching report.
2. A named report that does not exist is logged and marks the set partial; the
   others are read.
