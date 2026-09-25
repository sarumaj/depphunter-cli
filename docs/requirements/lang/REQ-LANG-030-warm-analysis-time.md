---
id: REQ-LANG-030
uuid: 60133cd2-bd8b-427a-b4c8-32c5523527ec
title: Warm analysis time
scope: lang
type: non-functional
priority: should
status: implemented
source:
  - docs/REQUIREMENTS.md §6
  - docs/REQUIREMENTS.md M3
verification:
  - e2e
---

## Statement

A warm analysis of an unchanged project **should** complete near-instantly
through the content-hash cache, dominated by scanning and resolution rather than
parsing.

## Rationale

Repeated runs and watch-mode re-analyses happen far more often than cold runs.

## Acceptance criteria

1. A warm run over an unchanged project parses no file (REQ-LANG-028).
2. A warm run over a 10 000-file project completes in a small fraction of the
   cold run's time.

## Notes

Measured on 2026-09-25 on the reference project of REQ-LANG-029 (4 cores):
the warm run took 1.2 s against 8.3 s cold.
