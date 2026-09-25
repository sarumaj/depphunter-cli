---
id: REQ-LANG-011
uuid: 9866b8ed-a35f-4597-8f61-00ad0e52deed
title: Per-file parse time bound
scope: lang
type: non-functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

The system **shall** bound the time spent parsing one file with tree-sitter to 3
seconds, and **shall** use the partial result when the bound is reached.

## Rationale

A single pathological file must not dominate the analysis; the imports near the
top of a file are usually parsed before the bound.

## Acceptance criteria

1. Each tree-sitter parser is created with a timeout of 3 000 000 microseconds.
2. A parse that times out does not fail the analysis.
