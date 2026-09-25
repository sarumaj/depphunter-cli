---
id: REQ-FND-001
uuid: a7284f70-4ff9-4c11-b312-da4ff677c0dd
title: Scanner reports named with a repeatable flag
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
