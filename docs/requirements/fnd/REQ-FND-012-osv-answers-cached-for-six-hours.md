---
id: REQ-FND-012
uuid: 2bbe4905-4103-43f0-bd80-8d3d0f76ef58
title: OSV answers cached for six hours
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

The system **shall** cache the OSV answers for six hours and reuse a cached
answer within that time; an incomplete answer **shall not** be cached.

## Rationale

A version gains advisories with time, so the answers expire; within a working
session they do not have to be asked again.

## Acceptance criteria

1. A second query within six hours makes no request.
2. An answer past its time to live is a cache miss.
3. A batch answer with fewer results than questions is not cached.
