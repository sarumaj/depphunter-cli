---
id: REQ-MD-013
uuid: 8b273f31-6d3f-4196-890c-6958d58b17e6
title: Uncheckable web links counted, not drawn
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** count the external links that could not be checked (no
answer, 403, 429 or 5xx), **shall** state that count in the log, **shall not**
draw them as findings, and **shall not** mark the set of findings incomplete
because of them.

## Rationale

A host that refuses a robot or does not answer in time is the ordinary case; a
set marked incomplete on every run because of it would stop meaning anything.

## Acceptance criteria

1. A run whose external links include refused and failing ones logs
   `links: N of M external links could not be checked` and reports no finding
   for them.
2. The findings set is not marked incomplete by them.

## Notes

Criterion 2 is not asserted by an automated test.
