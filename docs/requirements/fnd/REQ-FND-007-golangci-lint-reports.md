---
id: REQ-FND-007
title: golangci-lint reports
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read golangci-lint JSON reports as lint findings on the
reported file, line and column, named by the linter.

## Rationale

The most common Go linter aggregator.

## Acceptance criteria

1. A golangci-lint issue becomes a lint finding with the linter name as
   reference and the position of the issue.
