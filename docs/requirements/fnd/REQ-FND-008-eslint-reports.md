---
id: REQ-FND-008
title: eslint reports
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read eslint JSON reports as lint findings on the reported
file, line and column, named by the rule, with eslint severity 2 read as medium
and 1 as low.

## Rationale

The most common JavaScript and TypeScript linter.

## Acceptance criteria

1. An eslint message of severity 2 becomes a medium lint finding named by its
   `ruleId`.
