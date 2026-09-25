---
id: REQ-TRC-007
uuid: 6a24e4c8-c81e-421b-9ebd-a29699a3377a
title: Each request URL and status kept
scope: trc
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M18
verification:
  - unit
---

## Statement

For every question that made requests the report **shall** keep each request's
URL, the status or error it returned and its duration, in the order made.

## Rationale

A container image takes three round trips to answer, and which one failed is the
question.

## Acceptance criteria

1. A question answered 404 carries its one request with status `404 Not Found`.
2. A registry that answers 401 records the unauthorized status.
