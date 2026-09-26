---
id: REQ-TRC-005
uuid: 9ff69f6e-52b2-4773-b225-e3c7c52894a2
title: Who answered each question
scope: trc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The report **shall** hold one entry per question about a package, with plugin,
level, ecosystem, package, version, the index it went or would have gone to, the
number of dependencies and who answered it: a lock file, what a Python
environment has installed, an index, the on-disk cache, an answer already given
in this run, or nobody.

## Rationale

An answer from memory is the same question again, and saying so keeps the report
of a `--watch` re-analysis honest about what was asked.

## Acceptance criteria

1. A lock-file answer, an installed-environment answer, an index answer, a
   cached answer and a repeated question are recorded as `lock`, `installed`,
   `index`, `cache` and `memo`.
2. The totals count each kind of answer and the requests made.
